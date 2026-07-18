package page

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
	tmpl "blog/internal/template"
)

// RegisterAdmin 挂 /admin/pages 路由组，authMW 保护全部接口。
func (h *Handler) RegisterAdmin(r *gin.Engine, authMW gin.HandlerFunc) {
	g := r.Group("/admin/pages", authMW)
	g.GET("", h.list)
	g.GET("/", h.list)
	g.GET("/new", h.editForm)
	g.GET("/edit/:slug", h.editForm)
	g.POST("/save", h.save)
	g.POST("/delete/:slug", h.delete)
}

// list GET /admin/pages：页面列表。
func (h *Handler) list(c *gin.Context) {
	pages, err := h.store.ListPages()
	if err != nil {
		c.String(http.StatusInternalServerError, "list pages: %v", err)
		return
	}
	rows := make([]*tmpl.AdminPageRow, 0, len(pages))
	for _, p := range pages {
		rows = append(rows, &tmpl.AdminPageRow{
			Slug:   p.Slug,
			Title:  p.Title,
			Status: p.Status,
			Date:   p.UpdatedAt.Format("2006-01-02 15:04"),
		})
	}
	data := &tmpl.AdminPageData{
		Site:  h.site,
		Page:  &tmpl.Page{Title: "页面管理", MenuID: "pages"},
		Items: rows,
	}
	h.renderAdmin(c, "admin/page_list", data)
}

// editForm GET /admin/pages/new 或 /admin/pages/edit/:slug。
func (h *Handler) editForm(c *gin.Context) {
	slug := c.Param("slug")
	row := &tmpl.AdminPageRow{Status: store.StatusDraft}
	isEdit := false
	if slug != "" {
		p, err := h.store.GetPageBySlug(slug)
		if err != nil {
			c.String(http.StatusInternalServerError, "load page: %v", err)
			return
		}
		if p == nil {
			c.String(http.StatusNotFound, "page not found: %s", slug)
			return
		}
		row.Slug = p.Slug
		row.Title = p.Title
		row.ContentMD = p.ContentMD
		row.Status = p.Status
		isEdit = true
	}
	title := "新建页面"
	if isEdit {
		title = "编辑页面"
	}
	data := &tmpl.AdminPageData{
		Site:     h.site,
		Page:     &tmpl.Page{Title: title, MenuID: "pages"},
		EditPage: row,
		IsEdit:   isEdit,
	}
	h.renderAdmin(c, "admin/page_edit", data)
}

// save POST /admin/pages/save：创建或更新页面。
func (h *Handler) save(c *gin.Context) {
	slug := strings.TrimSpace(c.PostForm("slug"))
	title := strings.TrimSpace(c.PostForm("title"))
	contentMD := c.PostForm("content")
	status := c.PostForm("status")
	origSlug := c.PostForm("orig_slug")

	if slug == "" {
		h.renderSaveError(c, origSlug, slug, title, contentMD, status, "slug 不能为空")
		return
	}
	if status != store.StatusDraft && status != store.StatusPublished {
		status = store.StatusDraft
	}

	contentHTML, err := h.posts.Render(contentMD)
	if err != nil {
		h.renderSaveError(c, origSlug, slug, title, contentMD, status, "Markdown 渲染失败: "+err.Error())
		return
	}

	if origSlug != "" && origSlug != slug {
		if existing, err := h.store.GetPageBySlug(slug); err != nil {
			h.renderSaveError(c, origSlug, slug, title, contentMD, status, "保存失败: "+err.Error())
			return
		} else if existing != nil {
			h.renderSaveError(c, origSlug, slug, title, contentMD, status, "slug 已被其他页面占用: "+slug)
			return
		}
	}

	p := &store.Page{
		Slug:        slug,
		Title:       title,
		ContentMD:   contentMD,
		ContentHTML: contentHTML,
		Status:      status,
	}

	if origSlug != "" {
		if err := h.updateWithRename(origSlug, p); err != nil {
			h.renderSaveError(c, origSlug, slug, title, contentMD, status, "保存失败: "+err.Error())
			return
		}
	} else {
		if _, err := h.store.CreatePage(p); err != nil {
			h.renderSaveError(c, origSlug, slug, title, contentMD, status, "创建失败: "+err.Error())
			return
		}
	}
	c.Redirect(http.StatusFound, "/admin/pages")
}

func (h *Handler) updateWithRename(origSlug string, p *store.Page) error {
	if origSlug == p.Slug {
		return h.store.UpdatePage(origSlug, p)
	}
	existing, err := h.store.GetPageBySlug(origSlug)
	if err != nil {
		return err
	}
	if existing == nil {
		_, err = h.store.CreatePage(p)
		return err
	}
	if err := h.store.DeletePage(origSlug); err != nil {
		return err
	}
	p.CreatedAt = existing.CreatedAt
	_, err = h.store.CreatePage(p)
	return err
}

// delete POST /admin/pages/delete/:slug。
func (h *Handler) delete(c *gin.Context) {
	slug := c.Param("slug")
	if err := h.store.DeletePage(slug); err != nil {
		c.String(http.StatusInternalServerError, "delete: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/admin/pages")
}

func (h *Handler) renderSaveError(c *gin.Context, origSlug, slug, title, contentMD, status, errMsg string) {
	data := &tmpl.AdminPageData{
		Site: h.site,
		Page: &tmpl.Page{Title: "编辑页面", MenuID: "pages"},
		EditPage: &tmpl.AdminPageRow{
			Slug:      slug,
			Title:     title,
			ContentMD: contentMD,
			Status:    status,
		},
		IsEdit:    origSlug != "",
		SaveError: errMsg,
	}
	h.renderAdmin(c, "admin/page_edit", data)
}

func (h *Handler) renderAdmin(c *gin.Context, name string, data *tmpl.AdminPageData) {
	html, err := h.engine.RenderPageAdmin(name, data)
	if err != nil {
		c.String(http.StatusInternalServerError, "render %s: %v", name, err)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}
