package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/module"
	core "blog/internal/module/core"
	_ "blog/internal/module/moments"
	tmpl "blog/internal/settings"
	"blog/internal/store"
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

func TestCoreManifestContainsAppearanceNav(t *testing.T) {
	meta := (&core.Module{}).Meta()
	for _, item := range meta.Nav {
		if item.Path == "/appearance" {
			if item.Label != "主题设置" || item.Group != "外观" || item.Icon != "ri:palette-line" {
				t.Fatalf("appearance nav = %+v, want label/group/icon 主题设置/外观/ri:palette-line", item)
			}
			if len(item.Permissions) != 1 || item.Permissions[0] != store.PermissionSystemManage {
				t.Fatalf("appearance nav permissions = %+v, want [%s]", item.Permissions, store.PermissionSystemManage)
			}
			return
		}
	}
	t.Fatal("core manifest is missing the appearance nav item")
}

func TestPluginsNavHasFiveChildren(t *testing.T) {
	want := []string{
		"/plugins/shortcodes",
		"/plugins/assets",
		"/plugins/widgets",
		"/plugins/fixtures",
		"/plugins/runtime",
	}
	meta := (&core.Module{}).Meta()
	for _, item := range meta.Nav {
		if item.Path != "/plugins" {
			continue
		}
		if len(item.Children) != len(want) {
			t.Fatalf("plugins nav children = %d, want %d", len(item.Children), len(want))
		}
		got := make([]string, 0, len(item.Children))
		for _, child := range item.Children {
			if len(child.Permissions) == 0 {
				t.Fatalf("plugins child %s has no explicit permission; DefaultLayout evaluates child permission independently and does not inherit from the parent", child.Path)
			}
			got = append(got, child.Path)
		}
		for i, w := range want {
			if got[i] != w {
				t.Fatalf("plugins child[%d] = %q, want %q", i, got[i], w)
			}
		}
		return
	}
	t.Fatal("core manifest is missing the plugins nav item")
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

func TestRuntimeCoreManifestExposesExistingContentAndAppearanceEntries(t *testing.T) {
	r, st, admin, h := newFullStellarEnv(t)
	if err := st.EnsureModuleStatesTable(); err != nil {
		t.Fatal(err)
	}
	manager, err := module.NewManager(st)
	if err != nil {
		t.Fatal(err)
	}
	h.modules = manager
	alice, _ := st.GetUserByUsername("alice")
	for _, user := range []*store.User{admin, alice} {
		w := do2FA(r, http.MethodGet, "/api/v1/modules", "", authCookie(user))
		if w.Code != http.StatusOK {
			t.Fatalf("%s manifest: %d %s", user.Username, w.Code, w.Body.String())
		}
		var response ModuleListResponse
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		var core *module.ManifestItem
		for i := range response.Items {
			if response.Items[i].ID == "core" {
				core = &response.Items[i]
				break
			}
		}
		if core == nil || !core.Enabled || !core.Locked {
			t.Fatal("runtime core must remain enabled and locked")
		}
		paths := make([]string, 0, len(core.Nav))
		byPath := map[string]module.Nav{}
		for _, item := range core.Nav {
			paths = append(paths, item.Path)
			byPath[item.Path] = item
		}
		for _, expected := range []module.Nav{
			{Path: "/notebooks", Label: "笔记本", Icon: "ri:book-2-line", Group: "内容"},
			{Path: "/authors", Label: "作者", Icon: "ri:user-star-line", Group: "内容", Permissions: []string{store.PermissionSystemManage}},
			{Path: "/menus", Label: "菜单", Icon: "ri:menu-line", Group: "外观", Permissions: []string{store.PermissionSystemManage}},
		} {
			got, ok := byPath[expected.Path]
			if !ok {
				t.Fatalf("runtime manifest is missing %s", expected.Path)
			}
			if got.Label != expected.Label || got.Icon != expected.Icon || got.Group != expected.Group || !slices.Equal(got.Permissions, expected.Permissions) {
				t.Fatalf("%s differs from existing frontend contract: %+v", expected.Path, got)
			}
		}
		for _, order := range [][3]string{{"/topics", "/notebooks", "/pages"}, {"/attachments", "/authors", "/appearance"}, {"/appearance", "/menus", "/taxonomy"}} {
			if slices.Index(paths, order[1]) != slices.Index(paths, order[0])+1 || slices.Index(paths, order[2]) != slices.Index(paths, order[1])+1 {
				t.Fatalf("menu order changed: %v", paths)
			}
		}
	}
}
