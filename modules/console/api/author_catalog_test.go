package api

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"blog/internal/config"
	tmpl "blog/internal/settings"
	"blog/internal/store"
)

func TestStellarAuthorCatalogUsesCanonicalPublicProjectionForAuthors(t *testing.T) {
	r, st, admin, h := newFullStellarEnv(t)
	blog := `{"data":{"authors":{"admin":{"name":"管理员","avatar":"/admin.png","description":"公开简介","url":"https://example.test/admin","secret":"author-private"},"writer":{"name":"作者","hidden":true}}},"ai":{"api_key":"model-private"},"native_comments":{"notify":{"password":"smtp-private"}},"private":"extension-private"}`
	if err := st.PutSetting(config.SectionBlogExtensions, blog); err != nil {
		t.Fatal(err)
	}
	if err := st.PutSetting(config.SectionAuthors, `{"legacy":{"name":"旧作者"}}`); err != nil {
		t.Fatal(err)
	}
	current := *h.site.Load()
	current.Authors = map[string]tmpl.AuthorDef{"runtime": {Name: "运行时旧作者"}}
	h.site.Store(&current)
	author := &store.User{Username: "editor", PasswordHash: "x", AuthSecret: store.GenerateAuthSecret(), Role: store.RoleAuthor, Permissions: []string{store.PermissionContentWrite, store.PermissionConsoleAccess}, Status: store.UserStatusActive}
	if err := st.CreateUser(author); err != nil {
		t.Fatal(err)
	}
	before, _ := st.ReadSettingsSnapshot()
	for _, user := range []*store.User{admin, author} {
		w := do2FA(r, http.MethodGet, "/api/v1/authors/catalog", "", authCookie(user))
		if w.Code != http.StatusOK {
			t.Fatalf("catalog %s: %d %s", user.Username, w.Code, w.Body.String())
		}
		var result StellarAuthorCatalogResponse
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Authors) != 2 || result.Authors["admin"].Name != "管理员" || result.Authors["admin"].Avatar != "/admin.png" || !result.Authors["writer"].Hidden {
			t.Fatalf("wrong catalog: %+v", result)
		}
		for _, private := range []string{"legacy", "runtime", "secret", "author-private", "model-private", "smtp-private", "extension-private", "ai", "native_comments"} {
			if strings.Contains(w.Body.String(), private) {
				t.Fatalf("catalog leaked %s", private)
			}
		}
	}
	w := do2FA(r, http.MethodGet, "/api/v1/settings", "", authCookie(author))
	if w.Code != http.StatusForbidden {
		t.Fatalf("catalog granted settings permission: %d", w.Code)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/authors/catalog", "", "")
	if w.Code == http.StatusOK {
		t.Fatal("unauthenticated catalog allowed")
	}
	after, _ := st.ReadSettingsSnapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("catalog modified settings")
	}
}

func TestStellarAuthorCatalogEmptyAndStrictErrorsNeverFallback(t *testing.T) {
	r, st, admin, h := newFullStellarEnv(t)
	cookie := authCookie(admin)
	current := *h.site.Load()
	current.Authors = map[string]tmpl.AuthorDef{"legacy": {Name: "旧作者"}}
	h.site.Store(&current)
	if err := st.PutSetting(config.SectionAuthors, `{"legacy":{"name":"旧作者"}}`); err != nil {
		t.Fatal(err)
	}
	if err := st.PutSetting(config.SectionBlogExtensions, `{}`); err != nil {
		t.Fatal(err)
	}
	w := do2FA(r, http.MethodGet, "/api/v1/authors/catalog", "", cookie)
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"authors":{}}` {
		t.Fatalf("empty catalog: %d %s", w.Code, w.Body.String())
	}
	if err := st.PutSetting(config.SectionBlogExtensions, `{"data":{"authors":{"bad":{"name":42}}}}`); err != nil {
		t.Fatal(err)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/authors/catalog", "", cookie)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "data.authors.bad.name") {
		t.Fatalf("schema diagnosis: %d %s", w.Code, w.Body.String())
	}
	if err := st.PutSetting(config.SectionStellarV2, `{}`); err != nil {
		t.Fatal(err)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/authors/catalog", "", cookie)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "stellar_migration_required") {
		t.Fatalf("migration diagnosis: %d %s", w.Code, w.Body.String())
	}
}
