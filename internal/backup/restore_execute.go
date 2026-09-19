package backup

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

func (s *Service) checkpoint(j *restoreJournal, step string) error {
	j.Step = step
	if err := s.writeJournal(j); err != nil {
		return err
	}
	if s.restoreStep != nil {
		return s.restoreStep(step)
	}
	return nil
}

func (s *Service) executeRestore(j restoreJournal) {
	if s.operationLock != nil {
		release, err := s.operationLock()
		if err != nil {
			s.rejectBeforeCommit(&j, err)
			return
		}
		defer release()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	window, err := s.store.Maintenance().Begin(ctx, "restore")
	if err != nil {
		s.rejectBeforeCommit(&j, err)
		return
	}
	defer window.Close()
	if err := s.checkRestoreTargets(&j); err != nil {
		s.rejectBeforeCommit(&j, err)
		return
	}
	if err := s.capturePreimage(&j); err != nil {
		s.rejectBeforeCommit(&j, err)
		return
	}
	if err := s.prepareTargetUploads(&j); err != nil {
		s.rejectBeforeCommit(&j, err)
		return
	}
	j.PreimageReady = true
	j.HasPreimage = true
	j.Status = RestoreApplying
	if err := s.checkpoint(&j, "preimage"); err != nil {
		s.finishRollback(&j, err, window.Hold)
		return
	}
	if err := s.commitRestore(&j); err != nil {
		s.finishRollback(&j, err, window.Hold)
		return
	}
	j.Status = RestoreSucceeded
	j.Error = ""
	if err := s.checkpoint(&j, "completed"); err != nil {
		// rename 可能已完成而目录 fsync 失败。此时不再变更已一致的数据，
		// 下次启动根据实际持久日志选择保留成功结果或从前像补偿。
		window.Hold()
		s.markUncertain(j.ID)
		log.Printf("restore terminal persistence uncertain: %v", err)
		return
	}
	if err := s.pruneSuccessfulPreimages(3); err != nil {
		log.Printf("restore preimage retention: %v", err)
	}
	if err := s.cleanTaskStaging(j.ID); err != nil {
		log.Printf("restore staging cleanup: %v", err)
	}
	window.Close()
	// 普通通知与异步派生更新不参与整站事务，失败不能让成功恢复回滚。
	if s.restoreHook != nil {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Printf("restore completion callback failed")
				}
			}()
			s.restoreHook()
		}()
	}
}

