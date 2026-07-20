package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// dashboardStats GET /api/v1/dashboard/stats：仪表盘统计数据。
func (h *Handler) dashboardStats(c *gin.Context) {
	postTotal, err := h.store.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	published, err := h.store.CountPublished()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	commentTotal, err := h.store.CountComments("")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	commentPending, err := h.store.CountComments(store.CommentPending)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	viewTotal, err := h.store.SumViews()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	attachmentTotal, err := h.store.CountAttachments()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"post_total":       len(postTotal),
		"post_published":   published,
		"comment_total":    commentTotal,
		"comment_pending":  commentPending,
		"view_total":       viewTotal,
		"attachment_total": attachmentTotal,
	})
}

// dashboardRecentComments GET /api/v1/dashboard/recent-comments：最近评论。
func (h *Handler) dashboardRecentComments(c *gin.Context) {
	recent, err := h.store.ListRecentComments(5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	mirror := h.site.Comments.GravatarMirror
	items := make([]*commentJSON, 0, len(recent))
	for _, cm := range recent {
		items = append(items, toCommentJSON(cm, mirror))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
