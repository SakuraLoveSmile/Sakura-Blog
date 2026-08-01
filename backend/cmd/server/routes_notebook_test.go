package main

// routes_notebook_test.go: 笔记本体系前台链路回归测试（Spec 35b）。
import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/post"
	"blog/internal/store"
	tmpl "blog/internal/template"
)

// newNotebookVerifyServer 最小 webServer：内存库 + embed 模板 + 空站点配置。
func newNotebookVerifyServer(t *testing.T) (*webServer, *store.Store) {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := st.EnsureNotebooksTable(); err != nil {
		t.Fatalf("ensure notebooks: %v", err)
	}
	if err := st.EnsureNotesTable(); err != nil {
		t.Fatalf("ensure notes: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	eng, err := tmpl.NewEngineFromEmbed(&tmpl.IconRegistry{})
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	site := &tmpl.Site{Title: "测试站点", Widgets: tmpl.WidgetsConfig{}}
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

func TestNotebookFrontendChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ws, st := newNotebookVerifyServer(t)
	// 笔记本 + 笔记（2 篇带标签，1 篇空标签）
	nb := &store.Notebook{Name: "go", Description: "Go 学习笔记", Cover: "https://example.com/cover.png"}
	if err := st.CreateNotebook(nb); err != nil {
		t.Fatal(err)
	}
	n1 := &store.Note{NotebookID: nb.ID, Title: "第一章", Content: "# 第一章", ContentHTML: "<h1>第一章</h1>", Tags: "Go, 基础"}
	if err := st.CreateNote(n1); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateNote(&store.Note{NotebookID: nb.ID, Title: "第二章", ContentHTML: "<p>two</p>", Tags: "Go"}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateNote(&store.Note{NotebookID: nb.ID, Title: "无标签笔记", ContentHTML: "<p>none</p>", Tags: ""}); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	ws.registerTaxonomyRoutes(r)

	// 1) /notebooks → 200 列表页（面包屑/卡片/笔记数/空态不出现）
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/notebooks", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/notebooks = %d, want 200", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{
		"layout=\"notebooks\"", "笔记本", "Go 学习笔记", "/notebooks/go", "共 3 篇笔记", "https://example.com/cover.png",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/notebooks page missing %q", want)
		}
	}

	// 2) /notebooks/go → 200 详情页（笔记列表 + tagtree widget 注入 + 标签高亮过滤链接）
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/notebooks/go", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/notebooks/go = %d, want 200", w.Code)
	}
	body = w.Body.String()
	for _, want := range []string{
		"layout=\"notebook\"", "widget-wrapper tree", "data-tagtree", "tree-list",
		"/note/", "第一章", "第二章", "无标签笔记", "/notebooks/go?tag=Go",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/notebooks/go page missing %q", want)
		}
	}
	// 3) ?tag=Go 过滤：只显示带 Go 标签的笔记（无标签笔记不出现）
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/notebooks/go?tag=Go", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/notebooks/go?tag=Go = %d, want 200", w.Code)
	}
	body = w.Body.String()
	if !strings.Contains(body, "第一章") || !strings.Contains(body, "第二章") {
		t.Errorf("tag filter page should contain Go-tagged notes")
	}
	if strings.Contains(body, "无标签笔记") {
		t.Errorf("tag filter page must not contain untagged notes")
	}
	// 4) /notebooks/nonexistent → 404；未知 tag → 404
	for _, p := range []string{"/notebooks/nonexistent", "/notebooks/go?tag=unknown"} {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, p, nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s = %d, want 404", p, w.Code)
		}
	}
	// 5) /note/{id} → 200 笔记页（面包屑/正文/标签/tagtree/上下篇）
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/note/"+itoa(n1.ID), nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/note/%d = %d, want 200", n1.ID, w.Code)
	}
	body = w.Body.String()
	for _, want := range []string{
		"layout=\"note\"", "<h1>第一章</h1>", "data-tagtree", "read-next", "/notebooks/go?tag=" + strings.ToLower(url.QueryEscape("基础")), "/note/" + itoa(n1.ID),
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/note page missing %q", want)
		}
	}
	// 6) /note/999999（不存在）→ 404；非数字 id → 404
	for _, p := range []string{"/note/999999", "/note/abc"} {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, p, nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s = %d, want 404", p, w.Code)
		}
	}
}

// itoa 简化 int64 → string（测试内使用）。
func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}
