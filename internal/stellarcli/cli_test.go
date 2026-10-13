package stellarcli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"blog/internal/config"
	"blog/internal/post"
	"blog/internal/stellarimages"
	"blog/internal/store"
)

func testEnvironment() Environment {
	return Environment{Adapter: "go-ssr-sqlite", Go: runtime.Version(), RequiredGo: "go1.26.6", Theme: "2.0.0", Commit: "9d11978a24fe6610e3a7dbab741cf62634539d05", ResourceSHA256: strings.Repeat("a", 64)}
}
func fixture(t *testing.T) (*store.Store, string, int64) {
	t.Helper()
	for _, name := range []string{"BLOG_ROOT", "BLOG_DB", "BLOG_SEED"} {
		t.Setenv(name, "")
	}
	root := t.TempDir()
	st, err := store.New(filepath.Join(root, "data", "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	for _, ensure := range []func() error{st.EnsureSettingsTable, st.EnsurePagesTable, st.EnsureWikiProjectsTable, st.EnsureWikidocsTable, st.EnsureTopicsTable, st.EnsureNotebooksTable, st.EnsureNotesTable, st.EnsureUsersTable, st.EnsureAuthorizationTables} {
		if err := ensure(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := config.InitializeStellarV2(st, map[string]any{}, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	u := &store.User{Username: "writer", Role: store.RoleAuthor}
	if err := st.CreateUser(u); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateNotebook(&store.Notebook{ID: "journal", Name: "Journal", AuthorID: u.ID, StellarVersion: 2, StellarConfig: `{"name":"Journal"}`, BlogExtensions: "{}"}); err != nil {
		t.Fatal(err)
	}
	return st, root, u.ID
}
func testTime() time.Time {
	return time.Date(2026, 10, 13, 12, 34, 0, 0, time.FixedZone("+08", 8*60*60))
}

func TestNewNoteDryRunCreateConflictAndRollback(t *testing.T) {
	st, root, owner := fixture(t)
	options := Options{Root: root, Environment: testEnvironment(), Now: testTime}
	args := []string{"new", "note", "--notebook", "journal", "--title", " Hello 世界 ", "--tags", "go, go, GO, a/b"}
	var out, errs bytes.Buffer
	if code := Run(context.Background(), append(args, "--dry-run"), options, &out, &errs); code != 0 {
		t.Fatalf("dry-run %d: %s", code, errs.String())
	}
	var preview NotePlan
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Created || !preview.DryRun || preview.AuthorID != owner || preview.Title != "Hello 世界" || !reflect.DeepEqual(preview.Tags, []string{"go", "GO", "a/b"}) {
		t.Fatalf("bad preview: %#v", preview)
	}
	doc, err := post.ParseStellarDocument([]byte(preview.Content), post.StellarDocumentOptions{Source: "preview", Profile: "notebook"})
	if err != nil || doc.Raw["title"] != "Hello 世界" || doc.Raw["date"] != "2026-10-13 12:34" {
		t.Fatalf("invalid front matter %v %v", doc.Raw, err)
	}
	notes, err := st.ListNotes("journal")
	if err != nil || len(notes) != 0 {
		t.Fatal("dry-run created notes")
	}
	out.Reset()
	errs.Reset()
	if code := Run(context.Background(), args, options, &out, &errs); code != 0 {
		t.Fatalf("create %d: %s", code, errs.String())
	}
	note, err := st.GetNoteBySlug("journal", "Hello 世界")
	if err != nil || note == nil || note.Status != store.StatusDraft || note.AuthorID != owner {
		t.Fatalf("missing draft: %v %v", note, err)
	}
	if _, err := os.Stat(filepath.Join(root, "content")); !os.IsNotExist(err) {
		t.Fatal("created parallel Markdown copy")
	}
	out.Reset()
	errs.Reset()
	if code := Run(context.Background(), args, options, &out, &errs); code != 1 || !strings.Contains(errs.String(), store.ErrNoteConflict.Error()) {
		t.Fatalf("conflict %d: %s", code, errs.String())
	}
	plan, err := BuildNotePlan(st, NoteOptions{Notebook: "journal", Title: "Rollback", Author: owner, GeneratedAt: testTime()})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateUserStatus(owner, store.UserStatusBanned); err != nil {
		t.Fatal(err)
	}
	if err := WriteNotePlan(st, &plan); err == nil {
		t.Fatal("banned author accepted")
	}
	if note, err := st.GetNoteBySlug("journal", "Rollback"); err != nil || note != nil {
		t.Fatalf("partial draft survived %v %v", note, err)
	}
}

func TestNewNoteIdentityAndSafeSegments(t *testing.T) {
	st, _, owner := fixture(t)
	for _, bad := range []string{"", ".", "..", "../escape", "/absolute", "a\\b", "a:b", "nul\x00", "line\nbreak"} {
		if _, err := SafeSegment(bad, "--title"); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	if _, err := NormalizeTags("good,bad\nline"); err == nil {
		t.Fatal("tags newline accepted")
	}
	if _, err := BuildNotePlan(st, NoteOptions{Notebook: "journal", Title: "Mismatch", Author: owner + 1, GeneratedAt: testTime()}); err == nil {
		t.Fatal("wrong owner accepted")
	}
	nb, err := st.GetNotebook("journal")
	if err != nil {
		t.Fatal(err)
	}
	nb.AuthorID = 0
	if err := st.UpdateNotebook(nb.ID, nb); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildNotePlan(st, NoteOptions{Notebook: "journal", Title: "Unknown", GeneratedAt: testTime()}); err == nil {
		t.Fatal("unknown owner inferred")
	}
	if _, err := BuildNotePlan(st, NoteOptions{Notebook: "journal", Title: "Explicit", Author: owner, GeneratedAt: testTime()}); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorStrictRecordsMembershipAndWidgetWarnings(t *testing.T) {
	st, _, owner := fixture(t)
	r := Doctor(context.Background(), st, testEnvironment())
	if !r.OK || r.Checked.Collections != 1 {
		t.Fatalf("doctor: %#v", r)
	}
	plan, err := BuildNotePlan(st, NoteOptions{Notebook: "journal", Title: "Ready", Author: owner, GeneratedAt: testTime()})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteNotePlan(st, &plan); err != nil {
		t.Fatal(err)
	}
	r = Doctor(context.Background(), st, testEnvironment())
	if !r.OK || r.Checked.Pages != 1 {
		t.Fatalf("doctor note: %#v", r)
	}
	if _, err := st.DB().Exec(`UPDATE notes SET stellar_config='{"collection":{"profile":"notebook","id":"missing"}}'`); err != nil {
		t.Fatal(err)
	}
	r = Doctor(context.Background(), st, testEnvironment())
	if r.OK || !hasCode(r.Issues, "collection_not_found") {
		t.Fatalf("missing membership absent: %#v", r)
	}
	if _, err := st.DB().Exec(`UPDATE notes SET stellar_version=0`); err != nil {
		t.Fatal(err)
	}
	r = Doctor(context.Background(), st, testEnvironment())
	if !hasCode(r.Issues, "migration_required") {
		t.Fatal("legacy content not rejected")
	}
	if _, err := st.DB().Exec(`UPDATE settings SET value='{"version":2,"theme":{"style":{}}}' WHERE section='stellar_v2'`); err != nil {
		t.Fatal(err)
	}
	r = Doctor(context.Background(), st, testEnvironment())
	if !hasCode(r.Issues, "removed_field") {
		t.Fatalf("old config not rejected: %#v", r)
	}
	if strings.Contains(FormatDoctorText(r), "Node.js:") {
		t.Fatal("misreported Node runtime")
	}
}
func hasCode(issues []Diagnostic, code string) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

func TestDoctorProcess(t *testing.T) {
	if os.Getenv("STELLAR_CLI_TEST_CHILD") == "1" {
		os.Exit(Run(context.Background(), []string{"doctor", "--format", "json"}, Options{Root: os.Getenv("STELLAR_CLI_TEST_ROOT"), Environment: testEnvironment()}, os.Stdout, os.Stderr))
	}
	st, root, _ := fixture(t)
	st.Close()
	for _, want := range []int{0, 1} {
		if want == 1 {
			w, err := store.OpenExisting(filepath.Join(root, "data", "blog.db"), false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.DB().Exec(`UPDATE settings SET value='{"version":1,"theme":{}}' WHERE section='stellar_v2'`); err != nil {
				t.Fatal(err)
			}
			w.Close()
		}
		before, err := os.ReadFile(filepath.Join(root, "data", "blog.db"))
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(os.Args[0], "-test.run=^TestDoctorProcess$")
		cmd.Env = append(os.Environ(), "STELLAR_CLI_TEST_CHILD=1", "STELLAR_CLI_TEST_ROOT="+root)
		b, err := cmd.Output()
		got := 0
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				got = exit.ExitCode()
			} else {
				t.Fatal(err)
			}
		}
		var report DoctorResult
		if err := json.Unmarshal(b, &report); err != nil {
			t.Fatalf("not standalone JSON: %q %v", b, err)
		}
		if got != want || report.OK != (want == 0) {
			t.Fatalf("exit=%d want=%d report=%#v", got, want, report)
		}
		after, err := os.ReadFile(filepath.Join(root, "data", "blog.db"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatal("doctor changed SQLite bytes")
		}
	}
}

func TestImagesUsesSnapshotProviderAndDoesNotBootstrap(t *testing.T) {
	st, root, _ := fixture(t)
	var out, errs bytes.Buffer
	called := false
	options := Options{Root: root, Images: func(ctx context.Context, app *config.App, ro *store.Store, opts stellarimages.PrepareOptions) (stellarimages.Report, error) {
		called = true
		if !opts.DryRun || opts.Page != "journal/" {
			t.Fatalf("bad options: %#v", opts)
		}
		if _, err := ro.DB().Exec(`INSERT INTO settings VALUES('forbidden','{}')`); err == nil {
			t.Fatal("image command opened writable database")
		}
		return stellarimages.Report{Found: 1, Missing: []string{"https://example.org/image.png"}}, nil
	}}
	if code := Run(context.Background(), []string{"images", "--page", "journal/", "--dry-run"}, options, &out, &errs); code != 0 || !called {
		t.Fatalf("images %d %s", code, errs.String())
	}
	var report stellarimages.Report
	if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.Found != 1 {
		t.Fatalf("bad output: %s %v", out.String(), err)
	}
	if _, err := st.GetSetting("forbidden"); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorReportsLegacyNotebookSchemaWithoutConversion(t *testing.T) {
	st, _, _ := fixture(t)
	if _, err := st.DB().Exec(`DROP TABLE notes; DROP TABLE notebooks; CREATE TABLE notebooks(id INTEGER PRIMARY KEY,label TEXT); INSERT INTO notebooks VALUES(7,'old notebook'); CREATE TABLE notes(id INTEGER PRIMARY KEY,notebook_id INTEGER,body TEXT); INSERT INTO notes VALUES(8,7,'old body')`); err != nil {
		t.Fatal(err)
	}
	report := Doctor(context.Background(), st, testEnvironment())
	found := false
	for _, issue := range report.Issues {
		if issue.Code == "migration_required" && issue.Source == "sqlite:notebooks" && issue.Path == "schema" {
			found = true
		}
	}
	if report.OK || !found {
		t.Fatalf("legacy schema was not diagnosed: %#v", report.Issues)
	}
	var body string
	if err := st.DB().QueryRow(`SELECT body FROM notes WHERE id=8`).Scan(&body); err != nil || body != "old body" {
		t.Fatalf("legacy body changed: %q %v", body, err)
	}
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM pragma_table_info('notebooks') WHERE name IN ('author_id','stellar_version','stellar_config')`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("doctor added migration columns: %d %v", count, err)
	}
}
