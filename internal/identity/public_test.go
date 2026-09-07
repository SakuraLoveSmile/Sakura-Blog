package identity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"

	tmpl "blog/internal/settings"
	"blog/internal/store"
)

func newPublicTestRouter(t *testing.T) (*gin.Engine, *store.Store) {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := st.EnsureUsersTable(); err != nil {
		t.Fatalf("ensure users: %v", err)
	}
	site := &atomic.Pointer[tmpl.Site]{}
	site.Store(&tmpl.Site{})
	h := NewPublicHandler(st, site)
	r := gin.New()
	r.Use(h.Middleware())
	routes := h.PublicRoutes()
	auth := r.Group("/api/auth")
	auth.GET("/register/challenge", routes.RegisterChallenge)
	auth.POST("/register", routes.Register)
	auth.POST("/login", routes.Login)
	auth.GET("/me", routes.Me)
	auth.POST("/logout", routes.Logout)
	auth.POST("/upgrade", routes.Upgrade)
	t.Cleanup(func() { st.Close() })
	return r, st
}

func publicRequest(r *gin.Engine, method, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestPublicRegistrationCaptchaAndReaderUpgrade(t *testing.T) {
	r, st := newPublicTestRouter(t)
	challengeResponse := publicRequest(r, http.MethodGet, "/api/auth/register/challenge", nil, nil)
	if challengeResponse.Code != http.StatusOK {
		t.Fatalf("challenge status = %d", challengeResponse.Code)
	}
	var challenge registerChallengeResponse
	if err := json.Unmarshal(challengeResponse.Body.Bytes(), &challenge); err != nil {
		t.Fatalf("decode challenge: %v", err)
	}
	var first, second int
	if _, err := fmt.Sscanf(challenge.Question, "%d + %d = ?", &first, &second); err != nil {
		t.Fatalf("parse challenge %q: %v", challenge.Question, err)
	}
	register := publicRequest(r, http.MethodPost, "/api/auth/register", map[string]string{
		"username": "reader-uc", "password": "reader-pass", "display_name": "UC Reader",
		"email": "reader-uc@example.com", "captcha_token": challenge.Token,
		"captcha_answer": fmt.Sprintf("%d", first+second),
	}, nil)
	if register.Code != http.StatusCreated {
		t.Fatalf("register status = %d body=%s", register.Code, register.Body.String())
	}
	cookies := register.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != UserCookieName {
		t.Fatalf("registration did not issue %s cookie: %v", UserCookieName, cookies)
	}
	readerCookie := cookies[0]
	var created publicAuthResponse
	if err := json.Unmarshal(register.Body.Bytes(), &created); err != nil || created.User.Role != store.RoleReader {
		t.Fatalf("registered user = %+v, err=%v", created.User, err)
	}

	upgrade := publicRequest(r, http.MethodPost, "/api/auth/upgrade", map[string]string{"password": "reader-pass"}, readerCookie)
	if upgrade.Code != http.StatusOK {
		t.Fatalf("upgrade status = %d body=%s", upgrade.Code, upgrade.Body.String())
	}
	var upgraded publicAuthResponse
	if err := json.Unmarshal(upgrade.Body.Bytes(), &upgraded); err != nil || upgraded.User.Role != store.RoleAuthor {
		t.Fatalf("upgraded user = %+v, err=%v", upgraded.User, err)
	}
	user, err := st.GetUserByUsername("reader-uc")
	if err != nil || user == nil || user.Role != store.RoleAuthor {
		t.Fatalf("stored role = %+v, err=%v", user, err)
	}

	login := publicRequest(r, http.MethodPost, "/api/auth/login", map[string]string{"username": "reader-uc", "password": "reader-pass"}, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("author public login status = %d body=%s", login.Code, login.Body.String())
	}
}

func TestPublicRegistrationCaptchaIsOneTime(t *testing.T) {
	r, _ := newPublicTestRouter(t)
	challengeResponse := publicRequest(r, http.MethodGet, "/api/auth/register/challenge", nil, nil)
	var challenge registerChallengeResponse
	_ = json.Unmarshal(challengeResponse.Body.Bytes(), &challenge)
	body := map[string]string{
		"username": "reader-once", "password": "reader-pass", "display_name": "Reader", "email": "once@example.com",
		"captcha_token": challenge.Token, "captcha_answer": "0",
	}
	first := publicRequest(r, http.MethodPost, "/api/auth/register", body, nil)
	if first.Code != http.StatusBadRequest {
		t.Fatalf("wrong captcha status = %d", first.Code)
	}
	body["captcha_answer"] = "1"
	second := publicRequest(r, http.MethodPost, "/api/auth/register", body, nil)
	if second.Code != http.StatusBadRequest {
		t.Fatalf("reused captcha status = %d", second.Code)
	}
}

func TestPublicRegistrationRateLimit(t *testing.T) {
	r, _ := newPublicTestRouter(t)
	for i := 0; i < 6; i++ {
		challengeResponse := publicRequest(r, http.MethodGet, "/api/auth/register/challenge", nil, nil)
		var challenge registerChallengeResponse
		if err := json.Unmarshal(challengeResponse.Body.Bytes(), &challenge); err != nil {
			t.Fatalf("decode challenge %d: %v", i, err)
		}
		var first, second int
		if _, err := fmt.Sscanf(challenge.Question, "%d + %d = ?", &first, &second); err != nil {
			t.Fatalf("parse challenge %d: %v", i, err)
		}
		response := publicRequest(r, http.MethodPost, "/api/auth/register", map[string]string{
			"username": fmt.Sprintf("rate-%d", i), "password": "reader-pass", "email": fmt.Sprintf("rate-%d@example.com", i),
			"captcha_token": challenge.Token, "captcha_answer": fmt.Sprintf("%d", first+second),
		}, nil)
		if i < 5 && response.Code != http.StatusCreated {
			t.Fatalf("registration %d status = %d body=%s", i, response.Code, response.Body.String())
		}
		if i == 5 && response.Code != http.StatusTooManyRequests {
			t.Fatalf("registration rate limit status = %d body=%s", response.Code, response.Body.String())
		}
	}
}

func TestBannedReaderAuthIsolation(t *testing.T) {
	r, st := newPublicTestRouter(t)
	hash, err := store.HashPassword("reader-pass")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u := &store.User{
		Username:     "banned-reader",
		PasswordHash: hash,
		DisplayName:  "Banned Reader",
		Email:        "banned@example.com",
		Role:         store.RoleReader,
		AuthSecret:   store.GenerateAuthSecret(),
		Status:       store.UserStatusActive,
	}
	if err := st.CreateUser(u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// 1. Active user can log in successfully
	login := publicRequest(r, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "banned-reader",
		"password": "reader-pass",
	}, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("active login failed: %d, body=%s", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no cookie set")
	}
	activeCookie := cookies[0]

	// 2. Verified me endpoint succeeds
	me := publicRequest(r, http.MethodGet, "/api/auth/me", nil, activeCookie)
	if me.Code != http.StatusOK {
		t.Fatalf("me status = %d", me.Code)
	}
	var meData map[string]any
	_ = json.Unmarshal(me.Body.Bytes(), &meData)
	if meData["verified"] != true {
		t.Fatalf("expected verified = true, got %v", meData["verified"])
	}

	// 3. Ban the user
	if err := st.UpdateUserStatus(u.ID, store.UserStatusBanned); err != nil {
		t.Fatalf("ban user: %v", err)
	}

	// 4. Existing cookie is immediately invalidated
	meAfterBan := publicRequest(r, http.MethodGet, "/api/auth/me", nil, activeCookie)
	var meAfterBanData map[string]any
	_ = json.Unmarshal(meAfterBan.Body.Bytes(), &meAfterBanData)
	if meAfterBanData["verified"] == true {
		t.Fatalf("banned user should not be verified with old cookie")
	}

	// 5. Login returns 401 "用户名或密码错误" without leaking account ban or existence
	bannedLogin := publicRequest(r, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "banned-reader",
		"password": "reader-pass",
	}, nil)
	if bannedLogin.Code != http.StatusUnauthorized {
		t.Fatalf("banned login status = %d (want 401), body=%s", bannedLogin.Code, bannedLogin.Body.String())
	}
	if !bytes.Contains(bannedLogin.Body.Bytes(), []byte("用户名或密码错误")) {
		t.Fatalf("banned login should return generic error, got: %s", bannedLogin.Body.String())
	}

	// 6. Unban user
	if err := st.UpdateUserStatus(u.ID, store.UserStatusActive); err != nil {
		t.Fatalf("unban user: %v", err)
	}

	// 7. Old cookie remains invalid (cannot be reused after unban)
	meAfterUnban := publicRequest(r, http.MethodGet, "/api/auth/me", nil, activeCookie)
	var meAfterUnbanData map[string]any
	_ = json.Unmarshal(meAfterUnban.Body.Bytes(), &meAfterUnbanData)
	if meAfterUnbanData["verified"] == true {
		t.Fatalf("banned user's pre-ban cookie must not be resurrected after unban")
	}

	// 8. Re-login works and issues fresh cookie
	relogin := publicRequest(r, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "banned-reader",
		"password": "reader-pass",
	}, nil)
	if relogin.Code != http.StatusOK {
		t.Fatalf("re-login status = %d, body=%s", relogin.Code, relogin.Body.String())
	}
}
