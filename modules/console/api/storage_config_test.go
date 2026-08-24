package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/module"
	_ "blog/internal/module/core"
	_ "blog/internal/module/storage"
	tmpl "blog/internal/settings"
	attachmentstorage "blog/internal/storage"
)

func TestStorageConfigEndpointPersistsAndMasksSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	if err := st.EnsureSettingsTable(); err != nil {
		t.Fatalf("ensure settings: %v", err)
	}
	if err := st.EnsureModuleStatesTable(); err != nil {
		t.Fatalf("ensure module states: %v", err)
	}
	storageManager, err := attachmentstorage.NewManager(filepath.Join(t.TempDir(), "uploads"), attachmentstorage.DefaultConfig())
	if err != nil {
		t.Fatalf("new storage manager: %v", err)
	}
	manager, err := module.NewManager(st)
	if err != nil {
		t.Fatalf("new module manager: %v", err)
	}
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "secret")
	admin, err := st.GetUserByUsername("admin")
	if err != nil || admin == nil {
		t.Fatalf("get admin: %v", err)
	}
	r := gin.New()
	if _, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site), Modules: manager, StorageManager: storageManager}}); err != nil {
		t.Fatalf("mount console: %v", err)
	}
	cookie := signAuthCookie(cookieAudAdmin, time.Now().Add(authCookieTTL).Unix(), admin.ID, admin.AuthSecret)
	request := func(method, url, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, url, bytes.NewBufferString(body))
		req.AddCookie(&http.Cookie{Name: authCookieName, Value: cookie})
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}

	put := request(http.MethodPut, "/api/v1/settings/storage", `{"provider":"local","region":"us-east-1","secret_key":"real-secret","prefix":"uploads"}`)
	if put.Code != http.StatusOK {
		t.Fatalf("PUT storage config status=%d body=%s", put.Code, put.Body.String())
	}
	get := request(http.MethodGet, "/api/v1/settings/storage", "")
	if get.Code != http.StatusOK || strings.Contains(get.Body.String(), "real-secret") || !strings.Contains(get.Body.String(), `"secret_key":"****"`) {
		t.Fatalf("GET storage config status=%d body=%s", get.Code, get.Body.String())
	}
	if storageManager.Config().SecretKey != "real-secret" || storageManager.Config().Prefix != "uploads" {
		t.Fatalf("runtime storage config = %+v", storageManager.Config())
	}
}
