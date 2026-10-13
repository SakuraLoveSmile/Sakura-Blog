package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"blog/internal/config"
	"blog/internal/post"
	"blog/internal/settings"
	"blog/internal/store"
	"blog/modules/console"
	"blog/modules/stellar"
	"github.com/gin-gonic/gin"
)

func TestConsoleRepairsPublishedBodyThroughStellarPublicContext(t *testing.T) {
	root := t.TempDir()
	st, err := store.New(filepath.Join(root, "data", "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureUsersTable(); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsurePostAuthorsTable(); err != nil {
		t.Fatal(err)
	}
	version, err := config.InitializeStellarV2(st, map[string]any{}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	site := &atomic.Pointer[settings.Site]{}
	site.Store(&settings.Site{URL: "http://127.0.0.1:8109", StellarV2: version})
	posts := post.NewService(filepath.Join(root, "content", "posts"))
	icons, err := settings.LoadIconsEmbed()
	if err != nil {
		t.Fatal(err)
	}
	row := &store.Post{Slug: "repair", Title: "Repair", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`, ContentMD: "{% tabs %}"}
	if _, err := st.Create(row); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	server, err := stellar.Mount(r, stellar.Dependencies{App: &config.App{Root: root, DBPath: filepath.Join(root, "data", "blog.db")}, Store: st, Site: site, Posts: posts, Icons: icons, Root: root, UploadDir: filepath.Join(root, "data", "uploads")})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	_, err = console.Mount(r, console.MountDeps{API: console.APIConfig{Store: st, Site: site, Posts: posts, RenderStellarMarkdown: server.RenderStellarMarkdown, RenderStellarContentPreview: server.RenderContentPreview}, DevMode: true, Root: root, Fallback: server.NoRoute(nil)})
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path string, payload any) *httptest.ResponseRecorder {
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	w := request(http.MethodGet, "/post/repair", nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("bad public body unexpectedly served: %d", w.Code)
	}
	replacement := "## Repaired\n\nValid candidate"
	payload := map[string]any{"title": "Repair", "content_md": replacement, "status": "published", "stellar_version": 2, "stellar_config": "{}", "blog_extensions": "{}"}
	w = request(http.MethodPut, "/api/v1/posts/repair", payload)
	if w.Code != http.StatusOK {
		t.Fatalf("valid repair rejected: %d %s", w.Code, w.Body.String())
	}
	saved, err := st.GetBySlug("repair")
	if err != nil || saved.ContentMD != replacement || !strings.Contains(saved.ContentHTML, "Valid candidate") {
		t.Fatalf("repair not persisted: %v %+v", err, saved)
	}
	w = request(http.MethodGet, "/post/repair", nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Valid candidate") {
		t.Fatalf("repaired body not served: %d", w.Code)
	}
	if _, err := st.Create(&store.Post{Slug: "unrelated", Title: "Unrelated", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`, ContentMD: "{% tabs %}"}); err != nil {
		t.Fatal(err)
	}
	payload["content_md"] = "Valid next revision"
	w = request(http.MethodPut, "/api/v1/posts/repair", payload)
	if w.Code != http.StatusOK {
		t.Fatalf("unrelated bad public body blocked update: %d %s", w.Code, w.Body.String())
	}
	w = request(http.MethodPost, "/api/v1/preview/markdown", map[string]any{"markdown": "Valid isolated preview", "title": "Repair", "layout": "post", "path": "/post/repair", "preview_token": "repair_preview_1234", "stellar_version": 2, "stellar_config": "{}", "blog_extensions": "{}"})
	var preview struct {
		HTML     string `json:"html"`
		Document string `json:"document"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &preview) != nil || !strings.Contains(preview.HTML, "Valid isolated preview") || !strings.Contains(preview.Document, "Valid isolated preview") {
		t.Fatalf("unrelated bad body blocked opaque preview: %d %s", w.Code, w.Body.String())
	}
	payload["content_md"] = "{% tabs %}"
	w = request(http.MethodPut, "/api/v1/posts/repair", payload)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid replacement accepted: %d %s", w.Code, w.Body.String())
	}
	saved, err = st.GetBySlug("repair")
	if err != nil || saved.ContentMD != "Valid next revision" {
		t.Fatalf("rejected replacement changed record: %v %+v", err, saved)
	}
	w = request(http.MethodGet, "/post/unrelated", nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("bad public body failure was suppressed: %d", w.Code)
	}
}
