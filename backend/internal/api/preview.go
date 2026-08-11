package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

// previewMarkdown POST /api/v1/preview/markdown：把 Markdown 渲染成 HTML。
// 复用 post.Service 的 goldmark 管线（shortcode/KaTeX/Chroma 高亮），保证预览与前台渲染一致。
func (h *Handler) previewMarkdown(c *gin.Context) {
	var req struct {
		Markdown string `json:"markdown"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	html, err := h.posts.Render(req.Markdown)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "渲染失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"html": html})
}

// previewSite POST /api/v1/preview/site：设置页「前端预览」所见即所得。
// 请求体 {sections: {分段名: 草稿JSON}, page: index|categories|tags}，
// 后端用草稿配置（不落库）渲染真实前台整页 HTML 返回，Console 塞入 iframe 展示。
func (h *Handler) previewSite(c *gin.Context) {
	var req struct {
		Sections map[string]json.RawMessage `json:"sections"`
		Page     string                     `json:"page"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	if h.renderSitePreview == nil {
		Fail(c, http.StatusNotImplemented, "预览服务未装配")
		return
	}
	html, err := h.renderSitePreview(req.Sections, req.Page)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "渲染失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"html": html})
}
