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
		{Name: "HTTP to HTTPS", GroupTitle: "友链", URL: "http://redirect.example", LastStatus: http.StatusMovedPermanently, LastCheckedAt: 10},
		{Name: "temporary redirect", GroupTitle: "友链", URL: "https://temporary.example", LastStatus: http.StatusFound, LastCheckedAt: 10},
		{Name: "preserved method redirect", GroupTitle: "友链", URL: "https://method.example", LastStatus: http.StatusTemporaryRedirect, LastCheckedAt: 10},
		{Name: "permanent redirect", GroupTitle: "友链", URL: "https://permanent.example", LastStatus: http.StatusPermanentRedirect, LastCheckedAt: 10},
		{Name: "last reachable status", GroupTitle: "友链", URL: "https://last.example", LastStatus: 399, LastCheckedAt: 10},
		{Name: "first HTTP error", GroupTitle: "友链", URL: "https://bad.example", LastStatus: http.StatusBadRequest, LastCheckedAt: 10},
		{Name: "http error", GroupTitle: "友链", URL: "https://down.example", LastStatus: http.StatusBadGateway, LastCheckedAt: 11},
		{Name: "network error", GroupTitle: "友链", URL: "https://timeout.example", LastStatus: 0, LastCheckedAt: 12},
		{Name: "manual offline", GroupTitle: "友链", URL: "https://manual.example", Label: "失联", LastStatus: http.StatusMovedPermanently, LastCheckedAt: 10},
		{Name: "manual expired", GroupTitle: "友链", URL: "https://expired.example", Label: " 失效 "},
	}

	groups := LinksToGroups(links)
	if len(groups) != 1 || len(groups[0].Items) != len(links) {
		t.Fatalf("groups = %+v", groups)
	}
	want := []bool{false, false, false, false, false, false, false, true, true, true, true, true}
	for i, item := range groups[0].Items {
		if item.Unavailable != want[i] {
			t.Errorf("item %q unavailable = %v, want %v", item.Name, item.Unavailable, want[i])
		}
	}
}
