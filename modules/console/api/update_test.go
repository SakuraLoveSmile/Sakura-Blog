package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"

	ghcr "blog/internal/ghcr"
	ghpkg "blog/internal/github"
	tmpl "blog/internal/settings"
)

// newUpdateEnvWithCookie 构造带 GHCR 模拟的测试环境，额外返回 admin cookie 与标签接口命中次数。
func newUpdateEnvWithCookie(t *testing.T, tagsHandler http.HandlerFunc) (*gin.Engine, *Handler, string, *int32) {
	t.Helper()
	return newUpdateEnvWithRelease(t, tagsHandler, nil)
}

// newUpdateEnvWithRelease 在 GHCR 模拟之外再注入 GitHub Release 模拟（nil 表示没有 Release）。
func newUpdateEnvWithRelease(t *testing.T, tagsHandler, releaseHandler http.HandlerFunc) (*gin.Engine, *Handler, string, *int32) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "secret")

	admin, err := st.GetUserByUsername("admin")
	if err != nil || admin == nil {
		t.Fatalf("get admin: %v", err)
	}

	var tagHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"token":"test-token"}`))
		case strings.HasPrefix(r.URL.Path, "/v2/"):
			atomic.AddInt32(&tagHits, 1)
			if tagsHandler != nil {
				tagsHandler(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"name":"sakuralovesmile/sakura-blog","tags":["2.0.4","2.0","2","latest","2.0.3","sha-abc1234"]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	registry := ghcr.New(ghcr.Config{RegistryBase: srv.URL, CacheDir: t.TempDir(), HTTPClient: srv.Client()})

	ghSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/repos/SakuraLoveSmile/Sakura-Blog/releases/latest") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if releaseHandler != nil {
			releaseHandler(w, r)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not Found"}`))
	}))
	t.Cleanup(ghSrv.Close)
	gh := ghpkg.New(ghpkg.Config{CacheDir: t.TempDir(), APIBase: ghSrv.URL, HTTPClient: ghSrv.Client()})

	r := gin.New()
	h, err := MountConsole(r, MountDeps{
		Config: Config{
			Store:      st,
			Site:       sitePtr(site),
			AppVersion: "v1.4.1",
			UpdateRepo: "SakuraLoveSmile/Sakura-Blog",
			Root:       t.TempDir(),
			Registry:   registry,
		},
		GH: gh,
	})
	if err != nil {
		t.Fatalf("mount: %v", err)
	}
	cookie := authCookie(admin)
	return r, h, cookie, &tagHits
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
	if resp.LatestVersion != "2.0.4" {
		t.Errorf("latest: got %q, want 2.0.4", resp.LatestVersion)
	}
	if !resp.UpdateAvailable {
		t.Error("update_available should be true")
	}
	if !strings.Contains(resp.ReleaseURL, "/pkgs/container/sakura-blog") {
		t.Errorf("release_url should point to the GHCR package page, got %q", resp.ReleaseURL)
	}
}

