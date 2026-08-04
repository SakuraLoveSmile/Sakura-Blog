package module

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// —— 测试假模块：按需实现扩展接口（DependencyModule / LifecycleModule / ConfigModule）——

type fakeModule struct {
	id         string
	defaultOn  bool
	deps       []string
	failEnable bool
	failDisable bool
	section    string
	cfg        map[string]any
	enables    []string // 记录 OnEnable 调用序
	disables   []string
}

func (f *fakeModule) Meta() Meta { return Meta{ID: f.id, Title: f.id, DefaultOn: f.defaultOn} }
func (f *fakeModule) Migrate(Deps) error {
	return nil
}
func (f *fakeModule) RegisterRoutes(*gin.RouterGroup, Deps)   {}
func (f *fakeModule) RegisterPublic(*gin.Engine, Deps)        {}
func (f *fakeModule) Dependencies() []string                  { return f.deps }
func (f *fakeModule) OnEnable(Deps) error {
	if f.failEnable {
		return errors.New("enable failed")
	}
	f.enables = append(f.enables, f.id)
	return nil
}
func (f *fakeModule) OnDisable(Deps) error {
	if f.failDisable {
		return errors.New("disable failed")
	}
	f.disables = append(f.disables, f.id)
	return nil
}
func (f *fakeModule) ConfigSection() string          { return f.section }
func (f *fakeModule) GetConfig(Deps) (any, error)    { return f.cfg, nil }
func (f *fakeModule) PutConfig(Deps, []byte) error   { return nil }

