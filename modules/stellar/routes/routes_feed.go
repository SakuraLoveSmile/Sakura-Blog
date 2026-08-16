package routes

import (
	"encoding/xml"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/feed"
	"blog/internal/og"
	"blog/internal/store"
	"blog/internal/textutil"
)

// RSS 2.0 结构体（encoding/xml 标准库）
type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel Channel  `xml:"channel"`
}

type Channel struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Items       []Item `xml:"item"`
}

type Item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Guid        string `xml:"guid"`
	// 多作者体系（Spec 38 Wave 6）：RSS author 字段
	Author string `xml:"author,omitempty"`
}

// resolveAuthor 查找作者名：post.Author 在 site.Authors 中存在则返回对应名，否则回退站点标题。
func (s *webServer) resolveAuthor(postAuthor string) string {
	if postAuthor != "" {
		if a, ok := s.site.Load().Authors[postAuthor]; ok && a.Name != "" {
			return a.Name
		}
	}
	if s.site.Load().Title != "" {
		return s.site.Load().Title
	}
	return ""
}

// rssItemsFromPosts 文章转 RSS item：密码文章隐藏正文，无摘要截断正文前 200 字节。
// resolveAuthor 将 post.Author 键解析为可读作者名。
func rssItemsFromPosts(posts []*store.Post, siteURL string, resolveAuthor func(string) string) []Item {
	items := make([]Item, 0, len(posts))
	for _, p := range posts {
		desc := p.Excerpt
		if p.IsPasswordProtected() {
			desc = "本文受密码保护"
		} else if desc == "" {
			desc = textutil.StripHTML(p.ContentHTML)
			if len(desc) > 200 {
				desc = desc[:200]
			}
		}
		link := siteURL + "/post/" + p.Slug
		items = append(items, Item{
			Title:       p.Title,
			Link:        link,
			Description: desc,
			PubDate:     p.CreatedAt.Format(time.RFC1123Z),
			Guid:        link,
			Author:      resolveAuthor(p.Author),
		})
	}
	return items
}

// registerFeedRoutes 注册订阅/SEO 域路由：Atom/JSON Feed/robots.txt（feed 包）、/rss.xml、/sitemap.xml、/og/:slug。
func (s *webServer) registerFeedRoutes(r *gin.Engine) {
	// Atom / JSON Feed / 分类标签 RSS / robots.txt
	feed.New(s.st, s.site).Register(r)

	// RSS 2.0 订阅输出：/rss.xml，最新 20 篇 published 文章。
	r.GET("/rss.xml", func(c *gin.Context) {
		posts, err := s.st.ListPublishedPaged(1, 20)
		if err != nil {
			log.Printf("list posts: %v", err)
			s.render500(c)
			return
		}
		siteURL := "http://" + c.Request.Host
		rss := RSS{
			Version: "2.0",
			Channel: Channel{
				Title:       s.site.Load().Title,
				Link:        siteURL,
				Description: s.site.Load().Description,
				Items:       rssItemsFromPosts(posts, siteURL, s.resolveAuthor),
			},
		}
		output, err := xml.MarshalIndent(rss, "", "  ")
		if err != nil {
			log.Printf("marshal rss: %v", err)
			s.render500(c)
			return
		}
		c.Data(http.StatusOK, "application/rss+xml; charset=utf-8", append([]byte(xml.Header), output...))
	})

	// Sitemap 输出：/sitemap.xml，包含首页、所有已发布文章、wiki 已发布文档、静态索引页。
	// Spec 61 阶段 5：生成期缓存（按 baseURL 分 key，写路径置脏后重建）。
	r.GET("/sitemap.xml", func(c *gin.Context) {
		output, err := s.contentCache.sitemapXML(s.baseURL(c))
		if err != nil {
			log.Printf("sitemap: %v", err)
			s.render500(c)
			return
		}
		c.Data(http.StatusOK, "application/xml; charset=utf-8", output)
	})

	// OG 社交分享图（Spec 27）：/og/:slug.png，磁盘缓存 + 写时失效。
	r.GET("/og/:slug", func(c *gin.Context) {
		slug := c.Param("slug")
		slug = strings.TrimSuffix(slug, ".png")
		if slug == "" {
			c.Status(http.StatusNotFound)
			return
		}
		p, err := s.st.GetBySlug(slug)
		if err != nil || p == nil || p.Status != store.StatusPublished {
			c.Status(http.StatusNotFound)
			return
		}
		data, err := s.ogGen.GetOrGenerate(slug, og.PostInfo{
			Title:     p.Title,
			Date:      p.CreatedAt,
			SiteTitle: s.site.Load().Title,
		})
		if err != nil {
			log.Printf("og: generate %s failed: %v", slug, err)
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Header("Cache-Control", "public, max-age=86400")
		c.Data(http.StatusOK, "image/png", data)
	})
}
