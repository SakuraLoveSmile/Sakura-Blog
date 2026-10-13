package stellarmigration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"blog/internal/maintenance"
	"golang.org/x/sys/unix"
)

// MigrationRuntime由composition root提供。Quiesce必须排空全部请求、worker并关闭全部DB连接；
// ValidateStaged只可打开演练root，须完成正式导入/索引重建/渲染检查且返回前关闭全部连接。
// Resume在维护窗口内启动，成功前不放行普通流量；HoldRecovery必须保持服务不可用。
type MigrationRuntime interface {
	Quiesce(context.Context) error
	ValidateStaged(context.Context, RootRehearsal) error
	Resume(context.Context) error
	HoldRecovery(error)
}
type RootApplyResult struct {
	BackupDirectory         string `json:"backup_directory"`
	RootDigest              string `json:"root_digest"`
	ResolvedCandidateDigest string `json:"resolved_candidate_digest"`
}
type RootRecoveryResult struct {
	Recovered       bool   `json:"recovered"`
	BackupDirectory string `json:"backup_directory,omitempty"`
}
type rootJournal struct {
	Version         int        `json:"version"`
	Layout          RootLayout `json:"layout"`
	BackupDirectory string     `json:"backup_directory"`
	RootDigest      string     `json:"root_digest"`
	Targets         []string   `json:"targets"`
	State           string     `json:"state"`
}

