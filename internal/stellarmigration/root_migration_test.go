package stellarmigration

import (
	"blog/internal/maintenance"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func completeRootSource(t *testing.T) (RootCapture, Candidate, RootMigrationApproval) {
	t.Helper()
	path, s := completeMigrationSource(t)
	root := t.TempDir()
	for _, name := range []string{"config", "content/posts", "content/_data/caches", "data/uploads", "data/backups", "plugins", "runtime", "cache"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"data/blog.db": data, "config/app.yml": []byte("root: .\ndb_path: data/blog.db\nupload_dir: data/uploads\nseed_on_start: false\nauto_backup: false\n"), "config/site.yml": []byte("title: fixture\n"), "content/posts/tracked.md": []byte(s.Files["content/posts/tracked.md"]), "content/untouched.md": []byte("untouched opaque content\n"), "data/uploads/picture.bin": {0, 1, 255}, "data/backups/old.bin": []byte("old backup"), "plugins/plugin.yml": []byte("fixture"), "runtime/state.json": []byte("{}"), "cache/kept.bin": []byte("cache"), "content/_data/caches/images_metadata.json": []byte("{}")}
	for name, data := range files {
		if err = os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	db, err := sql.Open("sqlite", filepath.Join(root, "data/blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE INDEX fixture_post_index ON posts(slug);CREATE VIEW fixture_posts AS SELECT id,slug,status FROM posts;CREATE TABLE fixture_audit(n INTEGER);INSERT INTO fixture_audit VALUES(0);CREATE TRIGGER fixture_post_trigger AFTER UPDATE OF status ON posts BEGIN UPDATE fixture_audit SET n=n+1;END;PRAGMA user_version=17;PRAGMA application_id=23;`)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// WAL模式首次只读打开可建立空WAL/SHM；先单独只读快照，再捕获稳定完整根。
	_ = snapshot(t, filepath.Join(root, "data/blog.db"), nil)
	r, err := CaptureMigrationRoot(context.Background(), RootLayout{Root: root}, []string{"content/posts/tracked.md"})
	if err != nil {
		t.Fatal(err)
	}
	c := candidate(t, r.Source)
	a := RootMigrationApproval{MigrationApproval: approveFixture(c), RootDigest: r.Digest}
	a.Resolutions = []MigrationResolution{{Source: "content/posts/tracked.md", Code: "file_host_fields", Path: "status", Action: "preserve_status"}}
	return r, c, a
}
func TestFullRootRehearsalPreservesResourcesAndDatabaseObjects(t *testing.T) {
	r, c, a := completeRootSource(t)
	f, err := BuildRootRehearsal(context.Background(), r, c, a, NotebookStructureDecisions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(f.Directory) })
	for _, resource := range r.Resources {
		if resource.Kind != "file" || resource.Path == r.Layout.DBPath || resource.Path == r.Layout.DBPath+"-wal" || resource.Path == r.Layout.DBPath+"-journal" || resource.Path == r.Layout.AppPath || resource.Path == filepath.Join(r.Layout.Root, r.TrackedSources[0]) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(f.Directory, filepath.FromSlash(resource.VirtualPath)))
		if err != nil || !reflect.DeepEqual(data, resource.Data) {
			t.Fatalf("resource not preserved: %s", resource.VirtualPath)
		}
	}
	db, err := sql.Open("sqlite", f.Layout.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for query, want := range map[string]int{"PRAGMA user_version": 17, "PRAGMA application_id": 23, "SELECT n FROM fixture_audit": 0, "SELECT count(*) FROM fixture_posts": 1} {
		var n int
		if err = db.QueryRow(query).Scan(&n); err != nil || n != want {
			t.Fatalf("%s got %d want %d: %v", query, n, want, err)
		}
	}
	if _, err = db.Exec("UPDATE posts SET status=status"); err != nil {
		t.Fatal(err)
	}
	var n int
	db.QueryRow("SELECT n FROM fixture_audit").Scan(&n)
	if n != 1 {
		t.Fatal("original trigger did not run")
	}
	after, err := CaptureMigrationRoot(context.Background(), r.Layout, r.TrackedSources)
	if err != nil || after.Digest != r.Digest {
		t.Fatal("source changed during rehearsal")
	}
}

type rootRuntime struct {
	events                 []string
	validateErr, resumeErr error
	held                   bool
}

func (r *rootRuntime) Quiesce(context.Context) error {
	r.events = append(r.events, "quiesce")
	return nil
}
func (r *rootRuntime) ValidateStaged(context.Context, RootRehearsal) error {
	r.events = append(r.events, "validate")
	return r.validateErr
}
func (r *rootRuntime) Resume(context.Context) error {
	r.events = append(r.events, "resume")
	return r.resumeErr
}
func (r *rootRuntime) HoldRecovery(error) { r.events = append(r.events, "hold"); r.held = true }
func TestRootApplyAndRollbackUseCompleteApprovalAndKeepArchive(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "mid_install_rollback"}[failure], func(t *testing.T) {
			r, c, a := completeRootSource(t)
			runtime := &rootRuntime{}
			fail := errors.New("injected file failure")
			hook := func(path string) error {
				if failure && path == filepath.Join(r.Layout.Root, r.TrackedSources[0]) {
					return fail
				}
				return nil
			}
			result, err := applyReviewedRootMigration(context.Background(), r, c, a, NotebookStructureDecisions{}, runtime, hook)
			if failure {
				if !errors.Is(err, fail) {
					t.Fatal(err)
				}
				backups, _ := filepath.Glob(filepath.Join(filepath.Dir(r.Layout.DBPath), ".stellar-migration-backup-*"))
				if len(backups) != 1 {
					t.Fatal("rollback archive missing")
				}
				after, err := captureMigrationRoot(context.Background(), r.Layout, r.TrackedSources, backups[0])
				if err != nil || after.Digest != r.Digest {
					t.Fatal("rollback did not restore complete original source")
				}
				if runtime.held {
					t.Fatal("successful rollback held runtime")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if result.BackupDirectory == "" {
					t.Fatal("complete backup missing")
				}
				var archive RootCapture
				if err = readPrivateRegular(filepath.Join(result.BackupDirectory, "root-capture.private.json"), &archive); err != nil || archive.Digest != r.Digest {
					t.Fatal("backup does not contain complete original root")
				}
				db, err := sql.Open("sqlite", r.Layout.DBPath)
				if err != nil {
					t.Fatal(err)
				}
				var version int
				err = db.QueryRow("SELECT stellar_version FROM posts").Scan(&version)
				db.Close()
				if err != nil || version != 2 {
					t.Fatal("new canonical database not installed")
				}
			}
			if _, err = os.Stat(rootJournalPath(r.Layout)); !os.IsNotExist(err) {
				t.Fatal("completed journal left behind")
			}
			if len(runtime.events) < 3 || runtime.events[0] != "quiesce" || runtime.events[1] != "validate" {
				t.Fatal(runtime.events)
			}
		})
	}
}
func TestRootMigrationRejectsStaleMissingApprovalAndUnsafeResources(t *testing.T) {
	r, c, a := completeRootSource(t)
	if _, err := ApplyReviewedRootMigration(context.Background(), r, c, a, NotebookStructureDecisions{}, nil); err == nil {
		t.Fatal("missing maintenance gate accepted")
	}
	a.RootDigest = "wrong"
	runtime := &rootRuntime{}
	if _, err := ApplyReviewedRootMigration(context.Background(), r, c, a, NotebookStructureDecisions{}, runtime); err == nil || len(runtime.events) != 0 {
		t.Fatal("unapproved root stopped runtime")
	}
	a.RootDigest = r.Digest
	os.WriteFile(filepath.Join(r.Layout.UploadDir, "new.bin"), []byte("new asset"), 0600)
	if _, err := ApplyReviewedRootMigration(context.Background(), r, c, a, NotebookStructureDecisions{}, runtime); err == nil {
		t.Fatal("new upload did not invalidate approval")
	}
	if _, err := os.Stat(filepath.Join(r.Layout.UploadDir, "new.bin")); err != nil {
		t.Fatal("stale validation deleted upload")
	}
	if err := os.Symlink(r.Layout.DBPath, filepath.Join(r.Layout.Root, "content/unsafe.db")); err != nil {
		t.Fatal(err)
	}
	if _, err := CaptureMigrationRoot(context.Background(), r.Layout, r.TrackedSources); err == nil {
		t.Fatal("symlink accepted")
	}
}
func TestRootRecoveryRestoresInterruptedGenerationAndRejectsTamperedTargets(t *testing.T) {
	r, _, _ := completeRootSource(t)
	backup, err := os.MkdirTemp(filepath.Dir(r.Layout.DBPath), ".stellar-migration-backup-")
	if err != nil {
		t.Fatal(err)
	}
	if err = writePrivateDurable(filepath.Join(backup, "root-capture.private.json"), r); err != nil {
		t.Fatal(err)
	}
	j := rootJournal{Version: 1, Layout: r.Layout, BackupDirectory: backup, RootDigest: r.Digest, Targets: []string{r.Layout.DBPath, r.Layout.DBPath + "-wal", r.Layout.DBPath + "-journal", filepath.Join(r.Layout.Root, r.TrackedSources[0])}, State: "installing"}
	original := append([]string{}, j.Targets...)
	j.Targets[3] = filepath.Join(r.Layout.Root, "config/app.yml")
	if err = writePrivateDurable(rootJournalPath(r.Layout), j); err != nil {
		t.Fatal(err)
	}
	if _, err = RecoverRootMigration(context.Background(), r.Layout); err == nil {
		t.Fatal("tampered recovery target accepted")
	}
	j.Targets = original
	os.WriteFile(r.Layout.DBPath, []byte("interrupted new DB"), 0600)
	os.WriteFile(j.Targets[3], []byte("new document"), 0600)
	if err = writePrivateDurable(rootJournalPath(r.Layout), j); err != nil {
		t.Fatal(err)
	}
	recovered, err := RecoverRootMigration(context.Background(), r.Layout)
	if err != nil || !recovered.Recovered {
		t.Fatal(err)
	}
	after, err := captureMigrationRoot(context.Background(), r.Layout, r.TrackedSources, backup)
	if err != nil || after.Digest != r.Digest {
		t.Fatal("startup recovery did not restore original generation")
	}
}

func TestRootMaintenanceLockCancellationAndFailedRecovery(t *testing.T) {
	t.Run("operation lock", func(t *testing.T) {
		r, c, a := completeRootSource(t)
		unlock, err := maintenance.LockOperation(r.Layout.OperationLock)
		if err != nil {
			t.Fatal(err)
		}
		defer unlock()
		runtime := &rootRuntime{}
		if _, err = ApplyReviewedRootMigration(context.Background(), r, c, a, NotebookStructureDecisions{}, runtime); !errors.Is(err, maintenance.ErrBusy) || len(runtime.events) != 0 {
			t.Fatal("busy lock did not reject before quiesce")
		}
		if _, err = os.Stat(r.Layout.OperationLock); err != nil {
			t.Fatal("existing lock removed")
		}
	})
	t.Run("cancellation", func(t *testing.T) {
		r, c, a := completeRootSource(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		runtime := &rootRuntime{}
		_, err := applyReviewedRootMigration(ctx, r, c, a, NotebookStructureDecisions{}, runtime, func(path string) error {
			if path == r.Layout.DBPath+"-wal" {
				cancel()
			}
			return nil
		})
		if !errors.Is(err, context.Canceled) || runtime.held {
			t.Fatal(err)
		}
		backups, _ := filepath.Glob(filepath.Join(filepath.Dir(r.Layout.DBPath), ".stellar-migration-backup-*"))
		after, e := captureMigrationRoot(context.Background(), r.Layout, r.TrackedSources, backups[0])
		if e != nil || after.Digest != r.Digest {
			t.Fatal("cancelled operation did not compensate")
		}
	})
	t.Run("unsafe rollback held then recovered", func(t *testing.T) {
		r, c, a := completeRootSource(t)
		target := filepath.Join(r.Layout.Root, r.TrackedSources[0])
		runtime := &rootRuntime{}
		fail := errors.New("injected unsafe target")
		_, err := applyReviewedRootMigration(context.Background(), r, c, a, NotebookStructureDecisions{}, runtime, func(path string) error {
			if path == target {
				if e := os.Remove(target); e != nil {
					return e
				}
				if e := os.Symlink(r.Layout.AppPath, target); e != nil {
					return e
				}
				return fail
			}
			return nil
		})
		if err == nil || !runtime.held {
			t.Fatal("unsafe rollback did not hold runtime")
		}
		if _, err = os.Stat(rootJournalPath(r.Layout)); err != nil {
			t.Fatal("failed compensation lost journal")
		}
		if _, err = RecoverRootMigration(context.Background(), r.Layout); err == nil {
			t.Fatal("recovery followed symlink")
		}
		if err = os.Remove(target); err != nil {
			t.Fatal(err)
		}
		if _, err = RecoverRootMigration(context.Background(), r.Layout); err != nil {
			t.Fatal(err)
		}
	})
}
func TestCapturedSchemaStatementsRejectExternalWrites(t *testing.T) {
	for _, object := range []DatabaseObject{{Kind: "index", SQL: "CREATE INDEX fixture ON posts(slug); ATTACH DATABASE '/tmp/forbidden.db' AS other"}, {Kind: "view", SQL: "CREATE VIEW fixture AS SELECT 1; VACUUM INTO '/tmp/forbidden.db'"}, {Kind: "trigger", SQL: "CREATE TRIGGER fixture AFTER UPDATE ON posts BEGIN SELECT 1; END; ATTACH DATABASE '/tmp/forbidden.db' AS other"}} {
		if err := safeDatabaseObjectStatement(object); err == nil {
			t.Fatal("multi-statement schema accepted")
		}
	}
	if err := safeDatabaseObjectStatement(DatabaseObject{Kind: "trigger", SQL: "CREATE TRIGGER fixture AFTER UPDATE ON posts BEGIN SELECT CASE WHEN NEW.status='END; BEGIN' THEN 1 ELSE 0 END; END; -- audit\n"}); err != nil {
		t.Fatal(err)
	}
}
func TestRootCaptureExternalResourcesAndPrivateModes(t *testing.T) {
	r, _, _ := completeRootSource(t)
	external := t.TempDir()
	upload := filepath.Join(external, "uploads")
	if err := os.Mkdir(upload, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(upload, "asset"), []byte("external upload"), 0640); err != nil {
		t.Fatal(err)
	}
	layout := r.Layout
	layout.UploadDir = upload
	layout.AppPath = filepath.Join(external, "app.yml")
	app, err := os.ReadFile(r.Layout.AppPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(layout.AppPath, app, 0600); err != nil {
		t.Fatal(err)
	}
	captured, err := CaptureMigrationRoot(context.Background(), layout, r.TrackedSources)
	if err != nil {
		t.Fatal(err)
	}
	c := candidate(t, captured.Source)
	a := RootMigrationApproval{MigrationApproval: approveFixture(c), RootDigest: captured.Digest}
	a.Resolutions = []MigrationResolution{{Source: "content/posts/tracked.md", Code: "file_host_fields", Path: "status", Action: "preserve_status"}}
	f, err := BuildRootRehearsal(context.Background(), captured, c, a, NotebookStructureDecisions{})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(f.Directory)
	info, err := os.Stat(f.Layout.UploadDir)
	if err != nil || info.Mode().Perm() != 0750 {
		t.Fatal("external directory mode lost")
	}
	info, err = os.Stat(filepath.Join(f.Layout.UploadDir, "asset"))
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatal("external file mode lost")
	}
	for _, path := range []string{f.Directory, filepath.Join(f.Directory, "root-capture.private.json"), filepath.Join(f.Directory, "approval.private.json")} {
		info, err = os.Stat(path)
		if err != nil || info.Mode().Perm()&0077 != 0 {
			t.Fatal("private archive exposed")
		}
	}
}
