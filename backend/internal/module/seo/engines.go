package seo

import "strings"

// crawlerUAMatchers 搜索引擎爬虫 UA 关键词 → 归一化引擎名（小写匹配，顺序敏感：先具体后宽泛）。
var crawlerUAMatchers = []struct {
	kw     string
	engine string
}{
	{"baiduspider", "Baidu"},
	{"bingbot", "Bing"},
	{"msnbot", "Bing"},
	{"googlebot", "Google"},
	{"adsbot-google", "Google"},
	{"mediapartners-google", "Google"},
	{"sogou", "Sogou"},
	{"360spider", "360"},
	{"haosouspider", "360"},
	{"yisouspider", "Shenma"},
	{"yandex", "Yandex"},
	{"bytespider", "Bytespider"},
	{"duckduckbot", "DuckDuckGo"},
	{"applebot", "Apple"},
	{"facebookexternalhit", "Facebook"},
	{"facebot", "Facebook"},
	{"twitterbot", "Twitter"},
	{"semrushbot", "SEMrush"},
	{"ahrefsbot", "Ahrefs"},
}

// referrerDomainMatchers 搜索引擎来源域名片段 → 归一化引擎名。
var referrerDomainMatchers = []struct {
	frag   string
	engine string
}{
	{"baidu.com", "Baidu"},
	{"google.", "Google"},
	{"bing.com", "Bing"},
	{"sogou.com", "Sogou"},
	{"so.com", "360"},
	{"360.cn", "360"},
	{"sm.cn", "Shenma"},
	{"yandex.", "Yandex"},
	{"duckduckgo.com", "DuckDuckGo"},
	{"yahoo.com", "Yahoo"},
	{"ecosia.org", "Ecosia"},
}

// crawlerEngine 识别 UA 属于哪个搜索引擎爬虫；非爬虫返回空串。
func crawlerEngine(ua string) string {
	l := strings.ToLower(ua)
	for _, m := range crawlerUAMatchers {
		if strings.Contains(l, m.kw) {
			return m.engine
		}
	}
	return ""
}

// referrerEngine 识别来源 referrer 属于哪个搜索引擎；非搜索来源返回空串。
func referrerEngine(referrer string) string {
	l := strings.ToLower(referrer)
	for _, m := range referrerDomainMatchers {
		if strings.Contains(l, m.frag) {
			return m.engine
		}
	}
	return ""
}
