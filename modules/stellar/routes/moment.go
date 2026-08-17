// MomentHandler owns the public moments route and its reusable query view.
package routes

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/gin-gonic/gin"

	"blog/internal/comments"
	"blog/internal/post"
	tmpl "blog/internal/settings"
	"blog/internal/store"
)

const pageSize = 20

// Handler 瞬间请求处理器。
type MomentHandler struct {
	store  *store.Store
	posts  *post.Service
	engine MomentRenderer
	// site 站点配置原子指针（Spec 61 阶段 3）
	site        *atomic.Pointer[tmpl.Site]
	fillSidebar func(*tmpl.ViewData)
	fillSEO     func(*tmpl.ViewData, *gin.Context)
}

// Renderer is the narrow view boundary consumed by the internal moment
// service; the concrete Engine belongs to modules/stellar/web.
type MomentRenderer interface {
	Render(string, *tmpl.ViewData) (string, error)
}

// New 创建 moment handler，确保 moments 表存在。
func NewMomentHandler(
	s *store.Store,
	posts *post.Service,
	engine MomentRenderer,
	site *atomic.Pointer[tmpl.Site],
	fillSidebar func(*tmpl.ViewData),
	fillSEO func(*tmpl.ViewData, *gin.Context),
) (*MomentHandler, error) {
	if err := s.EnsureMomentsTable(); err != nil {
		return nil, err
	}
	return &MomentHandler{
		store:       s,
		posts:       posts,
		engine:      engine,
		site:        site,
		fillSidebar: fillSidebar,
		fillSEO:     fillSEO,
	}, nil
}

// Register 挂前台 /moments 路由（后台管理已迁移到 Vue Console + /api/v1）。
func (h *MomentHandler) Register(r *gin.Engine) {
	r.GET("/moments", h.listPublic)
	r.GET("/moments/", h.listPublic)
}

// listPublic GET /moments：时间流 + 分页（20/页）。
func (h *MomentHandler) listPublic(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	total, err := h.store.CountPublicMoments()
	if err != nil {
		c.String(http.StatusInternalServerError, "count moments: %v", err)
		return
	}
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages > 0 && page > totalPages {
		page = totalPages
	}
	moments, err := h.store.ListPublicMoments(page, pageSize)
	if err != nil {
		c.String(http.StatusInternalServerError, "list moments: %v", err)
		return
	}

	items := make([]*tmpl.MomentItem, 0, len(moments))
	for _, m := range moments {
		items = append(items, &tmpl.MomentItem{
			ID:          m.ID,
			ContentHTML: template.HTML(m.ContentHTML),
			Images:      store.DecodeMomentImages(m.Images),
			TimeAgo:     comments.TimeAgo(m.CreatedAt),
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
		Site: h.site.Load(),
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

// RecentItems 返回最新 limit 条公开 moments 的视图模型，供其它页面（如 /notes 近期动态）内联复用。
func (h *MomentHandler) RecentItems(limit int) []*tmpl.MomentItem {
	moments, err := h.store.ListPublicMoments(1, limit)
	if err != nil {
		return nil
	}
	items := make([]*tmpl.MomentItem, 0, len(moments))
	for _, m := range moments {
		items = append(items, &tmpl.MomentItem{
			ID:          m.ID,
			ContentHTML: template.HTML(m.ContentHTML),
			Images:      store.DecodeMomentImages(m.Images),
			TimeAgo:     comments.TimeAgo(m.CreatedAt),
			Date:        m.CreatedAt.Format("2006-01-02 15:04"),
		})
	}
	return items
}

func joinImages(urls []string) string {
	return strings.Join(urls, ",")
}
