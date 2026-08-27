package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

func TestRSSItemsFromPosts(t *testing.T) {
	created := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	long := strings.Repeat("长", 100) // 300 字节 UTF-8，触发 200 字节截断
	posts := []*store.Post{
		{Slug: "a", Title: "有摘要", Excerpt: "摘要", CreatedAt: created, Author: "sakura"},
		{Slug: "b", Title: "无摘要", ContentHTML: "<p>" + long + "</p>", CreatedAt: created, Author: "unknown"},
		{Slug: "c", Title: "加密", ContentHTML: "<p>秘密</p>", Password: "pw", CreatedAt: created},
	}
	// 模拟作者解析：已知作者返回名称，否则回退站点名
	resolveAuthor := func(postAuthor string) string {
		if postAuthor == "sakura" {
			return "樱花"
		}
		return "默认站点"
	}
	items := rssItemsFromPosts(posts, "https://example.com", resolveAuthor)
	if len(items) != 3 {
		t.Fatalf("want 3 items, got %d", len(items))
	}
	if items[0].Description != "摘要" || items[0].Link != "https://example.com/post/a" || items[0].Guid != items[0].Link {
		t.Errorf("item with excerpt mismatch: %+v", items[0])
	}
	if items[0].PubDate != created.Format(time.RFC1123Z) {
		t.Errorf("pubDate mismatch: %s", items[0].PubDate)
	}
	if len(items[1].Description) > 200 {
		t.Errorf("description should be truncated to 200 bytes, got %d", len(items[1].Description))
	}
	if items[2].Description != "本文受密码保护" {
		t.Errorf("protected post must hide content: %q", items[2].Description)
	}
	// 验证 author 字段：已知作者返回解析名，未知作者回退默认
	if items[0].Author != "樱花" {
		t.Errorf("author mismatch: got %q, want %q", items[0].Author, "樱花")
	}
	if items[1].Author != "默认站点" {
		t.Errorf("unknown author fallback: got %q, want %q", items[1].Author, "默认站点")
	}
}

// TestSitemapIncludesWikidocs sitemap 收录 wiki 已发布文档（Spec 35a）：
// 完整 URL（含 site URL 前缀）+ lastmod/changefreq/priority，draft 不收录。
func TestSitemapIncludesWikidocs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ws, st := newWikiVerifyServer(t)
	// published 文档
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "stellar", Slug: "intro", Title: "引言", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	// draft 文档不应被收录
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "stellar", Slug: "secret", Title: "草稿", Status: store.StatusDraft}); err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	ws.registerFeedRoutes(r)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("sitemap = %d, want 200", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{
		"<loc>http://example.com/wiki/stellar/intro</loc>",
		"<changefreq>weekly</changefreq>",
		"<priority>0.6</priority>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("sitemap missing %q", want)
		}
	}
	if !strings.Contains(body, "<lastmod>") {
		t.Errorf("sitemap wiki entry missing lastmod")
	}
	if strings.Contains(body, "/wiki/stellar/secret") {
		t.Errorf("sitemap must not include draft docs")
	}
}
