// Package feed 生成 Atom、JSON Feed、分类/标签 RSS 与 robots.txt。
package feed

import (
	"encoding/xml"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	tmpl "blog/internal/settings"
	"blog/internal/store"
)

const (
	feedPageSize   = 20
	feedCacheAge   = 600
	robotsCacheAge = 86400
)

// Handler 订阅 feed 处理器，持 store 与站点配置。
type Handler struct {
	store *store.Store
	// site 站点配置原子指针（Spec 61 阶段 3）
	site *atomic.Pointer[tmpl.Site]
}

// New 创建 feed handler。
func New(st *store.Store, site *atomic.Pointer[tmpl.Site]) *Handler {
	return &Handler{store: st, site: site}
}

// Register 注册 feed 相关路由。
func (h *Handler) Register(r *gin.Engine) {
	r.GET("/atom.xml", h.atom)
	r.GET("/feed.json", h.jsonFeed)
	r.GET("/categories/:name/rss.xml", h.categoryRSS)
	r.GET("/tags/:name/rss.xml", h.tagRSS)
	r.GET("/robots.txt", h.robots)
}

func (h *Handler) baseURL(c *gin.Context) string {
	if h.site.Load().URL != "" {
		return strings.TrimRight(h.site.Load().URL, "/")
	}
	return "http://" + c.Request.Host
}

func (h *Handler) setFeedCache(c *gin.Context) {
	c.Header("Cache-Control", "max-age="+strconv.Itoa(feedCacheAge))
}

func (h *Handler) postURL(base, slug string) string {
	return base + "/post/" + slug
}

func feedBody(p *store.Post) string {
	if p.IsPasswordProtected() {
		return "<p>本文受密码保护</p>"
	}
	return p.ContentHTML
}

// resolveAuthor 查找作者名：post.Author 在 site.Authors 中存在则返回对应名，否则回退站点标题。
func (h *Handler) resolveAuthor(postAuthor string) string {
	if postAuthor != "" {
		if a, ok := h.site.Load().Authors[postAuthor]; ok && a.Name != "" {
			return a.Name
		}
	}
	if h.site.Load().Title != "" {
		return h.site.Load().Title
	}
	return ""
}

// atom GET /atom.xml — Atom 1.0 全站订阅，最新 20 篇 published 全文。
func (h *Handler) atom(c *gin.Context) {
	posts, err := h.store.ListPublishedPaged(1, feedPageSize)
	if err != nil {
		c.String(http.StatusInternalServerError, "list posts: %v", err)
		return
	}

	base := h.baseURL(c)
	feedURL := base + "/atom.xml"
	updated := time.Now().UTC().Format(time.RFC3339)
	if len(posts) > 0 {
		t := posts[0].UpdatedAt
		if t.IsZero() {
			t = posts[0].CreatedAt
		}
		updated = t.UTC().Format(time.RFC3339)
	}

	entries := make([]atomEntry, 0, len(posts))
	for _, p := range posts {
		link := h.postURL(base, p.Slug)
		pub := p.CreatedAt.UTC().Format(time.RFC3339)
		upd := p.UpdatedAt.UTC().Format(time.RFC3339)
		if p.UpdatedAt.IsZero() {
			upd = pub
		}
		authorName := h.resolveAuthor(p.Author)
		entries = append(entries, atomEntry{
			Title:     p.Title,
			Link:      atomLink{Href: link},
			ID:        link,
			Updated:   upd,
			Published: pub,
			Content:   atomContent{Type: "html", Body: feedBody(p)},
			Author:    &atomAuthor{Name: authorName},
		})
	}

	feed := atomFeed{
		XMLNS: "http://www.w3.org/2005/Atom",
		Title: h.site.Load().Title,
		Link: []atomLink{
			{Href: base + "/"},
			{Href: feedURL, Rel: "self"},
		},
		ID:      base + "/",
		Updated: updated,
		Entries: entries,
	}

	output, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		c.String(http.StatusInternalServerError, "marshal atom: %v", err)
		return
	}
	h.setFeedCache(c)
	c.Data(http.StatusOK, "application/atom+xml; charset=utf-8", append([]byte(xml.Header), output...))
}

