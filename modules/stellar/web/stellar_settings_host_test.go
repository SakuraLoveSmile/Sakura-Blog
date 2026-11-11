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

// 设置页「个人资料」只在能用站点身份读写时出现：原生账号与 artalk/waline/twikoo 一致，
// 没有身份能力的第三方评论后端（如 beaudar）保持不渲染。
func TestSettingsProfileSectionFollowsCommentIdentity(t *testing.T) {
	engine, err := NewEngineFromEmbed(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		provider any
		want     bool
		native   bool
	}{
		{"no provider", nil, true, true},
		{"native", "native", true, true},
		{"artalk", "artalk", true, false},
		{"beaudar", "beaudar", false, false},
	} {
		data := prepareFixture(t, "settings")
		data.Site.StellarV2.Theme["comments"] = map[string]any{"provider": tc.provider}
		data.Stellar = &settings.StellarRenderContext{Version: 2, Profile: "settings", Input: stellarmodels.Input{Page: map[string]any{"layout": "settings", "path": "settings/"}}}
		output, err := engine.Render("layout/base", data)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		rendered := strings.Contains(output, `aria-labelledby="settings-profile"`)
		if rendered != tc.want {
			t.Fatalf("%s: profile section rendered = %v, want %v", tc.name, rendered, tc.want)
		}
		if !tc.want {
			continue
		}
		if !strings.Contains(output, `data-profile-form`) || !strings.Contains(output, `data-profile-logout`) {
			t.Fatalf("%s: profile form/logout missing", tc.name)
		}
		if tc.native && !strings.Contains(output, `data-provider="native"`) {
			t.Fatalf("%s: native provider attribute missing", tc.name)
		}
	}
}

// 左栏「设置」挂件紧跟同一套身份模型：原生评论（账号或本地资料）也要能显示读者昵称，
// 没有身份能力的第三方后端仍只显示「设置」。
func TestSettingsWidgetIdentityFollowsCommentProvider(t *testing.T) {
	engine, err := NewEngineFromEmbed(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		provider any
		want     bool
	}{
		{"no provider", nil, true},
		{"native", "native", true},
		{"artalk", "artalk", true},
		{"beaudar", "beaudar", false},
	} {
		data := prepareFixture(t, "settings")
		data.Site.StellarV2.Theme["comments"] = map[string]any{"provider": tc.provider}
		data.Stellar = &settings.StellarRenderContext{Version: 2, Profile: "settings", Input: stellarmodels.Input{Page: map[string]any{"layout": "settings", "path": "settings/"}}}
		output, err := engine.Render("layout/base", data)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		enabled := strings.Contains(output, `data-profile-identity-enabled="true"`)
		if enabled != tc.want {
			t.Fatalf("%s: widget identity enabled = %v, want %v", tc.name, enabled, tc.want)
		}
	}
}
