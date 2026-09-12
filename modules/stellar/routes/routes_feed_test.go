package routes

import (
	tmpl "blog/modules/stellar/viewmodel"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestFeedsUseCanonicalOriginalHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ws, st := newWikiVerifyServer(t)
	site := ws.site.Load()
	site.URL = "https://mirror.example/"
	site.SEO.Canonical.OriginalHost = "origin.example"
	if _, err := st.Create(&store.Post{Slug: "hello", Title: "Hello", Status: store.StatusPublished, CreatedAt: time.Now(), ContentHTML: "<p>body</p>"}); err != nil {
		t.Fatalf("create post: %v", err)
	}
	r := gin.New()
	ws.registerFeedRoutes(r)
	for _, path := range []string{"/rss.xml", "/sitemap.xml"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("%s status = %d", path, w.Code)
		}
		if strings.Contains(w.Body.String(), "mirror.example") || !strings.Contains(w.Body.String(), "https://origin.example") {
			t.Fatalf("%s did not consistently use canonical host: %s", path, w.Body.String())
		}
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

// 数据库 slug 是未编码文本，必须逐段编码，避免被解释成查询、片段或转义序列。
func TestCanonicalAndFeedsEscapeStoredSlugs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ws, st := newWikiVerifyServer(t)
	ws.site.Load().URL = "https://example.com/"
	router := gin.New()
	ws.registerFeedRoutes(router)
	for _, slug := range []string{"中文", "a#b", "a?b", "a%b"} {
		if _, err := st.Create(&store.Post{Slug: slug, Title: slug, Status: store.StatusPublished, ContentHTML: "<p>body</p>", CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	for _, slug := range []string{"中文", "a#b", "a?b", "a%b"} {
		path := "/post/" + url.PathEscape(slug)
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest(http.MethodGet, "https://example.com"+path, nil)
		data := &tmpl.ViewData{Site: ws.site.Load(), Page: &tmpl.Page{}}
		ws.fillSEO(data, ctx)
		want := "https://example.com" + path
		if data.Page.CanonicalURL != want {
			t.Fatalf("canonical = %q, want %q", data.Page.CanonicalURL, want)
		}
		for _, endpoint := range []string{"/rss.xml", "/atom.xml", "/feed.json", "/sitemap.xml"} {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, endpoint, nil))
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), want) {
				t.Errorf("%s missing encoded URL %q (status %d)", endpoint, want, response.Code)
			}
		}
	}
	// Wiki 项目和文档分别编码，不把分隔字符带入另一个路径段。
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "stellar", Slug: "文档#1", Title: "文档", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	ws.contentCache.invalidate()
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
	var sitemap struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(response.Body.Bytes(), &sitemap); err != nil {
		t.Fatal(err)
	}
	want := "https://example.com/wiki/stellar/" + url.PathEscape("文档#1")
	for _, entry := range sitemap.URLs {
		if entry.Loc == want {
			return
		}
	}
	t.Fatalf("sitemap missing wiki URL %q", want)
}
