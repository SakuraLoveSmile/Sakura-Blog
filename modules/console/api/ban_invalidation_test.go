package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	tmpl "blog/internal/settings"
	"blog/internal/store"
)

// TestBannedUserImmediateInvalidation 验证 A3：封禁状态判定与即时凭据作废
// 1. 封禁后已签发的 Cookie 和 Bearer API Token 立即失效返回 401。
// 2. 登录接口对被封禁用户返回模糊凭据错误（401 "用户名或密码错误"），不泄露账户封禁或存在性。
// 3. 2FA 登录直接拦截，不派发 2FA 挑战；直接请求 /api/v1/auth/2fa 亦返回 401。
// 4. 解封后历史凭证不可复用（必须重新登录获得新 Cookie）。
func TestBannedUserImmediateInvalidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	if err := st.EnsureUsersTable(); err != nil {
		t.Fatalf("ensure users: %v", err)
	}
	if err := st.EnsureLoginLogsTable(); err != nil {
		t.Fatalf("ensure login logs: %v", err)
	}
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "admin-secret")

	// 创建 author 用户
	authorHash, _ := store.HashPassword("author-pass")
	author := &store.User{
		Username:     "author1",
		PasswordHash: authorHash,
		AuthSecret:   store.GenerateAuthSecret(),
		Role:         store.RoleAuthor,
		Status:       store.UserStatusActive,
	}
	if err := st.CreateUser(author); err != nil {
		t.Fatalf("create author: %v", err)
	}

	r := gin.New()
	_, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site)}})
	if err != nil {
		t.Fatalf("mount: %v", err)
	}

	// 1. 正常用户登录获得 cookie1
	loginW := do2FA(r, http.MethodPost, "/api/v1/auth/login", `{"username":"author1","password":"author-pass"}`, "")
	if loginW.Code != http.StatusOK {
		t.Fatalf("author login failed: %d, %s", loginW.Code, loginW.Body.String())
	}
	var cookie1 string
	for _, ck := range loginW.Result().Cookies() {
		if ck.Name == authCookieName && ck.Value != "" {
			cookie1 = ck.Value
			break
		}
	}
	if cookie1 == "" {
		t.Fatal("expected auth cookie")
	}

	// 2. 用 cookie1 生成 API Token token1
	genW := do2FA(r, http.MethodPost, "/api/v1/profile/api-token", "", cookie1)
	if genW.Code != http.StatusOK {
		t.Fatalf("gen token: %d, %s", genW.Code, genW.Body.String())
	}
	var genBody struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(genW.Body.Bytes(), &genBody); err != nil || genBody.Token == "" {
		t.Fatalf("decode token: %v", err)
	}
	token1 := genBody.Token

	// 3. 验证 cookie1 与 token1 均能成功访问受保护接口
	wProfileCookie := do2FA(r, http.MethodGet, "/api/v1/profile", "", cookie1)
	if wProfileCookie.Code != http.StatusOK {
		t.Fatalf("profile with cookie: status = %d", wProfileCookie.Code)
	}
	wProfileToken := doBearer(r, token1)
	if wProfileToken.Code != http.StatusOK {
		t.Fatalf("profile with token: status = %d", wProfileToken.Code)
	}

	// 4. 封禁该 author 用户
	if err := st.UpdateUserStatus(author.ID, store.UserStatusBanned); err != nil {
		t.Fatalf("ban author: %v", err)
	}
	// 封禁后提权也不能让已失效凭据复活或变成管理员会话。
	if err := st.SetUserRoles(author.ID, []string{store.RoleAdmin}); err != nil {
		t.Fatalf("promote banned author: %v", err)
	}

	// 5. 封禁后 cookie1 立即返回 401
	wBannedCookie := do2FA(r, http.MethodGet, "/api/v1/profile", "", cookie1)
	if wBannedCookie.Code != http.StatusUnauthorized {
		t.Fatalf("banned cookie status = %d (want 401)", wBannedCookie.Code)
	}

	// 6. 封禁后 token1 立即返回 401
	wBannedToken := doBearer(r, token1)
	if wBannedToken.Code != http.StatusUnauthorized {
		t.Fatalf("banned token status = %d (want 401)", wBannedToken.Code)
	}

	// 7. 封禁状态下尝试登录，返回通用 401 "用户名或密码错误"，不泄露封禁状态
	wBannedLogin := do2FA(r, http.MethodPost, "/api/v1/auth/login", `{"username":"author1","password":"author-pass"}`, "")
	if wBannedLogin.Code != http.StatusUnauthorized {
		t.Fatalf("banned login status = %d (want 401)", wBannedLogin.Code)
	}
	if !bytes.Contains(wBannedLogin.Body.Bytes(), []byte("用户名或密码错误")) {
		t.Fatalf("banned login error message leaked: %s", wBannedLogin.Body.String())
	}

	// 8. 解封用户
	if err := st.UpdateUserStatus(author.ID, store.UserStatusActive); err != nil {
		t.Fatalf("unban author: %v", err)
	}

	// 9. 解封后旧 cookie1 依然返回 401（无法复用历史凭据）
	wOldCookieAfterUnban := do2FA(r, http.MethodGet, "/api/v1/profile", "", cookie1)
	if wOldCookieAfterUnban.Code != http.StatusUnauthorized {
		t.Fatalf("old cookie after unban status = %d (want 401)", wOldCookieAfterUnban.Code)
	}

	// 10. 解封后旧 token1 依然返回 401（已在封禁时被清空）
	wOldTokenAfterUnban := doBearer(r, token1)
	if wOldTokenAfterUnban.Code != http.StatusUnauthorized {
		t.Fatalf("old token after unban status = %d (want 401)", wOldTokenAfterUnban.Code)
	}

	// 11. 重新登录成功并派发新 cookie2
	reloginW := do2FA(r, http.MethodPost, "/api/v1/auth/login", `{"username":"author1","password":"author-pass"}`, "")
	if reloginW.Code != http.StatusOK {
		t.Fatalf("relogin status = %d (want 200)", reloginW.Code)
	}
	var cookie2 string
	for _, ck := range reloginW.Result().Cookies() {
		if ck.Name == authCookieName && ck.Value != "" {
			cookie2 = ck.Value
			break
		}
	}
	if cookie2 == "" {
		t.Fatal("expected new auth cookie")
	}
	wNewProfile := do2FA(r, http.MethodGet, "/api/v1/profile", "", cookie2)
	if wNewProfile.Code != http.StatusOK {
		t.Fatalf("new profile status = %d (want 200)", wNewProfile.Code)
	}
}

