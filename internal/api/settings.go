package api

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"blog/internal/config"
)

// getSettings GET /api/v1/settings：返回全部可编辑分段的原始 JSON。
func (h *Handler) getSettings(c *gin.Context) {
	secs, err := config.LoadSectionsJSON(h.store)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, secs)
}

// updateSection PUT /api/v1/settings/:section：保存单个分段配置，成功后热更新站点。
func (h *Handler) updateSection(c *gin.Context) {
	section := c.Param("section")
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取请求体失败"})
		return
	}
	if err := config.SaveSection(h.store, h.site, section, raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