// jsonFeed GET /feed.json — JSON Feed 1.1 全站订阅。
func (h *Handler) jsonFeed(c *gin.Context) {
	posts, err := h.store.ListPublishedPaged(1, feedPageSize)
	if err != nil {
		c.String(http.StatusInternalServerError, "list posts: %v", err)
		return
	}

	base := h.baseURL(c)
	items := make([]jsonFeedItem, 0, len(posts))
	for _, p := range posts {
		link := h.postURL(base, p.Slug)
		items = append(items, jsonFeedItem{
			ID:            link,
			URL:           link,
			Title:         p.Title,
			DatePublished: p.CreatedAt.UTC().Format(time.RFC3339),
			ContentHTML:   feedBody(p),
			Author:        h.resolveAuthor(p.Author),
		})
	}

	feed := jsonFeed{
		Version:     "https://jsonfeed.org/version/1.1",
		Title:       h.site.Load().Title,
		HomePageURL: base + "/",
		FeedURL:     base + "/feed.json",
		Description: h.site.Load().Description,
		Items:       items,
	}

	h.setFeedCache(c)
	c.JSON(http.StatusOK, feed)
}

// categoryRSS GET /categories/:name/rss.xml — 分类维度 RSS 2.0。
func (h *Handler) categoryRSS(c *gin.Context) {
	name := c.Param("name")
	posts, err := h.listCategoryPosts(name)
	if err != nil {
		c.String(http.StatusInternalServerError, "list posts: %v", err)
		return
	}
	base := h.baseURL(c)
	h.writeRSS(c, h.site.Load().Title+" — 分类："+name, base+"/categories/"+name, posts)
}

// tagRSS GET /tags/:name/rss.xml — 标签维度 RSS 2.0。
func (h *Handler) tagRSS(c *gin.Context) {
	name := c.Param("name")
	posts, err := h.listTagPosts(name)
	if err != nil {
		c.String(http.StatusInternalServerError, "list posts: %v", err)
		return
	}
	base := h.baseURL(c)
	h.writeRSS(c, h.site.Load().Title+" — 标签："+name, base+"/tags/"+name, posts)
}

func (h *Handler) listCategoryPosts(name string) ([]*store.Post, error) {
	counts, err := h.store.CountByCategory()
	if err != nil {
		return nil, err
	}
	if _, ok := counts[name]; !ok {
		return nil, nil
	}
	return h.store.ListByCategory(name)
}

func (h *Handler) listTagPosts(name string) ([]*store.Post, error) {
	counts, err := h.store.CountByTag()
	if err != nil {
		return nil, err
	}
	if _, ok := counts[name]; !ok {
		return nil, nil
	}
	return h.store.ListByTag(name)
}

func (h *Handler) writeRSS(c *gin.Context, title, channelLink string, posts []*store.Post) {
	base := h.baseURL(c)
	items := make([]rssItem, 0, len(posts))
	for _, p := range posts {
		link := h.postURL(base, p.Slug)
		items = append(items, rssItem{
			Title:       p.Title,
			Link:        link,
			Description: feedBody(p),
			PubDate:     p.CreatedAt.Format(time.RFC1123Z),
			Guid:        link,
			Author:      h.resolveAuthor(p.Author),
		})
	}
	rss := rssFeed{
		Version: "2.0",
		Channel: rssChannel{
			Title:       title,
			Link:        channelLink,
			Description: h.site.Load().Description,
			Items:       items,
		},
	}
	output, err := xml.MarshalIndent(rss, "", "  ")
	if err != nil {
		c.String(http.StatusInternalServerError, "marshal rss: %v", err)
		return
	}
	h.setFeedCache(c)
	c.Data(http.StatusOK, "application/rss+xml; charset=utf-8", append([]byte(xml.Header), output...))
}

// robots GET /robots.txt
func (h *Handler) robots(c *gin.Context) {
	base := h.baseURL(c)
	lines := []string{
		"User-agent: *",
		"Allow: /",
		"Disallow: /console",
		"Sitemap: " + base + "/sitemap.xml",
	}
	// Spec 42：站点级 SEO 配置的 robots 附加规则，多行原样追加（热更新：h.site.Load() 指针字段即最新值）
	if extra := strings.TrimSpace(h.site.Load().SEO.RobotsExtra); extra != "" {
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
