package web

import "testing"

func TestImagePageBaseUsesCanonicalOriginAndActualPathOnce(t *testing.T) {
	for _, c := range []struct{ site, page, want string }{
		{"https://site.test/sub", "https://mirror.test/sub/post/item", "https://site.test/sub/post/item"},
		{"https://site.test/sub/", "http://mirror.test/sub/post/item?x=1#part", "https://site.test/sub/post/item"},
		{"https://site.test/sub", "https://site.test/sub/", "https://site.test/sub/"},
	} {
		got, err := ImagePageBaseURL(c.site, c.page)
		if err != nil || got != c.want {
			t.Fatalf("base %q/%q=%q %v", c.site, c.page, got, err)
		}
	}
	if _, err := ImagePageBaseURL("/relative", "https://mirror.test/sub"); err == nil {
		t.Fatal("relative site accepted")
	}
}
