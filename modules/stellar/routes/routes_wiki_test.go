package routes

// routes_wiki_test.go: wiki 文档前台链路回归测试（Spec 35a）。
import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/post"
	"blog/internal/store"
	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"
)

// newWikiVerifyServer 最小 webServer：内存库 + embed 模板 + 空站点配置。
func newWikiVerifyServer(t *testing.T) (*webServer, *store.Store) {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := st.EnsureWikidocsTable(); err != nil {
		t.Fatalf("ensure wikidocs: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	eng, err := stellarweb.NewEngineFromEmbed(&tmpl.IconRegistry{})
	if err != nil {
		t.Fatalf("engine: %v", err)
	}

	site := &tmpl.Site{
		Title:   "测试站点",
		Widgets: tmpl.WidgetsConfig{},
		// 评论开关：TOC widget footer 的「参与讨论」按钮依赖它（对齐原版 comments.service 判断）
		Comments: tmpl.CommentsConfig{Enabled: true},
		Wiki: []tmpl.WikiProject{
			{ID: "stellar", Name: "stellar", Title: "Stellar 文档", Description: "d", Icon: "i", Tags: []string{"主题"}, URL: "", Sort: 1},
			// 第二个项目：与 stellar 共享 tag（wiki_related 推荐）+ 配置 repo（ghrepo 卡片）
			{ID: "go-blog", Name: "go-blog", Title: "Go 博客笔记", Description: "g", Icon: "i2", Tags: []string{"主题", "后端"}, URL: "", Sort: 2, Repo: "user/repo"},
		},
	}
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
	// 插入三级树 + 草稿
	intro := &store.WikiDoc{ProjectID: "stellar", Slug: "intro", Title: "引言", Order: 1, Parent: 0, ContentHTML: "<h2 id=\"intro-title\">引言</h2><p>intro body</p>", Status: store.StatusPublished}
	if err := st.CreateWikidoc(intro); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "stellar", Slug: "setup", Title: "安装", Order: 2, Parent: 0, ContentHTML: "<p>setup body</p>", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	// 子文档（验证 tree 递归层级）
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "stellar", Slug: "install", Title: "安装指南", Order: 1, Parent: intro.ID, ContentHTML: "<p>install body</p>", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "stellar", Slug: "draft-only", Title: "草稿", Order: 3, Parent: 0, ContentHTML: "<p>hidden</p>", Status: store.StatusDraft}); err != nil {
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
		"layout=\"wiki-doc\"", "Stellar 文档", "引言", "intro body",
		// doc-tree 结构（对齐原版 widgets/tree.ejs）：分组 header + body a.link + 当前文档高亮
		"widget-wrapper doc-tree post-list", "toc-text", "class=\"link active\"", "active-icon",
		"/wiki/stellar/setup", "/wiki/stellar/install", // 上下篇 + 树链接
		// 搜索框项目化（Task #4）：data-filter 限定当前项目 + placeholder 显示项目名
		`data-filter="/wiki/stellar/"`, "在 Stellar 文档 中搜索",
		// 同 tag 项目推荐（Task #4）：summary collection + "更多：{标签名}" + 推荐项目链接
		"widget-wrapper wiki-related", `data-layout="list"`, `data-variant="summary"`, "更多：主题", "/wiki/go-blog",
		// 最近更新（Task #8）：左栏 recent widget 渲染当前项目 wiki 文档
		"widget-wrapper recent",
		// 右栏 TOC widget：目录 + footer 回到顶部/参与讨论（对齐原版 toc.ejs layoutDiv）
		"widget-wrapper toc", `class="top" onclick="util.scrollTop()"`,
		`class="buttom" onclick="util.scrollComment()"`, "参与讨论",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("wiki-doc page missing %q", want)
		}
	}
	// 最近更新数据源为当前项目已发布文档（recent widget 之后出现 /wiki/stellar/ 链接）
	if recentIdx := strings.Index(body, "widget-wrapper recent"); recentIdx < 0 {
		t.Error("wiki-doc page missing recent widget")
	} else if !strings.Contains(body[recentIdx:], "/wiki/stellar/") {
		t.Error("recent widget must show current project wiki docs")
	}
	// 搜索框 data-filter 由 handleWikiDoc 注入（Task #4 A）
	if !strings.Contains(body, `data-filter="/wiki/stellar/"`) {
		t.Error("search input missing data-filter for current project")
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
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "layout=\"wiki\"") {
		t.Fatalf("/wiki/tags/主题 = %d, want 200 wiki list", w.Code)
	}
	// 4) project=tags 文档路由防御 → 404
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/wiki/tags/nonexistent-tag", nil))
	if w.Code != http.StatusOK {
		// /wiki/tags/:tag 静态段优先命中 handleWiki，未知 tag 自身 render404
		if w.Code != http.StatusNotFound {
			t.Fatalf("/wiki/tags/nonexistent-tag = %d, want 200/404", w.Code)
		}
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
	ws.site.Load().Wiki = append(ws.site.Load().Wiki, tmpl.WikiProject{
		ID: "readme", Title: "README 项目", Description: "项目默认说明", Repo: "owner/repo", Branch: "main",
	})
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "readme", Slug: "home", Title: "首页", Order: 1, Status: store.StatusPublished}); err != nil {
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
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "go-blog", Slug: "gb-intro", Title: "GB 引言", Order: 1, Parent: 0, ContentHTML: "<p>gb body</p>", Status: store.StatusPublished}); err != nil {
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
		// ghrepo 卡片：repo 参数从 WikiProject.Repo 注入（assembleWidgets case "ghrepo"）
		"widget-wrapper ghrepo", "/api/gh/widget/repo?repo=user/repo", "user/repo",
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
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "stellar", Slug: "intro", Title: "引言", Order: 1, Parent: 0, ContentHTML: "<p>intro body</p>", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "stellar", Slug: "draft-only", Title: "草稿", Order: 2, Parent: 0, ContentHTML: "<p>hidden</p>", Status: store.StatusDraft}); err != nil {
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
	if !strings.Contains(body, `"path":"/wiki/stellar/intro"`) || !strings.Contains(body, "引言") {
		t.Errorf("/search.json missing published wiki doc entry: %s", body)
	}
	if strings.Contains(body, "draft-only") {
		t.Error("/search.json must not include draft wiki docs")
	}
}

