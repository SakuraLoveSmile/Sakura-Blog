package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// 友链 RSS 订阅阅读器 API（Spec 三-3b）。文章由 friends 聚合器抓取后落库（feed_articles 表），
// 已读/收藏/稍后状态持久化。刷新复用聚合器的 Refresh（re-fetch 全部 + upsert）。

// listSubscriptions GET /subscriptions：订阅源列表（各源未读/总数）+ 全局未读合计。
func (h *Handler) listSubscriptions(c *gin.Context) {
	sources, err := h.store.ListFeedSources()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	total, unread := 0, 0
	for _, s := range sources {
		total += s.Total
		unread += s.Unread
	}
	c.JSON(http.StatusOK, gin.H{"sources": sources, "total": total, "unread": unread})
}

// listSubscriptionArticles GET /subscriptions/articles?feed=&filter=&page=：文章分页列表。
func (h *Handler) listSubscriptionArticles(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	const pageSize = 20
	items, err := h.store.ListFeedArticles(store.FeedArticleFilter{
		FriendURL: c.Query("feed"),
		Status:    c.DefaultQuery("filter", "all"),
		Limit:     pageSize,
		Offset:    (page - 1) * pageSize,
	})
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "page": page, "page_size": pageSize})
}

// markSubscriptionRead POST /subscriptions/read {id, read}：单条已读/未读切换。
func (h *Handler) markSubscriptionRead(c *gin.Context) {
	var req struct {
		ID   int64 `json:"id"`
		Read bool  `json:"read"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	if err := h.store.SetFeedArticleFlag(req.ID, "read", req.Read); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// markAllSubscriptionRead POST /subscriptions/read-all?feed=：全部（或指定源）标为已读。
func (h *Handler) markAllSubscriptionRead(c *gin.Context) {
	if err := h.store.MarkAllFeedRead(c.Query("feed")); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// toggleSubscriptionFlag POST /subscriptions/flag {id, flag, value}：favorite/later 切换。
func (h *Handler) toggleSubscriptionFlag(c *gin.Context) {
	var req struct {
		ID    int64  `json:"id"`
		Flag  string `json:"flag"`
		Value bool   `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Flag != "favorite" && req.Flag != "later" {
		Fail(c, http.StatusBadRequest, "非法 flag")
		return
	}
	if err := h.store.SetFeedArticleFlag(req.ID, req.Flag, req.Value); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// refreshSubscriptions POST /subscriptions/refresh：触发聚合器重新抓取全部 RSS 并落库（异步）。
func (h *Handler) refreshSubscriptions(c *gin.Context) {
	if h.onLinksChanged == nil {
		Fail(c, http.StatusServiceUnavailable, "聚合器未初始化")
		return
	}
	h.onLinksChanged() // = friendsAgg.Refresh()（异步 re-fetch + upsert）
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
