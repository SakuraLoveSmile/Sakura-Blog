package seo

import "testing"

func TestCrawlerEngine(t *testing.T) {
	cases := map[string]string{
		"Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com)": "Baidu",
		"Mozilla/5.0 (compatible; Googlebot/2.1)":                          "Google",
		"Mozilla/5.0 (compatible; bingbot/2.0)":                            "Bing",
		"Sogou web spider/4.0":                                             "Sogou",
		"Mozilla/5.0 Bytespider":                                           "Bytespider",
		"Mozilla/5.0 (Windows NT 10.0) Chrome/120":                         "", // 普通浏览器非爬虫
	}
	for ua, want := range cases {
		if got := crawlerEngine(ua); got != want {
			t.Errorf("crawlerEngine(%q) = %q, want %q", ua, got, want)
		}
	}
}

func TestReferrerEngine(t *testing.T) {
	cases := map[string]string{
		"https://www.baidu.com/s?wd=x":  "Baidu",
		"https://www.google.com/search": "Google",
		"https://cn.bing.com/search":    "Bing",
		"https://example.com/blog":      "", // 非搜索来源
		"":                              "",
	}
	for ref, want := range cases {
		if got := referrerEngine(ref); got != want {
			t.Errorf("referrerEngine(%q) = %q, want %q", ref, got, want)
		}
	}
}

func TestRunChecks(t *testing.T) {
	f := pageFacts{
		Title:       "一个长度合适的标题示例",
		Description: "desc",
		Canonical:   "https://x/",
		OGTitle:     "t",
		OGDesc:      "d",
		Viewport:    "width=device-width",
		Lang:        "zh-CN",
		RobotsMeta:  "index,follow",
	}
	checks := runChecks("https://example.com/", f)
	byKey := map[string]bool{}
	for _, c := range checks {
		byKey[c.Key] = c.Passed
	}
	if !byKey["https"] || !byKey["description"] || !byKey["canonical"] || !byKey["indexable"] || !byKey["viewport"] {
		t.Fatalf("expected core checks to pass, got %+v", byKey)
	}
	// noindex 时 indexable 应失败
	f.RobotsMeta = "noindex"
	for _, c := range runChecks("https://example.com/", f) {
		if c.Key == "indexable" && c.Passed {
			t.Fatalf("indexable should fail when robots meta has noindex")
		}
	}
	// http 时 https 检查应失败
	for _, c := range runChecks("http://example.com/", f) {
		if c.Key == "https" && c.Passed {
			t.Fatalf("https check should fail for http URL")
		}
	}
}

func TestAtoiDefault(t *testing.T) {
	if atoiDefault("", 5) != 5 || atoiDefault("abc", 3) != 3 || atoiDefault("12", 1) != 12 {
		t.Fatal("atoiDefault behaves unexpectedly")
	}
}
