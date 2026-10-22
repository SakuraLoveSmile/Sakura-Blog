package routes

import (
	"sort"
	"time"

	tmpl "blog/modules/stellar/viewmodel"

	"github.com/gin-gonic/gin"
)

// handleTopicIndex 专栏索引页：按 topics.sort DESC 展示专栏名称、描述、封面与已发布文章数。
// 专栏没有配置描述/封面时，使用该专栏最新文章的摘要/封面，再回退到本地预览素材。
func (s *webServer) handleTopicIndex(c *gin.Context) { s.serveStellarCollectionIndex(c, "topic", "") }

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
	s.serveStellarCollection(c, "topic", c.Param("slug"), 1, nil)
}