// TestBannedUser2FAIsolation 验证带 2FA 的被封禁用户在两步认证中的拦截
func TestBannedUser2FAIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	_ = st.EnsureUsersTable()
	_ = st.EnsureLoginLogsTable()
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "admin-secret")

	passHash, _ := store.HashPassword("totp-pass")
	totpSecret := GenerateSecret()
	user := &store.User{
		Username:     "totp_user",
		PasswordHash: passHash,
		AuthSecret:   store.GenerateAuthSecret(),
		Role:         store.RoleAuthor,
		Status:       store.UserStatusBanned, // 直接封禁
		TOTPSecret:   totpSecret,
	}
	if err := st.CreateUser(user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	r := gin.New()
	_, _ = MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site)}})

	// 第一步：登录直接返回 401 "用户名或密码错误"，不派发 2FA challenge token
	wLogin := do2FA(r, http.MethodPost, "/api/v1/auth/login", `{"username":"totp_user","password":"totp-pass"}`, "")
	if wLogin.Code != http.StatusUnauthorized {
		t.Fatalf("banned 2fa user login status = %d (want 401)", wLogin.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(wLogin.Body.Bytes(), &resp)
	if resp["require_2fa"] == true {
		t.Fatal("banned 2fa user must not receive require_2fa=true")
	}

	// 假定有残留 2FA token，直接打 /api/v1/auth/2fa 也必须 401
	token, _ := generate2FAToken(user.ID, user.AuthSecret)
	code := totpCodeFor(t, totpSecret)
	w2FA := do2FA(r, http.MethodPost, "/api/v1/auth/2fa", fmt.Sprintf(`{"token":%q,"code":%q}`, token, code), "")
	if w2FA.Code != http.StatusUnauthorized {
		t.Fatalf("banned user direct 2fa status = %d (want 401)", w2FA.Code)
	}

	// 封禁前签发的有效挑战在解封后也不能继续使用。
	activeHash, _ := store.HashPassword("active-totp-pass")
	active := &store.User{
		Username:     "active_totp_user",
		PasswordHash: activeHash,
		AuthSecret:   store.GenerateAuthSecret(),
		Role:         store.RoleAuthor,
		Status:       store.UserStatusActive,
		TOTPSecret:   GenerateSecret(),
	}
	if err := st.CreateUser(active); err != nil {
		t.Fatalf("create active totp user: %v", err)
	}
	step1 := do2FA(r, http.MethodPost, "/api/v1/auth/login", `{"username":"active_totp_user","password":"active-totp-pass"}`, "")
	if step1.Code != http.StatusOK {
		t.Fatalf("active 2fa login: status = %d", step1.Code)
	}
	var challenge struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(step1.Body.Bytes(), &challenge); err != nil || challenge.Token == "" {
		t.Fatalf("decode active 2fa challenge: %v", err)
	}
	if err := st.UpdateUserStatus(active.ID, store.UserStatusBanned); err != nil {
		t.Fatalf("ban active 2fa user: %v", err)
	}
	if err := st.UpdateUserStatus(active.ID, store.UserStatusActive); err != nil {
		t.Fatalf("unban active 2fa user: %v", err)
	}
	w2FA = do2FA(r, http.MethodPost, "/api/v1/auth/2fa", fmt.Sprintf(`{"token":%q,"code":%q}`, challenge.Token, totpCodeFor(t, active.TOTPSecret)), "")
	if w2FA.Code != http.StatusUnauthorized {
		t.Fatalf("pre-ban 2fa challenge after unban status = %d (want 401)", w2FA.Code)
	}
}
