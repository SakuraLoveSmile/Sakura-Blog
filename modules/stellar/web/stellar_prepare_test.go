package web

import (
	"encoding/json"
	"html/template"
	"reflect"
	"strings"
	"testing"

	"blog/internal/config"
	"blog/internal/settings"
	"blog/internal/stellarmodels"
	"blog/modules/stellar/clientconfig"
)

func prepareFixture(t *testing.T, layout string) *settings.ViewData {
	t.Helper()
	v2, err := config.ValidateStellarV2(map[string]any{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &settings.ViewData{Site: &settings.Site{Title: "Strict site", URL: "https://blog.example.test/blog/", StellarV2: v2}, Page: &settings.Page{Title: "Fixture", Layout: layout, Lang: "en", CanonicalURL: "https://blog.example.test/blog/fixture"}}
}

func TestPrepareStellarRequiresMigrationWithoutLegacyGuessing(t *testing.T) {
	for _, data := range []*settings.ViewData{nil, {Site: &settings.Site{}, Page: &settings.Page{}}, prepareFixture(t, "post"), prepareFixture(t, "page")} {
		if _, err := PrepareStellar(data); err == nil || !strings.Contains(err.Error(), "migration required") {
			t.Fatalf("migration gate: %v", err)
		}
	}
}

func TestPrepareStellarDoesNotMutatePublishedSnapshot(t *testing.T) {
	data := prepareFixture(t, "index")
	data.Site.StellarV2.Blog = map[string]any{"data": map[string]any{"widgets": map[string]any{"custom": map[string]any{"layout": "markdown", "content": "fixture"}}, "custom": map[string]any{"number": json.Number("9007199254740993"), "false": false, "null": nil}}}
	data.Stellar = &settings.StellarRenderContext{Version: 2, Profile: "home", Input: stellarmodels.Input{RuntimeData: map[string]any{"title": "public title", "custom": prepCloneMap(prepMap(data.Site.StellarV2.Blog["data"]))["custom"], "widgets": MergeStellarWidgetCatalog(prepMap(prepGet(data.Site.StellarV2.Blog, "data.widgets")))}}}
	data.Site.StellarV2.Blog["data"].(map[string]any)["privateSecret"] = "DO_NOT_PROPAGATE"
	beforeTheme := prepCloneMap(data.Site.StellarV2.Theme)
	beforeBlog := prepCloneMap(data.Site.StellarV2.Blog)
	a, err := PrepareStellar(data)
	if err != nil {
		t.Fatal(err)
	}
	b, err := PrepareStellar(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(beforeTheme, data.Site.StellarV2.Theme) || !reflect.DeepEqual(beforeBlog, data.Site.StellarV2.Blog) {
		t.Fatal("published configuration mutated")
	}
	if a.Stellar.Data["privateSecret"] != nil {
		t.Fatal("raw configuration escaped public snapshot")
	}
	if prepGet(a.Stellar.Data, "custom.number") != json.Number("9007199254740993") {
		t.Fatal("lossy public data copy")
	}
	if prepGet(a.Stellar.Data, "widgets.toc.layout") != "toc" || prepGet(a.Stellar.Data, "widgets.custom.layout") != "markdown" {
		t.Fatal("default catalog or override lost")
	}
	prepMap(prepGet(a.Stellar.Data, "widgets.custom"))["content"] = "changed"
	if prepGet(b.Stellar.Data, "widgets.custom.content") != "fixture" {
		t.Fatal("requests share mutable widget data")
	}
}

func TestPrepareStellarGenericPageAndPostUseDistinctOfficialModels(t *testing.T) {
	for _, layout := range []string{"page", "post"} {
		t.Run(layout, func(t *testing.T) {
			data := prepareFixture(t, layout)
			data.Stellar = &settings.StellarRenderContext{Version: 2, Profile: layout, Input: stellarmodels.Input{Page: map[string]any{"title": "Fixture", "layout": layout, "content": "<p>body</p>", "path": "fixture", "permalink": data.Page.CanonicalURL}, FrontMatter: map[string]any{}}}
			prepared, err := PrepareStellar(data)
			if err != nil {
				t.Fatal(err)
			}
			if (prepared.Stellar.Model != nil) != (layout == "post") {
				t.Fatalf("%s model: %#v", layout, prepared.Stellar.Model)
			}
			if prepared.Stellar.Profile != layout {
				t.Fatalf("profile %s", prepared.Stellar.Profile)
			}
		})
	}
}

func TestPrepareStellarStylesScriptsAndPublicGoSearch(t *testing.T) {
	data := prepareFixture(t, "index")
	data.Site.StellarV2.Blog = map[string]any{"search": map[string]any{"provider": "go"}}
	prepared, err := PrepareStellar(data)
	if err != nil {
		t.Fatal(err)
	}
	bundle := prepared.Stellar.Client.(clientconfig.Bundle)
	if !reflect.DeepEqual(bundle.CoreScripts, []string{"/assets/js/utils.js", "/assets/js/main.js", "/assets/js/navigation-init.js", "/assets/js/profile.js"}) {
		t.Fatal(bundle.CoreScripts)
	}
	if strings.Contains(string(prepared.Stellar.CSS), "@import") {
		t.Fatal("duplicate highlight import")
	}
	seenGo, seenStock := false, false
	for _, e := range bundle.Manifest.Extensions {
		if e.ID == "blog-search" {
			seenGo = true
		}
		if e.ID == "search" {
			seenStock = true
		}
	}
	if !seenGo || seenStock {
		t.Fatalf("exclusive search: go=%v stock=%v", seenGo, seenStock)
	}
}

func TestGeneratedStellarAssetsContainPublicFallbackAndNamespaces(t *testing.T) {
	reg, err := settings.LoadIconsEmbed()
	if err != nil {
		t.Fatal(err)
	}
	assets, err := GeneratedStellarAssets(reg)
	if err != nil {
		t.Fatal(err)
	}
	fallback, namespace := false, false
	for _, asset := range assets {
		if strings.HasPrefix(asset.Path, "images/stellar-error.") {
			fallback = true
		}
		if asset.Path == "js/icons/default.json" {
			namespace = true
		}
		if len(asset.Body) == 0 {
			t.Fatal("empty generated asset")
		}
	}
	if !fallback || !namespace {
		t.Fatalf("fallback=%v namespace=%v", fallback, namespace)
	}
}

func TestEngineStellarCompositionUsesOfficialShellAndNavigation(t *testing.T) {
	reg, err := settings.LoadIconsEmbed()
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngineFromEmbed(reg)
	if err != nil {
		t.Fatal(err)
	}
	data := prepareFixture(t, "post")
	data.Content = template.HTML(`<h2 id="section">Section</h2><p>body <a href="/blog/other">next</a></p>`)
	data.Stellar = &settings.StellarRenderContext{Version: 2, Profile: "post", RequestURL: "https://blog.example.test/blog/fixture", NavigationMembers: map[string]string{"/blog/other": `["post","post"]`}, Input: stellarmodels.Input{Page: map[string]any{"title": "Fixture", "layout": "post", "content": string(data.Content), "path": "fixture", "permalink": data.Page.CanonicalURL}, FrontMatter: map[string]any{}}}
	output, err := engine.Render("layout/base", data)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`id="stellar-runtime-config"`, `id="stellar-navigation-config"`, `data-stellar-navigation=""`, `window.stellarClientData=`, `/assets/js/utils.js`, `/assets/js/main.js`, `Section`} {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(output, "const utils") || strings.Contains(output, "__stellarPlugins") {
		t.Fatal("legacy runtime globals emitted")
	}
	utilsPos, mainPos := strings.Index(output, "/assets/js/utils.js"), strings.Index(output, "/assets/js/main.js")
	if utilsPos < 0 || mainPos <= utilsPos {
		t.Fatal("stock globals load out of order")
	}
	if strings.Contains(output, `id="topbar-region"`) {
		t.Fatal("disabled topbar emitted")
	}
}

func TestPreparePublicNativeOverridesAndWidgetDeletion(t *testing.T) {
	data := prepareFixture(t, "page")
	data.Site.StellarV2.Blog = map[string]any{"native_comments": map[string]any{"enabled": true, "title": "global"}, "data": map[string]any{"widgets": map[string]any{"toc": nil}}}
	data.Stellar = &settings.StellarRenderContext{Version: 2, Profile: "page", Input: stellarmodels.Input{RuntimeData: map[string]any{"nativeComments": map[string]any{"enabled": false, "title": "record"}, "widgets": MergeStellarWidgetCatalog(map[string]any{"toc": nil})}, Page: map[string]any{}, FrontMatter: map[string]any{}}}
	prepared, err := PrepareStellar(data)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Stellar.BlogNativeComments || prepGet(prepared.Stellar.Data, "nativeComments.title") != "record" {
		t.Fatal("native record override lost")
	}
	if _, exists := prepMap(prepared.Stellar.Data["widgets"])["toc"]; exists {
		t.Fatal("deleted runtime widget restored")
	}
	partial := MergeStellarWidgetCatalog(map[string]any{"toc": map[string]any{"max_depth": 3}})
	if prepGet(partial, "toc.layout") != "toc" || prepGet(partial, "toc.max_depth") != 3 {
		t.Fatal("partial descriptor lost default layout")
	}
}
