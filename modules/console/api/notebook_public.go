package api

import (
	"net/url"
	"strings"

	tmpl "blog/internal/settings"
	"blog/internal/stellarv2"
	"blog/internal/store"

	"github.com/gin-gonic/gin"
)

// notebookNoteTotal 是一个笔记本里当前用户可管理的笔记数量。
type notebookNoteTotal struct {
	total     int
	published int
}

// notebookNoteTotals 按当前用户的管理权限统计每个笔记本的笔记数量，不加载正文。
func (h *Handler) notebookNoteTotals(c *gin.Context) (map[string]notebookNoteTotal, error) {
	rows, err := h.store.CountNotes()
	if err != nil {
		return nil, err
	}
	totals := map[string]notebookNoteTotal{}
	for _, row := range rows {
		if !notebookOwned(c, row.AuthorID) {
			continue
		}
		total := totals[row.NotebookID]
		total.total += row.Count
		if row.Status == store.StatusPublished {
			total.published += row.Count
		}
		totals[row.NotebookID] = total
	}
	return totals, nil
}

// notebookView 在基础响应上补充前台路径与笔记数量。
func (h *Handler) notebookView(notebook *store.Notebook, totals map[string]notebookNoteTotal) NotebookResponse {
	out := notebookResponse(notebook)
	out.PublicPath = h.notebookPublicPath(notebook)
	out.NoteCount = totals[notebook.ID].total
	out.PublishedCount = totals[notebook.ID].published
	return out
}

// respondNotebook 写出带前台路径和笔记数量的笔记本响应。
func (h *Handler) respondNotebook(c *gin.Context, status int, notebook *store.Notebook) {
	totals, err := h.notebookNoteTotals(c)
	if err != nil {
		notebookError(c, err)
		return
	}
	c.JSON(status, h.notebookView(notebook, totals))
}

// noteView 在基础响应上补充已发布笔记的前台路径。
func (h *Handler) noteView(notebook *store.Notebook, note *store.Note) NotebookNoteResponse {
	out := notebookNoteResponse(note)
	out.PublicPath = h.notePublicPath(notebook, note)
	return out
}

// loadSite 返回当前站点快照；测试或最小装配没有注入时为 nil。
func (h *Handler) loadSite() *tmpl.Site {
	if h.site == nil {
		return nil
	}
	return h.site.Load()
}

// sitePublicPath 把站内路径换成浏览器可直接访问的路径：带上站点 URL 自带的路径前缀，保留原有转义。
func sitePublicPath(site *tmpl.Site, path string) string {
	parsed, err := url.Parse(tmpl.JoinSiteURL(site.URL, path))
	if err != nil {
		return path
	}
	return parsed.EscapedPath()
}

// notebookBasePath 与前台路由共用 stellarv2.CollectionBasePath，返回笔记本集合的站内路径。
// 站点还没有 Stellar 2.0 主题快照（迁移前）时 ok 为 false，Console 据此不展示前台入口。
func (h *Handler) notebookBasePath(notebook *store.Notebook) (site *tmpl.Site, base string, ok bool) {
	site = h.loadSite()
	if site == nil || site.StellarV2 == nil {
		return nil, "", false
	}
	parsed, err := notebook.ValidateStellar()
	if err != nil {
		return nil, "", false
	}
	return site, stellarv2.CollectionBasePath(site.StellarV2.Theme, "notebooks", notebook.ID, notebookText(parsed.Value, "route.path", "")), true
}

// notebookPublicPath 返回笔记本在前台的首页路径；拿不到时为空串。
func (h *Handler) notebookPublicPath(notebook *store.Notebook) string {
	site, base, ok := h.notebookBasePath(notebook)
	if !ok {
		return ""
	}
	return sitePublicPath(site, base)
}

// notePublicPath 只给已发布笔记返回前台路径，草稿没有公开页面；笔记配置了 permalink 时与前台一样优先使用它。
func (h *Handler) notePublicPath(notebook *store.Notebook, note *store.Note) string {
	if note.Status != store.StatusPublished {
		return ""
	}
	site, base, ok := h.notebookBasePath(notebook)
	if !ok {
		return ""
	}
	parsed, err := note.ValidateStellar()
	if err != nil {
		return ""
	}
	if permalink := notebookText(parsed.Value, "permalink", ""); permalink != "" {
		target, err := url.Parse(permalink)
		if err != nil {
			return ""
		}
		return sitePublicPath(site, "/"+strings.Trim(target.EscapedPath(), "/"))
	}
	return sitePublicPath(site, base+"/"+url.PathEscape(note.Slug))
}
