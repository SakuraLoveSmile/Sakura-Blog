package routes

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"

	"github.com/gin-gonic/gin"
)

// handleTopicIndex 专栏索引页：按 topics.sort DESC 展示专栏名称、描述、封面与已发布文章数。
// 专栏没有配置描述/封面时，使用该专栏最新文章的摘要/封面，再回退到本地预览素材。
func (s *webServer) handleTopicIndex(c *gin.Context) {
	topics, err := s.st.ListTopics()
	if err != nil {
		log.Printf("list topics: %v", err)
		s.render500(c)
		return
	}
	items := make([]*tmpl.TopicItem, 0, len(topics))
	fallbackCovers := []string{
		"/assets/img/preview/gallery-1.jpg",
		"/assets/img/preview/gallery-2.jpg",
		"/assets/img/preview/gallery-3.jpg",
		"/assets/img/preview/image-demo.jpg",
	}
	for i, t := range topics {
		n, err := s.st.CountByTopicID(t.ID)
		if err != nil {
			log.Printf("count posts of topic %d: %v", t.ID, err)
			s.render500(c)
			return
		}

		description := strings.TrimSpace(t.Description)
		cover := strings.TrimSpace(t.Cover)
		posts, err := s.st.ListPostsByTopicID(t.ID)
		if err != nil {
			log.Printf("list posts of topic %d: %v", t.ID, err)
			s.render500(c)
			return
		}
		if len(posts) > 0 {
			if description == "" {
				description = strings.TrimSpace(posts[0].Excerpt)
			}
			if cover == "" {
				cover = strings.TrimSpace(posts[0].Cover)
			}
		}
		if description == "" {
			description = fmt.Sprintf("围绕“%s”的文章与实践记录。", t.Name)
		}
		if cover == "" {
			cover = strings.TrimSpace(t.Icon)
		}
		if cover == "" {
			cover = fallbackCovers[i%len(fallbackCovers)]
		}
		cards := s.toCards(posts)
		var latest *tmpl.PostCard
		var rest []*tmpl.PostCard
		if len(cards) > 0 {
			latest = cards[0]
			if len(cards) > 1 {
				rest = cards[1:]
				if len(rest) > 3 {
					rest = rest[:3]
				}
			}
		}
		slug := strings.TrimSpace(t.Slug)
		if slug == "" {
			slug = t.Name
		}
		items = append(items, &tmpl.TopicItem{
			Name:        t.Name,
			Slug:        slug,
			Count:       n,
			Description: description,
			Cover:       cover,
			Icon:        t.Icon,
			LatestPost:  latest,
			Posts:       rest,
			CreatedAt:   t.CreatedAt,
		})
	}
	s.renderBase(c, &tmpl.ViewData{
		Site:   s.site.Load(),
		Page:   &tmpl.Page{Title: "专栏", Layout: "topics", MenuID: "post", Lang: "zh-CN"},
		Topics: items,
	})
}

// handleTopicDetail 专栏文章列表页（Spec 52）：专栏头（名称/描述/图标/封面 + 文章数）+ 文章卡片流。
// 数据走独立 topics 表（slug 定位 + topic_id 关联文章）；slug 未命中时按 name 兑底 301 到 slug 路径。
func (s *webServer) handleTopicDetail(c *gin.Context) {
	slug := c.Param("slug")
	t, err := s.st.GetTopicBySlug(slug)
	if err != nil {
		log.Printf("get topic %q: %v", slug, err)
		s.render500(c)
		return
	}
	// /topic/:name 旧路径兼容（Spec 52）：slug 未命中时按 name 查，命中则 301 到 slug 路径。
	if t == nil {
		legacy, err := s.st.GetTopicByName(slug)
		if err != nil {
			log.Printf("get topic by name %q: %v", slug, err)
			s.render500(c)
			return
		}
		if legacy != nil {
			c.Redirect(http.StatusMovedPermanently, "/topic/"+legacy.Slug)
			return
		}
		s.render404(c)
		return
	}
	posts, err := s.st.ListPostsByTopicID(t.ID)
	if err != nil {
		log.Printf("list posts of topic %d: %v", t.ID, err)
		s.render500(c)
		return
	}
	count, err := s.st.CountByTopicID(t.ID)
	if err != nil {
		log.Printf("count posts of topic %d: %v", t.ID, err)
		s.render500(c)
		return
	}
	hue := 0
	for _, r := range t.Name {
		hue = (hue*31 + int(r)) % 360
	}
	initial := ""
	if rs := []rune(t.Name); len(rs) > 0 {
		initial = string(rs[0])
	}
	s.renderBase(c, &tmpl.ViewData{
		Site: s.site.Load(),
		Page: &tmpl.Page{
			Title:  stellarweb.Lang("nav.topic", t.Name),
			Layout: "topic",
			MenuID: "post",
			Lang:   "zh-CN",
			Topic:  t.Name,
			Type:   s.site.Load().Article.Type,
		},
		TopicDetail: &tmpl.TopicDetail{
			Slug:        t.Slug,
			Name:        t.Name,
			Description: t.Description,
			Icon:        t.Icon,
			Cover:       t.Cover,
			Count:       count,
			Hue:         hue,
			Initial:     initial,
		},
		Posts: s.toCards(posts),
	})
}
