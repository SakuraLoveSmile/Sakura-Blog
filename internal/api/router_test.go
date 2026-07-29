package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/friends"
	"blog/internal/store"
	tmpl "blog/internal/template"
)

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
	h, err := Mount(r, MountDeps{Config: Config{Store: st, Site: &tmpl.Site{}}})
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
		"GET /api/v1/shares",
		"GET /api/rating", // 评分公开端点随 Mount 注册
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
	if _, err := Mount(r, MountDeps{Config: Config{Store: st, Site: &tmpl.Site{}}}); err != nil {
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

func TestMountWithFriendsAggregator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	site := &tmpl.Site{}
	r := gin.New()
	// 只注册路由，不调用 Start()（生命周期由调用方管理）
	if _, err := Mount(r, MountDeps{
		Config:  Config{Store: st, Site: site},
		Friends: friends.New(site),
	}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	if !routeSet(r)["GET /api/friends-aggregated"] {
		t.Error("friends aggregated route should be registered")
	}
}

func TestMountAuthGuardsProtectedRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	site := &tmpl.Site{}
	site.Admin.Password = "secret"
	r := gin.New()
	if _, err := Mount(r, MountDeps{Config: Config{Store: st, Site: site}}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated request should get 401, got %d", w.Code)
	}
}
