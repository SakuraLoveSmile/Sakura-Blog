package routes

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"blog/internal/config"
	"blog/internal/settings"
	"blog/internal/stellarv2"
	"blog/internal/store"
	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"
)

func TestStellarDetailsDefaultAuthorUsesSavedDocumentOrder(t *testing.T) {
	for _, tc := range []struct {
		name     string
		authors  map[string]any
		selected any
		want     string
	}{
		{"saved names", map[string]any{"zoe": map[string]any{"name": "Zoe"}, "alice.dev": map[string]any{"name": "Alice", "desc": "Author bio", "password": "PRIVATE_AUTHOR"}}, nil, "alice.dev"},
		{"integer keys", map[string]any{"10": map[string]any{"name": "Ten"}, "2": map[string]any{"name": "Two"}, "01": map[string]any{"name": "Leading zero"}}, nil, "2"},
		{"not an array index", map[string]any{"4294967295": map[string]any{"name": "Not index"}, "100": map[string]any{"name": "Index"}}, nil, "100"},
		{"explicit string", map[string]any{"alice": map[string]any{"name": "Alice"}, "bob": map[string]any{"name": "Bob"}}, "bob", "bob"},
		{"explicit object", map[string]any{"alice": map[string]any{"name": "Alice"}, "bob": map[string]any{"name": "Bob"}}, map[string]any{"id": "bob"}, "bob"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			site := &tmpl.Site{StellarV2: &settings.StellarV2Config{Theme: stellarv2.DefaultTheme(), Blog: map[string]any{"data": map[string]any{"authors": tc.authors, "defaultAuthor": tc.selected}}}}
			data := stellarData(site)
			author := stellarObject(data["defaultAuthor"])
			if author["id"] != tc.want || author["name"] != stellarObject(tc.authors[tc.want])["name"] {
				t.Fatalf("default author = %#v, want %s with public fields", author, tc.want)
			}
			for id, raw := range stellarObject(data["authors"]) {
				a := stellarObject(raw)
				if a["id"] != id || stellarString(a["path"]) == "" {
					t.Errorf("author %s lacks identity/path: %#v", id, a)
				}
			}
			if tc.want == "alice.dev" && author["desc"] != "Author bio" {
				t.Error("upstream desc was lost")
			}
			raw, _ := json.Marshal(data)
			if strings.Contains(string(raw), "PRIVATE_AUTHOR") {
				t.Fatal("non-public author field leaked")
			}
		})
	}
	site := &tmpl.Site{StellarV2: &settings.StellarV2Config{Theme: stellarv2.DefaultTheme(), Blog: map[string]any{}}}
	if got := stellarData(site)["defaultAuthor"]; got != nil {
		t.Fatalf("empty author catalog should have no default: %#v", got)
	}
}

func TestStellarDetailsDefaultAuthorAndFooterThroughActualTemplate(t *testing.T) {
	ws, st, capture, router := stellarRouteFixture(t)
	site := *ws.site.Load()
	v2 := *site.StellarV2
	v2.Blog = stellarCopyObject(v2.Blog)
	v2.Blog["site"] = map[string]any{"author": "Site Owner", "per_page": 2}
	v2.Blog["data"] = map[string]any{"authors": map[string]any{"alice": map[string]any{"name": "Alice"}, "bob": map[string]any{"name": "Bob"}}}
	v2.Theme = stellarCopyObject(v2.Theme)
	v2.Theme["footer"] = map[string]any{"content": "Written by {author.name}"}
	site.StellarV2 = &v2
	ws.site.Store(&site)
	if _, err := config.SaveStellarV2(st, map[string]any{}, v2.Blog); err != nil {
		t.Fatal(err)
	}
	createStellarPost(t, st, "default-author", "")
	createStellarPost(t, st, "explicit-author", `,"article":{"author":"bob"}`)
	for _, tc := range []struct{ slug, id string }{{"default-author", "alice"}, {"explicit-author", "bob"}} {
		response := stellarRequest(router, "/post/"+tc.slug)
		if response.Code != http.StatusOK || len(capture.last.Page.AuthorLinks) != 1 || capture.last.Page.AuthorLinks[0].ID != tc.id {
			t.Fatalf("%s: author links missing or wrong: status=%d links=%#v", tc.slug, response.Code, capture.last.Page.AuthorLinks)
		}
		if got := stellarGet(capture.last.Stellar.Input.RuntimeData, "authorName"); got != "Site Owner" {
			t.Fatalf("site author not projected: %#v", got)
		}
	}
	engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
	if err != nil {
		t.Fatal(err)
	}
	ws.engine = engine
	response := stellarRequest(router, "/post/default-author")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Written by Site Owner") || strings.Contains(response.Body.String(), "Written by Anonymity") {
		t.Fatal("footer did not use the configured site author")
	}
}

