package web

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"blog/internal/settings"
)

func TestStellarArchivesSortCanonicalDatesWithoutMutatingInput(t *testing.T) {
	for _, author := range []bool{false, true} {
		d := stellarTemplateFixture(t)
		if author {
			d.AuthorProfile = &settings.AuthorDef{}
		}
		item := func(title string, date any, listed bool) map[string]any {
			return map[string]any{"date": "2099-01-01", "render": map[string]any{"listing": map[string]any{"title": title, "date": date, "href": "/" + title, "listed": listed}}}
		}
		d.Stellar.ListItems = []map[string]any{
			item("July", "2024-07-20T00:00:00Z", true),
			item("Old", "2023-12-31", true),
			item("October", "2024-10-22T00:00:00Z", true),
			item("October-tie", "2024-10-22T00:00:00Z", true),
			item("New", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), true),
			item("February", "2024-02-21 00:00:00", true),
			item("Hidden", "2026-01-01", false),
			item("Undated", "", true),
		}
		before, _ := json.Marshal(d.Stellar.ListItems)
		got := string((stellarTemplateRenderer{}).archives(d))
		previous := -1
		for _, marker := range []string{`archive-header h4">2025`, `href="/New"`, `archive-header h4">2024`, `href="/October"`, `href="/October-tie"`, `href="/July"`, `href="/February"`, `archive-header h4">2023`, `href="/Old"`} {
			index := strings.Index(got, marker)
			if index <= previous {
				t.Fatalf("author=%v: archive marker %q missing or out of order: %s", author, marker, got)
			}
			previous = index
		}
		if strings.Contains(got, "Hidden") || strings.Contains(got, "Undated") || strings.Contains(got, "2099") {
			t.Fatalf("archive must use visible canonical projection: %s", got)
		}
		after, _ := json.Marshal(d.Stellar.ListItems)
		if string(before) != string(after) {
			t.Fatal("archive rendering mutated the shared listing snapshot")
		}
	}
}
