package api

import (
	"testing"
	"time"

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
	cases := []struct {
		url  string
		want bool
	}{
		{"https://blog.example.com", true},
		{"http://blog.example.com", false},
		{"", false},
		{"http://localhost:8080", false},
	}
	for _, c := range cases {
		h := New(Config{Site: &tmpl.Site{URL: c.url}})
		if got := h.cookieSecure(); got != c.want {
			t.Fatalf("cookieSecure(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}
