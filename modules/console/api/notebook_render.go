package api

import (
	"fmt"
	"net/http"

	"blog/internal/post"
	"blog/internal/store"

	"github.com/gin-gonic/gin"
)

func (h *Handler) renderNotebookNote(c *gin.Context, note *store.Note, page map[string]any) bool {
	if h.posts == nil {
		Fail(c, http.StatusServiceUnavailable, "笔记渲染服务未初始化")
		return false
	}
	site := h.site.Load()
	if site == nil || site.StellarV2 == nil {
		notebookError(c, fmt.Errorf("%w: settings.stellar_v2", store.ErrStellarMigrationRequired))
		return false
	}
	configuration := site.StellarV2
	data, _ := configuration.Blog["data"].(map[string]any)
	language, _ := configuration.Blog["language"].(string)
	if language == "" {
		language = "zh-CN"
	}
	rendered, err := h.posts.RenderStellarV2WithContext(note.ContentMD, post.StellarRenderContext{Theme: configuration.Theme, Data: data, Page: page, Language: language, BlogExtensions: true})
	if err != nil {
		Fail(c, http.StatusBadRequest, "笔记 Markdown 渲染失败："+err.Error())
		return false
	}
	note.ContentHTML = rendered
	return true
}
