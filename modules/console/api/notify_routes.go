package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// 站内通知中心路由（Wave 3b）：认证组 /api/v1/notifications。
// notify 服务未注入（测试等场景）时统一返回空数据，不报错。

// listNotifications GET /api/v1/notifications?page=&size=：分页查询（未读优先）。
func (h *Handler) listNotifications(c *gin.Context) {
	userID := c.GetInt64("userID")
	if h.notify == nil || userID <= 0 {
		c.JSON(http.StatusOK, gin.H{"items": []*store.Notification{}, "total": 0, "unread": 0, "page": 1, "size": 20})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	items, total, err := h.notify.List(userID, page, size)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if items == nil {
		items = []*store.Notification{}
	}
	unread, _ := h.notify.UnreadCount(userID)
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "unread": unread, "page": page, "size": size})
}

// unreadNotifications GET /api/v1/notifications/unread-count：未读数（铃铛角标轮询用）。
func (h *Handler) unreadNotifications(c *gin.Context) {
	userID := c.GetInt64("userID")
	if h.notify == nil || userID <= 0 {
		c.JSON(http.StatusOK, gin.H{"count": 0})
		return
	}
	n, err := h.notify.UnreadCount(userID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": n})
}

// markNotificationRead PUT /api/v1/notifications/:id/read：标记单条已读。
func (h *Handler) markNotificationRead(c *gin.Context) {
	userID := c.GetInt64("userID")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "无效的通知 ID")
		return
	}
	if h.notify == nil || userID <= 0 {
		Fail(c, http.StatusNotFound, "通知不存在")
		return
	}
	ok, err := h.notify.MarkRead(userID, id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		Fail(c, http.StatusNotFound, "通知不存在或已读")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// markAllNotificationsRead PUT /api/v1/notifications/read-all：全部标记已读。
func (h *Handler) markAllNotificationsRead(c *gin.Context) {
	userID := c.GetInt64("userID")
	if h.notify == nil || userID <= 0 {
		c.JSON(http.StatusOK, gin.H{"ok": true, "updated": 0})
		return
	}
	n, err := h.notify.MarkAllRead(userID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "updated": n})
}
