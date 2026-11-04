package clientconfig

import (
	"blog/internal/stellarv2"
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

type oracleFile struct {
	Tag, Commit string
	Fixtures    []struct {
		Name                 string
		Input                Input
		Manifest, ClientData any
	}
	Navigation []struct {
		Name, HTML string
		Options    NavigationOptions
		Eligible   bool
		Marked     []string
		Signature  string
	}
}

func loadOracle(t *testing.T) oracleFile {
	t.Helper()
	file, err := os.Open("testdata/oracle.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var data oracleFile
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	d := json.NewDecoder(reader)
	d.UseNumber()
	if err := d.Decode(&data); err != nil {
		t.Fatal(err)
	}
	if data.Tag != "2.0.0" || data.Commit != "9d11978a24fe6610e3a7dbab741cf62634539d05" {
		t.Fatal("wrong oracle version")
	}
	return data
}
func assertJSON(t *testing.T, got, want any) {
	t.Helper()
	g, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	w, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var gm, wm any
	_ = json.Unmarshal(g, &gm)
	_ = json.Unmarshal(w, &wm)
	if !reflect.DeepEqual(gm, wm) {
		t.Fatalf("actual: %s\nexpected: %s", g, w)
	}
}

// normalizeManifest 去掉产品已移除的扩展，并按产品的选择器集合归一 oracle 期望值。
func normalizeManifest(t *testing.T, manifest any, drop map[string]bool) any {
	t.Helper()
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	exts, _ := m["extensions"].([]any)
	filtered := make([]any, 0, len(exts))
	for _, entry := range exts {
		obj, ok := entry.(map[string]any)
		if !ok {
			filtered = append(filtered, entry)
			continue
		}
		id, _ := obj["id"].(string)
		if drop[id] {
			continue
		}
		if id == "services" {
			if when, ok := obj["when"].(map[string]any); ok {
				if selector, ok := when["selector"].(string); ok {
					selector = strings.TrimPrefix(selector, "#artalk_container, #twikoo_container, #waline_container, ")
					when["selector"] = strings.Replace(selector, ", a[data-md-link][data-siteinfo-api], .site-card [data-siteinfo-api]", "", 1)
				}
			}
		}
		filtered = append(filtered, entry)
	}
	m["extensions"] = filtered
	return m
}

func themeSearchProvider(theme map[string]any) string {
	search, _ := theme["search"].(map[string]any)
	provider, _ := search["provider"].(string)
	return provider
}

// normalizeClientData 把已移除的站点信息外部 API 归一为 null（产品不再向客户端暴露）。
func normalizeClientData(t *testing.T, data any) any {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if ctx, ok := m["ctx"].(map[string]any); ok {
		if services, ok := ctx["services"].(map[string]any); ok {
			if siteinfo, ok := services["siteinfo"].(map[string]any); ok {
				siteinfo["api"] = nil
			}
		}
	}
	return m
}

func TestOfficialProjectionOracle(t *testing.T) {
	for _, fixture := range loadOracle(t).Fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			before, _ := json.Marshal(fixture.Input)
			bundle, err := Build(fixture.Input)
			if err != nil {
				t.Fatal(err)
			}
			// 本地补齐的媒体生命周期单独精确验证；其余声明保持官方差分。
			official := bundle.Manifest
			official.Extensions = make([]Extension, 0, len(bundle.Manifest.Extensions))
			for _, entry := range bundle.Manifest.Extensions {
				if entry.ID != "hero-video" {
					official.Extensions = append(official.Extensions, entry)
				}
			}
			// 产品已移除第三方评论/Algolia：官方 oracle 对应的扩展不再生成，
			// 其中 Algolia 旧配置在产品侧回退到本地索引。
			drop := map[string]bool{"comments": true}
			if themeSearchProvider(fixture.Input.Theme) == "algolia" {
				drop["search"] = true
			}
			assertJSON(t, normalizeManifest(t, official, drop), normalizeManifest(t, fixture.Manifest, drop))
			assertJSON(t, normalizeClientData(t, bundle.ClientData), normalizeClientData(t, fixture.ClientData))
			after, _ := json.Marshal(fixture.Input)
			if string(before) != string(after) {
				t.Fatal("Build mutated input")
			}
			if err := validateManifest(bundle.Manifest); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestHeroVideoRegionalManifest(t *testing.T) {
	for _, fixture := range loadOracle(t).Fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			bundle, err := Build(fixture.Input)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, entry := range bundle.Manifest.Extensions {
				if entry.ID == "hero-video" {
					count++
					assertJSON(t, entry, Extension{ID: "hero-video", Module: "/assets/js/runtime/extensions/hero-video.js", Scope: "region", When: When{Selector: ".wiki-cover-background-video"}, Config: map[string]any{}})
				}
				if entry.ID == "services" && strings.Contains(entry.When.Selector, ".wiki-cover-background-video") {
					t.Fatal("Hero video must not be claimed by official videoEvents services")
				}
			}
			if count != 1 {
				t.Fatalf("Hero video declaration count = %d; want 1", count)
			}
		})
	}
}
func TestBlogExtensionsRemainSeparate(t *testing.T) {
	in := Input{Theme: stellarv2.DefaultTheme(), Blog: BlogConfig{GoSearch: true, NativeComments: true, AISummary: true, AIAsk: true, Reading: true, Tagcloud: true, Checkbox: true, Content: true, Gist: true, SearchPath: "/api/search-index", SearchTTL: 123, Services: map[string]ServiceBinding{"friends": {Module: "/assets/js/blog/services/friends.js", Selector: `[data-blog-service="friends"]`, Options: map[string]any{"extra": true}}}}}
	bundle, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]Extension{}
	for _, entry := range bundle.Manifest.Extensions {
		ids[entry.ID] = entry
	}
	if _, exists := ids["search"]; exists {
		t.Fatal("stock and Blog Go search overlap")
	}
	for _, id := range []string{"blog-search", "blog-comments", "blog-ai-summary", "blog-ai-ask", "blog-reading", "blog-tagcloud", "blog-checkbox", "blog-content", "blog-gist", "blog-services"} {
		if _, ok := ids[id]; !ok {
			t.Fatal("missing", id)
		}
	}
	if ids["blog-gist"].When.Selector != "iframe[data-stellar-gist]" {
		t.Fatal("Gist DOM contract mismatch")
	}
	if at(bundle.ClientData, "ctx.search.path") != "/api/search-index" || at(bundle.ClientData, "ctx.search.local_search.cache_ttl") != 123 {
		t.Fatal("Go search options not consumed")
	}
	if _, ok := in.Theme["blog"]; ok {
		t.Fatal("Blog polluted theme")
	}
}
func TestSummaryStylesDoNotRequireReadingProgress(t *testing.T) {
	for _, tc := range []struct {
		name             string
		reading, summary bool
	}{
		{"disabled", false, false},
		{"summary-only", false, true},
		{"reading-only", true, false},
		{"both", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bundle, err := Build(Input{Blog: BlogConfig{Reading: tc.reading, AISummary: tc.summary}})
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, style := range bundle.Styles {
				if style.Href == "/assets/css/reading.css" {
					count++
				}
			}
			want := 0
			if tc.reading || tc.summary {
				want = 1
			}
			if count != want {
				t.Fatalf("reading/summary stylesheet count = %d; want %d", count, want)
			}
			for _, ext := range bundle.Manifest.Extensions {
				if ext.ID == "blog-reading" && !tc.reading {
					t.Fatal("summary must not enable reading progress")
				}
			}
		})
	}
}

