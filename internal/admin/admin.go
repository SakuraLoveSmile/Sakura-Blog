// Package admin 管理后台 handler 集合，挂 /admin 路由组。
// 文章 CRUD + Markdown 编辑器，数据走 store.Store，Markdown 渲染复用 post.Service。
package admin

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/post"
	"blog/internal/store"
	tmpl "blog/internal/template"
)

// Handler 管理后台请求处理器，持有 store、post service、模板引擎、站点配置。
type Handler struct {
	store   *store.Store
	posts   *post.Service
	engine  *tmpl.Engine
	site    *tmpl.Site
}

// New 创建 admin handler。
func New(s *store.Store, posts *post.Service, engine *tmpl.Engine, site *tmpl.Site) *Handler {
	return &Handler{store: s, posts: posts, engine: engine, site: site}
}

// Register 把 admin 路由挂到 r 上。
func (h *Handler) Register(r *gin.Engine) {
	g := r.Group("/admin")
	g.GET("", h.list)           // 文章列表
	g.GET("/", h.list)
	g.GET("/new", h.editForm)   // 新建表单
	g.GET("/edit/:slug", h.editForm) // 编辑表单
	g.POST("/save", h.save)     // 创建/更新
	g.POST("/delete/:slug", h.delete) // 删除
}

// list 渲染文章列表页。
func (h *Handler) list(c *gin.Context) {
	posts, err := h.store.List()
	if err != nil {
		c.String(http.StatusInternalServerError, "list posts: %v", err)
		return
	}
	rows := make([]*tmpl.AdminPostRow, 0, len(posts))
	for _, p := range posts {
		rows = append(rows, &tmpl.AdminPostRow{
			Slug:    p.Slug,
			Title:   p.Title,
			Excerpt: p.Excerpt,
			Status:  p.Status,
			Date:    p.CreatedAt.Format("2006-01-02 15:04"),
		})
	}
	data := &tmpl.ViewData{
		Site: h.site,
		Page: &tmpl.Page{Title: "文章管理", MenuID: "admin"},
		Admin: &tmpl.AdminData{Posts: rows},
	}
	h.render(c, "list", data)
}

// editForm 渲染新建/编辑表单。:slug 存在为编辑，否则为新建。
func (h *Handler) editForm(c *gin.Context) {
	slug := c.Param("slug")
	row := &tmpl.AdminPostRow{Status: "draft"}
	isEdit := false
	if slug != "" {
		p, err := h.store.GetBySlug(slug)
		if err != nil {
			c.String(http.StatusInternalServerError, "load post: %v", err)
			return
		}
		if p == nil {
			c.String(http.StatusNotFound, "post not found: %s", slug)
			return
		}
		row.Slug = p.Slug
		row.Title = p.Title
		row.Excerpt = p.Excerpt
		row.Cover = p.Cover
		row.ContentMD = p.ContentMD
		row.Status = p.Status
		isEdit = true
	}
	data := &tmpl.ViewData{
		Site: h.site,
		Page: &tmpl.Page{Title: ternary(isEdit, "编辑文章", "新建文章"), MenuID: "admin"},
		Admin: &tmpl.AdminData{EditPost: row, IsEdit: isEdit},
	}
	h.render(c, "edit", data)
}

// save 处理表单提交：slug 存在则更新，否则创建。服务端用 goldmark 渲染 content_html。
func (h *Handler) save(c *gin.Context) {
	slug := strings.TrimSpace(c.PostForm("slug"))
	title := strings.TrimSpace(c.PostForm("title"))
	excerpt := strings.TrimSpace(c.PostForm("excerpt"))
	cover := strings.TrimSpace(c.PostForm("cover"))
	contentMD := c.PostForm("content")
	status := c.PostForm("status")
	origSlug := c.PostForm("orig_slug") // 编辑时的原 slug

	if slug == "" {
		h.renderSaveError(c, origSlug, slug, title, excerpt, cover, contentMD, status, "slug 不能为空")
		return
	}
	if status != store.StatusDraft && status != store.StatusPublished {
		status = store.StatusDraft
	}

	// 服务端渲染 Markdown 为 HTML（复用 post.Service 的 goldmark 实例）
	contentHTML, err := h.posts.Render(contentMD)
	if err != nil {
		h.renderSaveError(c, origSlug, slug, title, excerpt, cover, contentMD, status, "Markdown 渲染失败: "+err.Error())
		return
	}

	// 编辑模式：orig_slug 非空且 slug 变了，需先删旧再建新（slug 是唯一键）
	if origSlug != "" && origSlug != slug {
		if _, err := h.store.GetBySlug(slug); err == nil {
			// 新 slug 已被占用
			h.renderSaveError(c, origSlug, slug, title, excerpt, cover, contentMD, status, "slug 已被其他文章占用: "+slug)
			return
		}
	}

	p := &store.Post{
		Slug:        slug,
		Title:       title,
		Excerpt:     excerpt,
		Cover:       cover,
		ContentMD:   contentMD,
		ContentHTML: contentHTML,
		Status:      status,
	}

	if origSlug != "" {
		// 更新（用 orig_slug 定位）
		p.Slug = slug
		if err := h.updateWithRename(origSlug, p); err != nil {
			h.renderSaveError(c, origSlug, slug, title, excerpt, cover, contentMD, status, "保存失败: "+err.Error())
			return
		}
	} else {
		if _, err := h.store.Create(p); err != nil {
			h.renderSaveError(c, origSlug, slug, title, excerpt, cover, contentMD, status, "创建失败: "+err.Error())
			return
		}
	}
	c.Redirect(http.StatusFound, "/admin")
}

