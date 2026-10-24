package stellarhtml

import (
	"strings"
	"testing"
)

func TestEnhanceMarkdownLinksOnlyChangesLinks(t *testing.T) {
	ctx := Context{SiteURL: "https://example.test/blog/", Icon: func(string) string { return `<svg><path d="M0 0"/></svg>` }}
	input := `<table><tr><td>Cell</td></tr></table><img src="image.png" alt="Image"><a href="./source">Reference</a>`
	got, err := EnhanceMarkdownLinks(input, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "md-table-scroll") || strings.Contains(got, "data-src") || strings.Contains(got, "onerror") {
		t.Fatalf("link enhancement changed tables or images: %s", got)
	}
	if !strings.Contains(got, `<table>`) || !strings.Contains(got, `<img src="image.png" alt="Image"/>`) || strings.Count(got, `data-md-link=""`) != 1 || !strings.Contains(got, `class="md-link-icon ui-icon"`) {
		t.Fatal(got)
	}
	again, err := EnhanceMarkdownLinks(got, ctx)
	if err != nil || normalized(t, again) != normalized(t, got) {
		t.Fatalf("link enhancement not idempotent: %s, %v", again, err)
	}
	withoutLink := `<table><tr><td>Cell</td></tr></table><img src='image.png'>`
	if got, err := EnhanceMarkdownLinks(withoutLink, Context{}); err != nil || got != withoutLink {
		t.Fatalf("content without links changed: %q, %v", got, err)
	}
}

func TestEnhanceMarkdownLinksRetainsFilterExclusions(t *testing.T) {
	for _, input := range []string{
		`<a class="custom" href="/class">Class</a>`,
		`<a cardlink href="/card">Card</a>`,
		`<a href="#target">Anchor</a>`,
		`<a href="mailto:alice@example.test">Mail</a>`,
		`<a href="/image"><img src="image.png"></a>`,
		`<pre><a href="/code">Code</a></pre>`,
		`<a href="/existing" data-md-link>Existing</a>`,
	} {
		got, err := EnhanceMarkdownLinks(input, Context{SiteURL: "https://example.test/"})
		if err != nil || normalized(t, got) != normalized(t, input) {
			t.Errorf("excluded link changed: input=%s got=%s err=%v", input, got, err)
		}
	}
	if _, err := EnhanceMarkdownLinks(`<a href="/source">Reference</a>`, Context{SiteURL: "https://example.test/"}); err == nil {
		t.Fatal("missing inline icon accepted")
	}
}