func TestHostStylesFollowActiveFeaturesAndKeepAppearanceLast(t *testing.T) {
	for _, tc := range []struct {
		name             string
		blog             BlogConfig
		search, password bool
	}{
		{name: "stock"},
		{name: "go search", blog: BlogConfig{GoSearch: true}, search: true},
		{name: "AI ask", blog: BlogConfig{AIAsk: true}, search: true},
		{name: "both search features", blog: BlogConfig{GoSearch: true, AIAsk: true}, search: true},
		{name: "protected post", blog: BlogConfig{Password: true}, password: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bundle, err := Build(Input{Blog: tc.blog})
			if err != nil {
				t.Fatal(err)
			}
			counts := map[string]int{}
			for _, style := range bundle.Styles {
				counts[style.Href]++
				if style.Href == "/assets/css/post-password.css" && !style.PageStyle {
					t.Fatal("password style must follow the current page during partial navigation")
				}
			}
			if counts["/assets/css/highlight.css"] != 1 {
				t.Fatal("Chroma tokens need one local stylesheet for all Markdown pages")
			}
			if counts["/assets/css/pagination.css"] != 1 {
				t.Fatal("pagination must retain its complete navigation at narrow viewports")
			}
			for path, enabled := range map[string]bool{"/assets/css/search.css": tc.search, "/assets/css/post-password.css": tc.password} {
				want := 0
				if enabled {
					want = 1
				}
				if counts[path] != want {
					t.Fatalf("%s count = %d; want %d", path, counts[path], want)
				}
			}
			if got := bundle.Styles[len(bundle.Styles)-1].Href; got != "/assets/css/main-appearance-card.css" {
				t.Fatalf("appearance tail moved before host styles: %s", got)
			}
		})
	}
}