func TestStellarDetailsRecentUsesUpdatedAndPublicCollectionScope(t *testing.T) {
	ws, st, capture, router := stellarRouteFixture(t)
	for _, tc := range []struct{ slug, created, updated, extra string }{
		{"old-revised", "2020-01-01", "2026-10-24", ""},
		{"new-post", "2026-10-01", "2026-10-02", ""},
		{"unlisted", "2026-10-23", "2026-10-25", `,"visibility":{"listed":false}`},
	} {
		row := createStellarPost(t, st, tc.slug, tc.extra)
		if _, err := st.DB().Exec("UPDATE posts SET created_at=?, updated_at=? WHERE id=?", tc.created+"T00:00:00Z", tc.updated+"T00:00:00Z", row.ID); err != nil {
			t.Fatal(err)
		}
	}
	for i, id := range []string{"docs", "guide", "hidden"} {
		if err := st.CreateWikiProject(&store.WikiProject{ID: id, Visible: true, StellarVersion: 2, StellarConfig: `{"name":"Wiki ` + id + `","route":{"path":"/` + id + `/"}}`}); err != nil {
			t.Fatal(err)
		}
		if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: id, Slug: "intro", Title: "Doc " + id, Status: store.StatusPublished, ContentMD: "Document", StellarVersion: 2, StellarConfig: `{"collection":{"profile":"wiki","id":"` + id + `"}}`}); err != nil {
			t.Fatal(err)
		}
		stamp := time.Date(2026, 10, 20+i, 0, 0, 0, 0, time.UTC)
		if _, err := st.DB().Exec("UPDATE wikidocs SET updated_at=? WHERE project_id=?", stamp, id); err != nil {
			t.Fatal(err)
		}
		if err := st.CreateNotebook(&store.Notebook{ID: id, StellarVersion: 2, StellarConfig: `{"name":"Notebook ` + id + `","visibility":{"listed":` + map[bool]string{true: "false", false: "true"}[id == "hidden"] + `}}`}); err != nil {
			t.Fatal(err)
		}
		if _, err := st.CreateNote(&store.Note{NotebookID: id, Slug: "entry", Title: "Note " + id, Status: store.StatusPublished, ContentMD: "Note body", StellarVersion: 2, StellarConfig: `{"collection":{"profile":"notebook","id":"` + id + `"}}`}); err != nil {
			t.Fatal(err)
		}
		if _, err := st.DB().Exec("UPDATE notes SET updated_at=? WHERE notebook_id=?", stamp, id); err != nil {
			t.Fatal(err)
		}
	}
	site := *ws.site.Load()
	v2 := *site.StellarV2
	v2.Blog = stellarCopyObject(v2.Blog)
	data := stellarCopyObject(stellarObject(v2.Blog["data"]))
	data["wiki"] = []any{"docs", "guide"}
	v2.Blog["data"] = data
	site.StellarV2 = &v2
	ws.site.Store(&site)
	if response := stellarRequest(router, "/"); response.Code != http.StatusOK {
		t.Fatal(response.Body.String())
	}
	assertTitles := func(raw any, want []string) {
		t.Helper()
		items := stellarArray(raw)
		got := []string{}
		for _, item := range items {
			got = append(got, stellarString(stellarObject(item)["title"]))
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("recent titles %v, want %v", got, want)
		}
	}
	assertTitles(capture.last.Stellar.Input.RuntimeData["recentItems"], []string{"old-revised", "new-post"})
	for _, path := range []string{"/wiki", "/docs/intro"} {
		if response := stellarRequest(router, path); response.Code != http.StatusOK {
			t.Fatal(response.Body.String())
		}
		items := capture.last.Stellar.Input.RuntimeData["wikiRecentItems"]
		assertTitles(items, []string{"Doc guide", "Doc docs"})
		if stellarObject(stellarArray(items)[0])["collectionName"] != "Wiki guide" {
			t.Fatal("Wiki prefix missing")
		}
	}
	if response := stellarRequest(router, "/notebooks"); response.Code != http.StatusOK {
		t.Fatal(response.Body.String())
	}
	items := capture.last.Stellar.Input.RuntimeData["recentItems"]
	assertTitles(items, []string{"Note guide", "Note docs"})
	if stellarObject(stellarArray(items)[0])["collectionName"] != "Notebook guide" {
		t.Fatal("Notebook prefix missing")
	}
	if response := stellarRequest(router, "/notebooks/docs"); response.Code != http.StatusOK {
		t.Fatal(response.Body.String())
	}
	assertTitles(capture.last.Stellar.Input.RuntimeData["recentItems"], []string{"Note docs"})
}
