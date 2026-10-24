package stellarmigration

import (
	"strings"
	"testing"

	"blog/internal/stellarv2"
)

func TestLegacyV174ProductionShapes(t *testing.T) {
	s := settingsSnapshot(t, map[string]any{
		"site":     map[string]any{"title": "<Blog>", "url": "https://blog.example"},
		"feedback": map[string]any{"enabled": true, "display": "console", "secret": "private-feedback-token"},
		"footer":   map[string]any{"content": "", "beian": ""},
		"widgets":  map[string]any{"pages": nil},
		"plugins": map[string]any{
			"adaptive_text": map[string]any{"enable": true}, "copycode": map[string]any{"enable": true},
			"swiper": map[string]any{"enable": true}, "mathjax": map[string]any{"enable": false, "v3": false},
			"mermaid": map[string]any{"enable": true, "style_optimization": false},
		},
		"style": map[string]any{"error_page": "", "leftbar": map[string]any{"ui_style": "card"}, "loading": map[string]any{"loading": "", "error": ""}, "page_transition": map[string]any{"enable": nil}, "gradient": map[string]any{"avatar": "conic-gradient(red, blue)"}},
	})
	before := encode(s)
	c := buildTheme(s)
	if strings.Contains(c.ThemeJSON+c.BlogJSON+encode(c.Diagnostics)+encode(c.Mappings), "private-feedback-token") {
		t.Fatal("host feedback secret copied into theme or reports")
	}
	value, err := decode(c.ThemeJSON)
	if err != nil {
		t.Fatal(err)
	}
	theme := object(value)
	if _, err := stellarv2.ParseTheme(theme, stellarv2.Options{}); err != nil {
		t.Fatal(err)
	}
	if v, ok := get(theme, "profiles.error.image"); !ok || v != nil {
		t.Fatal("empty legacy error image inherited upstream image")
	}
	if v, _ := get(theme, "appearance.preset"); v != "card" {
		t.Fatal("card preset lost")
	}
	if v, _ := get(theme, "footer.content"); !strings.Contains(stringValue(v), "&copy; &lt;Blog&gt; · Powered by") || strings.Contains(stringValue(v), "creativecommons") {
		t.Fatal("legacy empty footer fallback changed")
	}
	if v, _ := get(theme, "features.diagrams.provider"); v != "mermaid" {
		t.Fatal("enabled Mermaid provider lost")
	}
	want := map[string]bool{"style.page_transition.enable": true, "style.gradient.avatar": true}
	for _, d := range c.Diagnostics {
		if d.Code != "legacy_official_policy" || !want[d.Path] || d.Source != "settings" || d.Kind != "policy" || !d.Blocking {
			t.Fatalf("unexpected diagnostic: %+v", d)
		}
		delete(want, d.Path)
	}
	if len(want) != 0 {
		t.Fatal("known visible behavior change lacks named policy")
	}
	if encode(s) != before {
		t.Fatal("source changed")
	}
}

func TestLegacyV174MappingsRemainNarrow(t *testing.T) {
	c := buildTheme(settingsSnapshot(t, map[string]any{
		"plugins": map[string]any{
			"swiper":        map[string]any{"enable": false, "js": "https://cdn.example/swiper.js", "css": "https://cdn.example/swiper.css", "future": true},
			"adaptive_text": map[string]any{"enable": false}, "copycode": map[string]any{"enable": false, "toast": "custom", "future": true},
			"mermaid": map[string]any{"style_optimization": true, "theme": "forest", "future": true},
		},
		"style":  map[string]any{"loading": map[string]any{"loading": "custom", "error": "custom", "future": true}, "gradient": map[string]any{"avatar": "gradient", "future": true}, "leftbar": map[string]any{"ui_style": "glass"}},
		"footer": map[string]any{"content": "**keep**", "beian": `<ICP & "quoted">`, "future": true},
	}))
	raw, _ := decode(c.ThemeJSON)
	theme := object(raw)
	if _, err := stellarv2.ParseTheme(theme, stellarv2.Options{}); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{"features.swiper.js": "https://cdn.example/swiper.js", "features.swiper.css": "https://cdn.example/swiper.css", "features.diagrams.mermaid.theme": "forest", "appearance.preset": "glass"} {
		if value, _ := get(theme, path); value != want {
			t.Errorf("%s not preserved", path)
		}
	}
	if _, exists := get(theme, "features.swiper.enable"); exists {
		t.Fatal("legacy enable leaked into strict swiper object")
	}
	footer, _ := get(theme, "footer.content")
	if footer != "**keep**" || !hasCode(c.Diagnostics, "footer_beian_content_conflict") {
		t.Fatal("custom footer with registration falsely treated as equivalent")
	}
	policies, unknowns := map[string]bool{}, map[string]bool{}
	for _, d := range c.Diagnostics {
		if d.Code == "legacy_official_policy" && d.Blocking {
			policies[d.Path] = true
		}
		if d.Code == "unmapped_setting" && d.Blocking {
			unknowns[d.Path] = true
		}
	}
	for _, path := range []string{"plugins.swiper.enable", "plugins.adaptive_text.enable", "plugins.copycode.enable", "plugins.copycode.toast", "plugins.mermaid.style_optimization", "style.loading.loading", "style.loading.error", "style.gradient.avatar"} {
		if !policies[path] || unknowns[path] {
			t.Errorf("%s not exactly a named policy", path)
		}
	}
	for _, path := range []string{"plugins.swiper.future", "plugins.copycode.future", "plugins.mermaid.future", "style.loading.future", "style.gradient.future", "footer.future"} {
		if !unknowns[path] {
			t.Errorf("unknown sibling %s silently consumed", path)
		}
	}
}

