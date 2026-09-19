package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"

	ghpkg "blog/internal/github"
	tmpl "blog/internal/settings"
	updr "blog/internal/updaterunner"
)

// fakeRunner 测试用 Runner，可控制 Supported/Start/Active 行为。
type fakeRunner struct {
	supported  bool
	reason     string
	startErr   error
	active     bool
	started    int32 // Start 调用计数
	lastTarget string
	taskInfo   *updr.TaskInfo
	statusInfo *updr.StatusInfo
}

func (f *fakeRunner) Supported() bool           { return f.supported }
func (f *fakeRunner) UnsupportedReason() string { return f.reason }
func (f *fakeRunner) Start(_ context.Context, targetTag string) (*updr.TaskInfo, error) {
	atomic.AddInt32(&f.started, 1)
	f.lastTarget = targetTag
	if f.startErr != nil {
		return nil, f.startErr
	}
	if f.taskInfo != nil {
		return f.taskInfo, nil
	}
	return &updr.TaskInfo{
		TaskID:        "test-task-1",
		TargetVersion: targetTag,
		Stage:         "queued",
		StartedAt:     "2026-08-15T10:00:00Z",
	}, nil
}
func (f *fakeRunner) Status(_ context.Context, currentVersion string) (*updr.StatusInfo, error) {
	if f.statusInfo != nil {
		return f.statusInfo, nil
	}
	if f.active {
		return &updr.StatusInfo{
			State:          "running",
			Stage:          "downloading",
			CurrentVersion: currentVersion,
			TargetVersion:  "1.4.2",
		}, nil
	}
	return &updr.StatusInfo{
		State:          "idle",
		CurrentVersion: currentVersion,
	}, nil
}
func (f *fakeRunner) Active(context.Context) bool { return f.active }

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
		w.Write([]byte(`{"tag_name":"v1.4.2","html_url":"https://github.com/SakuraLoveSmile/Sakura-Blog/releases/tag/v1.4.2","body":"Bug fixes.","published_at":"2026-08-15T10:00:00Z"}`))
	}))
	t.Cleanup(srv.Close)

	gh := ghpkg.New(ghpkg.Config{CacheDir: t.TempDir(), APIBase: srv.URL, HTTPClient: srv.Client()})

	r := gin.New()
	h, err := MountConsole(r, MountDeps{
		Config: Config{
			Store:      st,
			Site:       sitePtr(site),
			AppVersion: "v1.4.1",
			UpdateRepo: "SakuraLoveSmile/Sakura-Blog",
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

func TestSystemUpdateCheck_UnsupportedEnvironment(t *testing.T) {
	r, h, cookie, _ := newUpdateEnvWithCookie(t, nil)
	// 替换为不支持更新的 runner
	h.upd = &fakeRunner{supported: false, reason: "当前系统非 Linux，不支持一键更新"}
	w := doUpdate(r, http.MethodGet, "/api/v1/system/update/check", cookie)
	if w.Code != 200 {
		t.Fatalf("check: %d", w.Code)
	}
	var resp UpdateCheckResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.ApplySupported {
		t.Error("apply_supported should be false")
	}
	// 不支持原因只走 apply_note，不再污染 error（GitHub 正常时 error 应为空）
	if resp.ApplyNote == "" {
		t.Error("apply_note should contain unsupported reason")
	}
	if resp.Error != "" {
		t.Errorf("error should be empty when GitHub check succeeds, got %q", resp.Error)
	}
	// 仍应返回版本信息
	if resp.LatestVersion == "" {
		t.Error("latest_version should still be populated")
	}
}

// TestSystemUpdateCheck_UnsupportedShowsRealError 锁定：runner 不支持时，
// 若 GitHub 拉取真实失败（如 500），error 必须展示真实上游错误，
// 且 apply_note 同时存在——不再被「非 Linux」文案掩盖。
func TestSystemUpdateCheck_UnsupportedShowsRealError(t *testing.T) {
	r, h, cookie, _ := newUpdateEnvWithCookie(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("boom"))
	})
	h.upd = &fakeRunner{supported: false, reason: "当前系统非 Linux，不支持一键更新"}
	w := doUpdate(r, http.MethodGet, "/api/v1/system/update/check", cookie)
	if w.Code != 200 {
		t.Fatalf("check: %d %s", w.Code, w.Body.String())
	}
	var resp UpdateCheckResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.ApplyNote == "" {
		t.Error("apply_note should still carry unsupported reason")
	}
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

// --- apply 端点 ---

func TestSystemUpdateApply_Unsupported(t *testing.T) {
	r, h, cookie, _ := newUpdateEnvWithCookie(t, nil)
	h.upd = &fakeRunner{supported: false, reason: "不支持"}
	w := doUpdate(r, http.MethodPost, "/api/v1/system/update/apply", cookie)
	if w.Code != 400 {
		t.Errorf("apply on unsupported: got %d, want 400", w.Code)
	}
}

func TestSystemUpdateApply_Success(t *testing.T) {
	runner := &fakeRunner{supported: true, active: true}
	r, h, cookie, _ := newUpdateEnvWithCookie(t, nil)
	h.upd = runner
	w := doUpdate(r, http.MethodPost, "/api/v1/system/update/apply", cookie)
	if w.Code != 202 {
		t.Fatalf("apply: %d %s", w.Code, w.Body.String())
	}
	var resp UpdateApplyResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Started {
		t.Error("started should be true")
	}
	if resp.TargetVersion != "1.4.2" {
		t.Errorf("target: got %q, want 1.4.2", resp.TargetVersion)
	}
	if resp.TaskID != "test-task-1" {
		t.Errorf("task_id: got %q, want test-task-1", resp.TaskID)
	}
	if got := atomic.LoadInt32(&runner.started); got != 1 {
		t.Errorf("Start should be called once, got %d", got)
	}
}

func TestSystemUpdateApply_ConcurrentBlocked(t *testing.T) {
	runner := &fakeRunner{supported: true, active: true}
	r, h, cookie, _ := newUpdateEnvWithCookie(t, nil)
	h.upd = runner
	// 第一次成功
	w1 := doUpdate(r, http.MethodPost, "/api/v1/system/update/apply", cookie)
	if w1.Code != 202 {
		t.Fatalf("first apply: %d", w1.Code)
	}
	// 第二次由于 defer 释放 CAS 锁，再次调用返回 202
	w2 := doUpdate(r, http.MethodPost, "/api/v1/system/update/apply", cookie)
	if w2.Code != 202 {
		t.Fatalf("second apply after CAS release: %d", w2.Code)
	}
	// runner 返回 ErrBusy 时应返回 409
	runner.startErr = updr.ErrBusy
	w3 := doUpdate(r, http.MethodPost, "/api/v1/system/update/apply", cookie)
	if w3.Code != 409 {
		t.Fatalf("busy runner should return 409, got %d", w3.Code)
	}
}

func TestSystemUpdateApply_StartError(t *testing.T) {
	runner := &fakeRunner{supported: true, startErr: assertError("权限不足")}
	r, h, cookie, _ := newUpdateEnvWithCookie(t, nil)
	h.upd = runner
	w := doUpdate(r, http.MethodPost, "/api/v1/system/update/apply", cookie)
	if w.Code != 500 {
		t.Fatalf("apply with start error: got %d, want 500", w.Code)
	}
}

// --- status 端点 ---

func TestSystemUpdateStatus_Idle(t *testing.T) {
	r, _, cookie, _ := newUpdateEnvWithCookie(t, nil)
	w := doUpdate(r, http.MethodGet, "/api/v1/system/update/status", cookie)
	if w.Code != 200 {
		t.Fatalf("status: %d", w.Code)
	}
	var resp UpdateStatusResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.State != "idle" {
		t.Errorf("state: got %q, want idle", resp.State)
	}
}

func TestSystemUpdateStatus_Running(t *testing.T) {
	runner := &fakeRunner{supported: true, active: true}
	r, h, cookie, _ := newUpdateEnvWithCookie(t, nil)
	h.upd = runner
	// 先 apply 写入状态文件
	doUpdate(r, http.MethodPost, "/api/v1/system/update/apply", cookie)
	// 再查 status
	w := doUpdate(r, http.MethodGet, "/api/v1/system/update/status", cookie)
	if w.Code != 200 {
		t.Fatalf("status: %d", w.Code)
	}
	var resp UpdateStatusResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.State != "running" {
		t.Errorf("state: got %q, want running", resp.State)
	}
	if resp.TargetVersion != "1.4.2" {
		t.Errorf("target: got %q, want 1.4.2", resp.TargetVersion)
	}
}

func TestSystemUpdateStatus_Succeeded(t *testing.T) {
	r, h, cookie, _ := newUpdateEnvWithCookie(t, nil)
	// 手动写入状态文件
	h.writeUpdateState(updateState{From: "1.4.1", To: "1.4.1", StartedAt: "2026-08-15T10:00:00Z"})
	// 当前版本也是 1.4.1 → 应判定 succeeded
	runner := &fakeRunner{supported: true, active: false}
	h.upd = runner
	w := doUpdate(r, http.MethodGet, "/api/v1/system/update/status", cookie)
	if w.Code != 200 {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	var resp UpdateStatusResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.State != "succeeded" {
		t.Errorf("state: got %q, want succeeded", resp.State)
	}
}

func TestSystemUpdateStatus_Failed(t *testing.T) {
	r, h, cookie, _ := newUpdateEnvWithCookie(t, nil)
	// 状态文件 target=1.4.2 但当前版本=1.4.1 → failed
	h.writeUpdateState(updateState{From: "1.4.1", To: "1.4.2", StartedAt: "2026-08-15T10:00:00Z"})
	runner := &fakeRunner{supported: true, active: false}
	h.upd = runner
	w := doUpdate(r, http.MethodGet, "/api/v1/system/update/status", cookie)
	if w.Code != 200 {
		t.Fatalf("status: %d", w.Code)
	}
	var resp UpdateStatusResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.State != "failed" {
		t.Errorf("state: got %q, want failed", resp.State)
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

// --- NoopRunner ---

func TestNoopRunner(t *testing.T) {
	n := updr.NoopRunner{}
	if n.Supported() {
		t.Error("NoopRunner.Supported should be false")
	}
	if n.UnsupportedReason() == "" {
		t.Error("NoopRunner.UnsupportedReason should not be empty")
	}
	if _, err := n.Start(context.Background(), "1.4.2"); err == nil {
		t.Error("NoopRunner.Start should error")
	}
	st, err := n.Status(context.Background(), "1.4.1")
	if err != nil || st == nil || st.State != "idle" {
		t.Errorf("NoopRunner.Status unexpected: %v, %v", st, err)
	}
	if n.Active(context.Background()) {
		t.Error("NoopRunner.Active should be false")
	}
}

// assertError 是一个简单的 error 包装，用于测试中构造 Start 错误。
type assertError string

func (e assertError) Error() string { return string(e) }

// B 的维护预约必须在 C 的更新请求入队前阻止冲突，不调用更新器。
func TestSystemUpdateApplyRestoreReservationBlocksQueue(t *testing.T) {
	r, h, cookie, _ := newUpdateEnvWithCookie(t, nil)
	runner := &fakeRunner{supported: true}
	h.upd = runner
	release, err := h.store.Maintenance().Reserve("restore")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	w := doUpdate(r, http.MethodPost, "/api/v1/system/update/apply", cookie)
	if w.Code != http.StatusConflict || atomic.LoadInt32(&runner.started) != 0 {
		t.Fatalf("restore reservation must block enqueue: status=%d starts=%d", w.Code, runner.started)
	}
}
