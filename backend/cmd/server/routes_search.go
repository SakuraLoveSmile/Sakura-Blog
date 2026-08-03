package main

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// searchItem 本地搜索索引条目（/search.json）。
type searchItem struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Path    string `json:"path"`
}

// searchItemsFromPosts 已发布文章转本地搜索索引条目：受密码保护的文章不泄漏正文。
func searchItemsFromPosts(posts []*store.Post) []searchItem {
	items := make([]searchItem, 0, len(posts))
	for _, p := range posts {
		content := stripHTML(p.ContentHTML)
		if p.IsPasswordProtected() {
			content = "" // 受保护文章不泄漏正文
		}
		items = append(items, searchItem{
			Title:   p.Title,
			Content: content,
			Path:    "/post/" + p.Slug,
		})
	}
	return items
}

// registerSearchRoutes 注册搜索域路由：/api/search（FTS5）与 /search.json（前端本地匹配索引）。
func (s *webServer) registerSearchRoutes(r *gin.Engine) {
	// 服务端全文搜索（Spec 25）：FTS5 bm25 排序 + snippet 高亮。
	// 前端 local-search.js 优先走此端点，失败降级回 /search.json 本地匹配。
	r.GET("/api/search", func(c *gin.Context) {
		if s.searchIdx == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "server search disabled"})
			return
		}
		q := strings.TrimSpace(c.Query("q"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		results, err := s.searchIdx.Search(q, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		type apiItem struct {
			Title   string `json:"title"`
			Path    string `json:"path"`
			Snippet string `json:"snippet"`
		}
		items := make([]apiItem, 0, len(results))
		for _, res := range results {
			items = append(items, apiItem{Title: res.Title, Path: "/post/" + res.Slug, Snippet: res.Snippet})
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{"items": items})
	})

	// 本地搜索索引：全量 published 文章的标题/正文纯文本，前端 local-search.js 拉取。
	// no-store：索引需实时反映 admin 改动，禁止浏览器/CDN 缓存。
	r.GET("/search.json", func(c *gin.Context) {
		posts, err := s.st.ListPublished()
		if err != nil {
			log.Printf("list posts: %v", err)
			s.render500(c)
			return
		}
		items := searchItemsFromPosts(posts)
		// wiki 文档并入索引：遍历所有项目加载已发布文档（Path=/wiki/<projectID>/<slug>）。
		// 只加本地索引（/search.json），不改 FTS（/api/search 仍只管文章）。
		for i := range s.site.Wiki {
			projectID := s.site.Wiki[i].ID
			docs, err := s.st.ListWikidocs(projectID)
			if err != nil {
				log.Printf("list wikidocs %s for search index: %v", projectID, err)
				continue // 查询失败跳过该项目，不影响其余索引
			}
			for _, d := range docs {
				if d.Status != store.StatusPublished {
					continue
				}
				content := truncateText(stripHTML(d.ContentHTML), 2000)
				items = append(items, searchItem{
					Title:   d.Title,
					Content: content,
					Path:    "/wiki/" + projectID + "/" + d.Slug,
				})
			}
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, items)
	})
}
