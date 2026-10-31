package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/post"
	"blog/internal/store"
	tmpl "blog/modules/stellar/viewmodel"
)

func TestInvalidateSidebarCacheConcurrentWithReaders(t *testing.T) {
	ws := &webServer{
		sidebarCached: &sidebarSnapshot{},
		sidebarExpiry: time.Now().Add(time.Hour),
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			ws.sidebarMu.Lock()
			_ = ws.sidebarCached
			_ = ws.sidebarExpiry
			ws.sidebarMu.Unlock()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			ws.invalidateSidebarCache()
		}
	}()
	wg.Wait()

	ws.sidebarMu.Lock()
	defer ws.sidebarMu.Unlock()
	if !ws.sidebarExpiry.IsZero() {
		t.Fatalf("sidebar expiry = %v, want zero", ws.sidebarExpiry)
	}
}

func TestToCard(t *testing.T) {
	site := &tmpl.Site{}
	site.Article.CategoryColor = map[string]string{"技术": "ff0000"}
	site.Article.CategoryIcon = map[string]string{"技术": "default:category"}
	ws := newWebServer(webDeps{Site: sitePtr(site)})
	created := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	p := &store.Post{
		Slug: "hello", Title: "Hello", Excerpt: "ex", Cover: "/c.png",
		CreatedAt: created, Pinned: true, Category: "技术",
	}
	card := ws.toCard(p)
	if card.Slug != "hello" || card.Title != "Hello" || !card.Pinned || !card.Date.Equal(created) {
		t.Errorf("card basic fields mismatch: %+v", card)
	}
	if card.CategoryStyle == "" {
		t.Error("category with configured color should produce style")
	}
	if card.CategoryIcon != "default:category" {
		t.Errorf("category with configured icon = %q, want %q", card.CategoryIcon, "default:category")
	}
	if card.Poster != nil {
		t.Error("empty poster column should not produce poster card")
	}
	// 带海报：DecodePoster 命中触发 PosterCard；未配置图标分类回退 default:category
	p2 := &store.Post{Slug: "p2", Category: "随笔", Poster: store.EncodePoster(store.PosterData{Headline: "大字"})}
	c2 := ws.toCard(p2)
	if c2.Poster == nil || c2.Poster.Headline != "大字" {
		t.Errorf("poster card mismatch: %+v", c2.Poster)
	}
	if c2.CategoryIcon != "default:category" {
		t.Errorf("unconfigured category icon = %q, want default:category", c2.CategoryIcon)
	}
}

func TestArticleAutoExcerptAppliesToExistingPosts(t *testing.T) {
	site := &tmpl.Site{Article: tmpl.ArticleConfig{AutoExcerpt: 4}}
	ws := newWebServer(webDeps{Site: sitePtr(site)})
	card := ws.toCard(&store.Post{Title: "无摘要", ContentHTML: "<p>一二三四五六</p>"})
	if card.Excerpt != "一二三四…" {
		t.Fatalf("auto excerpt = %q, want %q", card.Excerpt, "一二三四…")
	}
	card = ws.toCard(&store.Post{Excerpt: "显式摘要", ContentHTML: "<p>正文</p>"})
	if card.Excerpt != "显式摘要" {
		t.Fatalf("explicit excerpt was replaced: %q", card.Excerpt)
	}
}

func TestArticleCardStyleDefaultsToHero(t *testing.T) {
	if got := (tmpl.ArticleConfig{}).EffectiveCardStyle(); got != "hero" {
		t.Fatalf("default card style = %q, want hero", got)
	}
	classic := "classic"
	if got := (tmpl.ArticleConfig{CardStyle: &classic}).EffectiveCardStyle(); got != "classic" {
		t.Fatalf("configured card style = %q, want classic", got)
	}
}

func TestCanonicalBaseURLUsesOriginalHost(t *testing.T) {
	site := &tmpl.Site{
		URL: "https://mirror.example",
		SEO: tmpl.SEOConfig{
			Canonical: tmpl.CanonicalConfig{OriginalHost: "origin.example", OfficialHosts: []string{"mirror.example"}},
		},
	}
	ws := newWebServer(webDeps{Site: sitePtr(site)})
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	c.Request = httptest.NewRequest(http.MethodGet, "http://mirror.example:8080/post/a", nil)
	if got := ws.canonicalBaseURL(&tmpl.ViewData{Site: site}, c); got != "https://origin.example" {
		t.Fatalf("canonical base = %q, want https://origin.example", got)
	}
}