// ApplyReviewedRootMigration仅在完整批准、runtime关闭及fresh资源摘要一致后安装。
// 完整前像永久保留；失败补偿不使用已取消的请求ctx。真实调用仍需用户最终执行授权。
func ApplyReviewedRootMigration(ctx context.Context, r RootCapture, c Candidate, a RootMigrationApproval, decisions NotebookStructureDecisions, runtime MigrationRuntime) (RootApplyResult, error) {
	return applyReviewedRootMigration(ctx, r, c, a, decisions, runtime, nil)
}
func applyReviewedRootMigration(ctx context.Context, r RootCapture, c Candidate, a RootMigrationApproval, decisions NotebookStructureDecisions, runtime MigrationRuntime, hook func(string) error) (out RootApplyResult, err error) {
	if runtime == nil {
		return out, fmt.Errorf("migration requires an explicit runtime maintenance gate")
	}
	if err = validateRootApproval(r, c, a); err != nil {
		return out, err
	}
	if _, _, _, err = resolveMigration(c, a.MigrationApproval, decisions); err != nil {
		return out, err
	}
	if err = noSymlinkPath(filepath.Dir(r.Layout.OperationLock)); err != nil {
		return out, err
	}
	unlock, err := maintenance.LockOperation(r.Layout.OperationLock)
	if err != nil {
		return out, err
	}
	defer unlock()
	if err = runtime.Quiesce(ctx); err != nil {
		runtime.HoldRecovery(err)
		return out, err
	}
	held := false
	defer func() {
		if err != nil && !held {
			if e := runtime.Resume(context.WithoutCancel(ctx)); e != nil {
				err = errors.Join(err, e)
				runtime.HoldRecovery(err)
			}
		}
	}()
	fresh, err := CaptureMigrationRoot(ctx, r.Layout, r.TrackedSources)
	if err != nil {
		return out, err
	}
	if fresh.Digest != r.Digest {
		return out, fmt.Errorf("approved root source changed; new complete review required")
	}
	backup, err := os.MkdirTemp(filepath.Dir(r.Layout.DBPath), ".stellar-migration-backup-")
	if err != nil {
		return out, err
	}
	// 一旦前像已归档便保留任务目录，即使演练失败也不删除审查/恢复证据。
	if err = writePrivateDurable(filepath.Join(backup, "root-capture.private.json"), r); err != nil {
		return out, err
	}
	if err = writePrivateDurable(filepath.Join(backup, "approval.private.json"), struct {
		Approval  RootMigrationApproval
		Notebook  NotebookStructureDecisions
		Candidate Candidate
	}{a, decisions, c}); err != nil {
		return out, err
	}
	rehearsal, err := buildRootRehearsal(ctx, r, c, a, decisions, backup)
	if err != nil {
		return out, err
	}
	if err = runtime.ValidateStaged(ctx, rehearsal); err != nil {
		return out, err
	}
	// ValidateStaged必须关完连接；不得只复制主DB而丢弃尚未checkpoint的staged WAL。
	if info, e := os.Lstat(rehearsal.Layout.DBPath + "-wal"); e == nil && (!info.Mode().IsRegular() || info.Size() != 0) {
		return out, fmt.Errorf("staged database has a pending WAL; validator must close/checkpoint all connections")
	} else if e != nil && !os.IsNotExist(e) {
		return out, e
	}
	targets := []string{r.Layout.DBPath, r.Layout.DBPath + "-wal", r.Layout.DBPath + "-journal"}
	for _, source := range r.TrackedSources {
		targets = append(targets, filepath.Join(r.Layout.Root, source))
	}
	// 最后一次校验忽略且仅忽略本次私密任务目录及维护journal/lock。
	fresh, err = captureMigrationRoot(ctx, r.Layout, r.TrackedSources, backup)
	if err != nil {
		return out, err
	}
	if fresh.Digest != r.Digest {
		return out, fmt.Errorf("root resources changed during staging")
	}
	j := rootJournal{Version: 1, Layout: r.Layout, BackupDirectory: backup, RootDigest: r.Digest, Targets: targets, State: "installing"}
	if err = writePrivateDurable(rootJournalPath(r.Layout), j); err != nil {
		return out, err
	}
	installed := true
	rollback := func(cause error) error {
		held = true
		if e := runtime.Quiesce(context.WithoutCancel(ctx)); e != nil {
			cause = errors.Join(cause, e)
			runtime.HoldRecovery(cause)
			return cause
		}
		if e := restoreRootTargets(j, r); e != nil {
			cause = errors.Join(cause, e)
			runtime.HoldRecovery(cause)
			return cause
		}
		j.State = "rolled_back"
		if e := writePrivateDurable(rootJournalPath(r.Layout), j); e != nil {
			cause = errors.Join(cause, e)
			runtime.HoldRecovery(cause)
			return cause
		}
		if e := removeRootJournal(r.Layout); e != nil {
			cause = errors.Join(cause, e)
			runtime.HoldRecovery(cause)
			return cause
		}
		if e := runtime.Resume(context.WithoutCancel(ctx)); e != nil {
			cause = errors.Join(cause, e)
			runtime.HoldRecovery(cause)
			return cause
		}
		return cause
	}
	defer func() {
		if err != nil && installed {
			err = rollback(err)
		}
	}()
	for _, target := range targets {
		if err = ctx.Err(); err != nil {
			return out, err
		}
		if hook != nil {
			if err = hook(target); err != nil {
				return out, err
			}
		}
		if target == r.Layout.DBPath+"-wal" || target == r.Layout.DBPath+"-journal" {
			err = removeRootTarget(target)
		} else {
			virtual, e := resourceVirtualPath(r.Layout, target)
			if e != nil {
				return out, e
			}
			data, e := os.ReadFile(filepath.Join(rehearsal.Directory, filepath.FromSlash(virtual)))
			if e != nil {
				return out, e
			}
			mode := os.FileMode(0600)
			for _, resource := range r.Resources {
				if resource.Path == target && resource.Kind == "file" {
					mode = os.FileMode(resource.Mode)
				}
			}
			err = writeRootAtomic(target, data, mode)
		}
		if err != nil {
			return out, err
		}
	}
	// 此DB的SHM属于已关闭的连接；不删任何操作锁或其它DB侧车。
	if err = removeRootTarget(r.Layout.DBPath + "-shm"); err != nil {
		return out, err
	}
	if err = runtime.Resume(ctx); err != nil {
		return out, err
	}
	j.State = "complete"
	if err = writePrivateDurable(rootJournalPath(r.Layout), j); err != nil {
		return out, err
	}
	installed = false
	if err = removeRootJournal(r.Layout); err != nil {
		held = true
		runtime.HoldRecovery(err)
		return out, err
	}
	out = RootApplyResult{BackupDirectory: backup, RootDigest: r.Digest, ResolvedCandidateDigest: rehearsal.ResolvedCandidateDigest}
	return out, nil
}

