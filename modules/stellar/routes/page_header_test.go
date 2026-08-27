package routes

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/post"
	settings "blog/internal/settings"
	"blog/internal/store"
	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"
)

func TestStandalonePageBannerSSR(t *testing.T) {
	root := t.TempDir()
	st, err := store.New(filepath.Join(root, "blog.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.EnsurePagesTable(); err != nil {
		t.Fatalf("ensure pages table: %v", err)
	}
	posts := post.NewService(filepath.Join(root, "posts"))
	engine, err := stellarweb.NewEngineFromEmbed(&tmpl.IconRegistry{})
	if err != nil {
		t.Fatalf("create template engine: %v", err)
	}
	site := &tmpl.Site{Title: "测试站", Authors: map[string]tmpl.AuthorDef{"alice": {Name: "Alice"}}}
	sitePtr := &atomic.Pointer[settings.Site]{}
	sitePtr.Store(site)
	h, err := NewPageHandler(st, posts, engine, sitePtr)
	if err != nil {
		t.Fatalf("create page handler: %v", err)
	}

	if _, err := st.CreatePage(&store.Page{
		Slug: "about", Title: "关于本站", ContentHTML: "<p>页面正文</p>", Status: store.StatusPublished,
		Banner: "https://example.test/about.jpg", BannerInfo: store.PageBannerInfo{Title: "关于横幅", Subtitle: "这是副标题", Avatar: "/avatar.png"},
		Breadcrumb: true, Author: "alice", PublishedAt: "2026-08-01",
	}); err != nil {
		t.Fatalf("create page: %v", err)
	}
	if _, err := st.CreatePage(&store.Page{
		Slug: "compact", Title: "紧凑页面", ContentHTML: "<p>紧凑正文</p>", Status: store.StatusPublished,
		BannerInfo: store.PageBannerInfo{Title: "紧凑横幅"}, Breadcrumb: false,
	}); err != nil {
		t.Fatalf("create compact page: %v", err)
	}

	r := gin.New()
	h.Register(r)
	request := func(path string) string {
		t.Helper()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s: status=%d body=%s", path, w.Code, w.Body.String())
		}
		return w.Body.String()
	}

	html := request("/pages/about")
	for _, want := range []string{
		`class="article-banner-wrap"`,
		`<div class="l_body content" id="start" layout="page"`,
		`--bg-url:url('https://example.test/about.jpg')`,
		`<img class="lazy bg" data-src="https://example.test/about.jpg" alt="">`,
		`<h1 class="text title"><span>关于横幅</span></h1>`,
		`<div class="text subtitle">这是副标题</div>`,
		`data-src="/avatar.png"`,
		`href="/about"`,
		`href="/author/alice/">Alice</a>`,
		`<time datetime="2026-08-01">2026-08-01</time>`,
		`<p>页面正文</p>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("ordinary page SSR missing %q", want)
		}
	}

	compact := request("/pages/compact")
	if strings.Contains(compact, `class="bread-nav footnote"`) {
		t.Fatal("breadcrumb=false should omit the banner breadcrumb/meta row")
	}
	if !strings.Contains(compact, `<p>紧凑正文</p>`) {
		t.Fatal("compact page body missing")
	}
}

func TestFriendsLegacyHeroMigratesOnceToPage(t *testing.T) {
	root := t.TempDir()
	st, err := store.New(filepath.Join(root, "blog.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.EnsurePagesTable(); err != nil {
		t.Fatalf("ensure pages table: %v", err)
	}
	site := &tmpl.Site{Pages: settings.PagesConfig{Friends: settings.PageFriendsConfig{Title: "旧友链标题", Hero: &settings.HeroConfig{Background: "/legacy.jpg", Description: "旧副标题"}}}}
	sitePtr := &atomic.Pointer[settings.Site]{}
	sitePtr.Store(site)
	h := &PageHandler{store: st, posts: post.NewService(filepath.Join(root, "posts")), site: sitePtr}
	if _, err := st.CreatePage(&store.Page{Slug: FriendsSlug, Title: "友链", ContentMD: defaultFriendsMD, ContentHTML: "<p>正文</p>", Status: store.StatusPublished, IsSystem: true, Breadcrumb: true}); err != nil {
		t.Fatalf("create friends page: %v", err)
	}
	if err := h.EnsureSystemFriendsPage(); err != nil {
		t.Fatalf("migrate friends hero: %v", err)
	}
	page, err := st.GetPageBySlug(FriendsSlug)
	if err != nil {
		t.Fatalf("get migrated friends page: %v", err)
	}
	if page.Banner != "/legacy.jpg" || page.BannerInfo.Title != "旧友链标题" || page.BannerInfo.Subtitle != "旧副标题" {
		t.Fatalf("legacy friends hero not migrated: %+v", page)
	}

	// 修改旧配置后再次启动不应覆盖 pages 中已迁移/编辑的值。
	site.Pages.Friends.Hero.Background = "/changed.jpg"
	if err := h.EnsureSystemFriendsPage(); err != nil {
		t.Fatalf("rerun friends migration: %v", err)
	}
	page, err = st.GetPageBySlug(FriendsSlug)
	if err != nil {
		t.Fatalf("get rerun friends page: %v", err)
	}
	if page.Banner != "/legacy.jpg" {
		t.Fatalf("legacy friends migration ran more than once: %q", page.Banner)
	}
	marker, err := st.GetSetting(legacyFriendsBannerMigration)
	if err != nil || marker != "1" {
		t.Fatalf("legacy migration marker = %q, err=%v", marker, err)
	}
}
