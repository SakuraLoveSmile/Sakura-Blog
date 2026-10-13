package appearance

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

// Options 固定动态样式迁入 inline <style> 后的资源上下文。
// RootURL 是站点目录，CSSURL 是实际选中的主样式公开地址，均不取当前文章地址。
// 零值等价上游默认 / 和 /css/main.css；仅提供 RootURL 时 CSSURL 随该目录解析。
type Options struct {
	RootURL string
	CSSURL  string
}

func resourceBases(options Options) (*url.URL, *url.URL, error) {
	root := options.RootURL
	if root == "" {
		root = "/"
	}
	rootURL, e := baseURL(root)
	if e != nil {
		return nil, nil, fmt.Errorf("stellar appearance RootURL: %w", e)
	}
	if rootURL.RawQuery != "" || rootURL.Fragment != "" {
		return nil, nil, fmt.Errorf("stellar appearance RootURL: expected directory URL without query or fragment")
	}
	if rootURL.Scheme == "" && rootURL.Host == "" && !strings.HasPrefix(rootURL.Path, "/") {
		return nil, nil, fmt.Errorf("stellar appearance RootURL: expected absolute or root-relative directory URL")
	}
	if !strings.HasSuffix(rootURL.Path, "/") {
		rootURL.Path += "/"
		rootURL.RawPath = ""
	}
	css := options.CSSURL
	if css == "" {
		css = "css/main.css"
	}
	cssURL, e := baseURL(css)
	if e != nil {
		return nil, nil, fmt.Errorf("stellar appearance CSSURL: %w", e)
	}
	return rootURL, rootURL.ResolveReference(cssURL), nil
}
func baseURL(value string) (*url.URL, error) {
	if value == "" || strings.TrimSpace(value) != value || strings.ContainsAny(value, "'\"\\<>") {
		return nil, fmt.Errorf("expected safe standard HTTP or path URL")
	}
	for _, c := range value {
		if unicode.IsControl(c) {
			return nil, fmt.Errorf("expected safe standard HTTP or path URL")
		}
	}
	u, e := url.Parse(value)
	if e != nil {
		return nil, fmt.Errorf("expected safe standard HTTP or path URL")
	}
	scheme := strings.ToLower(u.Scheme)
	if u.User != nil || scheme != "" && (scheme != "https" && scheme != "http" || u.Host == "") {
		return nil, fmt.Errorf("expected safe standard HTTP or path URL")
	}
	return u, nil
}
func resolvedResource(source string, base *url.URL) string {
	if source == "" {
		return ""
	}
	// 调用前已通过资源边界检查；data URL 保留其原始 opaque 字节。
	if strings.HasPrefix(strings.ToLower(source), "data:image/") {
		return source
	}
	reference, _ := url.Parse(source)
	return base.ResolveReference(reference).String()
}
