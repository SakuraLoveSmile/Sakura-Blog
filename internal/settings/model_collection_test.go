package settings

import (
	"strings"
	"testing"
)

func TestCollectionNormalizesUpstreamVariants(t *testing.T) {
	got := (Collection{Layout: "table", Variant: "unknown", Density: "wide", Columns: 0}).Normalized()
	if got.Layout != "list" || got.Variant != "nav" || got.Density != "auto" || got.Columns != 1 {
		t.Fatalf("collection defaults = %+v", got)
	}

	item := NewCollectionItem(`/post/a?x="1"`, `标题 <危险>`)
	item.Active = true
	item.LegacyLink = true
	item.Theme = "hsl(192 98% 55%)"
	item.Depth = 2
	item.GradientID = "menu-grad-a"
	if !item.ShouldShowTitle() || !item.ShouldShowIndicator() {
		t.Fatal("upstream collection item defaults should show title and indicator")
	}
	if got := item.ClassAttribute(); got != "link active ui-collection__item is-active" {
		t.Fatalf("active legacy class = %q", got)
	}
	if got := string(item.Style()); got != "--item-theme:hsl(192 98% 55%);--ui-item-depth:2;--item-grad:url(#menu-grad-a);" {
		t.Fatalf("style = %q", got)
	}

	item.Theme = `red;--bad:url(javascript:alert(1))`
	if got := string(item.Style()); strings.Contains(got, "javascript") || strings.Contains(got, "--bad") {
		t.Fatalf("unsafe theme escaped into CSS: %q", got)
	}

	item.ShowTitleSet = true
	item.ShowTitle = false
	if item.ShouldShowTitle() {
		t.Fatal("explicit showTitle=false should hide the title")
	}
}

func TestWidgetFrameAttributesAllowList(t *testing.T) {
	frame := WidgetFrame{Attrs: map[string]string{
		"data-wiki": "stellar",
		"id":        "data-tree",
		"onclick":   "alert(1)",
	}}
	attrs := frame.Attributes()
	if len(attrs) != 2 || attrs[0].Name != "data-wiki" || attrs[1].Name != "id" {
		t.Fatalf("allow-listed attrs = %+v", attrs)
	}
}
