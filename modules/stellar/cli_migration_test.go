package stellar

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"blog/internal/app"
	"blog/internal/backup"
	"blog/internal/config"
	"blog/internal/stellarmigration"
	"blog/internal/store"
)

func TestMigrationPrivateArtifactPreservesNumbersAndRefusesUnsafeModes(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "capture.json")
	if err := os.WriteFile(path, []byte(`{"row":9007199254740993}`), 0600); err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := readMigrationPrivateJSON(path, &value); err != nil {
		t.Fatal(err)
	}
	if number, ok := value["row"].(json.Number); !ok || number.String() != "9007199254740993" {
		t.Fatal("large identity rounded")
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if readMigrationPrivateJSON(path, &value) == nil {
		t.Fatal("public artifact accepted")
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if readMigrationPrivateJSON(link, &value) == nil {
		t.Fatal("symlink artifact accepted")
	}
	if err := os.WriteFile(path, []byte(`{"row":1} {"other":2}`), 0600); err != nil {
		t.Fatal(err)
	}
	if readMigrationPrivateJSON(path, &value) == nil {
		t.Fatal("trailing approval accepted")
	}
}

func TestMigrationValidatorUsesCheckedAppSnapshotAfterFileChanges(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "stellar-root-rehearsal-snapshot")
	root := filepath.Join(parent, "root")
	if err := os.MkdirAll(filepath.Join(root, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "data", "uploads"), 0700); err != nil {
		t.Fatal(err)
	}
	appPath := filepath.Join(root, "config", "app.yml")
	if err := os.WriteFile(appPath, []byte("auto_backup: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "site.yml"), []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	st, err := store.New(filepath.Join(root, "data", "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err = st.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	if err = st.EnsureUsersTable(); err != nil {
		t.Fatal(err)
	}
	if err = st.CreateUser(&store.User{Username: "admin", Role: store.RoleAdmin, PasswordHash: "synthetic-hash"}); err != nil {
		t.Fatal(err)
	}
	if _, err = config.InitializeStellarV2(st, map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	if err = st.PutSetting(config.SectionSite, `{"title":"Migration fixture","url":"https://fixture.example"}`); err != nil {
		t.Fatal(err)
	}
	st.Close()
	a, path, err := app.StellarMigrationConfig(app.Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	g, err := app.AcquireStellarRootGuard(a, path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	if !isPrivateMigrationRehearsal(g.AppSnapshot.Root, g.AppSnapshot.DBPath, g.AppSnapshot.UploadDir, g.AppPath) {
		t.Fatal("synthetic staged root was not private")
	}
	external := t.TempDir()
	outsideDB := filepath.Join(external, "outside.db")
	outsideUploads := filepath.Join(external, "uploads")
	if err = os.WriteFile(appPath, []byte("auto_backup: false\ndb_path: "+outsideDB+"\nupload_dir: "+outsideUploads+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = validateMigrationRuntime(context.Background(), g.AppSnapshot, g.AppPath); err != nil {
		t.Fatal(err)
	}
	for _, outside := range []string{outsideDB, outsideUploads} {
		if _, err = os.Stat(outside); !os.IsNotExist(err) {
			t.Fatalf("validator opened changed app resource %s", filepath.Base(outside))
		}
	}
}

func TestMigrationDefaultPreviewDirectoryMustBeOutsideRuntime(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TMPDIR", root)
	layout := stellarmigration.RootLayout{Root: root, DBPath: filepath.Join(root, "data", "blog.db"), UploadDir: filepath.Join(root, "data", "uploads"), AppPath: filepath.Join(root, "config", "app.yml")}
	if _, err := createMigrationPreviewDirectory("", layout); err == nil {
		t.Fatal("default temporary artifact was placed inside captured runtime")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("rejected default output still changed the captured root")
	}
}

func TestReviewedCLIPreviewBindsNotebookMetadataAndKeepsApprovalPending(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "app.yml"), []byte("auto_backup: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	st, err := store.New(filepath.Join(root, "data", "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err = st.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	if err = st.EnsureUsersTable(); err != nil {
		t.Fatal(err)
	}
	first := &store.User{Username: "owner-one", Role: store.RoleAuthor}
	second := &store.User{Username: "owner-two", Role: store.RoleAuthor}
	if err = st.CreateUser(first); err != nil {
		t.Fatal(err)
	}
	if err = st.CreateUser(second); err != nil {
		t.Fatal(err)
	}
	if _, err = config.InitializeStellarV2(st, map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = st.DB().Exec(`DROP TABLE notes; DROP TABLE notebooks;
CREATE TABLE notebooks(id INTEGER PRIMARY KEY,name TEXT UNIQUE,description TEXT,cover TEXT,nb_order INTEGER,created_at TEXT);
CREATE TABLE notes(id INTEGER PRIMARY KEY,notebook_id INTEGER,title TEXT,content TEXT,content_html TEXT,tags TEXT,created_at TEXT,updated_at TEXT);
INSERT INTO notebooks VALUES(2,'Book','','',0,'2024-01-01 00:00:00');
INSERT INTO notes VALUES(2,2,'Note','body','<p>old</p>','','2024-01-01 00:00:00','2024-01-01 00:00:00');`); err != nil {
		t.Fatal(err)
	}
	st.Close()
	if _, err = stellarmigration.ReadSnapshot(context.Background(), filepath.Join(root, "data", "blog.db"), nil); err != nil {
		t.Fatal(err)
	}
	a, path, err := app.StellarMigrationConfig(app.Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	layout := app.StellarMigrationLayout(a, path)
	capture, err := stellarmigration.CaptureMigrationRoot(context.Background(), layout, nil)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := stellarmigration.PreviewWithCapabilities(capture.Source, stellarmigration.ConsumerCapabilities{stellarmigration.ConsumerCollectionOrder: true})
	if err != nil {
		t.Fatal(err)
	}
	resolutions := []stellarmigration.MigrationResolution{}
	appendResolution := func(d stellarmigration.Diagnostic) {
		if !d.Blocking {
			return
		}
		if d.Code != "legacy_notebook_schema" && d.Code != "note_missing_slug_status" {
			t.Fatalf("unexpected synthetic conflict: %s", d.Code)
		}
		resolutions = append(resolutions, stellarmigration.MigrationResolution{Source: d.Source, Code: d.Code, Path: d.Path, Action: "use_notebook_decisions"})
	}
	for _, d := range candidate.Diagnostics {
		appendResolution(d)
	}
	for _, record := range candidate.Records {
		for _, d := range record.Diagnostics {
			appendResolution(d)
		}
	}
	slug, status, updated := "note-2", "draft", "2024-01-01 00:00:00"
	decisions := stellarmigration.NotebookStructureDecisions{SourceDigest: capture.Source.Digest, Records: map[string]stellarmigration.NotebookRecordMetadata{"notebooks/2": {Approved: true, OwnerID: &first.ID, UpdatedAt: &updated}, "notes/2": {Approved: true, OwnerID: &first.ID, Slug: &slug, Status: &status}}}
	review := t.TempDir()
	if err = os.Chmod(review, 0700); err != nil {
		t.Fatal(err)
	}
	pending := migrationCLIApproval{Approval: stellarmigration.RootMigrationApproval{MigrationApproval: stellarmigration.MigrationApproval{SourceDigest: capture.Source.Digest, CandidateDigest: stellarmigration.CandidateDigest(candidate), Resolutions: resolutions}, RootDigest: capture.Digest}, Notebook: decisions}
	for _, item := range []struct {
		name  string
		value any
	}{{"root-capture.private.json", capture}, {"candidate.private.json", candidate}, {"choices-one.private.json", pending}} {
		if err = writeMigrationPrivateJSON(filepath.Join(review, item.name), item.value); err != nil {
			t.Fatal(err)
		}
	}
	firstReview, err := previewMigrationSelections(context.Background(), layout, review, filepath.Join(review, "choices-one.private.json"), filepath.Join(t.TempDir(), "selected"))
	if err != nil {
		t.Fatal(err)
	}
	pending.Notebook.Records["notes/2"] = stellarmigration.NotebookRecordMetadata{Approved: true, OwnerID: &second.ID, Slug: &slug, Status: &status}
	if err = writeMigrationPrivateJSON(filepath.Join(review, "choices-two.private.json"), pending); err != nil {
		t.Fatal(err)
	}
	secondReview, err := previewMigrationSelections(context.Background(), layout, review, filepath.Join(review, "choices-two.private.json"), filepath.Join(t.TempDir(), "selected"))
	if err != nil {
		t.Fatal(err)
	}
	if firstReview.ReviewDigest == secondReview.ReviewDigest {
		t.Fatal("owner metadata changed without changing complete review digest")
	}
	var template migrationCLIApproval
	if err = readMigrationPrivateJSON(filepath.Join(secondReview.Directory, "approval-template.private.json"), &template); err != nil {
		t.Fatal(err)
	}
	if template.Approval.Approved || template.Offline.Approved || len(template.Approval.BodyDigests) != 0 {
		t.Fatal("selection preview inferred execution/body approval")
	}
	approved, err := approveMigrationSelections(context.Background(), layout, secondReview.Directory, filepath.Join(t.TempDir(), "approved-notebook"), false, false)
	if err != nil || !approved.OK {
		t.Fatalf("complete Notebook selection was not approvable: %v", err)
	}
	if err = os.Mkdir(filepath.Join(root, "runtime"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "runtime", "changed"), []byte("new state"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = previewMigrationSelections(context.Background(), layout, review, filepath.Join(review, "choices-two.private.json"), filepath.Join(t.TempDir(), "stale")); err == nil {
		t.Fatal("source root change reused approved review")
	}
}

func TestMigrationSourceInventoryMatchesFormalPostsAndExtraUnion(t *testing.T) {
	root := t.TempDir()
	posts := filepath.Join(root, "content", "posts")
	if err := os.MkdirAll(filepath.Join(posts, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"b.MD", "a.md", "ignore.txt", "nested/skipped.md"} {
		if err := os.WriteFile(filepath.Join(posts, name), []byte("body"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := migrationSourceInventory(root, []string{"content/extra.md", "content/posts/a.md", "content/extra.md"})
	want := []string{"content/extra.md", "content/posts/a.md", "content/posts/b.MD"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("full source union mismatch: %v %v", got, err)
	}
	if _, err = migrationSourceInventory(root, []string{"../outside.md"}); err == nil {
		t.Fatal("escape source accepted")
	}
	if err = os.Symlink(filepath.Join(posts, "a.md"), filepath.Join(posts, "link.md")); err != nil {
		t.Fatal(err)
	}
	if _, err = migrationSourceInventory(root, nil); err == nil {
		t.Fatal("symbolic Markdown followed")
	}
}

func TestMigrationSourceInventoryIncludesCompleteManifestSources(t *testing.T) {
	root := t.TempDir()
	st, err := store.New(filepath.Join(root, "data", "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err = st.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	if _, err = config.InitializeStellarV2(st, map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = st.Create(&store.Post{Slug: "from-manifest", Title: "Manifest", Status: store.StatusPublished, ContentMD: "formal body", StellarVersion: 2, StellarConfig: `{"title":"Manifest"}`, BlogExtensions: "{}"}); err != nil {
		t.Fatal(err)
	}
	service := backup.New(backup.Config{Store: st, BackupDir: filepath.Join(root, "data", "backups")})
	var archive bytes.Buffer
	if err = service.WriteStellarContentZip(&archive); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range reader.File {
		r, e := entry.Open()
		if e != nil {
			t.Fatal(e)
		}
		data, e := io.ReadAll(r)
		r.Close()
		if e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(root, "content", entry.Name)
		if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path, data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	manifest, err := backup.ReadStellarContentDirectory(filepath.Join(root, "content"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := migrationSourceInventory(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{}
	for _, record := range manifest.Records {
		if record.Source != "" {
			want = append(want, "content/"+record.Source)
		}
	}
	if len(want) != 1 || !reflect.DeepEqual(got, want) {
		t.Fatalf("manifest member silently omitted: %v expected %v", got, want)
	}
	if err = os.WriteFile(filepath.Join(root, "content", backup.StellarContentManifestName), []byte(`{"version":99,"records":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = migrationSourceInventory(root, nil); err == nil {
		t.Fatal("invalid manifest silently ignored")
	}
}
