package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	tmpl "blog/internal/settings"
)

// Wave 4 测试覆盖补强：认证流程补齐（登出/authCheck/改密码/旧版登录/2FA 两步登录）。

// TestLogoutClearsCookie 登出响应携带清 cookie 指令（无状态签名 cookie，服务端无法撤销，
// 清 cookie 是登出的唯一机制，旧 cookie 在过期前仍可继续使用，属设计现状）。
func TestLogoutClearsCookie(t *testing.T) {
	r, st, user, _ := authEnhanceEnv(t)
	seedLoginUser(t, st, user)
	cookie := authCookie(user)

	// 登出
	w := do2FA(r, http.MethodPost, "/api/v1/auth/logout", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("logout: status = %d", w.Code)
	}
	// 响应应携带清除指令：同名 cookie 且 MaxAge <= 0
	var cleared bool
	for _, ck := range w.Result().Cookies() {
		if ck.Name == authCookieName {
			cleared = true
			if ck.MaxAge > 0 || ck.Value != "" {
				t.Errorf("logout cookie: maxage = %d value = %q, want clearing", ck.MaxAge, ck.Value)
			}
		}
	}
	if !cleared {
		t.Error("logout should set clearing cookie")
	}
	// 登出后再登出应幂等（未登录也 200）
	w = do2FA(r, http.MethodPost, "/api/v1/auth/logout", "", "")
	if w.Code != http.StatusOK {
		t.Errorf("logout without auth: status = %d, want 200", w.Code)
	}
}

// TestAuthCheckStates auth/check 三态：无 cookie / 有效 cookie / 无密码站点。
func TestAuthCheckStates(t *testing.T) {
	// 带密码站点
	r, st, user, _ := authEnhanceEnv(t)
	seedLoginUser(t, st, user)

	w := do2FA(r, http.MethodGet, "/api/v1/auth/check", "", "")
	var resp struct {
		Authenticated    bool `json:"authenticated"`
		PasswordRequired bool `json:"passwordRequired"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode check: %v", err)
	}
	if resp.Authenticated || !resp.PasswordRequired {
		t.Errorf("no cookie: got %+v, want authenticated=false passwordRequired=true", resp)
	}

	w = do2FA(r, http.MethodGet, "/api/v1/auth/check", "", authCookie(user))
	resp.Authenticated = false
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Authenticated || !resp.PasswordRequired {
		t.Errorf("valid cookie: got %+v, want authenticated=true passwordRequired=true", resp)
	}

	// 无密码站点（Password 为空）
	st2 := newTestStore(t)
	r2 := gin.New()
	if _, err := MountConsole(r2, MountDeps{Config: Config{Store: st2, Site: sitePtr(&tmpl.Site{})}}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	w = do2FA(r2, http.MethodGet, "/api/v1/auth/check", "", "")
	resp = struct {
		Authenticated    bool `json:"authenticated"`
		PasswordRequired bool `json:"passwordRequired"`
	}{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode check: %v", err)
	}
	if !resp.Authenticated || resp.PasswordRequired {
		t.Errorf("no-password site: got %+v, want authenticated=true passwordRequired=false", resp)
	}
}

// TestLoginNoUsernameUsesAdminUser（Spec 61 阶段 4）：无 username 登录默认 admin 用户（users 表 bcrypt），
// 成功签发新格式 cookie（3 段 expiry.userID.sig）并可访问受保护接口。
// 旧版明文密码比对与旧格式 cookie（2 段明文 HMAC）已移除。
func TestLoginNoUsernameUsesAdminUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "secret")
	r := gin.New()
	h, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site)}})
	if err != nil {
		t.Fatalf("mount: %v", err)
	}
	// 错误密码 → 401 并写失败日志
	w := do2FA(r, http.MethodPost, "/api/v1/auth/login", `{"password":"wrong"}`, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: status = %d, want 401", w.Code)
	}
	// 正确密码 → 200 + 新格式 cookie（3 段 expiry.userID.sig）
	w = do2FA(r, http.MethodPost, "/api/v1/auth/login", `{"password":"secret"}`, "")
	if w.Code != http.StatusOK {
		t.Fatalf("login: status = %d, body = %s", w.Code, w.Body.String())
	}
	var cookie string
	for _, ck := range w.Result().Cookies() {
		if ck.Name == authCookieName {
			cookie = ck.Value
		}
	}
	if cookie == "" {
		t.Fatal("login should set auth cookie")
	}
	// 安全审计 2026-08-16：cookie 升级为 4 段（aud.expiry.userID.sig）
	if len(strings.Split(cookie, ".")) != 4 {
		t.Fatalf("cookie should be new format (4 segments), got %q", cookie)
	}
	// 新格式 cookie 可访问受保护接口
	w = do2FA(r, http.MethodGet, "/api/v1/profile", "", cookie)
	if w.Code != http.StatusOK {
		t.Errorf("profile with cookie: status = %d, want 200", w.Code)
	}
	if h == nil {
		t.Fatal("mount should return handler")
	}
}

// TestChangePasswordFlow 改密码：错误旧密码 401；成功后旧 cookie 失效、新密码可登录。
func TestChangePasswordFlow(t *testing.T) {
	r, st, user, _ := authEnhanceEnv(t)
	seedLoginUser(t, st, user)
	cookie := authCookie(user)

	// 旧密码错误 → 401
	w := do2FA(r, http.MethodPut, "/api/v1/auth/password",
		`{"current":"wrong","new":"newpass"}`, cookie)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong current password: status = %d, want 401", w.Code)
	}

	// 改密码成功 → 200
	w = do2FA(r, http.MethodPut, "/api/v1/auth/password",
		`{"current":"secret","new":"newpass"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("change password: status = %d, body = %s", w.Code, w.Body.String())
	}

	// 旧 cookie 因 auth_secret 轮换失效 → 401
	w = do2FA(r, http.MethodGet, "/api/v1/profile", "", cookie)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("old cookie after password change: status = %d, want 401", w.Code)
	}

	// 新密码可登录，旧密码不可
	w = do2FA(r, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"newpass"}`, "")
	if w.Code != http.StatusOK {
		t.Errorf("login with new password: status = %d, want 200", w.Code)
	}
	w = do2FA(r, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"secret"}`, "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("login with old password: status = %d, want 401", w.Code)
	}
}

