package seo

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"

	"blog/internal/safefetch"
)

// defaultAnalyzeClient 页面抓取用 SSRF 加固 client（短超时，最多跟随 5 次
// 重定向；每一跳都会重新解析并校验目标地址）。
var defaultAnalyzeClient = safefetch.New(safefetch.Options{Timeout: 12 * time.Second, MaxRedirects: 5})

// checkItem 单条检查结果。
type checkItem struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

// analyzeResult 页面分析结果。
type analyzeResult struct {
	URL    string      `json:"url"`
	Score  int         `json:"score"`
	Passed []checkItem `json:"passed"`
	Issues []checkItem `json:"issues"`
}

// registerAnalyze 挂页面分析端点。
func (m *Module) registerAnalyze(g *gin.RouterGroup) {
	g.POST("/analyze", m.analyzeHandler)
}

func (m *Module) analyzeHandler(c *gin.Context) {
	var body struct {
		URL string `json:"url"`
	}
	_ = c.ShouldBindJSON(&body)
	target := strings.TrimSpace(body.URL)
	if target == "" && m.deps.Site.Load() != nil {
		target = m.deps.Site.Load().URL
	}
	if target == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供待检查的 URL，或先在站点设置里配置 site.url"})
		return
	}
	targetURL, err := url.Parse(target)
	if err == nil && targetURL.Scheme == "" {
		target = "https://" + target
		targetURL, err = url.Parse(target)
	}
	if err != nil || targetURL.Host == "" || !strings.EqualFold(targetURL.Scheme, "http") && !strings.EqualFold(targetURL.Scheme, "https") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL 无效，仅支持 http 或 https"})
		return
	}
	targetURL.Scheme = strings.ToLower(targetURL.Scheme)
	target = targetURL.String()
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL 无效"})
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; StellarBlog-SEO/1.0)")
	client := m.analyzeClient
	if client == nil {
		client = defaultAnalyzeClient
	}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "无法访问该页面"})
		return
	}
	defer resp.Body.Close()
	doc, err := html.Parse(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "页面解析失败"})
		return
	}

	f := extractFacts(doc)
	checks := runChecks(target, f)

	res := analyzeResult{URL: target, Passed: []checkItem{}, Issues: []checkItem{}}
	for _, ck := range checks {
		if ck.Passed {
			res.Passed = append(res.Passed, ck)
		} else {
			res.Issues = append(res.Issues, ck)
		}
	}
	if len(checks) > 0 {
		res.Score = int(float64(len(res.Passed)) / float64(len(checks)) * 100)
	}
	c.JSON(http.StatusOK, res)
}

// pageFacts 从 HTML 抽取的事实集合。
type pageFacts struct {
	Title       string
	Description string
	Canonical   string
	RobotsMeta  string
	OGTitle     string
	OGDesc      string
	Viewport    string
	Lang        string
	ImgTotal    int
	ImgNoAlt    int
	ImgNoDim    int
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

// extractFacts 遍历 DOM 收集 SEO 相关信息。
func extractFacts(doc *html.Node) pageFacts {
	var f pageFacts
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "html":
				f.Lang = attr(n, "lang")
			case "title":
				if n.FirstChild != nil {
					f.Title = strings.TrimSpace(n.FirstChild.Data)
				}
			case "link":
				if strings.EqualFold(attr(n, "rel"), "canonical") {
					f.Canonical = attr(n, "href")
				}
			case "meta":
				name := strings.ToLower(attr(n, "name"))
				prop := strings.ToLower(attr(n, "property"))
				content := attr(n, "content")
				switch {
				case name == "description":
					f.Description = content
				case name == "robots":
					f.RobotsMeta = strings.ToLower(content)
				case name == "viewport":
					f.Viewport = content
				case prop == "og:title":
					f.OGTitle = content
				case prop == "og:description":
					f.OGDesc = content
				}
			case "img":
				f.ImgTotal++
				if strings.TrimSpace(attr(n, "alt")) == "" {
					f.ImgNoAlt++
				}
				if attr(n, "width") == "" || attr(n, "height") == "" {
					f.ImgNoDim++
				}
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(doc)
	return f
}

// runChecks 依据抽取事实生成检查清单。
func runChecks(target string, f pageFacts) []checkItem {
	titleLen := len([]rune(f.Title))
	return []checkItem{
		{Key: "https", Label: "使用 HTTPS 加密", Passed: strings.HasPrefix(target, "https://")},
		{Key: "title", Label: "title 长度适中（10-60 字）", Passed: titleLen >= 10 && titleLen <= 60,
			Detail: f.Title},
		{Key: "description", Label: "包含 meta description", Passed: strings.TrimSpace(f.Description) != ""},
		{Key: "canonical", Label: "包含 canonical 链接", Passed: strings.TrimSpace(f.Canonical) != ""},
		{Key: "indexable", Label: "页面未被 noindex 阻止索引", Passed: !strings.Contains(f.RobotsMeta, "noindex")},
		{Key: "og_title", Label: "包含 Open Graph 标题", Passed: strings.TrimSpace(f.OGTitle) != ""},
		{Key: "og_desc", Label: "包含 Open Graph 描述", Passed: strings.TrimSpace(f.OGDesc) != ""},
		{Key: "viewport", Label: "包含 viewport 视口声明", Passed: strings.TrimSpace(f.Viewport) != ""},
		{Key: "lang", Label: "html 标签声明了 lang", Passed: strings.TrimSpace(f.Lang) != ""},
		{Key: "img_alt", Label: "所有图片包含 alt 属性", Passed: f.ImgNoAlt == 0},
		{Key: "img_dim", Label: "图片声明了 width/height", Passed: f.ImgNoDim == 0},
	}
}
