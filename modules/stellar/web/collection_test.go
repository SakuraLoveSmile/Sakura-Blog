package web

import (
	"strings"
	"testing"

	"blog/internal/settings"
)

func testEngine(t *testing.T) *Engine {
	t.Helper()
	reg, err := LoadIconsEmbed()
	if err != nil {
		t.Fatalf("LoadIconsEmbed: %v", err)
	}
	engine, err := NewEngineFromEmbed(reg)
	if err != nil {
		t.Fatalf("NewEngineFromEmbed: %v", err)
	}
	return engine
}

func TestCollectionTemplateVariantsAndEscaping(t *testing.T) {
	html, err := testEngine(t).RenderValue("partial/components/collection", settings.Collection{
		Layout: "grid", Variant: "icon", Density: "compact", Columns: 3,
		Items: []settings.CollectionItem{
			{Href: `/post/a?x="1"`, Title: `标题 <危险>`, Icon: "default:documents", Active: true, Depth: 2, Theme: "#1BCDFC"},
			{Href: "/post/b", Title: "纯图标", Icon: "default:github", ShowTitleSet: true, ShowTitle: false},
		},
	})
	if err != nil {
		t.Fatalf("render collection: %v", err)
	}
	for _, want := range []string{
		`class="ui-collection"`, `data-layout="grid"`, `data-variant="icon"`, `data-density="compact"`, `--ui-columns:3`,
		`class="ui-collection__item is-active"`, `aria-current="page"`, `--ui-item-depth:2`,
		`title="标题 &lt;危险&gt;"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("collection output missing %q: %s", want, html)
		}
	}
	if strings.Contains(html, "ZgotmplZ") || strings.Contains(html, "<危险>") {
		t.Fatalf("collection output leaked unsafe template content: %s", html)
	}
	if strings.Contains(html, `class="ui-collection__title">纯图标`) {
		t.Fatal("explicit showTitle=false should suppress icon-only title")
	}
}

func TestFooterSocialRenderingLinksDropdownAndFallbackIcon(t *testing.T) {
	html, err := testEngine(t).Render("partial/sidebar", &ViewData{
		Site: &Site{Title: "测试", Footer: Footer{Social: []SocialItem{
			{Title: "GitHub", URL: "https://github.com/example", IconKey: "default:github"},
			{Spacer: true},
			{Title: "本站", URL: "/about/", IconKey: "default:documents"},
			{Title: "更多", Type: "dropdown", IconKey: "default:documents", Items: []SocialItem{
				{Title: "内链", URL: "/links/", IconKey: "default:documents"},
			}},
		}},
		},
		Page: &Page{Layout: "index"},
	})
	if err != nil {
		t.Fatalf("render sidebar social: %v", err)
	}
	for _, want := range []string{
		`rel="external nofollow noopener noreferrer" target="_blank"`,
		`href="/about/" rel="noopener noreferrer"`,
		`class="social-spacer"`,
		`<details class="dropdown social-dropdown"`,
		`<div class="dropdown-menu ui-collection"`,
		`class="dropdown-item ui-collection__item"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("social output missing %q: %s", want, html)
		}
	}
}
