package clientconfig

import (
	"encoding/json"
	"golang.org/x/net/html"
	"reflect"
	"strings"
	"testing"
)

func navOutput(t *testing.T, source string) (*navigationConfig, []string) {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	var nav *navigationConfig
	if found := find(doc, "#stellar-navigation-config"); len(found) == 1 {
		if err := json.Unmarshal([]byte(text(found[0])), &nav); err != nil {
			t.Fatal(err)
		}
	}
	marked := []string{}
	for _, node := range find(doc, "a[data-stellar-navigation]") {
		marked = append(marked, attr(node, "href"))
	}
	return nav, marked
}
func TestOfficialNavigationOracle(t *testing.T) {
	fixtures := loadOracle(t).Navigation
	actual := map[string]*navigationConfig{}
	official := map[string]string{}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			out, err := PrepareNavigation(fixture.HTML, fixture.Options)
			if err != nil {
				t.Fatal(err)
			}
			nav, marked := navOutput(t, out)
			if (nav != nil) != fixture.Eligible {
				t.Fatal("eligibility differs from official")
			}
			if !reflect.DeepEqual(marked, fixture.Marked) {
				t.Fatalf("links: actual %v, official %v", marked, fixture.Marked)
			}
			if nav != nil {
				if nav.Collection != `["wiki","docs"]` || nav.URL != fixture.Options.PageURL || len(nav.Signature) != 64 {
					t.Fatal(nav)
				}
			}
			actual[fixture.Name] = nav
			official[fixture.Name] = fixture.Signature
		})
	}
	for first, a := range actual {
		for second, b := range actual {
			if a != nil && b != nil {
				if (a.Signature == b.Signature) != (official[first] == official[second]) {
					t.Fatalf("compatibility differs for %s/%s", first, second)
				}
			}
		}
	}
}
func TestNavigationPersistentExtensionAndPolicyChanges(t *testing.T) {
	fixture := loadOracle(t).Navigation[0]
	out, err := PrepareNavigation(fixture.HTML, fixture.Options)
	if err != nil {
		t.Fatal(err)
	}
	base, _ := navOutput(t, out)
	for _, change := range []struct {
		from, to   string
		compatible bool
	}{{`"duration":100`, `"duration":200`, true}, {`"setting":1`, `"setting":2`, false}, {`"timeoutMs":5000`, `"timeoutMs":1000`, false}, {`<style id="shared">shared</style>`, `<style id="shared">other</style>`, false}, {`<main id="main" class="content">one</main>`, `<main id="main" class="content"><h1>New page</h1></main>`, true}, {`<link data-stellar-page-style href="/one.css">`, `<link data-stellar-page-style href="/new.css">`, true}} {
		source := strings.Replace(fixture.HTML, change.from, change.to, 1)
		if source == fixture.HTML {
			t.Fatalf("fixture mutation missing %s", change.from)
		}
		got, err := PrepareNavigation(source, fixture.Options)
		if err != nil {
			t.Fatal(err)
		}
		nav, _ := navOutput(t, got)
		if (base.Signature == nav.Signature) != change.compatible {
			t.Fatalf("change %s compatibility wrong", change.from)
		}
	}
}
func TestNavigationRejectsMalformedInputAndClearsUnsafePreviousConfig(t *testing.T) {
	fixture := loadOracle(t).Navigation[0]
	for _, source := range []string{strings.Replace(fixture.HTML, `"version":1`, `"version":2`, 1), strings.Replace(fixture.HTML, `"selector":".persistent:has(a[href])"`, `"selector":"["`, 1), strings.Replace(fixture.HTML, `"scope":"document"`, `"scope":"invalid"`, 1)} {
		if _, err := PrepareNavigation(source, fixture.Options); err == nil {
			t.Fatal("invalid navigation input accepted")
		}
	}
	out, err := PrepareNavigation(fixture.HTML, fixture.Options)
	if err != nil {
		t.Fatal(err)
	}
	out = strings.Replace(out, "</body>", "<script>unmanaged()</script></body>", 1)
	out, err = PrepareNavigation(out, fixture.Options)
	if err != nil {
		t.Fatal(err)
	}
	nav, marked := navOutput(t, out)
	if nav != nil || len(marked) > 0 {
		t.Fatal("stale unsafe navigation retained")
	}
	options := fixture.Options
	options.PageURL = "/relative/"
	if _, err := PrepareNavigation(fixture.HTML, options); err == nil {
		t.Fatal("relative base accepted")
	}
}
func TestNavigationURLRootAndReservedEscapes(t *testing.T) {
	fixture := loadOracle(t).Navigation[0]
	source := strings.Replace(fixture.HTML, "one</main>", `<a href="/sub/space%20name/">space</a><a href="/sub/a%2Fb/">reserved</a><a href="/submarine/">outside</a><a href="//else.test/sub/one/two/">external</a></main>`, 1)
	options := fixture.Options
	options.Members = map[string]string{"/sub/space name/": `["wiki","docs"]`, "/sub/a/b/": `["wiki","docs"]`, "/submarine/": `["wiki","docs"]`}
	out, err := PrepareNavigation(source, options)
	if err != nil {
		t.Fatal(err)
	}
	_, marked := navOutput(t, out)
	if !reflect.DeepEqual(marked, []string{"/sub/space%20name/"}) {
		t.Fatal(marked)
	}
}

func TestRefreshNavigationPreservesMembershipAndUpdatesShellSignature(t *testing.T) {
	fixture := loadOracle(t).Navigation[0]
	out, err := PrepareNavigation(fixture.HTML, fixture.Options)
	if err != nil {
		t.Fatal(err)
	}
	before, links := navOutput(t, out)
	// 模拟图片回填改变保留的 shell，而不是重新扫描/猜测集合成员。
	out = strings.Replace(out, "</head>", `<meta name="image-metadata" content="ready"/></head>`, 1)
	out, err = RefreshNavigationSignature(out)
	if err != nil {
		t.Fatal(err)
	}
	after, refreshedLinks := navOutput(t, out)
	if before.Collection != after.Collection || before.URL != after.URL || before.Signature == after.Signature || !reflect.DeepEqual(links, refreshedLinks) {
		t.Fatalf("navigation semantics lost: before=%+v after=%+v links=%v/%v", before, after, links, refreshedLinks)
	}
	again, err := RefreshNavigationSignature(out)
	if err != nil {
		t.Fatal(err)
	}
	stable, _ := navOutput(t, again)
	if stable.Signature != after.Signature {
		t.Fatal("refresh is not stable")
	}
}
