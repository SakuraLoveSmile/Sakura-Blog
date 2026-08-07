package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// pageWidgetJSON 前端 widget 配置项：组件名 + 页面级参数覆盖（与全局 library 浅合并）。
type pageWidgetJSON struct {
	Name   string         `json:"name"`
	Params map[string]any `json:"params,omitempty"`
}

// pageJSON 页面 API 响应模型。leftbar_widgets / rightbar_widgets：
// null = 未配置（继承全局 page 级默认）；[] = 明确空侧栏；[{name, params}, ...] = 自定义序列。
type pageJSON struct {
	Slug            string            `json:"slug"`
	Title           string            `json:"title"`
	ContentMD       string            `json:"content_md,omitempty"`
	Status          string            `json:"status"`
	IsSystem        bool              `json:"is_system"`
	Views           int64             `json:"views"`
	LeftbarWidgets  []pageWidgetJSON  `json:"leftbar_widgets,omitempty"`
	RightbarWidgets []pageWidgetJSON  `json:"rightbar_widgets,omitempty"`
	CreatedAt       string            `json:"created_at"`
	UpdatedAt       string            `json:"updated_at"`
}

func toPageJSON(p *store.Page) *pageJSON {
	j := &pageJSON{
		Slug:      p.Slug,
		Title:     p.Title,
		ContentMD: p.ContentMD,
		Status:    p.Status,
		IsSystem:  p.IsSystem,
		Views:     p.Views,
		CreatedAt: p.CreatedAt.Format(time.RFC3339),
		UpdatedAt: p.UpdatedAt.Format(time.RFC3339),
	}
	// 空串 "": 未配置 → 输出 null（omitempty 生效）。
	// "[]": 明确空侧栏 → 解析为 0 长切片 → 显式输出 []。
	// 非空：store 解析自动兼容旧格式字符串数组 → 统一转成 pageWidgetJSON。
	if cfgs, ok := p.ParseLeftbar(); ok {
		j.LeftbarWidgets = make([]pageWidgetJSON, 0, len(cfgs))
		for _, c := range cfgs {
			j.LeftbarWidgets = append(j.LeftbarWidgets, pageWidgetJSON{Name: c.Name, Params: c.Params})
		}
	}
	if cfgs, ok := p.ParseRightbar(); ok {
		j.RightbarWidgets = make([]pageWidgetJSON, 0, len(cfgs))
		for _, c := range cfgs {
			j.RightbarWidgets = append(j.RightbarWidgets, pageWidgetJSON{Name: c.Name, Params: c.Params})
		}
	}
	return j
}

// toPageListJSON 列表用，不包含大字段 content_md
func toPageListJSON(p *store.Page) *pageJSON {
	j := &pageJSON{
		Slug:      p.Slug,
		Title:     p.Title,
		Status:    p.Status,
		IsSystem:  p.IsSystem,
		Views:     p.Views,
		CreatedAt: p.CreatedAt.Format(time.RFC3339),
		UpdatedAt: p.UpdatedAt.Format(time.RFC3339),
	}
	return j
}

// decodeWidgetConfigs 前端入参（[{name, params}, ...]）→ store 存的 JSON 串：
// null/缺失字段 → ""（未配置，继承全局）；非 nil（含 0 长 []）→ EncodeWidgetConfigs 编码。
func decodeWidgetConfigs(widgets []pageWidgetJSON) string {
	if widgets == nil {
		return ""
	}
	cfgs := make([]store.WidgetConfig, 0, len(widgets))
	for _, w := range widgets {
		cfgs = append(cfgs, store.WidgetConfig{Name: w.Name, Params: w.Params})
	}
	return store.EncodeWidgetConfigs(cfgs)
}

// listPages GET /api/v1/pages：页面列表（支持分页、搜索、状态筛选）。
func (h *Handler) listPages(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	status := c.Query("status")
	keyword := c.Query("search")

	var (
		pages []*store.Page
		total int
		err   error
	)

	if status == "trashed" {
		pages, total, err = h.store.ListTrashedPagesPage(page, size, keyword)
	} else {
		pages, total, err = h.store.ListPagesPage(page, size, status, keyword)
	}

	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]*pageJSON, 0, len(pages))
	for _, p := range pages {
		items = append(items, toPageListJSON(p))
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
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
		Slug            string           `json:"slug"`
		Title           string           `json:"title"`
		ContentMD       string           `json:"content_md"`
		Status          string           `json:"status"`
		LeftbarWidgets  []pageWidgetJSON `json:"leftbar_widgets"`
		RightbarWidgets []pageWidgetJSON `json:"rightbar_widgets"`
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
		Slug:            strings.TrimSpace(req.Slug),
		Title:           strings.TrimSpace(req.Title),
		ContentMD:       req.ContentMD,
		ContentHTML:     contentHTML,
		Status:          status,
		LeftbarWidgets:  decodeWidgetConfigs(req.LeftbarWidgets),
		RightbarWidgets: decodeWidgetConfigs(req.RightbarWidgets),
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
		Slug            string           `json:"slug"`
		Title           string           `json:"title"`
		ContentMD       string           `json:"content_md"`
		Status          string           `json:"status"`
		LeftbarWidgets  []pageWidgetJSON `json:"leftbar_widgets"`
		RightbarWidgets []pageWidgetJSON `json:"rightbar_widgets"`
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

	// 左右栏：前端传非 nil []pageWidgetJSON（含 0 长 []）才覆盖原值；
	// 若完全没传该 key（ShouldBindJSON 后为 nil，区分于显式 []），保留已有值。
	leftRaw, rightRaw := existing.LeftbarWidgets, existing.RightbarWidgets
	if req.LeftbarWidgets != nil {
		leftRaw = decodeWidgetConfigs(req.LeftbarWidgets)
	}
	if req.RightbarWidgets != nil {
		rightRaw = decodeWidgetConfigs(req.RightbarWidgets)
	}

	p := &store.Page{
		Slug:            newSlug,
		Title:           strings.TrimSpace(req.Title),
		ContentMD:       req.ContentMD,
		ContentHTML:     contentHTML,
		Status:          status,
		LeftbarWidgets:  leftRaw,
		RightbarWidgets: rightRaw,
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
