// Package friends 友链动态聚合：定时抓取配置中带 rss 的友链 RSS/Atom，
// 缓存每个友链最新文章，供 /friends 页的 ds-friends_and_posts 前端组件异步渲染。
package friends

import (
	"context"
	"encoding/xml"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/safefetch"
	tmpl "blog/internal/settings"
)

const (
	fetchTimeout    = 10 * time.Second
	defaultInterval = time.Hour
)

// Post 单条友链文章（对齐 friends_and_posts.js 的 post.{title,link,published}）。
type Post struct {
	Title     string `json:"title"`
	Link      string `json:"link"`
	Published string `json:"published"`
	Summary   string `json:"summary"` // 摘要（订阅阅读器用）
}

// Label 友链分类标签（对齐 friends_and_posts.js 的 label.{name,color,hue,saturation,lightness}）。
type Label struct {
	Name       string `json:"name"`
	Color      string `json:"color"`
	Hue        int    `json:"hue"`
	Saturation int    `json:"saturation"`
	Lightness  int    `json:"lightness"`
}

// Friend 聚合后的友链条目（对齐 friends_and_posts.js 期望字段）。
type Friend struct {
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	HTMLURL     string  `json:"html_url"`
	AvatarURL   string  `json:"avatar_url"`
	Labels      []Label `json:"labels"`
	Description string  `json:"description"`
	Posts       []Post  `json:"posts"`
	Feed        []Post  `json:"feed"` // 抓取失败但配置了 rss 时置一项，前端显示"RSS 解析失败"

	rss    string    // 配置的 RSS 地址（内部用）
	latest time.Time // 最新文章时间（排序用，不序列化）
}

// Aggregator 友链 RSS 聚合器。
type Aggregator struct {
	// site 站点配置原子指针（Spec 61 阶段 3）
	site   *atomic.Pointer[tmpl.Site]
	client *http.Client

	// onArticles 每个友链抓取到文章后的回调（供落库持久化，由 main.go 装配），可空。
	onArticles func(friendTitle, friendURL string, posts []Post)

	mu        sync.RWMutex
	cache     []Friend
	startOnce sync.Once
	closeOnce sync.Once
	workMu    sync.Mutex
	closed    bool
	stop      chan struct{}
	done      chan struct{}
	work      sync.WaitGroup
}

// New 从站点配置构建聚合器，并用配置初始化缓存（此时 posts 为空，RSS 抓取后填充）。
// 安全审计 2026-08-16：抓取客户端改用 safefetch（SSRF 加固）——rss URL 来自 links
// 表（作者可写/GitHub issue 可间接注入），且抓取内容会经公开端点回显，属非盲 SSRF；
// 允许最多 5 跳重定向（http→https 订阅源跳转常见），每跳均经拨号层校验。
func New(site *atomic.Pointer[tmpl.Site]) *Aggregator {
	a := &Aggregator{
		site:   site,
		client: safefetch.New(safefetch.Options{Timeout: fetchTimeout, MaxRedirects: 5}),
		stop:   make(chan struct{}),
	}
	a.cache = a.buildBase()
	return a
}

// currentInterval 当前生效的刷新周期（pages.friends 优先、回退 friends 段，<=0 用默认 1h；每周期重读以热更生效）。
func (a *Aggregator) currentInterval() time.Duration {
	if m := a.site.Load().FriendsRefreshInterval(); m > 0 {
		return time.Duration(m) * time.Minute
	}
	return defaultInterval
}

// SetOnArticles 注册抓取回调（将文章落库）；在 Start 前调用。
func (a *Aggregator) SetOnArticles(f func(friendTitle, friendURL string, posts []Post)) {
	a.onArticles = f
}

// buildBase 从 site.Links 构建友链基础列表（不含抓取到的文章）。
func (a *Aggregator) buildBase() []Friend {
	var out []Friend
	for _, group := range a.site.Load().Links {
		for _, it := range group.Items {
			f := Friend{
				Title:       it.Name,
				URL:         it.URL,
				HTMLURL:     it.URL,
				AvatarURL:   it.Avatar,
				Description: firstNonEmpty(it.Desc, it.URL),
				Labels:      []Label{},
				Posts:       []Post{},
				Feed:        []Post{},
				rss:         strings.TrimSpace(it.RSS),
			}
			if it.Label != "" {
				h, s, l := hexToHSL(it.Color)
				f.Labels = append(f.Labels, Label{
					Name: it.Label, Color: strings.TrimPrefix(it.Color, "#"),
					Hue: h, Saturation: s, Lightness: l,
				})
			}
			out = append(out, f)
		}
	}
	return out
}

