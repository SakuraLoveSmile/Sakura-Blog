package stellarmigration

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"blog/internal/config"
	"blog/internal/store"
)

func approveFixture(c Candidate) MigrationApproval {
	return MigrationApproval{Approved: true, SourceDigest: c.Source.Digest, CandidateDigest: CandidateDigest(c)}
}

func TestLosslessBlobSnapshotCloneAndFTSFixture(t *testing.T) {
	source := fixture(t, `CREATE TABLE assets(id INTEGER PRIMARY KEY,payload BLOB);INSERT INTO assets VALUES(9007199254740993,X'00fffe6180');CREATE VIRTUAL TABLE fixture_fts USING fts5(title,body);INSERT INTO fixture_fts(rowid,title,body) VALUES(7,'标题','searchable content');`)
	before, _ := os.ReadFile(source)
	s := snapshot(t, source, nil)
	if s.FormatVersion != SnapshotFormatVersion || s.Tables["fixture_fts_data"].Kind != "shadow" {
		t.Fatal("lossless format/object class absent")
	}
	copy := clone(s)
	if err := copy.Verify(); err != nil {
		t.Fatal("JSON clone changed binary snapshot digest")
	}
	got, ok, err := blobBytes(copy.Tables["assets"].Rows[0]["payload"])
	if err != nil || !ok || !reflect.DeepEqual(got, []byte{0, 255, 254, 97, 128}) {
		t.Fatal("BLOB bytes lost")
	}
	c := candidate(t, s)
	f, err := BuildMigrationFixture(context.Background(), c, approveFixture(c), NotebookStructureDecisions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(f.Directory) })
	db, err := sql.Open("sqlite", f.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var id int64
	var payload []byte
	var kind string
	if err = db.QueryRow("SELECT id,payload,typeof(payload) FROM assets").Scan(&id, &payload, &kind); err != nil || id != 9007199254740993 || kind != "blob" || !reflect.DeepEqual(payload, got) {
		t.Fatal("SQL BLOB/large ID differs")
	}
	var rowID int64
	var body string
	if err = db.QueryRow("SELECT rowid,body FROM fixture_fts WHERE fixture_fts MATCH 'searchable'").Scan(&rowID, &body); err != nil || rowID != 7 || body != "searchable content" {
		t.Fatalf("FTS shadow restore not searchable: %v", err)
	}
	after, _ := os.ReadFile(source)
	if string(before) != string(after) {
		t.Fatal("readonly source changed")
	}
	if len(f.ExcludedResources) < 3 || len(f.SnapshotCoverage) < 3 || !strings.HasPrefix(f.SourcePath, f.Directory+string(filepath.Separator)) {
		t.Fatal("coverage or archive boundary omitted")
	}
}

func TestMigrationFixtureRejectsLegacyBinaryAndOrphanShadow(t *testing.T) {
	s := snapshot(t, fixture(t, `CREATE TABLE blobs(id INTEGER PRIMARY KEY,value BLOB);INSERT INTO blobs VALUES(1,X'fffe');`), nil)
	for name, mutate := range map[string]func(*Snapshot){
		"legacy format": func(s *Snapshot) { s.FormatVersion = 0 },
		"string blob":   func(s *Snapshot) { s.Tables["blobs"].Rows[0]["value"] = "not trusted bytes" },
		"orphan shadow": func(s *Snapshot) { table := s.Tables["blobs"]; table.Kind = "shadow"; s.Tables["blobs"] = table },
		"injected SQL": func(s *Snapshot) {
			table := s.Tables["blobs"]
			table.Schema += "; ATTACH DATABASE '/tmp/forbidden-migration-target.db' AS other"
			s.Tables["blobs"] = table
		},
	} {
		t.Run(name, func(t *testing.T) {
			input := clone(s)
			mutate(&input)
			input.Digest = input.SourceDigest()
			c := candidate(t, input)
			f, err := BuildMigrationFixture(context.Background(), c, approveFixture(c), NotebookStructureDecisions{})
			if err == nil || f.Directory != "" {
				t.Fatal("unsafe historical/corrupt snapshot emitted fixture")
			}
		})
	}
}