func TestSystemUpdateCheck_NoUpdate(t *testing.T) {
	r, _, cookie, _ := newUpdateEnvWithCookie(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tags":["1.4.1","1.4.0","2.0","latest"]}`))
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
	if resp.LatestVersion != "1.4.1" {
		t.Errorf("latest: got %q, want 1.4.1", resp.LatestVersion)
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
	if !strings.Contains(resp.Error, "检查镜像标签失败") || !strings.Contains(resp.Error, "500") {
		t.Errorf("error should surface the real upstream failure (status 500), got %q", resp.Error)
	}
	if resp.LatestVersion != "" {
		t.Errorf("latest_version should stay empty on failure, got %q", resp.LatestVersion)
	}
}

// Release 与最新镜像标签一致时，更新日志、发布时间与发布说明链接应回填。
func TestSystemUpdateCheck_ReleaseNotes(t *testing.T) {
	const releaseURL = "https://github.com/SakuraLoveSmile/Sakura-Blog/releases/tag/v2.0.4"
	r, _, cookie, _ := newUpdateEnvWithRelease(t, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":"v2.0.4","html_url":"` + releaseURL + `","body":"- 统一镜像发布\n- 修复若干问题","published_at":"2026-11-02T09:00:00Z"}`))
	})
	w := doUpdate(r, http.MethodGet, "/api/v1/system/update/check", cookie)
	if w.Code != 200 {
		t.Fatalf("check: %d %s", w.Code, w.Body.String())
	}
	var resp UpdateCheckResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.LatestVersion != "2.0.4" {
		t.Fatalf("latest: got %q, want 2.0.4", resp.LatestVersion)
	}
	if !strings.Contains(resp.ReleaseNotes, "统一镜像发布") {
		t.Errorf("release_notes should carry the release body, got %q", resp.ReleaseNotes)
	}
	if resp.ReleaseNotesURL != releaseURL {
		t.Errorf("release_notes_url = %q, want %q", resp.ReleaseNotesURL, releaseURL)
	}
	if resp.PublishedAt == "" {
		t.Error("published_at should be filled from the release")
	}
	if !strings.Contains(resp.ReleaseURL, "/pkgs/container/sakura-blog") {
		t.Errorf("release_url should stay on the package page, got %q", resp.ReleaseURL)
	}
}

// Release 版本落后于最新镜像标签时不得展示过期日志。
func TestSystemUpdateCheck_StaleReleaseNotesIgnored(t *testing.T) {
	r, _, cookie, _ := newUpdateEnvWithRelease(t, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":"v2.0.3","html_url":"https://example.com/v2.0.3","body":"旧版本日志","published_at":"2026-10-31T07:00:00Z"}`))
	})
	w := doUpdate(r, http.MethodGet, "/api/v1/system/update/check", cookie)
	var resp UpdateCheckResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.LatestVersion != "2.0.4" {
		t.Fatalf("latest: got %q, want 2.0.4", resp.LatestVersion)
	}
	if resp.ReleaseNotes != "" || resp.ReleaseNotesURL != "" || resp.PublishedAt != "" {
		t.Errorf("stale release notes must be ignored, got notes=%q url=%q at=%q", resp.ReleaseNotes, resp.ReleaseNotesURL, resp.PublishedAt)
	}
}

// GitHub 获取失败只降级更新日志，不影响版本检查结果。
func TestSystemUpdateCheck_ReleaseFetchErrorDegrades(t *testing.T) {
	r, _, cookie, _ := newUpdateEnvWithRelease(t, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	w := doUpdate(r, http.MethodGet, "/api/v1/system/update/check", cookie)
	var resp UpdateCheckResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.LatestVersion != "2.0.4" || !resp.UpdateAvailable {
		t.Fatalf("version check must survive release fetch failure, got latest=%q update=%v", resp.LatestVersion, resp.UpdateAvailable)
	}
	if resp.ReleaseNotes != "" {
		t.Errorf("release notes should stay empty, got %q", resp.ReleaseNotes)
	}
	if resp.Error != "" {
		t.Errorf("release fetch failure must not surface as check error, got %q", resp.Error)
	}
}

// 派生标签（2 / 2.0 / latest / sha-*）不参与版本比较。
func TestLatestVersionTag(t *testing.T) {
	cases := []struct {
		tags []string
		want string
	}{
		{[]string{"2.0.4", "2.0", "2", "latest", "2.0.3", "sha-abc1234"}, "2.0.4"},
		{[]string{"1.10.0", "1.9.9"}, "1.10.0"},
		{[]string{"latest", "sha-abc"}, ""},
		{nil, ""},
	}
	for _, c := range cases {
		if got := latestVersionTag(c.tags); got != c.want {
			t.Errorf("latestVersionTag(%v) = %q, want %q", c.tags, got, c.want)
		}
	}
}

func TestRegistryPackageURL(t *testing.T) {
	got := registryPackageURL("SakuraLoveSmile/Sakura-Blog")
	want := "https://github.com/SakuraLoveSmile/Sakura-Blog/pkgs/container/sakura-blog"
	if got != want {
		t.Errorf("registryPackageURL = %q, want %q", got, want)
	}
	if got := registryPackageURL("single"); got != "" {
		t.Errorf("registryPackageURL without owner should be empty, got %q", got)
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
		{"SakuraLoveSmile/Sakura-Blog", "SakuraLoveSmile", "Sakura-Blog"},
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