func (s *Service) rejectBeforeCommit(j *restoreJournal, cause error) {
	j.Status = RestoreRolledBack
	j.Error = "恢复预备失败，原数据未改变"
	if err := s.writeJournal(j); err != nil {
		log.Printf("restore preparation journal: %v", err)
		s.store.Maintenance().Block()
	}
	if err := s.cleanTaskStaging(j.ID); err != nil {
		log.Printf("restore preparation cleanup: %v", err)
	}
	if !j.HasPreimage {
		if err := os.RemoveAll(filepath.Join(s.taskRoot(j.ID), "preimage")); err != nil {
			log.Printf("restore incomplete preimage cleanup: %v", err)
		}
	}
	log.Printf("restore preparation failed: %v", cause)
}
func (s *Service) capturePreimage(j *restoreJournal) error {
	root := filepath.Join(s.taskRoot(j.ID), "preimage")
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	if err := s.store.BackupTo(s.beforePath(j.ID, "blog.db")); err != nil {
		return err
	}
	if file, err := os.OpenFile(s.beforePath(j.ID, "blog.db"), os.O_RDWR, 0); err != nil {
		return err
	} else if err := errors.Join(file.Sync(), file.Close()); err != nil {
		return err
	}
	if s.uploadDir != "" {
		info, err := os.Lstat(s.uploadDir)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("uploads changed to non-directory")
			}
			j.UploadsExisted = true
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := copyTree(s.uploadDir, s.beforePath(j.ID, "uploads")); err != nil {
			return err
		}
	}
	if s.sitePath != "" {
		info, err := os.Lstat(s.sitePath)
		if err == nil {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("site configuration is not a regular file")
			}
			j.SiteExisted = true
			j.SiteMode = uint32(info.Mode().Perm())
			if err := copyFile(s.sitePath, s.beforePath(j.ID, "site.yml"), 0600); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if err := syncTree(root); err != nil {
		return err
	}
	// 前像目录入口也必须落盘；仅同步其中的文件不足以抵抗断电。
	for _, directory := range []string{s.taskRoot(j.ID), filepath.Dir(s.taskRoot(j.ID)), s.backupDir, filepath.Dir(s.backupDir)} {
		if err := syncDirectory(directory); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) prepareTargetUploads(j *restoreJournal) error {
	if j.DatabaseOnly || s.uploadDir == "" {
		return nil
	}
	next := s.uploadStage(j.ID, "next")
	if err := os.Mkdir(next, 0700); err != nil {
		return err
	}
	if !j.Strict {
		if err := copyTree(s.beforePath(j.ID, "uploads"), next); err != nil {
			return err
		}
	}
	if err := copyTree(s.inputPath(j.ID, "uploads"), next); err != nil {
		return err
	}
	return syncTree(next)
}
func (s *Service) commitRestore(j *restoreJournal) error {
	if err := s.store.RestoreFrom(s.inputPath(j.ID, "blog.db")); err != nil {
		return fmt.Errorf("restore database: %w", err)
	}
	if err := s.checkpoint(j, "database"); err != nil {
		return err
	}
	if !j.DatabaseOnly && s.uploadDir != "" {
		if j.UploadsExisted {
			if err := os.Rename(s.uploadDir, s.uploadStage(j.ID, "old")); err != nil {
				return err
			}
		}
		if err := os.Rename(s.uploadStage(j.ID, "next"), s.uploadDir); err != nil {
			return err
		}
		if err := syncDirectory(filepath.Dir(s.uploadDir)); err != nil {
			return err
		}
	}
	if err := s.checkpoint(j, "uploads"); err != nil {
		return err
	}
	if j.RestoreSite {
		data, err := readRegularFile(s.inputPath(j.ID, "site.yml"), maxZipEntryUncomp)
		if err != nil {
			return err
		}
		if err := writeFileSync(s.sitePath, data, os.FileMode(j.SiteMode)); err != nil {
			return err
		}
	}
	if err := s.checkpoint(j, "site"); err != nil {
		return err
	}
	j.Status = RestoreRebuilding
	if err := s.checkpoint(j, "rebuilding"); err != nil {
		return err
	}
	if s.restoreRebuild != nil {
		if err := s.restoreRebuild(); err != nil {
			return fmt.Errorf("rebuild restored state: %w", err)
		}
	}
	return nil
}

func (s *Service) finishRollback(j *restoreJournal, cause error, hold func()) {
	j.Status = RestoreApplying
	j.Step = "compensating"
	j.Error = "恢复未完成，正在补偿"
	// 应用阶段日志已落盘；即使本次日志写失败，下次启动仍须按非终态补偿。
	journalErr := s.writeJournal(j)
	compensationErr := s.compensate(j)
	if compensationErr == nil && s.restoreRebuild != nil {
		compensationErr = s.restoreRebuild()
	}
	if compensationErr != nil || journalErr != nil {
		hold()
		j.Status = RestoreRecoveryRequired
		j.Error = "自动补偿未完成，站点保持维护，请管理员检查恢复日志"
		if err := s.writeJournal(j); err != nil {
			log.Printf("restore recovery journal: %v", err)
		}
		log.Printf("restore failed: %v; compensation: %v; journal: %v", cause, compensationErr, journalErr)
		return
	}
	j.Status = RestoreRolledBack
	j.Error = "恢复失败，已恢复原数据和运行状态"
	if err := s.writeJournal(j); err != nil {
		hold()
		s.markUncertain(j.ID)
		log.Printf("restore rollback journal: %v", err)
		return
	}
	if err := s.cleanTaskStaging(j.ID); err != nil {
		log.Printf("restore rollback cleanup: %v", err)
	}
	log.Printf("restore rolled back: %v", cause)
}

// compensate 从不可变前像重新构造目标，重复执行和在补偿途中崩溃都不会消费前像。
func (s *Service) compensate(j *restoreJournal) error {
	if !j.PreimageReady {
		return fmt.Errorf("restore preimage is incomplete")
	}
	if err := s.store.RestoreFrom(s.beforePath(j.ID, "blog.db")); err != nil {
		return err
	}
	if !j.DatabaseOnly && s.uploadDir != "" {
		rollback := s.uploadStage(j.ID, "rollback")
		if err := os.RemoveAll(rollback); err != nil {
			return err
		}
		if j.UploadsExisted {
			if err := copyTree(s.beforePath(j.ID, "uploads"), rollback); err != nil {
				return err
			}
			if err := syncTree(rollback); err != nil {
				return err
			}
		}
		if err := os.RemoveAll(s.uploadDir); err != nil {
			return err
		}
		if j.UploadsExisted {
			if err := os.Rename(rollback, s.uploadDir); err != nil {
				return err
			}
		}
		if err := syncDirectory(filepath.Dir(s.uploadDir)); err != nil {
			return err
		}
	}
	if j.RestoreSite {
		if !j.SiteExisted {
			return fmt.Errorf("configuration preimage is missing")
		}
		data, err := readRegularFile(s.beforePath(j.ID, "site.yml"), maxZipEntryUncomp)
		if err != nil {
			return err
		}
		if err := writeFileSync(s.sitePath, data, os.FileMode(j.SiteMode)); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) cleanTaskStaging(id string) error {
	if err := os.RemoveAll(filepath.Join(s.taskRoot(id), "input")); err != nil {
		return err
	}
	if s.uploadDir != "" {
		for _, suffix := range []string{"next", "old", "rollback"} {
			if err := os.RemoveAll(s.uploadStage(id, suffix)); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *Service) pruneSuccessfulPreimages(keep int) error {
	journals, err := s.journals()
	if err != nil {
		return err
	}
	retained := 0
	for _, j := range journals {
		if j.Status != RestoreSucceeded || !j.HasPreimage {
			continue
		}
		retained++
		if retained <= keep {
			continue
		}
		// 先记录前像已过期，再删除数据；中断最多遗留可人工清理的旧前像。
		j.HasPreimage = false
		if err := s.writeJournal(&j); err != nil {
			return err
		}
		if err := os.RemoveAll(filepath.Join(s.taskRoot(j.ID), "preimage")); err != nil {
			return err
		}
	}
	return nil
}

// RecoverPending 必须在读取 site.yml 和执行任何数据库迁移之前调用。
func (s *Service) RecoverPending() error {
	journals, err := s.journals()
	if err != nil {
		return err
	}
	for _, j := range journals {
		if j.Status == RestoreSucceeded || j.Status == RestoreRolledBack {
			continue
		}
		if s.operationLock != nil {
			release, err := s.operationLock()
			if err != nil {
				return err
			}
			err = s.recoverTask(&j)
			release()
			if err != nil {
				return err
			}
		} else if err := s.recoverTask(&j); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) recoverTask(j *restoreJournal) error {
	if j.Status == RestorePreparing || j.Status == RestoreWaiting {
		j.Status = RestoreRolledBack
		j.Error = "进程中断发生在提交前，原数据未改变"
	} else {
		if err := s.compensate(j); err != nil {
			j.Status = RestoreRecoveryRequired
			j.Error = "启动补偿失败，站点保持维护"
			if writeErr := s.writeJournal(j); writeErr != nil {
				return errors.Join(err, writeErr)
			}
			return fmt.Errorf("%w: %v", ErrRecoveryRequired, err)
		}
		j.Status = RestoreRolledBack
		j.Error = "启动时已从完整前像恢复原数据"
	}
	if err := s.writeJournal(j); err != nil {
		return err
	}
	return s.cleanTaskStaging(j.ID)
}
