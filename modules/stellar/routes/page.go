// PageHandler owns the public standalone-page routes.
package routes

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/post"
	tmpl "blog/internal/settings"
	"blog/internal/store"
)

// FriendsSlug 友链页系统页面 slug：/friends 路由按此 slug 从 pages 表渲染内容页。
const FriendsSlug = "friends"

// NotesSlug 笔记页系统页面 slug：/notes 路由按此 slug 从 pages 表渲染内容页。
const NotesSlug = "notes"

// LinksSlug 友链卡片页系统页面 slug：/links 路由按此 slug 从 pages 表渲染内容页。
const LinksSlug = "links"

// defaultLinksMD 友链卡片页缺省正文：{% links %} 服务端直出分组友链卡片。
const defaultLinksMD = `{% links %}
`

// defaultFriendsMD 友链页缺省正文：友链列表组件（带 RSS 文章）+ 交换说明。
// 站点可在控制台「页面」模块编辑该内容（shortcode {% friends posts:true %} 控制组件形态）。
const defaultFriendsMD = `## 友链列表

{% friends posts:true %}

## 如何交换友链？

{% note color:blue 欢迎交换友链：请先在您的站点添加本站链接，再通过评论留下您的站点信息（名称 / 网址 / 头像 / 描述），审核通过后即可展示。 %}
`

// Handler 独立页面请求处理器。
type PageHandler struct {
	store  *store.Store
	posts  *post.Service
	engine PageRenderer
	// site 站点配置原子指针（Spec 61 阶段 3）：RebuildSite 复制-交换发布，渲染时 .Load() 读取
	site      *atomic.Pointer[tmpl.Site]
	render404 gin.HandlerFunc
	fillExtra func(*tmpl.ViewData, *gin.Context) // 前台渲染前填充 sidebar/SEO
}

// Renderer keeps the internal page service independent from Stellar's web
// template implementation.
type PageRenderer interface {
	Render(string, *tmpl.ViewData) (string, error)
}

// New 创建 page handler，确保 pages 表存在。
func NewPageHandler(s *store.Store, posts *post.Service, engine PageRenderer, site *atomic.Pointer[tmpl.Site]) (*PageHandler, error) {
	if err := s.EnsurePagesTable(); err != nil {
		return nil, err
	}
	return &PageHandler{store: s, posts: posts, engine: engine, site: site}, nil
}

// SetRender404 注入自定义 404 渲染（main.go 的 render404）；未设置时返回简单 404。
func (h *PageHandler) SetRender404(fn gin.HandlerFunc) {
	h.render404 = fn
}

// SetFillExtra 注入前台页面渲染前的 sidebar/SEO 填充回调。
func (h *PageHandler) SetFillExtra(fn func(*tmpl.ViewData, *gin.Context)) {
	h.fillExtra = fn
}

// EnsureSystemFriendsPage 确保 pages 表存在友链系统页（slug=friends，published）。
// 已存在（含用户编辑过的）不覆盖；缺失时创建并预渲染 ContentHTML。
func (h *PageHandler) EnsureSystemFriendsPage() error {
	if p, err := h.store.GetPageBySlug(FriendsSlug); err == nil && p != nil {
		return nil
	}
	html, err := h.posts.Render(defaultFriendsMD)
	if err != nil {
		return err
	}
	now := time.Now()
	id, err := h.store.CreatePage(&store.Page{
		Slug: FriendsSlug, Title: "友链", ContentMD: defaultFriendsMD, ContentHTML: html,
		Status: store.StatusPublished, IsSystem: true, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return err
	}
	log.Printf("[info] seeded system friends page (id=%d, slug=%s)", id, FriendsSlug)
	return nil
}

// EnsureSystemNotesPage 确保 pages 表存在笔记系统页（slug=notes，published）。
// 首次迁移时导入旧 content/notes.md 原文作为正文（记录已存在不覆盖）。
func (h *PageHandler) EnsureSystemNotesPage(importPath string) error {
	if p, err := h.store.GetPageBySlug(NotesSlug); err == nil && p != nil {
		return nil
	}
	md := ""
	if importPath != "" {
		if b, err := os.ReadFile(importPath); err == nil {
			md = string(b)
		}
	}
	if md == "" {
		md = "（笔记页正文，可在控制台编辑）\n"
	}
	html, err := h.posts.Render(md)
	if err != nil {
		return err
	}
	now := time.Now()
	id, err := h.store.CreatePage(&store.Page{
		Slug: NotesSlug, Title: "笔记", ContentMD: md, ContentHTML: html,
		Status: store.StatusPublished, IsSystem: true, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return err
	}
	log.Printf("[info] seeded system notes page (id=%d, slug=%s, imported=%t)", id, NotesSlug, importPath != "" && md != "")
	return nil
}

// EnsureSystemLinksPage 确保 pages 表存在友链卡片系统页（slug=links，published）。
// 已存在（含用户编辑过的）不覆盖。
func (h *PageHandler) EnsureSystemLinksPage() error {
	if p, err := h.store.GetPageBySlug(LinksSlug); err == nil && p != nil {
		return nil
	}
	html, err := h.posts.Render(defaultLinksMD)
	if err != nil {
		return err
	}
	now := time.Now()
	id, err := h.store.CreatePage(&store.Page{
		Slug: LinksSlug, Title: "友链", ContentMD: defaultLinksMD, ContentHTML: html,
		Status: store.StatusPublished, IsSystem: true, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return err
	}
	log.Printf("[info] seeded system links page (id=%d, slug=%s)", id, LinksSlug)
	return nil
}

// Register 挂前台路由 GET /pages/:slug（兼容旧路径）。
// 根路径 /:slug 由 main.go NoRoute 兜底调 ShowByPath，避免与 /wiki/:project /author/:id 等参数路由冲突。
func (h *PageHandler) Register(r *gin.Engine) {
	r.GET("/pages/:slug", h.show)
}

// ShowByPath NoRoute 兜底：如果请求路径是单段 slug（不含中间 '/', 非 /console/），
// 尝试按 pages 表 published 记录渲染；命中返回 true，未命中返回 false 由调用方继续 404。
func (h *PageHandler) ShowByPath(c *gin.Context) bool {
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

func (h *PageHandler) renderPage(c *gin.Context, p *store.Page) {
	data := &tmpl.ViewData{
		Site: h.site.Load(),
		Page: &tmpl.Page{
			Title:  p.Title,
			Slug:   p.Slug,
			Layout: "page",
			Lang:   "zh-CN",
			MenuID: p.Slug, // 自动菜单项（MenuPages）按 slug 高亮
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
func (h *PageHandler) show(c *gin.Context) {
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

func (h *PageHandler) notFound(c *gin.Context) {
	if h.render404 != nil {
		h.render404(c)
		return
	}
	c.Status(http.StatusNotFound)
}