func TestFixtureApprovalIsCompleteCandidateBoundAndCannotEraseDiagnostics(t *testing.T) {
	s := settingsSnapshot(t, map[string]any{"plugins": map[string]any{"pjax": map[string]any{"selectors": []any{"main"}, "timeout": 5000}}})
	c := candidate(t, s)
	for name, mutate := range map[string]func(*Candidate, *MigrationApproval){
		"unapproved":      func(_ *Candidate, a *MigrationApproval) { a.Approved = false },
		"wrong source":    func(_ *Candidate, a *MigrationApproval) { a.SourceDigest = "wrong" },
		"wrong candidate": func(_ *Candidate, a *MigrationApproval) { a.CandidateDigest = "wrong" },
		"still blocking":  func(_ *Candidate, _ *MigrationApproval) {},
		"deleted diagnostic": func(c *Candidate, a *MigrationApproval) {
			c.Theme.Diagnostics = nil
			a.CandidateDigest = CandidateDigest(*c)
		},
		"caps policy bypass": func(c *Candidate, a *MigrationApproval) {
			c.ConsumerCapabilities = ConsumerCapabilities{ConsumerPages: true}
			a.CandidateDigest = CandidateDigest(*c)
		},
	} {
		t.Run(name, func(t *testing.T) {
			copy := clone(c)
			a := approveFixture(copy)
			mutate(&copy, &a)
			f, err := BuildMigrationFixture(context.Background(), copy, a, NotebookStructureDecisions{})
			if err == nil || f.Directory != "" {
				t.Fatal("missing/tampered approval accepted")
			}
		})
	}
	a := approveFixture(c)
	a.Resolutions = []MigrationResolution{{Source: "settings", Code: "pjax_policy_removed", Path: "plugins.pjax", Action: "use_official"}}
	f, err := BuildMigrationFixture(context.Background(), c, a, NotebookStructureDecisions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(f.Directory) })
	archive, err := os.ReadFile(f.SourcePath)
	if err != nil || !strings.Contains(string(archive), `"original"`) || !strings.Contains(string(archive), `"resolutions"`) {
		t.Fatal("original diagnostics/resolutions not retained")
	}
}