func TestFillSEOCanonicalURLNoDoubleSlash(t *testing.T) {
	site := &tmpl.Site{
		URL: "https://origin.example/",
	}
	ws := newWebServer(webDeps{Site: sitePtr(site)})
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	c.Request = httptest.NewRequest(http.MethodGet, "/wiki", nil)

	data := &tmpl.ViewData{Site: site, Page: &tmpl.Page{}}
	ws.fillSEO(data, c)

	if got, want := data.Page.CanonicalURL, "https://origin.example/wiki"; got != want {
		t.Fatalf("fillSEO CanonicalURL = %q, want %q", got, want)
	}
}

func TestFillSEOCanonicalURLEscapesEncodedPath(t *testing.T) {
	site := &tmpl.Site{URL: "https://origin.example"}
	ws := newWebServer(webDeps{Site: sitePtr(site)})
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	c.Request = httptest.NewRequest(http.MethodGet, "https://mirror.example/post/%E4%B8%AD%2F%E6%96%87?view=full", nil)

	data := &tmpl.ViewData{Site: site, Page: &tmpl.Page{}}
	ws.fillSEO(data, c)
	if got, want := data.Page.CanonicalURL, "https://origin.example/post/%E4%B8%AD%2F%E6%96%87"; got != want {
		t.Fatalf("canonical URL = %q, want %q", got, want)
	}
}

func TestBaseURLRespectsXForwardedProto(t *testing.T) {
	site := &tmpl.Site{URL: ""}
	ws := newWebServer(webDeps{Site: sitePtr(site)})

	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	c.Request = httptest.NewRequest(http.MethodGet, "http://proxy.example/wiki", nil)
	c.Request.Header.Set("X-Forwarded-Proto", "https")
	c.Request.Host = "proxy.example"
	c.Set(requestSchemeKey, "https")

	if got, want := ws.baseURL(c), "https://proxy.example"; got != want {
		t.Fatalf("baseURL = %q, want %q", got, want)
	}

	data := &tmpl.ViewData{Site: site, Page: &tmpl.Page{}}
	ws.fillSEO(data, c)
	if got, want := data.Page.CanonicalURL, "https://proxy.example/wiki"; got != want {
		t.Fatalf("fillSEO CanonicalURL = %q, want %q", got, want)
	}
}

