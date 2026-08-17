package seo

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/module"
	tmpl "blog/internal/settings"
	"blog/internal/store"
)

// newSitePtr 构造原子站点指针（Spec 61 阶段 3）。
func newSitePtr() *atomic.Pointer[tmpl.Site] {
	p := &atomic.Pointer[tmpl.Site]{}
	p.Store(&tmpl.Site{})
	return p
}

func TestCrawlerEngine(t *testing.T) {
	cases := map[string]string{
		"Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com)": "Baidu",
		"Mozilla/5.0 (compatible; Googlebot/2.1)":                          "Google",
		"Mozilla/5.0 (compatible; bingbot/2.0)":                            "Bing",
		"Sogou web spider/4.0":                                             "Sogou",
		"Mozilla/5.0 Bytespider":                                           "Bytespider",
		"Mozilla/5.0 (Windows NT 10.0) Chrome/120":                         "", // 普通浏览器非爬虫
	}
	for ua, want := range cases {
		if got := crawlerEngine(ua); got != want {
			t.Errorf("crawlerEngine(%q) = %q, want %q", ua, got, want)
		}
	}
}

func TestReferrerEngine(t *testing.T) {
	cases := map[string]string{
		"https://www.baidu.com/s?wd=x":  "Baidu",
		"https://www.google.com/search": "Google",
		"https://cn.bing.com/search":    "Bing",
		"https://example.com/blog":      "", // 非搜索来源
		"":                              "",
	}
	for ref, want := range cases {
		if got := referrerEngine(ref); got != want {
			t.Errorf("referrerEngine(%q) = %q, want %q", ref, got, want)
		}
	}
}

func TestRunChecks(t *testing.T) {
	f := pageFacts{
		Title:       "一个长度合适的标题示例",
		Description: "desc",
		Canonical:   "https://x/",
		OGTitle:     "t",
		OGDesc:      "d",
		Viewport:    "width=device-width",
		Lang:        "zh-CN",
		RobotsMeta:  "index,follow",
	}
	checks := runChecks("https://example.com/", f)
	byKey := map[string]bool{}
	for _, c := range checks {
		byKey[c.Key] = c.Passed
	}
	if !byKey["https"] || !byKey["description"] || !byKey["canonical"] || !byKey["indexable"] || !byKey["viewport"] {
		t.Fatalf("expected core checks to pass, got %+v", byKey)
	}
	// noindex 时 indexable 应失败
	f.RobotsMeta = "noindex"
	for _, c := range runChecks("https://example.com/", f) {
		if c.Key == "indexable" && c.Passed {
			t.Fatalf("indexable should fail when robots meta has noindex")
		}
	}
	// http 时 https 检查应失败
	for _, c := range runChecks("http://example.com/", f) {
		if c.Key == "https" && c.Passed {
			t.Fatalf("https check should fail for http URL")
		}
	}
}

func TestAtoiDefault(t *testing.T) {
	if atoiDefault("", 5) != 5 || atoiDefault("abc", 3) != 3 || atoiDefault("12", 1) != 12 {
		t.Fatal("atoiDefault behaves unexpectedly")
	}
}

// newSeoTestEnv 建内存 store + 真实 SEO 模块（默认启用），返回 manager/deps 供生命周期与埋点测试。
func newSeoTestEnv(t *testing.T) (*module.Manager, module.Deps, *store.Store) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.EnsureCrawlerLogsTable(); err != nil {
		t.Fatalf("ensure crawler_logs: %v", err)
	}
	if err := st.EnsureModuleStatesTable(); err != nil {
		t.Fatalf("ensure module_states: %v", err)
	}
	if err := st.EnsureSettingsTable(); err != nil {
		t.Fatalf("ensure settings: %v", err)
	}
	mgr, err := module.NewManager(st)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	deps := module.Deps{Store: st, Site: newSitePtr(), Modules: mgr}
	return mgr, deps, st
}

// crawlerCount 当前爬虫日志总数（ListCrawlerLogs 返回 total）。
func crawlerCount(t *testing.T, st *store.Store) int {
	t.Helper()
	_, total, err := st.ListCrawlerLogs(1, 50, "")
	if err != nil {
		t.Fatalf("list crawler logs: %v", err)
	}
	return total
}

