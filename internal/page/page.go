// Package page 独立页面：前台 /pages/:slug 与后台 /admin/pages CRUD。
package page

import (
	"html/template"
	"net/http"

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

// Register 挂前台路由 GET /pages/:slug。
func (h *Handler) Register(r *gin.Engine) {
	r.GET("/pages/:slug", h.show)
}

// show GET /pages/:slug：仅 published 展示，否则 404。
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

func (h *Handler) notFound(c *gin.Context) {
	if h.render404 != nil {
		h.render404(c)
		return
	}
	c.Status(http.StatusNotFound)
}
