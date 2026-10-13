package web

import (
	"html/template"
	"reflect"
	"strings"
	"testing"

	"blog/internal/settings"
	"blog/internal/stellarhtml"
	"blog/internal/stellarimages"
	"blog/internal/stellarmodels"
)

func renderBlogFixture(t *testing.T, d *StellarRenderData) string {
	t.Helper()
	e, err := NewEngineFromEmbed(nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.RenderValue("layout/stellar", d)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func blogListingFixture() map[string]any {
	return map[string]any{"item": map[string]any{"id": "post/photo"}, "render": map[string]any{"listing": map[string]any{"title": "Canonical title", "href": "/post/photo", "cover": "/cover.jpg", "cardStyle": "classic"}}}
}

func TestStellarBlogPosterUsesPublicProjectionAndCanonicalCover(t *testing.T) {
	d := stellarTemplateFixture(t)
	d.Stellar.Layout = "index"
	d.Stellar.ListItems = []map[string]any{blogListingFixture()}
	d.Stellar.Data["itemBlogs"] = map[string]any{"post/photo": map[string]any{"content": map[string]any{"poster": map[string]any{"headline": "Poster <headline>", "topic": "Topic", "caption": "Caption", "color": "#123abc"}, "banner_image": "/banner-only.jpg"}}}
	before := prepCloneMap(d.Stellar.Data)
	out := renderBlogFixture(t, d)
	for _, fragment := range []string{`data-blog-poster`, `post photo`, `src="/cover.jpg"`, `class="text headline">Poster &lt;headline&gt;`, `class="text topic">Topic`, `class="text caption">Caption`, `top:0;bottom:auto;color:#123abc;`} {
		if !strings.Contains(out, fragment) {
			t.Fatalf("missing %s", fragment)
		}
	}
	if strings.Contains(out, "banner-only") || !reflect.DeepEqual(before, d.Stellar.Data) {
		t.Fatal("listing used banner or mutated projection")
	}
	delete(d.Stellar.Data, "itemBlogs")
	out = renderBlogFixture(t, d)
	if strings.Contains(out, "data-blog-poster") || !strings.Contains(out, "Canonical title") {
		t.Fatal("ordinary card changed")
	}
}

func TestStellarBlogBannerAndWikiExternalOnlyChangePresentation(t *testing.T) {
	d := stellarTemplateFixture(t)
	d.Stellar.Root = "/sub/"
	d.Page.CanonicalURL = "https://example.test/sub/page"
	d.Stellar.Data["page"] = map[string]any{"cover": "/canonical.jpg"}
	d.Stellar.Data["recordBlog"] = map[string]any{"content": map[string]any{"banner_image": "/detail.jpg"}}
	out := renderBlogFixture(t, d)
	if !strings.Contains(out, "/sub/detail.jpg") || !strings.Contains(out, d.Page.CanonicalURL) {
		t.Fatal("detail banner or canonical lost")
	}
	d.Stellar.Layout, d.Stellar.Profile = "index_wiki", "wikiIndex"
	listing := map[string]any{"id": "docs", "href": "/wiki/docs/", "title": "Docs", "priority": 1}
	d.Stellar.Collections = []map[string]any{listing}
	d.Stellar.Data["allWikiItems"] = []any{listing}
	d.Stellar.Data["collectionBlogs"] = map[string]any{"wiki/docs": map[string]any{"content": map[string]any{"wiki_external_url": "https://docs.example.test/guide"}}}
	before := prepCloneMap(listing)
	out = renderBlogFixture(t, d)
	if strings.Count(out, `href="https://docs.example.test/guide"`) != 2 || !reflect.DeepEqual(before, listing) {
		t.Fatal("wiki cards failed or canonical model changed")
	}
	for _, invalid := range []string{"javascript:alert(1)", "//other.test/x", `/\other.test`, "https://x.test/\n"} {
		if stellarBlogContentURL(d.Stellar.Root, invalid) != "" {
			t.Fatalf("accepted unsafe URL %q", invalid)
		}
	}
	if stellarBlogPosterColor("red;background:url(https://evil.test)") != "" {
		t.Fatal("accepted CSS injection")
	}
}

func TestStellarBlogAuthorsDeduplicatePublicIDs(t *testing.T) {
	d := stellarTemplateFixture(t)
	d.Stellar.Data["recordBlog"] = map[string]any{"content": map[string]any{"authors": []any{"primary", "extra", "unknown", "extra"}}}
	primary := map[string]any{"name": "Primary", "path": "/author/primary/"}
	d.Stellar.Data["defaultAuthor"] = primary
	d.Stellar.Data["page"] = map[string]any{"article": map[string]any{"author": "primary"}}
	d.Stellar.Data["authors"] = map[string]any{"primary": primary, "extra": map[string]any{"name": "Extra", "path": "/author/extra/"}}
	out := renderBlogFixture(t, d)
	if strings.Count(out, `href="/author/extra/"`) != 1 || strings.Count(out, `href="/author/primary/"`) != 1 || strings.Contains(out, "unknown") {
		t.Fatal("author order or public filtering failed")
	}
}

func TestStellarBlogMomentsLocalesGalleryAndNewPaginator(t *testing.T) {
	for _, language := range []string{"zh-CN", "zh-TW", "en"} {
		d := stellarTemplateFixture(t)
		d.Stellar.Layout, d.Stellar.Language, d.Stellar.Root = "moments", language, "/sub/"
		d.Page.Title = "Moment page"
		d.Paginator = &settings.Paginator{Current: 2, Total: 3, Pages: []int{1, 2, 3}}
		d.Moments = []*settings.MomentItem{{ID: 7, ContentHTML: template.HTML("<p>Public moment</p>"), Date: "2026-10-13", TimeAgo: "now", Images: []string{"/one.jpg", "/two.jpg"}}}
		before := prepCloneMap(d.Stellar.Data)
		out := renderBlogFixture(t, d)
		for _, f := range []string{`tag-plugin timeline`, `class="timenode"`, `<p>Public moment</p>`, `gallery grid-box" size="xl" ratio="square"`, `data-fancybox="moment-7"`, `data-src="/sub/one.jpg"`, `href="/sub/moments/"`, `href="/sub/moments/page/3/"`, stellarBlogMomentsLabel(language, "label")} {
			if !strings.Contains(out, f) {
				t.Fatalf("%s missing %s", language, f)
			}
		}
		if !reflect.DeepEqual(before, d.Stellar.Data) {
			t.Fatal("moments mutated runtime data")
		}
		d.Moments = nil
		out = renderBlogFixture(t, d)
		if !strings.Contains(out, stellarBlogMomentsLabel(language, "empty")) {
			t.Fatal("missing localized empty state")
		}
	}
	data := prepareFixture(t, "moments")
	data.Stellar = &settings.StellarRenderContext{Version: 2, Profile: "page", Input: stellarmodels.Input{Page: map[string]any{"layout": "moments"}}}
	prepared, err := PrepareStellar(data)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Stellar.PageType != "content" {
		t.Fatal("moments must be content")
	}
}

func TestStellarBlogMomentAvatarKeepsCompactDimensionsAfterImageEnrichment(t *testing.T) {
	for _, dimensions := range []stellarimages.Dimensions{
		{Status: "ready", Width: 512, Height: 512},
		{Status: "ready", Width: 1024, Height: 256},
	} {
		d := stellarTemplateFixture(t)
		d.Stellar.Layout = "moments"
		d.Site.Avatar = "/avatar.png"
		d.Site.Title, d.Site.LogoTitle = "Stellar 2.0 验收站", ""
		d.Moments = []*settings.MomentItem{{ID: 7, ContentHTML: template.HTML("<p>Public moment</p>"), Date: "2026-10-13", TimeAgo: "12小时前"}}
		out, err := stellarhtml.ProcessSite(renderBlogFixture(t, d), stellarhtml.Context{
			SiteURL: "https://site.test/", IconData: func(string) string { return "/fallback.svg" },
		})
		if err != nil {
			t.Fatal(err)
		}
		out, err = stellarimages.Enrich(out, "https://site.test/moments/", stellarimages.Store{Version: 1, Images: map[string]stellarimages.Entry{
			"https://site.test/avatar.png": {Dimensions: &dimensions},
		}}, stellarimages.EnrichOptions{})
		if err != nil {
			t.Fatal(err)
		}
		start := strings.Index(out, `<div class="user-info">`)
		if start < 0 {
			t.Fatal("missing moment author header")
		}
		header := strings.SplitN(out[start:], `</div>`, 2)[0]
		for _, fragment := range []string{`width="16"`, `height="16"`, `src="/avatar.png"`, `no-lazy`, `Stellar 2.0 验收站`} {
			if !strings.Contains(header, fragment) {
				t.Fatalf("avatar must remain compact after %dx%d metadata: missing %s in %s", dimensions.Width, dimensions.Height, fragment, header)
			}
		}
		if strings.Contains(header, "data-src") || strings.Contains(header, "data-stellar-image-dimensions") {
			t.Fatalf("moment avatar unexpectedly became a lazy or intrinsic-size image: %s", header)
		}
	}
}
