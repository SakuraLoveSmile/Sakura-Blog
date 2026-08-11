package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// listAssetIcons GET /api/v1/assets/icons：枚举 icons.yml 全部图标（key + 内联 SVG/URL），
// 供 Console 插件库「素材速查」展示与一键复制（{% icon key %}、分类/标签图标配置）。
func (h *Handler) listAssetIcons(c *gin.Context) {
	if h.icons == nil {
		c.JSON(http.StatusOK, gin.H{"items": []any{}})
		return
	}
	keys := h.icons.Keys()
	items := make([]gin.H, 0, len(keys))
	for _, k := range keys {
		items = append(items, gin.H{"key": k, "value": h.icons.Get(k)})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
