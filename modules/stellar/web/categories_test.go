package web

import (
	"strings"
	"testing"

	tmpl "blog/internal/settings"
)

func TestCategoriesTemplateRendersRecursiveTree(t *testing.T) {
	engine, err := NewEngineFromEmbed(&tmpl.IconRegistry{})
	if err != nil {
		t.Fatal(err)
	}
	html, err := engine.Render("partial/categories", &tmpl.ViewData{
		Site: &tmpl.Site{},
		Page: &tmpl.Page{Layout: "categories", MenuID: "post"},
		Cats: []*tmpl.CatItem{{
			Name: "产研", Path: "产研", Count: 2,
			Children: []*tmpl.CatItem{{
				Name: "技术", Path: "产研/技术", Count: 2,
				Children: []*tmpl.CatItem{{Name: "前端", Path: "产研/技术/前端", Count: 1}},
			}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`href="/categories/`, "cat-children", `class="cat child"`, "前端"} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered categories missing %q", want)
		}
	}
}
