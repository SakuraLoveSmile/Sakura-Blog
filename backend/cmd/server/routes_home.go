package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	tmpl "blog/internal/template"
)

// registerHomeRoutes 注册首页域路由：/ 与 /page/:num。
func (s *webServer) registerHomeRoutes(r *gin.Engine) {
	// 首页：文章列表（对齐 Stellar layout/index.ejs）
	r.GET("/", func(c *gin.Context) {
		s.renderHome(c, 1)
	})
	// 首页分页：/page/N，非数字或 <1 返回 404
	r.GET("/page/:num", func(c *gin.Context) {
		n, err := strconv.Atoi(c.Param("num"))
		if err != nil || n < 1 {
			s.render404(c)
			return
		}
		s.renderHome(c, n)
	})
}

// renderHome 首页渲染：/ 与 /page/:num 共用。page 从 1 开始，超出末页回退到最后一页。
func (s *webServer) renderHome(c *gin.Context, page int) {
	const pageSize = 10
	total, err := s.st.CountPublished()
	if err != nil {
		log.Printf("count posts: %v", err)
		s.render500(c)
		return
	}
	totalPages := (total + pageSize - 1) / pageSize
	if page < 1 {
		page = 1
	}
	if totalPages > 0 && page > totalPages {
		page = totalPages
	}
	posts, err := s.st.ListPublishedPaged(page, pageSize)
	if err != nil {
		log.Printf("list posts: %v", err)
		s.render500(c)
		return
	}
	pages := make([]int, totalPages)
	for i := range pages {
		pages[i] = i + 1
	}
	data := &tmpl.ViewData{
		Site:      s.site,
		Page:      &tmpl.Page{Title: s.site.Title, Layout: "index", MenuID: "post", Lang: "zh-CN", Type: s.site.Article.Type},
		Posts:     s.toCards(posts),
		Paginator: &tmpl.Paginator{Current: page, Total: totalPages, Pages: pages},
	}
	s.fillSidebar(data)
	// 1.38 site_tree.home.comments：首页可配置评论区（默认关），page_key 用 "/"
	// 对齐上游 index.ejs 的 is_home_first_page()：仅首页第一页填充，/page/N 不渲染评论区
	// 仅首页第一页且路径精确为 / 时填充评论区，/page/1 与越界钳制页不填充
	if page == 1 && c.Request.URL.Path == "/" && s.site.Comments.Enabled && s.site.SiteTree.Home.Comments {
		s.fillComments(data, "/")
	}
	s.fillSEO(data, c)
	html, err := s.engine.Render("layout/base", data)
	if err != nil {
		log.Printf("render: %v", err)
		s.render500(c)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}