// TestLogin2FAFlow 两步登录：TOTP 用户登录返回 require_2fa token，正确 code 换 cookie。
func TestLogin2FAFlow(t *testing.T) {
	r, st, user, _ := authEnhanceEnv(t)
	seedLoginUser(t, st, user)

	// 给用户启用 2FA
	secret := GenerateSecret()
	if err := st.UpdateTOTPSecret(user.ID, secret, GenerateRecoveryCodes(8)); err != nil {
		t.Fatalf("enable 2fa: %v", err)
	}

	// 第一步：登录返回 require_2fa + token
	w := do2FA(r, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"secret"}`, "")
	if w.Code != http.StatusOK {
		t.Fatalf("login step1: status = %d, body = %s", w.Code, w.Body.String())
	}
	var step1 struct {
		Require2FA bool   `json:"require_2fa"`
		Token      string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &step1); err != nil {
		t.Fatalf("decode step1: %v", err)
	}
	if !step1.Require2FA || step1.Token == "" {
		t.Fatalf("step1 should require 2fa with token, got %+v", step1)
	}

	// 错误 code → 401（token 被一次性消费）
	w = do2FA(r, http.MethodPost, "/api/v1/auth/2fa", `{"token":"`+step1.Token+`","code":"000000"}`, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("step2 wrong code: status = %d, want 401", w.Code)
	}

	// 重新登录拿新 token，再验证正确 code → 200 + cookie
	w = do2FA(r, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"secret"}`, "")
	if err := json.Unmarshal(w.Body.Bytes(), &step1); err != nil || !step1.Require2FA || step1.Token == "" {
		t.Fatalf("second login step1: %+v (err %v)", step1, err)
	}
	w = do2FA(r, http.MethodPost, "/api/v1/auth/2fa",
		`{"token":"`+step1.Token+`","code":"`+totpCodeFor(t, secret)+`"}`, "")
	if w.Code != http.StatusOK {
		t.Fatalf("step2 ok: status = %d, body = %s", w.Code, w.Body.String())
	}
	var cookie string
	for _, ck := range w.Result().Cookies() {
		if ck.Name == authCookieName {
			cookie = ck.Value
		}
	}
	if cookie == "" {
		t.Fatal("step2 should set auth cookie")
	}
	// 复放同一 token → 401（一次性）
	w = do2FA(r, http.MethodPost, "/api/v1/auth/2fa",
		`{"token":"`+step1.Token+`","code":"`+totpCodeFor(t, secret)+`"}`, "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("replay 2fa token: status = %d, want 401", w.Code)
	}
}

// TestProfileUpdate PUT /api/v1/profile 更新资料并回读。
func TestProfileUpdate(t *testing.T) {
	r, st, user, _ := authEnhanceEnv(t)
	cookie := authCookie(user)

	w := do2FA(r, http.MethodPut, "/api/v1/profile",
		`{"display_name":"博主","email":"a@b.com","avatar":"https://x/a.png"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("update profile: status = %d, body = %s", w.Code, w.Body.String())
	}
	got, err := st.GetUserByID(user.ID)
	if err != nil || got.DisplayName != "博主" || got.Email != "a@b.com" {
		t.Errorf("profile not updated: %+v (err %v)", got, err)
	}
	// 非法 JSON → 400
	w = do2FA(r, http.MethodPut, "/api/v1/profile", `{bad`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid profile json: status = %d, want 400", w.Code)
	}
}
