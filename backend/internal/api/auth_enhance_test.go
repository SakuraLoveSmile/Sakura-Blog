package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
	tmpl "blog/internal/template"
)

// Wave 3a 认证增强测试：cookie 滑动续期 / Bearer API Token / 登录日志落库与查询。

// authEnhanceEnv 带密码站点 + users 表 admin 用户，返回 engine、store、用户与 handler。
func authEnhanceEnv(t *testing.T) (*gin.Engine, *store.Store, *store.User, *Handler) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	if err := st.EnsureUsersTable(); err != nil {
		t.Fatalf("ensure users table: %v", err)
	}
	if err := st.EnsureLoginLogsTable(); err != nil {
		t.Fatalf("ensure login logs table: %v", err)
	}
	user := &store.User{
		Username:     "admin",
		PasswordHash: "x",
		AuthSecret:   store.GenerateAuthSecret(),
		Role:         store.RoleAdmin,
	}
	if err := st.CreateUser(user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "secret")
	r := gin.New()
	h, err := Mount(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site)}})
	if err != nil {
		t.Fatalf("mount: %v", err)
	}
	return r, st, user, h
}

// seedLoginUser 给 admin 用户设置可用的 bcrypt 密码（默认 "secret"），返回用户。
func seedLoginUser(t *testing.T, st *store.Store, user *store.User) *store.User {
	t.Helper()
	hash, err := store.HashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := st.UpdatePassword(user.ID, hash, user.AuthSecret); err != nil {
		t.Fatalf("update password: %v", err)
	}
	return user
}

// TestCookieSlidingRenewal 滑动续期：剩余有效期不足 TTL 一半时响应重签新 cookie；
// 剩余充足时不重签。
func TestCookieSlidingRenewal(t *testing.T) {
	r, _, user, h := authEnhanceEnv(t)

	// 剩余 2 天（< 7/2=3.5 天）→ 应续期
	old := signAuthCookie(time.Now().Add(2*24*time.Hour).Unix(), user.ID, user.AuthSecret)
	w := do2FA(r, http.MethodGet, "/api/v1/profile", "", old)
	if w.Code != http.StatusOK {
		t.Fatalf("profile with near-expiry cookie: status = %d", w.Code)
	}
	setCookies := w.Result().Cookies()
	var renewed string
	for _, ck := range setCookies {
		if ck.Name == authCookieName && ck.Value != "" && ck.Value != old {
			renewed = ck.Value
		}
	}
	if renewed == "" {
		t.Fatal("near-expiry cookie should trigger renewal Set-Cookie")
	}
	// 新 cookie 应为新格式且过期时间晚于原 cookie
	if _, _, expiry, _, ok := h.validAuthCookie(renewed); !ok {
		t.Fatal("renewed cookie should be valid")
	} else if expiry <= time.Now().Add(2*24*time.Hour).Unix() {
		t.Errorf("renewed expiry = %d, want later than original %d", expiry, time.Now().Add(2*24*time.Hour).Unix())
	}

	// 剩余 6 天（> 3.5 天）→ 不续期
	fresh := signAuthCookie(time.Now().Add(6*24*time.Hour).Unix(), user.ID, user.AuthSecret)
	w2 := do2FA(r, http.MethodGet, "/api/v1/profile", "", fresh)
	if w2.Code != http.StatusOK {
		t.Fatalf("profile with fresh cookie: status = %d", w2.Code)
	}
	for _, ck := range w2.Result().Cookies() {
		if ck.Name == authCookieName {
			t.Errorf("fresh cookie should NOT trigger renewal, got Set-Cookie %q", ck.Value)
		}
	}
}

