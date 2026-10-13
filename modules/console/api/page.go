package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// PageWidgetJSON 前端 widget 配置项：组件名 + 页面级参数覆盖（与全局 library 浅合并）。
type PageWidgetJSON struct {
	Name   string         `json:"name"`
	Params map[string]any `json:"params,omitempty"`
}

// PageBannerInfoJSON 页面横幅可编辑覆盖项。
type PageBannerInfoJSON struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Avatar   string `json:"avatar"`
	Icon     string `json:"icon"`
}

// PageJSON 页面详情 API 响应模型。leftbar_widgets / rightbar_widgets：
// null = 未配置（继承全局 page 级默认）；[] = 明确空侧栏；[{name, params}, ...] = 自定义序列。
type PageJSON struct {
	StellarContentMetadata
	Slug            string             `json:"slug"`
	Title           string             `json:"title"`
	ContentMD       string             `json:"content_md,omitempty"`
	Status          string             `json:"status"`
	Banner          string             `json:"banner"`
	BannerInfo      PageBannerInfoJSON `json:"banner_info"`
	Breadcrumb      bool               `json:"breadcrumb"`
	Author          string             `json:"author"`
	Date            string             `json:"date"`
	IsSystem        bool               `json:"is_system"`
	Views           int64              `json:"views"`
	LeftbarWidgets  []PageWidgetJSON   `json:"leftbar_widgets,omitempty"`
	RightbarWidgets []PageWidgetJSON   `json:"rightbar_widgets,omitempty"`
	CreatedAt       string             `json:"created_at"`
	UpdatedAt       string             `json:"updated_at"`
}

// 兼容现有包内测试与调用点。
type pageJSON = PageJSON

