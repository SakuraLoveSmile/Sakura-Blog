package template

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
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
