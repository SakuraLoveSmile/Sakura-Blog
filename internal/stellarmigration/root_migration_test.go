package stellarmigration

import (
	"blog/internal/maintenance"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
	_, err = db.Exec(`CREATE INDEX fixture_post_index ON posts(slug);CREATE INDEX fixture_partial_index ON posts(status DESC,slug COLLATE NOCASE) WHERE status='published';CREATE VIEW fixture_posts AS SELECT id,slug,status FROM posts;CREATE TABLE fixture_audit(n INTEGER);INSERT INTO fixture_audit VALUES(0);CREATE TRIGGER fixture_post_trigger AFTER UPDATE OF status ON posts BEGIN UPDATE fixture_audit SET n=n+1;END;PRAGMA user_version=17;PRAGMA application_id=23;`)
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
	a.ReviewDigest, err = RootReviewDigest(r, c, a.Resolutions, NotebookStructureDecisions{})
	if err != nil {
		t.Fatal(err)
	}
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
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	index, err := readDatabaseIndex(context.Background(), tx, "fixture_partial_index", "posts")
	tx.Rollback()
	if err != nil || !index.Partial || len(index.Columns) != 3 || index.Columns[0].Name == nil || *index.Columns[0].Name != "status" || !index.Columns[0].Descending || index.Columns[1].Name == nil || *index.Columns[1].Name != "slug" || index.Columns[1].Collation == nil || *index.Columns[1].Collation != "NOCASE" {
		t.Fatalf("composite partial index shape changed: %#v, %v", index, err)
	}
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
func (r *rootRuntime) StartHeld(context.Context) error {
	r.events = append(r.events, "start-held")
	return r.resumeErr
}
func (r *rootRuntime) Release(context.Context) error {
	r.events = append(r.events, "release")
	return nil
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
	a.ReviewDigest, err = RootReviewDigest(captured, c, a.Resolutions, NotebookStructureDecisions{})
	if err != nil {
		t.Fatal(err)
	}
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

// 正式真实DB写入证明：journal不确定时不得重新开放runtime，避免随后恢复吞掉新写。
type writingRootRuntime struct {
	rootRuntime
	dbPath         string
	acceptedWrites int
}

func (r *writingRootRuntime) Release(ctx context.Context) error {
	r.events = append(r.events, "release")
	db, err := sql.Open("sqlite", r.dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err = db.Exec("UPDATE fixture_audit SET n=n+1"); err != nil {
		return err
	}
	r.acceptedWrites++
	return nil
}
func TestJournalRenameFailureNeverAcceptsRuntimeWrites(t *testing.T) {
	r, c, a := completeRootSource(t)
	runtime := &writingRootRuntime{dbPath: r.Layout.DBPath}
	fault := errors.New("injected directory fsync failure after journal rename")
	_, err := applyReviewedRootMigration(context.Background(), r, c, a, NotebookStructureDecisions{}, runtime, func(stage string) error {
		if stage == "installing-journal-after-rename" {
			return fault
		}
		return nil
	})
	if !errors.Is(err, fault) {
		t.Fatal(err)
	}
	if !runtime.held || runtime.acceptedWrites != 0 {
		t.Fatalf("uncertain journal resumed runtime: held=%v accepted writes=%d", runtime.held, runtime.acceptedWrites)
	}
	if _, err = os.Stat(rootJournalPath(r.Layout)); err != nil {
		t.Fatal("uncertain durable journal discarded")
	}
	if _, err = RecoverRootMigration(context.Background(), r.Layout); err != nil {
		t.Fatal(err)
	}
}
func TestCompleteJournalFailureNeverLosesAcceptedNewWrites(t *testing.T) {
	r, c, a := completeRootSource(t)
	runtime := &writingRootRuntime{dbPath: r.Layout.DBPath}
	fault := errors.New("injected complete journal write failure")
	_, err := applyReviewedRootMigration(context.Background(), r, c, a, NotebookStructureDecisions{}, runtime, func(stage string) error {
		if stage == "complete-journal-before-write" {
			return fault
		}
		return nil
	})
	if !errors.Is(err, fault) {
		t.Fatal(err)
	}
	if runtime.acceptedWrites != 0 {
		t.Fatalf("runtime accepted %d writes before durable commit; rollback lost accepted data", runtime.acceptedWrites)
	}
}

func TestCompleteRenameSyncFailureDoesNotStartPartialCompensation(t *testing.T) {
	r, c, a := completeRootSource(t)
	runtime := &writingRootRuntime{dbPath: r.Layout.DBPath}
	fault := errors.New("complete rename succeeded but directory fsync failed")
	restores := 0
	_, err := applyReviewedRootMigration(context.Background(), r, c, a, NotebookStructureDecisions{}, runtime, func(stage string) error {
		if strings.HasPrefix(stage, "restore:") {
			restores++
			if restores == 2 {
				return errors.New("second restore target failure")
			}
		}
		if stage == "complete-journal-after-rename" {
			return fault
		}
		return nil
	})
	if !errors.Is(err, fault) || !runtime.held || runtime.acceptedWrites != 0 || restores != 0 {
		t.Fatalf("uncertain complete state partially restored: %v, restores %d, writes %d", err, restores, runtime.acceptedWrites)
	}
	var j rootJournal
	if err = readPrivateRegular(rootJournalPath(r.Layout), &j); err != nil || j.State != "complete" {
		t.Fatal("uncertain complete journal not preserved")
	}
	tracked := filepath.Join(r.Layout.Root, r.TrackedSources[0])
	before, err := os.ReadFile(tracked)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = RecoverRootMigration(context.Background(), r.Layout); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(tracked)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("complete recovery changed new document generation")
	}
	db, err := sql.Open("sqlite", r.Layout.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var v int
	if err = db.QueryRow("SELECT stellar_version FROM posts").Scan(&v); err != nil || v != 2 {
		t.Fatal("complete recovery reverted new DB generation")
	}
}
func TestRollbackIntentSurvivesPartialRestoreFailure(t *testing.T) {
	r, c, a := completeRootSource(t)
	runtime := &rootRuntime{}
	restores := 0
	_, err := applyReviewedRootMigration(context.Background(), r, c, a, NotebookStructureDecisions{}, runtime, func(stage string) error {
		if stage == filepath.Join(r.Layout.Root, r.TrackedSources[0]) {
			return errors.New("install failure")
		}
		if strings.HasPrefix(stage, "restore:") {
			restores++
			if restores == 2 {
				return errors.New("second restore failure")
			}
		}
		return nil
	})
	if err == nil || !runtime.held {
		t.Fatal("partial compensation did not hold runtime")
	}
	var j rootJournal
	if err = readPrivateRegular(rootJournalPath(r.Layout), &j); err != nil || j.State != "rollback_required" {
		t.Fatal("partial compensation lost durable rollback intent")
	}
	if _, err = RecoverRootMigration(context.Background(), r.Layout); err != nil {
		t.Fatal(err)
	}
	after, err := captureMigrationRoot(context.Background(), r.Layout, r.TrackedSources, j.BackupDirectory)
	if err != nil || after.Digest != r.Digest {
		t.Fatal("startup recovery did not finish failed compensation")
	}
}
func TestCommittedReleaseWritesSurviveSubsequentRecovery(t *testing.T) {
	r, c, a := completeRootSource(t)
	runtime := &writingRootRuntime{dbPath: r.Layout.DBPath}
	if _, err := ApplyReviewedRootMigration(context.Background(), r, c, a, NotebookStructureDecisions{}, runtime); err != nil {
		t.Fatal(err)
	}
	if runtime.acceptedWrites != 1 {
		t.Fatal("release did not accept real new DB write")
	}
	if len(runtime.events) < 4 || runtime.events[len(runtime.events)-2] != "start-held" || runtime.events[len(runtime.events)-1] != "release" {
		t.Fatal(runtime.events)
	}
	if recovered, err := RecoverRootMigration(context.Background(), r.Layout); err != nil || recovered.Recovered {
		t.Fatal("completed operation retained rollback journal")
	}
	db, err := sql.Open("sqlite", r.Layout.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err = db.QueryRow("SELECT n FROM fixture_audit").Scan(&n); err != nil || n != 1 {
		t.Fatal("accepted committed write lost after recovery")
	}
}

func TestCompleteReviewDigestRejectsChangedOwnerStatusAndUpdatedAt(t *testing.T) {
	r, _, _ := completeRootSource(t)
	legacy := fixture(t, `CREATE TABLE users(id INTEGER PRIMARY KEY);INSERT INTO users VALUES(7),(8);
 CREATE TABLE notebooks(id INTEGER PRIMARY KEY,name TEXT NOT NULL UNIQUE,description TEXT NOT NULL,cover TEXT NOT NULL,nb_order INTEGER NOT NULL,created_at DATETIME NOT NULL);INSERT INTO notebooks VALUES(2,'Notebook','Description','',0,'2020-01-02 03:04:05');
 CREATE TABLE notes(id INTEGER PRIMARY KEY,notebook_id INTEGER NOT NULL,title TEXT NOT NULL,content TEXT NOT NULL,content_html TEXT NOT NULL,tags TEXT NOT NULL,created_at DATETIME NOT NULL,updated_at DATETIME NOT NULL);INSERT INTO notes VALUES(9007199254740993,2,'Note','Body','<p>Old cache</p>','','2020-01-02 03:04:05','2021-02-03 04:05:06');`)
	data, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err = writeRootAtomic(r.Layout.DBPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if err = removeRootTarget(r.Layout.DBPath + suffix); err != nil {
			t.Fatal(err)
		}
	}
	r, err = CaptureMigrationRoot(context.Background(), r.Layout, r.TrackedSources)
	if err != nil {
		t.Fatal(err)
	}
	c := candidate(t, r.Source)
	decisions := structureApprovals(r.Source)
	a := RootMigrationApproval{MigrationApproval: approveFixture(c), RootDigest: r.Digest}
	a.Resolutions = []MigrationResolution{{Source: "schema", Code: "legacy_notebook_schema", Path: "notebooks/notes", Action: "use_notebook_decisions"}, {Source: "notes/9007199254740993", Code: "note_missing_slug_status", Path: "slug/status", Action: "use_notebook_decisions"}, {Source: "content/posts/tracked.md", Code: "file_host_fields", Path: "status", Action: "preserve_status"}}
	a.ReviewDigest, err = RootReviewDigest(r, c, a.Resolutions, decisions)
	if err != nil {
		t.Fatal(err)
	}
	f, err := BuildRootRehearsal(context.Background(), r, c, a, decisions)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(f.Directory) })
	db, err := sql.Open("sqlite", f.Layout.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	index, err := readDatabaseIndex(context.Background(), tx, "sqlite_autoindex_notebooks_1", "notebooks")
	if err != nil {
		t.Fatal(err)
	}
	tx.Rollback()
	if !index.Unique || index.Origin != "u" || index.Columns[0].Name == nil || *index.Columns[0].Name != "name" {
		t.Fatal("source name UNIQUE was replaced by new id PK")
	}
	if _, err = db.Exec("INSERT INTO notebooks(id,name) VALUES('duplicate','Notebook')"); err == nil {
		t.Fatal("source name uniqueness constraint lost")
	}

	for name, mutate := range map[string]func(*NotebookStructureDecisions){"owner": func(d *NotebookStructureDecisions) {
		m := d.Records["notes/9007199254740993"]
		owner := int64(8)
		m.OwnerID = &owner
		d.Records["notes/9007199254740993"] = m
	}, "status": func(d *NotebookStructureDecisions) {
		m := d.Records["notes/9007199254740993"]
		status := "published"
		m.Status = &status
		d.Records["notes/9007199254740993"] = m
	}, "updated_at": func(d *NotebookStructureDecisions) {
		m := d.Records["notebooks/2"]
		date := "2024-01-01 00:00:00"
		m.UpdatedAt = &date
		d.Records["notebooks/2"] = m
	}} {
		t.Run(name, func(t *testing.T) {
			changed := clone(decisions)
			mutate(&changed)
			runtime := &rootRuntime{}
			if _, err := ApplyReviewedRootMigration(context.Background(), r, c, a, changed, runtime); err == nil || len(runtime.events) != 0 {
				t.Fatal("changed reviewed metadata accepted or stopped runtime")
			}
			if _, err := BuildRootRehearsal(context.Background(), r, c, a, changed); err == nil {
				t.Fatal("changed reviewed metadata emitted rehearsal")
			}
		})
	}
	noReview := a
	noReview.ReviewDigest = ""
	if _, err = ApplyReviewedRootMigration(context.Background(), r, c, noReview, decisions, &rootRuntime{}); err == nil {
		t.Fatal("missing full review approval accepted")
	}
}

func TestRootRehearsalRejectsUnsupportedImplicitNotebookConstraints(t *testing.T) {
	for name, constraint := range map[string]string{"nocase": "UNIQUE(name COLLATE NOCASE)", "descending": "UNIQUE(name DESC)"} {
		t.Run(name, func(t *testing.T) {
			r, _, _ := completeRootSource(t)
			legacy := fixture(t, `CREATE TABLE users(id INTEGER PRIMARY KEY);INSERT INTO users VALUES(7),(8);
CREATE TABLE notebooks(id INTEGER PRIMARY KEY,name TEXT NOT NULL,description TEXT NOT NULL,cover TEXT NOT NULL,nb_order INTEGER NOT NULL,created_at DATETIME NOT NULL,`+constraint+`);INSERT INTO notebooks VALUES(2,'Notebook','Description','',0,'2020-01-02 03:04:05');
CREATE TABLE notes(id INTEGER PRIMARY KEY,notebook_id INTEGER NOT NULL,title TEXT NOT NULL,content TEXT NOT NULL,content_html TEXT NOT NULL,tags TEXT NOT NULL,created_at DATETIME NOT NULL,updated_at DATETIME NOT NULL);INSERT INTO notes VALUES(9007199254740993,2,'Note','Body','<p>Old cache</p>','','2020-01-02 03:04:05','2021-02-03 04:05:06');`)
			data, err := os.ReadFile(legacy)
			if err != nil {
				t.Fatal(err)
			}
			if err = writeRootAtomic(r.Layout.DBPath, data, 0600); err != nil {
				t.Fatal(err)
			}
			for _, suffix := range []string{"-wal", "-shm", "-journal"} {
				if err = removeRootTarget(r.Layout.DBPath + suffix); err != nil {
					t.Fatal(err)
				}
			}
			r, err = CaptureMigrationRoot(context.Background(), r.Layout, r.TrackedSources)
			if err != nil {
				t.Fatal(err)
			}
			c := candidate(t, r.Source)
			decisions := structureApprovals(r.Source)
			a := RootMigrationApproval{MigrationApproval: approveFixture(c), RootDigest: r.Digest}
			a.Resolutions = []MigrationResolution{{Source: "schema", Code: "legacy_notebook_schema", Path: "notebooks/notes", Action: "use_notebook_decisions"}, {Source: "notes/9007199254740993", Code: "note_missing_slug_status", Path: "slug/status", Action: "use_notebook_decisions"}, {Source: "content/posts/tracked.md", Code: "file_host_fields", Path: "status", Action: "preserve_status"}}
			a.ReviewDigest, err = RootReviewDigest(r, c, a.Resolutions, decisions)
			if err != nil {
				t.Fatal(err)
			}
			if f, err := BuildRootRehearsal(context.Background(), r, c, a, decisions); err == nil || f.Directory != "" || !strings.Contains(err.Error(), "source implicit index constraint/columns cannot be preserved") {
				t.Fatalf("unsupported source constraint silently changed: %v", err)
			}
		})
	}
}