// RecoverRootMigration必须在任何普通Store/worker/服务启动之前调用，并复用维护互斥锁。
// 未完成安装一律恢复前像；complete journal只收尾，不回滚已经运行的新代数据。
func RecoverRootMigration(ctx context.Context, layout RootLayout) (RootRecoveryResult, error) {
	layout, err := normalizeRootLayout(layout)
	if err != nil {
		return RootRecoveryResult{}, err
	}
	if _, err = os.Lstat(rootJournalPath(layout)); os.IsNotExist(err) {
		return RootRecoveryResult{}, nil
	} else if err != nil {
		return RootRecoveryResult{}, err
	}
	if err = noSymlinkPath(filepath.Dir(layout.OperationLock)); err != nil {
		return RootRecoveryResult{}, err
	}
	unlock, err := maintenance.LockOperation(layout.OperationLock)
	if err != nil {
		return RootRecoveryResult{}, err
	}
	defer unlock()
	if err = ctx.Err(); err != nil {
		return RootRecoveryResult{}, err
	}
	var j rootJournal
	if err = readPrivateRegular(rootJournalPath(layout), &j); err != nil {
		return RootRecoveryResult{}, err
	}
	if j.Version != 1 || !reflect.DeepEqual(j.Layout, layout) || !strings.HasPrefix(filepath.Base(j.BackupDirectory), ".stellar-migration-backup-") || filepath.Dir(j.BackupDirectory) != filepath.Dir(layout.DBPath) {
		return RootRecoveryResult{}, fmt.Errorf("invalid migration recovery identity")
	}
	if err = noSymlinkPath(j.BackupDirectory); err != nil {
		return RootRecoveryResult{}, err
	}
	var r RootCapture
	if err = readPrivateRegular(filepath.Join(j.BackupDirectory, "root-capture.private.json"), &r); err != nil {
		return RootRecoveryResult{}, err
	}
	if err = r.Verify(); err != nil {
		return RootRecoveryResult{}, err
	}
	expected := []string{layout.DBPath, layout.DBPath + "-wal", layout.DBPath + "-journal"}
	for _, s := range r.TrackedSources {
		if !filepath.IsLocal(s) || !strings.HasPrefix(filepath.ToSlash(s), "content/") {
			return RootRecoveryResult{}, fmt.Errorf("invalid recovery source")
		}
		expected = append(expected, filepath.Join(layout.Root, s))
	}
	if !reflect.DeepEqual(r.Layout, layout) || r.Digest != j.RootDigest || !reflect.DeepEqual(j.Targets, expected) {
		return RootRecoveryResult{}, fmt.Errorf("migration recovery targets changed")
	}
	if j.State != "complete" && j.State != "installing" && j.State != "rolled_back" {
		return RootRecoveryResult{}, fmt.Errorf("unknown migration recovery state")
	}
	if j.State != "complete" {
		if err = restoreRootTargets(j, r); err != nil {
			return RootRecoveryResult{}, err
		}
	}
	if err = removeRootJournal(layout); err != nil {
		return RootRecoveryResult{}, err
	}
	return RootRecoveryResult{Recovered: true, BackupDirectory: j.BackupDirectory}, nil
}
func restoreRootTargets(j rootJournal, r RootCapture) error {
	byPath := map[string]RootResource{}
	for _, resource := range r.Resources {
		byPath[resource.Path] = resource
	}
	for _, target := range j.Targets {
		resource, ok := byPath[target]
		if !ok || resource.Kind == "absent" {
			if target != r.Layout.DBPath+"-wal" && target != r.Layout.DBPath+"-journal" {
				return fmt.Errorf("missing recovery target preimage")
			}
			if err := removeRootTarget(target); err != nil {
				return err
			}
			continue
		}
		if resource.Kind != "file" {
			return fmt.Errorf("recovery target is not an ordinary file")
		}
		if err := writeRootAtomic(target, resource.Data, os.FileMode(resource.Mode)); err != nil {
			return err
		}
	}
	return removeRootTarget(r.Layout.DBPath + "-shm")
}
func syncRootDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}
func removeRootTarget(path string) error {
	if err := noSymlinkPath(filepath.Dir(path)); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("migration target is not an ordinary file")
	}
	if err = os.Remove(path); err != nil {
		return err
	}
	return syncRootDirectory(filepath.Dir(path))
}
func removeRootJournal(layout RootLayout) error { return removeRootTarget(rootJournalPath(layout)) }
func writeRootAtomic(path string, data []byte, mode os.FileMode) error {
	if err := noSymlinkPath(filepath.Dir(path)); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("migration target is not an ordinary file")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".stellar-migration-write-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(mode.Perm()); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = errors.Join(f.Sync(), f.Close()); err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	return syncRootDirectory(filepath.Dir(path))
}
func writePrivateDurable(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeRootAtomic(path, append(data, '\n'), 0600)
}
func readPrivateRegular(path string, v any) error {
	if err := noSymlinkPath(filepath.Dir(path)); err != nil {
		return err
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("migration archive must be a private ordinary file")
	}
	decoder := json.NewDecoder(f)
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(v); err != nil {
		return err
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("migration archive has trailing content")
	}
	return nil
}
