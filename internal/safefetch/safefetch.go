// Package safefetch 出站请求 SSRF 防护（安全审计 2026-08-16）。
//
// 背景：linkwatch 友链探测 / friends RSS 聚合的 URL 来自 links 表（登录作者可写、
// GitHub issue 可间接注入），此前使用默认 http.Client——无 scheme/IP 校验、默认跟随
// 重定向，构成内网探测 SSRF（含云元数据 169.254.169.254），且探测结果回显形成
// 端口扫描 oracle。
//
// 防护策略（对齐 api 包 guest AI 端点的既有实现并泛化）：
//   - 拨号时校验：DialContext 内解析并逐 IP 校验后才拨号，校验与拨号无时间差，
//     天然封堵 TOCTOU DNS rebinding（TTL-0 换解析绕过首跳校验的手法）；
//   - 逐跳生效：重定向的每一跳都经同一 DialContext，302 跳内网同样被拒；
//   - IP 黑名单：环回/私网/链路本地/多播/未指定/0.0.0.0-8/CGNAT 100.64/10/NAT64。
package safefetch

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// ErrForbiddenHost 目标主机解析到禁访地址段（私网/环回/链路本地等）。
var ErrForbiddenHost = errors.New("safefetch: target host resolves to a forbidden address")

// ErrInvalidURL indicates a URL that is not an allowed HTTP(S) target.
var ErrInvalidURL = errors.New("safefetch: only http and https URLs are allowed")

// ValidateURL performs the cheap preflight checks that must happen before any
// request is created. DNS/IP checks are repeated by New's DialContext at the
// final dial to close the DNS rebinding window.
func ValidateURL(raw string) (*url.URL, error) {
	u, err := url.ParseRequestURI(strings.TrimSpace(raw))
	if err != nil || u == nil || u.Host == "" {
		return nil, ErrInvalidURL
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, ErrInvalidURL
	}
	if u.User != nil || u.Hostname() == "" {
		return nil, ErrInvalidURL
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && ForbiddenIP(ip) {
		return nil, ErrForbiddenHost
	}
	return u, nil
}

// Options 客户端构造参数。
type Options struct {
	// Timeout 整请求超时（含重定向链）。0 = 15s 默认。
	Timeout time.Duration
	// DialTimeout 单次 TCP 拨号超时。0 = 8s 默认。
	DialTimeout time.Duration
	// TLSHandshakeTimeout TLS 握手超时。0 = 8s 默认。
	TLSHandshakeTimeout time.Duration
	// MaxRedirects 重定向前缀最大跳数。0 = 不跟随重定向（直接返回 30x 响应）。
	// 跟随时每一跳仍经 DialContext 校验。
	MaxRedirects int
	// ResolveIP overrides DNS resolution. It is intended for deterministic tests;
	// production callers should leave it nil.
	ResolveIP func(context.Context, string) ([]net.IP, error)
	// DialContext overrides the final dial after every resolved IP has passed
	// ForbiddenIP. It is intended for deterministic tests; production callers
	// should leave it nil.
	DialContext func(context.Context, string, string) (net.Conn, error)
}

// New 构造 SSRF 加固的 http.Client。Transport 每次拨号前解析目标主机，
// 解析结果含任一禁访 IP 即拒绝（防混合应答攻击：攻击者让域名同时解析出
// 一个公网 IP 与一个内网 IP）。
func New(opts Options) *http.Client {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	dialTimeout := opts.DialTimeout
	if dialTimeout == 0 {
		dialTimeout = 8 * time.Second
	}
	tlsTimeout := opts.TLSHandshakeTimeout
	if tlsTimeout == 0 {
		tlsTimeout = 8 * time.Second
	}
	dialer := &net.Dialer{Timeout: dialTimeout, KeepAlive: 30 * time.Second}
	resolveIP := opts.ResolveIP
	if resolveIP == nil {
		resolveIP = func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip", host)
		}
	}
	dialContext := opts.DialContext
	if dialContext == nil {
		dialContext = dialer.DialContext
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			// 拨号时解析 + 校验（无校验/拨号时间差，封堵 DNS rebinding）
			var ips []net.IP
			if ip := net.ParseIP(host); ip != nil {
				ips = []net.IP{ip}
			} else {
				ips, err = resolveIP(ctx, host)
				if err != nil {
					return nil, err
				}
			}
			for _, ip := range ips {
				if ForbiddenIP(ip) {
					return nil, ErrForbiddenHost
				}
			}
			var lastErr error
			for _, ip := range ips {
				conn, err := dialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			if lastErr == nil {
				lastErr = errors.New("safefetch: no usable address")
			}
			return nil, lastErr
		},
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   tlsTimeout,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		MaxIdleConns:          10,
		IdleConnTimeout:       60 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if opts.MaxRedirects <= 0 {
				return http.ErrUseLastResponse // 不跟随：把 30x 响应原样交给调用方
			}
			if len(via) >= opts.MaxRedirects {
				return errors.New("safefetch: too many redirects")
			}
			return nil
		},
	}
}

