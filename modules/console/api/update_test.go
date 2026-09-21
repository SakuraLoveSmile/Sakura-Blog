package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"

	ghpkg "blog/internal/github"
	tmpl "blog/internal/settings"
)

// newUpdateEnvWithCookie 构造带 GitHub 模拟的测试环境，额外返回 admin cookie。
func newUpdateEnvWithCookie(t *testing.T, ghHandler http.HandlerFunc) (*gin.Engine, *Handler, string, *int32) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "secret")

	admin, err := st.GetUserByUsername("admin")
	if err != nil || admin == nil {
		t.Fatalf("get admin: %v", err)
	}

	var ghHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&ghHits, 1)
		if ghHandler != nil {
			ghHandler(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":"v1.4.2","html_url":"https://github.com/SakuraLoveSmile/Sakura-Blog-Release/releases/tag/v1.4.2","body":"Bug fixes.","published_at":"2026-08-15T10:00:00Z"}`))
	}))
	t.Cleanup(srv.Close)

	gh := ghpkg.New(ghpkg.Config{CacheDir: t.TempDir(), APIBase: srv.URL, HTTPClient: srv.Client()})

	r := gin.New()
	h, err := MountConsole(r, MountDeps{
		Config: Config{
			Store:      st,
			Site:       sitePtr(site),
			AppVersion: "v1.4.1",
			UpdateRepo: "SakuraLoveSmile/Sakura-Blog-Release",
			Root:       t.TempDir(),
		},
		GH: gh,
	})
	if err != nil {
		t.Fatalf("mount: %v", err)
	}
	cookie := authCookie(admin)
	return r, h, cookie, &ghHits
}

// doUpdate 发起带 cookie 的请求。
func doUpdate(r *gin.Engine, method, path, cookie string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: authCookieName, Value: cookie})
	}
	r.ServeHTTP(w, req)
	return w
}

// --- version 端点 ---

func TestSystemVersion_Success(t *testing.T) {
	r, _, cookie, _ := newUpdateEnvWithCookie(t, nil)
	w := doUpdate(r, http.MethodGet, "/api/v1/system/version", cookie)
	if w.Code != 200 {
		t.Fatalf("version: %d %s", w.Code, w.Body.String())
	}
	var resp SystemVersionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Version == "" {
		t.Error("version should not be empty")
	}
	if !resp.IsDev {
		t.Error("expected dev mode by default in tests")
	}
	if resp.GitCommit != "dev" {
		t.Errorf("expected GitCommit to be dev, got %q", resp.GitCommit)
	}
}

func TestSystemVersion_RequiresAdmin(t *testing.T) {
	r, _, _, _ := newUpdateEnvWithCookie(t, nil)
	w := doUpdate(r, http.MethodGet, "/api/v1/system/version", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated: got %d, want 401", w.Code)
	}
}

// --- check 端点 ---

func TestSystemUpdateCheck_UpdateAvailable(t *testing.T) {
	r, _, cookie, _ := newUpdateEnvWithCookie(t, nil)
	w := doUpdate(r, http.MethodGet, "/api/v1/system/update/check", cookie)
	if w.Code != 200 {
		t.Fatalf("check: %d %s", w.Code, w.Body.String())
	}
	var resp UpdateCheckResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.CurrentVersion != "1.4.1" {
		t.Errorf("current: got %q, want 1.4.1", resp.CurrentVersion)
	}
	if resp.LatestVersion != "1.4.2" {
		t.Errorf("latest: got %q, want 1.4.2", resp.LatestVersion)
	}
	if !resp.UpdateAvailable {
		t.Error("update_available should be true")
	}
	if resp.ReleaseURL == "" {
		t.Error("release_url should not be empty")
	}
}

func TestSystemUpdateCheck_NoUpdate(t *testing.T) {
	r, _, cookie, _ := newUpdateEnvWithCookie(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":"v1.4.1","html_url":"https://example.com","body":"","published_at":"2026-08-10T10:00:00Z"}`))
	})
	w := doUpdate(r, http.MethodGet, "/api/v1/system/update/check", cookie)
	if w.Code != 200 {
		t.Fatalf("check: %d %s", w.Code, w.Body.String())
	}
	var resp UpdateCheckResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.UpdateAvailable {
		t.Error("update_available should be false for same version")
	}
}

