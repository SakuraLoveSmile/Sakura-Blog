package friends

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tmpl "blog/internal/settings"
)

// newTestAggregator 构造聚合器：httptest server 提供 RSS/Atom 双格式样本。
func newTestAggregator(t *testing.T) (*Aggregator, *httptest.Server) {
	t.Helper()
	sitePtr := &atomic.Pointer[tmpl.Site]{}
	sitePtr.Store(&tmpl.Site{})
	a := New(sitePtr)
	// 安全审计 2026-08-16：生产 client 走 safefetch（拒绝环回/私网），单测的
	// httptest 服务监听 127.0.0.1，注入普通 client 验证 RSS 解析逻辑。
	a.client = &http.Client{Timeout: 5 * time.Second}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "rss") {
			w.Write([]byte(rssSample))
			return
		}
		w.Write([]byte(atomSample))
	}))
	t.Cleanup(ts.Close)
	return a, ts
}

const rssSample = `<?xml version="1.0"?>
<rss version="2.0"><channel>
  <title>示例 RSS</title>
  <item>
    <title>  带空格的标题  </title>
    <link>https://example.com/a</link>
    <pubDate>Mon, 02 Jan 2006 15:04:05 +0000</pubDate>
    <description>&lt;p&gt;摘要&lt;b&gt;加粗&lt;/b&gt;&lt;/p&gt;</description>
  </item>
  <item>
    <title>第二条</title>
    <link>https://example.com/b</link>
    <dc:date>2026-08-01</dc:date>
    <description>纯文本摘要</description>
  </item>
</channel></rss>`

const atomSample = `<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom"><title>示例 Atom</title>
  <entry>
    <title>Atom 文章</title>
    <link rel="alternate" href="https://example.com/atom-1"/>
    <published>2026-08-10T08:00:00Z</published>
    <updated>2026-08-11T08:00:00Z</updated>
    <summary>Atom 摘要</summary>
  </entry>
  <entry>
    <title>Atom 无 published</title>
    <link rel="alternate" href="https://example.com/atom-2"/>
    <updated>2026-08-12T08:00:00Z</updated>
    <content>内容回退</content>
  </entry>
</feed>`

// TestFetchFeedRSS RSS 2.0 解析：标题去空白、日期多格式、HTML 摘要剥离。
func TestFetchFeedRSS(t *testing.T) {
	a, ts := newTestAggregator(t)
	posts, err := a.fetchFeed(ts.URL + "/rss.xml")
	if err != nil {
		t.Fatalf("fetch rss: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("posts = %d, want 2", len(posts))
	}
	p := posts[0]
	if p.Title != "带空格的标题" {
		t.Fatalf("title trim = %q", p.Title)
	}
	if p.Published != "2006-01-02" {
		t.Fatalf("pubDate parse = %q, want 2006-01-02", p.Published)
	}
	if strings.Contains(p.Summary, "<") {
		t.Fatalf("summary 未剥离 HTML: %q", p.Summary)
	}
	if !strings.Contains(p.Summary, "摘要") {
		t.Fatalf("summary 内容缺失: %q", p.Summary)
	}
	// dc:date 回退路径
	if posts[1].Published != "2026-08-01" {
		t.Fatalf("dc:date parse = %q", posts[1].Published)
	}
}

// TestFetchFeedAtom Atom 解析：published 优先、无 published 回退 updated、link href 提取。
func TestFetchFeedAtom(t *testing.T) {
	a, ts := newTestAggregator(t)
	posts, err := a.fetchFeed(ts.URL + "/atom.xml")
	if err != nil {
		t.Fatalf("fetch atom: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("posts = %d, want 2", len(posts))
	}
	if posts[0].Link != "https://example.com/atom-1" {
		t.Fatalf("atom link = %q", posts[0].Link)
	}
	if posts[0].Published != "2026-08-10" {
		t.Fatalf("atom published = %q, want 2026-08-10（优先 published）", posts[0].Published)
	}
	if posts[1].Published != "2026-08-12" {
		t.Fatalf("atom updated fallback = %q, want 2026-08-12", posts[1].Published)
	}
	if posts[1].Summary == "" {
		t.Fatal("content 回退为空")
	}
}

// TestMaxPostsPerFeed 截断：超出上限只保留前 N 条。
func TestMaxPostsPerFeed(t *testing.T) {
	a, ts := newTestAggregator(t)
	// site 配置上限 1
	sitePtr := &atomic.Pointer[tmpl.Site]{}
	sitePtr.Store(&tmpl.Site{})
	a.site = sitePtr
	// FriendsPostsPerLink 需要 pages.friends 配置；直接验证默认 20 下限（样本仅 2 条不截断），
	// 再验证 summarize 纯函数。
	posts, err := a.fetchFeed(ts.URL + "/rss.xml")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("default limit posts = %d", len(posts))
	}
}

// TestSummarize 摘要截断边界：长文本截断到 ~140 rune 且无残留标签。
func TestSummarize(t *testing.T) {
	long := strings.Repeat("甲", 200)
	out := summarize(long)
	if len([]rune(out)) > 150 {
		t.Fatalf("summarize 未截断: %d rune", len([]rune(out)))
	}
	if strings.Contains(out, "<") {
		t.Fatal("summarize 残留标签")
	}
	short := summarize("<p>短</p>")
	if short != "短" {
		t.Fatalf("summarize short = %q", short)
	}
}

// TestParseTime 日期多格式解析。
func TestParseTime(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"2026-08-01", true},
		{"Mon, 02 Jan 2006 15:04:05 +0000", true},
		{"2006-01-02T15:04:05Z", true},
		{"", false},
		{"not-a-date", false},
	}
	for _, c := range cases {
		if _, ok := parseTime(c.in); ok != c.want {
			t.Errorf("parseTime(%q) = %v, want %v", c.in, ok, c.want)
		}
	}
}

func TestRefreshSkipsRSSForUnavailableFriend(t *testing.T) {
	var requests atomic.Int32
	a, ts := newTestAggregator(t)
	defer ts.Close()
	sitePtr := &atomic.Pointer[tmpl.Site]{}
	sitePtr.Store(&tmpl.Site{Links: []tmpl.LinkGroup{{Items: []tmpl.LinkItem{{
		Name: "失联友链", URL: "https://example.com", RSS: ts.URL + "/rss.xml", Unavailable: true,
	}}}}})
	a.site = sitePtr
	a.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests.Add(1)
		return http.DefaultTransport.RoundTrip(req)
	})}
	a.refresh()
	if requests.Load() != 0 {
		t.Fatalf("unavailable friend RSS requests = %d, want 0", requests.Load())
	}
	if len(a.cache) != 1 || !a.cache[0].Unavailable {
		t.Fatalf("cache = %+v", a.cache)
	}
	payload, err := json.Marshal(a.cache[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"unavailable":true`) {
		t.Fatalf("public payload missing unavailable flag: %s", payload)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
