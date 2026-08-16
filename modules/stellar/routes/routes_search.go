package routes

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/search"
	"blog/internal/store"
	"blog/internal/textutil"
	tmpl "blog/modules/stellar/viewmodel"
)

// searchItem 本地搜索索引条目（/search.json）。
// Anchors 为内容标题锚点（1.41 搜索章节化）：前端按 offset 区间把命中拆到章节、
// 跳转带 #锚点；空数组（无标题内容）回退页面级匹配，向后兼容。
type searchItem struct {
	Title   string            `json:"title"`
	Content string            `json:"content"`
	Path    string            `json:"path"`
	Anchors []textutil.Anchor `json:"anchors,omitempty"`
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
		content, anchors := textutil.StripHTMLWithAnchors(p.ContentHTML)
		if p.IsPasswordProtected() {
			content = "" // 受保护文章不泄漏正文
			anchors = nil
		}
		if cfg.LocalSearch.Content != nil && !*cfg.LocalSearch.Content {
			content = ""
			anchors = nil
		}
		items = append(items, searchItem{
			Title:   p.Title,
			Content: content,
			Path:    path,
			Anchors: anchors,
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
			// 章节锚点（1.41）：命中内容所在标题的 id/文本，前端拼 #锚点直达章节
			AnchorID   string `json:"anchor_id,omitempty"`
			AnchorText string `json:"anchor_text,omitempty"`
		}
		terms := strings.Fields(strings.ToLower(q))
		items := make([]apiItem, 0, len(results))
		for _, res := range results {
			it := apiItem{Type: res.Type, Title: res.Title, Path: searchPath(res.Type, res.Slug), Snippet: res.Snippet}
			if id, text := s.resultAnchor(res.Type, res.Slug, terms); id != "" || text != "" {
				it.AnchorID, it.AnchorText = id, text
			}
			items = append(items, it)
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

// resultAnchor 服务端搜索结果的章节锚点（1.41）：加载命中文档内容，按首个命中词
// 位置映射所属标题锚点。moment 无正文章节，直接无锚点；查询失败静默降级（跳转仍可用）。
func (s *webServer) resultAnchor(docType, slug string, terms []string) (id, text string) {
	if len(terms) == 0 {
		return "", ""
	}
	var contentHTML string
	switch docType {
	case search.TypePost:
		if p, err := s.st.GetPublishedBySlug(slug); err == nil && p != nil && !p.IsPasswordProtected() {
			contentHTML = p.ContentHTML
		}
	case search.TypePage:
		if p, err := s.st.GetPageBySlug(slug); err == nil && p != nil && p.Status == store.StatusPublished {
			contentHTML = p.ContentHTML
		}
	default:
		return "", ""
	}
	if contentHTML == "" {
		return "", ""
	}
	content, anchors := textutil.StripHTMLWithAnchors(contentHTML)
	if len(anchors) == 0 {
		return "", ""
	}
	lower := strings.ToLower(content)
	first := -1
	for _, t := range terms {
		if i := strings.Index(lower, t); i >= 0 && (first < 0 || i < first) {
			first = i
		}
	}
	if first < 0 {
		return "", ""
	}
	u16 := textutil.Utf16Len(content[:first])
	var hit *textutil.Anchor
	for i := range anchors {
		if anchors[i].Offset <= u16 {
			hit = &anchors[i]
		} else {
			break
		}
	}
	if hit == nil {
		return "", ""
	}
	return hit.ID, hit.Text
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
