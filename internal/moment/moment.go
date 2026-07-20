// Package moment 瞬间/说说：前台时间流与后台发布管理。
package moment

import (
	"html/template"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/comment"
	"blog/internal/post"
	"blog/internal/store"
	tmpl "blog/internal/template"
)

const pageSize = 20

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)
var whitespaceRe = regexp.MustCompile(`\s+`)

// Handler 瞬间请求处理器。
type Handler struct {
	store       *store.Store
	posts       *post.Service
	engine      *tmpl.Engine
	site        *tmpl.Site
	fillSidebar func(*tmpl.ViewData)
	fillSEO     func(*tmpl.ViewData, *gin.Context)
}

// New 创建 moment handler，确保 moments 表存在。
func New(
	s *store.Store,
	posts *post.Service,
	engine *tmpl.Engine,
	site *tmpl.Site,
	fillSidebar func(*tmpl.ViewData),
	fillSEO func(*tmpl.ViewData, *gin.Context),
) (*Handler, error) {
	if err := s.EnsureMomentsTable(); err != nil {
		return nil, err
	}
	return &Handler{
		store:       s,
		posts:       posts,
		engine:      engine,
		site:        site,
		fillSidebar: fillSidebar,
		fillSEO:     fillSEO,
	}, nil
}

// Register 挂前台 /moments 与后台 /admin/moments 路由。
func (h *Handler) Register(r *gin.Engine, authMW gin.HandlerFunc) {
	r.GET("/moments", h.listPublic)
	r.GET("/moments/", h.listPublic)

	g := r.Group("/admin/moments", authMW)
	g.GET("", h.adminList)
	g.GET("/", h.adminList)
	g.POST("/create", h.create)
	g.POST("/delete/:id", h.delete)
}

// listPublic GET /moments：时间流 + 分页（20/页）。
func (h *Handler) listPublic(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	total, err := h.store.CountMoments()
	if err != nil {
		c.String(http.StatusInternalServerError, "count moments: %v", err)
		return
	}
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages > 0 && page > totalPages {
		page = totalPages
	}
	moments, err := h.store.ListMoments(page, pageSize)
	if err != nil {
		c.String(http.StatusInternalServerError, "list moments: %v", err)
		return
	}

	items := make([]*tmpl.MomentItem, 0, len(moments))
	for _, m := range moments {
		items = append(items, &tmpl.MomentItem{
			ID:          m.ID,
			ContentHTML: template.HTML(m.ContentHTML),
			Images:      parseImages(m.Images),
			TimeAgo:     comment.TimeAgo(m.CreatedAt),
			Date:        m.CreatedAt.Format("2006-01-02 15:04"),
		})
	}

	var paginator *tmpl.Paginator
	if totalPages > 1 {
		pages := make([]int, totalPages)
		for i := range pages {
			pages[i] = i + 1
		}
		paginator = &tmpl.Paginator{Current: page, Total: totalPages, Pages: pages}
	}

	data := &tmpl.ViewData{
		Site: h.site,
		Page: &tmpl.Page{
			Title:  "瞬间",
			Layout: "moments",
			Type:   "story",
			Lang:   "zh-CN",
			MenuID: "moments",
		},
		Moments:   items,
		Paginator: paginator,
	}
	if h.fillSidebar != nil {
		h.fillSidebar(data)
	}
	if h.fillSEO != nil {
		h.fillSEO(data, c)
	}
	html, err := h.engine.Render("layout/base", data)
	if err != nil {
		c.String(http.StatusInternalServerError, "render moments: %v", err)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// RecentItems 返回最新 limit 条 moments 的视图模型，供其它页面（如 /notes 近期动态）内联复用。
func (h *Handler) RecentItems(limit int) []*tmpl.MomentItem {
	moments, err := h.store.ListMoments(1, limit)
	if err != nil {
		return nil
	}
	items := make([]*tmpl.MomentItem, 0, len(moments))
	for _, m := range moments {
		items = append(items, &tmpl.MomentItem{
			ID:          m.ID,
			ContentHTML: template.HTML(m.ContentHTML),
			Images:      parseImages(m.Images),
			TimeAgo:     comment.TimeAgo(m.CreatedAt),
			Date:        m.CreatedAt.Format("2006-01-02 15:04"),
		})
	}
	return items
}

func parseImages(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func joinImages(urls []string) string {
	return strings.Join(urls, ",")
}

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(whitespaceRe.ReplaceAllString(s, " "))
}

func excerpt(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
