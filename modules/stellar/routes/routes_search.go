package routes

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/search"
	"blog/internal/store"
	tmpl "blog/internal/template"
	"blog/internal/textutil"
)

// searchItem 本地搜索索引条目（/search.json）。
type searchItem struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Path    string `json:"path"`
}

// searchItemsFromPosts 已发布文章转本地搜索索引条目：受密码保护的文章不泄漏正文。
func searchItemsFromPosts(posts []*store.Post) []searchItem {
	include := true
	return searchItemsFromPostsWithConfig(posts, tmpl.SearchConfig{LocalSearch: tmpl.LocalSearchConfig{Content: &include}})
}

func searchItemsFromPostsWithConfig(posts []*store.Post, cfg tmpl.SearchConfig) []searchItem {
	items := make([]searchItem, 0, len(posts))
	for _, p := range posts {
		if cfg.LocalSearch.Field == "page" {
			continue
		}
		path := "/post/" + p.Slug
		if !searchPathAllowed(path, cfg) {
			continue
		}
		content := textutil.StripHTML(p.ContentHTML)
		if p.IsPasswordProtected() {
			content = "" // 受保护文章不泄漏正文
		}
		if cfg.LocalSearch.Content != nil && !*cfg.LocalSearch.Content {
			content = ""
		}
		items = append(items, searchItem{
			Title:   p.Title,
			Content: content,
			Path:    path,
		})
	}
	return items
}

func searchPathAllowed(path string, cfg tmpl.SearchConfig) bool {
	for _, blocked := range cfg.LocalSearch.SkipSearch {
		blocked = strings.TrimSpace(blocked)
		if blocked != "" && (path == blocked || strings.HasPrefix(path, strings.TrimRight(blocked, "/")+"/")) {
			return false
		}
	}
	return true
}

// registerSearchRoutes 注册搜索域路由：/api/search（FTS5）与 /search.json（前端本地匹配索引）。
func (s *webServer) registerSearchRoutes(r *gin.Engine) {
	// 服务端全文搜索（Spec 25 + 搜索范围扩展）：FTS5 bm25 排序 + snippet 高亮，
	// 覆盖文章/页面/瞬间三类（type 参数过滤，空串为全部）。
	// 前端 local-search.js 优先走此端点，失败降级回 /search.json 本地匹配。
	r.GET("/api/search", func(c *gin.Context) {
		if s.searchIdx == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "server search disabled"})
			return
		}
		q := strings.TrimSpace(c.Query("q"))
		docType := strings.TrimSpace(c.Query("type")) // post / page / moment，空 = 全部
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		results, err := s.searchIdx.Search(q, docType, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		type apiItem struct {
			Type    string `json:"type"` // post / page / moment
			Title   string `json:"title"`
			Path    string `json:"path"`
			Snippet string `json:"snippet"`
		}
		items := make([]apiItem, 0, len(results))
		for _, res := range results {
			items = append(items, apiItem{Type: res.Type, Title: res.Title, Path: searchPath(res.Type, res.Slug), Snippet: res.Snippet})
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{"items": items})
	})

	// 本地搜索索引：全量 published 文章的标题/正文纯文本，前端 local-search.js 拉取。
	// Spec 61 阶段 5：内存缓存（写路径置脏后重建）；响应仍 no-store，禁止浏览器/CDN 缓存。
	r.GET("/search.json", func(c *gin.Context) {
		data, err := s.contentCache.searchJSON()
		if err != nil {
			log.Printf("search index: %v", err)
			s.render500(c)
			return
		}
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "application/json; charset=utf-8", data)
	})
}

// searchPath 搜索结果路径：post/page 按 slug 拼接，moment 无详情页指向 /moments 时间流。
func searchPath(docType, slug string) string {
	switch docType {
	case search.TypePage:
		return "/pages/" + slug
	case search.TypeMoment:
		return "/moments"
	default:
		return "/post/" + slug
	}
}
