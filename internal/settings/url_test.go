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
