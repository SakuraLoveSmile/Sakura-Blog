package web

import (
	"strings"
	"testing"

	tmpl "blog/internal/settings"
)

func TestTagsTemplateRendersDefaultHashtagIcon(t *testing.T) {
	icons, err := LoadIconsEmbed()
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngineFromEmbed(icons)
	if err != nil {
		t.Fatal(err)
	}
	html, err := engine.Render("partial/tags", &tmpl.ViewData{
		Site:     &tmpl.Site{},
		Page:     &tmpl.Page{Layout: "tags", MenuID: "post"},
		TagItems: []*tmpl.TagItem{{Name: "Stellar"}, {Name: "自定义", Icon: "quot:hashtag"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(html, `class="tag"`) != 2 {
		t.Fatalf("tag links = %d, want 2", strings.Count(html, `class="tag"`))
	}
	if !strings.Contains(html, `href="/tags/Stellar"`) || !strings.Contains(html, "<svg") {
		t.Fatalf("tags page missing hashtag icon or link:\n%s", html)
	}
}
