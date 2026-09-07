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
	return strings.TrimRight(raw, "/")
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
	if u.Host == "" {
		return fmt.Errorf("URL host cannot be empty")
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

	if requestPath == "" || requestPath == "/" {
		return base + "/"
	}

	if !strings.HasPrefix(requestPath, "/") {
		requestPath = "/" + requestPath
	}

	return base + requestPath
}
