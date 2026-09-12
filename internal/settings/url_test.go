package settings

import (
	"testing"
)

func TestNormalizeSiteURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"   ", ""},
		{"https://example.com", "https://example.com"},
		{"https://example.com/", "https://example.com"},
		{"https://example.com///", "https://example.com"},
		{"https://example.com/blog", "https://example.com/blog"},
		{"https://example.com/blog/", "https://example.com/blog"},
		{"https://example.com/blog/?redirect=/", "https://example.com/blog?redirect=/"},
		{"https://example.com/blog?redirect=/", "https://example.com/blog?redirect=/"},
		{"http://localhost:8080/", "http://localhost:8080"},
	}

	for _, tt := range tests {
		if got := NormalizeSiteURL(tt.input); got != tt.want {
			t.Errorf("NormalizeSiteURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestValidateSiteURL(t *testing.T) {
	valid := []string{
		"",
		"   ",
		"http://localhost",
		"http://localhost:8080",
		"https://example.com",
		"https://example.com/",
		"https://example.com/blog",
	}
	for _, raw := range valid {
		if err := ValidateSiteURL(raw); err != nil {
			t.Errorf("ValidateSiteURL(%q) unexpectedly failed: %v", raw, err)
		}
	}

	invalid := []string{
		"not-a-url",
		"ftp://example.com",
		"javascript:alert(1)",
		"://bad",
		"http://",
		"https://",
		"https://user:pass@example.com",
		"https://example.com/?query=1",
		"https://example.com/#fragment",
	}
	for _, raw := range invalid {
		if err := ValidateSiteURL(raw); err == nil {
			t.Errorf("ValidateSiteURL(%q) expected error, got nil", raw)
		}
	}
}

func TestJoinSiteURL(t *testing.T) {
	tests := []struct {
		base string
		path string
		want string
	}{
		{"https://example.com", "/wiki", "https://example.com/wiki"},
		{"https://example.com/", "/wiki", "https://example.com/wiki"},
		{"https://example.com///", "//wiki", "https://example.com/wiki"},
		{"https://example.com", "wiki", "https://example.com/wiki"},
		{"https://example.com/", "/", "https://example.com/"},
		{"https://example.com", "", "https://example.com/"},
		{"https://example.com", "/search?q=foo%20bar&page=1", "https://example.com/search?q=foo%20bar&page=1"},
		{"https://example.com", "/中文/%E2%9C%93?q=foo%2Fbar/", "https://example.com/中文/%E2%9C%93?q=foo%2Fbar/"},
		{"https://example.com/base?old=1#old", "/中文/%E2%9C%93?new=2#new", "https://example.com/base/中文/%E2%9C%93?new=2#new"},
		{"https://example.com/base?old=1#old", "/post", "https://example.com/base/post?old=1#old"},
		{"https://example.com/base?old=1#old", "/post?new=2", "https://example.com/base/post?new=2#old"},
		{"https://example.com/base?old=1#old", "/post#new", "https://example.com/base/post?old=1#new"},
		{"https://example.com/sub", "/wiki/doc-1", "https://example.com/sub/wiki/doc-1"},
		{"", "/wiki", "/wiki"},
		{"", "wiki", "/wiki"},
		{"", "", "/"},
	}

	for _, tt := range tests {
		if got := JoinSiteURL(tt.base, tt.path); got != tt.want {
			t.Errorf("JoinSiteURL(%q, %q) = %q, want %q", tt.base, tt.path, got, tt.want)
		}
	}
}
