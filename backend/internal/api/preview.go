package api

import (
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
		Fail(c, http.StatusInternalServerError, "渲染失败: " + err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"html": html})
}
