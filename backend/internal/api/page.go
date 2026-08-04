package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// pageJSON 页面 API 响应模型。
type pageJSON struct {
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	ContentMD string `json:"content_md"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func toPageJSON(p *store.Page) *pageJSON {
	return &pageJSON{
		Slug:      p.Slug,
		Title:     p.Title,
		ContentMD: p.ContentMD,
		Status:    p.Status,
		CreatedAt: p.CreatedAt.Format(time.RFC3339),
		UpdatedAt: p.UpdatedAt.Format(time.RFC3339),
	}
}

// listPages GET /api/v1/pages：页面列表。
func (h *Handler) listPages(c *gin.Context) {
	// 回收站专用列表
	if c.Query("status") == "trashed" {
		pages, err := h.store.ListTrashedPages()
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		items := make([]*pageJSON, 0, len(pages))
		for _, p := range pages {
			items = append(items, toPageJSON(p))
		}
		c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
		return
	}

	pages, err := h.store.ListPages()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]*pageJSON, 0, len(pages))
	for _, p := range pages {
		items = append(items, toPageJSON(p))
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// getPage GET /api/v1/pages/:slug：页面详情。
func (h *Handler) getPage(c *gin.Context) {
	slug := c.Param("slug")
	p, err := h.store.GetPageBySlug(slug)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if p == nil {
		Fail(c, http.StatusNotFound, "页面不存在")
		return
	}
	c.JSON(http.StatusOK, toPageJSON(p))
}

// createPage POST /api/v1/pages：创建页面。
func (h *Handler) createPage(c *gin.Context) {
	var req struct {
		Slug      string `json:"slug"`
		Title     string `json:"title"`
		ContentMD string `json:"content_md"`
		Status    string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if strings.TrimSpace(req.Slug) == "" {
		Fail(c, http.StatusBadRequest, "slug 不能为空")
		return
	}

	if existing, _ := h.store.GetPageBySlug(req.Slug); existing != nil {
		Fail(c, http.StatusConflict, "slug 已存在")
		return
	}

	contentHTML, err := h.posts.Render(req.ContentMD)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "Markdown 渲染失败")
		return
	}

	status := req.Status
	if status != store.StatusDraft && status != store.StatusPublished {
		status = store.StatusDraft
	}

	p := &store.Page{
		Slug:        strings.TrimSpace(req.Slug),
		Title:       strings.TrimSpace(req.Title),
		ContentMD:   req.ContentMD,
		ContentHTML: contentHTML,
		Status:      status,
	}
	if _, err := h.store.CreatePage(p); err != nil {
		Fail(c, http.StatusInternalServerError, "创建失败: " + err.Error())
		return
	}
	c.JSON(http.StatusCreated, toPageJSON(p))
}

// updatePage PUT /api/v1/pages/:slug：更新页面。
func (h *Handler) updatePage(c *gin.Context) {
	slug := c.Param("slug")
	existing, err := h.store.GetPageBySlug(slug)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		Fail(c, http.StatusNotFound, "页面不存在")
		return
	}

	var req struct {
		Slug      string `json:"slug"`
		Title     string `json:"title"`
		ContentMD string `json:"content_md"`
		Status    string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}

	newSlug := strings.TrimSpace(req.Slug)
	if newSlug == "" {
		newSlug = slug
	}
	if newSlug != slug {
		if other, _ := h.store.GetPageBySlug(newSlug); other != nil {
			Fail(c, http.StatusConflict, "slug 已被占用")
			return
		}
	}

	contentHTML, err := h.posts.Render(req.ContentMD)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "Markdown 渲染失败")
		return
	}

	status := req.Status
	if status != store.StatusDraft && status != store.StatusPublished {
		status = existing.Status
	}

	p := &store.Page{
		Slug:        newSlug,
		Title:       strings.TrimSpace(req.Title),
		ContentMD:   req.ContentMD,
		ContentHTML: contentHTML,
		Status:      status,
	}
	if err := h.store.UpdatePage(slug, p); err != nil {
		Fail(c, http.StatusInternalServerError, "更新失败: " + err.Error())
		return
	}
	c.JSON(http.StatusOK, toPageJSON(p))
}

// deletePage DELETE /api/v1/pages/:slug：删除页面。
func (h *Handler) deletePage(c *gin.Context) {
	slug := c.Param("slug")
	if err := h.store.DeletePage(slug); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// restorePage POST /api/v1/pages/:slug/restore：从回收站恢复页面。
func (h *Handler) restorePage(c *gin.Context) {
	slug := c.Param("slug")
	if err := h.store.RestorePage(slug); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// purgePage DELETE /api/v1/pages/:slug/purge：彻底删除页面。
func (h *Handler) purgePage(c *gin.Context) {
	slug := c.Param("slug")
	if err := h.store.PurgePage(slug); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
