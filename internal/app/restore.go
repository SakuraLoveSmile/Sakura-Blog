package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"blog/internal/backup"
	"blog/internal/config"
	"blog/internal/content"
	"blog/internal/maintenance"
	"blog/internal/module"
	storagemodule "blog/internal/module/storage"
	"blog/internal/search"
	"blog/internal/store"
)

func operationLockPath(app *config.App) string {
	if path := os.Getenv("BLOG_MAINTENANCE_LOCK"); path != "" {
		return path
	}
	return filepath.Join(filepath.Dir(app.DBPath), "maintenance.lock")
}

// recoverBeforeBootstrap 在配置解析和迁移之前只使用原生数据库连接执行补偿。
func recoverBeforeBootstrap(app *config.App, sitePath string) error {
	cfg := backup.Config{BackupDir: filepath.Join(filepath.Dir(app.DBPath), "backups"), UploadDir: app.UploadDir, SitePath: sitePath}
	probe := backup.New(cfg)
	tasks, err := probe.ListRestoreTasks()
	if err != nil {
		return err
	}
	pending := false
	for _, task := range tasks {
		if task.Status != backup.RestoreSucceeded && task.Status != backup.RestoreRolledBack {
			pending = true
		}
	}
	if !pending {
		return nil
	}
	if _, err := os.Stat(app.DBPath); err != nil {
		return fmt.Errorf("recovery database unavailable: %w", err)
	}
	native, err := store.OpenForRecovery(app.DBPath)
	if err != nil {
		return err
	}
	cfg.Store = native
	service := backup.New(cfg)
	service.SetOperationLock(func() (func(), error) { return maintenance.LockOperation(operationLockPath(app)) })
	return errors.Join(service.RecoverPending(), native.Close())
}

func (s *Services) rebuildRestoredState() error {
	base, err := config.LoadSite(s.SitePath)
	if err != nil {
		if !s.devMode {
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			base, err = config.LoadPersistedStellarSite(s.Store)
			if err != nil {
				return err
			}
		} else {
			base = config.DefaultSite()
		}
	}
	if err := config.RebuildSiteFromBase(s.Store, s.Site, base); err != nil {
		return err
	}
	if !s.devMode && !s.Site.Load().Admin.AuthEnabled {
		return fmt.Errorf("restored database must retain administrator authentication")
	}
	candidate, err := storagemodule.NewManager(s.Store, s.App.UploadDir)
	if err != nil {
		return err
	}
	if err := s.Storage.Reconfigure(candidate.Config()); err != nil {
		return err
	}
	deps := module.Deps{Store: s.Store, Posts: s.Post, Storage: s.Storage, Site: s.Site, Modules: s.Modules}
	if err := s.Modules.ReloadRestoredState(deps); err != nil {
		return err
	}
	if s.Search != nil {
		items, err := search.ItemsForRestore(s.Store)
		if err != nil {
			return err
		}
		if err := s.Search.Rebuild(items); err != nil {
			return err
		}
	}
	if s.RAG != nil {
		if err := s.RAG.InvalidateForRestore(); err != nil {
			return err
		}
	}
	if s.Friends != nil {
		s.Friends.ResetRestoredCache()
	}
	cacheDir := filepath.Join(filepath.Dir(s.App.DBPath), "ogcache")
	entries, err := os.ReadDir(cacheDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".png" {
			if err := os.Remove(filepath.Join(cacheDir, entry.Name())); err != nil {
				return err
			}
		}
	}
	// 必要缓存失效在维护许可释放之前完成，避免新请求看到恢复前的派生内容。
	s.Events.Publish(content.Event{Kind: content.Restored})
	return nil
}
