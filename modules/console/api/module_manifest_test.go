package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/module"
	core "blog/internal/module/core"
	_ "blog/internal/module/moments"
	tmpl "blog/internal/settings"
)

func TestCoreManifestContainsAssistantNav(t *testing.T) {
	meta := (&core.Module{}).Meta()
	for _, item := range meta.Nav {
		if item.Path == "/assistant" {
			if item.Label != "AI 助手" || item.Group != "工具" {
				t.Fatalf("assistant nav = %+v, want label/group AI 助手/工具", item)
			}
			return
		}
	}
	t.Fatal("core manifest is missing the AI assistant nav item")
}

func TestMomentsRouteFollowsManifestGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	if err := st.EnsureMomentsTable(); err != nil {
		t.Fatalf("ensure moments: %v", err)
	}
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "secret")
	if err := st.EnsureModuleStatesTable(); err != nil {
		t.Fatalf("ensure module states: %v", err)
	}
	mgr, err := module.NewManager(st)
	if err != nil {
		t.Fatalf("new module manager: %v", err)
	}
	r := gin.New()
	if _, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site), Modules: mgr}}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	admin, err := st.GetUserByUsername("admin")
	if err != nil || admin == nil {
		t.Fatalf("get admin: %v", err)
	}
	cookie := signAuthCookie(cookieAudAdmin, time.Now().Add(authCookieTTL).Unix(), admin.ID, admin.AuthSecret)
	request := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/moments", nil)
		req.AddCookie(&http.Cookie{Name: authCookieName, Value: cookie})
		r.ServeHTTP(w, req)
		return w
	}
	if response := request(); response.Code != http.StatusOK {
		t.Fatalf("enabled moments status = %d, want 200: %s", response.Code, response.Body.String())
	}
	if err := mgr.SetEnabled("moments", false, module.Deps{Store: st, Site: sitePtr(site), Modules: mgr}); err != nil {
		t.Fatalf("disable moments: %v", err)
	}
	if got := request().Code; got != http.StatusForbidden {
		t.Fatalf("disabled moments status = %d, want 403", got)
	}
}