// PageListJSON 页面列表响应模型，不包含正文与横幅大字段。
type PageListJSON struct {
	StellarContentMetadata
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	IsSystem  bool   `json:"is_system"`
	Views     int64  `json:"views"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// PageListResponse 页面列表响应。
type PageListResponse struct {
	Items []*PageListJSON `json:"items"`
	Total int             `json:"total"`
}

// pageRequest 是创建/更新共用的页面输入；新增字段使用指针以区分缺失和显式清空。
type pageRequest struct {
	StellarContentMetadata
	Slug            string              `json:"slug"`
	Title           string              `json:"title"`
	ContentMD       string              `json:"content_md"`
	Status          string              `json:"status"`
	Banner          *string             `json:"banner"`
	BannerInfo      *PageBannerInfoJSON `json:"banner_info"`
	Breadcrumb      *bool               `json:"breadcrumb"`
	Author          *string             `json:"author"`
	Date            *string             `json:"date"`
	LeftbarWidgets  []PageWidgetJSON    `json:"leftbar_widgets"`
	RightbarWidgets []PageWidgetJSON    `json:"rightbar_widgets"`
}

func toPageJSON(p *store.Page) *PageJSON {
	j := &PageJSON{
		StellarContentMetadata: StellarContentMetadata{p.StellarVersion, p.StellarConfig, p.BlogExtensions},
		Slug:                   p.Slug,
		Title:                  p.Title,
		ContentMD:              p.ContentMD,
		Status:                 p.Status,
		Banner:                 p.Banner,
		BannerInfo: PageBannerInfoJSON{
			Title:    p.BannerInfo.Title,
			Subtitle: p.BannerInfo.Subtitle,
			Avatar:   p.BannerInfo.Avatar,
			Icon:     p.BannerInfo.Icon,
		},
		Breadcrumb: p.Breadcrumb,
		Author:     p.Author,
		Date:       p.PublishedAt,
		IsSystem:   p.IsSystem,
		Views:      p.Views,
		CreatedAt:  p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  p.UpdatedAt.Format(time.RFC3339),
	}
	// 空串 "": 未配置 → 输出 null（omitempty 生效）。
	// "[]": 明确空侧栏 → 解析为 0 长切片 → 显式输出 []。
	// 非空：store 解析自动兼容旧格式字符串数组 → 统一转成 PageWidgetJSON。
	if cfgs, ok := p.ParseLeftbar(); ok {
		j.LeftbarWidgets = make([]PageWidgetJSON, 0, len(cfgs))
		for _, c := range cfgs {
			j.LeftbarWidgets = append(j.LeftbarWidgets, PageWidgetJSON{Name: c.Name, Params: c.Params})
		}
	}
	if cfgs, ok := p.ParseRightbar(); ok {
		j.RightbarWidgets = make([]PageWidgetJSON, 0, len(cfgs))
		for _, c := range cfgs {
			j.RightbarWidgets = append(j.RightbarWidgets, PageWidgetJSON{Name: c.Name, Params: c.Params})
		}
	}
	return j
}

// toPageListJSON 列表用，不包含大字段 content_md
func toPageListJSON(p *store.Page) *PageListJSON {
	j := &PageListJSON{
		StellarContentMetadata: StellarContentMetadata{p.StellarVersion, p.StellarConfig, p.BlogExtensions},
		Slug:                   p.Slug,
		Title:                  p.Title,
		Status:                 p.Status,
		IsSystem:               p.IsSystem,
		Views:                  p.Views,
		CreatedAt:              p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:              p.UpdatedAt.Format(time.RFC3339),
	}
	return j
}

// decodeWidgetConfigs 前端入参（[{name, params}, ...]）→ store 存的 JSON 串：
// null/缺失字段 → ""（未配置，继承全局）；非 nil（含 0 长 []）→ EncodeWidgetConfigs 编码。
func decodeWidgetConfigs(widgets []PageWidgetJSON) string {
	if widgets == nil {
		return ""
	}
	cfgs := make([]store.WidgetConfig, 0, len(widgets))
	for _, w := range widgets {
		cfgs = append(cfgs, store.WidgetConfig{Name: w.Name, Params: w.Params})
	}
	return store.EncodeWidgetConfigs(cfgs)
}

func normalizePageBannerInfo(info *PageBannerInfoJSON) store.PageBannerInfo {
	if info == nil {
		return store.PageBannerInfo{}
	}
	return store.PageBannerInfo{
		Title:    strings.TrimSpace(info.Title),
		Subtitle: strings.TrimSpace(info.Subtitle),
		Avatar:   strings.TrimSpace(info.Avatar),
		Icon:     strings.TrimSpace(info.Icon),
	}
}

func pageHeaderValues(req pageRequest, current *store.Page, creating bool) (string, store.PageBannerInfo, bool, string, string) {
	banner := ""
	bannerInfo := store.PageBannerInfo{}
	breadcrumb := true
	author := ""
	date := ""
	if current != nil {
		banner = current.Banner
		bannerInfo = current.BannerInfo
		breadcrumb = current.Breadcrumb
		author = current.Author
		date = current.PublishedAt
	}
	if req.Banner != nil {
		banner = strings.TrimSpace(*req.Banner)
	}
	if req.BannerInfo != nil {
		bannerInfo = normalizePageBannerInfo(req.BannerInfo)
	}
	if req.Breadcrumb != nil {
		breadcrumb = *req.Breadcrumb
	} else if creating {
		breadcrumb = true
	}
	if req.Author != nil {
		author = strings.TrimSpace(*req.Author)
	}
	if req.Date != nil {
		date = strings.TrimSpace(*req.Date)
	} else if creating {
		date = time.Now().UTC().Format("2006-01-02")
	}
	return banner, bannerInfo, breadcrumb, author, date
}

func (h *Handler) validatePageHeader(_ *string, _ *PageBannerInfoJSON, _ *bool, author, date *string) error {
	if author != nil {
		id := strings.TrimSpace(*author)
		if id != "" {
			if h.site == nil || h.site.Load() == nil {
				return fmt.Errorf("作者不存在: %s", id)
			}
			if _, ok := h.site.Load().Authors[id]; !ok {
				return fmt.Errorf("作者不存在: %s", id)
			}
		}
	}
	if date != nil {
		value := strings.TrimSpace(*date)
		if value != "" {
			if _, err := time.Parse("2006-01-02", value); err != nil {
				return fmt.Errorf("日期格式必须为 YYYY-MM-DD")
			}
		}
	}
	return nil
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

	items := make([]*PageListJSON, 0, len(pages))
	for _, p := range pages {
		items = append(items, toPageListJSON(p))
	}
	c.JSON(http.StatusOK, PageListResponse{Items: items, Total: total})
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
	var req pageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	projectPageForm(&req)
	if strings.TrimSpace(req.Slug) == "" {
		Fail(c, http.StatusBadRequest, "slug 不能为空")
		return
	}
	if err := h.validatePageHeader(req.Banner, req.BannerInfo, req.Breadcrumb, req.Author, req.Date); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	if existing, _ := h.store.GetPageBySlug(req.Slug); existing != nil {
		Fail(c, http.StatusConflict, "slug 已存在")
		return
	}

	if err := h.checkEditorCollection(req.StellarContentMetadata, "page", ""); err != nil {
		failEditorStellar(c, err)
		return
	}
	contentHTML, err := h.renderEditorStellar(req.ContentMD, req.StellarContentMetadata, "page")
	if err != nil {
		failEditorStellar(c, err)
		return
	}

	status := req.Status
	if status != store.StatusDraft && status != store.StatusPublished {
		status = store.StatusDraft
	}

	banner, bannerInfo, breadcrumb, author, publishedAt := pageHeaderValues(req, nil, true)
	p := &store.Page{
		StellarVersion: req.StellarVersion, StellarConfig: req.StellarConfig, BlogExtensions: req.BlogExtensions,
		Slug:            strings.TrimSpace(req.Slug),
		Title:           strings.TrimSpace(req.Title),
		ContentMD:       req.ContentMD,
		ContentHTML:     contentHTML,
		Status:          status,
		Banner:          banner,
		BannerInfo:      bannerInfo,
		Breadcrumb:      breadcrumb,
		Author:          author,
		PublishedAt:     publishedAt,
		LeftbarWidgets:  decodeWidgetConfigs(req.LeftbarWidgets),
		RightbarWidgets: decodeWidgetConfigs(req.RightbarWidgets),
	}
	if _, err := h.store.CreatePage(p); err != nil {
		Fail(c, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}
	h.publishContentChanged()
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

	var req pageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	projectPageForm(&req)
	if err := h.validatePageHeader(req.Banner, req.BannerInfo, req.Breadcrumb, req.Author, req.Date); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
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

	if err := h.checkEditorCollection(req.StellarContentMetadata, "page", ""); err != nil {
		failEditorStellar(c, err)
		return
	}
	contentHTML, err := h.renderEditorStellar(req.ContentMD, req.StellarContentMetadata, "page")
	if err != nil {
		failEditorStellar(c, err)
		return
	}

	status := req.Status
	if status != store.StatusDraft && status != store.StatusPublished {
		status = existing.Status
	}

	// 左右栏：前端传非 nil []PageWidgetJSON（含 0 长 []）才覆盖原值；
	// 若完全没传该 key（ShouldBindJSON 后为 nil，区分于显式 []），保留已有值。
	leftRaw, rightRaw := existing.LeftbarWidgets, existing.RightbarWidgets
	if req.LeftbarWidgets != nil {
		leftRaw = decodeWidgetConfigs(req.LeftbarWidgets)
	}
	if req.RightbarWidgets != nil {
		rightRaw = decodeWidgetConfigs(req.RightbarWidgets)
	}
	banner, bannerInfo, breadcrumb, author, publishedAt := pageHeaderValues(req, existing, false)

	p := &store.Page{
		StellarVersion: req.StellarVersion, StellarConfig: req.StellarConfig, BlogExtensions: req.BlogExtensions,
		ID:              existing.ID,
		Slug:            newSlug,
		Title:           strings.TrimSpace(req.Title),
		ContentMD:       req.ContentMD,
		ContentHTML:     contentHTML,
		Status:          status,
		Banner:          banner,
		BannerInfo:      bannerInfo,
		Breadcrumb:      breadcrumb,
		Author:          author,
		PublishedAt:     publishedAt,
		IsSystem:        existing.IsSystem,
		Views:           existing.Views,
		LeftbarWidgets:  leftRaw,
		RightbarWidgets: rightRaw,
		CreatedAt:       existing.CreatedAt,
		DeletedAt:       existing.DeletedAt,
	}
	if err := h.store.UpdatePage(slug, p); err != nil {
		Fail(c, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}
	h.publishContentChanged()
	c.JSON(http.StatusOK, toPageJSON(p))
}

// deletePage DELETE /api/v1/pages/:slug：删除页面。
func (h *Handler) deletePage(c *gin.Context) {
	slug := c.Param("slug")
	if err := h.store.DeletePage(slug); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.publishContentChanged()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// restorePage POST /api/v1/pages/:slug/restore：从回收站恢复页面。
func (h *Handler) restorePage(c *gin.Context) {
	slug := c.Param("slug")
	if err := h.store.RestorePage(slug); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.publishContentChanged()
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
