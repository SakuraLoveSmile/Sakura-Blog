package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"blog/internal/config"
	"blog/internal/settings"
	"blog/internal/store"

	"github.com/gin-gonic/gin"
)

func newNotebookAPI(t *testing.T) (*gin.Engine, *store.Store, *Handler, *store.User, *store.User, *store.User) {
	t.Helper()
	r, st, admin, h := newFullEnv(t)
	if err := st.EnsureSettingsTable(); err != nil {
		t.Fatal(err)
	}
	v2, err := config.InitializeStellarV2(st, map[string]any{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	next := *h.site.Load()
	next.StellarV2 = v2
	h.site.Store(&next)
	alice, _ := st.GetUserByUsername("alice")
	bob, _ := st.GetUserByUsername("bob")
	return r, st, h, admin, alice, bob
}

func notebookPayload(id, raw string) string {
	return marshalNotebookRequest(NotebookRequest{ID: &id, StellarVersion: ptrNotebook(2), StellarConfig: &raw, BlogExtensions: ptrNotebook(` {"value":false,"none":null,"large":900719925474099312345} `)})
}

func ptrNotebook[T any](value T) *T { return &value }

func marshalNotebookRequest(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func TestNotebookAPILegacyLayoutReturnsLocatedMigrationGate(t *testing.T) {
	r, st, _, admin, _, _ := newNotebookAPI(t)
	// newFullEnv 使用隔离库；只在此夹具换回真实旧表结构。
	if _, err := st.DB().Exec(`DROP TABLE notes; DROP TABLE notebooks;
	CREATE TABLE notebooks(id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE, description TEXT NOT NULL DEFAULT '', cover TEXT NOT NULL DEFAULT '', nb_order INTEGER NOT NULL DEFAULT 0, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
	CREATE TABLE notes(id INTEGER PRIMARY KEY AUTOINCREMENT, notebook_id INTEGER NOT NULL, title TEXT NOT NULL DEFAULT '', content TEXT NOT NULL DEFAULT '', content_html TEXT NOT NULL DEFAULT '', tags TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(notebook_id) REFERENCES notebooks(id));
	INSERT INTO notebooks(id,name) VALUES(2,'private notebook');
	INSERT INTO notes(id,notebook_id,content) VALUES(2,2,'private note body');`); err != nil {
		t.Fatal(err)
	}
	for _, request := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/notebooks", ""},
		{http.MethodGet, "/api/v1/notebooks/2", ""},
		{http.MethodPost, "/api/v1/notebooks", notebookPayload("new", `{}`)},
		{http.MethodPut, "/api/v1/notebooks/2", notebookPayload("2", `{}`)},
		{http.MethodDelete, "/api/v1/notebooks/2", ""},
		{http.MethodGet, "/api/v1/notebooks/2/notes", ""},
		{http.MethodGet, "/api/v1/notebooks/2/notes/2", ""},
		{http.MethodDelete, "/api/v1/notebooks/2/notes/2", ""},
		{http.MethodPost, "/api/v1/notebooks/2/notes/2/publish", ""},
	} {
		w := do2FA(r, request.method, request.path, request.body, authCookie(admin))
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "migration required") || !strings.Contains(w.Body.String(), "notebooks schema:") || !strings.Contains(w.Body.String(), "notes schema:") {
			t.Errorf("%s %s failed to locate migration: %d %s", request.method, request.path, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "private") || strings.Contains(w.Body.String(), "SQL logic error") {
			t.Error("migration response exposed content or raw SQL")
		}
	}
	var body string
	if err := st.DB().QueryRow(`SELECT content FROM notes WHERE id=2 AND notebook_id=2`).Scan(&body); err != nil || body != "private note body" {
		t.Fatal("rejected API call changed legacy identity/body")
	}
}

func TestNotebookNoteAPIRenderUsesCanonicalHostAndPageLanguage(t *testing.T) {
	r, _, h, _, alice, _ := newNotebookAPI(t)
	site := *h.site.Load()
	site.StellarV2 = &settings.StellarV2Config{Theme: map[string]any{}, Blog: map[string]any{"site": map[string]any{"language": []any{"", "en"}}}}
	h.site.Store(&site)
	w := do2FA(r, http.MethodPost, "/api/v1/notebooks", notebookPayload("book", `{}`), authCookie(alice))
	if w.Code != http.StatusCreated {
		t.Fatal(w.Body.String())
	}
	for _, test := range []struct{ slug, raw, want string }{
		{"host", `{}`, "Architect"},
		{"page", `{"lang":["","zh-TW"]}`, "架構師"},
	} {
		request := NotebookNoteRequest{Slug: &test.slug, ContentMD: ptrNotebook("{% mbti INTJ-T 93-72-78-83-51 %}"), StellarVersion: ptrNotebook(2), StellarConfig: &test.raw}
		w := do2FA(r, http.MethodPost, "/api/v1/notebooks/book/notes", marshalNotebookRequest(request), authCookie(alice))
		var note NotebookNoteResponse
		if w.Code != http.StatusCreated || json.Unmarshal(w.Body.Bytes(), &note) != nil || !strings.Contains(note.ContentHTML, test.want) {
			t.Fatalf("%s language lost: %d %s", test.slug, w.Code, w.Body.String())
		}
	}
}

func TestNotebookAPIOwnershipAndStrictMetadata(t *testing.T) {
	r, st, _, admin, alice, bob := newNotebookAPI(t)
	raw := ` {"name":"工作","description":"笔记本","leftbar":{"widgets":[{"custom":{"large":900719925474099312345,"flag":false}}]}} `
	w := do2FA(r, http.MethodPost, "/api/v1/notebooks", notebookPayload("work", raw), authCookie(alice))
	if w.Code != http.StatusCreated {
		t.Fatalf("create notebook: %d %s", w.Code, w.Body.String())
	}
	created, err := st.GetNotebook("work")
	if err != nil || created.AuthorID != alice.ID || created.StellarConfig != raw {
		t.Fatalf("ownership/raw bytes lost: %v", err)
	}
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		w := do2FA(r, method, "/api/v1/notebooks/work", `{}`, authCookie(bob))
		if w.Code != http.StatusForbidden {
			t.Fatalf("other author %s accepted: %d", method, w.Code)
		}
	}
	w = do2FA(r, http.MethodGet, "/api/v1/notebooks", "", authCookie(bob))
	var list NotebookListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || list.Total != 0 {
		t.Fatal("list disclosed another author's notebook")
	}
	w = do2FA(r, http.MethodGet, "/api/v1/notebooks/work", "", authCookie(admin))
	if w.Code != http.StatusOK {
		t.Fatal("admin cannot read author's notebook")
	}
	for _, payload := range []string{`{"author_id":999}`, `{"id":"moved"}`, `null`, `{"stellar_version":0}`, `{"stellar_config":"{\"style\":{}}"}`} {
		w := do2FA(r, http.MethodPut, "/api/v1/notebooks/work", payload, authCookie(alice))
		if w.Code != http.StatusBadRequest && w.Code != http.StatusConflict {
			t.Fatalf("invalid update accepted: %d", w.Code)
		}
	}
	unchanged, _ := st.GetNotebook("work")
	if !reflect.DeepEqual(created, unchanged) {
		t.Fatal("invalid update changed notebook")
	}
	w = do2FA(r, http.MethodPost, "/api/v1/notebooks", notebookPayload("work", `{}`), authCookie(alice))
	if w.Code != http.StatusConflict {
		t.Fatal("duplicate notebook not rejected")
	}
}

func TestNotebookNoteAPILifecycleTagPaginationAndIsolation(t *testing.T) {
	r, st, h, admin, alice, bob := newNotebookAPI(t)
	for _, id := range []string{"work", "other"} {
		w := do2FA(r, http.MethodPost, "/api/v1/notebooks", notebookPayload(id, `{}`), authCookie(alice))
		if w.Code != http.StatusCreated {
			t.Fatal(w.Body.String())
		}
	}
	raw := ` {"title":"正文","collection":{"profile":"notebook","id":"work"},"comments":{"enabled":false,"title":null}} `
	request := NotebookNoteRequest{Slug: ptrNotebook("nested/first"), ContentMD: ptrNotebook("正文\n\n{% button 查看 /target %}"), Tags: ptrNotebook([]string{"A/B", "A/B", "100%/under_score"}), StellarVersion: ptrNotebook(2), StellarConfig: &raw, BlogExtensions: ptrNotebook(` {"zero":0,"false":false} `)}
	w := do2FA(r, http.MethodPost, "/api/v1/notebooks/work/notes", marshalNotebookRequest(request), authCookie(alice))
	if w.Code != http.StatusCreated {
		t.Fatalf("create note: %d %s", w.Code, w.Body.String())
	}
	var note NotebookNoteResponse
	if err := json.Unmarshal(w.Body.Bytes(), &note); err != nil || note.AuthorID != alice.ID || note.Status != store.StatusDraft || note.StellarConfig != raw || !strings.Contains(note.ContentHTML, `class="tag-plugin colorful button"`) {
		t.Fatalf("note creation contract failed: %v %s", err, note.ContentHTML)
	}
	path := fmt.Sprintf("/api/v1/notebooks/work/notes/%d", note.ID)
	for _, cookie := range []string{authCookie(bob), ""} {
		w := do2FA(r, http.MethodGet, path, "", cookie)
		if w.Code != http.StatusForbidden && w.Code != http.StatusUnauthorized {
			t.Fatal("note auth boundary missing")
		}
	}
	w = do2FA(r, http.MethodGet, fmt.Sprintf("/api/v1/notebooks/other/notes/%d", note.ID), "", authCookie(alice))
	if w.Code != http.StatusNotFound {
		t.Fatal("note crossed notebook boundary")
	}
	w = do2FA(r, http.MethodPut, path, `{"notebook_id":"other"}`, authCookie(admin))
	if w.Code != http.StatusBadRequest {
		t.Fatal("implicit notebook move accepted")
	}
	w = do2FA(r, http.MethodPost, path+"/publish", `{}`, authCookie(alice))
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	if published, _ := st.GetPublishedNote("work", note.Slug); published == nil {
		t.Fatal("publish did not change public status")
	}
	w = do2FA(r, http.MethodGet, "/api/v1/notebooks/work/notes?tag=a&status=published&page=1&size=1", "", authCookie(alice))
	var list NotebookNoteListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || list.Total != 1 || len(list.Items) != 1 {
		t.Fatalf("hierarchical tag/pagination query failed: %v", err)
	}
	w = do2FA(r, http.MethodPut, path+"/draft", `{"content_md":"新草稿"}`, authCookie(alice))
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	if published, _ := st.GetPublishedNote("work", note.Slug); published != nil {
		t.Fatal("draft remained public")
	}
	w = do2FA(r, http.MethodDelete, "/api/v1/notebooks/work", "", authCookie(alice))
	if w.Code != http.StatusConflict {
		t.Fatal("nonempty notebook deleted")
	}
	before, _ := st.GetNoteByID(note.ID)
	w = do2FA(r, http.MethodPut, path, `{"stellar_config":"{\"collection\":{\"profile\":\"notebook\",\"id\":\"other\"}}"}`, authCookie(alice))
	if w.Code != http.StatusBadRequest {
		t.Fatal("inconsistent collection accepted")
	}
	after, _ := st.GetNoteByID(note.ID)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("invalid collection changed stored note")
	}
	// 严格配置必须经过当前主题渲染；不能退回旧 renderer。
	next := *h.site.Load()
	next.StellarV2 = nil
	h.site.Store(&next)
	w = do2FA(r, http.MethodPut, path, `{"content_md":"不能保存"}`, authCookie(alice))
	if w.Code != http.StatusConflict {
		t.Fatal("missing migrated theme accepted")
	}
	current := *h.site.Load()
	current.StellarV2 = &settings.StellarV2Config{Theme: map[string]any{}, Blog: map[string]any{}}
	h.site.Store(&current)
	w = do2FA(r, http.MethodDelete, path, "", authCookie(alice))
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	w = do2FA(r, http.MethodDelete, "/api/v1/notebooks/work", "", authCookie(alice))
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
}

func TestNotebookUnmigratedRecordsRemainReadOnly(t *testing.T) {
	r, st, _, admin, alice, _ := newNotebookAPI(t)
	if _, err := st.DB().Exec(`INSERT INTO notebooks(id,name,stellar_version) VALUES('legacy','旧本',0)`); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		w := do2FA(r, method, "/api/v1/notebooks/legacy", `{}`, authCookie(admin))
		if w.Code != http.StatusConflict {
			t.Fatalf("legacy notebook did not require migration: %d", w.Code)
		}
	}
	w := do2FA(r, http.MethodGet, "/api/v1/notebooks/legacy", "", authCookie(alice))
	if w.Code != http.StatusForbidden {
		t.Fatal("owner0 legacy record exposed to author")
	}
	legacy, _ := st.GetNotebook("legacy")
	if legacy == nil || legacy.AuthorID != 0 || legacy.StellarVersion != 0 {
		t.Fatal("legacy ownership/version guessed")
	}
}