// updateWithRename 处理 slug 变更：若 orig != new，先删旧记录再 Create 新记录（保留 created_at）。
func (h *Handler) updateWithRename(origSlug string, p *store.Post) error {
	if origSlug == p.Slug {
		return h.store.Update(origSlug, p)
	}
	existing, err := h.store.GetBySlug(origSlug)
	if err != nil {
		return err
	}
	if existing == nil {
		return h.store.Upsert(p)
	}
	// 保留原 created_at
	if err := h.store.Delete(origSlug); err != nil {
		return err
	}
	p.CreatedAt = existing.CreatedAt
	_, err = h.store.Create(p)
	return err
}

// delete 删除文章。
func (h *Handler) delete(c *gin.Context) {
	slug := c.Param("slug")
	if err := h.store.Delete(slug); err != nil {
		c.String(http.StatusInternalServerError, "delete: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/admin")
}

// renderSaveError 保存失败时回显表单数据并提示错误。
func (h *Handler) renderSaveError(c *gin.Context, origSlug, slug, title, excerpt, cover, contentMD, status, errMsg string) {
	row := &tmpl.AdminPostRow{
		Slug:      slug,
		Title:     title,
		Excerpt:   excerpt,
		Cover:     cover,
		ContentMD: contentMD,
		Status:    status,
	}
	data := &tmpl.ViewData{
		Site: h.site,
		Page: &tmpl.Page{Title: "编辑文章", MenuID: "admin"},
		Admin: &tmpl.AdminData{EditPost: row, IsEdit: origSlug != "", SaveError: errMsg},
	}
	h.render(c, "edit", data)
}

// render 渲染指定 admin 模板并写入响应。
func (h *Handler) render(c *gin.Context, name string, data *tmpl.ViewData) {
	html, err := h.engine.Render(name, data)
	if err != nil {
		c.String(http.StatusInternalServerError, "render %s: %v", name, err)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// Seed 扫描 content/posts/*.md 导入 DB（按 slug UPSERT），供 main.go -seed 调用。
func (h *Handler) Seed() error {
	slugs, err := h.posts.ListSlugs()
	if err != nil {
		return err
	}
	for _, slug := range slugs {
		meta, body, err := h.posts.ParseFile(slug)
		if err != nil {
			return err
		}
		html, err := h.posts.Render(body)
		if err != nil {
			return err
		}
		title := meta.Title
		if title == "" {
			title = slug
		}
		p := &store.Post{
			Slug:        slug,
			Title:       title,
			Excerpt:     meta.Excerpt,
			Cover:       meta.Cover,
			ContentMD:   body,
			ContentHTML: html,
			Status:      store.StatusPublished,
			CreatedAt:   parseSeedDate(meta.Date),
		}
		if err := h.store.Upsert(p); err != nil {
			return err
		}
	}
	return nil
}

// SeedCount 返回 seed 导入的文章数（仅日志用）。
func (h *Handler) SeedCount() (int, error) {
	slugs, err := h.posts.ListSlugs()
	if err != nil {
		return 0, err
	}
	return len(slugs), nil
}

// parseSeedDate 把 frontmatter 的 date 字段（如 2026-07-16）解析成 time.Time，失败用当前时间。
func parseSeedDate(s string) time.Time {
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Now()
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
