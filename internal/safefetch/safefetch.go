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
	"time"
)

// ErrForbiddenHost 目标主机解析到禁访地址段（私网/环回/链路本地等）。
var ErrForbiddenHost = errors.New("safefetch: target host resolves to a forbidden address")

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
				ips, err = net.DefaultResolver.LookupIP(ctx, "ip", host)
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
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
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
// 环回 / RFC1918 私网 / 链路本地 / 多播 / 未指定 / 0.0.0.0-8 / CGNAT 100.64.0.0/10 /
// NAT64 64:ff9b::/96。IPv4-mapped IPv6 由 To4 归一化后按 IPv4 规则判定。
func ForbiddenIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	switch {
	case ip.IsLoopback(), ip.IsPrivate(), ip.IsLinkLocalUnicast(),
		ip.IsLinkLocalMulticast(), ip.IsMulticast(), ip.IsUnspecified():
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
