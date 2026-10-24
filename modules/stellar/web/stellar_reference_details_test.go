package web

import (
	"strings"
	"testing"

	"blog/internal/post"
	"blog/internal/settings"
	"blog/internal/stellarmodels"
)

func TestStellarReferencesEnhanceLinksButNotLicense(t *testing.T) {
	for _, endpoint := range []string{"", "https://metadata.test/info?url={href}&source=references"} {
		t.Run(endpoint, func(t *testing.T) {
			d := stellarTemplateFixture(t)
			d.Site.URL = "https://example.test/blog"
			d.Stellar.Theme["services"] = map[string]any{"siteInfo": map[string]any{
				"provider": "custom", "custom": map[string]any{"endpoint": endpoint},
			}}
			d.markdownRender = post.NewService("").RenderStellarV2WithContext
			icons, err := settings.LoadIconsEmbed()
			if err != nil {
				t.Fatal(err)
			}
			source := string((stellarTemplateRenderer{icons: icons}).articleFooter(d, map[string]any{
				"references": []any{"[Reference](./source)"}, "license": "[License](./license)",
			}))
			if d.markdownError != nil {
				t.Fatal(d.markdownError)
			}
			sections := strings.Split(source, `<section id="license">`)
			if len(sections) != 2 {
				t.Fatal(source)
			}
			if !strings.Contains(sections[0], `data-md-link=""`) || !strings.Contains(sections[0], `class="md-link-icon ui-icon"`) || !strings.Contains(sections[0], `<svg`) {
				t.Fatalf("reference lacks enhanced inline link: %s", sections[0])
			}
			if endpoint != "" {
				if !strings.Contains(sections[0], `data-siteinfo-api="https://metadata.test/info?url=https%3A%2F%2Fexample.test%2Fblog%2Fsource&amp;source=references"`) {
					t.Fatalf("siteInfo did not use configured endpoint and site root: %s", sections[0])
				}
			} else if strings.Contains(sections[0], `data-siteinfo-api`) {
				t.Fatalf("unconfigured metadata service added: %s", sections[0])
			}
			if !strings.Contains(sections[1], `href="./license"`) || strings.Contains(sections[1], "data-md-link") || strings.Contains(sections[1], "md-link-icon") || strings.Contains(sections[1], "data-siteinfo-api") {
				t.Fatalf("license should remain plain Markdown: %s", sections[1])
			}
		})
	}
}

func TestStellarReferencesPreserveLinkExclusions(t *testing.T) {
	d := stellarTemplateFixture(t)
	d.markdownRender = post.NewService("").RenderStellarV2WithContext
	icons, err := settings.LoadIconsEmbed()
	if err != nil {
		t.Fatal(err)
	}
	source := string((stellarTemplateRenderer{icons: icons}).articleFooter(d, map[string]any{"references": []any{
		`<a class="custom" href="/class">Class</a>`, `<a cardlink href="/card">Card</a>`,
		`[Anchor](#target)`, `[Mail](mailto:alice@example.test)`,
	}}))
	if d.markdownError != nil {
		t.Fatal(d.markdownError)
	}
	if strings.Contains(source, "data-md-link") || strings.Contains(source, "md-link-icon") || strings.Contains(source, "data-siteinfo-api") {
		t.Fatalf("excluded links were enhanced: %s", source)
	}
	for _, label := range []string{"Class</a>", "Card</a>", "Anchor</a>", "Mail</a>"} {
		if !strings.Contains(source, label) {
			t.Fatalf("excluded link lost: %s", source)
		}
	}
}

func TestStellarDateInfoSupportsDottedAuthorID(t *testing.T) {
	for _, listing := range []bool{false, true} {
		d := stellarTemplateFixture(t)
		d.Stellar.Root = "/blog/"
		d.Stellar.Data = map[string]any{
			"defaultAuthor": map[string]any{"name": "Default", "path": "/authors/default/"},
			"authors":       map[string]any{"alice.dev": map[string]any{"name": "Alice Dev", "path": "/authors/alice.dev/"}},
			"page":          map[string]any{"article": map[string]any{"author": "alice.dev"}},
		}
		d.Stellar.Model = &stellarmodels.PageModel{Collection: map[string]any{"profile": "post"}}
		if listing {
			d.Stellar.Model.Render = map[string]any{"listing": map[string]any{"authorId": "alice.dev"}}
		}
		source := string((stellarTemplateRenderer{}).dateInfo(d))
		if !strings.Contains(source, `<a class="author" href="/blog/authors/alice.dev/">Alice Dev</a>`) || strings.Contains(source, ">Default</a>") {
			t.Errorf("listing=%v: wrong author name/link: %s", listing, source)
		}
	}
}
