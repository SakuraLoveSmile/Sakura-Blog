package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"blog/internal/store"
)

func recoveryFixture(t *testing.T) (*Service, *store.Store, []byte) {
	t.Helper()
	svc, st, dir := newTestService(t)
	svc.sitePath = filepath.Join(dir, "site.yml")
	if err := os.WriteFile(svc.sitePath, []byte("title: backup\n"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := st.Upsert(&store.Post{Slug: "state", Title: "backup", ContentMD: "backup body", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(svc.uploadDir, "state.png"), []byte("backup"), 0640); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	if err := svc.WriteZip(&archive); err != nil {
		t.Fatal(err)
	}
	if err := st.Upsert(&store.Post{Slug: "state", Title: "live", ContentMD: "live body", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(svc.uploadDir, "state.png"), []byte("live"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(svc.uploadDir, "extra.png"), []byte("extra"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(svc.sitePath, []byte("title: live\n"), 0640); err != nil {
		t.Fatal(err)
	}
	return svc, st, archive.Bytes()
}
func assertRestoredState(t *testing.T, svc *Service, st *store.Store, want string) {
	t.Helper()
	post, err := st.GetBySlug("state")
	if err != nil || post == nil || post.Title != want {
		t.Fatalf("post state=%v err=%v", post, err)
	}
	image, err := os.ReadFile(filepath.Join(svc.uploadDir, "state.png"))
	if err != nil || string(image) != want {
		t.Fatalf("upload=%q err=%v", image, err)
	}
	site, err := os.ReadFile(svc.sitePath)
	if err != nil || string(site) != "title: "+want+"\n" {
		t.Fatalf("site=%q err=%v", site, err)
	}
	info, err := os.Stat(svc.sitePath)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatalf("configuration permissions changed: %v", err)
	}
}
func TestRestoreEveryCommitFailureCompensatesAllState(t *testing.T) {
	for _, step := range []string{"preimage", "database", "uploads", "site", "rebuilding"} {
		t.Run(step, func(t *testing.T) {
			svc, st, data := recoveryFixture(t)
			svc.restoreStep = func(actual string) error {
				if actual == step {
					return errors.New("injected failure")
				}
				return nil
			}
			if err := svc.Restore(data, true); err == nil {
				t.Fatal("fault did not fail restore")
			}
			assertRestoredState(t, svc, st, "live")
			if st.Maintenance().Active() {
				t.Fatal("successful compensation did not reopen service")
			}
			tasks, err := svc.ListRestoreTasks()
			if err != nil || len(tasks) != 1 || tasks[0].Status != RestoreRolledBack {
				t.Fatalf("task state=%v err=%v", tasks, err)
			}
		})
	}
}
func TestRestoreRebuildFailureRebuildsPreimageBeforeOpening(t *testing.T) {
	svc, st, data := recoveryFixture(t)
	calls := 0
	svc.SetRestoreRebuild(func() error {
		calls++
		if calls == 1 {
			return errors.New("new runtime failed")
		}
		return nil
	})
	if err := svc.Restore(data, true); err == nil {
		t.Fatal("expected runtime failure")
	}
	if calls != 2 {
		t.Fatalf("expected candidate and preimage rebuild, got %d", calls)
	}
	assertRestoredState(t, svc, st, "live")
	if st.Maintenance().Active() {
		t.Fatal("service remains unavailable after successful compensation")
	}
}
func TestRestoreFailedCompensationStaysClosedAndCanRecover(t *testing.T) {
	svc, st, data := recoveryFixture(t)
	svc.SetRestoreRebuild(func() error { return errors.New("runtime unavailable") })
	if err := svc.Restore(data, true); !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("want recovery required, got %v", err)
	}
	if !st.Maintenance().Active() {
		t.Fatal("mixed/unknown runtime exposed")
	}
	tasks, err := svc.ListRestoreTasks()
	if err != nil || len(tasks) != 1 {
		t.Fatal(err)
	}
	if !tasks[0].HasPreimage {
		t.Fatal("lost compensation preimage")
	}
	svc.SetRestoreRebuild(nil)
	if err := svc.RecoverPending(); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecoverPending(); err != nil {
		t.Fatal("compensation was not idempotent", err)
	}
	assertRestoredState(t, svc, st, "live")
}
func TestRestoreTerminalRenameUncertaintyDoesNotRollback(t *testing.T) {
	svc, st, data := recoveryFixture(t)
	terminalWrites := 0
	svc.restoreJournalWrite = func(path string, data []byte) error {
		var journal restoreJournal
		if err := json.Unmarshal(data, &journal); err != nil {
			return err
		}
		if terminalWrites > 0 {
			return errors.New("disk unavailable after terminal rename")
		}
		if journal.Status == RestoreSucceeded {
			terminalWrites++
			// 实际写入和 rename 已发生，模拟目录 fsync 返回失败。
			if err := writeAtomicFile(path, data, 0600); err != nil {
				return err
			}
			return errors.New("terminal directory fsync failed")
		}
		return writeAtomicFile(path, data, 0600)
	}
	if err := svc.Restore(data, true); !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("uncertain terminal result=%v", err)
	}
	if !st.Maintenance().Active() {
		t.Fatal("uncertain durability reopened service")
	}
	assertRestoredState(t, svc, st, "backup")
	if terminalWrites != 1 {
		t.Fatalf("unexpected rollback journal writes: %d", terminalWrites)
	}
	// 重启使用实际终态；成功已落盘则保留完整新状态，不尝试半次补偿。
	svc.restoreJournalWrite = nil
	if err := svc.RecoverPending(); err != nil {
		t.Fatal(err)
	}
	assertRestoredState(t, svc, st, "backup")
}
func TestRestoreIdempotencySurvivesServiceRecreationAndIncludesSource(t *testing.T) {
	svc, _, data := recoveryFixture(t)
	input := RestoreInput{Data: data, IdempotencyKey: "same-request"}
	task, err := svc.StartRestore(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.WaitRestore(context.Background(), task.ID); err != nil {
		t.Fatal(err)
	}
	reopened := New(Config{Store: svc.store, BackupDir: svc.backupDir, UploadDir: svc.uploadDir, SitePath: svc.sitePath})
	replay, err := reopened.StartRestore(context.Background(), input)
	if err != nil || replay.ID != task.ID {
		t.Fatalf("replayed task %v err=%v", replay, err)
	}
	input.Strict = true
	if _, err := reopened.StartRestore(context.Background(), input); !errors.Is(err, ErrRestoreConflict) {
		t.Fatalf("strict mismatch=%v", err)
	}
	snapshot := filepath.Join(svc.backupDir, "input.zip")
	if err := os.WriteFile(snapshot, data, 0600); err != nil {
		t.Fatal(err)
	}
	source := RestoreInput{Snapshot: "input.zip", IdempotencyKey: "snapshot-request"}
	task, err = reopened.StartRestore(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.WaitRestore(context.Background(), task.ID); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshot, append(data, 'x'), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.StartRestore(context.Background(), source); !errors.Is(err, ErrRestoreConflict) {
		t.Fatalf("snapshot content mismatch=%v", err)
	}
}
func TestRestoreRetainsThreeSuccessfulCompletePreimages(t *testing.T) {
	svc, _, data := recoveryFixture(t)
	for i := 0; i < 4; i++ {
		if err := svc.Restore(data, false); err != nil {
			t.Fatal(err)
		}
	}
	tasks, err := svc.ListRestoreTasks()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, task := range tasks {
		if task.HasPreimage {
			count++
			for _, name := range []string{"blog.db", "uploads", "site.yml"} {
				if _, err := os.Stat(svc.beforePath(task.ID, name)); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if count != 3 {
		t.Fatalf("retained %d preimages", count)
	}
}

// 子进程真实退出覆盖持久化阶段边界；不将进程退出测试声称为断电模拟。
func TestRestoreCrashChild(t *testing.T) {
	dir := os.Getenv("BLOG_RESTORE_CRASH_DIR")
	if dir == "" {
		return
	}
	st, err := store.OpenForRecovery(filepath.Join(dir, "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	svc := New(Config{Store: st, BackupDir: filepath.Join(dir, "backups"), UploadDir: filepath.Join(dir, "uploads"), SitePath: filepath.Join(dir, "site.yml")})
	data, err := os.ReadFile(filepath.Join(dir, "source.zip"))
	if err != nil {
		t.Fatal(err)
	}
	phase := os.Getenv("BLOG_RESTORE_CRASH_PHASE")
	svc.restoreStep = func(step string) error {
		if step == phase {
			if phase == "site" {
				if err := os.WriteFile(svc.sitePath, []byte("title: [truncated"), 0640); err != nil {
					t.Fatal(err)
				}
			}
			os.Exit(72)
		}
		return nil
	}
	if err := svc.Restore(data, true); err != nil {
		t.Fatal(err)
	}
	t.Fatal("child did not crash at checkpoint")
}
func TestRestoreProcessInterruptionCompensatesOnStartup(t *testing.T) {
	for _, phase := range []string{"preimage", "database", "uploads", "site", "rebuilding", "completed"} {
		t.Run(phase, func(t *testing.T) {
			svc, st, data := recoveryFixture(t)
			dir := filepath.Dir(st.DBPath())
			if err := os.WriteFile(filepath.Join(dir, "source.zip"), data, 0600); err != nil {
				t.Fatal(err)
			}
			child := exec.Command(os.Args[0], "-test.run=^TestRestoreCrashChild$")
			child.Env = append(os.Environ(), "BLOG_RESTORE_CRASH_DIR="+dir, "BLOG_RESTORE_CRASH_PHASE="+phase)
			output, err := child.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 72 {
				t.Fatalf("child failed unexpectedly: %v %s", err, output)
			}
			if err := svc.RecoverPending(); err != nil {
				t.Fatal(err)
			}
			if err := svc.RecoverPending(); err != nil {
				t.Fatal(err)
			}
			want := "live"
			if phase == "completed" {
				want = "backup"
			}
			assertRestoredState(t, svc, st, want)
		})
	}
}
func TestRestoreCorruptJournalFailsClosed(t *testing.T) {
	svc, _, _ := recoveryFixture(t)
	if err := os.MkdirAll(svc.backupDir, 0700); err != nil {
		t.Fatal(err)
	}
	id, err := newRestoreID()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(svc.journalPath(id), []byte(`{"version":99}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecoverPending(); err == nil {
		t.Fatal("unsupported journal accepted")
	}
	if _, err := svc.StartRestore(context.Background(), RestoreInput{Data: []byte("bad")}); err == nil {
		t.Fatal("restore allowed over unknown journal")
	}
}
func TestRestorePreflightPreservesCurrentData(t *testing.T) {
	svc, st, data := recoveryFixture(t)
	if err := os.Chmod(svc.sitePath, 0400); err != nil {
		t.Fatal(err)
	}
	if err := svc.Restore(data, true); err == nil || !strings.Contains(err.Error(), "writable") {
		t.Fatalf("unwritable configuration accepted: %v", err)
	}
	if err := os.Chmod(svc.sitePath, 0640); err != nil {
		t.Fatal(err)
	}
	assertRestoredState(t, svc, st, "live")
	if err := checkFreeSpace(filepath.Dir(svc.sitePath), 1<<62); err == nil {
		t.Fatal(fmt.Errorf("insufficient space accepted"))
	}
}

func TestRestoreUploadsOnDifferentFilesystem(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("cross-filesystem fixture uses Linux /dev/shm")
	}
	svc, st, data := recoveryFixture(t)
	alternate, err := os.MkdirTemp("/dev/shm", "blog-restore-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(alternate)
	target := filepath.Join(alternate, "uploads")
	if err := copyTree(svc.uploadDir, target); err != nil {
		t.Fatal(err)
	}
	svc.uploadDir = target
	if err := svc.Restore(data, true); err != nil {
		t.Fatal(err)
	}
	assertRestoredState(t, svc, st, "backup")
}

func TestRestoreRejectsCaseInsensitiveTargetsBeforeMutation(t *testing.T) {
	for _, names := range [][]string{{"uploads/Photo.png", "uploads/photo.png"}, {"uploads/A/one.png", "uploads/a/two.png"}} {
		t.Run(names[0], func(t *testing.T) {
			svc, st, data := recoveryFixture(t)
			reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatal(err)
			}
			var archive bytes.Buffer
			writer := zip.NewWriter(&archive)
			for _, entry := range reader.File {
				source, err := entry.Open()
				if err != nil {
					t.Fatal(err)
				}
				target, err := writer.CreateHeader(&entry.FileHeader)
				if err != nil {
					t.Fatal(err)
				}
				_, err = io.Copy(target, source)
				source.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range names {
				target, err := writer.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := target.Write([]byte("image")); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			if err := svc.Restore(archive.Bytes(), true); err == nil || !strings.Contains(err.Error(), "case-insensitive target conflict") {
				t.Fatalf("expected conflict rejection, got %v", err)
			}
			assertRestoredState(t, svc, st, "live")
			if st.Maintenance().Active() {
				t.Fatal("preflight failure left maintenance active")
			}
		})
	}
}
