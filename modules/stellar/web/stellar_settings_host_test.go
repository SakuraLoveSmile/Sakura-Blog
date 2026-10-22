package web

import (
	"strings"
	"testing"

	"blog/internal/settings"
	"blog/internal/stellarmodels"
)

func TestSettingsUsesActualHostAndPreservesCustomAbout(t *testing.T) {
	engine, err := NewEngineFromEmbed(nil)
	if err != nil {
		t.Fatal(err)
	}
	engine.SetStellarHost(StellarHost{Name: "Blog", Version: "v9.8.7", Homepage: "https://host.example.test/source"})
	data := prepareFixture(t, "settings")
	data.Stellar = &settings.StellarRenderContext{Version: 2, Profile: "settings", Input: stellarmodels.Input{Page: map[string]any{"layout": "settings", "path": "settings/"}}}
	output, err := engine.Render("layout/base", data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "Blog v9.8.7") || !strings.Contains(output, "https://host.example.test/source") || strings.Contains(output, "Hexo v9.8.7") {
		t.Fatal("default About row does not identify actual host")
	}
	profiles := prepMap(data.Site.StellarV2.Theme["profiles"])
	prepMap(profiles["settings"])["about"] = map[string]any{"items": []any{
		map[string]any{"key": "custom", "value": "Hexo {hexo.version}", "url": "https://custom.example.test/"},
		map[string]any{"key": "tokens", "value": "{hexo.name} {theme.version}", "url": "{hexo.homepage}"},
	}}
	output, err = engine.Render("layout/base", data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "Hexo v9.8.7") || !strings.Contains(output, "https://custom.example.test/") || !strings.Contains(output, "Blog 2.0.0") {
		t.Fatal("custom About item was overwritten")
	}
}
