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
	"time"

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
	if err := st.EnsureSchema(); err != nil {
		t.Fatalf("ensure schema: %v", err)
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

func publicTestInvite(t *testing.T, st *store.Store) string {
	t.Helper()
	admin, err := st.GetUserByUsername("invite-admin")
	if err != nil {
		t.Fatal(err)
	}
	if admin == nil {
		admin = &store.User{Username: "invite-admin", PasswordHash: "unused", Role: store.RoleAdmin}
		if err := st.CreateUser(admin); err != nil {
			t.Fatal(err)
		}
	}
	code, err := st.CreateRegistrationInvite(admin.ID, time.Now().Add(7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func TestPublicRegistrationInviteAndReaderUpgrade(t *testing.T) {
	r, st := newPublicTestRouter(t)
	code := publicTestInvite(t, st)
	register := publicRequest(r, http.MethodPost, "/api/auth/register", map[string]string{
		"username": "reader-uc", "password": "reader-pass", "display_name": "UC Reader",
		"email": "reader-uc@example.com", "invitation_code": code,
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

func TestPublicRegistrationInviteIsOneTime(t *testing.T) {
	r, st := newPublicTestRouter(t)
	code := publicTestInvite(t, st)
	body := map[string]string{"username": "reader-once", "password": "reader-pass", "email": "once@example.com", "invitation_code": "wrong"}
	first := publicRequest(r, http.MethodPost, "/api/auth/register", body, nil)
	if first.Code != http.StatusBadRequest {
		t.Fatalf("wrong invite status = %d", first.Code)
	}
	body["invitation_code"] = code
	created := publicRequest(r, http.MethodPost, "/api/auth/register", body, nil)
	if created.Code != http.StatusCreated {
		t.Fatalf("register = %d body=%s", created.Code, created.Body.String())
	}
	body["username"] = "reader-reused"
	reused := publicRequest(r, http.MethodPost, "/api/auth/register", body, nil)
	if reused.Code != http.StatusBadRequest {
		t.Fatalf("reused invite status = %d", reused.Code)
	}
	if user, err := st.GetUserByUsername("reader-reused"); err != nil || user != nil {
		t.Fatalf("invalid invite created user: %v / %v", user, err)
	}
	me := publicRequest(r, http.MethodGet, "/api/auth/me", nil, created.Result().Cookies()[0])
	var reader publicUserResponse
	if err := json.Unmarshal(me.Body.Bytes(), &reader); err != nil || !reader.Verified || reader.Role != store.RoleReader || reader.IsAdmin {
		t.Fatalf("me = %+v err=%v", reader, err)
	}
	login := publicRequest(r, http.MethodPost, "/api/auth/login", map[string]string{"username": "reader-once", "password": "reader-pass"}, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("reader login = %d %s", login.Code, login.Body.String())
	}
	loggedIn := publicRequest(r, http.MethodGet, "/api/auth/me", nil, login.Result().Cookies()[0])
	if err := json.Unmarshal(loggedIn.Body.Bytes(), &reader); err != nil || !reader.Verified || reader.Role != store.RoleReader {
		t.Fatalf("logged-in me = %+v err=%v", reader, err)
	}
}

func TestPublicRegistrationRequiresInviteNotCaptcha(t *testing.T) {
	r, _ := newPublicTestRouter(t)
	removed := publicRequest(r, http.MethodGet, "/api/auth/register/challenge", nil, nil)
	if removed.Code != http.StatusNotFound {
		t.Fatalf("removed challenge = %d", removed.Code)
	}
	response := publicRequest(r, http.MethodPost, "/api/auth/register", map[string]string{"username": "old-captcha", "password": "reader-pass", "email": "captcha@example.com", "captcha_token": "token", "captcha_answer": "2"}, nil)
	if response.Code != http.StatusBadRequest || !bytes.Contains(response.Body.Bytes(), []byte("邀请码")) {
		t.Fatalf("captcha-only registration = %d %s", response.Code, response.Body.String())
	}
}

func TestPublicRegistrationValidationDoesNotConsumeInvite(t *testing.T) {
	r, st := newPublicTestRouter(t)
	code := publicTestInvite(t, st)
	body := map[string]string{"username": "reader-valid", "password": "bad", "email": "valid@example.com", "invitation_code": code}
	invalid := publicRequest(r, http.MethodPost, "/api/auth/register", body, nil)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid password = %d", invalid.Code)
	}
	body["password"] = "reader-pass"
	created := publicRequest(r, http.MethodPost, "/api/auth/register", body, nil)
	if created.Code != http.StatusCreated {
		t.Fatalf("valid retry = %d %s", created.Code, created.Body.String())
	}
}

func TestPublicRegistrationRateLimit(t *testing.T) {
	r, st := newPublicTestRouter(t)
	for i := 0; i < 6; i++ {
		response := publicRequest(r, http.MethodPost, "/api/auth/register", map[string]string{
			"username": fmt.Sprintf("rate-%d", i), "password": "reader-pass", "email": fmt.Sprintf("rate-%d@example.com", i),
			"invitation_code": publicTestInvite(t, st),
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
