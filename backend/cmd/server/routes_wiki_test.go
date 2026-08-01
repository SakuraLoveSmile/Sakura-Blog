package main

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
	tmpl "blog/internal/template"
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
	eng, err := tmpl.NewEngineFromEmbed(&tmpl.IconRegistry{})
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	site := &tmpl.Site{
		Title:   "测试站点",
		Widgets: tmpl.WidgetsConfig{},
		Wiki: []tmpl.WikiProject{
			{ID: "stellar", Name: "stellar", Title: "Stellar 文档", Description: "d", Icon: "i", Tags: []string{"主题"}, URL: "", Sort: 1},
		},
	}
	ws := &webServer{
		site:      site,
		st:        st,
		posts:     post.NewService(filepath.Join(t.TempDir(), "posts")),
		engine:    eng,
		devMode:   true,
		uploadDir: t.TempDir(),
	}
	return ws, st
}

func TestWikiDocFrontendChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ws, st := newWikiVerifyServer(t)
	// 插入三级树 + 草稿
	intro := &store.WikiDoc{ProjectID: "stellar", Slug: "intro", Title: "引言", Order: 1, Parent: 0, ContentHTML: "<p>intro body</p>", Status: store.StatusPublished}
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
		"widget-wrapper tree", "tree-list", "tree-children", "tree-item active",
		"/wiki/stellar/setup", "/wiki/stellar/install", // 上下篇 + 树链接
	} {
		if !strings.Contains(body, want) {
			t.Errorf("wiki-doc page missing %q", want)
		}
	}
	if strings.Contains(body, "draft-only") {
		t.Errorf("wiki-doc page must not expose draft docs")
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
