package routes

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

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
		sortTopicCards(cards)
		latest, rest, visible, hidden := splitTopicCards(cards)
		slug := strings.TrimSpace(t.Slug)
		if slug == "" {
			slug = t.Name
		}
		items = append(items, &tmpl.TopicItem{
			Name:         t.Name,
			Slug:         slug,
			Count:        n,
			Description:  description,
			Cover:        cover,
			Icon:         t.Icon,
			LatestPost:   latest,
			Posts:        rest,
			VisiblePosts: visible,
			HiddenPosts:  hidden,
			CreatedAt:    t.CreatedAt,
			UpdatedAt:    t.UpdatedAt,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := topicItemDate(items[i])
		right := topicItemDate(items[j])
		if !left.Equal(right) {
			return left.After(right)
		}
		if items[i].Slug != items[j].Slug {
			return items[i].Slug < items[j].Slug
		}
		return items[i].Name < items[j].Name
	})
	s.renderBase(c, &tmpl.ViewData{
		Site:   s.site.Load(),
		Page:   &tmpl.Page{Title: "专栏", Layout: "topics", MenuID: "post", Lang: "zh-CN"},
		Topics: items,
	})
}

func topicItemDate(item *tmpl.TopicItem) time.Time {
	if item == nil {
		return time.Time{}
	}
	if item.LatestPost != nil {
		if !item.LatestPost.Date.IsZero() {
			return item.LatestPost.Date
		}
		if !item.LatestPost.UpdatedAt.IsZero() {
			return item.LatestPost.UpdatedAt
		}
	}
	if !item.UpdatedAt.IsZero() {
		return item.UpdatedAt
	}
	return item.CreatedAt
}

// sortTopicCards makes the first card the latest published article. Store
// queries historically put pinned posts first, which is correct for home lists
// but not for the topic homepage. Date is the published-date representation in
// the current view model; UpdatedAt and slug/title provide deterministic
// fallbacks for legacy rows with an empty date.
func sortTopicCards(cards []*tmpl.PostCard) {
	sort.SliceStable(cards, func(i, j int) bool {
		left, right := topicCardDate(cards[i]), topicCardDate(cards[j])
		if !left.Equal(right) {
			return left.After(right)
		}
		leftUpdated, rightUpdated := topicCardUpdated(cards[i]), topicCardUpdated(cards[j])
		if !leftUpdated.Equal(rightUpdated) {
			return leftUpdated.After(rightUpdated)
		}
		if cards[i].Slug != cards[j].Slug {
			return cards[i].Slug < cards[j].Slug
		}
		return cards[i].Title < cards[j].Title
	})
}

func topicCardDate(card *tmpl.PostCard) time.Time {
	if card == nil {
		return time.Time{}
	}
	if !card.Date.IsZero() {
		return card.Date
	}
	return card.UpdatedAt
}

func topicCardUpdated(card *tmpl.PostCard) time.Time {
	if card == nil {
		return time.Time{}
	}
	return card.UpdatedAt
}

// splitTopicCards implements topic_card.ejs: one latest card, three visible
// archive rows, and a native details block for all remaining rows.
func splitTopicCards(cards []*tmpl.PostCard) (*tmpl.PostCard, []*tmpl.PostCard, []*tmpl.ArchiveItem, []*tmpl.ArchiveItem) {
	if len(cards) == 0 {
		return nil, nil, nil, nil
	}
	latest := cards[0]
	rest := cards[1:]
	visible := make([]*tmpl.ArchiveItem, 0, minInt(len(rest), 3))
	hidden := make([]*tmpl.ArchiveItem, 0, maxInt(len(rest)-3, 0))
	for i, card := range rest {
		item := &tmpl.ArchiveItem{Slug: card.Slug, Title: card.Title, Date: card.Date}
		if i < 3 {
			visible = append(visible, item)
		} else {
			hidden = append(hidden, item)
		}
	}
	return latest, rest, visible, hidden
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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