func TestPinSliderActiveOnlyOnHomeFirstPage(t *testing.T) {
	cases := []struct {
		name string
		data *tmpl.ViewData
		want bool
	}{
		{
			name: "home first page",
			data: &tmpl.ViewData{Page: &tmpl.Page{Layout: "index"}, Paginator: &tmpl.Paginator{Current: 1}},
			want: true,
		},
		{
			name: "home second page",
			data: &tmpl.ViewData{Page: &tmpl.Page{Layout: "index"}, Paginator: &tmpl.Paginator{Current: 2}},
			want: false,
		},
		{
			name: "category list",
			data: &tmpl.ViewData{Page: &tmpl.Page{Layout: "index", Category: "技术"}},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pinSliderReplacesPosts(tc.data); got != tc.want {
				t.Fatalf("pinSliderReplacesPosts = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBuildTagCloud(t *testing.T) {
	counts := map[string]int{"a": 1, "b": 3, "c": 3, "d": 2}
	items := buildTagCloud(counts, 3)
	if len(items) != 3 {
		t.Fatalf("limit not applied: got %d items", len(items))
	}
	// 数量降序，同量按名升序：b(3) c(3) d(2)
	if items[0].Name != "b" || items[1].Name != "c" || items[2].Name != "d" {
		t.Errorf("order mismatch: %s %s %s", items[0].Name, items[1].Name, items[2].Name)
	}
	for _, it := range items {
		if it.Size == 0 {
			t.Errorf("tag %s should have font size", it.Name)
		}
	}
}

// TestRenderSitePreview 设置页「前端预览」：草稿分段应用到 site 副本并渲染整页，
// 不影响全局共享 site（所见即所得不落库）。
func TestRenderSitePreview(t *testing.T) {
	ws, _ := newWikiVerifyServer(t)
	live := canonicalRouteSite(t, "线上标题", nil, nil)
	ws.site.Store(live)
	document, err := ws.renderSitePreview(map[string]json.RawMessage{"site": json.RawMessage(`{"title":"草稿标题"}`)}, "index")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(document, "草稿标题") {
		t.Error("draft title missing")
	}
	if live.Title != "线上标题" {
		t.Fatal("preview mutated live site")
	}
}

func TestRenderSitePreviewWidgetsOnlyKeepsLiveSiteSections(t *testing.T) {
	ws, _ := newWikiVerifyServer(t)
	live := canonicalRouteSite(t, "线上标题", nil, map[string]any{"native_comments": map[string]any{"enabled": true}})
	ws.site.Store(live)
	document, err := ws.renderSitePreview(map[string]json.RawMessage{
		"stellar_v2":      json.RawMessage(`{"version":2,"theme":{"profiles":{"home":{"leftbar":{"widgets":["__preview__"]}}}}}`),
		"blog_extensions": json.RawMessage(`{"native_comments":{"enabled":true},"data":{"widgets":{"__preview__":{"layout":"markdown","content":"预览组件"}}}}`),
	}, "index")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"线上标题", "预览组件"} {
		if !strings.Contains(document, want) {
			t.Errorf("widget preview missing %s", want)
		}
	}
	if live.StellarV2.Blog["native_comments"].(map[string]any)["enabled"] != true {
		t.Fatal("preview mutated live native comments")
	}
	if len(live.StellarV2.RawTheme) != 0 {
		t.Fatal("widget preview mutated live theme")
	}
	if strings.Contains(document, `id="blog-comments"`) {
		t.Fatal("preview must disable interactive native comments")
	}
}

func TestRenderSitePreviewWikiUsesFirstProjectTreeAndRelatedData(t *testing.T) {
	ws, st := newWikiVerifyServer(t)
	createRouteWikiProject(t, st, "first", map[string]any{"name": "首个项目", "tags": []any{"go"}, "listing": map[string]any{"order": 10}})
	createRouteWikiProject(t, st, "second", map[string]any{"name": "相关项目", "tags": []any{"go"}, "listing": map[string]any{"order": 1}})
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "first", Slug: "intro", Title: "首个项目文档", ContentMD: "说明", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`}); err != nil {
		t.Fatal(err)
	}
	for _, d := range []*store.WikiDoc{
		{ProjectID: "first", Slug: "setup", Title: "安装", ContentMD: "安装正文", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`},
		{ProjectID: "second", Slug: "overview", Title: "相关项目概览", ContentMD: "相关正文", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`},
	} {
		if err := st.CreateWikidoc(d); err != nil {
			t.Fatal(err)
		}
	}
	ws.site.Store(canonicalRouteSite(t, "文库预览", nil, map[string]any{"data": map[string]any{"wiki": []any{"first", "second"}}}))
	document, err := ws.renderSitePreview(nil, "wiki")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"首个项目文档", "相关项目", `data-wiki="first"`, "doc-tree-widget", "related-summary-widget"} {
		if !strings.Contains(document, want) {
			t.Errorf("wiki preview missing %s", want)
		}
	}
	index, err := ws.renderSitePreview(nil, "wiki-index")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"首个项目", "相关项目", `href="/wiki/first/intro"`, "wiki-card"} {
		if !strings.Contains(index, want) {
			t.Errorf("wiki index preview missing %s", want)
		}
	}
	doc, err := st.GetWikidocBySlug("first", "intro")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Title != "首个项目文档" || doc.ContentMD != "说明" {
		t.Fatal("wiki preview mutated content")
	}
}

func TestRenderSitePreviewPostUsesNewestAcrossStoredDateFormats(t *testing.T) {
	for _, tc := range []struct {
		name, imported, native, title string
	}{
		{"older-import", "2026-10-31T06:17:58Z", "2026-10-31T06:21:16Z", "Native article"},
		{"newer-import", "2026-10-31T06:21:16Z", "2026-10-31T06:17:58Z", "Imported article"},
		{"same-second-keeps-id-order", "2026-10-31T06:21:16.9Z", "2026-10-31T06:21:16.1Z", "Native article"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws, st, capture, _ := stellarRouteFixture(t)
			ws.site.Store(canonicalRouteSite(t, "Preview", nil, nil))
			for _, row := range []struct{ slug, title, date string }{
				{"imported", "Imported article", tc.imported},
				{"native", "Native article", tc.native},
			} {
				created, err := time.Parse(time.RFC3339Nano, row.date)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := st.Create(&store.Post{Slug: row.slug, Title: row.title, CreatedAt: created, Status: store.StatusPublished, ContentMD: row.title, StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`}); err != nil {
					t.Fatal(err)
				}
			}
			// portable 导入原样保存 RFC3339，普通新建由驱动保存 Go 时间文本。
			if _, err := st.DB().Exec(`UPDATE posts SET created_at=? WHERE slug='imported'`, tc.imported); err != nil {
				t.Fatal(err)
			}
			var before string
			if err := st.DB().QueryRow(`SELECT created_at FROM posts WHERE slug='native'`).Scan(&before); err != nil {
				t.Fatal(err)
			}
			if _, err := ws.renderSitePreview(nil, "post"); err != nil {
				t.Fatal(err)
			}
			if capture.last == nil {
				t.Fatal("preview did not render a page")
			}
			if capture.last.Page.Title != tc.title {
				t.Fatalf("preview selected %v, want %s", capture.last.Page.Title, tc.title)
			}
			var imported, native string
			if err := st.DB().QueryRow(`SELECT created_at FROM posts WHERE slug='imported'`).Scan(&imported); err != nil {
				t.Fatal(err)
			}
			if err := st.DB().QueryRow(`SELECT created_at FROM posts WHERE slug='native'`).Scan(&native); err != nil {
				t.Fatal(err)
			}
			if imported != tc.imported || native != before {
				t.Fatal("preview rewrote persisted timestamps")
			}
		})
	}
}

