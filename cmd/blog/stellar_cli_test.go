package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"blog/internal/config"
	"blog/internal/stellarcli"
	"blog/internal/stellarimages"
	"blog/internal/store"
)

// 子进程执行实际 main 路径，避免只测命令核心却漏掉 Bootstrap 前的 dispatch。
func TestStellarMainProcess(t *testing.T) {
	if raw := os.Getenv("STELLAR_MAIN_TEST_ARGS"); raw != "" {
		var args []string
		if err := json.Unmarshal([]byte(raw), &args); err != nil {
			panic(err)
		}
		flag.CommandLine = flag.NewFlagSet("blog", flag.ExitOnError)
		os.Args = append([]string{os.Args[0]}, args...)
		main()
		panic("tool command returned instead of exiting")
	}
}
func cliProcess(t *testing.T, root string, args ...string) ([]byte, []byte, int) {
	t.Helper()
	argv := append([]string{"-root", root, "stellar"}, args...)
	encoded, err := json.Marshal(argv)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestStellarMainProcess$")
	if binary := os.Getenv("STELLAR_CLI_BINARY"); binary != "" {
		command = exec.Command(binary, argv...)
	}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "BLOG_") && !strings.HasPrefix(entry, "STELLAR_MAIN_TEST_ARGS=") {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "STELLAR_MAIN_TEST_ARGS="+string(encoded))
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err = command.Run()
	code := 0
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	return stdout.Bytes(), stderr.Bytes(), code
}

