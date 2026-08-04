package api

import (
	"html"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// 管理端全局搜索（Spec 39）：GET /api/v1/search?q=&type=，Console ⌘K 弹窗消费。
//
// 与公开端点 /api/search 的区别：公开端点走 FTS5 索引，而索引只收录已发布
// 文章/页面与公开瞬间（写路径 hook 会主动移除草稿/回收站/定时未到的条目），无法覆盖管理端
// 「草稿与回收站也可搜」的需求。因此管理端直接对 store 全量扫描做多词 AND 子串匹配
// （个人博客量级性能足够），匹配语义对齐公开端点：按空白拆词、词间 AND、CJK 天然子串命中。
// type 参数限定范围（post/page/moment，空 = 全部），与公开端点过滤语义一致。
// 端点挂在鉴权中间件之后，受保护文章正文对已登录管理员可搜，不影响公开语义。

// searchResultItem 单条管理端搜索结果。Snippet 为 HTML 片段（<mark> 高亮，其余已转义）。
type searchResultItem struct {
	ID      int64  `json:"id"`
	Type    string `json:"type"` // post / page / moment
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Status  string `json:"status"` // published / draft / trashed；moment 为 public / private
	Snippet string `json:"snippet"`
}

// adminSearch GET /api/v1/search?q=&type=&limit= 全状态搜索文章+页面+瞬间。
func (h *Handler) adminSearch(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	docType := c.Query("type") // post / page / moment，空 = 全部
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	terms := searchTerms(q)
	if len(terms) == 0 {
		c.JSON(http.StatusOK, gin.H{"items": []searchResultItem{}})
		return
	}

	// 标题命中排在正文命中之前（近似公开端点标题加权的排序语义）
	var titleHits, contentHits []searchResultItem
	collect := func(item searchResultItem, title, content string) {
		ok, inTitle := matchSearchDoc(title, content, terms)
		if !ok {
			return
		}
		item.Snippet = buildSearchSnippet(content, terms)
		if inTitle {
			titleHits = append(titleHits, item)
		} else {
			contentHits = append(contentHits, item)
		}
	}

	if docType == "" || docType == "post" {
		posts, err := h.store.List()
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		trashedPosts, err := h.store.ListTrashed()
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		for _, p := range append(posts, trashedPosts...) {
			collect(searchResultItem{
				ID: p.ID, Type: "post", Slug: p.Slug, Title: p.Title, Status: docStatus(p.Status, p.DeletedAt),
			}, p.Title, plainContent(p.ContentHTML))
		}
	}

	if docType == "" || docType == "page" {
		pages, err := h.store.ListPages()
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		trashedPages, err := h.store.ListTrashedPages()
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		for _, p := range append(pages, trashedPages...) {
			collect(searchResultItem{
				ID: p.ID, Type: "page", Slug: p.Slug, Title: p.Title, Status: docStatus(p.Status, p.DeletedAt),
			}, p.Title, plainContent(p.ContentHTML))
		}
	}

	if docType == "" || docType == "moment" {
		// 瞬间无标题/状态/回收站：标题用正文截断，状态取可见性（公开/私密均可搜，对齐全状态语义）
		// 查询失败跳过（moments 表由瞬间模块 Ensure，未启用时不应让整个搜索 500）
		for page := 1; ; page++ {
			moments, err := h.store.ListMoments(page, 50)
			if err != nil {
				break
			}
			if len(moments) == 0 {
				break
			}
			for _, m := range moments {
				content := plainContent(m.ContentHTML)
				collect(searchResultItem{
					ID: m.ID, Type: "moment", Slug: store.MomentSlugKey(m.ID),
					Title: truncateRunes(content, 40), Status: m.Visibility,
				}, truncateRunes(content, 40), content)
			}
			if len(moments) < 50 {
				break
			}
		}
	}

	items := append(titleHits, contentHits...)
	if len(items) > limit {
		items = items[:limit]
	}
	if items == nil {
		items = []searchResultItem{}
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// truncateRunes 按 rune 截断字符串，超长加省略号（瞬间无标题，截断正文充当）。
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// plainContent HTML 正文 → 纯文本：去标签后再解码实体（如 &#34;），
// 避免 snippet 构建时二次转义成 &amp;#34; 漏给前端。
func plainContent(contentHTML string) string {
	return html.UnescapeString(stripHTML(contentHTML))
}

// docStatus 归一化状态：软删除 → trashed（对齐 Console 列表筛选值），否则原状态。
func docStatus(status, deletedAt string) string {
	if deletedAt != "" {
		return "trashed"
	}
	if status == "" {
		return store.StatusDraft
	}
	return status
}

// searchTerms 按空白拆词并小写化；空查询返回 nil。
func searchTerms(q string) []string {
	fields := strings.Fields(q)
	terms := make([]string, 0, len(fields))
	for _, f := range fields {
		terms = append(terms, strings.ToLower(f))
	}
	if len(terms) == 0 {
		return nil
	}
	return terms
}

// matchSearchDoc 判断标题/正文是否命中全部词（AND 语义）；inTitle 表示至少一词命中标题（排序用）。
func matchSearchDoc(title, content string, terms []string) (ok, inTitle bool) {
	lowerTitle := strings.ToLower(title)
	lowerContent := strings.ToLower(content)
	for _, t := range terms {
		hitTitle := strings.Contains(lowerTitle, t)
		if hitTitle {
			inTitle = true
		}
		if !hitTitle && !strings.Contains(lowerContent, t) {
			return false, false
		}
	}
	return true, inTitle
}

// snippet 窗口尺寸（rune）：命中位置前保留 contextBefore，总长 snippetLen。
const (
	snippetContextBefore = 20
	snippetLen           = 80
)

// buildSearchSnippet 提取首个命中词附近文本窗口，HTML 转义后用 <mark> 高亮全部命中词。
// 正文无命中（仅标题命中）时返回开头截断片段。
func buildSearchSnippet(content string, terms []string) string {
	if content == "" {
		return ""
	}
	lower := strings.ToLower(content)
	// 定位最靠前的命中位置（字节偏移）
	first := -1
	for _, t := range terms {
		if i := strings.Index(lower, t); i >= 0 && (first < 0 || i < first) {
			first = i
		}
	}
	runes := []rune(content)
	start := 0
	if first > 0 {
		// 字节偏移 → rune 偏移，再前移 contextBefore 个字符
		start = utf8.RuneCountInString(content[:first]) - snippetContextBefore
		if start < 0 {
			start = 0
		}
	}
	end := start + snippetLen
	if end > len(runes) {
		end = len(runes)
	}
	window := string(runes[start:end])
	out := highlightTerms(window, terms)
	if start > 0 {
		out = "…" + out
	}
	if end < len(runes) {
		out += "…"
	}
	return out
}

// highlightTerms 对窗口文本做 HTML 转义，并把命中词（大小写不敏感）包进 <mark>。
// ToLower 极少数字符会改变字节长度（如 İ），此时放弃高亮仅返回转义文本，保证偏移安全。
func highlightTerms(window string, terms []string) string {
	lower := strings.ToLower(window)
	if len(lower) != len(window) {
		return html.EscapeString(window)
	}
	var b strings.Builder
	i := 0
	for i < len(window) {
		matched := 0
		for _, t := range terms {
			if len(t) > matched && strings.HasPrefix(lower[i:], t) {
				matched = len(t)
			}
		}
		if matched > 0 {
			b.WriteString("<mark>")
			b.WriteString(html.EscapeString(window[i : i+matched]))
			b.WriteString("</mark>")
			i += matched
			continue
		}
		_, size := utf8.DecodeRuneInString(window[i:])
		b.WriteString(html.EscapeString(window[i : i+size]))
		i += size
	}
	return b.String()
}
