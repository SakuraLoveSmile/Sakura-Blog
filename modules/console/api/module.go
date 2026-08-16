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
		Fail(c, http.StatusServiceUnavailable, "模块系统未初始化")
		return
	}
	id := c.Param("id")
	if !h.modules.Has(id) {
		Fail(c, http.StatusNotFound, "模块不存在")
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		Fail(c, http.StatusBadRequest, "请求体格式错误")
		return
	}
	if err := h.modules.SetEnabled(id, body.Enabled, module.Deps{Store: h.store, Posts: h.posts, Site: h.site, Modules: h.modules}); err != nil {
		// 依赖缺失/未启用/被依赖等拓扑问题属于请求错误（400）
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "enabled": body.Enabled})
}

// notifySEOPublished 文章发布后触发 SEO 自动推送（模块启用 + push.enabled 时才生效）。
func (h *Handler) notifySEOPublished(slug string) {
	if h.modules == nil || h.site.Load() == nil || h.site.Load().URL == "" {
		return
	}
	url := strings.TrimRight(h.site.Load().URL, "/") + "/post/" + slug
	seo.NotifyPublished(module.Deps{Store: h.store, Site: h.site, Modules: h.modules}, url)
}
