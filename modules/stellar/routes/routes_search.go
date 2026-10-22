package routes

import (
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
	r.GET("/api/search", s.serveStellarSearchAPI)
	r.GET("/search.json", s.serveStellarSearchJSON)
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
