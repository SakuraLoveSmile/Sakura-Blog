package settings

import (
	"fmt"
	"net/url"
	"strings"
)

// NormalizeSiteURL cleans a site base URL:
// - Trims leading and trailing whitespace
// - Removes trailing slashes so base URLs never end with "/"
// - Preserves scheme (http/https), host, port, path
func NormalizeSiteURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// 只裁剪路径部分；查询或片段中的斜线属于数据，不能改变。
	cut := len(raw)
	if i := strings.IndexAny(raw, "?#"); i >= 0 {
		cut = i
	}
	return strings.TrimRight(raw[:cut], "/") + raw[cut:]
}

// ValidateSiteURL validates a newly configured site URL.
// Allows empty string (unconfigured). If non-empty, requires http or https scheme and a non-empty host.
func ValidateSiteURL(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https")
	}
	if u.Host == "" || u.Hostname() == "" {
		return fmt.Errorf("URL host cannot be empty")
	}
	if strings.ContainsAny(u.Host, " \t\r\n") {
		return fmt.Errorf("URL host contains whitespace")
	}
	if u.User != nil {
		return fmt.Errorf("URL userinfo is not supported")
	}
	if u.RawQuery != "" || u.ForceQuery {
		return fmt.Errorf("URL query is not supported")
	}
	if strings.Contains(trimmed, "#") {
		return fmt.Errorf("URL fragment is not supported")
	}
	return nil
}

// JoinSiteURL joins a normalized base URL and a request path (which may include query params or fragment).
// It ensures that no double slashes (e.g. "//wiki") occur at the boundary between base and path,
// while preserving path slashes, query parameters, URL encoding, and protocols.
func JoinSiteURL(base, requestPath string) string {
	base = NormalizeSiteURL(base)
	requestPath = strings.TrimSpace(requestPath)

	// Clean any multi-slash prefix from requestPath (e.g. "//wiki" -> "/wiki")
	for strings.HasPrefix(requestPath, "//") {
		requestPath = strings.TrimPrefix(requestPath, "/")
	}

	if base == "" {
		if requestPath == "" {
			return "/"
		}
		if !strings.HasPrefix(requestPath, "/") {
			return "/" + requestPath
		}
		return requestPath
	}

	basePath, baseQuery, baseHasQuery, baseFragment, baseHasFragment := splitURLSuffix(base)
	requestPath, requestQuery, requestHasQuery, requestFragment, requestHasFragment := splitURLSuffix(requestPath)
	if requestPath == "" || requestPath == "/" {
		requestPath = "/"
	}

	if !strings.HasPrefix(requestPath, "/") {
		requestPath = "/" + requestPath
	}

	query := baseQuery
	if requestHasQuery {
		query, baseHasQuery = requestQuery, true
	}
	fragment := baseFragment
	if requestHasFragment {
		fragment, baseHasFragment = requestFragment, true
	}
	result := basePath + requestPath
	if baseHasQuery {
		result += "?" + query
	}
	if baseHasFragment {
		result += "#" + fragment
	}
	return result
}

// splitURLSuffix 按原始字符串拆分，避免重新编码已经编码的路径。
func splitURLSuffix(raw string) (path, query string, hasQuery bool, fragment string, hasFragment bool) {
	fragmentAt := strings.IndexByte(raw, '#')
	beforeFragment := raw
	if fragmentAt >= 0 {
		beforeFragment = raw[:fragmentAt]
		fragment, hasFragment = raw[fragmentAt+1:], true
	}
	queryAt := strings.IndexByte(beforeFragment, '?')
	if queryAt >= 0 {
		return beforeFragment[:queryAt], beforeFragment[queryAt+1:], true, fragment, hasFragment
	}
	return beforeFragment, "", false, fragment, hasFragment
}
