package safefetch

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// TestForbiddenIP 安全审计 2026-08-16 回归：SSRF IP 黑名单覆盖面。
func TestForbiddenIP(t *testing.T) {
	forbidden := []string{
		"127.0.0.1", "127.8.8.8", // 环回
		"10.0.0.1", "192.168.1.1", "172.16.0.1", "172.31.255.255", // RFC1918
		"169.254.169.254",    // 链路本地（云元数据）
		"0.0.0.0", "0.1.2.3", // 未指定/0.0.0.0-8
		"100.64.0.1", "100.127.255.255", // CGNAT
		"192.0.2.1", "198.18.0.1", "198.51.100.1", "203.0.113.1", // 文档/基准测试保留段
		"224.0.0.1", "255.255.255.255", // 多播/广播
		"::1", "::", // IPv6 环回/未指定
		"fe80::1",                  // IPv6 链路本地
		"fd00::1",                  // IPv6 ULA
		"2001:db8::1", "2001:2::1", // IPv6 文档/基准测试保留段
		"::ffff:127.0.0.1", "::ffff:10.0.0.1", // IPv4-mapped
		"64:ff9b::127.0.0.1", "64:ff9b::1a2b:3c4d", // NAT64
	}
	allowed := []string{
		"1.1.1.1", "8.8.8.8", "100.128.0.1", // CGNAT 上界外
		"172.32.0.1",      // RFC1918 172.16/12 之外
		"2606:4700::1111", // 公网 IPv6
	}
	for _, s := range forbidden {
		if !ForbiddenIP(net.ParseIP(s)) {
			t.Errorf("ForbiddenIP(%s) = false, want true", s)
		}
	}
	for _, s := range allowed {
		if ForbiddenIP(net.ParseIP(s)) {
			t.Errorf("ForbiddenIP(%s) = true, want false", s)
		}
	}
}

func TestClientRejectsRedirectToForbiddenIP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://private.test:"+serverPort(t, r)+"/secret", http.StatusFound)
	}))
	defer server.Close()
	serverURL, _ := url.Parse(server.URL)
	port := serverURL.Port()
	dialer := &net.Dialer{}
	client := New(Options{
		MaxRedirects: 5,
		ResolveIP: func(_ context.Context, host string) ([]net.IP, error) {
			switch host {
			case "public.test":
				return []net.IP{net.ParseIP("1.1.1.1")}, nil
			case "private.test":
				return []net.IP{net.ParseIP("127.0.0.1")}, nil
			default:
				return nil, fmt.Errorf("unexpected host %s", host)
			}
		},
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, server.Listener.Addr().String())
		},
	})

	resp, err := client.Get("http://public.test:" + port + "/redirect")
	if err == nil {
		resp.Body.Close()
		t.Fatal("redirect to a forbidden address should fail")
	}
}

func serverPort(t *testing.T, r *http.Request) string {
	t.Helper()
	if _, port, err := net.SplitHostPort(r.Host); err == nil {
		return port
	}
	t.Fatalf("request host has no port: %q", r.Host)
	return ""
}
