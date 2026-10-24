package routes

import (
	"net/http"
	"strings"
	"testing"

	"blog/internal/config"
	"blog/internal/content"
	"blog/internal/store"
	stellarweb "blog/modules/stellar/web"
)

func TestStellarHostFriendsDefaultsUseActualEmbedWithoutChangingRecords(t *testing.T) {
	ws, st, _, router := stellarRouteFixture(t)
	site := *ws.site.Load()
	v := *site.StellarV2
	v.Blog = stellarCopyObject(v.Blog)
	v.Blog["pages"] = map[string]any{"friends": map[string]any{"title": "Host friends", "show_posts": false, "hero": map[string]any{"title": "Host hero", "description": "Host tagline", "background": "/host.png"}}}
	site.StellarV2 = &v
	if err := config.ApplyStellarBlogRuntime(&site); err != nil {
		t.Fatal(err)
	}
	ws.site.Store(&site)
	engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
	if err != nil {
		t.Fatal(err)
	}
	ws.engine = engine
	md := `{% friends api:https://example.test/friends.json posts:true %}`
	for _, slug := range []string{"friends", "unrelated"} {
		if _, err := st.CreatePage(&store.Page{Slug: slug, Title: "Stored title", ContentMD: md, Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{}`}); err != nil {
			t.Fatal(err)
		}
	}
	render := func(path string) string {
		t.Helper()
		w := stellarRequest(router, path)
		if w.Code != http.StatusOK {
			t.Fatal(w.Code, w.Body.String())
		}
		return w.Body.String()
	}
	got := render("/friends")
	for _, want := range []string{"Host friends", "Host hero", "Host tagline", "/host.png", `data-show-posts="false"`} {
		if !strings.Contains(got, want) {
			t.Errorf("friends HTML missing %s", want)
		}
	}
	preview, err := ws.renderContentPreview(content.StellarPreviewInput{StellarVersion: 2, StellarConfig: `{}`, Layout: "page", Path: "/friends", Title: "Stored title", HTML: `<div class="data-service ds-friends_and_posts" data-api="https://example.test/friends.json"><div class="grid-box"></div></div>`, RequestURL: "https://fixture.example/api/v1/preview", PreviewToken: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Host hero", "Host tagline", "/host.png", `data-show-posts="false"`} {
		if !strings.Contains(preview, want) {
			t.Errorf("preview missing %s", want)
		}
	}
	other := render("/unrelated")
	for _, absent := range []string{"Host hero", "/host.png", `data-show-posts="false"`} {
		if strings.Contains(other, absent) {
			t.Errorf("friends defaults leaked %s", absent)
		}
	}
	row, err := st.GetPageBySlug("friends")
	if err != nil || row.StellarConfig != `{}` || row.BlogExtensions != "" || row.Title != "Stored title" {
		t.Fatal("default rendering changed persisted content")
	}
	row.StellarConfig = `{"title":"Explicit title","banner":{"headline":"Explicit hero","tagline":"","background":false}}`
	row.BlogExtensions = `{"content":{"banner_image":""}}`
	if err := st.UpdatePage("friends", row); err != nil {
		t.Fatal(err)
	}
	got = render("/friends")
	if !strings.Contains(got, "Explicit hero") || !strings.Contains(got, "Explicit title") || strings.Contains(got, "Host hero") || strings.Contains(got, "Host tagline") || strings.Contains(got, "/host.png") {
		t.Fatal("explicit empty/false page fields did not suppress host defaults")
	}
}

func TestStellarHostRobotsUsesCanonicalExtensionAndThemeHost(t *testing.T) {
	ws, _, _, router := stellarRouteFixture(t)
	site := *ws.site.Load()
	v := *site.StellarV2
	v.Theme = stellarCopyObject(v.Theme)
	v.Theme["canonical"] = map[string]any{"host": "canonical.test", "allowedHosts": []any{"mirror.test"}}
	v.Blog = stellarCopyObject(v.Blog)
	v.Blog["seo"] = map[string]any{"robots_extra": "Disallow: /private\nCrawl-delay: 4"}
	site.StellarV2 = &v
	if err := config.ApplyStellarBlogRuntime(&site); err != nil {
		t.Fatal(err)
	}
	ws.site.Store(&site)
	w := stellarRequest(router, "/robots.txt")
	for _, want := range []string{"Sitemap: https://canonical.test/sitemap.xml", "Disallow: /private\nCrawl-delay: 4"} {
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), want) {
			t.Fatalf("robots missing %s: %d %s", want, w.Code, w.Body.String())
		}
	}
}
