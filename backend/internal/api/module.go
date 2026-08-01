package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/module"
	"blog/internal/module/seo"
)

// listModules GET /api/v1/modules：返回全部功能模块 meta + 启停状态（供管理页与动态菜单）。
func (h *Handler) listModules(c *gin.Context) {
	if h.modules == nil {
		c.JSON(http.StatusOK, gin.H{"items": []any{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": h.modules.List()})
}

// toggleModule PUT /api/v1/modules/:id：启用/禁用某模块，持久化并即时生效。
func (h *Handler) toggleModule(c *gin.Context) {
	if h.modules == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "模块系统未初始化"})
		return
	}
	id := c.Param("id")
	if !h.modules.Has(id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "模块不存在"})
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式错误"})
		return
	}
	if err := h.modules.SetEnabled(id, body.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "enabled": body.Enabled})
}

// notifySEOPublished 文章发布后触发 SEO 自动推送（模块启用 + push.enabled 时才生效）。
func (h *Handler) notifySEOPublished(slug string) {
	if h.modules == nil || h.site == nil || h.site.URL == "" {
		return
	}
	url := strings.TrimRight(h.site.URL, "/") + "/post/" + slug
	seo.NotifyPublished(module.Deps{Store: h.store, Site: h.site, Modules: h.modules}, url)
}
