package safefetch

import (
	"net"
	"testing"
)

// TestForbiddenIP 安全审计 2026-08-16 回归：SSRF IP 黑名单覆盖面。
func TestForbiddenIP(t *testing.T) {
	forbidden := []string{
		"127.0.0.1", "127.8.8.8",             // 环回
		"10.0.0.1", "192.168.1.1", "172.16.0.1", "172.31.255.255", // RFC1918
		"169.254.169.254",                     // 链路本地（云元数据）
		"0.0.0.0", "0.1.2.3",                  // 未指定/0.0.0.0-8
		"100.64.0.1", "100.127.255.255",       // CGNAT
		"224.0.0.1", "255.255.255.255",        // 多播/广播
		"::1", "::",                           // IPv6 环回/未指定
		"fe80::1",                             // IPv6 链路本地
		"fd00::1",                             // IPv6 ULA
		"::ffff:127.0.0.1", "::ffff:10.0.0.1", // IPv4-mapped
		"64:ff9b::127.0.0.1", "64:ff9b::1a2b:3c4d", // NAT64
	}
	allowed := []string{
		"1.1.1.1", "8.8.8.8", "100.128.0.1", // CGNAT 上界外
		"172.32.0.1",                          // RFC1918 172.16/12 之外
		"2606:4700::1111",                     // 公网 IPv6
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
