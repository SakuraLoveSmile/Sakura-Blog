package web

import (
	"strings"
	"testing"

	"blog/internal/stellarmodels"
)

func TestStellarRecentWidgetSelectsMenuAndNotebookScope(t *testing.T) {
	for _, tc := range []struct {
		name, profile, menu, want, prefix string
		modelItems                        bool
	}{
		{name: "blog", profile: "post", menu: "post", want: "Blog recent"},
		{name: "wiki index", profile: "wikiIndex", menu: "wiki", want: "Wiki recent", prefix: "Wiki collection"},
		{name: "wiki document", profile: "wiki", menu: "wiki", want: "Wiki recent", prefix: "Wiki collection"},
		{name: "profile navigation alone retains posts", profile: "wiki", want: "Blog recent"},
		{name: "page active wiki", profile: "page", menu: "wiki", want: "Wiki recent", prefix: "Wiki collection"},
		{name: "notebook collections", profile: "notebooks", menu: "wiki", want: "Notebook recent", prefix: "Notebook collection"},
		{name: "notebook notes", profile: "notebook", menu: "wiki", want: "Notebook recent"},
		{name: "note model wins", profile: "notebook", menu: "wiki", want: "Model recent", modelItems: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := stellarTemplateFixture(t)
			d.Stellar.Profile = tc.profile
			d.Stellar.Navigation = map[string]any{"menu": "post"}
			d.Stellar.Data = map[string]any{
				"page":            map[string]any{"activeMenu": tc.menu},
				"recentItems":     []any{map[string]any{"href": "/blog-recent/", "title": "Blog recent"}},
				"wikiRecentItems": []any{map[string]any{"href": "/wiki-recent/", "title": "Wiki recent", "collectionName": "Wiki collection"}},
			}
			if tc.menu == "" {
				d.Stellar.Navigation = map[string]any{"menu": "wiki"}
				d.Stellar.Data["page"] = map[string]any{}
				stellarMap(stellarMap(d.Stellar.Theme["profiles"])[tc.profile])["activeMenu"] = "wiki"
			}
			if tc.profile == "notebooks" || tc.profile == "notebook" {
				d.Stellar.Data["recentItems"] = []any{map[string]any{"href": "/note-recent/", "title": "Notebook recent", "collectionName": "Notebook collection"}}
			}
			if tc.modelItems {
				d.Stellar.Model = &stellarmodels.PageModel{Render: map[string]any{"layout": map[string]any{"recentItems": []any{map[string]any{"href": "/model-recent/", "title": "Model recent", "collectionName": "Notebook collection"}}}}}
			}
			got := string((stellarTemplateRenderer{}).widget(d, "leftbar", "recent", map[string]any{"limit": 5}, nil))
			if !strings.Contains(got, tc.want) {
				t.Fatalf("wrong recent source: %s", got)
			}
			if tc.prefix != "" && !strings.Contains(got, tc.prefix) {
				t.Fatalf("missing collection prefix: %s", got)
			}
			if tc.profile == "notebook" && strings.Contains(got, "Notebook collection") {
				t.Fatalf("per-notebook recent must omit redundant prefix: %s", got)
			}
		})
	}
}