func completeMigrationSource(t *testing.T) (string, Snapshot) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "source.db")
	st, err := store.New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	for _, ensure := range []func() error{st.EnsureUsersTable, st.EnsureAuthorizationTables, st.EnsurePostAuthorsTable} {
		if err = ensure(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = st.Create(&store.Post{Slug: "migrate", Title: "Migrated title", ContentMD: "## Public heading\n\nBody", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	if _, err = st.CreateComment(&store.Comment{PageKey: "/post/migrate", Nick: "Fixture reader", ContentMD: "Comment", ContentHTML: "<p>Comment</p>", Status: store.CommentApproved}); err != nil {
		t.Fatal(err)
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	return path, snapshot(t, path, map[string]string{"content/posts/tracked.md": "---\ntitle: Tracked\nstatus: draft\n---\nTracked body\n"})
}

func TestMigrationFixtureCompleteStoreReadAndSourcePreservation(t *testing.T) {
	path, s := completeMigrationSource(t)
	before, _ := os.ReadFile(path)
	c := candidate(t, s)
	a := approveFixture(c)
	a.Resolutions = []MigrationResolution{{Source: "content/posts/tracked.md", Code: "file_host_fields", Path: "status", Action: "preserve_status"}}
	f, err := BuildMigrationFixture(context.Background(), c, a, NotebookStructureDecisions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(f.Directory) })
	for _, p := range []string{f.Directory, filepath.Join(f.Directory, "config"), filepath.Join(f.Directory, "content"), filepath.Join(f.Directory, "data/uploads"), filepath.Join(f.Directory, "cache"), filepath.Join(f.Directory, "runtime")} {
		info, err := os.Stat(p)
		if err != nil || info.Mode().Perm() != 0700 {
			t.Fatal("root locations not isolated/private")
		}
	}
	for _, p := range []string{f.DBPath, f.SourcePath, filepath.Join(f.Directory, "config/app.yml"), filepath.Join(f.Directory, "content/posts/tracked.md")} {
		info, err := os.Stat(p)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("artifact not private")
		}
	}
	st, err := store.OpenExisting(f.DBPath, true)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	settings, err := st.ReadSettingsSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = config.LoadStellarV2Snapshot(settings); err != nil {
		t.Fatal(err)
	}
	p, err := st.GetBySlug("migrate")
	if err != nil || p.StellarVersion != 2 || p.Status != store.StatusPublished || p.ContentMD != "## Public heading\n\nBody" {
		t.Fatal("formal strict Store cannot read canonical candidate")
	}
	for _, table := range []string{"users", "comments", "post_authors", "permissions", "roles", "user_roles", "role_permissions"} {
		var count int
		if err = st.DB().QueryRow("SELECT COUNT(*) FROM " + quoteIdentifier(table)).Scan(&count); err != nil || count != len(s.Tables[table].Rows) {
			t.Fatalf("host table %s count=%d expected=%d err=%v", table, count, len(s.Tables[table].Rows), err)
		}
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("source database changed")
	}
	broken := clone(c)
	broken.Records = broken.Records[:len(broken.Records)-1]
	if f, err := BuildMigrationFixture(context.Background(), broken, approveFixture(broken), NotebookStructureDecisions{}); err == nil || f.Directory != "" {
		t.Fatal("incomplete source record set accepted")
	}
}

func TestFixtureMidTransactionFailureAndCancellationRemoveOnlyNewRoot(t *testing.T) {
	path, s := completeMigrationSource(t)
	before, _ := os.ReadFile(path)
	c := candidate(t, s)
	a := approveFixture(c)
	a.Resolutions = []MigrationResolution{{Source: "content/posts/tracked.md", Code: "file_host_fields", Path: "status", Action: "preserve_status"}}
	var created string
	f, err := buildMigrationFixture(context.Background(), c, a, NotebookStructureDecisions{}, func(stage, dir string, tx *sql.Tx) error {
		if stage == "after-canonical" {
			created = dir
			var count int
			if err := tx.QueryRow("SELECT COUNT(*) FROM posts WHERE stellar_version=2").Scan(&count); err != nil || count != 1 {
				t.Fatal("failure injection did not reach canonical writes")
			}
			observer, e := sql.Open("sqlite", filepath.Join(dir, "data/blog.db"))
			if e != nil {
				t.Fatal(e)
			}
			var committedTables int
			e = observer.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&committedTables)
			_ = observer.Close()
			if e != nil || committedTables != 0 {
				t.Fatal("source/canonical writes committed before whole transaction")
			}
			return errors.New("injected failure")
		}
		return nil
	})
	if err == nil || f.Directory != "" || created == "" {
		t.Fatal("injected failure accepted")
	}
	if _, err = os.Stat(created); !os.IsNotExist(err) {
		t.Fatal("failed private root was published")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if f, err := BuildMigrationFixture(ctx, c, a, NotebookStructureDecisions{}); err == nil || f.Directory != "" {
		t.Fatal("cancelled fixture emitted")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("failed/cancelled operation wrote source")
	}
}

func TestFixtureNotebookMetadataRequiresNamedStructureResolutions(t *testing.T) {
	s := legacyStructureFixture(t)
	c := candidate(t, s)
	a := approveFixture(c)
	a.Resolutions = []MigrationResolution{{Source: "schema", Code: "legacy_notebook_schema", Path: "notebooks/notes", Action: "use_notebook_decisions"}, {Source: "notes/9007199254740993", Code: "note_missing_slug_status", Path: "slug/status", Action: "use_notebook_decisions"}}
	a.BodyDigests = map[string]string{c.Records[1].Source: bytesDigest([]byte(c.Records[1].Markdown))}
	if _, err := BuildMigrationFixture(context.Background(), c, a, NotebookStructureDecisions{}); err == nil {
		t.Fatal("missing metadata guessed")
	}
	withoutBody := a
	withoutBody.BodyDigests = nil
	if _, err := BuildMigrationFixture(context.Background(), c, withoutBody, structureApprovals(s)); err == nil {
		t.Fatal("body transformation accepted without exact digest approval")
	}
	f, err := BuildMigrationFixture(context.Background(), c, a, structureApprovals(s))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(f.Directory) })
	st, err := store.OpenExisting(f.DBPath, true)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	note, err := st.GetNoteByID(9007199254740993)
	if err != nil || note.StellarVersion != 2 || note.AuthorID != 7 || note.Status != "draft" || note.Slug != "技术/原笔记" || note.ContentMD != c.Records[1].Markdown {
		t.Fatalf("note canonical/host conversion differs: %v", err)
	}
}

