package web

import (
	"html/template"
	"os"
	"regexp"
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
		`class="ui-collection__item is-active ui-interactive card-hover card-hover--spotlight"`, `aria-current="page"`, `--ui-item-depth:2`,
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

func TestRegressionWidgetsUseCollectionContract(t *testing.T) {
	engine := testEngine(t)

	popular, err := engine.RenderValue("partial/widgets/popular", &settings.WidgetInstance{
		Layout: "popular",
		Params: settings.WidgetDef{"title": "热门"},
		Data:   []*settings.RecentPost{{URL: "/post/hot", Title: `热门 <文章>`}},
	})
	if err != nil {
		t.Fatalf("render popular: %v", err)
	}
	for _, want := range []string{
		`class="widget-wrapper popular"`,
		`data-layout="list"`, `data-variant="nav"`, `data-density="compact"`,
		`class="ui-collection__title">热门 &lt;文章&gt;`,
	} {
		if !strings.Contains(popular, want) {
			t.Errorf("popular output missing %q: %s", want, popular)
		}
	}
	for _, legacy := range []string{`post-list`, `class="item title"`} {
		if strings.Contains(popular, legacy) {
			t.Errorf("popular output still uses legacy %q: %s", legacy, popular)
		}
	}

	welcome, err := engine.RenderValue("partial/widgets/markdown", &settings.WidgetInstance{
		Name:   "welcome",
		Layout: "markdown",
		Params: settings.WidgetDef{"title": "欢迎"},
		V: &settings.ViewData{
			Site: &settings.Site{Rightbar: settings.RightbarConfig{WelcomeLinklist: settings.LinkListConfig{
				Columns: 2,
				Items: []settings.LinkListItem{
					{Title: `文库 <查看>`, URL: "/library/"},
					{Title: "笔记", URL: "/notes/"},
				},
			}}},
			Page: &settings.Page{CanonicalURL: "https://example.test/library/index.html"},
		},
	})
	if err != nil {
		t.Fatalf("render welcome: %v", err)
	}
	for _, want := range []string{
		`data-layout="grid"`, `data-variant="nav"`, `--ui-columns:2`,
		`class="ui-collection__item is-active ui-interactive card-hover card-hover--spotlight"`,
		`文库 &lt;查看&gt;`, `aria-current="page"`,
	} {
		if !strings.Contains(welcome, want) {
			t.Errorf("welcome output missing %q: %s", want, welcome)
		}
	}
	for _, legacy := range []string{`class="linklist`, `class="link"`, `grid-template-columns`} {
		if strings.Contains(welcome, legacy) {
			t.Errorf("welcome output still uses legacy %q: %s", legacy, welcome)
		}
	}

	linklist, err := engine.RenderValue("partial/widgets/linklist", &settings.WidgetInstance{
		Layout: "linklist",
		Params: settings.WidgetDef{
			"columns": 3,
			"items": []any{
				map[string]any{"title": "图标 <一>", "url": "/icons/"},
				map[string]any{"title": "图标二", "url": "/two/"},
				map[string]any{"title": "图标三", "url": "/three/"},
			},
		},
		V: &settings.ViewData{Page: &settings.Page{CanonicalURL: "https://example.test/icons/"}},
	})
	if err != nil {
		t.Fatalf("render linklist: %v", err)
	}
	for _, want := range []string{
		`data-layout="grid"`, `data-variant="icon"`, `--ui-columns:3`,
		`class="ui-collection__item is-active ui-interactive card-hover card-hover--spotlight"`, `title="图标 &lt;一&gt;"`,
	} {
		if !strings.Contains(linklist, want) {
			t.Errorf("linklist output missing %q: %s", want, linklist)
		}
	}
	for _, legacy := range []string{`class="linklist`, `class="link"`, `grid-template-columns`} {
		if strings.Contains(linklist, legacy) {
			t.Errorf("linklist output still uses legacy %q: %s", legacy, linklist)
		}
	}

	related, err := engine.RenderValue("partial/widgets/wiki_related", &settings.WidgetInstance{
		Layout: "wiki_related",
		Params: settings.WidgetDef{"title": `更多 <项目>`},
		Data: []*settings.WikiProject{{
			ID:          "go-blog",
			Title:       `Go <博客>`,
			Description: `描述 <安全>`,
			HasDocs:     true,
		}},
	})
	if err != nil {
		t.Fatalf("render wiki related: %v", err)
	}
	for _, want := range []string{
		`class="widget-wrapper wiki-related"`,
		`data-layout="list"`, `data-variant="summary"`,
		`Go &lt;博客&gt;`, `描述 &lt;安全&gt;`,
	} {
		if !strings.Contains(related, want) {
			t.Errorf("wiki related output missing %q: %s", want, related)
		}
	}
	for _, legacy := range []string{`widget-wrapper post-card`, `class="item wiki"`, `class="post-card`} {
		if strings.Contains(related, legacy) {
			t.Errorf("wiki related output still uses legacy %q: %s", legacy, related)
		}
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
		`<div class="dropdown-menu ui-collection ui-surface"`,
		`class="dropdown-item ui-collection__item ui-interactive"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("social output missing %q: %s", want, html)
		}
	}
}

// 2.0 的组件尺寸只消费 .ui-icon；原始 SVG 的固有宽高不能代替容器。
func TestWidgetsUseStellarIconSizingContainers(t *testing.T) {
	engine := testEngine(t)
	view := &ViewData{Site: &Site{}, TOC: template.HTML(`<nav class="toc"><a class="toc-link" href="#heading">标题</a></nav>`)}
	wrappedSVG := regexp.MustCompile(`<span class="ui-icon">\s*<svg\b`)
	for _, tc := range []struct {
		layout string
		params settings.WidgetDef
		icons  int
		href   string
	}{
		{"toc", nil, 2, `href="#heading"`},
		{"recent", settings.WidgetDef{"rss": "/custom.xml"}, 1, `href="/custom.xml"`},
		{"author", nil, 1, `href="/"`},
		{"ghrepo", settings.WidgetDef{"repo": "example/project"}, 4, `href="https://github.com/example/project"`},
		{"ghuser", settings.WidgetDef{"username": "example", "header": true}, 2, `href="https://github.com/example"`},
	} {
		t.Run(tc.layout, func(t *testing.T) {
			html, err := engine.RenderValue("partial/widgets/"+tc.layout, &settings.WidgetInstance{Layout: tc.layout, Params: tc.params, V: view})
			if err != nil {
				t.Fatal(err)
			}
			if got := len(wrappedSVG.FindAllStringIndex(html, -1)); got != tc.icons || strings.Count(html, "<svg") != tc.icons {
				t.Errorf("widget icons must all use sizing containers: got %d wrapped SVGs, want %d", got, tc.icons)
			}
			if !strings.Contains(html, tc.href) {
				t.Errorf("widget lost existing link %s", tc.href)
			}
		})
	}
	css, err := os.ReadFile("assets/css/main.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, selector := range []string{
		".widget-wrapper .widget-header .cap-action .ui-icon",
		".widget-wrapper.ghrepo .repo .ui-icon",
		".widgets .widget-wrapper.user-card .follow .ui-icon",
		".widget-wrapper.toc .widget-footer a .ui-icon",
	} {
		if !strings.Contains(string(css), selector) {
			t.Errorf("Stellar CSS missing icon size consumer %s", selector)
		}
	}
}
