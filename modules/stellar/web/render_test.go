package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"blog/internal/settings"
)

func boolPtr(v bool) *bool { return &v }

func renderPostDocument(t *testing.T, site *Site) string {
	t.Helper()
	engine, err := NewEngineFromEmbed(&IconRegistry{})
	if err != nil {
		t.Fatalf("NewEngineFromEmbed: %v", err)
	}
	html, err := engine.Render("layout/base", &ViewData{
		Site: site,
		Page: &Page{
			Title:        "测试\"文章",
			Slug:         "seo-test",
			Layout:       "post",
			CanonicalURL: "https://origin.example/post/seo-test",
			Path:         "/post/seo-test",
			Description:  "测试\\描述",
			Date:         "2026-08-15",
			Author:       "作者",
		},
	})
	if err != nil {
		t.Fatalf("render layout/base: %v", err)
	}
	return html
}

func TestHeadJSONLDIsValidWithAndWithoutSameAs(t *testing.T) {
	jsonLD := regexp.MustCompile(`(?s)<script type="application/ld\+json">\s*(\{.*?\})\s*</script>`)
	for _, tc := range []struct {
		name       string
		links      []string
		wantSameAs bool
	}{
		{name: "without sameAs"},
		{name: "with sameAs", links: []string{"https://github.com/example"}, wantSameAs: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			html := renderPostDocument(t, &Site{
				Title: "测试站点",
				SEO:   SEOConfig{StructuredData: StructuredDataConfig{Links: tc.links}},
			})
			match := jsonLD.FindStringSubmatch(html)
			if len(match) != 2 {
				t.Fatalf("JSON-LD block not found")
			}
			var payload map[string]any
			if err := json.Unmarshal([]byte(match[1]), &payload); err != nil {
				t.Fatalf("invalid JSON-LD: %v\n%s", err, match[1])
			}
			_, hasSameAs := payload["sameAs"]
			if hasSameAs != tc.wantSameAs {
				t.Fatalf("sameAs presence = %v, want %v", hasSameAs, tc.wantSameAs)
			}
		})
	}
}

func TestHeadAndBaseRespectSEOAndPluginSettings(t *testing.T) {
	disabled := renderPostDocument(t, &Site{
		Title: "禁用配置",
		SEO:   SEOConfig{OpenGraph: OpenGraphConfig{Enable: boolPtr(false)}},
		Plugins: PluginsConfig{
			Swiper:   &SwiperConfig{Enable: false},
			KaTeX:    &KaTeXConfig{Enable: false},
			MathJax:  &MathJaxConfig{Enable: false},
			Mermaid:  &MermaidConfig{Enable: false},
			Copycode: &CopycodeConfig{Enable: false},
		},
	})
	if strings.Contains(disabled, `property="og:title"`) {
		t.Error("Open Graph tags should be omitted when disabled")
	}
	for _, asset := range []string{"math-mermaid-loader.js", "copycode.js", "services/swiper.js"} {
		if strings.Contains(disabled, asset) {
			t.Errorf("disabled plugin still injects %q", asset)
		}
	}

	enabled := renderPostDocument(t, &Site{
		Title: "启用配置",
		SEO: SEOConfig{
			Canonical: CanonicalConfig{OriginalHost: "origin.example"},
			OpenGraph: OpenGraphConfig{Enable: boolPtr(true), TwitterID: "example"},
		},
		Plugins: PluginsConfig{
			Swiper:   &SwiperConfig{Enable: true, CSS: "https://cdn.example/swiper.css", JS: "https://cdn.example/swiper.js"},
			KaTeX:    &KaTeXConfig{Enable: true},
			Mermaid:  &MermaidConfig{Enable: true, JS: "https://cdn.example/mermaid.js"},
			Copycode: &CopycodeConfig{Enable: true, DefaultText: "复制", SuccessText: "已复制", Toast: "完成"},
		},
	})
	for _, want := range []string{
		`property="og:title"`,
		`name="twitter:site" content="@example"`,
		`math-mermaid-loader.js`,
		`copycode.js`,
		`services\/swiper.js`,
		`https://cdn.example/swiper.css`,
		`https://cdn.example/swiper.js`,
		`复制`, `已复制`, `完成`,
		`originalHost: 'origin.example'`,
	} {
		if !strings.Contains(enabled, want) {
			t.Errorf("enabled document missing %q", want)
		}
	}
	if !strings.Contains(enabled, `https://origin.example/post/seo-test`) {
		t.Error("canonical URL missing from rendered head")
	}
}

