// Package page 独立页面：前台 /pages/:slug 渲染（后台管理已迁移到 Vue Console + /api/v1）。
package page

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/post"
	"blog/internal/store"
	tmpl "blog/internal/template"
)

// Handler 独立页面请求处理器。
type Handler struct {
	store     *store.Store
	posts     *post.Service
	engine    *tmpl.Engine
	site      *tmpl.Site
	render404 gin.HandlerFunc
	fillExtra func(*tmpl.ViewData, *gin.Context) // 前台渲染前填充 sidebar/SEO
}

// New 创建 page handler，确保 pages 表存在。
func New(s *store.Store, posts *post.Service, engine *tmpl.Engine, site *tmpl.Site) (*Handler, error) {
	if err := s.EnsurePagesTable(); err != nil {
		return nil, err
	}
	return &Handler{store: s, posts: posts, engine: engine, site: site}, nil
}

// SetRender404 注入自定义 404 渲染（main.go 的 render404）；未设置时返回简单 404。
func (h *Handler) SetRender404(fn gin.HandlerFunc) {
	h.render404 = fn
}

// SetFillExtra 注入前台页面渲染前的 sidebar/SEO 填充回调。
func (h *Handler) SetFillExtra(fn func(*tmpl.ViewData, *gin.Context)) {
	h.fillExtra = fn
}

// Register 挂前台路由 GET /pages/:slug（兼容旧路径）。
// 根路径 /:slug 由 main.go NoRoute 兜底调 ShowByPath，避免与 /wiki/:project /author/:id 等参数路由冲突。
func (h *Handler) Register(r *gin.Engine) {
	r.GET("/pages/:slug", h.show)
}

// ShowByPath NoRoute 兜底：如果请求路径是单段 slug（不含中间 '/', 非 /console/），
// 尝试按 pages 表 published 记录渲染；命中返回 true，未命中返回 false 由调用方继续 404。
func (h *Handler) ShowByPath(c *gin.Context) bool {
	p := c.Request.URL.Path
	if len(p) < 2 || p[0] != '/' {
		return false
	}
	seg := p[1:]
	// 拒多段、空段、控制台前缀（/console 已被外层 NoRoute 提前处理，这里双层保险）
	if seg == "" || strings.Contains(seg, "/") || strings.HasPrefix(seg, "console") {
		return false
	}
	page, err := h.store.GetPublished(seg)
	if err != nil || page == nil {
		return false
	}
	_ = h.store.IncrementPageViews(seg)
	h.renderPage(c, page)
	return true
}

func (h *Handler) renderPage(c *gin.Context, p *store.Page) {
	data := &tmpl.ViewData{
		Site: h.site,
		Page: &tmpl.Page{
			Title:  p.Title,
			Slug:   p.Slug,
			Layout: "page",
			Lang:   "zh-CN",
		},
		Content: template.HTML(p.ContentHTML),
	}
	// 页面级自定义侧栏：只要左右栏至少一个是「显式配置过」（非空字符串，配置=继承全局不进来），
	// 就构造 CustomPageBars 指针；未配置的一侧用空切片（不落全局 DefaultPageBars 的默认组件）。
	// 同时提取页面级参数覆盖：WidgetOverrides → 转成 tmpl.WidgetDef 存入 PageWidgetOverrides，
	// BuildBar 时按组件名浅合并进全局 WidgetDef 参数。
	leftCfgs, leftOK := p.ParseLeftbar()
	rightCfgs, rightOK := p.ParseRightbar()
	if leftOK || rightOK {
		pb := &tmpl.PageBars{}
		if leftOK {
			pb.Leftbar = store.WidgetNames(leftCfgs)
		}
		if rightOK {
			pb.Rightbar = store.WidgetNames(rightCfgs)
		}
		data.CustomPageBars = pb
		// 页面级参数覆盖（组件名 → WidgetDef）
		overrides := make(map[string]tmpl.WidgetDef)
		for name, params := range store.WidgetOverrides(leftCfgs) {
			overrides[name] = tmpl.WidgetDef(params)
		}
		for name, params := range store.WidgetOverrides(rightCfgs) {
			overrides[name] = tmpl.WidgetDef(params)
		}
		if len(overrides) > 0 {
			data.PageWidgetOverrides = overrides
		}
	}
	if h.fillExtra != nil {
		h.fillExtra(data, c)
	}
	html, err := h.engine.Render("layout/base", data)
	if err != nil {
		c.String(http.StatusInternalServerError, "render: %v", err)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// show GET /pages/:slug：仅 published 展示，否则 404。复用 renderPage 与 ShowByPath 走同一渲染分支。
func (h *Handler) show(c *gin.Context) {
	slug := c.Param("slug")
	p, err := h.store.GetPublished(slug)
	if err != nil {
		c.String(http.StatusInternalServerError, "get page: %v", err)
		return
	}
	if p == nil {
		h.notFound(c)
		return
	}
	_ = h.store.IncrementPageViews(slug)
	h.renderPage(c, p)
}

func (h *Handler) notFound(c *gin.Context) {
	if h.render404 != nil {
		h.render404(c)
		return
	}
	c.Status(http.StatusNotFound)
}
