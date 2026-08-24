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
