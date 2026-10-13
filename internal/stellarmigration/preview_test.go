package stellarmigration

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, ddl string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(ddl); err != nil {
		t.Fatal(err)
	}
	return path
}
func snapshot(t *testing.T, path string, files map[string]string) Snapshot {
	t.Helper()
	s, err := ReadSnapshot(context.Background(), path, files)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func candidate(t *testing.T, s Snapshot) Candidate {
	t.Helper()
	c, err := Preview(s)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func hasCode(ds []Diagnostic, code string) bool {
	for _, d := range ds {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestReadSnapshotExistingReadOnlyAndDeterministic(t *testing.T) {
	path := fixture(t, `CREATE TABLE settings(section TEXT PRIMARY KEY,value TEXT);INSERT INTO settings VALUES('custom','{"n":9007199254740993,"false":false,"null":null}');CREATE TABLE comments(id INTEGER PRIMARY KEY,page_key TEXT);INSERT INTO comments VALUES(1,'post:a');`)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := snapshot(t, path, nil)
	again := snapshot(t, path, nil)
	if s.Digest != again.Digest {
		t.Fatal("snapshot digest not deterministic")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatal("read-only source changed")
	}
	if !strings.Contains(encode(s), "9007199254740993") {
		t.Fatal("number lost")
	}
	missing := filepath.Join(t.TempDir(), "does-not-exist.db")
	if _, err := ReadSnapshot(context.Background(), missing, nil); err == nil {
		t.Fatal("created missing source")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("read created database")
	}
}

func TestUnknownSettingsPreservedAndBlocked(t *testing.T) {
	path := fixture(t, `CREATE TABLE settings(section TEXT PRIMARY KEY,value TEXT);INSERT INTO settings VALUES('custom','{"n":9007199254740993,"false":false,"null":null}');`)
	s := snapshot(t, path, nil)
	c := candidate(t, s)
	if !strings.Contains(encode(c.Source), "9007199254740993") || !strings.Contains(encode(c.Source), `\"false\":false`) || !strings.Contains(encode(c.Source), `\"null\":null`) {
		t.Fatal("unknown source values not preserved")
	}
	if !hasCode(c.Theme.Diagnostics, "unmapped_setting") {
		t.Fatal("unknown silently discarded")
	}
	if ValidateCandidate(c, s) == nil {
		t.Fatal("unresolved candidate accepted")
	}
	if snapshot(t, path, nil).Digest != s.Digest {
		t.Fatal("failed preflight wrote source")
	}
}

func TestVersionTwoConfigAndNumberRawPreserved(t *testing.T) {
	path := fixture(t, `CREATE TABLE settings(section TEXT PRIMARY KEY,value TEXT);INSERT INTO settings VALUES('stellar_v2','{ "version":2, "theme":{} }');INSERT INTO settings VALUES('blog_extensions','{"custom":{"n":9007199254740993,"off":false,"nil":null}}');CREATE TABLE posts(id INTEGER PRIMARY KEY,slug TEXT,title TEXT,content_md TEXT,stellar_version INTEGER,stellar_config TEXT,blog_extensions TEXT);INSERT INTO posts VALUES(1,'a','Title','{% blog_links %}',2,'{ "title":"Title", "comments":{"enabled":false} }','{ "custom":{"n":9007199254740993,"nil":null,"off":false} }');`)
	s := snapshot(t, path, nil)
	c := candidate(t, s)
	if !c.Theme.Preserved || c.Theme.DocumentJSON != `{ "version":2, "theme":{} }` {
		t.Fatal("versioned theme rebuilt")
	}
	p := c.Records[0]
	if !p.Preserved || p.ConfigJSON != `{ "title":"Title", "comments":{"enabled":false} }` || p.Markdown != "{% blog_links %}" {
		t.Fatal("versioned content rewritten")
	}
	if err := ValidateCandidate(c, s); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.BlogJSON, "9007199254740993") {
		t.Fatal("number altered")
	}
}

func TestSourceChangedAndIncompleteCandidateRejected(t *testing.T) {
	path := fixture(t, `CREATE TABLE settings(section TEXT PRIMARY KEY,value TEXT);CREATE TABLE posts(id INTEGER PRIMARY KEY,slug TEXT,title TEXT,content_md TEXT);INSERT INTO posts VALUES(1,'a','Title','body');CREATE TABLE comments(id INTEGER PRIMARY KEY,page_key TEXT);INSERT INTO comments VALUES(1,'post:a');`)
	s := snapshot(t, path, nil)
	c := candidate(t, s)
	if err := ValidateCandidate(c, s); err != nil {
		t.Fatal(err)
	}
	incomplete := clone(c)
	incomplete.Records = nil
	if ValidateCandidate(incomplete, s) == nil {
		t.Fatal("partial batch accepted")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`UPDATE comments SET page_key='post:b'`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	current := snapshot(t, path, nil)
	if ValidateCandidate(c, current) == nil {
		t.Fatal("changed untouched comments not detected")
	}
	changed := clone(s)
	changed.Files["new.md"] = "new"
	if changed.Verify() == nil {
		t.Fatal("tampered snapshot accepted")
	}
}

func TestCollectionsMembershipAndHostLifecyclePreserved(t *testing.T) {
	path := fixture(t, `CREATE TABLE settings(section TEXT PRIMARY KEY,value TEXT);CREATE TABLE topics(id INTEGER PRIMARY KEY,name TEXT,slug TEXT,sort INTEGER);INSERT INTO topics VALUES(7,'Series','series',2);CREATE TABLE posts(id INTEGER PRIMARY KEY,slug TEXT,title TEXT,topic_id INTEGER,topic TEXT,content_md TEXT,password TEXT,status TEXT,draft_json TEXT,pinned INTEGER,poster TEXT,banner TEXT,cover TEXT);INSERT INTO posts VALUES(5,'post-a','Post',7,'Series','{% links %}','secret','draft','{"n":9007199254740993}',1,'{"headline":"Poster","caption":"Caption","color":"#fff","topic":"Series"}','/banner.png','/cover.png');CREATE TABLE comments(id INTEGER PRIMARY KEY,page_key TEXT);INSERT INTO comments VALUES(3,'post:post-a');`)
	s := snapshot(t, path, nil)
	c := candidate(t, s)
	if len(c.Records) != 2 {
		t.Fatal("missing collection/member")
	}
	p := c.Records[1]
	v, err := decode(p.ConfigJSON)
	if err != nil {
		t.Fatal(err)
	}
	if id, _ := get(object(v), "collection.id"); id != "series" {
		t.Fatal("topic source ownership lost")
	}
	if _, ok := object(v)["password"]; ok {
		t.Fatal("password entered schema")
	}
	if _, ok := object(v)["status"]; ok {
		t.Fatal("status entered schema")
	}
	if p.Markdown != "{% blog_links %}" || !strings.Contains(p.BlogJSON, `"headline":"Poster"`) || !hasCode(p.Diagnostics, "banner_cover") || !hasCode(p.Diagnostics, "poster_consumer") {
		t.Fatal("candidate changes/conflicts missing")
	}
	if c.Source.Tables["posts"].Rows[0]["password"] != "secret" || c.Source.Tables["comments"].Rows[0]["page_key"] != "post:post-a" {
		t.Fatal("host properties altered")
	}
	public := encode(c.PublicReport())
	for _, secret := range []string{"secret", "Caption", "9007199254740993", "/banner.png"} {
		if strings.Contains(public, secret) {
			t.Fatal("private value leaked to public report")
		}
	}
}

func TestWikiConflictsHaveConcreteAlternatives(t *testing.T) {
	path := fixture(t, `CREATE TABLE settings(section TEXT PRIMARY KEY,value TEXT);CREATE TABLE wiki_projects(id TEXT PRIMARY KEY,name TEXT,title TEXT,description TEXT,sort INTEGER,pin INTEGER,visible INTEGER);INSERT INTO wiki_projects VALUES('docs','technical','Display title','Description',3,0,1);CREATE TABLE wikidocs(id INTEGER PRIMARY KEY,project_id TEXT,slug TEXT,title TEXT,doc_order INTEGER,content_md TEXT,status TEXT);INSERT INTO wikidocs VALUES(1,'docs','start','Intro',1,'body','published');`)
	c := candidate(t, snapshot(t, path, nil))
	p := c.Records[0]
	for _, code := range []string{"wiki_name_title", "wiki_homepage"} {
		found := false
		for _, d := range p.Diagnostics {
			if d.Code == code {
				found = true
				if len(d.Alternatives) != 2 || d.Alternatives[0].ConfigJSON == "" || d.Alternatives[1].ConfigJSON == "" {
					t.Fatal("unreviewable alternatives")
				}
			}
		}
		if !found {
			t.Fatalf("missing %s", code)
		}
	}
	for _, p := range c.Records {
		if hasCode(p.Diagnostics, "collection_not_found") {
			t.Fatal("valid explicit wiki membership rejected")
		}
	}
}

func TestLegacyNotebookSchemaAndNoInventedLifecycle(t *testing.T) {
	path := fixture(t, `CREATE TABLE notebooks(id INTEGER PRIMARY KEY,name TEXT,nb_order INTEGER);INSERT INTO notebooks VALUES(2,'Notebook',10);CREATE TABLE notes(id INTEGER PRIMARY KEY,notebook_id INTEGER,title TEXT,content TEXT,tags TEXT);INSERT INTO notes VALUES(4,2,'Note','body','a/b,c');`)
	s := snapshot(t, path, nil)
	c := candidate(t, s)
	if !hasCode(c.Diagnostics, "legacy_notebook_schema") || !hasCode(c.Records[1].Diagnostics, "note_missing_slug_status") {
		t.Fatal("old schema conflict missing")
	}
	p := c.Records[1]
	if p.ID != "4" || p.Markdown != "body" || p.Path != "" {
		t.Fatal("old identity/body changed or route invented")
	}
	if _, ok := s.Tables["notes"].Rows[0]["status"]; ok {
		t.Fatal("invented old lifecycle")
	}
}

func TestMarkdownMigrationSkipsCodePreservesUnknownAndArguments(t *testing.T) {
	src := "```md\n{% links %}\n```\n`{% moments %}`\n    {% about %}\n{% moments limit:5 %}\n{% tip text:hello %}body{% endtip %}\n{% image /a.png alt:hello %}\n{% unknown arg:a %}\n"
	out, ds, _ := migrateMarkdown(src, "test", nil, nil)
	for _, expected := range []string{"```md\n{% links %}", "`{% moments %}`", "    {% about %}", "{% blog_moments limit:5 %}", "{% blog_tip text:hello %}body{% endblog_tip %}", "{% image /a.png alt:hello  fancybox:false %}", "{% unknown arg:a %}"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("missing %q", expected)
		}
	}
	if !hasCode(ds, "unknown_shortcode") {
		t.Fatal("unknown tag silently accepted")
	}
	inline := "{% image /a.png fancybox:true %}"
	got, _, _ := migrateMarkdown(inline, "test", nil, nil)
	if got != inline {
		t.Fatal("explicit image behavior altered")
	}
	got, ds, _ = migrateMarkdown("{% links\narg %}", "test", nil, nil)
	if got != "{% links\narg %}" || !hasCode(ds, "ambiguous_shortcode") {
		t.Fatal("ambiguous syntax rewritten")
	}
}

func TestTrackedFileFullCandidateNumbersAndUnknownFields(t *testing.T) {
	path := fixture(t, `CREATE TABLE settings(section TEXT PRIMARY KEY,value TEXT);`)
	file := ";;;\n{\"title\":\"Title\",\"comments\":{\"enabled\":false},\"blog\":{\"custom\":{\"n\":9007199254740993,\"nil\":null}},\"unknown\":false}\n;;;\n{% links %}\n"
	c := candidate(t, snapshot(t, path, map[string]string{"content/posts/test.md": file}))
	p := c.Records[0]
	if !strings.Contains(p.BlogJSON, "9007199254740993") || !strings.Contains(p.BlogJSON, `"nil":null`) || !strings.Contains(p.ConfigJSON, `"unknown":false`) || !hasCode(p.Diagnostics, "unknown_field") {
		t.Fatal("file field/number preservation failed")
	}
	if p.Markdown != "{% blog_links %}\n" {
		t.Fatal("file body candidate not produced")
	}
}

func TestPrivateArtifactNeverOverwritesOrFollowsSymlink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "candidate.json")
	if err := WritePrivate(path, map[string]any{"number": json.Number("9007199254740993")}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatal("private artifact permissions")
	}
	if WritePrivate(path, map[string]any{}) == nil {
		t.Fatal("existing candidate overwritten")
	}
	link := filepath.Join(t.TempDir(), "link.json")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if WritePrivate(link, map[string]any{}) == nil {
		t.Fatal("symlink followed")
	}
}

func TestThemeMappingStrictFalseNullAndRemovedPolicies(t *testing.T) {
	path := fixture(t, `CREATE TABLE settings(section TEXT PRIMARY KEY,value TEXT);INSERT INTO settings VALUES('style','{"prefers_theme":"dark","color":{"accent":"#ff0000"}}');INSERT INTO settings VALUES('plugins','{"heti":{"enable":false},"scrollreveal":{"enable":false,"distance":"60px","duration":600,"interval":100}}');INSERT INTO settings VALUES('comments','{"enabled":false,"notify":{"password":"PRIVATE_VALUE"}}');INSERT INTO settings VALUES('ai','{"summary_enabled":false,"allow_user_api":true,"qa_enabled":false,"qa_allow_user_llm":true,"api_key":"PRIVATE_VALUE"}');INSERT INTO settings VALUES('article','{"tags":null,"indent":false,"share":{"enabled":true,"platforms":["wechat","weibo","link"]}}');`)
	s := snapshot(t, path, nil)
	c := candidate(t, s)
	if err := ValidateCandidate(c, s); err != nil {
		t.Fatal(err)
	}
	v, _ := decode(c.Theme.ThemeJSON)
	theme := object(v)
	for p, want := range map[string]any{"appearance.color_scheme": "dark", "features.heti.enabled": false, "features.reveal.distance": json.Number("60"), "article.paragraph_indent": "never"} {
		got, exists := get(theme, p)
		if !exists || encode(got) != encode(want) {
			t.Fatalf("mapping %s", p)
		}
	}
	v, _ = decode(c.Theme.BlogJSON)
	blog := object(v)
	for p, want := range map[string]any{"native_comments.enabled": false, "ai.summary.enabled": false, "ai.summary.allow_user_api": true, "ai.ask.enabled": false, "ai.ask.allow_user_llm": true, "content.enabled": true} {
		got, exists := get(blog, p)
		if !exists || got != want {
			t.Fatalf("extension mapping %s", p)
		}
	}
	if strings.Contains(encode(c.PublicReport()), "PRIVATE_VALUE") {
		t.Fatal("secret leaked")
	}
}
func TestWikiShelfUsesOfficialIDArrayAndDiagnosesSourceConflict(t *testing.T) {
	path := fixture(t, `CREATE TABLE settings(section TEXT PRIMARY KEY,value TEXT);INSERT INTO settings VALUES('wiki','[{"id":"docs","name":"Docs","title":"Old title"}]');CREATE TABLE wiki_projects(id TEXT PRIMARY KEY,name TEXT,title TEXT,sort INTEGER,pin INTEGER,visible INTEGER);INSERT INTO wiki_projects VALUES('docs','Docs','New title',1,0,1);`)
	c := candidate(t, snapshot(t, path, nil))
	v, _ := decode(c.Theme.BlogJSON)
	got, exists := get(object(v), "data.wiki")
	if !exists || encode(got) != `["docs"]` {
		t.Fatal("official shelf IDs absent")
	}
	if _, exists := get(object(v), "data.wiki_shelf"); exists {
		t.Fatal("parallel shelf protocol invented")
	}
	if !hasCode(c.Theme.Diagnostics, "wiki_shelf_definition_conflict") {
		t.Fatal("different shelf and collection titles silently overwritten")
	}
}
func TestUnknownVersionIsNotSilentlyDowngraded(t *testing.T) {
	path := fixture(t, `CREATE TABLE posts(id INTEGER PRIMARY KEY,slug TEXT,title TEXT,content_md TEXT,stellar_version INTEGER);INSERT INTO posts VALUES(1,'a','Title','body',3);`)
	s := snapshot(t, path, nil)
	c := candidate(t, s)
	if !hasCode(c.Records[0].Diagnostics, "unsupported_content_version") || ValidateCandidate(c, s) == nil {
		t.Fatal("future content version overwritten")
	}
}
func TestPartialMigrationMetadataIsPreservedAndBlocksOverwrite(t *testing.T) {
	path := fixture(t, `CREATE TABLE posts(id INTEGER PRIMARY KEY,slug TEXT,title TEXT,content_md TEXT,stellar_version INTEGER,stellar_config TEXT,blog_extensions TEXT);INSERT INTO posts VALUES(1,'a','Title','body',0,'{"unknown":9007199254740993}','{"native_comments":{"enabled":false}}');`)
	s := snapshot(t, path, nil)
	c := candidate(t, s)
	if !hasCode(c.Records[0].Diagnostics, "unversioned_metadata") || ValidateCandidate(c, s) == nil {
		t.Fatal("partial migration metadata silently overwritten")
	}
	if s.Tables["posts"].Rows[0]["stellar_config"] != `{"unknown":9007199254740993}` {
		t.Fatal("original partial metadata lost")
	}
}
func TestMarkdownNestedCodeAndUnmatchedBacktick(t *testing.T) {
	src := " > ```md\n > {% links %}\n > ```\n`unclosed {% moments %}\n{% image /a.png \"caption fancybox:true\" %}\n{% links \"unclosed %}\n"
	out, ds, _ := migrateMarkdown(src, "test", nil, nil)
	if !strings.Contains(out, " > {% links %}") || !strings.Contains(out, "`unclosed {% blog_moments %}") || !strings.Contains(out, `"caption fancybox:true"  fancybox:false`) || !strings.Contains(out, `{% links "unclosed %}`) || !hasCode(ds, "ambiguous_shortcode") {
		t.Fatalf("code/args preservation failed: %s", out)
	}
}
func TestMultilevelPriorityAndAllCategoryPaths(t *testing.T) {
	path := fixture(t, `CREATE TABLE posts(id INTEGER PRIMARY KEY,slug TEXT,title TEXT,content_md TEXT,pinned INTEGER,category TEXT);INSERT INTO posts VALUES(1,'a','Title','body',3,'first');CREATE TABLE categories(id INTEGER PRIMARY KEY,name TEXT,parent_id INTEGER);INSERT INTO categories VALUES(1,'Root',0),(2,'Child',1),(3,'Second',0);CREATE TABLE post_categories(post_id INTEGER,category_id INTEGER,position INTEGER);INSERT INTO post_categories VALUES(1,3,0),(1,2,1);`)
	c := candidate(t, snapshot(t, path, nil))
	v, _ := decode(c.Records[0].ConfigJSON)
	raw := object(v)
	priority, _ := get(raw, "listing.priority")
	if stringValue(priority) != "3" || encode(raw["categories"]) != `["Second","Root/Child"]` {
		t.Fatal("integer priority or complete category paths lost")
	}
}

// Explicitly opted-in evidence pass; normal tests never access real data.
func TestReadOnlySourcePreviewEvidence(t *testing.T) {
	path := os.Getenv("STELLAR_MIGRATION_SOURCE")
	if path == "" {
		t.Skip("optional read-only source preview")
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sidecarsBefore := fileFingerprints(t, path)
	files := map[string]string{}
	if root := os.Getenv("STELLAR_MIGRATION_CONTENT_ROOT"); root != "" {
		for _, name := range []string{"content/posts/stellar-143-acceptance.md", "content/posts/stellar-144-acceptance.md"} {
			content, e := os.ReadFile(filepath.Join(root, name))
			if e != nil {
				t.Fatal(e)
			}
			files[name] = string(content)
		}
	}
	s := snapshot(t, path, files)
	c := candidate(t, s)
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("read-only source bytes changed")
	}
	sidecarsAfter := fileFingerprints(t, path)
	if encode(sidecarsBefore) != encode(sidecarsAfter) {
		t.Fatal("source or SQLite sidecar changed")
	}
	if output := os.Getenv("STELLAR_MIGRATION_OUTPUT"); output != "" {
		if err = os.MkdirAll(output, 0700); err != nil {
			t.Fatal(err)
		}
		for name, value := range map[string]any{"candidate.private.json": c, "report.public.json": c.PublicReport(), "source-check.json": map[string]any{"source_digest": s.Digest, "file_sha256_before": bytesDigest(before), "file_sha256_after": bytesDigest(after), "sidecar_sha256_before": sidecarsBefore, "sidecar_sha256_after": sidecarsAfter, "unchanged": true}} {
			if err = WritePrivate(filepath.Join(output, name), value); err != nil {
				t.Fatal(err)
			}
		}
	}
	r := c.PublicReport()
	t.Logf("records=%d tracked=%d blockers=%d source_sha=%s", len(c.Records), len(s.Files), r.Blocking, s.Digest)
}
func fileFingerprints(t *testing.T, path string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		data, err := os.ReadFile(path + suffix)
		if os.IsNotExist(err) {
			out[suffix] = "absent"
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		out[suffix] = bytesDigest(data)
	}
	return out
}
