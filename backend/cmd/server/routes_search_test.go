package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/search"
	"blog/internal/store"
)

func TestSearchItemsFromPosts(t *testing.T) {
	posts := []*store.Post{
		{Slug: "open", Title: "公开文章", ContentHTML: "<p>正文内容</p>"},
		{Slug: "locked", Title: "加密文章", ContentHTML: "<p>秘密</p>", Password: "pw"},
	}
	items := searchItemsFromPosts(posts)
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d", len(items))
	}
	if items[0].Path != "/post/open" || items[0].Content != "正文内容" {
		t.Errorf("open item mismatch: %+v", items[0])
	}
	// 受密码保护文章：标题保留、正文不泄漏
	if items[1].Title != "加密文章" || items[1].Content != "" {
		t.Errorf("protected post must not leak content: %+v", items[1])
	}
	if strings.Contains(items[1].Content, "秘密") {
		t.Error("protected content leaked")
	}
}

// TestAPISearchTypeFilter /api/search 支持 type 参数（post/page/moment），
// 返回条目携带 type 且路径按类型拼接；空 type 返回全部三类。
func TestAPISearchTypeFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	if err := st.EnsurePagesTable(); err != nil {
		t.Fatalf("ensure pages: %v", err)
	}
	if err := st.EnsureMomentsTable(); err != nil {
		t.Fatalf("ensure moments: %v", err)
	}
	idx := search.New(st.DB())
	if err := idx.Ensure(); err != nil {
		t.Fatalf("ensure fts: %v", err)
	}
	// 直接入索引（隔离端点测试点，不走写路径 hook）
	if err := idx.IndexDoc(search.TypePost, "p1", "文章标题", "文章正文含独特词甲"); err != nil {
		t.Fatal(err)
	}
	if err := idx.IndexDoc(search.TypePage, "about", "页面标题", "页面正文含独特词乙"); err != nil {
		t.Fatal(err)
	}
	if err := idx.IndexDoc(search.TypeMoment, "moment-1", "瞬间标题", "瞬间正文含独特词丙"); err != nil {
		t.Fatal(err)
	}

	ws := &webServer{st: st, searchIdx: idx}
	r := gin.New()
	ws.registerSearchRoutes(r)

	doSearch := func(q, docType string) []map[string]any {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/search?q="+q+"&type="+docType, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("search q=%s type=%s: status=%d body=%s", q, docType, w.Code, w.Body.String())
		}
		var body struct {
			Items []map[string]any `json:"items"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return body.Items
	}

	// 空 type：三类全出
	if items := doSearch("独特词", ""); len(items) != 3 {
		t.Fatalf("search all want 3, got %d", len(items))
	}
	// 各类型过滤 + 路径拼接 + type 字段
	if items := doSearch("独特词", "post"); len(items) != 1 || items[0]["type"] != "post" || items[0]["path"] != "/post/p1" {
		t.Errorf("post filter mismatch: %v", items)
	}
	if items := doSearch("独特词", "page"); len(items) != 1 || items[0]["type"] != "page" || items[0]["path"] != "/pages/about" {
		t.Errorf("page filter mismatch: %v", items)
	}
	if items := doSearch("独特词", "moment"); len(items) != 1 || items[0]["type"] != "moment" || items[0]["path"] != "/moments" {
		t.Errorf("moment filter mismatch: %v", items)
	}
	// 非法类型无结果（不报错）
	if items := doSearch("独特词", "wiki"); len(items) != 0 {
		t.Errorf("unknown type should be empty, got %d", len(items))
	}
}
