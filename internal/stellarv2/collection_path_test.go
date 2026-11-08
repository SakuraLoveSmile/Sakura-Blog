package stellarv2

import "testing"

func TestCollectionBasePath(t *testing.T) {
	theme := map[string]any{"profiles": map[string]any{
		"notebooks": map[string]any{"path": "/notebooks/"},
		"wikiIndex": map[string]any{"path": "docs"},
		"topic":     map[string]any{"path": ""},
	}}
	cases := []struct {
		name         string
		theme        map[string]any
		profile, id  string
		route, wants string
	}{
		{"route path wins and is normalised", theme, "notebooks", "journal", "/my/notes/", "/my/notes"},
		{"index path plus id", theme, "notebooks", "journal", "", "/notebooks/journal"},
		{"index path without slashes", theme, "wikiIndex", "go", "", "/docs/go"},
		{"empty index path falls back to root", theme, "topic", "weekly", "", "/weekly"},
		{"missing profile falls back to root", theme, "notebooks2", "x", "", "/x"},
		{"nil theme falls back to root", nil, "notebooks", "x", "", "/x"},
		{"id is path escaped", theme, "notebooks", "日记 本", "", "/notebooks/%E6%97%A5%E8%AE%B0%20%E6%9C%AC"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CollectionBasePath(tc.theme, tc.profile, tc.id, tc.route); got != tc.wants {
				t.Fatalf("CollectionBasePath = %q, want %q", got, tc.wants)
			}
		})
	}
}
