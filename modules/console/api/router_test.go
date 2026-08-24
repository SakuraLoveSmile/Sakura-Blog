package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"

	tmpl "blog/internal/settings"
	"blog/internal/store"
)

// enableAdmin 测试助手（Spec 61 阶段 4）：users 表建/更新 admin 用户（bcrypt 密码）
// 并打开 AuthEnabled gate（旧语义 enableAdmin(t, st, site, "x") 已被派生布尔替代）。
func enableAdmin(t *testing.T, st *store.Store, site *tmpl.Site, password string) {
	t.Helper()
	if err := st.EnsureUsersTable(); err != nil {
		t.Fatalf("users table: %v", err)
	}
	if password == "" {
		site.Admin.AuthEnabled = false
		return
	}
	hash, err := store.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	admin, err := st.GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("get admin: %v", err)
	}
	if admin == nil {
		admin = &store.User{
			Username: "admin", PasswordHash: hash, DisplayName: "管理员",
			AuthSecret: store.GenerateAuthSecret(), Role: store.RoleAdmin,
		}
		if err := st.CreateUser(admin); err != nil {
			t.Fatalf("create admin: %v", err)
		}
	} else if admin.PasswordHash == "" {
		if err := st.UpdatePassword(admin.ID, hash, admin.AuthSecret); err != nil {
			t.Fatalf("update admin: %v", err)
		}
	}
	site.Admin.AuthEnabled = true
}

// sitePtr 包装 *tmpl.Site 为原子指针（Spec 61 阶段 3：Config.Site 线程化后测试构造点）。
func sitePtr(s *tmpl.Site) *atomic.Pointer[tmpl.Site] {
	p := &atomic.Pointer[tmpl.Site]{}
	p.Store(s)
	return p
}

// newTestStore 临时 SQLite 库（modernc 纯 Go 驱动，测试后自动清理）。
func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// routeSet 收集已注册路由为 "METHOD path" 集合。
func routeSet(r *gin.Engine) map[string]bool {
	set := map[string]bool{}
	for _, ri := range r.Routes() {
		set[ri.Method+" "+ri.Path] = true
	}
	return set
}

func TestMountRegistersCoreRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	r := gin.New()
	h, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(&tmpl.Site{})}})
	if err != nil {
		t.Fatalf("mount: %v", err)
	}
	if h == nil {
		t.Fatal("mount should return handler")
	}
	routes := routeSet(r)
	for _, want := range []string{
		"POST /api/v1/auth/login",
		"GET /api/v1/posts",
		"GET /api/v1/posts/:slug/revisions",
		"GET /api/v1/attachments",
		"POST /api/v1/attachments/import-url",
		"GET /api/v1/shares",
		"GET /api/v1/dashboard/layout",
		"PUT /api/v1/dashboard/layout",
		"GET /api/v1/dashboard/stellar-version",
		"GET /api/v1/plugins/catalog",
	} {
		if !routes[want] {
			t.Errorf("route %q not registered", want)
		}
	}
	// Friends/GH 未注入时不注册对应公共端点
	if routes["GET /api/friends-aggregated"] {
		t.Error("friends route should not be registered without aggregator")
	}
}

func TestMountEnsuresTables(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	r := gin.New()
	if _, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(&tmpl.Site{})}}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	// shared_posts 表已建：按 token 查询应返回 (nil, nil) 而非表不存在错误
	if sp, err := st.GetShareByToken("no-such-token"); err != nil || sp != nil {
		t.Errorf("shared_posts table not ready: sp=%v err=%v", sp, err)
	}
	// page_views 表已建：记录访问不报错
	if err := st.RecordView("/post/x", "", "hash"); err != nil {
		t.Errorf("page_views table not ready: %v", err)
	}
}

func TestMountAuthGuardsProtectedRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "secret")
	r := gin.New()
	if _, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site)}}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated request should get 401, got %d", w.Code)
	}
}

// TestConsoleRouteOwnershipSnapshot fixes the public surface owned by Console.
func TestConsoleRouteOwnershipSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	r := gin.New()
	if _, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(&tmpl.Site{})}}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	routes := routeSet(r)
	for _, want := range []string{
		"POST /api/v1/auth/login",
		"POST /api/v1/auth/logout",
		"GET /api/v1/auth/check",
		"POST /api/v1/auth/2fa",
		"GET /api/v1/posts",
		"PUT /api/v1/settings/:section",
		"POST /api/v1/backup/restore",
	} {
		if !routes[want] {
			t.Errorf("Console route missing from ownership snapshot: %s", want)
		}
	}
	for _, forbidden := range []string{
		"POST /api/auth/register",
		"POST /api/auth/login",
		"GET /api/auth/me",
		"POST /api/auth/logout",
		"GET /api/public/v1/posts",
		"GET /api/rating",
		"POST /api/rating",
		"GET /api/vote/info",
		"POST /api/vote/update",
		"GET /api/friends-aggregated",
	} {
		if routes[forbidden] {
			t.Errorf("Stellar public route registered by Console: %s", forbidden)
		}
	}
}
