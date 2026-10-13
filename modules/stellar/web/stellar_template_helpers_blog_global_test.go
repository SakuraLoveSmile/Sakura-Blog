package web

import (
	"encoding/json"
	"html/template"
	"reflect"
	"strings"
	"testing"
	"time"

	"blog/internal/settings"
	"blog/internal/stellarmodels"
)

func globalBlogFixture(t *testing.T, layout string, config map[string]any) *StellarRenderData {
	t.Helper()
	host := prepareFixture(t, layout)
	host.Site.StellarV2.Blog = map[string]any{"content": config}
	host.Stellar = &settings.StellarRenderContext{Version: 2, Profile: layout, Input: stellarmodels.Input{Page: map[string]any{"title": "Fixture", "layout": layout, "path": "fixture", "source": "posts/fixture.md", "permalink": host.Page.CanonicalURL, "content": "<p>Body</p>", "date": time.Now().AddDate(-2, 0, 0), "updated": time.Now().AddDate(-2, 0, 0), "previous": map[string]any{"title": "Previous", "path": "previous"}, "next": map[string]any{"title": "Next", "path": "next"}}, FrontMatter: map[string]any{"cover": "/cover.png"}}}
	d, err := PrepareStellar(host)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func renderGlobalBlogFixture(t *testing.T, d *StellarRenderData) string {
	t.Helper()
	e, err := NewEngineFromEmbed(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, region := range []string{"topbar", "leftbar", "rightbar"} {
		if prepGet(d.Stellar.Regions, region+".enabled") == false {
			continue
		}
		d.Region = region
		body, err := e.RenderValue("partial/stellar/region_body", d)
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(body) != "" {
			d.Stellar.RegionBodies[region] = template.HTML(body)
		} else {
			delete(d.Stellar.RegionBodies, region)
		}
	}
	d.Region = ""
	out, err := e.RenderValue("layout/stellar", d)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func TestStellarBlogGlobalProjectionDropsUnknownSecrets(t *testing.T) {
	raw := map[string]any{"enabled": true, "api_key": "never-public", "banner_ratio": json.Number("2.75"), "reward": map[string]any{"enabled": true, "text": "Donate", "password": "never-public"}, "ai_label": map[string]any{"default": "reviewed", "reviewed": map[string]any{"color": "abc", "icon": "/label.svg", "api_key": "never-public"}, "provider_key": "never-public"}, "nav_tabs": map[string]any{"items": []any{map[string]any{"title": "Public", "url": "/public", "password": "never-public"}}, "token": "never-public"}}
	d := globalBlogFixture(t, "post", raw)
	public := stellarMap(stellarGet(d.Stellar.Data, "blog.content"))
	encoded, _ := json.Marshal(public)
	if strings.Contains(string(encoded), "never-public") || strings.Contains(string(encoded), "api_key") {
		t.Fatal("open extension bag leaked")
	}
	if public["banner_ratio"] != json.Number("2.75") || stellarGet(public, "ai_label.reviewed.icon") != "/label.svg" {
		t.Fatal("public field lost")
	}
	prepMap(public["reward"])["text"] = "Changed"
	if prepGet(raw, "reward.text") != "Donate" {
		t.Fatal("published source mutated")
	}
}

func TestStellarBlogReadNextAndExpirationActualArticle(t *testing.T) {
	d := globalBlogFixture(t, "post", map[string]any{"read_next": false, "expire_days": 30})
	before := prepCloneMap(d.Stellar.Model.Render)
	out := renderGlobalBlogFixture(t, d)
	if strings.Contains(out, `id="read-next"`) || !strings.Contains(out, `data-blog-expired`) || !strings.Contains(out, "may be outdated") {
		t.Fatal("article extension not consumed")
	}
	if !reflect.DeepEqual(before, d.Stellar.Model.Render) {
		t.Fatal("model mutated")
	}
	delete(stellarMap(stellarGet(d.Stellar.Data, "blog.content")), "read_next")
	if !strings.Contains(renderGlobalBlogFixture(t, d), `id="read-next"`) {
		t.Fatal("official omitted behavior changed")
	}
	d.Stellar.Model.Collection["profile"] = "wiki"
	if !strings.Contains(renderGlobalBlogFixture(t, d), `id="read-next"`) {
		t.Fatal("Wiki navigation was disabled")
	}
	for language, phrase := range map[string]string{"zh-CN": "信息可能已经过时", "zh-TW": "資訊可能已經過時"} {
		d.Stellar.Model.Collection["profile"] = "post"
		d.Stellar.Language = language
		if !strings.Contains(renderGlobalBlogFixture(t, d), phrase) {
			t.Fatalf("missing locale %s", language)
		}
	}
}

func TestStellarBlogAILabelCustomizationAndOfficialFallback(t *testing.T) {
	d := globalBlogFixture(t, "post", map[string]any{"ai_label": map[string]any{"default": "reviewed", "reviewed": map[string]any{"color": "123abc", "icon": "/custom-label.svg"}}})
	out := renderGlobalBlogFixture(t, d)
	// Background banners deliberately retain adaptive text colors.
	if !strings.Contains(out, `src="/custom-label.svg"`) || !strings.Contains(out, "ai-label") || strings.Contains(out, `color:#123abc`) {
		t.Fatal("custom default label or adaptive contrast missing")
	}
	stellarMap(d.Stellar.Model.Item["presentation"])["banner"] = map[string]any{"background": false}
	out = renderGlobalBlogFixture(t, d)
	if !strings.Contains(out, `color:#123abc`) {
		t.Fatal("custom foreground not consumed")
	}
	stellarMap(stellarGet(d.Stellar.Data, "blog.content.ai_label"))["reviewed"] = map[string]any{"color": "red;background:url(https://bad.test)", "icon": ""}
	if strings.Contains(renderGlobalBlogFixture(t, d), "bad.test") {
		t.Fatal("AI label CSS injection")
	}
	delete(stellarMap(stellarGet(d.Stellar.Data, "blog.content")), "ai_label")
	if strings.Contains(renderGlobalBlogFixture(t, d), "ai-label") {
		t.Fatal("omitted label must not create default")
	}
}

func TestStellarBlogBannerRatioAndEditSourceLongestPrefix(t *testing.T) {
	d := globalBlogFixture(t, "post", map[string]any{"banner_ratio": json.Number("2.75"), "edit_this_page": map[string]any{"posts/": "https://github.com/test/repo/blob/main/content/", "posts/fixture": "https://github.com/test/specific/blob/main/file"}})
	out := renderGlobalBlogFixture(t, d)
	for _, fragment := range []string{"--banner-ratio:2.75", `data-blog-edit`, `href="https://github.com/test/specific/blob/main/file.md"`, `data-api="https://api.github.com/repos/test/specific/commits?path=file.md"`} {
		if !strings.Contains(out, fragment) {
			t.Fatalf("missing %s", fragment)
		}
	}
	stellarMap(stellarGet(d.Stellar.Data, "blog.content"))["banner_ratio"] = "2;position:fixed"
	if strings.Contains(renderGlobalBlogFixture(t, d), "position:fixed") {
		t.Fatal("ratio CSS injection")
	}
	stellarMap(stellarGet(d.Stellar.Model.Render, "article.footer"))["contributor"] = map[string]any{"editUrl": "https://official.test/edit", "commitsUrl": "https://official.test/commits"}
	if strings.Contains(renderGlobalBlogFixture(t, d), "data-blog-edit") {
		t.Fatal("official contributor was overridden")
	}
}

func TestStellarBlogWelcomeOnlyNamedInstanceAndExplicitOverrides(t *testing.T) {
	d := globalBlogFixture(t, "index", map[string]any{"rightbar_intro": "Intro <unsafe>\nSecond line", "welcome_linklist": map[string]any{"columns": 1, "items": []any{map[string]any{"title": "Read", "url": "/read", "icon": "/read.svg"}}}})
	d.Stellar.RegionNames = "rightbar"
	d.Stellar.Regions["rightbar"] = map[string]any{"enabled": true}
	d.Stellar.Widgets["rightbar"] = []StellarWidget{{State: map[string]any{"layout": "markdown", "instanceId": "rightbar:welcome", "item": map[string]any{"title": "Welcome"}}, Widget: &settings.WidgetInstance{Name: "welcome"}}}
	out := renderGlobalBlogFixture(t, d)
	if !strings.Contains(out, "Intro &lt;unsafe&gt;") || !strings.Contains(out, `href="/blog/read"`) {
		t.Fatal("welcome extension not visible")
	}
	d.Stellar.Widgets["rightbar"][0].Widget.Name = "other"
	if strings.Contains(renderGlobalBlogFixture(t, d), "data-blog-intro") {
		t.Fatal("intro affected another widget")
	}
	d.Stellar.Widgets["rightbar"][0].Widget.Name = "welcome"
	stellarMap(d.Stellar.Widgets["rightbar"][0].State["item"])["linklist"] = nil
	if strings.Contains(renderGlobalBlogFixture(t, d), `href="/blog/read"`) {
		t.Fatal("explicit null linklist did not clear")
	}
}

func TestStellarBlogNavReplacementAndAppend(t *testing.T) {
	d := globalBlogFixture(t, "index", map[string]any{"nav_tabs": map[string]any{"enable_custom": true, "items": []any{map[string]any{"title": "Custom", "url": "/fixture", "icon": "/nav.svg", "active_rules": "/blog/fixture$"}, map[string]any{"title": "External", "url": "https://external.test/", "external": true}, map[string]any{"title": "Unsafe", "url": "javascript:alert(1)"}}}})
	out := renderGlobalBlogFixture(t, d)
	for _, fragment := range []string{`data-blog-nav class="active" href="/blog/fixture"`, `src="/nav.svg"`, `href="https://external.test/" target="_blank" rel="noopener noreferrer"`} {
		if !strings.Contains(out, fragment) {
			t.Fatalf("missing %s", fragment)
		}
	}
	if strings.Contains(out, "Unsafe") || strings.Contains(out, "Recent") {
		t.Fatal("custom replacement or safe URL failed")
	}
	stellarMap(stellarBlogGlobal(d, "nav_tabs"))["enable_custom"] = false
	if !strings.Contains(renderGlobalBlogFixture(t, d), "Recent") {
		t.Fatal("append removed official tabs")
	}
}

func TestStellarBlogAvatarAndTaxonomyIcons(t *testing.T) {
	d := globalBlogFixture(t, "index", map[string]any{"avatar_background": "/avatar-bg.png", "category_icons": map[string]any{"Go": "/go.svg"}, "tag_icons": map[string]any{"SSR": "/ssr.svg"}})
	d.Stellar.Regions["leftbar"] = map[string]any{"enabled": true, "brand": map[string]any{"name": "Site", "image": map[string]any{"src": "/avatar.png", "variant": "avatar"}}}
	out := renderGlobalBlogFixture(t, d)
	if !strings.Contains(out, `brand-image--animated`) || !strings.Contains(out, "/blog/avatar-bg.png") {
		t.Fatal("avatar background not consumed")
	}
	r := stellarTemplateRenderer{}
	card := string(r.postCard(d, map[string]any{"title": "Card", "categories": []any{"Go"}, "tags": []any{"SSR"}}, false, false))
	if !strings.Contains(card, `src="/go.svg"`) || !strings.Contains(card, `src="/ssr.svg"`) {
		t.Fatal("card icons missing")
	}
	d.Stellar.Data["index"] = map[string]any{"terms": []any{map[string]any{"name": "Go", "path": "/categories/go/", "count": 1}}}
	if !strings.Contains(string(r.categories(d)), `src="/go.svg"`) {
		t.Fatal("category index icon missing")
	}
	d.Stellar.Data["index"] = map[string]any{"terms": []any{map[string]any{"name": "SSR", "path": "/tags/ssr/", "count": 1}}}
	if !strings.Contains(string(r.tags(d)), `src="/ssr.svg"`) || !strings.Contains(string(r.articleTags(d, []any{map[string]any{"name": "SSR", "path": "/tags/ssr/"}})), `src="/ssr.svg"`) {
		t.Fatal("tag index/detail icon missing")
	}
}

func TestStellarBlogNotebookFooterDefaultsAreRenderedNotPersisted(t *testing.T) {
	for _, test := range []struct {
		name                         string
		pageFooter, collectionFooter map[string]any
		wantLicense                  string
		wantShare                    bool
	}{
		{"extension", nil, nil, "Custom license", false},
		{"page", map[string]any{"license": "Page license", "share": true}, nil, "Page license", true},
		{"collection", nil, map[string]any{"license": "Collection license", "share": true}, "Collection license", true},
		{"inherit", map[string]any{"license": nil, "share": nil}, nil, "<official>", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			host := prepareFixture(t, "note")
			host.Site.StellarV2.Blog = map[string]any{"content": map[string]any{"enabled": true, "notebook": map[string]any{"license": "Custom license", "share": false}, "share": map[string]any{"enabled": true, "platforms": []any{"link"}}}}
			host.Stellar = &settings.StellarRenderContext{Version: 2, Profile: "note", Input: stellarmodels.Input{Page: map[string]any{"title": "Note", "layout": "note", "path": "notebooks/book/note", "permalink": host.Page.CanonicalURL, "content": "<p>Note</p>"}, FrontMatter: map[string]any{"collection": map[string]any{"profile": "notebook", "id": "book"}}, CollectionID: "book", CollectionConfig: map[string]any{"name": "Book"}}}
			if test.pageFooter != nil {
				host.Stellar.Input.FrontMatter["footer"] = test.pageFooter
			}
			if test.collectionFooter != nil {
				host.Stellar.Input.CollectionConfig["footer"] = test.collectionFooter
			}
			before := prepCloneMap(host.Stellar.Input.FrontMatter)
			published := prepCloneMap(host.Site.StellarV2.Blog)
			d, err := PrepareStellar(host)
			if err != nil {
				t.Fatal(err)
			}
			footer := stellarMap(stellarRenderGet(d, "article.footer"))
			wantLicense := test.wantLicense
			if wantLicense == "<official>" {
				wantLicense = stellarText(prepGet(host.Site.StellarV2.Theme, "article.footer.license"))
			}
			if footer["license"] != wantLicense {
				t.Fatalf("license %v", footer["license"])
			}
			if stellarBlogNotebookShareAllowed(d) != test.wantShare {
				t.Fatal("share priority failed")
			}
			if !reflect.DeepEqual(before, host.Stellar.Input.FrontMatter) || !reflect.DeepEqual(published, host.Site.StellarV2.Blog) {
				t.Fatal("source mutated")
			}
			if test.wantShare == false {
				if strings.Contains(renderGlobalBlogFixture(t, d), `id="share"`) {
					t.Fatal("disabled notebook still renders extension share")
				}
			}
		})
	}
}

func TestStellarStrictHeadHostSEOPriorityAndManagedMeta(t *testing.T) {
	d := globalBlogFixture(t, "post", nil)
	d.Site.SEO = settings.SEOConfig{NoIndex: true, DefaultDescription: "Site SEO", DefaultKeywords: "go, ssr", GoogleVerification: "google<&", BingVerification: "bing", BaiduVerification: "baidu"}
	seo := stellarMap(d.Stellar.Model.Render["seo"])
	seo["robots"] = "index,nofollow"
	before := prepCloneMap(seo)
	out := renderGlobalBlogFixture(t, d)
	for _, fragment := range []string{`name="robots" content="noindex,nofollow"`, `name="description" content="Site SEO"`, `name="keywords" content="go, ssr"`, `property="og:description" content="Site SEO"`, `name="twitter:description" content="Site SEO"`, `"description":"Site SEO"`, `data-stellar-page-meta name="google-site-verification" content="google&lt;&amp;"`, `name="msvalidate.01" content="bing"`, `name="baidu-site-verification" content="baidu"`} {
		if !strings.Contains(out, fragment) {
			t.Fatalf("missing %s", fragment)
		}
	}
	d.Stellar.Data["page"] = map[string]any{"description": "Page SEO", "keywords": []any{"page"}}
	out = renderGlobalBlogFixture(t, d)
	if !strings.Contains(out, `name="description" content="Page SEO"`) || !strings.Contains(out, `name="keywords" content="page"`) || !strings.Contains(out, `"description":"Page SEO"`) {
		t.Fatal("page precedence inconsistent")
	}
	if !reflect.DeepEqual(before, seo) {
		t.Fatal("head mutated SEO snapshot")
	}
}
