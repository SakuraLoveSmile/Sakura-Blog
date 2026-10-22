package web

import (
	"strings"
	"testing"
)

func TestStellarListingTaxonomyNavOfficialLabelsAndPaths(t *testing.T) {
	for _, locale := range []struct{ language, category, tag, colon string }{
		{"zh-CN", "分类", "标签", "："},
		{"zh-TW", "分類", "標籤", "："},
		{"en", "Category", "Tag", ": "},
	} {
		for _, root := range []string{"/", "/sub/"} {
			for _, kind := range []string{"category", "tag"} {
				t.Run(locale.language+root+kind, func(t *testing.T) {
					d := stellarTemplateFixture(t)
					d.Stellar.Root, d.Stellar.Language, d.Stellar.Layout = root, locale.language, kind
					d.Stellar.Data["hasCategories"], d.Stellar.Data["hasTags"] = true, true
					d.Stellar.Data["categoriesPath"], d.Stellar.Data["tagsPath"] = "blog/categories", "blog/tags"
					title, path := locale.category, "blog/categories/"
					if kind == "category" {
						d.Page.Category = `A<&"B`
					} else {
						d.Page.Tag = `A<&"B`
						title, path = locale.tag, "blog/tags/"
					}
					output := string((stellarTemplateRenderer{}).listingNav(d, false))
					want := `<a class="active" href="` + root + path + `">` + title + locale.colon + `A&lt;&amp;&#34;B</a>`
					if !strings.Contains(output, want) {
						t.Errorf("missing official taxonomy nav %s in %s", want, output)
					}
					if strings.Count(output, `class="active"`) != 1 {
						t.Error("taxonomy detail must select one built-in tab")
					}
				})
			}
		}
	}
}

func TestStellarListingNavCustomTabsKeepUpstreamURLFor(t *testing.T) {
	d := stellarTemplateFixture(t)
	d.Stellar.Root = "/sub/"
	config := stellarMap(stellarGet(d.Stellar.Theme, "profiles.blogIndex.listingNav"))
	config["tabs"] = []any{
		map[string]any{"title": "Custom local", "url": "/extra?query=1#part"},
		map[string]any{"title": "External", "url": "https://external.test/path"},
	}
	output := string((stellarTemplateRenderer{}).listingNav(d, false))
	for _, href := range []string{"/sub/extra?query=1#part", "https://external.test/path"} {
		if !strings.Contains(output, `href="`+href+`"`) {
			t.Errorf("custom tab url_for changed %s in %s", href, output)
		}
	}
}