// ForbiddenIP 判断 IP 是否落在禁访段：
// 环回 / RFC1918 私网 / 链路本地 / 多播 / 未指定 / 0.0.0.0-8 / 保留地址 /
// CGNAT / NAT64。IPv4-mapped IPv6 由 netip.Unmap 与 To4 共同归一化。
// forbiddenReservedPrefixes 是不应被服务端主动访问的特殊用途/保留网段。
// 除 RFC1918、环回和链路本地外，额外覆盖文档地址、基准测试网段、IPv4
// 保留地址以及 IPv6 文档/协议分配网段，避免把“非私网”误当成“公网可访问”。
var forbiddenReservedPrefixes = []netip.Prefix{
	mustPrefix("100.64.0.0/10"),
	mustPrefix("192.0.0.0/24"),
	mustPrefix("192.0.2.0/24"),
	mustPrefix("192.88.99.0/24"),
	mustPrefix("198.18.0.0/15"),
	mustPrefix("198.51.100.0/24"),
	mustPrefix("203.0.113.0/24"),
	mustPrefix("240.0.0.0/4"),
	mustPrefix("64:ff9b::/96"),
	mustPrefix("100::/64"),
	mustPrefix("2001:0::/32"),
	mustPrefix("2001:2::/48"),
	mustPrefix("2001:10::/28"),
	mustPrefix("2001:20::/28"),
	mustPrefix("2001:db8::/32"),
}

func mustPrefix(raw string) netip.Prefix {
	prefix, err := netip.ParsePrefix(raw)
	if err != nil {
		panic(err)
	}
	return prefix
}

func hasForbiddenReservedPrefix(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	addr = addr.Unmap()
	for _, prefix := range forbiddenReservedPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func ForbiddenIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	switch {
	case ip.IsLoopback(), ip.IsPrivate(), ip.IsLinkLocalUnicast(),
		ip.IsLinkLocalMulticast(), ip.IsMulticast(), ip.IsUnspecified():
		return true
	}
	if hasForbiddenReservedPrefix(ip) {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		// 0.0.0.0/8："本网络上的主机"，部分系统可路由至本机（IsUnspecified 只匹配 .0）
		if ip4[0] == 0 {
			return true
		}
		// CGNAT 运营商级 NAT 段 100.64.0.0/10（IsPrivate 不覆盖）
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return true
		}
		// 受限广播地址 255.255.255.255（IsMulticast 只覆盖 224/4 组播段）
		if ip4[0] == 255 && ip4[1] == 255 && ip4[2] == 255 && ip4[3] == 255 {
			return true
		}
		return false
	}
	// NAT64 well-known 前缀 64:ff9b::/96（其内嵌 IPv4 按对端规则可达私网）：
	// 首组 0x0064、次组 0xff9b，即字节 00 64 ff 9b，随后 2 字节为 0。
	if len(ip) == net.IPv6len &&
		ip[0] == 0x00 && ip[1] == 0x64 && ip[2] == 0xff && ip[3] == 0x9b &&
		ip[4] == 0x00 && ip[5] == 0x00 {
		return true
	}
	return false
}