func TestBaseInjectsRuntimeConfigAndTagPluginOverrides(t *testing.T) {
	searchLazy := false
	searchTTL := 0
	cacheEnabled := false
	cacheTTL := 0
	cacheMax := 7
	site := &Site{
		Title:     "运行时配置",
		Search:    settings.SearchConfig{LocalSearch: settings.LocalSearchConfig{LazyLoad: &searchLazy, CacheTTL: &searchTTL}},
		DataCache: settings.DataCacheConfig{Enable: &cacheEnabled, DefaultTTL: &cacheTTL, MaxEntries: &cacheMax, TTL: map[string]int{"sites": 0}},
		Services:  settings.ServicesConfig{SiteInfo: settings.SiteInfoConfig{API: `https://api.example.test?url={href}&x="quoted"`}},
		TagPlugins: settings.TagPluginsConfig{
			Note:     settings.TagPluginNote{Border: boolPtr(false)},
			Checkbox: settings.TagPluginCheckbox{Interactive: false},
			OKR:      settings.TagPluginOKR{Border: boolPtr(false)},
		},
	}
	html := renderPostDocument(t, site)
	for _, want := range []string{
		`lazy_load: false`,
		`cache_ttl:  0`,
		`default_ttl:  0`,
		`"sites":0`,
		`max_entries:  7`,
		`api: "https://api.example.test?url={href}\u0026x=\"quoted\""`,
		`.md-text .tag-plugin.note{border:0}`,
		`.md-text .tag-plugin.okr{border:0}`,
		`.md-text .tag-plugin.checkbox input{pointer-events:none}`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered base missing %q", want)
		}
	}
}

// TestBaseRendersFeedbackWidget feedback 段：enabled+api_base+app_id 齐备才渲染
// <feedback-widget> 与 ESM 入口脚本；主题固定 dark；page-label 取页面 canonical path。
func TestBaseRendersFeedbackWidget(t *testing.T) {
	off := renderPostDocument(t, &Site{Title: "关"})
	if strings.Contains(off, "feedback-widget") {
		t.Error("feedback disabled should not render widget")
	}
	incomplete := renderPostDocument(t, &Site{
		Title:    "缺配置",
		Feedback: settings.FeedbackConfig{Enabled: true, APIBase: "https://feedback.example.com"},
	})
	if strings.Contains(incomplete, "feedback-widget") {
		t.Error("missing app_id should not render widget")
	}

	on := renderPostDocument(t, &Site{
		Title: "开",
		Feedback: settings.FeedbackConfig{
			Enabled: true, APIBase: "https://feedback.example.com", AppID: "com.example.blog",
			AppName: "测试博客", Side: "left", LauncherMode: "tab", CaptureMode: "off", LauncherBottom: "80px",
		},
	})
	for _, want := range []string{
		`<feedback-widget`,
		`api-base="https://feedback.example.com"`,
		`app-id="com.example.blog"`,
		`app-name="测试博客"`,
		`page-label="/post/seo-test"`,
		`side="left"`,
		`theme="dark"`,
		`launcher-mode="tab"`,
		`capture-mode="off"`,
		`launcher-bottom="80px"`,
		`/assets/vendor/feedback/feedback-web.js?v=`,
		`type="module"`,
		// html2canvas-pro 兼容覆盖：头像 border-radius calc 乘法降级为字面值
		`.logo-wrap a.avatar img.avatar { border-radius: 50%; }`,
	} {
		if !strings.Contains(on, want) {
			t.Errorf("enabled document missing %q", want)
		}
	}
	if strings.Contains(off, "border-radius: 50%") {
		t.Error("capture-compat override should be omitted when feedback disabled")
	}
}

