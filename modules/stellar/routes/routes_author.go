package routes

import (
	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"
	"log"
	"strconv"

	"github.com/gin-gonic/gin"
)

// handleAuthor 作者文章列表页（Spec 38 多作者体系）。
// 对齐 Stellar author.ejs：检查作者存在 → 分页查询文章 → 构建 ViewData → 渲染。
func (s *webServer) handleAuthor(c *gin.Context) {
	id := c.Param("id")
	// 检查作者是否存在
	author, ok := s.site.Load().Authors[id]
	if !ok {
		s.render404(c)
		return
	}
	// 分页参数
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	const pageSize = 10
	total, err := s.st.CountByAuthor(id)
	if err != nil {
		log.Printf("count posts by author %s: %v", id, err)
		s.render500(c)
		return
	}
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages > 0 && page > totalPages {
		page = totalPages
	}
	offset := (page - 1) * pageSize
	posts, err := s.st.ListByAuthor(id, offset, pageSize)
	if err != nil {
		log.Printf("list posts by author %s: %v", id, err)
		s.render500(c)
		return
	}
	pages := make([]int, totalPages)
	for i := range pages {
		pages[i] = i + 1
	}
	data := &tmpl.ViewData{
		Site: s.site.Load(),
		Page: &tmpl.Page{
			Title:  stellarweb.Lang("author.page_title", author.Name),
			Layout: "author",
			MenuID: "post",
			Lang:   "zh-CN",
			Author: id,
		},
		Posts:         s.toCards(posts),
		Paginator:     &tmpl.Paginator{Current: page, Total: totalPages, Pages: pages},
		AuthorProfile: &author,
	}
	s.fillSidebar(data)
	s.fillSEO(data, c)
	html, err := s.engine.Render("layout/base", data)
	if err != nil {
		log.Printf("render: %v", err)
		s.render500(c)
		return
	}
	c.Data(200, "text/html; charset=utf-8", []byte(html))
}
