package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"blog/internal/store"
)

func TestStellarLegacyNotesEntryPreservesPagesAndDraftPrivacy(t *testing.T) {
	for _, root := range []string{"", "/sub"} {
		for _, target := range []string{"/notebooks", "/journals"} {
			for _, state := range []string{"absent", "draft", "deleted", "published"} {
				t.Run(root+target+"/"+state, func(t *testing.T) {
					ws, st, _, router := stellarRouteFixture(t)
					site := *ws.site.Load()
					site.URL = "https://fixture.example" + root
					site.StellarV2.Theme["profiles"].(map[string]any)["notebooks"].(map[string]any)["path"] = target
					ws.site.Store(&site)
					if state != "absent" {
						status, deleted := store.StatusPublished, ""
						if state == "draft" {
							status = store.StatusDraft
						}
						if state == "deleted" {
							deleted = "2026-08-28T09:28:57Z"
						}
						if _, err := st.CreatePage(&store.Page{Slug: "notes", Title: "Original notes page", ContentMD: "PRIVATE_PAGE_BODY", Status: status, DeletedAt: deleted, StellarVersion: 2, StellarConfig: `{}`}); err != nil {
							t.Fatal(err)
						}
					}
					if err := st.CreateNotebook(&store.Notebook{ID: "journal", StellarVersion: 2, StellarConfig: `{"name":"Journal"}`}); err != nil {
						t.Fatal(err)
					}
					if _, err := st.CreateNote(&store.Note{NotebookID: "journal", Slug: "test", Title: "PRIVATE_NOTE_TITLE", ContentMD: "PRIVATE_NOTE_BODY", Status: store.StatusDraft, StellarVersion: 2, StellarConfig: `{"collection":{"profile":"notebook","id":"journal"}}`}); err != nil {
						t.Fatal(err)
					}
					for _, suffix := range []string{"", "/"} {
						for _, method := range []string{http.MethodGet, http.MethodHead} {
							w := httptest.NewRecorder()
							router.ServeHTTP(w, httptest.NewRequest(method, root+"/notes"+suffix, nil))
							// Gin 对已注册 GET 路由先规范尾斜杠，再进入动态路由。
							if suffix == "/" && method == http.MethodGet && root == "" {
								if w.Code != http.StatusMovedPermanently || w.Header().Get("Location") != "/notes" {
									t.Fatalf("canonical notes entry: %d %q", w.Code, w.Header().Get("Location"))
								}
								w = stellarRequest(router, w.Header().Get("Location"))
							}
							if state == "published" {
								if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Original notes page") || w.Header().Get("Location") != "" {
									t.Fatalf("published page %s: %d %s", method, w.Code, w.Body.String())
								}
							} else if w.Code != http.StatusFound || w.Header().Get("Location") != root+target || strings.Contains(w.Body.String(), "PRIVATE_") {
								t.Fatalf("legacy entry %s: %d %q %s", method, w.Code, w.Header().Get("Location"), w.Body.String())
							}
						}
						w := stellarRequest(router, root+target)
						if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "PRIVATE_") {
							t.Fatalf("notebook directory: %d %s", w.Code, w.Body.String())
						}
					}
					if w := stellarRequest(router, root+"/notebooks/journal/test"); w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), "PRIVATE_") {
						t.Fatalf("draft note leaked: %d %s", w.Code, w.Body.String())
					}
					if state == "deleted" {
						var deleted string
						if err := st.DB().QueryRow("SELECT deleted_at FROM pages WHERE slug = 'notes'").Scan(&deleted); err != nil || deleted != "2026-08-28T09:28:57Z" {
							t.Fatalf("deleted page was changed: %q %v", deleted, err)
						}
					}
				})
			}
		}
	}
}

func TestStellarLegacyNotesEntryRespectsProfileOwnership(t *testing.T) {
	ws, _, _, router := stellarRouteFixture(t)
	site := *ws.site.Load()
	site.StellarV2.Theme["profiles"].(map[string]any)["notebooks"].(map[string]any)["path"] = "/notes"
	ws.site.Store(&site)
	w := stellarRequest(router, "/notes")
	if w.Code != http.StatusOK || w.Header().Get("Location") != "" {
		t.Fatalf("notebook profile redirected to itself: %d %q", w.Code, w.Header().Get("Location"))
	}
	site.StellarV2 = nil
	ws.site.Store(&site)
	w = stellarRequest(router, "/notes")
	if w.Code != http.StatusConflict {
		t.Fatalf("missing migration was bypassed: %d %s", w.Code, w.Body.String())
	}
}
