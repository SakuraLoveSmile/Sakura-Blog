package stellarhtml

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type oracle struct {
	Upstream string
	Contents []struct {
		Input, Base, Expected string
		Theme                 map[string]any
	}
	Images     []struct{ Input, Base, Fallback, Expected string }
	IconValues map[string]string
	Icons      []struct {
		Key, Args, Expected string
		Inline              bool
	}
	FallbackAssets []struct {
		Value    string
		Expected *struct{ Path, Body string }
	}
}

func loadOracle(t *testing.T) oracle {
	t.Helper()
	raw, err := os.ReadFile("testdata/official.json")
	if err != nil {
		t.Fatal(err)
	}
	var o oracle
	if err := json.Unmarshal(raw, &o); err != nil {
		t.Fatal(err)
	}
	return o
}
func normalized(t *testing.T, s string) string {
	t.Helper()
	nodes, err := html.ParseFragment(strings.NewReader(s), &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, n := range nodes {
		if err := html.Render(&b, n); err != nil {
			t.Fatal(err)
		}
	}
	return b.String()
}
func TestOfficialContentOracle(t *testing.T) {
	o := loadOracle(t)
	for _, c := range o.Contents {
		got, err := ProcessContent(c.Input, Context{Theme: c.Theme, SiteURL: c.Base, Icon: func(string) string { return `<svg viewBox="0 0 1 1"><path d="M0 0"/></svg>` }})
		if err != nil {
			t.Fatal(err)
		}
		if normalized(t, got) != normalized(t, c.Expected) {
			t.Errorf("base=%s input=%s\ngot %s\nwant %s", c.Base, c.Input, got, c.Expected)
		}
	}
}
func TestOfficialImageOracle(t *testing.T) {
	o := loadOracle(t)
	for _, c := range o.Images {
		got, err := ProcessSite(c.Input, Context{SiteURL: c.Base, IconData: func(string) string { return c.Fallback }})
		if err != nil {
			t.Fatal(err)
		}
		if normalized(t, got) != normalized(t, c.Expected) {
			t.Errorf("base=%s fallback=%s input=%s\ngot %s\nwant %s", c.Base, c.Fallback, c.Input, got, c.Expected)
		}
		again, err := ProcessSite(got, Context{SiteURL: c.Base, IconData: func(string) string { return c.Fallback }})
		if err != nil || again != got {
			t.Errorf("image filter not idempotent: %q %v", got, err)
		}
	}
}
func TestOfficialIconOracle(t *testing.T) {
	o := loadOracle(t)
	for _, c := range o.Icons {
		if got := RenderIcon(c.Key, c.Args, c.Inline, o.IconValues); got != c.Expected {
			t.Errorf("icon %s inline=%v args=%s got=%s want=%s", c.Key, c.Inline, c.Args, got, c.Expected)
		}
	}
}
func TestOfficialFallbackAssetOracle(t *testing.T) {
	o := loadOracle(t)
	for _, c := range o.FallbackAssets {
		got, err := ImageFallbackAsset(c.Value)
		if err != nil {
			t.Fatal(err)
		}
		if c.Expected == nil {
			if got != nil {
				t.Errorf("unexpected asset for %s", c.Value)
			}
			continue
		}
		if got == nil || got.Path != c.Expected.Path || base64.StdEncoding.EncodeToString(got.Body) != c.Expected.Body {
			t.Errorf("fallback %s got=%+v expected=%+v", c.Value, got, c.Expected)
		}
	}
}
func TestSiteFilterPreservesInertAndManagedCode(t *testing.T) {
	inert := `<script data-stellar-script="core">const a='<img src="script">';</script><style id="x">.x{content:'<img src="style">'}</style><!-- <img src="comment"> -->`
	ctx := Context{SiteURL: "https://example.com/", IconData: func(string) string { return "/fallback.svg" }}
	got, err := ProcessSite(inert+`<IMG src="real" alt="a > b"/><p>tail</p>`, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, inert) || !strings.HasSuffix(got, "<p>tail</p>") {
		t.Fatal(got)
	}
	for _, s := range []string{`<script>const a='<img src="x">';`, `<!-- <img src="x">`, `<img src="unterminated`, `<style>.x{content:'<img src=x>'}`} {
		got, err := ProcessSite(s, ctx)
		if err != nil || got != s {
			t.Errorf("unterminated content must not be truncated: got=%q err=%v", got, err)
		}
	}
}
func TestImageRewriteDoesNotCreateAuthorAttributes(t *testing.T) {
	input := `<img src='image.png" onload="evil()' class='name" onclick="evil()'>`
	got, err := ProcessSite(input, Context{SiteURL: "https://example.com/", IconData: func(string) string { return "/fallback.svg" }})
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := html.ParseFragment(strings.NewReader(got), &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		t.Fatal(err)
	}
	image := nodes[0]
	if hasAttr(image, "onload") || hasAttr(image, "onclick") {
		t.Fatalf("rewrite expanded author privileges: %s", got)
	}
	if src, _ := attr(image, "data-src"); src != `image.png" onload="evil()` {
		t.Fatalf("source changed: %s", src)
	}
}
func TestMissingRequiredDataReportsError(t *testing.T) {
	if _, err := ProcessContent(`<a href="/x">X</a>`, Context{SiteURL: "https://example.com/"}); err == nil {
		t.Fatal("missing icon accepted")
	}
	if _, err := ProcessSite(`<img src="x">`, Context{}); err == nil {
		t.Fatal("missing fallback accepted")
	}
	if got, err := ProcessSite(`<img src="data:image/png,AA">`, Context{}); err != nil || got != `<img src="data:image/png,AA">` {
		t.Fatal(got, err)
	}
}
func TestIconAssetsNamespacesCommentsAndPaths(t *testing.T) {
	assets, err := IconAssets(map[string]string{"a:one": "<svg><!-- attribution --><path/></svg>", "a:two": "<svg></svg>", "b:url": "https://example.com/icon.svg", "b:svg": " <svg></svg>", ":empty": "<svg></svg>"})
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 2 || assets[0].Path != "js/icons/a.json" || assets[1].Path != "js/icons/b.json" {
		t.Fatalf("%+v", assets)
	}
	var set map[string]map[string]string
	if err := json.Unmarshal(assets[0].Body, &set); err != nil {
		t.Fatal(err)
	}
	if set["a"]["a:one"] != "<svg><path/></svg>" {
		t.Fatal(set)
	}
	for _, key := range []string{"../x:icon", "..:icon", "x\\y:icon", "x?y:icon"} {
		if _, err := IconAssets(map[string]string{key: "<svg></svg>"}); err == nil {
			t.Errorf("unsafe asset path accepted: %s", key)
		}
	}
}

func TestUnknownIconPreservesURLAndEscapesHTML(t *testing.T) {
	if got := RenderIcon(`<script>alert(1)</script>`, "", true, nil); strings.Contains(got, "<script>") {
		t.Fatal(got)
	}
	if got := RenderIcon(`https://example.com/a?x=1&y=2`, "", false, nil); got != `<img  src="https://example.com/a?x=1&amp;y=2" />` {
		t.Fatal(got)
	}
}

func TestHTTPURLBackslashEncodingBoundaries(t *testing.T) {
	for _, c := range []struct{ href, base, want string }{
		{`https://example.com/a\b?q=\path#\fragment`, `https://example.com/`, `https://example.com/a/b?q=\path#\fragment`},
		{`?q=\path#\fragment`, `https://example.com/a`, `https://example.com/a?q=\path#\fragment`},
		{`#\fragment`, `https://example.com/a?q=\path`, `https://example.com/a?q=\path#\fragment`},
	} {
		got, err := resolveHTTPURL(c.href, c.base)
		if err != nil || got != c.want {
			t.Errorf("got %s %v want %s", got, err, c.want)
		}
	}
}