func TestStylesAndUseNumber(t *testing.T) {
	d := json.NewDecoder(strings.NewReader(`{"appearance":{"preset":"minimal","color_scheme":"light"},"features":{"card_hover":{"spotlight":true,"tilt":true},"lazy_loading":{"transition":"blur"},"math":{"provider":"katex","katex":{"css":"https://custom.test/katex.css","css_integrity":"sha384-custom"}},"reveal":{"duration":123}},"search":{"provider":"local","local":{"cache_ttl_seconds":9007199254740993}}}`))
	d.UseNumber()
	var raw map[string]any
	if err := d.Decode(&raw); err != nil {
		t.Fatal(err)
	}
	parsed, err := stellarv2.ParseTheme(raw, stellarv2.Options{})
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := Build(Input{Theme: parsed.Value})
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"/assets/css/main-minimal.css", "/assets/css/main-card-hover-spotlight.css", "/assets/css/main-card-hover-tilt.css", "/assets/css/main-lazyload-blur.css", "/assets/css/main-katex.css", "https://custom.test/katex.css", "https://gcore.jsdelivr.net/gh/highlightjs/cdn-release@11.9/build/styles/atom-one-dark.min.css", "/assets/css/highlight.css", "/assets/css/pagination.css", "/assets/css/site-footer.css", "/assets/css/main-appearance-minimal.css"}
	for i, path := range expected {
		if bundle.Styles[i].Href != path {
			t.Fatalf("styles[%d]: %+v", i, bundle.Styles)
		}
	}
	if bundle.Styles[5].Integrity != "sha384-custom" || !bundle.Styles[5].PageStyle {
		t.Fatalf("KaTeX SRI/page style missing: %+v", bundle.Styles[5])
	}
	for _, e := range bundle.Manifest.Extensions {
		if e.ID == "search" {
			v := at(e.Config, "options.cacheTtlSeconds")
			if v != json.Number("9007199254740993") {
				t.Fatalf("large parameter integer lost: %#v", v)
			}
		}
		if e.ID == "reveal" && e.Config["duration"] != json.Number("123") {
			t.Fatalf("numeric config not consumed: %#v", e.Config)
		}
	}
}

func TestPageMathOverride(t *testing.T) {
	for _, themeProvider := range []string{"katex", "mathjax"} {
		for _, tc := range []struct {
			name     string
			render   map[string]any
			provider string
		}{
			{"missing", nil, themeProvider},
			{"null", map[string]any{"math": nil}, themeProvider},
			{"disabled", map[string]any{"math": false}, ""},
			{"katex", map[string]any{"math": "katex"}, "katex"},
			{"mathjax", map[string]any{"math": "mathjax"}, "mathjax"},
		} {
			t.Run(themeProvider+"/"+tc.name, func(t *testing.T) {
				bundle, err := Build(Input{
					Theme: map[string]any{"features": map[string]any{"math": map[string]any{"provider": themeProvider}}},
					Page:  map[string]any{"render": tc.render},
				})
				if err != nil {
					t.Fatal(err)
				}
				hasMathJax := false
				for _, extension := range bundle.Manifest.Extensions {
					if extension.ID == "mathjax" {
						hasMathJax = true
					}
				}
				if hasMathJax != (tc.provider == "mathjax") {
					t.Errorf("MathJax extension present = %v; selected provider = %q", hasMathJax, tc.provider)
				}
				katexStyles := 0
				for _, style := range bundle.Styles {
					if strings.Contains(style.Href, "katex") {
						katexStyles++
					}
				}
				wantStyles := 0
				if tc.provider == "katex" {
					wantStyles = 2
				}
				if katexStyles != wantStyles {
					t.Errorf("KaTeX stylesheet count = %d; want %d", katexStyles, wantStyles)
				}
			})
		}
	}
}