func TestDiskAndEmbedEnginesRenderIdentically(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(file), "templates")
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("template directory missing: %v", err)
	}
	icons, err := LoadIconsEmbed()
	if err != nil {
		t.Fatalf("load icons: %v", err)
	}
	disk, err := NewEngine(root, icons)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	embed, err := NewEngineFromEmbed(icons)
	if err != nil {
		t.Fatalf("NewEngineFromEmbed: %v", err)
	}
	data := &ViewData{
		Site: &Site{Title: "一致性测试"},
		Page: &Page{Title: "404", Layout: "404", Slug: "missing"},
	}
	for _, name := range []string{"layout/base", "partial/footer", "partial/error404"} {
		diskHTML, err := disk.Render(name, data)
		if err != nil {
			t.Fatalf("disk render %s: %v", name, err)
		}
		embedHTML, err := embed.Render(name, data)
		if err != nil {
			t.Fatalf("embed render %s: %v", name, err)
		}
		if diskHTML != embedHTML {
			t.Fatalf("disk/embed output differs for %s", name)
		}
	}
}

func TestEmbeddedStellarAssetsExist(t *testing.T) {
	for _, path := range []string{
		"templates/layout/base.html",
		"templates/partial/error404.html",
		"assets/css/main.css",
		"assets/js/main.js",
		"assets/manifest.webmanifest",
		"assets/fonts/lxgw/lxgwwenkaiscreen.css",
		"assets/img/favicon.ico",
		"assets/vendor/feedback/feedback-web.js",
	} {
		if _, err := FS.ReadFile(path); err != nil {
			t.Errorf("embedded Stellar asset missing %q: %v", path, err)
		}
	}
}

// TestHeadSEO141 Spec 71 T6（上游 1.41 SEO 修复移植）：og:site_name 恒为站点名、
// 分页 title 带页码后缀、og:description 走 MetaDescription 回退链。
func TestHeadSEO141(t *testing.T) {
	engine, err := NewEngineFromEmbed(&IconRegistry{})
	if err != nil {
		t.Fatalf("NewEngineFromEmbed: %v", err)
	}
	render := func(vd *ViewData) string {
		t.Helper()
		html, err := engine.Render("layout/base", vd)
		if err != nil {
			t.Fatalf("render layout/base: %v", err)
		}
		return html
	}

	t.Run("og:site_name 恒为站点名", func(t *testing.T) {
		html := render(&ViewData{
			Site: &Site{Title: "测试站"},
			Page: &Page{Title: "文章", Layout: "post"},
		})
		if !strings.Contains(html, `<meta property="og:site_name" content="测试站">`) {
			t.Fatalf("og:site_name 缺失")
		}
	})

	t.Run("首页分页 title 带页码", func(t *testing.T) {
		html := render(&ViewData{
			Site:      &Site{Title: "测试站"},
			Page:      &Page{Layout: "index"},
			Paginator: &Paginator{Current: 2},
		})
		if !strings.Contains(html, "<title>测试站 - 第 2 页 - 测试站</title>") {
			t.Fatalf("分页 title 缺失页码后缀，got: %s", html[:200])
		}
	})

	t.Run("非文章页 og:image 回退站点头像", func(t *testing.T) {
		html := render(&ViewData{
			Site: &Site{Title: "测试站", Avatar: "https://example.com/a.png"},
			Page: &Page{Layout: "index"},
		})
		if !strings.Contains(html, `<meta property="og:image" content="https://example.com/a.png">`) {
			t.Fatalf("非文章页 og:image 未回退头像")
		}
	})
}
