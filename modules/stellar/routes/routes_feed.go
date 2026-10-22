package routes

import (
	"encoding/xml"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/og"
	"blog/internal/store"
	"blog/internal/textutil"
	tmpl "blog/modules/stellar/viewmodel"
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
// resolvePostAuthors 将关联作者解析为可读作者名，顺序与文章设置一致。
func resolvePostAuthors(p *store.Post, resolveAuthor func(string) string) string {
	if p == nil {
		return ""
	}
	ids := append([]string(nil), p.Authors...)
	if len(ids) == 0 && p.Author != "" {
		ids = []string{p.Author}
	}
	if len(ids) == 0 {
		return resolveAuthor("")
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if name := resolveAuthor(id); name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, "、")
}

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
		link := tmpl.JoinSiteURL(siteURL, "/post/"+url.PathEscape(p.Slug))
		items = append(items, Item{
			Title:       p.Title,
			Link:        link,
			Description: desc,
			PubDate:     p.CreatedAt.Format(time.RFC1123Z),
			Guid:        link,
			Author:      resolvePostAuthors(p, resolveAuthor),
		})
	}
	return items
}

// registerFeedRoutes 注册订阅/SEO 域路由：Atom/JSON Feed/robots.txt（feed 包）、/rss.xml、/sitemap.xml、/og/:slug。
func (s *webServer) registerFeedRoutes(r *gin.Engine) {
	r.GET("/atom.xml", func(c *gin.Context) { s.serveStellarFeed(c, "atom", "", "") })
	r.GET("/feed.json", func(c *gin.Context) { s.serveStellarFeed(c, "json", "", "") })
	r.GET("/rss.xml", func(c *gin.Context) { s.serveStellarFeed(c, "rss", "", "") })
	r.GET("/categories/:name/rss.xml", func(c *gin.Context) { s.serveStellarFeed(c, "rss", "categories", c.Param("name")) })
	r.GET("/tags/:name/rss.xml", func(c *gin.Context) { s.serveStellarFeed(c, "rss", "tags", c.Param("name")) })
	r.GET("/robots.txt", NewFeedHandler(s.st, s.site).robots)
	r.GET("/sitemap.xml", s.serveStellarSitemap)
	r.GET("/og/:slug", func(c *gin.Context) {
		slug := strings.TrimSuffix(c.Param("slug"), ".png")
		p, err := s.st.GetPublishedBySlug(slug)
		if err != nil {
			stellarError(c, err)
			return
		}
		if p == nil || p.IsPasswordProtected() {
			c.Status(http.StatusNotFound)
			return
		}
		if _, err := p.ValidateStellar(); err != nil {
			stellarError(c, err)
			return
		}
		if s.ogGen == nil {
			c.Status(http.StatusNotFound)
			return
		}
		data, err := s.ogGen.GetOrGenerate(slug, og.PostInfo{Title: p.Title, Date: p.CreatedAt, SiteTitle: s.site.Load().Title})
		if err != nil {
			stellarError(c, err)
			return
		}
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "image/png", data)
	})
}