func TestLanguageDefaultsAndExplicitMessages(t *testing.T) {
	for language, want := range map[string]string{"zh-CN": "刚刚", "zh-TW": "剛剛", "en": "Just"} {
		bundle, err := Build(Input{Language: language})
		if err != nil {
			t.Fatal(err)
		}
		if at(bundle.ClientData, "ctx.date_suffix.just") != want {
			t.Fatal(language, bundle.ClientData)
		}
	}
	bundle, err := Build(Input{Language: "en", Messages: map[string]any{"dateSuffix": map[string]any{"just": "Now"}}})
	if err != nil {
		t.Fatal(err)
	}
	if at(bundle.ClientData, "ctx.date_suffix.just") != "Now" || at(bundle.ClientData, "ctx.date_suffix.day") != "days ago" {
		t.Fatal("language override lost other defaults")
	}
	if _, err := Build(Input{Messages: map[string]any{"copy": []any{}}}); err == nil {
		t.Fatal("invalid message bag accepted")
	}
}
func TestBlogServiceBoundaryAndConcurrentBuild(t *testing.T) {
	_, err := Build(Input{Blog: BlogConfig{Services: map[string]ServiceBinding{"bad": {Module: "/assets/js/blog/services/../bad.js", Selector: ".x"}}}})
	if err == nil {
		t.Fatal("traversing service module accepted")
	}
	theme := stellarv2.DefaultTheme()
	before, _ := json.Marshal(theme)
	done := make(chan error, 20)
	for i := 0; i < 20; i++ {
		go func() { _, err := Build(Input{Theme: theme}); done <- err }()
	}
	for i := 0; i < 20; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	after, _ := json.Marshal(theme)
	if string(before) != string(after) {
		t.Fatal("concurrent Build mutated shared config")
	}
}

func TestMetadataSearchProviderSelection(t *testing.T) {
	for _, provider := range []any{"local", "algolia", nil} {
		for _, include := range []bool{true, false} {
			for _, goSearch := range []bool{true, false} {
				parsed, err := stellarv2.ParseTheme(map[string]any{"search": map[string]any{"provider": provider, "local": map[string]any{"include_content": include}}}, stellarv2.Options{})
				if err != nil {
					t.Fatal(err)
				}
				bundle, err := Build(Input{Theme: parsed.Value, Blog: BlogConfig{GoSearch: goSearch}})
				if err != nil {
					t.Fatal(err)
				}
				var stock, blog *Extension
				for i := range bundle.Manifest.Extensions {
					entry := &bundle.Manifest.Extensions[i]
					if entry.ID == "search" {
						stock = entry
					}
					if entry.ID == "blog-search" {
						blog = entry
					}
				}
				if goSearch {
					if blog == nil || stock != nil {
						t.Fatal("Go provider not exclusive")
					}
					continue
				}
				if provider == nil {
					if stock != nil || blog != nil {
						t.Fatal("null provider enabled")
					}
					continue
				}
				if stock == nil {
					t.Fatal("missing provider")
				}
				// Algolia 已移除，旧配置回退本地索引，行为与 local 相同。
				expected := "/assets/js/runtime/extensions/search.js"
				asset := at(stock.Config, "assets.provider")
				if !include {
					expected = "/assets/js/runtime/extensions/blog-title-search.js"
					if asset != "/assets/js/blog/search/title-search.js" {
						t.Fatal("metadata provider asset missing")
					}
				}
				if stock.Module != expected {
					t.Fatal(provider, include, stock.Module)
				}
				if at(stock.Config, "provider") != "local" {
					t.Fatal(provider, "did not fall back to local")
				}
			}
		}
	}
}