// TestSEODisableStopsCrawler 验收：禁用 SEO 模块后爬虫埋点停止写入（crawler_logs 计数不增）。
func TestSEODisableStopsCrawler(t *testing.T) {
	mgr, deps, st := newSeoTestEnv(t)
	if !mgr.Enabled("seo") {
		t.Fatal("seo should default on")
	}
	m := &Module{}
	r := gin.New()
	r.Use(m.crawlerMiddleware(deps))
	r.GET("/", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	crawlerUA := "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
	do := func() {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("User-Agent", crawlerUA)
		r.ServeHTTP(w, req)
	}

	do()
	if got := crawlerCount(t, st); got != 1 {
		t.Fatalf("crawler count after enabled hit = %d, want 1", got)
	}
	// 禁用模块：OnDisable 钩子执行，埋点立即停止
	if err := mgr.SetEnabled("seo", false, deps); err != nil {
		t.Fatalf("disable seo: %v", err)
	}
	do()
	do()
	if got := crawlerCount(t, st); got != 1 {
		t.Fatalf("crawler count after disabled hits = %d, want 1 (unchanged)", got)
	}
}

// TestSEOLegacyRowMigration 历史迁移：共用 "seo" 行的工具集键拆到 seo_tools，
// verify 并入站点级 *_verification（仅空缺回填），站点级行移除工具集键，重复执行无副作用。
func TestSEOLegacyRowMigration(t *testing.T) {
	_, _, st := newSeoTestEnv(t)
	legacy := `{"default_description":"old","google_verification":"","push":{"enabled":true,"indexnow_key":"k1"},"crawler":{"enabled":true},"traffic":{"enabled":false},"verify":{"google":"gv","bing":"","baidu":""}}`
	if err := st.PutSetting(legacySeoKey, legacy); err != nil {
		t.Fatalf("put legacy row: %v", err)
	}
	if !migrateLegacySeoRow(st) {
		t.Fatal("migration should report seo row changed")
	}
	// 工具集配置拆到独立行且内容完整
	cfg := loadConfig(st)
	if !cfg.Push.Enabled || cfg.Push.IndexNowKey != "k1" {
		t.Fatalf("push config lost after migration: %+v", cfg.Push)
	}
	if !cfg.Crawler.Enabled || cfg.Traffic.Enabled {
		t.Fatalf("toggle config lost after migration: crawler=%v traffic=%v", cfg.Crawler, cfg.Traffic)
	}
	// 站点级 SEO 行：工具集键移除、原字段保留、verify 回填 google_verification
	raw, err := st.GetSetting(legacySeoKey)
	if err != nil {
		t.Fatalf("get seo row: %v", err)
	}
	for _, k := range []string{`"push"`, `"crawler"`, `"traffic"`, `"verify"`} {
		if strings.Contains(raw, k) {
			t.Fatalf("legacy row still contains toolset key %s: %s", k, raw)
		}
	}
	if !strings.Contains(raw, `"default_description":"old"`) {
		t.Fatalf("site-level fields lost: %s", raw)
	}
	if !strings.Contains(raw, `"google_verification":"gv"`) {
		t.Fatalf("verify not merged into google_verification: %s", raw)
	}
	// 幂等：seo_tools 行已存在时不再迁移
	if migrateLegacySeoRow(st) {
		t.Fatal("migration should be idempotent")
	}
}

// TestSEOConfigEndpoint 配置隔离路由：GET/PUT /api/v1/settings/seo_tools 可用且脱敏，
// 不再劫持站点级 /settings/seo 分段保存路由。
func TestSEOConfigEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr, deps, st := newSeoTestEnv(t)
	m := &Module{}
	if got := m.ConfigSection(); got != "seo_tools" {
		t.Fatalf("ConfigSection = %q, want seo_tools", got)
	}

	// 预置含密钥的配置
	if err := m.PutConfig(deps, []byte(`{"push":{"enabled":true,"indexnow_key":"secret"}}`)); err != nil {
		t.Fatalf("put config: %v", err)
	}
	r := gin.New()
	grp := r.Group("/api/v1", mgr.Gate("seo"))
	if !module.MountConfigRoutes(grp, deps, m) {
		t.Fatal("seo should mount config routes")
	}

	// GET：配置可读且密钥脱敏
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings/seo_tools", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"indexnow_key":"****"`) {
		t.Fatalf("GET should mask secret, body = %s", w.Body.String())
	}
	// GET 后配置仍未被脱敏污染（密钥仍在库中）
	cfg := loadConfig(st)
	if cfg.Push.IndexNowKey != "secret" {
		t.Fatalf("stored key corrupted after masked GET, got %q", cfg.Push.IndexNowKey)
	}

	// PUT：脱敏占位回写不覆盖原密钥
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/v1/settings/seo_tools", strings.NewReader(`{"push":{"enabled":false,"indexnow_key":"****"}}`))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", w.Code, w.Body.String())
	}
	cfg = loadConfig(st)
	if cfg.Push.Enabled || cfg.Push.IndexNowKey != "secret" {
		t.Fatalf("PUT should keep secret and apply toggles, got %+v", cfg.Push)
	}

	// 禁用模块后配置端点被 Gate 拦截（403）
	if err := mgr.SetEnabled("seo", false, deps); err != nil {
		t.Fatalf("disable seo: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/settings/seo_tools", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("GET after disable status = %d, want 403", w.Code)
	}
}
