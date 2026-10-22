package routes

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	tmpl "blog/internal/settings"
	"blog/internal/store"
)

// newTestHandler 构造 feed handler：临时 SQLite + 预置文章 + 固定 site（含 baseURL）。
func newTestHandler(t *testing.T, posts []*store.Post) *FeedHandler {
	t.Helper()
	st, err := store.New(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	ensureRouteSchema(t, st)
	for _, p := range posts {
		p.StellarVersion, p.BlogExtensions = 2, `{}`
		if p.StellarConfig == "" {
			p.StellarConfig = `{}`
		}
		p.ContentMD = p.ContentHTML
		if p.CreatedAt.IsZero() {
			p.CreatedAt = p.UpdatedAt
		}
		if _, err := st.Create(p); err != nil {
			t.Fatalf("create post: %v", err)
		}
	}
	sitePtr := &atomic.Pointer[tmpl.Site]{}
	site := canonicalRouteSite(t, "测试站", nil, nil)
	site.URL, site.Description = "https://example.com", "描述"
	sitePtr.Store(site)
	return NewFeedHandler(st, sitePtr)
}

func doGET(t *testing.T, h *FeedHandler, path string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.Register(r)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(w, req)
	return w
}

// TestFeedBodyPasswordProtected 密码保护文章完全排除公开 feed。
func TestFeedBodyPasswordProtected(t *testing.T) {
	h := newTestHandler(t, []*store.Post{{
		Slug: "secret", Title: "机密文章", Status: store.StatusPublished,
		Password: "pw123", ContentHTML: "<p>这是不该出现在 feed 里的正文</p>",
	}})
	atom := doGET(t, h, "/atom.xml")
	if strings.Contains(atom.Body.String(), "这是不该出现在 feed 里的正文") {
		t.Fatal("atom feed leaked password-protected content")
	}
	if atom.Code != http.StatusOK || strings.Contains(atom.Body.String(), "机密文章") || strings.Contains(atom.Body.String(), "/post/secret") {
		t.Fatalf("protected article must not be included: status=%d body=%s", atom.Code, atom.Body.String())
	}
	var atomDocument atomFeed
	if err := xml.Unmarshal(atom.Body.Bytes(), &atomDocument); err != nil {
		t.Fatal(err)
	}
	if len(atomDocument.Entries) != 0 {
		t.Fatal("protected atom entry must be excluded")
	}
	jf := doGET(t, h, "/feed.json")
	if jf.Code != http.StatusOK || strings.Contains(jf.Body.String(), "机密文章") || strings.Contains(jf.Body.String(), "这是不该出现在 feed 里的正文") {
		t.Fatal("json feed leaked password-protected content")
	}
	var jsonDocument jsonFeed
	if err := json.Unmarshal(jf.Body.Bytes(), &jsonDocument); err != nil {
		t.Fatal(err)
	}
	if len(jsonDocument.Items) != 0 {
		t.Fatal("protected JSON feed entry must be excluded")
	}
}

// TestAtomFeedStructure Atom XML 结构：条目、链接、缓存头、作者回退站点标题。
func TestAtomFeedStructure(t *testing.T) {
	h := newTestHandler(t, []*store.Post{
		{Slug: "a", Title: "文章甲", Status: store.StatusPublished, ContentHTML: "<p>甲</p>", CreatedAt: mustTime(t, "2026-08-01T00:00:00Z"), UpdatedAt: mustTime(t, "2026-08-02T00:00:00Z")},
		{Slug: "b", Title: "草稿乙", Status: store.StatusDraft, ContentHTML: "<p>乙</p>", CreatedAt: mustTime(t, "2026-08-03T00:00:00Z")},
	})
	w := doGET(t, h, "/atom.xml")
	if w.Code != http.StatusOK {
		t.Fatalf("atom status = %d", w.Code)
	}
	if ct := w.Header().Get("Cache-Control"); !strings.Contains(ct, "max-age=600") {
		t.Fatalf("atom cache-control = %q, want max-age=600", ct)
	}
	var feed atomFeed
	if err := xml.Unmarshal(w.Body.Bytes(), &feed); err != nil {
		t.Fatalf("unmarshal atom: %v", err)
	}
	if len(feed.Entries) != 1 {
		t.Fatalf("entries = %d, want 1（草稿不入 feed）", len(feed.Entries))
	}
	e := feed.Entries[0]
	if e.Title != "文章甲" || e.Link.Href != "https://example.com/post/a" || e.ID != "https://example.com/post/a" {
		t.Fatalf("entry mismatch: %+v", e)
	}
	if e.Author == nil || e.Author.Name != "测试站" {
		t.Fatalf("author fallback = %+v, want 站点标题", e.Author)
	}
	if !strings.Contains(e.Content.Body, "<p>甲</p>") {
		t.Fatalf("entry content missing: %q", e.Content.Body)
	}
}

// TestJSONFeedStructure JSON Feed 结构 + 缓存头。
func TestJSONFeedStructure(t *testing.T) {
	h := newTestHandler(t, []*store.Post{
		{Slug: "a", Title: "文章甲", Status: store.StatusPublished, ContentHTML: "<p>甲</p>", CreatedAt: mustTime(t, "2026-08-01T00:00:00Z")},
	})
	w := doGET(t, h, "/feed.json")
	if w.Code != http.StatusOK {
		t.Fatalf("json feed status = %d", w.Code)
	}
	if ct := w.Header().Get("Cache-Control"); !strings.Contains(ct, "max-age=600") {
		t.Fatalf("json feed cache-control = %q", ct)
	}
	var feed jsonFeed
	if err := json.Unmarshal(w.Body.Bytes(), &feed); err != nil {
		t.Fatalf("unmarshal json feed: %v", err)
	}
	if feed.Version != "https://jsonfeed.org/version/1.1" || feed.Title != "测试站" {
		t.Fatalf("feed meta mismatch: %+v", feed)
	}
	if len(feed.Items) != 1 || feed.Items[0].URL != "https://example.com/post/a" || feed.Items[0].Author != "测试站" {
		t.Fatalf("items mismatch: %+v", feed.Items)
	}
}

// TestCategoryRSS 分类 RSS：条目、Channel 元信息。
func TestCategoryRSS(t *testing.T) {
	h := newTestHandler(t, []*store.Post{
		{Slug: "a", Title: "文章甲", Category: "Go", StellarConfig: `{"categories":["Go"]}`, Status: store.StatusPublished, ContentHTML: "<p>甲</p>", CreatedAt: mustTime(t, "2026-08-01T00:00:00Z")},
	})
	w := doGET(t, h, "/categories/Go/rss.xml")
	if w.Code != http.StatusOK {
		t.Fatalf("category rss status = %d", w.Code)
	}
	var feed rssFeed
	if err := xml.Unmarshal(w.Body.Bytes(), &feed); err != nil {
		t.Fatalf("unmarshal rss: %v", err)
	}
	if feed.Version != "2.0" || feed.Channel.Title != "测试站 — 分类：Go" {
		t.Fatalf("rss meta mismatch: %+v", feed.Channel)
	}
	if len(feed.Channel.Items) != 1 || feed.Channel.Items[0].Guid != "https://example.com/post/a" {
		t.Fatalf("rss items mismatch: %+v", feed.Channel.Items)
	}
	// 不存在的分类返回空 feed（200 + 0 条目）
	w2 := doGET(t, h, "/categories/不存在/rss.xml")
	var empty rssFeed
	if err := xml.Unmarshal(w2.Body.Bytes(), &empty); err != nil {
		t.Fatalf("unmarshal empty rss: %v", err)
	}
	if len(empty.Channel.Items) != 0 {
		t.Fatalf("empty category should have no items, got %d", len(empty.Channel.Items))
	}
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %s: %v", s, err)
	}
	return v
}
