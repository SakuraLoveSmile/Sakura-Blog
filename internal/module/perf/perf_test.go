package perf

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/module"
	tmpl "blog/internal/settings"
	"blog/internal/store"
)

// newPerfTestEnv 建临时 store + 真实 perf 模块（默认关闭），返回模块/manager/deps。
// 模块实例直接构造（不走全局注册表），避免与 init 自注册实例共享聚合器。
func newPerfTestEnv(t *testing.T) (*Module, *module.Manager, module.Deps, *store.Store) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.EnsurePerfTables(); err != nil {
		t.Fatalf("ensure perf tables: %v", err)
	}
	if err := st.EnsureModuleStatesTable(); err != nil {
		t.Fatalf("ensure module_states: %v", err)
	}
	mgr, err := module.NewManager(st)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	m := &Module{agg: newAggregator()}
	deps := module.Deps{Store: st, Site: newSitePtr(), Modules: mgr}
	return m, mgr, deps, st
}

// routeCount 区间内路由统计行数（0 表示无任何采集数据）。
func routeCount(t *testing.T, st *store.Store) int {
	t.Helper()
	stats, err := st.PerfRouteStatsSince(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("route stats: %v", err)
	}
	return len(stats)
}

// TestPerfDefaultOff 验收：模块默认关闭（用户主动启用）。
// newSitePtr 构造原子站点指针（Spec 61 阶段 3）。
func newSitePtr() *atomic.Pointer[tmpl.Site] {
	p := &atomic.Pointer[tmpl.Site]{}
	p.Store(&tmpl.Site{})
	return p
}

func TestPerfDefaultOff(t *testing.T) {
	_, mgr, _, _ := newPerfTestEnv(t)
	if mgr.Enabled("perf") {
		t.Fatal("perf should default off")
	}
}

func TestPerfDropsBucketsFromBeforeMaintenance(t *testing.T) {
	m, _, deps, st := newPerfTestEnv(t)
	m.agg.recordAt("GET", "/old", http.StatusOK, time.Millisecond, st.Maintenance().Generation())
	window, err := st.Maintenance().Begin(context.Background(), "test")
	if err != nil {
		t.Fatalf("begin maintenance: %v", err)
	}
	window.Close()
	m.flushAll(deps)
	if count := routeCount(t, st); count != 0 {
		t.Fatalf("pre-maintenance buckets wrote %d rows after recovery", count)
	}
}

