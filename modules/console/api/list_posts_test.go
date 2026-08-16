package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"blog/internal/store"
)

// TestListPostsFilterPaginationViews（Spec 62）：listPosts SQL 下推后——
// status/category/tag 筛选、分页 total、views 随列表投影返回（无逐篇 GetViews）。
func TestListPostsFilterPaginationViews(t *testing.T) {
	r, st, user, _ := newFullEnv(t)
	cookie := authCookie(user)

	mk := func(slug, title, status, category, tags string) {
		t.Helper()
		if _, err := st.Create(&store.Post{
			Slug: slug, Title: title, Status: status,
			Category: category, Tags: tags,
			ContentMD: "# 标题", ContentHTML: "<h1>标题</h1>",
		}); err != nil {
			t.Fatalf("create %s: %v", slug, err)
		}
	}
	mk("go-1", "Go 文章一", store.StatusPublished, "技术", "Go,Gin")
	mk("go-2", "Go 文章二", store.StatusPublished, "技术", "Go")
	mk("go-draft", "Go 草稿", store.StatusDraft, "技术", "Go")
	mk("life", "生活", store.StatusPublished, "生活", "随笔")

	// 1) status 筛选：published 3 篇，total 精确
	w := do2FA(r, http.MethodGet, "/api/v1/posts?status=published&page=1&size=2", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("status filter: code = %d, body = %s", w.Code, w.Body.String())
	}
	var list struct {
		Items []postJSON `json:"items"`
		Total int        `json:"total"`
		Page  int        `json:"page"`
		Size  int        `json:"size"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if list.Total != 3 {
		t.Fatalf("published total = %d, want 3", list.Total)
	}
	if len(list.Items) != 2 || list.Page != 1 {
		t.Fatalf("page 1 size: %d items, page %d", len(list.Items), list.Page)
	}
	// 第 2 页应只剩 1 条
	w = do2FA(r, http.MethodGet, "/api/v1/posts?status=published&page=2&size=2", "", cookie)
	var page2 struct {
		Items []postJSON `json:"items"`
		Total int        `json:"total"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &page2)
	if page2.Total != 3 || len(page2.Items) != 1 {
		t.Fatalf("page 2: total %d items %d, want 3/1", page2.Total, len(page2.Items))
	}

	// 2) category 筛选
	w = do2FA(r, http.MethodGet, "/api/v1/posts?category=生活", "", cookie)
	if !jsonHas(w.Body.String(), `"total":1`) {
		t.Fatalf("category filter: %s", w.Body.String())
	}

	// 3) tag 筛选：逗号包裹 LIKE 精确匹配（Go 不误配 Golang 类标签）
	w = do2FA(r, http.MethodGet, "/api/v1/posts?tag=Go", "", cookie)
	if !jsonHas(w.Body.String(), `"total":3`) {
		t.Fatalf("tag filter total: %s", w.Body.String())
	}

	// 4) views 投影：自增后列表直接返回 views（不依赖逐篇 GetViews）
	if err := st.IncrViews("go-1"); err != nil {
		t.Fatalf("incr views: %v", err)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/posts?status=published", "", cookie)
	var viewsList struct {
		Items []postJSON `json:"items"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &viewsList)
	seen := false
	for _, it := range viewsList.Items {
		if it.Slug == "go-1" {
			if it.Views != 1 {
				t.Fatalf("go-1 views = %d, want 1", it.Views)
			}
			seen = true
		}
	}
	if !seen {
		t.Fatal("go-1 missing from list")
	}
}

// TestDashboardStatsCounts（Spec 62）：dashboardStats 返回计数且不再全表加载
// （仅验证响应正确性；是否触发大字段加载由实现保证 COUNT 化）。
func TestDashboardStatsCounts(t *testing.T) {
	r, st, user, _ := newFullEnv(t)
	cookie := authCookie(user)

	if _, err := st.Create(&store.Post{Slug: "a", Title: "A", Status: store.StatusPublished, ContentHTML: "<p>a</p>"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := st.Create(&store.Post{Slug: "b", Title: "B", Status: store.StatusDraft, ContentHTML: "<p>b</p>"}); err != nil {
		t.Fatalf("create: %v", err)
	}

	w := do2FA(r, http.MethodGet, "/api/v1/dashboard/stats", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("stats status = %d, body = %s", w.Code, w.Body.String())
	}
	var stats struct {
		PostTotal    int `json:"post_total"`
		Published    int `json:"post_published"`
		CommentTotal int `json:"comment_total"`
		ViewTotal    int `json:"view_total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if stats.PostTotal != 2 || stats.Published != 1 {
		t.Fatalf("counts mismatch: %+v", stats)
	}
}

// jsonHas 轻量断言：响应体包含子串（total 等数值字段格式稳定）。
func jsonHas(body, sub string) bool {
	return strings.Contains(body, sub)
}