func TestRenderSitePreviewPostUsesDraftTOCDepth(t *testing.T) {
	ws, st := newWikiVerifyServer(t)
	if _, err := st.Create(&store.Post{Slug: "toc-preview", Title: "TOC 预览", Status: store.StatusPublished, ContentMD: `<h1 id="h1">一级标题</h1><h2 id="h2">二级标题</h2>`, StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`}); err != nil {
		t.Fatal(err)
	}
	ws.posts = post.NewService(filepath.Join(t.TempDir(), "posts"))
	site := canonicalRouteSite(t, "TOC 预览", map[string]any{"profiles": map[string]any{"post": map[string]any{"rightbar": map[string]any{"widgets": []any{"toc"}}}}}, map[string]any{"data": map[string]any{"widgets": map[string]any{"toc": map[string]any{"layout": "toc", "min_depth": 1}}}})
	ws.site.Store(site)
	baseline, err := ws.renderSitePreview(nil, "post")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(baseline, `href="#h1"`) || !strings.Contains(baseline, `href="#h2"`) {
		t.Fatal("baseline toc missing both headings")
	}
	document, err := ws.renderSitePreview(map[string]json.RawMessage{
		"blog_extensions": json.RawMessage(`{"data":{"widgets":{"toc":{"layout":"toc","min_depth":2}}}}`),
	}, "post")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(document, `href="#h1"`) || !strings.Contains(document, `href="#h2"`) {
		t.Error("draft min_depth=2 not applied")
	}
	after, err := ws.renderSitePreview(nil, "post")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(after, `href="#h1"`) {
		t.Fatal("draft mutated live toc config")
	}
}

func TestRenderSitePreviewRejectsLegacySections(t *testing.T) {
	ws, _ := newWikiVerifyServer(t)
	if _, err := ws.renderSitePreview(map[string]json.RawMessage{"widgets": json.RawMessage(`{}`)}, "index"); err == nil || !strings.Contains(err.Error(), "requires migration") {
		t.Fatalf("legacy preview error=%v", err)
	}
	ws.site.Load().StellarV2 = nil
	if _, err := ws.renderSitePreview(nil, "index"); err == nil {
		t.Fatal("legacy site preview must require migration")
	}
}

// 正式公开入口拒绝未迁移/无效内容，且不能借读取补写配置或内容版本。
func TestStrictPublicRouteMigrationGateDoesNotWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, invalid := range []bool{false, true} {
		t.Run(map[bool]string{false: "unmigrated", true: "invalid"}[invalid], func(t *testing.T) {
			ws, st := newWikiVerifyServer(t)
			p := &store.Post{Slug: "migration-gate", Title: "待迁移", ContentMD: "正文", Status: store.StatusPublished}
			if _, err := st.Create(p); err != nil {
				t.Fatal(err)
			}
			if invalid {
				if _, err := st.DB().Exec(`UPDATE posts SET stellar_version=2,stellar_config='{"pin":true,"sensitive":"fixture-secret"}',blog_extensions='{}' WHERE id=?`, p.ID); err != nil {
					t.Fatal(err)
				}
			}
			settingsBefore, err := st.ReadSettingsSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			before, err := st.GetBySlug(p.Slug)
			if err != nil {
				t.Fatal(err)
			}
			r := gin.New()
			ws.registerPostRoutes(r)
			ws.registerFeedRoutes(r)
			ws.registerSearchRoutes(r)
			for _, path := range []string{"/post/migration-gate", "/rss.xml", "/sitemap.xml", "/search.json"} {
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
				want := http.StatusConflict
				code := "stellar_migration_required"
				if invalid {
					want = http.StatusUnprocessableEntity
					code = "stellar_content_invalid"
				}
				if w.Code != want || !strings.Contains(w.Body.String(), code) {
					t.Fatalf("%s status=%d body=%s", path, w.Code, w.Body.String())
				}
				if strings.Contains(w.Body.String(), "fixture-secret") {
					t.Fatal("schema diagnostics leaked parameter value")
				}
			}
			settingsAfter, err := st.ReadSettingsSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			beforeJSON, _ := json.Marshal(settingsBefore)
			afterJSON, _ := json.Marshal(settingsAfter)
			if string(beforeJSON) != string(afterJSON) {
				t.Fatal("public read wrote settings migration")
			}
			after, err := st.GetBySlug(p.Slug)
			if err != nil {
				t.Fatal(err)
			}
			if after.StellarVersion != before.StellarVersion || after.StellarConfig != before.StellarConfig || after.ContentMD != before.ContentMD || after.Views != before.Views {
				t.Fatal("public read mutated migration candidate")
			}
		})
	}
}

func TestStrictPublicIndexesExcludeNonPublicContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ws, st := newWikiVerifyServer(t)
	for _, p := range []*store.Post{
		{Slug: "open", Title: "公开标题", ContentMD: "公开正文", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`},
		{Slug: "locked", Title: "密码标题", ContentMD: "密码正文", Password: "pw", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`},
		{Slug: "draft", Title: "草稿标题", ContentMD: "草稿正文", Status: store.StatusDraft},
		{Slug: "private", Title: "私密标题", ContentMD: "私密正文", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{"visibility":{"listed":false,"searchable":false}}`, BlogExtensions: `{}`},
	} {
		if _, err := st.Create(p); err != nil {
			t.Fatal(err)
		}
	}
	r := gin.New()
	ws.registerFeedRoutes(r)
	ws.registerSearchRoutes(r)
	for _, path := range []string{"/rss.xml", "/atom.xml", "/feed.json", "/sitemap.xml", "/search.json"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "/post/open") {
			t.Fatalf("%s status=%d body=%s", path, w.Code, w.Body.String())
		}
		for _, hidden := range []string{"/post/locked", "/post/draft", "/post/private", "密码正文", "草稿正文", "私密正文"} {
			if strings.Contains(w.Body.String(), hidden) {
				t.Errorf("%s leaked %s", path, hidden)
			}
		}
	}
}

func TestStrictPublicPageMigrationGateDoesNotWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ws, st := newWikiVerifyServer(t)
	if _, err := st.CreatePage(&store.Page{Slug: "legacy-page", Title: "旧页面", ContentMD: "待迁移正文", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	handler, err := NewPageHandler(st, ws.posts, ws.engine, ws.site)
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	handler.Register(r)
	before, err := st.GetPageBySlug("legacy-page")
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/pages/legacy-page", nil))
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "stellar_migration_required") {
		t.Fatalf("legacy page status=%d body=%s", w.Code, w.Body.String())
	}
	after, err := st.GetPageBySlug("legacy-page")
	if err != nil {
		t.Fatal(err)
	}
	if after.Views != before.Views || after.StellarVersion != before.StellarVersion || after.ContentMD != before.ContentMD {
		t.Fatal("migration rejection mutated legacy page")
	}
}
