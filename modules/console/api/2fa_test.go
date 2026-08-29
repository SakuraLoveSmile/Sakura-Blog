package api

import (
	"bytes"
	"encoding/base32"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	tmpl "blog/internal/settings"
	"blog/internal/store"
)

type failingRandomReader struct{}

func (failingRandomReader) Read([]byte) (int, error) {
	return 0, errors.New("entropy unavailable")
}

func TestGenerate2FATokenRandomFailure(t *testing.T) {
	pending2FA.Lock()
	before := len(pending2FA.tokens)
	pending2FA.Unlock()

	token, err := generate2FATokenFrom(failingRandomReader{}, 42)
	if err == nil {
		t.Fatal("random reader failure should be returned")
	}
	if token != "" {
		t.Fatalf("failed token = %q, want empty", token)
	}

	pending2FA.Lock()
	after := len(pending2FA.tokens)
	pending2FA.Unlock()
	if after != before {
		t.Fatalf("pending 2FA tokens changed after random failure: before=%d after=%d", before, after)
	}
}

func TestLogin2FATokenGenerationFailure(t *testing.T) {
	r, st, user, h := authEnhanceEnv(t)
	seedLoginUser(t, st, user)
	if err := st.UpdateTOTPSecret(user.ID, GenerateSecret(), GenerateRecoveryCodes(8)); err != nil {
		t.Fatalf("enable 2fa: %v", err)
	}
	h.generate2FA = func(userID int64) (string, error) {
		return generate2FATokenFrom(failingRandomReader{}, userID)
	}

	pending2FA.Lock()
	before := len(pending2FA.tokens)
	pending2FA.Unlock()
	w := do2FA(r, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"secret"}`, "")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("login with random failure: status=%d body=%s, want 500", w.Code, w.Body.String())
	}
	if w.Result().Cookies() != nil {
		for _, cookie := range w.Result().Cookies() {
			if cookie.Name == authCookieName && cookie.Value != "" {
				t.Fatalf("random failure must not issue auth cookie: %q", cookie.Value)
			}
		}
	}
	pending2FA.Lock()
	after := len(pending2FA.tokens)
	pending2FA.Unlock()
	if after != before {
		t.Fatalf("pending 2FA tokens changed after login failure: before=%d after=%d", before, after)
	}
}

var _ io.Reader = failingRandomReader{}

// new2FATestEnv 带密码站点 + users 表 admin 用户，返回 engine、store 与用户。
func new2FATestEnv(t *testing.T) (*gin.Engine, *store.Store, *store.User) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	if err := st.EnsureUsersTable(); err != nil {
		t.Fatalf("ensure users table: %v", err)
	}
	user := &store.User{
		Username:     "admin",
		PasswordHash: "x",
		AuthSecret:   store.GenerateAuthSecret(),
	}
	if err := st.CreateUser(user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "secret")
	r := gin.New()
	if _, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site)}}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	return r, st, user
}

// authCookie 为测试用户签发有效签名 cookie（admin audience，安全审计 2026-08-16 起 4 段格式）。
func authCookie(user *store.User) string {
	return signAuthCookie(cookieAudAdmin, time.Now().Add(authCookieTTL).Unix(), user.ID, user.AuthSecret)
}

// do2FA 发起带 cookie 的 JSON 请求，返回 recorder。
func do2FA(r *gin.Engine, method, path, body, cookie string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: authCookieName, Value: cookie})
	}
	r.ServeHTTP(w, req)
	return w
}

// totpCodeFor 用测试 secret 计算当前窗口 TOTP code。
func totpCodeFor(t *testing.T, secret string) string {
	t.Helper()
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}
	return generateTOTP(key, time.Now().Unix()/totpPeriod)
}

// Test2FAEndpointsUnauthorized 未登录访问 4 个 2FA 管理端点应 401。
func Test2FAEndpointsUnauthorized(t *testing.T) {
	r, _, _ := new2FATestEnv(t)
	cases := []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/profile/2fa/setup", ""},
		{http.MethodPost, "/api/v1/profile/2fa/confirm", `{"secret":"S","code":"123456"}`},
		{http.MethodPost, "/api/v1/profile/2fa/disable", `{"code":"123456"}`},
		{http.MethodGet, "/api/v1/profile/2fa/recovery-codes", ""},
	}
	for _, c := range cases {
		w := do2FA(r, c.method, c.path, c.body, "")
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: got %d, want 401", c.method, c.path, w.Code)
		}
	}
}

// Test2FASetup 生成 secret/auth_url/8 恢复码，且不落库。
func Test2FASetup(t *testing.T) {
	r, st, user := new2FATestEnv(t)
	w := do2FA(r, http.MethodPost, "/api/v1/profile/2fa/setup", "", authCookie(user))
	if w.Code != http.StatusOK {
		t.Fatalf("setup: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Secret        string   `json:"secret"`
		AuthURL       string   `json:"auth_url"`
		RecoveryCodes []string `json:"recovery_codes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Secret == "" {
		t.Fatal("secret should not be empty")
	}
	if !strings.Contains(resp.AuthURL, "otpauth://totp/") || !strings.Contains(resp.AuthURL, "secret="+resp.Secret) || !strings.Contains(resp.AuthURL, "Stellar%20Blog") {
		t.Errorf("unexpected auth_url: %s", resp.AuthURL)
	}
	if len(resp.RecoveryCodes) != 8 {
		t.Errorf("recovery codes: got %d, want 8", len(resp.RecoveryCodes))
	}
	// 未确认前不落库
	got, _ := st.GetUserByID(user.ID)
	if got.TOTPSecret != "" {
		t.Error("setup must not persist TOTP secret before confirm")
	}
}