// TestBearerTokenAuth 三例：有效 token 通过、无效 token 401、撤销后失效 401。
func TestBearerTokenAuth(t *testing.T) {
	r, _, user, _ := authEnhanceEnv(t)
	cookie := authCookie(user)

	// 未生成 token 时：Bearer 无效
	w2 := doBearer(r, "some-invalid-token")
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("invalid bearer before generation: status = %d, want 401", w2.Code)
	}

	// 生成 token（需要 cookie 认证）
	gen := do2FA(r, http.MethodPost, "/api/v1/profile/api-token", "", cookie)
	if gen.Code != http.StatusOK {
		t.Fatalf("generate api token: status = %d, body = %s", gen.Code, gen.Body.String())
	}
	var genBody struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(gen.Body.Bytes(), &genBody); err != nil || genBody.Token == "" {
		t.Fatalf("generate api token should return plaintext token once: %v / %q", err, gen.Body.String())
	}

	// 有效 token：Bearer 认证通过
	w3 := doBearer(r, genBody.Token)
	if w3.Code != http.StatusOK {
		t.Fatalf("valid bearer: status = %d, want 200", w3.Code)
	}

	// 状态查询：has_token = true
	st := do2FA(r, http.MethodGet, "/api/v1/profile/api-token", "", cookie)
	var stBody struct {
		HasToken bool `json:"has_token"`
	}
	if err := json.Unmarshal(st.Body.Bytes(), &stBody); err != nil || !stBody.HasToken {
		t.Fatalf("api-token status should report has_token=true: %v / %s", err, st.Body.String())
	}

	// 撤销后原 token 立即失效
	rev := do2FA(r, http.MethodDelete, "/api/v1/profile/api-token", "", cookie)
	if rev.Code != http.StatusOK {
		t.Fatalf("revoke api token: status = %d", rev.Code)
	}
	w4 := doBearer(r, genBody.Token)
	if w4.Code != http.StatusUnauthorized {
		t.Fatalf("revoked bearer: status = %d, want 401", w4.Code)
	}
}

// TestLoginLogs 登录成功/失败落库，查询 API 分页 + 状态筛选；未认证访问 401。
func TestLoginLogs(t *testing.T) {
	r, st, user, _ := authEnhanceEnv(t)
	seedLoginUser(t, st, user)
	cookie := authCookie(user)

	// 未认证访问日志 → 401
	w := do2FA(r, http.MethodGet, "/api/v1/auth/login-logs", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("login-logs without auth: status = %d, want 401", w.Code)
	}

	// 成功登录（用户表密码）
	ok := do2FA(r, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"secret"}`, "")
	if ok.Code != http.StatusOK {
		t.Fatalf("login success: status = %d, body = %s", ok.Code, ok.Body.String())
	}
	// 失败登录（错误密码）
	bad := do2FA(r, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"wrong"}`, "")
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("login fail: status = %d, want 401", bad.Code)
	}

	// 查询全部：应有 success + fail 各 1
	logs := fetchLoginLogs(t, r, cookie, "")
	if logs.Total < 2 {
		t.Fatalf("login logs total = %d, want >= 2", logs.Total)
	}
	got := map[string]int{}
	for _, it := range logs.Items {
		got[it.Status]++
	}
	if got["success"] < 1 || got["fail"] < 1 {
		t.Errorf("login logs should contain both success and fail, got %+v", got)
	}
	if logs.Items[0].Username != "admin" || logs.Items[0].IP == "" {
		t.Errorf("log item should carry username/ip, got %+v", logs.Items[0])
	}

	// status=fail 筛选：只返回失败记录
	onlyFail := fetchLoginLogs(t, r, cookie, "fail")
	if onlyFail.Total < 1 {
		t.Fatalf("filter fail: total = %d, want >= 1", onlyFail.Total)
	}
	for _, it := range onlyFail.Items {
		if it.Status != "fail" {
			t.Errorf("filter fail returned status = %q", it.Status)
		}
	}
}

// loginLogsResp login-logs 响应结构。
type loginLogsResp struct {
	Items []*store.LoginLog `json:"items"`
	Total int64             `json:"total"`
}

// fetchLoginLogs 带 cookie 查询登录日志，解析 items/total。
func fetchLoginLogs(t *testing.T, r *gin.Engine, cookie, status string) loginLogsResp {
	t.Helper()
	path := "/api/v1/auth/login-logs"
	if status != "" {
		path += "?status=" + status
	}
	w := do2FA(r, http.MethodGet, path, "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("fetch login logs: status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp loginLogsResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode login logs: %v", err)
	}
	return resp
}

// doBearer 发起带 Bearer 头的 GET 请求。
func doBearer(r *gin.Engine, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r.ServeHTTP(w, req)
	return w
}