// TestWikiCardHrefSemantics 文库卡片跳转语义（Task #9）：
// 有已发布本地文档 → href=/wiki/<id>；无文档有 URL → 保持外链；都没有 → 不输出 href。
func TestWikiCardHrefSemantics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ws, st := newWikiVerifyServer(t)
	// 追加两个项目：ext（无文档，配置外链）、none（无文档且无 URL）
	ws.site.Load().Wiki = append(ws.site.Load().Wiki,
		tmpl.WikiProject{ID: "ext", Name: "ext", Title: "外部文档", Description: "d", Icon: "i", Tags: nil, URL: "https://example.com/docs", Sort: 0},
		tmpl.WikiProject{ID: "none", Name: "none", Title: "空项目", Description: "d", Icon: "i", Tags: nil, URL: "", Sort: -1},
	)
	// stellar 插入已发布文档 → HasDocs；none 仅草稿 → 不算 HasDocs
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "stellar", Slug: "intro", Title: "引言", Order: 1, Parent: 0, ContentHTML: "<p>body</p>", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "none", Slug: "draft", Title: "草稿", Order: 1, Parent: 0, ContentHTML: "<p>hidden</p>", Status: store.StatusDraft}); err != nil {
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
	// 有已发布文档 → 跳 /wiki/<id>（handleWikiProject 302 到首篇）
	if !strings.Contains(body, `class="wiki-card no-cover" href="/wiki/stellar"`) {
		t.Errorf("card stellar: want href=/wiki/stellar")
	}
	// 无文档 → 保持外链
	if !strings.Contains(body, `href="https://example.com/docs"`) {
		t.Errorf("card ext: want external URL href")
	}
	// 无文档且无 URL（仅草稿）→ 不进入项目卡片列表。
	if strings.Contains(body, "空项目") || strings.Contains(body, "/wiki/none") {
		t.Errorf("card none: empty project must not be rendered")
	}
}

func TestWikiProjectCardAndHeroTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ws, st := newWikiVerifyServer(t)
	if err := st.EnsureWikiProjectsTable(); err != nil {
		t.Fatalf("ensure wiki projects: %v", err)
	}
	if err := st.CreateWikiProject(&store.WikiProject{
		ID: "cover", Name: "cover", Title: "封面项目", Cover: "https://example.com/cover.jpg", Visible: true,
		Hero: &store.WikiHero{
			Background: "galaxy", Title: "项目 Hero", Description: "Hero 描述",
			Preview: &store.WikiHeroPreview{Type: "terminal", Commands: []store.WikiHeroCommand{{Codes: []string{"make dev"}}}},
		},
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "cover", Slug: "intro", Title: "项目引言", Status: store.StatusPublished}); err != nil {
		t.Fatalf("create project doc: %v", err)
	}
	r := gin.New()
	ws.registerTaxonomyRoutes(r)

	index := httptest.NewRecorder()
	r.ServeHTTP(index, httptest.NewRequest(http.MethodGet, "/wiki", nil))
	if index.Code != http.StatusOK {
		t.Fatalf("wiki index = %d, want 200", index.Code)
	}
	if !strings.Contains(index.Body.String(), `class="wiki-card" style="--cover-url:url('https://example.com/cover.jpg')"`) || !strings.Contains(index.Body.String(), `class="wiki-card-content"`) || !strings.Contains(index.Body.String(), `class="wiki-card-cover"`) || !strings.Contains(index.Body.String(), `data-text-adaptive="split"`) {
		t.Fatalf("cover project card missing adaptive markup: %s", index.Body.String())
	}

	detail := httptest.NewRecorder()
	r.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/wiki/cover/intro", nil))
	if detail.Code != http.StatusOK {
		t.Fatalf("wiki detail = %d, want 200", detail.Code)
	}
	for _, want := range []string{`class="l_cover wiki wiki-hero"`, `class="wiki-cover-background galaxy"`, `data-text-adaptive="split"`, "项目 Hero", "wiki-cover-terminal", "make dev"} {
		if !strings.Contains(detail.Body.String(), want) {
			t.Errorf("wiki detail missing %q", want)
		}
	}
}