// Start 启动后台聚合：立即异步抓取一次，之后按 currentInterval 周期刷新（配置热更后下周期生效）。
func (a *Aggregator) Start() {
	a.startOnce.Do(func() {
		a.done = make(chan struct{})
		go func() {
			defer close(a.done)
			select {
			case <-a.stop:
				return
			default:
			}
			a.refresh()
			for {
				timer := time.NewTimer(a.currentInterval())
				select {
				case <-a.stop:
					timer.Stop()
					return
				case <-timer.C:
					a.refresh()
				}
			}
		}()
	})
}

// Close stops periodic and explicitly requested refreshes before Store closes.
func (a *Aggregator) Close(ctx context.Context) error {
	if a == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	a.closeOnce.Do(func() {
		a.workMu.Lock()
		a.closed = true
		a.workMu.Unlock()
		close(a.stop)
	})
	if a.done != nil {
		select {
		case <-a.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	finished := make(chan struct{})
	go func() {
		a.work.Wait()
		close(finished)
	}()
	select {
	case <-finished:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Register 挂 /api/friends-aggregated 与 /api/friends 两个 JSON 端点。
func (a *Aggregator) Register(r *gin.Engine) {
	r.GET("/api/friends-aggregated", a.handleAggregated)
	r.GET("/api/friends", a.handleAggregated)
}

// Refresh 立即异步重建友链缓存（供友链在 console 变更后即时反映到 /friends，不阻塞调用方）。
func (a *Aggregator) Refresh() {
	a.workMu.Lock()
	if a.closed {
		a.workMu.Unlock()
		return
	}
	a.work.Add(1)
	a.workMu.Unlock()
	go func() {
		defer a.work.Done()
		a.refresh()
	}()
}

func (a *Aggregator) handleAggregated(c *gin.Context) {
	a.mu.RLock()
	content := make([]Friend, len(a.cache))
	copy(content, a.cache)
	a.mu.RUnlock()
	c.JSON(http.StatusOK, gin.H{"content": content})
}

// checkLinksConcurrency 并发抓取上限（Spec 61 阶段 5：信号量 10，防止大量友链同时拉起抓取打爆连接）。
const checkLinksConcurrency = 10

// refresh 并发抓取所有带 rss 的友链，聚合后按最新文章时间倒序写回缓存。
func (a *Aggregator) refresh() {
	base := a.buildBase()
	var wg sync.WaitGroup
	sem := make(chan struct{}, checkLinksConcurrency)
	for i := range base {
		f := &base[i]
		if f.rss == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{} // 信号量：满 10 个并发时阻塞排队
		go func(fr *Friend) {
			defer wg.Done()
			defer func() { <-sem }()
			posts, err := a.fetchFeed(fr.rss)
			if err != nil {
				log.Printf("friends: fetch %s failed: %v", fr.rss, err)
				fr.Feed = []Post{{Title: "error"}} // 触发前端"RSS 解析失败"
				return
			}
			if len(posts) == 0 {
				fr.Feed = []Post{{Title: "empty"}}
				return
			}
			fr.Posts = posts
			if t, ok := parseTime(posts[0].Published); ok {
				fr.latest = t
			}
			// 落库持久化（订阅阅读器）：保留已读/收藏/稍后状态
			if a.onArticles != nil {
				a.onArticles(fr.Title, fr.URL, posts)
			}
		}(f)
	}
	wg.Wait()

	// 有最新文章的友链排在前，按时间倒序；其余保持原序追加在后。
	sort.SliceStable(base, func(i, j int) bool {
		return base[i].latest.After(base[j].latest)
	})

	a.mu.Lock()
	a.cache = base
	a.mu.Unlock()
	log.Printf("friends: aggregated %d links refreshed", len(base))
}

// fetchFeed 抓取并解析单个 RSS/Atom，返回最新 maxPostsPerFeed 篇（保持源顺序，通常已倒序）。
func (a *Aggregator) fetchFeed(url string) ([]Post, error) {
	resp, err := a.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20)) // 上限 4MB，防超大 feed
	if err != nil {
		return nil, err
	}
	var fx feedXML
	if err := xml.Unmarshal(body, &fx); err != nil {
		return nil, err
	}
	var posts []Post
	// RSS 2.0
	for _, it := range fx.Channel.Items {
		posts = append(posts, Post{
			Title:     strings.TrimSpace(it.Title),
			Link:      strings.TrimSpace(it.Link),
			Published: formatDate(firstNonEmpty(it.PubDate, it.Date)),
			Summary:   summarize(it.Description),
		})
	}
	// Atom
	for _, e := range fx.Entries {
		posts = append(posts, Post{
			Title: strings.TrimSpace(e.Title),
			Link:  atomLink(e.Links),
			// 优先 published（原始发布时间），回退 updated（部分生成器如 Hexo 的 updated 为构建时间，全条目相同）
			Published: formatDate(firstNonEmpty(e.Published, e.Updated)),
			Summary:   summarize(firstNonEmpty(e.Summary, e.Content)),
		})
	}
	if len(posts) > a.maxPostsPerFeed() {
		posts = posts[:a.maxPostsPerFeed()]
	}
	return posts, nil
}

// maxPostsPerFeed 返回每个友链展示的最新文章数上限（pages.friends 优先，回退 friends.posts_per_link，<=0 用默认 20）。
func (a *Aggregator) maxPostsPerFeed() int {
	if n := a.site.Load().FriendsPostsPerLink(); n > 0 {
		return n
	}
	return 20
}

// feedXML 同时兼容 RSS2.0(<rss><channel><item>) 与 Atom(<feed><entry>)。
// encoding/xml 按子元素名匹配，与根元素名无关，故一个结构体可解析两种格式。
type feedXML struct {
	Channel struct {
		Items []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			PubDate     string `xml:"pubDate"`
			Date        string `xml:"date"` // dc:date
			Description string `xml:"description"`
		} `xml:"item"`
	} `xml:"channel"`
	Entries []struct {
		Title string `xml:"title"`
		Links []struct {
			Href string `xml:"href,attr"`
			Rel  string `xml:"rel,attr"`
		} `xml:"link"`
		Updated   string `xml:"updated"`
		Published string `xml:"published"`
		Summary   string `xml:"summary"`
		Content   string `xml:"content"`
	} `xml:"entry"`
}

// summarize 去 HTML 标签并截断为纯文本摘要（订阅阅读器列表用，上限 ~140 rune）。
func summarize(raw string) string {
	if raw == "" {
		return ""
	}
	var b strings.Builder
	inTag := false
	for _, r := range raw {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	text := strings.Join(strings.Fields(b.String()), " ")
	runes := []rune(text)
	if len(runes) > 140 {
		return string(runes[:140]) + "…"
	}
	return text
}

// atomLink 取 rel="alternate" 或首个无 rel 的链接 href。
func atomLink(links []struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}) string {
	for _, l := range links {
		if l.Rel == "alternate" || l.Rel == "" {
			return l.Href
		}
	}
	if len(links) > 0 {
		return links[0].Href
	}
	return ""
}

var dateLayouts = []string{
	time.RFC1123Z, time.RFC1123, time.RFC3339, time.RFC3339Nano,
	"2006-01-02T15:04:05Z07:00", "2006-01-02 15:04:05", "2006-01-02",
	"Mon, 2 Jan 2006 15:04:05 -0700", "Mon, 2 Jan 2006 15:04:05 MST",
}

func parseTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	// formatDate 已把日期规整为 2006-01-02
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// formatDate 把各类日期字符串规整为 2006-01-02；无法解析则原样返回。
func formatDate(s string) string {
	s = strings.TrimSpace(s)
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return s
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// hexToHSL 把 16 进制颜色（可带/不带 #，6 位）转 HSL 整数分量，供前端 label 文字取色。
func hexToHSL(hex string) (h, s, l int) {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(hex) != 6 {
		return 0, 0, 50
	}
	r := float64(hexByte(hex[0:2])) / 255
	g := float64(hexByte(hex[2:4])) / 255
	b := float64(hexByte(hex[4:6])) / 255
	max, min := maxf(r, g, b), minf(r, g, b)
	lf := (max + min) / 2
	var hf, sf float64
	if max == min {
		hf, sf = 0, 0
	} else {
		d := max - min
		if lf > 0.5 {
			sf = d / (2 - max - min)
		} else {
			sf = d / (max + min)
		}
		switch max {
		case r:
			hf = (g - b) / d
			if g < b {
				hf += 6
			}
		case g:
			hf = (b-r)/d + 2
		default:
			hf = (r-g)/d + 4
		}
		hf /= 6
	}
	return int(hf*360 + 0.5), int(sf*100 + 0.5), int(lf*100 + 0.5)
}

func hexByte(s string) int {
	n, _ := strconv.ParseInt(s, 16, 0)
	return int(n)
}

func maxf(a, b, c float64) float64 {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	return m
}

func minf(a, b, c float64) float64 {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