// newTestManager 建内存 store + 用给定模块列表构建 Manager（不污染全局注册表）。
func newTestManager(t *testing.T, mods ...Module) (*Manager, *store.Store) {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := st.EnsureModuleStatesTable(); err != nil {
		t.Fatalf("ensure module_states: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	m, err := newManagerWith(st, mods)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	return m, st
}

// TestNewManagerMissingDep 启动校验：依赖未注册 → 报错含缺失模块 id。
func TestNewManagerMissingDep(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.EnsureModuleStatesTable(); err != nil {
		t.Fatalf("ensure module_states: %v", err)
	}
	a := &fakeModule{id: "a", defaultOn: true, deps: []string{"ghost"}}
	if _, err := newManagerWith(st, []Module{a}); err == nil {
		t.Fatal("expected error for missing dependency")
	} else if !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("error should list missing module, got: %v", err)
	}
}

// TestNewManagerDisabledDep 启动校验：依赖未启用 → 报错。
func TestNewManagerDisabledDep(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.EnsureModuleStatesTable(); err != nil {
		t.Fatalf("ensure module_states: %v", err)
	}
	a := &fakeModule{id: "a", defaultOn: true, deps: []string{"b"}}
	b := &fakeModule{id: "b", defaultOn: false}
	if _, err := newManagerWith(st, []Module{a, b}); err == nil {
		t.Fatal("expected error for disabled dependency")
	} else if !strings.Contains(err.Error(), "未启用") {
		t.Fatalf("error should mention disabled dependency, got: %v", err)
	}
}

// TestNewManagerCycle 启动校验：循环依赖 → 报错含环路径。
func TestNewManagerCycle(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.EnsureModuleStatesTable(); err != nil {
		t.Fatalf("ensure module_states: %v", err)
	}
	a := &fakeModule{id: "a", defaultOn: true, deps: []string{"b"}}
	b := &fakeModule{id: "b", defaultOn: true, deps: []string{"a"}}
	if _, err := newManagerWith(st, []Module{a, b}); err == nil {
		t.Fatal("expected error for dependency cycle")
	} else if !strings.Contains(err.Error(), "循环") {
		t.Fatalf("error should mention cycle, got: %v", err)
	}
}

// TestSetEnabledDependencyOrder 启停语义：依赖未启用不可启用；被依赖者不可禁用；
// 钩子按 启用先自身、禁用先自身 调用。
func TestSetEnabledDependencyOrder(t *testing.T) {
	a := &fakeModule{id: "a", deps: []string{"b"}}
	b := &fakeModule{id: "b"}
	m, _ := newTestManager(t, a, b)
	deps := Deps{}

	// a 依赖 b 未启用 → 拒绝
	if err := m.SetEnabled("a", true, deps); err == nil || !strings.Contains(err.Error(), "依赖未启用") {
		t.Fatalf("enable a with disabled dep should fail, got: %v", err)
	}
	if m.Enabled("a") {
		t.Fatal("a should stay disabled")
	}
	// 启用 b → a
	if err := m.SetEnabled("b", true, deps); err != nil {
		t.Fatalf("enable b: %v", err)
	}
	if err := m.SetEnabled("a", true, deps); err != nil {
		t.Fatalf("enable a: %v", err)
	}
	if len(b.enables) != 1 || len(a.enables) != 1 {
		t.Fatalf("hooks should fire once each, got b=%v a=%v", b.enables, a.enables)
	}
	// b 仍被 a 依赖 → 拒绝禁用
	if err := m.SetEnabled("b", false, deps); err == nil || !strings.Contains(err.Error(), "依赖") {
		t.Fatalf("disable b while depended should fail, got: %v", err)
	}
	// 先禁 a 再禁 b
	if err := m.SetEnabled("a", false, deps); err != nil {
		t.Fatalf("disable a: %v", err)
	}
	if err := m.SetEnabled("b", false, deps); err != nil {
		t.Fatalf("disable b: %v", err)
	}
	if len(a.disables) != 1 || len(b.disables) != 1 {
		t.Fatalf("disables should fire once each, got a=%v b=%v", a.disables, b.disables)
	}
}

// TestSetEnabledRollback OnEnable 失败 → 状态保持禁用；OnDisable 失败 → 状态保持启用。
func TestSetEnabledRollback(t *testing.T) {
	a := &fakeModule{id: "a", failEnable: true}
	m, _ := newTestManager(t, a)
	if err := m.SetEnabled("a", true, Deps{}); err == nil {
		t.Fatal("enable should fail")
	}
	if m.Enabled("a") {
		t.Fatal("state must roll back to disabled")
	}
	a.failEnable = false
	if err := m.SetEnabled("a", true, Deps{}); err != nil {
		t.Fatalf("enable: %v", err)
	}
	a.failDisable = true
	if err := m.SetEnabled("a", false, Deps{}); err == nil {
		t.Fatal("disable should fail")
	}
	if !m.Enabled("a") {
		t.Fatal("state must stay enabled after failed disable")
	}
}

// TestSetEnabledPersist 启停状态落库，重启 Manager 后保持。
func TestSetEnabledPersist(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.EnsureModuleStatesTable(); err != nil {
		t.Fatalf("ensure module_states: %v", err)
	}
	a := &fakeModule{id: "a"}
	m, err := newManagerWith(st, []Module{a})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if err := m.SetEnabled("a", true, Deps{}); err != nil {
		t.Fatalf("enable: %v", err)
	}
	m2, err := newManagerWith(st, []Module{a})
	if err != nil {
		t.Fatalf("reopen manager: %v", err)
	}
	if !m2.Enabled("a") {
		t.Fatal("enabled state should survive manager restart")
	}
}

// TestMountConfigRoutes 框架自动挂载：GET/PUT /settings/:section 走模块配置存取，
// 且与既有 /settings/:section 参数路由共存（静态优先，不 panic）。
func TestMountConfigRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// 模拟全局已有 PUT /settings/:section（router.go 既有路由）
	authed := r.Group("/api/v1")
	authed.PUT("/settings/:section", func(c *gin.Context) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown section " + c.Param("section")})
	})

	cfg := map[string]any{"push": map[string]any{"enabled": true}}
	f := &fakeModule{id: "seo", section: "seo", cfg: cfg}
	grp := authed.Group("", func(c *gin.Context) { c.Next() }) // 模拟 Gate 中间件
	if !MountConfigRoutes(grp, Deps{}, f) {
		t.Fatal("MountConfigRoutes should mount for ConfigModule")
	}
	if MountConfigRoutes(grp, Deps{}, &fakeModule{id: "plain"}) {
		t.Fatal("MountConfigRoutes should skip non-ConfigModule")
	}

	// GET 命中模块配置
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings/seo", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET config status = %d, want 200", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"enabled":true`)) {
		t.Fatalf("GET config body = %s", w.Body.String())
	}

	// PUT 命中模块配置（静态路由优先于 :section 参数路由）
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/v1/settings/seo", bytes.NewBufferString(`{}`))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT config status = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	// 其他 section 仍走全局参数路由
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/v1/settings/other", bytes.NewBufferString(`{}`))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("PUT other section status = %d, want 400", w.Code)
	}
}
