package moment

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
	tmpl "blog/internal/template"
)

const adminPageSize = 20

// adminList GET /admin/moments：列表 + 发布框。
func (h *Handler) adminList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	total, err := h.store.CountMoments()
	if err != nil {
		c.String(http.StatusInternalServerError, "count moments: %v", err)
		return
	}
	totalPages := (total + adminPageSize - 1) / adminPageSize
	if totalPages > 0 && page > totalPages {
		page = totalPages
	}
	moments, err := h.store.ListMoments(page, adminPageSize)
	if err != nil {
		c.String(http.StatusInternalServerError, "list moments: %v", err)
		return
	}

	rows := make([]*tmpl.AdminMomentRow, 0, len(moments))
	for _, m := range moments {
		text := stripHTML(m.ContentHTML)
		if text == "" {
			text = strings.TrimSpace(m.ContentMD)
		}
		imgs := parseImages(m.Images)
		rows = append(rows, &tmpl.AdminMomentRow{
			ID:         m.ID,
			Excerpt:    excerpt(text, 80),
			ImageCount: len(imgs),
			Date:       m.CreatedAt.Format("2006-01-02 15:04"),
		})
	}
	pages := make([]int, totalPages)
	for i := range pages {
		pages[i] = i + 1
	}

	data := &tmpl.AdminMomentData{
		Site:        h.site,
		Page:        &tmpl.Page{Title: "瞬间", MenuID: "moments"},
		Items:       rows,
		CurrentPage: page,
		Pages:       pages,
		CreateError: c.Query("error"),
	}
	html, err := h.engine.RenderMomentAdmin(data)
	if err != nil {
		c.String(http.StatusInternalServerError, "render admin/moments: %v", err)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// create POST /admin/moments/create：发布瞬间。
func (h *Handler) create(c *gin.Context) {
	contentMD := strings.TrimSpace(c.PostForm("content"))
	imagesRaw := strings.TrimSpace(c.PostForm("images"))
	if contentMD == "" && imagesRaw == "" {
		c.Redirect(http.StatusSeeOther, "/admin/moments?error="+url.QueryEscape("正文与图片不能同时为空"))
		return
	}

	contentHTML, err := h.posts.Render(contentMD)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/admin/moments?error="+url.QueryEscape(fmt.Sprintf("渲染失败: %v", err)))
		return
	}
	images := joinImages(parseImages(imagesRaw))

	m := &store.Moment{
		ContentMD:   contentMD,
		ContentHTML: contentHTML,
		Images:      images,
		CreatedAt:   time.Now(),
	}
	if _, err := h.store.CreateMoment(m); err != nil {
		c.Redirect(http.StatusSeeOther, "/admin/moments?error="+url.QueryEscape("保存失败"))
		return
	}
	c.Redirect(http.StatusSeeOther, "/admin/moments")
}

// delete POST /admin/moments/delete/:id：删除瞬间。
func (h *Handler) delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.String(http.StatusBadRequest, "invalid moment id")
		return
	}
	if err := h.store.DeleteMoment(id); err != nil {
		c.String(http.StatusInternalServerError, "delete: %v", err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/admin/moments")
}
