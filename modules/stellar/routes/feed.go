// FeedHandler owns Atom, JSON Feed, taxonomy RSS, and robots.txt routes.
package routes

import (
	"encoding/xml"
	"net/http"

	"strconv"
	"strings"
	"sync/atomic"

	"github.com/gin-gonic/gin"

	"blog/internal/post"
	tmpl "blog/internal/settings"
	"blog/internal/store"
)

const (
	feedPageSize   = 20
	feedCacheAge   = 600
	robotsCacheAge = 86400
)

// Handler 订阅 feed 处理器，持 store 与站点配置。
type FeedHandler struct {
	store *store.Store
	// site 站点配置原子指针（Spec 61 阶段 3）
	site *atomic.Pointer[tmpl.Site]
}

// New 创建 feed handler。
func NewFeedHandler(st *store.Store, site *atomic.Pointer[tmpl.Site]) *FeedHandler {
	return &FeedHandler{store: st, site: site}
}

// Register 注册 feed 相关路由。
func (h *FeedHandler) Register(r *gin.Engine) {
	r.GET("/atom.xml", h.atom)
	r.GET("/feed.json", h.jsonFeed)
	r.GET("/categories/:name/rss.xml", h.categoryRSS)
	r.GET("/tags/:name/rss.xml", h.tagRSS)
	r.GET("/robots.txt", h.robots)
}

// atom GET /atom.xml — Atom 1.0 全站订阅，最新 20 篇 published 全文。
func (h *FeedHandler) atom(c *gin.Context)        { h.stellarWeb().serveStellarFeed(c, "atom", "", "") }
func (h *FeedHandler) jsonFeed(c *gin.Context)    { h.stellarWeb().serveStellarFeed(c, "json", "", "") }
func (h *FeedHandler) categoryRSS(c *gin.Context) { h.categoryRSSPath(c, c.Param("name")) }
func (h *FeedHandler) categoryRSSPath(c *gin.Context, name string) {
	h.stellarWeb().serveStellarFeed(c, "rss", "categories", name)
}
func (h *FeedHandler) tagRSS(c *gin.Context) {
	h.stellarWeb().serveStellarFeed(c, "rss", "tags", c.Param("name"))
}
func (h *FeedHandler) stellarWeb() *webServer {
	return newWebServer(webDeps{Site: h.site, Store: h.store, Posts: post.NewService("")})
}

// robots GET /robots.txt
func (h *FeedHandler) robots(c *gin.Context) {
	site := h.site.Load()
	base := canonicalBaseURL(site, c)
	lines := []string{
		"User-agent: *",
		"Allow: /",
		"Disallow: /console",
		"Sitemap: " + tmpl.JoinSiteURL(base, "/sitemap.xml"),
	}
	// Spec 42：站点级 SEO 配置的 robots 附加规则，多行原样追加（热更新：h.site.Load() 指针字段即最新值）
	if extra := strings.TrimSpace(site.SEO.RobotsExtra); extra != "" {
		lines = append(lines, "", extra)
	}
	lines = append(lines, "")
	body := strings.Join(lines, "\n")
	c.Header("Cache-Control", "max-age="+strconv.Itoa(robotsCacheAge))
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(body))
}

// --- Atom 1.0 XML 结构 ---

type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	XMLNS   string      `xml:"xmlns,attr"`
	Title   string      `xml:"title"`
	Link    []atomLink  `xml:"link"`
	ID      string      `xml:"id"`
	Updated string      `xml:"updated"`
	Entries []atomEntry `xml:"entry"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr,omitempty"`
}

type atomEntry struct {
	Title     string      `xml:"title"`
	Link      atomLink    `xml:"link"`
	ID        string      `xml:"id"`
	Updated   string      `xml:"updated"`
	Published string      `xml:"published"`
	Content   atomContent `xml:"content"`
	// 多作者体系（Spec 38 Wave 6）：Atom author 子元素
	Author *atomAuthor `xml:"author,omitempty"`
}

type atomAuthor struct {
	Name string `xml:"name,omitempty"`
}

type atomContent struct {
	Type string `xml:"type,attr"`
	Body string `xml:",chardata"`
}

// --- JSON Feed 1.1 ---

type jsonFeed struct {
	Version     string         `json:"version"`
	Title       string         `json:"title"`
	HomePageURL string         `json:"home_page_url"`
	FeedURL     string         `json:"feed_url"`
	Description string         `json:"description,omitempty"`
	Items       []jsonFeedItem `json:"items"`
}

type jsonFeedItem struct {
	ID            string `json:"id"`
	URL           string `json:"url"`
	Title         string `json:"title"`
	DatePublished string `json:"date_published"`
	ContentHTML   string `json:"content_html"`
	// 多作者体系（Spec 38 Wave 6）：JSON Feed author 字段
	Author string `json:"author,omitempty"`
}

// --- RSS 2.0（分类/标签 feed）---

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Guid        string `xml:"guid"`
	// 多作者体系（Spec 38 Wave 6）：RSS author 字段
	Author string `xml:"author,omitempty"`
}
