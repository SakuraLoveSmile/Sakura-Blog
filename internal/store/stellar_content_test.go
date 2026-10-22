package store

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"blog/internal/stellarv2"
)

const stellarPageRaw = ` {"title":"原值", "comments":{"enabled":false,"title":null},"leftbar":{"widgets":[{"custom":{"huge":900719925474099312345,"value":false,"none":null}}]}} `
const stellarCollectionRaw = ` {"name":"集合","comments":{"enabled":false,"title":null},"leftbar":{"widgets":[{"custom":{"huge":900719925474099312345,"value":false,"none":null}}]}} `
const stellarExtensionsRaw = ` {"poster":{"headline":"海报"},"huge":900719925474099312345,"value":false,"none":null} `

func newStellarStore(t *testing.T) *Store {
	t.Helper()
	s := newTestStore(t)
	if err := s.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	return s
}

func assertStellarRaw(t *testing.T, version int, config, extensions, want string) {
	t.Helper()
	if version != 2 || config != want || extensions != stellarExtensionsRaw {
		t.Fatalf("metadata changed: version=%d config=%q extensions=%q", version, config, extensions)
	}
}

func TestStellarContentRoundtripAndStrictWrites(t *testing.T) {
	s := newStellarStore(t)
	p := &Post{Slug: "v2", Title: "初稿", ContentMD: "正文", Status: StatusPublished, StellarVersion: 2, StellarConfig: stellarPageRaw, BlogExtensions: stellarExtensionsRaw}
	if _, err := s.Create(p); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetBySlug(p.Slug)
	if err != nil {
		t.Fatal(err)
	}
	assertStellarRaw(t, got.StellarVersion, got.StellarConfig, got.BlogExtensions, stellarPageRaw)
	result, err := got.ValidateStellar()
	if err != nil {
		t.Fatal(err)
	}
	if value, _ := stellarv2.Get(result.Value, "comments.enabled"); value != false {
		t.Fatalf("false lost: %#v", result)
	}
	if value, exists := stellarv2.Get(result.Value, "comments.title"); !exists || value != nil {
		t.Fatalf("null lost: %#v", result)
	}
	posts, _, err := s.ListFiltered("", "", "", "", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	assertStellarRaw(t, posts[0].StellarVersion, posts[0].StellarConfig, posts[0].BlogExtensions, stellarPageRaw)
	got.Title = "更新"
	got.ContentMD = "新正文"
	got.StellarConfig = `{"title":"更新","comments":{"enabled":false}}`
	if err := s.UpdateSlug(p.Slug, "v2-renamed", got); err != nil {
		t.Fatal(err)
	}
	current, _ := s.GetBySlug("v2-renamed")
	if current.ID != p.ID || current.StellarConfig != got.StellarConfig || current.ContentMD != "新正文" {
		t.Fatalf("post update = %#v", current)
	}
	invalid := *current
	invalid.Title = "不可写"
	invalid.ContentMD = "不能提交"
	invalid.StellarConfig = `{"topic":"legacy"}`
	if err := s.Update(current.Slug, &invalid); err == nil {
		t.Fatal("legacy config accepted")
	}
	unchanged, _ := s.GetBySlug(current.Slug)
	if unchanged.ContentMD != current.ContentMD || unchanged.StellarConfig != current.StellarConfig {
		t.Fatal("failed validation changed content")
	}
	invalid.StellarVersion = 0
	invalid.StellarConfig = ""
	invalid.BlogExtensions = ""
	if err := s.Update(current.Slug, &invalid); err == nil {
		t.Fatal("migrated post downgraded")
	}
	if _, err := s.Create(&Post{Slug: "invalid", StellarVersion: 2, StellarConfig: `{"wiki":"legacy"}`}); err == nil {
		t.Fatal("invalid post created")
	}
	if absent, _ := s.GetBySlug("invalid"); absent != nil {
		t.Fatal("invalid create wrote row")
	}

	page := &Page{Slug: "page-v2", Title: "页面", ContentMD: "页面正文", Status: StatusPublished, StellarVersion: 2, StellarConfig: stellarPageRaw, BlogExtensions: stellarExtensionsRaw}
	if _, err := s.CreatePage(page); err != nil {
		t.Fatal(err)
	}
	pagegot, _ := s.GetPageBySlug(page.Slug)
	assertStellarRaw(t, pagegot.StellarVersion, pagegot.StellarConfig, pagegot.BlogExtensions, stellarPageRaw)
	pages, _, err := s.ListPagesPage(1, 20, "", "")
	if err != nil {
		t.Fatal(err)
	}
	assertStellarRaw(t, pages[0].StellarVersion, pages[0].StellarConfig, pages[0].BlogExtensions, stellarPageRaw)
	pagegot.ContentMD = "新页面"
	if err := s.UpdatePage(page.Slug, pagegot); err != nil {
		t.Fatal(err)
	}
	pagegot.StellarVersion = 0
	pagegot.StellarConfig = ""
	pagegot.BlogExtensions = ""
	if err := s.UpdatePage(page.Slug, pagegot); err == nil {
		t.Fatal("migrated page downgraded")
	}

	wiki := &WikiProject{ID: "v2-project", Name: "项目", Visible: true, Tags: []string{"A", "B"}, StellarVersion: 2, StellarConfig: stellarCollectionRaw, BlogExtensions: stellarExtensionsRaw}
	if err := s.CreateWikiProject(wiki); err != nil {
		t.Fatal(err)
	}
	wikigot, _ := s.GetWikiProject(wiki.ID)
	assertStellarRaw(t, wikigot.StellarVersion, wikigot.StellarConfig, wikigot.BlogExtensions, stellarCollectionRaw)
	wikigot.Title = "新项目"
	if err := s.UpdateWikiProject(wiki.ID, wikigot); err != nil {
		t.Fatal(err)
	}
	wikis, err := s.ListWikiProjects()
	if err != nil {
		t.Fatal(err)
	}
	assertStellarRaw(t, wikis[0].StellarVersion, wikis[0].StellarConfig, wikis[0].BlogExtensions, stellarCollectionRaw)
	wikigot.StellarVersion = 0
	wikigot.StellarConfig = ""
	wikigot.BlogExtensions = ""
	if err := s.UpdateWikiProject(wiki.ID, wikigot); err == nil {
		t.Fatal("migrated wiki downgraded")
	}

	doc := &WikiDoc{ProjectID: wiki.ID, Slug: "intro", ContentMD: "文档", StellarVersion: 2, StellarConfig: stellarPageRaw, BlogExtensions: stellarExtensionsRaw}
	if err := s.CreateWikidoc(doc); err != nil {
		t.Fatal(err)
	}
	docgot, _ := s.GetWikidocBySlug(wiki.ID, doc.Slug)
	assertStellarRaw(t, docgot.StellarVersion, docgot.StellarConfig, docgot.BlogExtensions, stellarPageRaw)
	docgot.ContentMD = "新文档"
	if err := s.UpdateWikidoc(docgot); err != nil {
		t.Fatal(err)
	}
	docs, err := s.ListWikidocs(wiki.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertStellarRaw(t, docs[0].StellarVersion, docs[0].StellarConfig, docs[0].BlogExtensions, stellarPageRaw)
	docgot.StellarVersion = 0
	docgot.StellarConfig = ""
	docgot.BlogExtensions = ""
	if err := s.UpdateWikidoc(docgot); err == nil {
		t.Fatal("migrated wikidoc downgraded")
	}

	topic := &Topic{Name: "专题", Slug: "v2-topic", StellarVersion: 2, StellarConfig: stellarCollectionRaw, BlogExtensions: stellarExtensionsRaw}
	if err := s.CreateTopic(topic); err != nil {
		t.Fatal(err)
	}
	topicgot, _ := s.GetTopicByID(topic.ID)
	assertStellarRaw(t, topicgot.StellarVersion, topicgot.StellarConfig, topicgot.BlogExtensions, stellarCollectionRaw)
	topicgot.Description = "新描述"
	if err := s.UpdateTopic(topicgot); err != nil {
		t.Fatal(err)
	}
	topics, err := s.ListTopics()
	if err != nil {
		t.Fatal(err)
	}
	assertStellarRaw(t, topics[0].StellarVersion, topics[0].StellarConfig, topics[0].BlogExtensions, stellarCollectionRaw)
	topicgot.StellarVersion = 0
	topicgot.StellarConfig = ""
	topicgot.BlogExtensions = ""
	if err := s.UpdateTopic(topicgot); err == nil {
		t.Fatal("migrated topic downgraded")
	}
}

func TestStellarValidationIsExplicitAndPreservesProfileWarnings(t *testing.T) {
	for _, record := range []interface {
		ValidateStellar() (stellarv2.Result, error)
	}{&Post{}, &Page{}, &WikiDoc{}, &WikiProject{}, &Topic{}, &Notebook{}, &Note{}} {
		if _, err := record.ValidateStellar(); !errors.Is(err, ErrStellarMigrationRequired) {
			t.Fatalf("unmigrated %T: %v", record, err)
		}
	}
	p := &Page{StellarVersion: 2, StellarConfig: `{"listing":{"priority":1}}`}
	result, err := p.ValidateStellar()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Issues) != 1 || result.Issues[0].Severity != "warning" {
		t.Fatalf("profile issues: %#v", result)
	}
	if _, exists := stellarv2.Get(result.Value, "listing.priority"); exists {
		t.Fatal("unsupported page priority consumed")
	}
	for _, raw := range []string{"", `null`, `[]`, `{} {}`, `{"topic":"old"}`, `{"comments":{"enabled":"false"}}`} {
		p.StellarConfig = raw
		if _, err := p.ValidateStellar(); err == nil {
			t.Fatalf("invalid strict input %q", raw)
		}
	}
	p.StellarConfig = `{}`
	p.BlogExtensions = `null`
	if _, err := p.ValidateStellar(); err == nil {
		t.Fatal("invalid extensions accepted")
	}
}

func TestStellarSchemaUpgradeLeavesLegacyRecordsUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"posts", "pages", "wikidocs", "wiki_projects", "topics"} {
		for _, column := range []string{"stellar_version", "stellar_config", "blog_extensions"} {
			if _, err := s.db.Exec(`ALTER TABLE ` + table + ` DROP COLUMN ` + column); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := s.db.Exec(`DELETE FROM schema_migrations WHERE version=2;
		INSERT INTO posts(slug,title,content_md,poster,topic) VALUES('legacy','旧文章','{% tip text:注释 %}正文{% endtip %}','{"headline":"海报"}','旧专题');
		INSERT INTO pages(slug,content_md,leftbar_widgets) VALUES('legacy','旧页面','[]');
		INSERT INTO wikidocs(project_id,slug,content_md) VALUES('legacy','intro','旧文档');
		INSERT INTO wiki_projects(id,name,hero_json) VALUES('legacy','旧项目','{"background":"old"}');
		INSERT INTO topics(name,slug) VALUES('旧专题','legacy');`); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"posts", "pages", "wikidocs", "wiki_projects", "topics"} {
		var version int
		var raw, ext string
		if err := s.db.QueryRow(`SELECT stellar_version,stellar_config,blog_extensions FROM `+table).Scan(&version, &raw, &ext); err != nil {
			t.Fatal(err)
		}
		if version != 0 || raw != "" || ext != "" {
			t.Fatalf("%s automatically migrated", table)
		}
	}
	p, err := s.GetBySlug("legacy")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.ContentMD, "{% tip") || p.Poster != `{"headline":"海报"}` || p.Topic != "旧专题" {
		t.Fatalf("legacy post changed: %#v", p)
	}
	if _, err := p.ValidateStellar(); !errors.Is(err, ErrStellarMigrationRequired) {
		t.Fatal(err)
	}
	page, _ := s.GetPageBySlug("legacy")
	if page.LeftbarWidgets != "[]" || page.ContentMD != "旧页面" {
		t.Fatal("legacy page changed")
	}
}

