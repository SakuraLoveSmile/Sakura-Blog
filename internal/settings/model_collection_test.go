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

func TestCollectionWidgetAdapters(t *testing.T) {
	popular := (&WidgetInstance{
		Layout: "popular",
		Data:   []*RecentPost{{URL: "/post/hot", Title: "热门文章"}},
	}).Collection()
	if popular.Layout != "list" || popular.Variant != "nav" || popular.Density != "compact" || len(popular.Items) != 1 {
		t.Fatalf("popular collection = %+v", popular)
	}

	links := []LinkListItem{
		{Title: "当前页", URL: "/library/index.html", Icon: "default:documents"},
		{Title: "外部页", URL: "https://example.com/docs", Icon: "default:github"},
		{Title: "第三项", URL: "/third", Icon: "default:rss"},
	}
	for _, tt := range []struct {
		name       string
		columns    int
		layout     string
		variant    string
		showTitles bool
	}{
		{"按项目数", 0, "grid", "icon", false},
		{"单列列表", 1, "list", "nav", true},
		{"双列网格", 2, "grid", "nav", true},
		{"三列图标", 3, "grid", "icon", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := (LinkListConfig{Columns: tt.columns, Items: links}).Collection("https://example.test/library/")
			if got.Layout != tt.layout || got.Variant != tt.variant || len(got.Items) != len(links) {
				t.Fatalf("collection = %+v", got)
			}
			if got.Items[0].Active != true || got.Items[1].Active {
				t.Fatalf("active links = %+v", got.Items)
			}
			for _, item := range got.Items {
				if item.ShouldShowTitle() != tt.showTitles {
					t.Fatalf("show title for %q = %v, want %v", item.Title, item.ShouldShowTitle(), tt.showTitles)
				}
			}
		})
	}

	related := (&WidgetInstance{
		Layout: "wiki_related",
		Data: []*WikiProject{{
			ID:          "go-blog",
			Title:       "Go 博客",
			Description: "项目描述",
			HasDocs:     true,
		}},
	}).Collection()
	if related.Layout != "list" || related.Variant != "summary" || len(related.Items) != 1 {
		t.Fatalf("wiki related collection = %+v", related)
	}
	if related.Items[0].Href != "/wiki/go-blog" || related.Items[0].Description != "项目描述" {
		t.Fatalf("wiki related item = %+v", related.Items[0])
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
