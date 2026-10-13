package routes

import (
	"testing"

	"blog/internal/content"
	"blog/internal/search"
)

func TestStrictSearchStartupAndContentLifecycle(t *testing.T) {
	for _, useEvents := range []bool{false, true} {
		name := "store-hooks"
		if useEvents {
			name = "content-events"
		}
		t.Run(name, func(t *testing.T) {
			ws, st, _, _ := stellarRouteFixture(t)
			row := createStellarPost(t, st, "canonical-search", "")
			row.ContentMD = "CurrentCanonicalOnly"
			row.ContentHTML = "<p>ObsoleteCachedOnly</p>"
			if err := st.Update(row.Slug, row); err != nil {
				t.Fatal(err)
			}
			if err := ws.searchIdx.IndexDoc(search.TypePost, row.Slug, row.Title, "ObsoleteCachedOnly"); err != nil {
				t.Fatal(err)
			}
			var events content.Events
			if useEvents {
				events = content.NewEvents()
				st.SetContentEvents(events)
			}
			server, err := New(Dependencies{Store: st, Site: ws.site, Posts: ws.posts, Search: ws.searchIdx, Events: events})
			if err != nil {
				t.Fatal(err)
			}
			defer server.Close()
			check := func(word string, want int) {
				t.Helper()
				got, err := ws.searchIdx.Search(word, search.TypePost, 20)
				if err != nil || len(got) != want {
					t.Fatalf("%s: hits=%d want=%d err=%v", word, len(got), want, err)
				}
			}
			check("CurrentCanonicalOnly", 1)
			check("ObsoleteCachedOnly", 0)
			row.ContentMD = "UpdatedCanonicalOnly"
			if err := st.Update(row.Slug, row); err != nil {
				t.Fatal(err)
			}
			check("UpdatedCanonicalOnly", 1)
			check("CurrentCanonicalOnly", 0)
			check("ObsoleteCachedOnly", 0)
			if events != nil {
				if err := ws.searchIdx.IndexDoc(search.TypePost, row.Slug, row.Title, "RestoredStaleOnly"); err != nil {
					t.Fatal(err)
				}
				events.Publish(content.Event{Kind: content.Restored})
				check("UpdatedCanonicalOnly", 1)
				check("RestoredStaleOnly", 0)
			}
			persisted, err := st.GetBySlug(row.Slug)
			if err != nil || persisted.ContentHTML != row.ContentHTML {
				t.Fatal("search rebuild rewrote preserved legacy HTML")
			}
		})
	}
}
