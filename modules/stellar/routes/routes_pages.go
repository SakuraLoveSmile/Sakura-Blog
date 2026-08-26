package routes

import (
	"blog/internal/store"
	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"
	"html/template"

	"github.com/gin-gonic/gin"
)

// handleLinksPage 友链卡片页（内容页化）：渲染 pages 表 slug=links 系统页。
// 正文 {% links %} 服务端直出分组友链卡片；banner/TOC 同内容页。
func (s *webServer) handleLinksPage(c *gin.Context) {
	p, err := s.st.GetPageBySlug(LinksSlug)
	if err != nil || p == nil || p.Status != store.StatusPublished {
		s.render404(c)
		return
	}
	contentHTML, err := renderStoredContent(s.posts, p.ContentMD, p.ContentHTML)
	if err != nil {
		s.render500(c)
		return
	}
	data := &tmpl.ViewData{
		Site:    s.site.Load(),
		Page:    &tmpl.Page{Title: s.menuTitle("friends", "友链"), Layout: "page", MenuID: p.Slug, Lang: "zh-CN", Slug: p.Slug},
		Content: template.HTML(contentHTML),
	}
	data.TOC = stellarweb.ExtractTOCWith(contentHTML, s.tocOptions())
	s.renderBase(c, data)
}

// menuTitle 页面标题与菜单配置联动：取 menubar 中 ID==menuID 的菜单项标题，
// 未配置时回退 fallback（避免标题硬编码与菜单改名脱节）。
func (s *webServer) menuTitle(menuID, fallback string) string {
	for _, it := range s.site.Load().Menubar.Items {
		if it.ID == menuID && it.Title != "" {
			return it.Title
		}
	}
	return fallback
}

// handleFriendsPage 友链页（内容页化）：渲染 pages 表 slug=friends 系统页。
// 正文 md 控制台可编辑（{% friends posts:true %} 嵌入友链组件）；
// banner 标题/封面/副标题取 pages.friends 设置（缺省回退页面标题）；TOC 与评论同内容页。
func (s *webServer) handleFriendsPage(c *gin.Context) {
	p, err := s.st.GetPageBySlug(FriendsSlug)
	if err != nil || p == nil || p.Status != store.StatusPublished {
		s.render404(c)
		return
	}
	contentHTML, err := renderStoredContent(s.posts, p.ContentMD, p.ContentHTML)
	if err != nil {
		s.render500(c)
		return
	}
	title := s.site.Load().Pages.Friends.Title
	if title == "" {
		title = p.Title
	}
	pg := &tmpl.Page{Title: title, Layout: "page", MenuID: "friends", Lang: "zh-CN", Slug: p.Slug}
	if hero := s.site.Load().Pages.Friends.Hero; hero != nil {
		pg.Banner = hero.Background
		pg.Subtitle = hero.Description
	}
	data := &tmpl.ViewData{Site: s.site.Load(), Page: pg, Content: template.HTML(contentHTML)}
	data.TOC = stellarweb.ExtractTOCWith(contentHTML, s.tocOptions())
	s.fillComments(data, "/friends")
	s.renderBase(c, data)
}
