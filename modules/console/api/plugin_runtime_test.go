package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/plugin"
	tmpl "blog/internal/settings"
	"blog/internal/store"
)

func TestScriptPluginRuntimeListReturnsEmptyWhenRuntimeIsAbsent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "secret")
	r := gin.New()
	if _, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site)}}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, runtimeListRequest(t, st))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var response ScriptPluginListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Items) != 0 {
		t.Fatalf("items = %#v, want empty", response.Items)
	}
}

func TestScriptPluginRuntimeListReturnsLoadedManifest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	runtime := plugin.NewRuntime()
	manifest := plugin.Manifest{ID: "demo", Name: "Demo", Version: "1.0.0", Kind: "bundle", Bundle: "/plugins/demo.js"}
	if err := runtime.Load(manifest, ""); err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	st := newTestStore(t)
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "secret")
	r := gin.New()
	if _, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site), Plugins: runtime}}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, runtimeListRequest(t, st))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var response ScriptPluginListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Items) != 1 || response.Items[0].ID != manifest.ID || response.Items[0].Bundle != manifest.Bundle {
		t.Fatalf("items = %#v, want %#v", response.Items, manifest)
	}
}

func runtimeListRequest(t *testing.T, st *store.Store) *http.Request {
	t.Helper()
	admin, err := st.GetUserByUsername("admin")
	if err != nil || admin == nil {
		t.Fatalf("get admin: %v", err)
	}
	cookie := signAuthCookie(cookieAudAdmin, time.Now().Add(authCookieTTL).Unix(), admin.ID, admin.AuthSecret)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/plugins/runtime", nil)
	request.AddCookie(&http.Cookie{Name: authCookieName, Value: cookie})
	return request
}
