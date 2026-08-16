package api

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	tmpl "blog/internal/template"
)

func TestLoginRateLimitTrigger(t *testing.T) {
	h := New(Config{})
	ip := "1.2.3.4"
	for i := 0; i < loginMaxFails-1; i++ {
		h.recordLoginFail(ip)
		if h.loginBlocked(ip) {
			t.Fatalf("blocked too early at %d fails", i+1)
		}
	}
	h.recordLoginFail(ip) // 达到阈值
	if !h.loginBlocked(ip) {
		t.Fatal("should be blocked at threshold")
	}
}

func TestLoginRateLimitClearOnSuccess(t *testing.T) {
	h := New(Config{})
	ip := "1.2.3.4"
	for i := 0; i < loginMaxFails; i++ {
		h.recordLoginFail(ip)
	}
	if !h.loginBlocked(ip) {
		t.Fatal("precondition: should be blocked")
	}
	h.clearLoginFail(ip)
	if h.loginBlocked(ip) {
		t.Fatal("successful login should clear the block")
	}
}

func TestLoginRateLimitBlockExpiry(t *testing.T) {
	h := New(Config{})
	ip := "1.2.3.4"
	for i := 0; i < loginMaxFails; i++ {
		h.recordLoginFail(ip)
	}
	// 把最后一次失败时间调到封锁期之外，模拟时间流逝。
	h.loginMu.Lock()
	v := h.loginFails[ip]
	v.t = time.Now().Add(-loginBlockTime - time.Second)
	h.loginFails[ip] = v
	h.loginMu.Unlock()
	if h.loginBlocked(ip) {
		t.Fatal("block should expire after loginBlockTime")
	}
}

func TestLoginFailWindowReset(t *testing.T) {
	h := New(Config{})
	ip := "1.2.3.4"
	// 窗口（loginRateLimit）之前的失败不计数：塞入一条过期记录。
	h.loginMu.Lock()
	h.loginFails[ip] = struct {
		n int
		t time.Time
	}{n: loginMaxFails - 1, t: time.Now().Add(-loginRateLimit - time.Second)}
	h.loginMu.Unlock()

	h.recordLoginFail(ip)

	h.loginMu.Lock()
	n := h.loginFails[ip].n
	h.loginMu.Unlock()
	if n != 1 {
		t.Fatalf("expired window should reset counter to 1, got %d", n)
	}
	if h.loginBlocked(ip) {
		t.Fatal("stale failures outside window must not trigger block")
	}
}

func TestCookieSecure(t *testing.T) {
	// 构造带指定 TLS/X-Forwarded-Proto 状态的 gin.Context。
	ctxOf := func(useTLS bool, xfp string) *gin.Context {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		if useTLS {
			req.TLS = &tls.ConnectionState{}
		}
		if xfp != "" {
			req.Header.Set("X-Forwarded-Proto", xfp)
		}
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = req
		return c
	}

	cases := []struct {
		name    string
		siteURL string
		ctx     *gin.Context
		want    bool
	}{
		{"https站点+TLS直连", "https://blog.example.com", ctxOf(true, ""), true},
		{"https站点+反代XFP", "https://blog.example.com", ctxOf(false, "https"), true},
		{"https站点+http IP调试", "https://blog.example.com", ctxOf(false, ""), false}, // 核心回归：http 调试不签 Secure
		{"https站点+XFP http", "https://blog.example.com", ctxOf(false, "http"), false},
		{"http站点+TLS", "http://blog.example.com", ctxOf(true, ""), false},
		{"空站点+TLS", "", ctxOf(true, ""), false},
	}
	for _, tc := range cases {
		h := New(Config{Site: sitePtr(&tmpl.Site{URL: tc.siteURL})})
		if got := h.cookieSecure(tc.ctx); got != tc.want {
			t.Fatalf("%s: cookieSecure = %v, want %v", tc.name, got, tc.want)
		}
	}
}
