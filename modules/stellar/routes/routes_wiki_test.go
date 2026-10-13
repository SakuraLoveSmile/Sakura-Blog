package routes

// routes_wiki_test.go: wiki 文档前台链路回归测试（Spec 35a）。
import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"

	"blog/internal/post"
	"blog/internal/store"
	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"
)

// newWikiVerifyServer 明确迁移过的主题与集合夹具；内容记录各自声明版本。
func newWikiVerifyServer(t *testing.T) (*webServer, *store.Store) {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	ensureRouteSchema(t, st)
	t.Cleanup(func() { st.Close() })
	eng, err := stellarweb.NewEngineFromEmbed(&tmpl.IconRegistry{})
	if err != nil {
		t.Fatalf("engine: %v", err)
	}

	site := canonicalRouteSite(t, "测试站点", map[string]any{
		"comments": map[string]any{"provider": "giscus"},
		"profiles": map[string]any{"wiki": map[string]any{"leftbar": map[string]any{"widgets": []any{"tree", "related", "recent"}}, "rightbar": map[string]any{"widgets": []any{"toc", "ghrepo"}}}},
	}, map[string]any{"data": map[string]any{"wiki": []any{"stellar", "go-blog"}}})
	createRouteWikiProject(t, st, "stellar", map[string]any{"name": "Stellar 文档", "description": "d", "tags": []any{"主题"}})
	createRouteWikiProject(t, st, "go-blog", map[string]any{"name": "Go 博客笔记", "description": "g", "tags": []any{"主题", "后端"}, "source": map[string]any{"repository": "user/repo"}})
	ws := &webServer{
		site:         sitePtr(site),
		st:           st,
		posts:        post.NewService(filepath.Join(t.TempDir(), "posts")),
		engine:       eng,
		devMode:      true,
		uploadDir:    t.TempDir(),
		contentCache: newContentCache(st, sitePtr(site)),
	}
	return ws, st
}

func TestWikiDocFrontendChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ws, st := newWikiVerifyServer(t)
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "go-blog", Slug: "overview", Title: "Go 概览", ContentMD: "简介", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`}); err != nil {
		t.Fatal(err)
	}
	// 插入三级树 + 草稿
	intro := &store.WikiDoc{StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`, ProjectID: "stellar", Slug: "intro", Title: "引言", Order: 1, Parent: 0, ContentMD: "<h2 id=\"intro-title\">引言</h2><p>intro body</p>", Status: store.StatusPublished}
	if err := st.CreateWikidoc(intro); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateWikidoc(&store.WikiDoc{StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`, ProjectID: "stellar", Slug: "setup", Title: "安装", Order: 2, Parent: 0, ContentMD: "<p>setup body</p>", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	// 子文档（验证 tree 递归层级）
	if err := st.CreateWikidoc(&store.WikiDoc{StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`, ProjectID: "stellar", Slug: "install", Title: "安装指南", Order: 1, Parent: intro.ID, ContentMD: "<p>install body</p>", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateWikidoc(&store.WikiDoc{StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`, ProjectID: "stellar", Slug: "draft-only", Title: "草稿", Order: 3, Parent: 0, ContentMD: "<p>hidden</p>", Status: store.StatusDraft}); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	ws.registerTaxonomyRoutes(r)
	// 1) /wiki/stellar → 302 到第一篇已发布文档
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/wiki/stellar", nil))
	if w.Code != http.StatusFound || !strings.HasSuffix(w.Header().Get("Location"), "/wiki/stellar/intro") {
		t.Fatalf("/wiki/stellar = %d loc=%q, want 302 → /wiki/stellar/intro", w.Code, w.Header().Get("Location"))
	}
	// 2) /wiki/stellar/intro → 200 wiki-doc 页（面包屑/正文/上下篇/tree widget 注入）
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/wiki/stellar/intro", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/wiki/stellar/intro = %d, want 200", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{
		`data-page-layout="wiki"`, "Stellar 文档", "引言", "intro body",
		// 官方 tree 的 collection item 和当前文档高亮。
		"doc-tree-widget", "toc-text", "is-active", "ui-collection__item",
		"/wiki/stellar/setup", "/wiki/stellar/install", // 上下篇 + 树链接
		// 官方搜索触发器携带当前项目 domain 与名称。
		`data-search-domain="wiki:stellar"`, "Stellar 文档",
		// 同 tag 项目推荐（Task #4）：summary collection + "更多：{标签名}" + 推荐项目链接
		"related-summary-widget", `data-layout="list"`, `data-variant="summary"`, "更多：主题", "/wiki/go-blog",
		// 最近更新（Task #8）：左栏 recent widget 渲染当前项目 wiki 文档
		"widget-instance--recent",
		// 右栏 TOC widget：目录 + footer 回到顶部/参与讨论（对齐原版 toc.ejs layoutDiv）
		`id="data-toc"`, `class="top ui-interactive" onclick="util.scrollTop()"`,
		`class="buttom ui-interactive" onclick="util.scrollComment()"`, "参与讨论",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("wiki-doc page missing %q", want)
		}
	}
	// 最近更新数据源为当前项目已发布文档（recent widget 之后出现 /wiki/stellar/ 链接）
	if recentIdx := strings.Index(body, "widget-instance--recent"); recentIdx < 0 {
		t.Error("wiki-doc page missing recent widget")
	} else if !strings.Contains(body[recentIdx:], "/wiki/stellar/") {
		t.Error("recent widget must show current project wiki docs")
	}
	// 搜索触发器的当前集合范围由严格模型提供。
	if !strings.Contains(body, `data-search-domain="wiki:stellar"`) {
		t.Error("search trigger missing current project domain")
	}
	if strings.Contains(body, "draft-only") {
		t.Errorf("wiki-doc page must not expose draft docs")
	}
	if strings.Contains(body, `widget-wrapper post-card`) || strings.Contains(body, `class="item wiki"`) {
		t.Errorf("wiki-related widget must not depend on legacy post-card/item markup: %s", body)
	}
	// 3) 路由安全：/wiki/tags/主题 仍走标签过滤页（handleWiki，Layout=wiki）
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/wiki/tags/%E4%B8%BB%E9%A2%98", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `data-page-layout="wiki"`) {
		t.Fatalf("/wiki/tags/主题 = %d, want 200 wiki list", w.Code)
	}
	// 4) project=tags 文档路由防御 → 404
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/wiki/tags/nonexistent-tag", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("/wiki/tags/nonexistent-tag = %d, want 404", w.Code)
	}
	// 5) 未配置项目 → 404；draft slug → 404
	for _, p := range []string{"/wiki/unknown-proj", "/wiki/stellar/draft-only"} {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, p, nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s = %d, want 404", p, w.Code)
		}
	}
}

func TestWikiReadmeFallback(t *testing.T) {
	project := &tmpl.WikiProject{Repo: "owner/repo", Branch: "docs/main"}
	html, ok := wikiReadmePlaceholder(project)
	if !ok {
		t.Fatal("valid wiki repo should produce README placeholder")
	}
	out := string(html)
	for _, want := range []string{
		`class="data-service ds-mdrender"`,
		`src="https://raw.githubusercontent.com/owner/repo/docs/main/README.md"`,
		`data-base="https://raw.githubusercontent.com/owner/repo/docs/main/"`,
		`data-replace="true"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("README placeholder missing %q: %s", want, out)
		}
	}
	for _, invalid := range []string{"owner", "owner/repo/extra", "owner/repo?<x>", "owner/repo"} {
		candidate := &tmpl.WikiProject{Repo: invalid}
		if invalid == "owner/repo" {
			candidate.Branch = "../escape"
		}
		if _, ok := wikiReadmePlaceholder(candidate); ok {
			t.Errorf("invalid repo/branch should not produce placeholder: %q", invalid)
		}
	}
}

func TestWikiDocHomepageUsesReadmeAndProjectDescription(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ws, st := newWikiVerifyServer(t)
	createRouteWikiProject(t, st, "readme", map[string]any{"name": "README 项目", "description": "项目默认说明", "source": map[string]any{"repository": "owner/repo", "branch": "main"}})
	if err := st.CreateWikidoc(&store.WikiDoc{StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`, ProjectID: "readme", Slug: "home", Title: "首页", Order: 1, Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	ws.registerTaxonomyRoutes(r)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/wiki/readme/home", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("README wiki page = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `src="https://raw.githubusercontent.com/owner/repo/main/README.md"`) {
		t.Fatalf("README homepage missing remote markdown placeholder: %s", body)
	}
	if !strings.Contains(body, `content="项目默认说明"`) {
		t.Fatalf("README homepage missing project description SEO fallback: %s", body)
	}
}

// TestWikiDocGhrepoWidget 项目配置 repo 时，wiki 文档页右栏渲染 ghrepo 卡片（Task #7）。
func TestWikiDocGhrepoWidget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ws, st := newWikiVerifyServer(t)
	if err := st.CreateWikidoc(&store.WikiDoc{StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`, ProjectID: "go-blog", Slug: "gb-intro", Title: "GB 引言", Order: 1, Parent: 0, ContentMD: "<p>gb body</p>", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "stellar", Slug: "overview", Title: "Stellar 概览", ContentMD: "简介", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`}); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	ws.registerTaxonomyRoutes(r)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/wiki/go-blog/gb-intro", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/wiki/go-blog/gb-intro = %d, want 200", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{
		// ghrepo 卡片使用 canonical source.repository 的官方 GitHub 服务。
		"ghrepo", `data-api="https://api.github.com/repos/user/repo"`, `href="https://github.com/user/repo"`,
		// wiki_related 推荐 stellar（共同 tag "主题"）
		"更多：主题", "/wiki/stellar",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("go-blog wiki-doc page missing %q", want)
		}
	}
}

// TestSearchJSONIncludesWikidocs /search.json 本地索引包含已发布 wiki 文档、排除 draft（Task #4 B）。
func TestSearchJSONIncludesWikidocs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ws, st := newWikiVerifyServer(t)
	if err := st.CreateWikidoc(&store.WikiDoc{StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`, ProjectID: "stellar", Slug: "intro", Title: "引言", Order: 1, Parent: 0, ContentMD: "<p>intro body</p>", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateWikidoc(&store.WikiDoc{StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`, ProjectID: "stellar", Slug: "draft-only", Title: "草稿", Order: 2, Parent: 0, ContentMD: "<p>hidden</p>", Status: store.StatusDraft}); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	ws.registerSearchRoutes(r)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/search.json", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/search.json = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"path":"/wiki/stellar/intro/"`) || !strings.Contains(body, "引言") {
		t.Errorf("/search.json missing published wiki doc entry: %s", body)
	}
	if strings.Contains(body, "draft-only") {
		t.Error("/search.json must not include draft wiki docs")
	}
}

// TestWikiCardHrefSemantics 文库卡片跳转语义（Task #9）：
// 已发布文档跳官方 homepage；显式列入 shelf 的空集合仍可见，未列入的集合不展示。
func TestWikiCardHrefSemantics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ws, st := newWikiVerifyServer(t)
	// 追加两个空集合，只有 shelf 上明确列出的集合可见。
	createRouteWikiProject(t, st, "listed-empty", map[string]any{"name": "陈列空项目"})
	createRouteWikiProject(t, st, "none", map[string]any{"name": "未陈列空项目"})
	ws.site.Load().StellarV2.Blog["data"].(map[string]any)["wiki"] = []any{"stellar", "go-blog", "listed-empty"}
	// stellar 插入已发布文档 → HasDocs；none 仅草稿 → 不算 HasDocs
	if err := st.CreateWikidoc(&store.WikiDoc{StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`, ProjectID: "stellar", Slug: "intro", Title: "引言", Order: 1, Parent: 0, ContentMD: "<p>body</p>", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateWikidoc(&store.WikiDoc{StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`, ProjectID: "none", Slug: "draft", Title: "草稿", Order: 1, Parent: 0, ContentMD: "<p>draft-body-do-not-expose</p>", Status: store.StatusDraft}); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	ws.registerTaxonomyRoutes(r)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/wiki", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/wiki = %d, want 200", w.Code)
	}
	body := w.Body.String()
	// 已发布文档使用模型生成的 homepage，而非旧中间跳转页。
	if !strings.Contains(body, `class="wiki-card no-cover card-hover card-hover--spotlight card-hover--tilt" href="/wiki/stellar/intro"`) {
		t.Errorf("card stellar: want href=/wiki/stellar/intro")
	}
	if !strings.Contains(body, "陈列空项目") {
		t.Error("explicitly listed empty collection must be visible")
	}
	if strings.Contains(body, "未陈列空项目") {
		t.Error("unlisted collection must not be visible")
	}
	if strings.Contains(body, "/wiki/none") || strings.Contains(body, "draft-body-do-not-expose") {
		t.Error("unlisted empty collection or draft body exposed")
	}

}

func TestWikiProjectCardAndHeroTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ws, st := newWikiVerifyServer(t)
	if err := st.EnsureWikiProjectsTable(); err != nil {
		t.Fatalf("ensure wiki projects: %v", err)
	}
	createRouteWikiProject(t, st, "cover", map[string]any{"name": "封面项目", "headline": "项目 Hero", "description": "Hero 描述", "cover": "https://example.com/cover.jpg", "hero": map[string]any{"enabled": true, "background": map[string]any{"effect": map[string]any{"type": "galaxy"}}, "preview": map[string]any{"type": "terminal", "commands": []any{map[string]any{"codes": "make dev"}}}}})
	ws.site.Load().StellarV2.Blog["data"].(map[string]any)["wiki"] = []any{"stellar", "go-blog", "cover"}
	if err := st.CreateWikidoc(&store.WikiDoc{StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`, ProjectID: "cover", Slug: "intro", Title: "项目引言", Status: store.StatusPublished}); err != nil {
		t.Fatalf("create project doc: %v", err)
	}
	r := gin.New()
	ws.registerTaxonomyRoutes(r)

	index := httptest.NewRecorder()
	r.ServeHTTP(index, httptest.NewRequest(http.MethodGet, "/wiki", nil))
	if index.Code != http.StatusOK {
		t.Fatalf("wiki index = %d, want 200", index.Code)
	}
	if !strings.Contains(index.Body.String(), `class="wiki-card card-hover card-hover--spotlight card-hover--tilt" style="--cover-url:url('https://example.com/cover.jpg')"`) || !strings.Contains(index.Body.String(), `class="wiki-card-content"`) || !strings.Contains(index.Body.String(), `class="wiki-card-cover"`) || !strings.Contains(index.Body.String(), `data-text-adaptive="split"`) {
		t.Fatalf("cover project card missing adaptive markup: %s", index.Body.String())
	}

	detail := httptest.NewRecorder()
	r.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/wiki/cover/intro", nil))
	if detail.Code != http.StatusOK {
		t.Fatalf("wiki detail = %d, want 200", detail.Code)
	}
	for _, want := range []string{`class="cover-content wiki wiki-hero"`, `data-hero-effect="galaxy"`, `data-text-adaptive="split"`, "项目 Hero", "wiki-cover-terminal", "make dev"} {
		if !strings.Contains(detail.Body.String(), want) {
			t.Errorf("wiki detail missing %q", want)
		}
	}
	// Hero 必须占据工作区之前的 cover slot，不能被正文栏或侧栏挤窄。
	document, err := html.Parse(strings.NewReader(detail.Body.String()))
	if err != nil {
		t.Fatalf("parse wiki document: %v", err)
	}
	var hero, cover, shell *html.Node
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		for _, attr := range node.Attr {
			if attr.Key == "id" && attr.Val == "site-cover" {
				cover = node
			}
			if attr.Key == "id" && attr.Val == "start" {
				shell = node
			}
			if attr.Key == "class" && strings.Contains(" "+attr.Val+" ", " wiki-hero ") {
				hero = node
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(document)
	if hero == nil || cover == nil || shell == nil || hero.Parent != cover || cover.Parent != shell.Parent {
		t.Fatal("Wiki Hero must be in the standalone cover slot outside the workspace")
	}
	for sibling := cover.NextSibling; sibling != nil; sibling = sibling.NextSibling {
		if sibling == shell {
			return
		}
	}
	t.Fatal("Wiki cover must precede the shell")
}

func TestLegacyWikiHeroBackgroundGalaxyFallback(t *testing.T) {
	got := normalizeLegacyWikiHero(&tmpl.HeroConfig{Background: " galaxy ", Title: "旧配置"})
	if got == nil || got.Background != "" || got.Animation == nil || got.Animation.Type != "galaxy" || got.Title != "旧配置" {
		t.Fatalf("legacy hero shorthand was not normalized: %#v", got)
	}
}