// Test2FAConfirm 正确 code 启用 2FA 并落库，错误 code 拒绝。
func Test2FAConfirm(t *testing.T) {
	r, st, user := new2FATestEnv(t)
	cookie := authCookie(user)

	secret := GenerateSecret()
	codes := GenerateRecoveryCodes(8)

	// 错误 code → 400 且不落库
	w := do2FA(r, http.MethodPost, "/api/v1/profile/2fa/confirm",
		`{"secret":"`+secret+`","code":"000000","recovery_codes":[]}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad code: got %d, want 400", w.Code)
	}
	got, _ := st.GetUserByID(user.ID)
	if got.TOTPSecret != "" {
		t.Error("failed confirm must not persist TOTP secret")
	}

	// 正确 code → 200 且落库
	body, _ := json.Marshal(map[string]any{
		"secret":         secret,
		"code":           totpCodeFor(t, secret),
		"recovery_codes": codes,
	})
	w = do2FA(r, http.MethodPost, "/api/v1/profile/2fa/confirm", string(body), cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("confirm: %d %s", w.Code, w.Body.String())
	}
	got, _ = st.GetUserByID(user.ID)
	if got.TOTPSecret != secret {
		t.Errorf("stored secret mismatch: got %q", got.TOTPSecret)
	}
	var stored []string
	if err := json.Unmarshal([]byte(got.TOTPRecoveryCodes), &stored); err != nil || len(stored) != 8 {
		t.Errorf("stored recovery codes: %v (err %v)", stored, err)
	}
}

// Test2FADisable 正确 code 清除 2FA，错误 code 拒绝。
func Test2FADisable(t *testing.T) {
	r, st, user := new2FATestEnv(t)
	cookie := authCookie(user)

	// 预置已启用状态
	secret := GenerateSecret()
	if err := st.UpdateTOTPSecret(user.ID, secret, GenerateRecoveryCodes(8)); err != nil {
		t.Fatalf("pre-enable: %v", err)
	}

	// 错误 code → 400，仍启用
	w := do2FA(r, http.MethodPost, "/api/v1/profile/2fa/disable", `{"code":"000000"}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad code: got %d, want 400", w.Code)
	}
	got, _ := st.GetUserByID(user.ID)
	if got.TOTPSecret != secret {
		t.Error("failed disable must keep TOTP secret")
	}

	// 正确 code → 200 且清除
	w = do2FA(r, http.MethodPost, "/api/v1/profile/2fa/disable",
		`{"code":"`+totpCodeFor(t, secret)+`"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("disable: %d %s", w.Code, w.Body.String())
	}
	got, _ = st.GetUserByID(user.ID)
	if got.TOTPSecret != "" {
		t.Errorf("disable should clear secret, got %q", got.TOTPSecret)
	}
}

// Test2FARecoveryCodes 返回已保存恢复码；未启用时返回空数组。
func Test2FARecoveryCodes(t *testing.T) {
	r, st, user := new2FATestEnv(t)
	cookie := authCookie(user)

	// 未启用 → 空数组
	w := do2FA(r, http.MethodGet, "/api/v1/profile/2fa/recovery-codes", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("recovery-codes: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Codes []string `json:"recovery_codes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Codes == nil || len(resp.Codes) != 0 {
		t.Errorf("expected empty codes, got %v", resp.Codes)
	}

	// 已保存 → 原样返回
	codes := GenerateRecoveryCodes(8)
	if err := st.UpdateTOTPSecret(user.ID, GenerateSecret(), codes); err != nil {
		t.Fatalf("set codes: %v", err)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/profile/2fa/recovery-codes", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("recovery-codes: %d %s", w.Code, w.Body.String())
	}
	resp.Codes = nil
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Codes) != 8 || resp.Codes[0] != codes[0] {
		t.Errorf("codes mismatch: got %v, want %v", resp.Codes, codes)
	}
}
