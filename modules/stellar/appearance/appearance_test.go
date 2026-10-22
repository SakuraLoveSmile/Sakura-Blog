package appearance

import (
	"encoding/json"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"blog/internal/stellarv2"
)

type fixture struct {
	Name   string
	Input  map[string]any
	Tokens map[string]map[string]string
}

func defaults(t *testing.T) map[string]any { t.Helper(); return stellarv2.DefaultTheme() }
func set(theme map[string]any, path string, value any) {
	keys := strings.Split(path, ".")
	cur := theme
	for _, key := range keys[:len(keys)-1] {
		cur = cur[key].(map[string]any)
	}
	cur[keys[len(keys)-1]] = value
}

var rulePattern = regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`)

func tokens(css string) map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, m := range rulePattern.FindAllStringSubmatch(css, -1) {
		sel := strings.TrimSpace(m[1])
		if strings.HasPrefix(sel, "@import") {
			sel = sel[strings.LastIndex(sel, ";")+1:]
		}
		if strings.HasPrefix(sel, "@") {
			continue
		}
		sel = strings.Join(strings.Fields(strings.ReplaceAll(sel, "\"", "'")), " ")
		sel = strings.ReplaceAll(sel, ", ", ",")
		if out[sel] == nil {
			out[sel] = map[string]string{}
		}
		for _, d := range strings.Split(m[2], ";") {
			i := strings.IndexByte(d, ':')
			if i > 0 {
				out[sel][strings.TrimSpace(d[:i])] = strings.TrimSpace(d[i+1:])
			}
		}
	}
	return out
}

var numericPattern = regexp.MustCompile(`-?(?:\d*\.)?\d+`)

func normalized(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	s = strings.ReplaceAll(s, "\"", "'")
	s = strings.ReplaceAll(s, ", ", ",")
	return numericPattern.ReplaceAllStringFunc(s, func(x string) string {
		v, e := strconv.ParseFloat(x, 64)
		if e != nil {
			return x
		}
		return strconv.FormatFloat(math.Round(v*1e8)/1e8, 'f', -1, 64)
	})
}
func rgba(c channels) [4]float64 {
	h := c.h / 60
	s := c.s / 100
	l := c.l / 100
	v := (1 - math.Abs(2*l-1)) * s
	x := v * (1 - math.Abs(math.Mod(h, 2)-1))
	m := l - v/2
	r, g, b := 0.0, 0.0, 0.0
	switch {
	case h < 1:
		r, g = v, x
	case h < 2:
		r, g = x, v
	case h < 3:
		g, b = v, x
	case h < 4:
		g, b = x, v
	case h < 5:
		r, b = x, v
	default:
		r, b = v, x
	}
	return [4]float64{math.Round((r + m) * 255), math.Round((g + m) * 255), math.Round((b + m) * 255), c.a}
}
func equalValue(a, b string) bool {
	if ca, ok := parseColor(a); ok {
		if cb, ok := parseColor(b); ok {
			aa, bb := rgba(ca), rgba(cb)
			for i := range aa {
				if math.Abs(aa[i]-bb[i]) > 1e-8 {
					return false
				}
			}
			return true
		}
	}
	return normalized(a) == normalized(b)
}
func TestOfficialStylusOracle(t *testing.T) {
	data, e := os.ReadFile("testdata/stylus-oracle.json")
	if e != nil {
		t.Fatal(e)
	}
	var oracle struct {
		Commit   string
		Fixtures []fixture
	}
	if e = json.Unmarshal(data, &oracle); e != nil {
		t.Fatal(e)
	}
	if oracle.Commit != "9d11978a24fe6610e3a7dbab741cf62634539d05" {
		t.Fatal("oracle source changed")
	}
	for _, f := range oracle.Fixtures {
		t.Run(f.Name, func(t *testing.T) {
			parsed, e := stellarv2.ParseTheme(f.Input, stellarv2.Options{})
			if e != nil {
				t.Fatal(e)
			}
			css, e := Build(parsed.Value)
			if e != nil {
				t.Fatal(e)
			}
			got := tokens(string(css))
			for selector, properties := range f.Tokens {
				for property, want := range properties {
					compare := strings.Contains(selector, "colorful") || strings.HasSuffix(selector, ".tag-plugin.tag") || strings.Contains(selector, "site-background") || strings.Contains(selector, "site-region__decoration")
					if selector == ":root" {
						compare = property == "--hue" || property == "--sat" || property == "--light" || property == "--alpha" || property == "--accent" || property == "--link" || property == "--link-a20" || strings.HasPrefix(property, "--theme")
					}
					if selector == "body > .site-background" {
						compare = property == "background-image"
					}
					if selector == "body > .site-background .site-background__blur" {
						compare = strings.HasPrefix(property, "--blur-")
					}
					if selector == ".post-list .post-card .post-cover img" || selector == ".post-list .post-card.post.photo .cover img" || selector == ".pin-slide.post-slide" {
						compare = property == "aspect-ratio"
					}
					if !compare {
						continue
					}
					actual := got[selector][property]
					if !equalValue(actual, want) {
						t.Errorf("%s %s: got %q, upstream %q", selector, property, actual, want)
					}
				}
			}
		})
	}
}

func TestListingCoverRatioRejectsInvalidNormalizedValue(t *testing.T) {
	for _, value := range []any{0, -1, math.NaN(), math.Inf(1), json.Number("1e999"), "2;display:none"} {
		theme := defaults(t)
		set(theme, "article.listing.coverRatio", value)
		css, err := Build(theme)
		if err == nil || css != "" || !strings.Contains(err.Error(), "article.listing.coverRatio") {
			t.Errorf("invalid ratio %v became trusted CSS: %v", value, err)
		}
	}
}
func TestCompleteParameters(t *testing.T) {
	theme := defaults(t)
	for path, value := range map[string]any{
		"appearance.typography.fontFamily.body": "Georgia, serif", "appearance.typography.fontFamily.code": "Consolas, monospace", "appearance.typography.fontSmoothing": "none", "appearance.typography.contentAlign": "justify", "appearance.typography.fontSize.root": "18px", "appearance.typography.fontSize.inlineCode": "92%", "appearance.typography.fontSize.codeBlock": "0.9rem", "appearance.shape.corner": "bevel", "appearance.shape.radius.cardLarge": "30px", "appearance.shape.radius.card": "22px", "appearance.shape.radius.cardSmall": "14px", "appearance.shape.radius.bar": "9px", "appearance.shape.radius.imageLarge": "32px", "appearance.shape.radius.image": "18px", "appearance.shape.radius.imageSmall": "6px", "appearance.gradients.primaryAction": "linear-gradient(to right, red, blue)", "appearance.gradients.searchBar": "linear-gradient(to right, lime, gold)", "appearance.codeBlock.scrollbarWidth": "7px", "tags.checkbox.interactive": true, "tags.note.border": false,
	} {
		set(theme, path, value)
	}
	for _, weight := range []string{"100", "200", "300", "400", "500", "600", "700", "800", "900"} {
		set(theme, "appearance.typography.fontWeight."+weight, 650)
	}
	css, e := Build(theme)
	if e != nil {
		t.Fatal(e)
	}
	root := tokens(string(css))[":root"]
	for property, value := range map[string]string{"--font-family-body": "Georgia, serif", "--font-family-code": "Consolas, monospace", "--font-smoothing": "none", "--content-align": "justify", "--font-size-root": "18px", "--font-size-inline-code": "92%", "--font-size-code-block": "0.9rem", "--corner-shape": "bevel", "--radius-card-l": "30px", "--radius-card": "22px", "--radius-card-s": "14px", "--radius-bar": "9px", "--radius-image-l": "32px", "--radius-image": "18px", "--radius-image-s": "6px", "--gradient-start": "linear-gradient(to right, red, blue)", "--gradient-searchbar": "linear-gradient(to right, lime, gold)", "--code-scrollbar-width": "7px", "--checkbox-pointer-events": "auto", "--note-border": "none"} {
		if root[property] != value {
			t.Errorf("%s got %q", property, root[property])
		}
	}
	for _, weight := range []string{"100", "200", "300", "400", "500", "600", "700", "800", "900"} {
		if root["--font-weight-"+weight] != "650" {
			t.Errorf("weight %s not consumed", weight)
		}
	}
	if !strings.HasPrefix(string(css), "@import url(\"") {
		t.Fatal("highlight stylesheet not consumed")
	}
	set(theme, "appearance.codeBlock.highlightStylesheet", nil)
	without, e := Build(theme)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(without), "@import") {
		t.Fatal("null stylesheet must disable loading")
	}
}
func TestModernSourcesAndAllDirectConsumers(t *testing.T) {
	for _, source := range []string{"var(--visitor-primary)", "var(--custom, rgb(10 20 30))", "hwb(90 10% 20%)", "lab(60% 20 30)", "lch(60% 30 90)", "oklab(60% .1 .2)", "oklch(60% .2 90)", "color(display-p3 .2 .5 .8)", "color-mix(in srgb, red 30%, blue)", "currentColor", "hsl(.5turn 60% 40%)", "hsl(3.141592653589793rad 60% 40%)", "hsl(200grad 60% 40%)"} {
		t.Run(source, func(t *testing.T) {
			theme := defaults(t)
			set(theme, "appearance.colors.primary", source)
			set(theme, "appearance.colors.accent", source)
			set(theme, "appearance.colors.link", source)
			css, e := Build(theme)
			if e != nil {
				t.Fatal(e)
			}
			rules := tokens(string(css))
			if rules[":root"]["--stellar-source-color"] != source {
				t.Fatal("source substituted")
			}
			for _, selector := range []string{":root", ".colorful[color='theme']", ".colorful[color='accent']", ":root[data-theme='dark'] .colorful[color='theme']", ":root:not([data-theme]) .colorful[color='accent']"} {
				for _, property := range []string{"--theme", "--theme-a10", "--theme-a20", "--theme-a30"} {
					if !strings.Contains(rules[selector][property], "hsl(from var(--stellar-source-color)") {
						t.Errorf("%s %s not relative", selector, property)
					}
				}
			}
			if !strings.Contains(string(css), "@media (prefers-color-scheme:dark)") {
				t.Fatal("automatic branch missing")
			}
			count := 0
			for selector, properties := range rules {
				for property, value := range properties {
					if (property == "background" || property == "background-color" || property == "color" || property == "--link-a20") && strings.Contains(value, "hsl(from var(--stellar-source-color)") {
						count++
						if selector == "" {
							t.Fatal("missing selector")
						}
					}
				}
			}
			if count < 8 {
				t.Errorf("only %d direct consumers", count)
			}
			// 固定投票颜色为子元素提供自己的来源，不能继承动态主色。
			if rules[".tag-plugin.ds-vote .body button.vote-up"]["--stellar-source-color"] != "#3dc550" {
				t.Fatal("vote source changed")
			}
		})
	}
}
func TestBackgroundAndArtalkTransitions(t *testing.T) {
	theme := defaults(t)
	set(theme, "appearance.preset", "glass")
	set(theme, "appearance.backgrounds.leftbar.type", "image")
	set(theme, "appearance.backgrounds.leftbar.image", "https://example.test/image.png?a=1&b=2")
	set(theme, "appearance.backgrounds.leftbar.opacity", .8)
	set(theme, "appearance.backgrounds.leftbar.backdrop.radius", "40px")
	set(theme, "appearance.backgrounds.page.image", "/page.jpg")
	css, e := Build(theme)
	if e != nil {
		t.Fatal(e)
	}
	got := tokens(string(css))
	if got[decoration]["--background-opacity"] != "0.8" || got[":root[data-theme='dark'] "+decoration]["--background-opacity"] != "0.6000000000000001" {
		t.Fatal("image opacity must follow upstream .75 dark multiplier")
	}
	if got[collapsedDecoration]["--blur-px"] != "20px" {
		t.Fatal("collapsed blur not half")
	}
	if !strings.Contains(string(css), "--at-color-border:transparent") {
		t.Fatal("Artalk background condition missing")
	}
	set(theme, "appearance.backgrounds.leftbar.type", "none")
	set(theme, "appearance.backgrounds.page.image", nil)
	css, e = Build(theme)
	if e != nil {
		t.Fatal(e)
	}
	got = tokens(string(css))
	if got[decoration]["display"] != "none" || got["body > .site-background"]["background-image"] != "none" {
		t.Fatal("removing background did not clear it")
	}
	if !strings.Contains(string(css), "--at-color-border:var(--block-border)") {
		t.Fatal("Artalk border not restored")
	}
}
func TestUnsafeValuesNeverBecomeTrustedCSS(t *testing.T) {
	for path, value := range map[string]any{"appearance.colors.primary": "red);}</style><script>alert(1)</script>", "appearance.gradients.primaryAction": "linear-gradient(red,blue);body{display:none}", "appearance.typography.fontFamily.body": "Arial /* escape */", "appearance.typography.fontFamily.code": "u\\72l(https://example.test)", "appearance.shape.corner": "round; color:red", "appearance.backgrounds.page.image": "https://example.test/a\") } body {color:red}", "appearance.backgrounds.page.backdrop.overlay": "var(--bg, url(https://example.test))", "appearance.backgrounds.leftbar.gradient.light": []any{"red", "blue", "green", "red;display:none"}, "appearance.typography.fontWeight.400": math.NaN(), "appearance.codeBlock.scrollbarWidth": "watpx"} {
		t.Run(path, func(t *testing.T) {
			theme := defaults(t)
			set(theme, path, value)
			css, e := Build(theme)
			if e == nil || css != "" {
				t.Fatalf("unsafe input trusted: %s", css)
			}
		})
	}
	if css, e := Build(nil); e == nil || css != "" {
		t.Fatal("missing normalized fields must fail")
	}
}
func TestBuildIsDeterministicAndReadOnly(t *testing.T) {
	theme := defaults(t)
	before, _ := json.Marshal(theme)
	a, e := Build(theme)
	if e != nil {
		t.Fatal(e)
	}
	b, e := Build(theme)
	if e != nil || a != b {
		t.Fatal("non-deterministic build")
	}
	after, _ := json.Marshal(theme)
	if string(before) != string(after) {
		t.Fatal("configuration mutated")
	}
}

func TestEveryCompiledHueConsumerRemainsValidForModernSource(t *testing.T) {
	for _, preset := range []string{"card", "glass", "minimal", "flat"} {
		t.Run(preset, func(t *testing.T) {
			base, e := os.ReadFile("../web/assets/css/main-" + preset + ".css")
			if e != nil {
				t.Fatal(e)
			}
			theme := defaults(t)
			set(theme, "appearance.preset", preset)
			set(theme, "appearance.colors.primary", "var(--custom-primary)")
			set(theme, "appearance.colors.accent", "oklch(60% .2 90)")
			css, e := Build(theme)
			if e != nil {
				t.Fatal(e)
			}
			overrides := tokens(string(css))
			count := 0
			for selector, properties := range tokens(string(base)) {
				for property, value := range properties {
					if !strings.Contains(value, "hsla(var(--hue)") {
						continue
					}
					count++
					// 固定色板和投票按钮有独立、有效的数值通道。
					if _, ok := parseNumber(strings.TrimSuffix(properties["--hue"], "deg")); ok {
						continue
					}
					if strings.Contains(overrides[selector][property], "hsl(from ") {
						continue
					}
					t.Errorf("uncovered compiled hue consumer %s %s", selector, property)
				}
			}
			if count < 30 {
				t.Fatalf("unexpectedly narrow upstream scan: %d", count)
			}
			t.Logf("checked %d compiled hue consumers", count)
		})
	}
}

func TestResourceBasesPreserveStylesheetAndSiteContexts(t *testing.T) {
	theme := defaults(t)
	set(theme, "appearance.preset", "glass")
	set(theme, "appearance.backgrounds.leftbar.type", "image")
	set(theme, "appearance.backgrounds.leftbar.image", "images/side photo.jpg")
	set(theme, "appearance.backgrounds.page.image", "images/page.png?rev=2#large")
	set(theme, "appearance.codeBlock.highlightStylesheet", "styles/highlight.css")
	cases := []struct {
		options               Options
		side, page, highlight string
	}{
		{Options{}, "/css/images/side%20photo.jpg", "/css/images/page.png?rev=2#large", "/styles/highlight.css"},
		{Options{RootURL: "/blog/", CSSURL: "assets/css/main-glass.css?v=hash"}, "/blog/assets/css/images/side%20photo.jpg", "/blog/assets/css/images/page.png?rev=2#large", "/blog/styles/highlight.css"},
		{Options{RootURL: "https://blog.example/blog/", CSSURL: "https://cdn.example/theme/main-glass.css"}, "https://cdn.example/theme/images/side%20photo.jpg", "https://cdn.example/theme/images/page.png?rev=2#large", "https://blog.example/blog/styles/highlight.css"},
	}
	for _, c := range cases {
		css, e := BuildWithOptions(theme, c.options)
		if e != nil {
			t.Fatal(e)
		}
		got := tokens(string(css))
		if got[decoration]["background-image"] != cssURL(c.side) || got["body > .site-background"]["background-image"] != cssURL(c.page) || !strings.HasPrefix(string(css), "@import "+cssURL(c.highlight)+";") {
			t.Errorf("wrong resource base: %+v", c.options)
		}
	}
	for _, source := range []string{"/uploads/background.png", "https://images.example/background.png", "//images.example/background.png", "data:image/png;base64,AAAA"} {
		set(theme, "appearance.backgrounds.page.image", source)
		css, e := BuildWithOptions(theme, Options{RootURL: "/blog/", CSSURL: "/assets/css/main-glass.css"})
		if e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(string(css), "body > .site-background{background-image:"+cssURL(source)+";") {
			t.Errorf("standard source URL changed: %q", source)
		}
	}
	a, e := Build(theme)
	if e != nil {
		t.Fatal(e)
	}
	b, e := BuildWithOptions(theme, Options{RootURL: "/", CSSURL: "/css/main.css"})
	if e != nil || a != b {
		t.Fatal("wrapper default differs from upstream default bases")
	}
}
func TestUnsafeResourceBasesAreRejected(t *testing.T) {
	for _, options := range []Options{{RootURL: "blog/"}, {RootURL: "javascript:alert(1)"}, {RootURL: "/blog/?secret=1"}, {RootURL: "https://user:pass@example.test/"}, {CSSURL: "https://example.test/x\";}</style>"}, {CSSURL: "file:///private/main.css"}} {
		if css, e := BuildWithOptions(defaults(t), options); e == nil || css != "" {
			t.Fatalf("unsafe base trusted: %+v", options)
		}
	}
}

func TestSkipHighlightStylesheetPreservesIndependentDefaultAndOtherFields(t *testing.T) {
	theme := defaults(t)
	set(theme, "appearance.codeBlock.highlightStylesheet", "styles/highlight.css")
	normal, e := Build(theme)
	if e != nil {
		t.Fatal(e)
	}
	prefix := "@import " + cssURL("/styles/highlight.css") + ";"
	if !strings.HasPrefix(string(normal), prefix) {
		t.Fatal("standalone Build must load highlight stylesheet")
	}
	skipped, e := BuildWithOptions(theme, Options{SkipHighlightStylesheet: true})
	if e != nil {
		t.Fatal(e)
	}
	if string(skipped) != strings.TrimPrefix(string(normal), prefix) {
		t.Fatal("skip must only remove the duplicate import")
	}
	if strings.Contains(string(skipped), "@import") {
		t.Fatal("duplicated highlight stylesheet import")
	}
	set(theme, "appearance.codeBlock.highlightStylesheet", "https://example.test/bad\")}</style>")
	if css, e := BuildWithOptions(theme, Options{SkipHighlightStylesheet: true}); e == nil || css != "" {
		t.Fatal("skip must not bypass resource validation")
	}
}