func TestStellarContentAtomicFailure(t *testing.T) {
	s := newStellarStore(t)
	if _, err := s.db.Exec(`CREATE TRIGGER reject_stellar_categories BEFORE INSERT ON post_categories BEGIN SELECT RAISE(ABORT,'reject categories'); END;`); err != nil {
		t.Fatal(err)
	}
	p := &Post{Slug: "atomic", Title: "候选", ContentMD: "正文", Categories: []string{"A/B"}, StellarVersion: 2, StellarConfig: stellarPageRaw, BlogExtensions: stellarExtensionsRaw}
	if _, err := s.Create(p); err == nil {
		t.Fatal("expected category failure")
	}
	if row, _ := s.GetBySlug(p.Slug); row != nil {
		t.Fatal("failed create left content/metadata row")
	}
	if _, err := s.db.Exec(`DROP TRIGGER reject_stellar_categories`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(p); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER reject_stellar_categories BEFORE INSERT ON post_categories BEGIN SELECT RAISE(ABORT,'reject categories'); END;`); err != nil {
		t.Fatal(err)
	}
	p.ContentMD = "不可写正文"
	p.StellarConfig = `{}`
	if err := s.Update(p.Slug, p); err == nil {
		t.Fatal("expected category update failure")
	}
	row, _ := s.GetBySlug(p.Slug)
	if row.ContentMD != "正文" || row.StellarConfig != stellarPageRaw {
		t.Fatal("failed update changed content/metadata")
	}

	if _, err := s.db.Exec(`CREATE TRIGGER reject_stellar_wiki_tags BEFORE INSERT ON wiki_project_tags BEGIN SELECT RAISE(ABORT,'reject tags'); END;`); err != nil {
		t.Fatal(err)
	}
	wiki := &WikiProject{ID: "atomic", Tags: []string{"A"}, StellarVersion: 2, StellarConfig: stellarCollectionRaw, BlogExtensions: stellarExtensionsRaw}
	if err := s.CreateWikiProject(wiki); err == nil {
		t.Fatal("expected wiki tag failure")
	}
	if got, _ := s.GetWikiProject(wiki.ID); got != nil {
		t.Fatal("failed wiki create left metadata row")
	}
}
