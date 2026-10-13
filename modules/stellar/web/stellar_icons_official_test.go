package web

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestStellarEmbeddedIconsMatchLockedUpstream(t *testing.T) {
	var fixture struct {
		Commit, Source, SourceSHA256 string
		Icons                        map[string]string
		BlogExtensions               map[string]string
	}
	data, err := os.ReadFile("stellar_template_helpers_testdata/icons.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Commit != "9d11978a24fe6610e3a7dbab741cf62634539d05" || fixture.Source != "_data/icons.yml" || fixture.SourceSHA256 != "c900f5556c2df923dece4030a3f2e9f886088c94f263c43138b85899b12702bd" || len(fixture.Icons) != 108 {
		t.Fatal("unlocked official icons oracle")
	}
	registry, err := LoadIconsEmbed()
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range fixture.Icons {
		if !registry.Has(key) || registry.Get(key) != want {
			t.Errorf("embedded icon %s differs from locked upstream", key)
		}
	}
	if len(fixture.BlogExtensions) != 5 || registry.Len() != len(fixture.Icons)+len(fixture.BlogExtensions) {
		t.Error("unexpected canonical/Blog extension icon key set")
	}
	for key, want := range fixture.BlogExtensions {
		if !registry.Has(key) || registry.Get(key) != want {
			t.Errorf("independent Blog extension icon changed %s", key)
		}
	}
	d := stellarTemplateFixture(t)
	renderer := stellarTemplateRenderer{icons: registry}
	for _, service := range []string{"qrcode", "weibo", "x", "telegram", "whatsapp", "email"} {
		output := string(renderer.share(d, map[string]any{"services": []any{service}, "permalink": "https://fixture.example/post/one", "title": "Public title"}))
		if !strings.Contains(output, fixture.Icons["share:"+service]) || strings.Contains(output, "icon-missing") {
			t.Errorf("official share helper did not render upstream %s SVG", service)
		}
	}
}
