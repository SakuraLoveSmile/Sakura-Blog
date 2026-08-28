package config

import (
	"net/http"
	"testing"

	"blog/internal/store"
)

func TestLinksToGroupsDerivesUnavailable(t *testing.T) {
	links := []*store.Link{
		{Name: "never checked", GroupTitle: "友链", URL: "https://never.example", LastStatus: 0},
		{Name: "healthy", GroupTitle: "友链", URL: "https://healthy.example", LastStatus: http.StatusNoContent, LastCheckedAt: 10},
		{Name: "http error", GroupTitle: "友链", URL: "https://down.example", LastStatus: http.StatusBadGateway, LastCheckedAt: 11},
		{Name: "network error", GroupTitle: "友链", URL: "https://timeout.example", LastStatus: 0, LastCheckedAt: 12},
		{Name: "manual offline", GroupTitle: "友链", URL: "https://manual.example", Label: "失联"},
		{Name: "manual expired", GroupTitle: "友链", URL: "https://expired.example", Label: " 失效 "},
	}

	groups := LinksToGroups(links)
	if len(groups) != 1 || len(groups[0].Items) != len(links) {
		t.Fatalf("groups = %+v", groups)
	}
	want := []bool{false, false, true, true, true, true}
	for i, item := range groups[0].Items {
		if item.Unavailable != want[i] {
			t.Errorf("item %q unavailable = %v, want %v", item.Name, item.Unavailable, want[i])
		}
	}
}