func TestLegacyV174FooterKeepsMarkdownAndRegistrationBoundaries(t *testing.T) {
	for _, content := range []string{"**keep**", "```text\nunclosed fence", "<pre>unclosed HTML", "<!-- unclosed comment"} {
		t.Run(content, func(t *testing.T) {
			s := settingsSnapshot(t, map[string]any{"footer": map[string]any{"content": content, "beian": "ICP123"}})
			before := encode(s)
			c := buildTheme(s)
			raw, _ := decode(c.ThemeJSON)
			if value, _ := get(object(raw), "footer.content"); value != content {
				t.Fatal("original Markdown was combined or repaired")
			}
			if len(c.Diagnostics) != 1 {
				t.Fatalf("expected exactly the footer blocker: %+v", c.Diagnostics)
			}
			d := c.Diagnostics[0]
			if d.Code != "footer_beian_content_conflict" || d.Source != "settings" || d.Path != "footer.beian" || !d.Blocking || d.Kind == "policy" || len(d.Alternatives) != 0 {
				t.Fatalf("footer boundary conflict became approvable: %+v", d)
			}
			if encode(s) != before {
				t.Fatal("original footer source changed")
			}
		})
		t.Run(content+" without registration", func(t *testing.T) {
			c := buildTheme(settingsSnapshot(t, map[string]any{"footer": map[string]any{"content": content, "beian": ""}}))
			raw, _ := decode(c.ThemeJSON)
			if value, _ := get(object(raw), "footer.content"); value != content || len(c.Diagnostics) != 0 {
				t.Fatalf("Markdown without registration changed or blocked: %+v", c.Diagnostics)
			}
		})
	}
	c := buildTheme(settingsSnapshot(t, map[string]any{"site": map[string]any{"title": "<Blog>", "url": "https://blog.example"}, "footer": map[string]any{"content": "", "beian": `<ICP & "quoted">`}}))
	raw, _ := decode(c.ThemeJSON)
	value, _ := get(object(raw), "footer.content")
	if !strings.Contains(stringValue(value), "&copy; &lt;Blog&gt; · Powered by") || !strings.Contains(stringValue(value), `href="https://beian.miit.gov.cn/"`) || !strings.Contains(stringValue(value), "&lt;ICP &amp; &#34;quoted&#34;&gt;") || len(c.Diagnostics) != 0 {
		t.Fatalf("closed fallback and escaped registration not preserved: %+v", c.Diagnostics)
	}
}

func TestLegacyV174RejectsUnsupportedVersionsAndShapes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		sections map[string]any
		code     string
	}{
		{"mathjax v2", map[string]any{"plugins": map[string]any{"mathjax": map[string]any{"enable": true, "v3": false}}}, "mathjax_version_unsupported"},
		{"mathjax unspecified version", map[string]any{"plugins": map[string]any{"mathjax": map[string]any{"enable": true}}}, "mathjax_version_unsupported"},
		{"mathjax unknown enable", map[string]any{"plugins": map[string]any{"mathjax": map[string]any{"v3": true}}}, "mathjax_version_unsupported"},
		{"mermaid unsupported theme", map[string]any{"plugins": map[string]any{"mermaid": map[string]any{"theme": "base"}}}, "mermaid_theme_unsupported"},
		{"invalid pages", map[string]any{"widgets": map[string]any{"pages": []any{}}}, "invalid_legacy_object"},
		{"invalid switch", map[string]any{"plugins": map[string]any{"copycode": map[string]any{"enable": "false"}}}, "invalid_legacy_boolean"},
		{"invalid transition", map[string]any{"style": map[string]any{"page_transition": map[string]any{"enable": "false"}}}, "invalid_legacy_boolean"},
		{"invalid text", map[string]any{"style": map[string]any{"loading": map[string]any{"loading": false}}}, "invalid_legacy_string"},
		{"invalid footer", map[string]any{"footer": map[string]any{"content": nil}}, "invalid_legacy_string"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := buildTheme(settingsSnapshot(t, tc.sections))
			if !hasCode(c.Diagnostics, tc.code) {
				t.Fatalf("missing %s: %+v", tc.code, c.Diagnostics)
			}
		})
	}
	for _, enabled := range []bool{false, true} {
		c := buildTheme(settingsSnapshot(t, map[string]any{"plugins": map[string]any{"mathjax": map[string]any{"enable": enabled, "v3": true}}}))
		if len(c.Diagnostics) != 0 {
			t.Fatalf("valid v3 configuration blocked: %+v", c.Diagnostics)
		}
	}
}