func cliFixture(t *testing.T) (string, int64) {
	t.Helper()
	root := t.TempDir()
	st, err := store.New(filepath.Join(root, "data", "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for _, ensure := range []func() error{st.EnsureSchema, st.EnsureUsersTable, st.EnsurePostAuthorsTable, st.EnsureNotebooksTable, st.EnsureNotesTable} {
		if err := ensure(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := config.InitializeStellarV2(st, map[string]any{}, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if err := st.PutSetting(config.SectionSite, `{"title":"CLI fixture","url":"https://fixture.example"}`); err != nil {
		t.Fatal(err)
	}
	user := &store.User{Username: "writer", Role: store.RoleAuthor}
	if err := st.CreateUser(user); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateNotebook(&store.Notebook{ID: "journal", AuthorID: user.ID, StellarVersion: 2, StellarConfig: `{"name":"Journal"}`, BlogExtensions: "{}"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateNote(&store.Note{NotebookID: "journal", Slug: "public", Title: "Public", AuthorID: user.ID, Status: store.StatusPublished, ContentMD: "![local](/uploads/fixture.png)", StellarVersion: 2, StellarConfig: `{"title":"Public","collection":{"profile":"notebook","id":"journal"}}`, BlogExtensions: "{}"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateNote(&store.Note{NotebookID: "journal", Slug: "draft", Title: "Draft", AuthorID: user.ID, Status: store.StatusDraft, ContentMD: "![private](/uploads/private-draft.png)", StellarVersion: 2, StellarConfig: `{"title":"Draft","collection":{"profile":"notebook","id":"journal"}}`, BlogExtensions: "{}"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`INSERT INTO posts(slug,title,status,password,content_md,stellar_version,stellar_config,blog_extensions) VALUES('protected','Protected','published','fixture-password','![private](/uploads/private-protected.png)',2,'{"title":"Protected"}','{}')`); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "data", "uploads"), 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(root, "data", "uploads", "fixture.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewNRGBA(image.Rect(0, 0, 3, 2))); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return root, user.ID
}

func TestStellarMainDoctorAndNewNote(t *testing.T) {
	root, owner := cliFixture(t)
	dbPath := filepath.Join(root, "data", "blog.db")
	before, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := cliProcess(t, root, "doctor", "--format", "json")
	var doctor stellarcli.DoctorResult
	if err := json.Unmarshal(stdout, &doctor); err != nil || code != 0 || !doctor.OK {
		t.Fatalf("doctor exit=%d stderr=%s stdout=%s err=%v", code, stderr, stdout, err)
	}
	after, err := os.ReadFile(dbPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("doctor changed main DB")
	}
	stdout, stderr, code = cliProcess(t, root, "new", "note", "--notebook", "journal", "--title", "Created via CLI", "--dry-run")
	var plan stellarcli.NotePlan
	if err := json.Unmarshal(stdout, &plan); err != nil || code != 0 || !plan.DryRun || plan.Created {
		t.Fatalf("dry run exit=%d stderr=%s stdout=%s err=%v", code, stderr, stdout, err)
	}
	after, err = os.ReadFile(dbPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("new note dry-run changed main DB")
	}
	stdout, stderr, code = cliProcess(t, root, "new", "note", "--notebook", "journal", "--title", "Created via CLI")
	if err := json.Unmarshal(stdout, &plan); err != nil || code != 0 || !plan.Created || plan.ID <= 0 || plan.AuthorID != owner {
		t.Fatalf("new note exit=%d stderr=%s stdout=%s err=%v", code, stderr, stdout, err)
	}
	createdID := plan.ID
	st, err := store.OpenExisting(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`UPDATE notes SET content_md='preserved authored body' WHERE id=?`, createdID); err != nil {
		t.Fatal(err)
	}
	st.Close()
	_, stderr, code = cliProcess(t, root, "new", "note", "--notebook", "journal", "--title", "Created via CLI")
	if code != 1 {
		t.Fatalf("duplicate exit=%d stderr=%s", code, stderr)
	}
	for _, args := range [][]string{{"--notebook", "../journal", "--title", "Escape"}, {"--notebook", "journal", "--title", "../escape"}} {
		_, stderr, code = cliProcess(t, root, append([]string{"new", "note"}, args...)...)
		if code != 1 {
			t.Fatalf("unsafe path exit=%d stderr=%s", code, stderr)
		}
	}
	st, err = store.OpenExisting(dbPath, true)
	if err != nil {
		t.Fatal(err)
	}
	note, err := st.GetNoteBySlug("journal", "Created via CLI")
	if err != nil || note == nil || note.ID != createdID || note.ContentMD != "preserved authored body" || note.Status != store.StatusDraft {
		t.Fatalf("duplicate overwrote draft: %#v %v", note, err)
	}
	notes, err := st.ListNotes("journal")
	if err != nil || len(notes) != 3 {
		t.Fatalf("failed commands left partial notes: %d %v", len(notes), err)
	}
	st.Close()
	_, _, code = cliProcess(t, root, "doctor", "--format", "yaml")
	if code != 2 {
		t.Fatalf("invalid format exit=%d", code)
	}
}

func TestStellarMainImagesActualSSRAndReadOnlyDryRun(t *testing.T) {
	root, _ := cliFixture(t)
	dbPath := filepath.Join(root, "data", "blog.db")
	cache := filepath.Join(root, "content", "_data", "caches", "images_metadata.json")
	before, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(filepath.Join(root, "data", "uploads", "fixture.png"))
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := cliProcess(t, root, "images", "--dry-run")
	var report stellarimages.Report
	if err := json.Unmarshal(stdout, &report); err != nil || code != 0 {
		t.Fatalf("images dry-run exit=%d stderr=%s stdout=%s err=%v", code, stderr, stdout, err)
	}
	found := false
	for _, url := range report.Missing {
		if url == "https://fixture.example/uploads/fixture.png" {
			found = true
		}
		if strings.Contains(url, "/private-") {
			t.Fatal("private content entered public SSR snapshot")
		}
	}
	if !found {
		t.Fatalf("actual public Note SSR image missing: %#v", report)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatal("images dry-run wrote cache")
	}
	after, err := os.ReadFile(dbPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("images dry-run changed DB")
	}
	stdout, stderr, code = cliProcess(t, root, "images", "--refresh", "/uploads/fixture.png")
	if err := json.Unmarshal(stdout, &report); err != nil || code != 0 || report.Found != 1 || report.Processed != 1 || report.Failed != 0 {
		t.Fatalf("images refresh exit=%d stderr=%s stdout=%s err=%v", code, stderr, stdout, err)
	}
	metadata, err := stellarimages.ReadStore(cache)
	if err != nil {
		t.Fatal(err)
	}
	entry := metadata.Images["https://fixture.example/uploads/fixture.png"]
	if entry.Dimensions == nil || entry.Dimensions.Width != 3 || entry.Dimensions.Height != 2 {
		t.Fatalf("bad dimensions: %#v", entry)
	}
	current, err := os.ReadFile(filepath.Join(root, "data", "uploads", "fixture.png"))
	if err != nil || !bytes.Equal(original, current) {
		t.Fatal("image processing replaced original")
	}
	_, stderr, code = cliProcess(t, root, "images", "--page", "missing/index.html", "--dry-run")
	if code != 1 {
		t.Fatalf("missing page exit=%d stderr=%s", code, stderr)
	}
}

func TestStellarMainDoctorMissingAndLegacyDatabase(t *testing.T) {
	missing := t.TempDir()
	stdout, stderr, code := cliProcess(t, missing, "doctor", "--format", "json")
	var doctor stellarcli.DoctorResult
	if err := json.Unmarshal(stdout, &doctor); err != nil || code != 1 || doctor.OK {
		t.Fatalf("missing doctor exit=%d stderr=%s stdout=%s err=%v", code, stderr, stdout, err)
	}
	if _, err := os.Stat(filepath.Join(missing, "data")); !os.IsNotExist(err) {
		t.Fatal("doctor created missing runtime directory")
	}
	root, _ := cliFixture(t)
	dbPath := filepath.Join(root, "data", "blog.db")
	st, err := store.OpenExisting(dbPath, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`DROP TABLE notes; DROP TABLE notebooks; CREATE TABLE notebooks(id INTEGER PRIMARY KEY,label TEXT); INSERT INTO notebooks VALUES(7,'old notebook'); CREATE TABLE notes(id INTEGER PRIMARY KEY,notebook_id INTEGER,body TEXT); INSERT INTO notes VALUES(8,7,'old body')`); err != nil {
		t.Fatal(err)
	}
	st.Close()
	before, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = cliProcess(t, root, "doctor", "--format", "json")
	if err := json.Unmarshal(stdout, &doctor); err != nil || code != 1 || doctor.OK {
		t.Fatalf("legacy doctor exit=%d stderr=%s stdout=%s err=%v", code, stderr, stdout, err)
	}
	found := false
	for _, issue := range doctor.Issues {
		if issue.Code == "migration_required" && issue.Source == "sqlite:notebooks" && issue.Path == "schema" {
			found = true
		}
	}
	if !found {
		t.Fatalf("legacy schema diagnostic absent: %#v", doctor.Issues)
	}
	stdout, stderr, code = cliProcess(t, root, "doctor")
	if code != 1 || !bytes.Contains(stdout, []byte("Stellar doctor: FAIL")) || !bytes.Contains(stdout, []byte("sqlite:notebooks")) {
		t.Fatalf("legacy text doctor exit=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	after, err := os.ReadFile(dbPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("legacy doctor changed main DB")
	}
}