func TestNamedFooterResolutionChangesValuesAndRevalidatesSchema(t *testing.T) {
	s := settingsSnapshot(t, map[string]any{"footer": map[string]any{"social": []any{map[string]any{"title": "One", "url": "#", "icon_key": "default:home"}, map[string]any{"title": "Two", "url": "#", "icon_key": "default:home"}, map[string]any{"title": "Keep", "url": "/keep", "icon_key": "default:home"}}}})
	c := candidate(t, s)
	a := approveFixture(c)
	a.Resolutions = []MigrationResolution{{Source: "migration.theme", Code: "invalid_value", Path: "leftbar.footer.actions[0].url", Action: "remove_placeholder"}, {Source: "migration.theme", Code: "invalid_value", Path: "leftbar.footer.actions[1].url", Action: "replace_url", ReplacementURL: "/approved"}}
	resolved, _, log, err := resolveMigration(c, a, NotebookStructureDecisions{})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := decode(resolved.Theme.ThemeJSON)
	value, _ := get(object(raw), "leftbar.footer.actions")
	actions := array(value)
	if len(log) != 2 || len(actions) != 2 || object(actions[0])["url"] != "/approved" || object(actions[1])["url"] != "/keep" {
		t.Fatal("footer named action did not apply only reviewed changes")
	}
	a.Resolutions[1].ReplacementURL = "javascript:alert(1)"
	if _, _, _, err = resolveMigration(c, a, NotebookStructureDecisions{}); err == nil {
		t.Fatal("approval bypassed strict URL validation")
	}
}

func TestReviewedWikiAndBannerAlternativesAreActuallyApplied(t *testing.T) {
	s := snapshot(t, fixture(t, `CREATE TABLE wiki_projects(id TEXT PRIMARY KEY,name TEXT,title TEXT,sort INTEGER,pin INTEGER,visible INTEGER);INSERT INTO wiki_projects VALUES('docs','Technical','Display',0,0,1);
CREATE TABLE wikidocs(id INTEGER PRIMARY KEY,project_id TEXT,slug TEXT,title TEXT,content_md TEXT,status TEXT,doc_order INTEGER);INSERT INTO wikidocs VALUES(2,'docs','first','First','Body','published',0);
CREATE TABLE posts(id INTEGER PRIMARY KEY,slug TEXT,title TEXT,content_md TEXT,cover TEXT,banner TEXT);INSERT INTO posts VALUES(1,'image','Image','Body','/cover.png','/banner.png');`), nil)
	c := candidate(t, s)
	a := approveFixture(c)
	a.Resolutions = []MigrationResolution{{Source: "wiki_projects/docs", Code: "wiki_name_title", Path: "name/title", Action: "choose_alternative", AlternativeName: "保留旧 title 为显示 name"}, {Source: "wiki_projects/docs", Code: "wiki_homepage", Path: "route.path/navigation.tree", Action: "choose_alternative", AlternativeName: "采用官方 Collection 首页"}, {Source: "posts/1", Code: "banner_cover", Path: "banner/cover", Action: "choose_alternative", AlternativeName: "保留独立 Blog banner 图源"}}
	resolved, _, log, err := resolveMigration(c, a, NotebookStructureDecisions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(log) != 3 {
		t.Fatal("decision log absent")
	}
	for _, p := range resolved.Records {
		raw, _ := decode(p.ConfigJSON)
		blog, _ := decode(p.BlogJSON)
		switch p.Source {
		case "wiki_projects/docs":
			if object(raw)["name"] != "Display" {
				t.Fatal("chosen display name not applied")
			}
			if _, exists := get(object(blog), "content.wiki_homepage_redirect"); exists {
				t.Fatal("official homepage retained redirect")
			}
		case "posts/1":
			image, _ := get(object(blog), "content.banner_image")
			if image != "/banner.png" || object(raw)["cover"] != "/cover.png" {
				t.Fatal("approved independent banner altered listing cover")
			}
		}
	}
	if encode(resolved.Source) != encode(s) {
		t.Fatal("resolution modified original source/permissions")
	}
	a.Resolutions[0].AlternativeName = "unreviewed replacement"
	if _, _, _, err = resolveMigration(c, a, NotebookStructureDecisions{}); err == nil {
		t.Fatal("unknown alternative guessed")
	}
}