func TestSystemUpdateCheck_RefreshBypassesCache(t *testing.T) {
	r, _, cookie, ghHits := newUpdateEnvWithCookie(t, nil)
	// 第一次请求走缓存
	doUpdate(r, http.MethodGet, "/api/v1/system/update/check", cookie)
	firstHits := atomic.LoadInt32(ghHits)
	if firstHits != 1 {
		t.Fatalf("first request should hit upstream once, got %d", firstHits)
	}
	// 第二次无 refresh 应命中缓存，不回源
	doUpdate(r, http.MethodGet, "/api/v1/system/update/check", cookie)
	if got := atomic.LoadInt32(ghHits); got != firstHits {
		t.Fatalf("cached request should not hit upstream, got %d", got)
	}
	// refresh=1 应绕过缓存直连
	doUpdate(r, http.MethodGet, "/api/v1/system/update/check?refresh=1", cookie)
	if got := atomic.LoadInt32(ghHits); got != 2 {
		t.Fatalf("refresh=1 should hit upstream again, got %d", got)
	}
}

// 上游真实失败（如 500）时 error 必须展示真实错误信息。
func TestSystemUpdateCheck_UpstreamError(t *testing.T) {
	r, _, cookie, _ := newUpdateEnvWithCookie(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("boom"))
	})
	w := doUpdate(r, http.MethodGet, "/api/v1/system/update/check", cookie)
	if w.Code != 200 {
		t.Fatalf("check: %d %s", w.Code, w.Body.String())
	}
	var resp UpdateCheckResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp.Error, "检查更新失败") || !strings.Contains(resp.Error, "500") {
		t.Errorf("error should surface the real upstream failure (status 500), got %q", resp.Error)
	}
}

func TestSystemUpdateCheck_RequiresAdmin(t *testing.T) {
	r, _, _, _ := newUpdateEnvWithCookie(t, nil)
	// 未认证应 401
	w := doUpdate(r, http.MethodGet, "/api/v1/system/update/check", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated: got %d, want 401", w.Code)
	}
}

// --- 已移除的一键更新端点应返回 404 ---

func TestSystemUpdateApply_Removed(t *testing.T) {
	r, _, cookie, _ := newUpdateEnvWithCookie(t, nil)
	w := doUpdate(r, http.MethodPost, "/api/v1/system/update/apply", cookie)
	if w.Code != http.StatusNotFound {
		t.Errorf("apply endpoint should be removed: got %d, want 404", w.Code)
	}
	w = doUpdate(r, http.MethodGet, "/api/v1/system/update/status", cookie)
	if w.Code != http.StatusNotFound {
		t.Errorf("status endpoint should be removed: got %d, want 404", w.Code)
	}
}

// --- semverCompare ---

func TestSemverCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.4.2", "1.4.1", 1},
		{"1.4.1", "1.4.2", -1},
		{"1.4.1", "1.4.1", 0},
		{"2.0.0", "1.9.9", 1},
		{"1.9.9", "2.0.0", -1},
		{"1.10.0", "1.9.0", 1},
		{"1.0.0-alpha", "1.0.0", -1},
		{"1.0.0", "1.0.0-alpha", 1},
		{"1.0.0-alpha", "1.0.0-alpha", 0},
		{"1.0.0-beta", "1.0.0-alpha", 1},
		{"1.0.0-alpha.1", "1.0.0-alpha", 1},
		// 无效输入返回 0
		{"invalid", "1.0.0", 0},
		{"1.0.0", "invalid", 0},
	}
	for _, c := range cases {
		got := semverCompare(c.a, c.b)
		if got != c.want {
			t.Errorf("semverCompare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// --- splitOwnerRepo ---

func TestSplitOwnerRepo(t *testing.T) {
	cases := []struct {
		input string
		owner string
		repo  string
	}{
		{"SakuraLoveSmile/Sakura-Blog-Release", "SakuraLoveSmile", "Sakura-Blog-Release"},
		{"a/b/c", "a", "b/c"},
		{"justrepo", "", "justrepo"},
	}
	for _, c := range cases {
		owner, repo := splitOwnerRepo(c.input)
		if owner != c.owner || repo != c.repo {
			t.Errorf("splitOwnerRepo(%q) = (%q, %q), want (%q, %q)", c.input, owner, repo, c.owner, c.repo)
		}
	}
}

// --- resolveCurrentVersion ---

func TestResolveCurrentVersion(t *testing.T) {
	r, h, cookie, _ := newUpdateEnvWithCookie(t, nil)
	_ = cookie
	_ = r
	// AppVersion=v1.4.1 → 应解析为 1.4.1
	if v := h.resolveCurrentVersion(); v != "1.4.1" {
		t.Errorf("resolveCurrentVersion: got %q, want 1.4.1", v)
	}

	// AppVersion=dev → 应回退到 unknown
	h.appVersion = "dev"
	if v := h.resolveCurrentVersion(); v != "unknown" {
		t.Errorf("resolveCurrentVersion with dev: got %q, want unknown", v)
	}
}
