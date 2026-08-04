package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
	tmpl "blog/internal/template"
)

// newSearchTestRouter 组装含管理端搜索路由的测试引擎（password 为空则鉴权中间件直接放行）。
func newSearchTestRouter(t *testing.T, password string) (*gin.Engine, *store.Store) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	if err := st.EnsurePagesTable(); err != nil {
		t.Fatalf("ensure pages table: %v", err)
	}
	site := &tmpl.Site{}
	site.Admin.Password = password
	r := gin.New()
	New(Config{Store: st, Site: site}).Register(r)
	return r, st
}

func doSearch(t *testing.T, r *gin.Engine, q string) []searchResultItem {
	t.Helper()
	return doSearchType(t, r, q, "")
}

func doSearchType(t *testing.T, r *gin.Engine, q, docType string) []searchResultItem {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q="+q+"&type="+docType, nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("search %q: status = %d, want 200", q, w.Code)
	}
	var body struct {
		Items []searchResultItem `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return body.Items
}

// 未登录访问管理端搜索应返回 401。
func TestAdminSearchUnauthorized(t *testing.T) {
	r, _ := newSearchTestRouter(t, "secret")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=hello", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated search should get 401, got %d", w.Code)
	}
}

// 正常查询：草稿/已发布/回收站文章与页面均可命中，状态与类型正确。
func TestAdminSearchAllStatuses(t *testing.T) {
	r, st := newSearchTestRouter(t, "")
	seed := []*store.Post{
		{Slug: "pub", Title: "Go 并发实战", ContentMD: "x", ContentHTML: "<p>goroutine 调度器详解</p>", Status: store.StatusPublished},
		{Slug: "draft", Title: "Go 泛型草稿", ContentMD: "x", ContentHTML: "<p>类型参数上手</p>", Status: store.StatusDraft},
		{Slug: "trash", Title: "Go 旧文", ContentMD: "x", ContentHTML: "<p>待清理内容</p>", Status: store.StatusPublished},
	}
	for _, p := range seed {
		if _, err := st.Create(p); err != nil {
			t.Fatalf("create post %s: %v", p.Slug, err)
		}
	}
	if err := st.Delete("trash"); err != nil {
		t.Fatalf("trash post: %v", err)
	}
	if _, err := st.CreatePage(&store.Page{Slug: "about", Title: "关于 Go 博客", ContentHTML: "<p>站点介绍</p>", Status: store.StatusPublished}); err != nil {
		t.Fatalf("create page: %v", err)
	}

	items := doSearch(t, r, "Go")
	if len(items) != 4 {
		t.Fatalf("search Go: got %d items, want 4: %+v", len(items), items)
	}
	got := map[string]searchResultItem{}
	for _, it := range items {
		got[it.Type+"/"+it.Slug] = it
	}
	for key, wantStatus := range map[string]string{
		"post/pub":   "published",
		"post/draft": "draft",
		"post/trash": "trashed",
		"page/about": "published",
	} {
		it, ok := got[key]
		if !ok {
			t.Errorf("result %s missing", key)
			continue
		}
		if it.Status != wantStatus {
			t.Errorf("%s status = %q, want %q", key, it.Status, wantStatus)
		}
		if it.ID <= 0 || it.Title == "" {
			t.Errorf("%s should carry id/title: %+v", key, it)
		}
	}
}

// 正文命中返回 <mark> 高亮 snippet；标题命中排在正文命中之前。
func TestAdminSearchSnippetAndRanking(t *testing.T) {
	r, st := newSearchTestRouter(t, "")
	for _, p := range []*store.Post{
		{Slug: "body-hit", Title: "无关标题", ContentMD: "x", ContentHTML: "<p>正文提到调度器的细节</p>", Status: store.StatusPublished},
		{Slug: "title-hit", Title: "调度器揭秘", ContentMD: "x", ContentHTML: "<p>别的内容</p>", Status: store.StatusPublished},
	} {
		if _, err := st.Create(p); err != nil {
			t.Fatalf("create post %s: %v", p.Slug, err)
		}
	}
	items := doSearch(t, r, "调度器")
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2: %+v", len(items), items)
	}
	if items[0].Slug != "title-hit" {
		t.Errorf("title hit should rank first, got %q", items[0].Slug)
	}
	if !strings.Contains(items[1].Snippet, "<mark>调度器</mark>") {
		t.Errorf("body hit snippet should highlight term, got %q", items[1].Snippet)
	}
}

// 空结果与空查询：均返回 200 + 空 items 数组。
func TestAdminSearchEmpty(t *testing.T) {
	r, st := newSearchTestRouter(t, "")
	if _, err := st.Create(&store.Post{Slug: "a", Title: "hello", ContentMD: "x", ContentHTML: "<p>world</p>", Status: store.StatusPublished}); err != nil {
		t.Fatalf("create post: %v", err)
	}
	if items := doSearch(t, r, "nomatchkeyword"); len(items) != 0 {
		t.Errorf("no-match query should return empty items, got %+v", items)
	}
	if items := doSearch(t, r, ""); len(items) != 0 {
		t.Errorf("blank query should return empty items, got %+v", items)
	}
}

// TestAdminSearchTypeAndMoments type 参数过滤正确；瞬间可搜（公开/私密均可，状态取可见性）。
func TestAdminSearchTypeAndMoments(t *testing.T) {
	r, st := newSearchTestRouter(t, "")
	if err := st.EnsureMomentsTable(); err != nil {
		t.Fatalf("ensure moments table: %v", err)
	}
	if _, err := st.Create(&store.Post{Slug: "p1", Title: "Go 并发实战", ContentMD: "x", ContentHTML: "<p>goroutine 详解</p>", Status: store.StatusPublished}); err != nil {
		t.Fatalf("create post: %v", err)
	}
	if _, err := st.CreatePage(&store.Page{Slug: "about", Title: "关于 Go 博客", ContentHTML: "<p>站点介绍</p>", Status: store.StatusPublished}); err != nil {
		t.Fatalf("create page: %v", err)
	}
	if _, err := st.CreateMoment(&store.Moment{ContentHTML: "<p>Go 瞬间感悟</p>", Visibility: store.MomentVisibilityPublic}); err != nil {
		t.Fatalf("create moment: %v", err)
	}
	if _, err := st.CreateMoment(&store.Moment{ContentHTML: "<p>Go 私密瞬间</p>", Visibility: store.MomentVisibilityPrivate}); err != nil {
		t.Fatalf("create private moment: %v", err)
	}

	// 空 type：三类全出（含私密瞬间）
	items := doSearch(t, r, "Go")
	if len(items) != 4 {
		t.Fatalf("search all want 4, got %d: %+v", len(items), items)
	}
	// 各类型过滤：只回对应类型
	for _, tc := range []struct {
		docType string
		want    int
	}{
		{"post", 1},
		{"page", 1},
		{"moment", 2},
	} {
		items := doSearchType(t, r, "Go", tc.docType)
		if len(items) != tc.want {
			t.Errorf("type=%s want %d, got %d", tc.docType, tc.want, len(items))
			continue
		}
		for _, it := range items {
			if it.Type != tc.docType {
				t.Errorf("type=%s got item type=%s", tc.docType, it.Type)
			}
		}
	}
	// 瞬间条目标题截断正文、状态为可见性、slug 为 moment-<id> 伪 slug
	moments := doSearchType(t, r, "Go", "moment")
	for _, it := range moments {
		if it.Status != store.MomentVisibilityPublic && it.Status != store.MomentVisibilityPrivate {
			t.Errorf("moment status should be visibility, got %q", it.Status)
		}
		if !strings.HasPrefix(it.Slug, "moment-") || it.Title == "" {
			t.Errorf("moment slug/title mismatch: %+v", it)
		}
	}
}