// TestPerfMiddlewareRecordsWhenEnabled 验收：启用后中间件按路由模板聚合耗时并落库。
func TestPerfMiddlewareRecordsWhenEnabled(t *testing.T) {
	m, mgr, deps, st := newPerfTestEnv(t)
	if err := mgr.SetEnabled("perf", true, deps); err != nil {
		t.Fatalf("enable perf: %v", err)
	}

	r := gin.New()
	r.Use(m.timingMiddleware(deps))
	r.GET("/posts/:slug", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// 两次请求同一路由模板 → 聚合为一行；一次 404（FullPath 为空）不记录
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/posts/hello", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", w.Code)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/not-exist", nil))

	m.flushAll(deps) // 当前分钟桶强制落库（定时 flush 只落已完成分钟）
	stats, err := st.PerfRouteStatsSince(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("route stats: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("want 1 route stat row, got %d: %+v", len(stats), stats)
	}
	if stats[0].Route != "/posts/:slug" || stats[0].Count != 2 {
		t.Fatalf("unexpected stat: %+v", stats[0])
	}
}

// TestPerfDisableStopsCollect 验收：禁用模块后中间件停止记录，已采数据在 OnDisable 落库。
func TestPerfDisableStopsCollect(t *testing.T) {
	m, mgr, deps, st := newPerfTestEnv(t)
	if err := mgr.SetEnabled("perf", true, deps); err != nil {
		t.Fatalf("enable perf: %v", err)
	}
	r := gin.New()
	r.Use(m.timingMiddleware(deps))
	r.GET("/hello", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/hello", nil))

	// 禁用：OnDisable 钩子把残留桶落库，此后请求不再记录
	//（钩子直接调测试实例，与 Manager 对注册表实例的调度同语义）
	if err := mgr.SetEnabled("perf", false, deps); err != nil {
		t.Fatalf("disable perf: %v", err)
	}
	if err := m.OnDisable(deps); err != nil {
		t.Fatalf("on disable: %v", err)
	}
	if routeCount(t, st) != 1 {
		t.Fatalf("disabled flush should persist pending bucket, got %d rows", routeCount(t, st))
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/hello", nil))
	m.flushAll(deps)
	if routeCount(t, st) != 1 {
		t.Fatal("disabled module must not record new requests")
	}
}

// TestPerfErrorCount 验收：4xx/5xx 计入 err_count（聚合摘要错误率用）。
func TestPerfErrorCount(t *testing.T) {
	m, mgr, deps, st := newPerfTestEnv(t)
	if err := mgr.SetEnabled("perf", true, deps); err != nil {
		t.Fatalf("enable perf: %v", err)
	}
	r := gin.New()
	r.Use(m.timingMiddleware(deps))
	r.GET("/boom", func(c *gin.Context) { c.String(http.StatusInternalServerError, "err") })
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))
	m.flushAll(deps)

	count, errCount, _, err := st.PerfRequestSummarySince(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if count != 1 || errCount != 1 {
		t.Fatalf("want count=1 err=1, got count=%d err=%d", count, errCount)
	}
}

// TestPerfAPIGate 验收：模块禁用时 API 被 Gate 拦截（403），启用后 200 且结构完整。
func TestPerfAPIGate(t *testing.T) {
	m, mgr, deps, _ := newPerfTestEnv(t)

	r := gin.New()
	grp := r.Group("/api/v1", mgr.Gate("perf"))
	m.RegisterRoutes(grp, deps)

	do := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}
	for _, path := range []string{"/api/v1/perf/overview", "/api/v1/perf/history", "/api/v1/perf/requests"} {
		if w := do(path); w.Code != http.StatusForbidden {
			t.Fatalf("%s disabled should be 403, got %d", path, w.Code)
		}
	}

	if err := mgr.SetEnabled("perf", true, deps); err != nil {
		t.Fatalf("enable perf: %v", err)
	}
	for _, path := range []string{"/api/v1/perf/overview", "/api/v1/perf/history", "/api/v1/perf/requests"} {
		if w := do(path); w.Code != http.StatusOK {
			t.Fatalf("%s enabled should be 200, got %d: %s", path, w.Code, w.Body.String())
		}
	}
}

// TestWorkersLifecycle 验收：OnEnable 启动采集 goroutine（幂等），OnDisable 停止。
func TestWorkersLifecycle(t *testing.T) {
	m, _, deps, _ := newPerfTestEnv(t)
	if m.workersRunning() {
		t.Fatal("workers must not run before enable")
	}
	if err := m.OnEnable(deps); err != nil {
		t.Fatalf("on enable: %v", err)
	}
	if !m.workersRunning() {
		t.Fatal("OnEnable should start workers")
	}
	if err := m.OnEnable(deps); err != nil { // 幂等：重复启用不重复启动
		t.Fatalf("on enable twice: %v", err)
	}
	if !m.workersRunning() {
		t.Fatal("workers should keep running after idempotent enable")
	}
	if err := m.OnDisable(deps); err != nil {
		t.Fatalf("on disable: %v", err)
	}
	if m.workersRunning() {
		t.Fatal("OnDisable should stop workers")
	}
}

// TestPerfSamplesAndPrune 验收：采样点写入/查询与过期清理。
func TestPerfSamplesAndPrune(t *testing.T) {
	_, _, _, st := newPerfTestEnv(t)

	now := time.Now()
	old := now.AddDate(0, 0, -10) // 超出 7 天保留期
	if err := st.InsertPerfSamples(old, map[string]float64{"heap_alloc_mb": 1}); err != nil {
		t.Fatalf("insert old sample: %v", err)
	}
	if err := st.InsertPerfSamples(now, captureRuntimeMetrics()); err != nil {
		t.Fatalf("insert samples: %v", err)
	}
	points, err := st.PerfSamplesSince(old.Add(-time.Hour))
	if err != nil {
		t.Fatalf("query samples: %v", err)
	}
	if len(points) != 5 { // 1 旧点 + 4 个运行时指标
		t.Fatalf("want 5 points, got %d", len(points))
	}
	if err := st.PrunePerfData(7); err != nil {
		t.Fatalf("prune: %v", err)
	}
	points, err = st.PerfSamplesSince(old.Add(-time.Hour))
	if err != nil {
		t.Fatalf("query after prune: %v", err)
	}
	if len(points) != 4 {
		t.Fatalf("old sample should be pruned, got %d points", len(points))
	}
}

func TestRestoreGenerationDropsOldPeriodicBuckets(t *testing.T) {
	aggregate := newAggregator()
	aggregate.recordAt("GET", "/old", 200, time.Millisecond, 0)
	aggregate.recordAt("GET", "/new", 200, time.Millisecond, 1)
	buckets := aggregate.takeOlder(time.Now().Add(time.Hour), 1)
	if len(buckets) != 1 || buckets[0].Route != "/new" {
		t.Fatalf("old generation escaped periodic flush: %+v", buckets)
	}
	if len(aggregate.takeAll(1)) != 0 {
		t.Fatal("old generation retained in memory")
	}
}
