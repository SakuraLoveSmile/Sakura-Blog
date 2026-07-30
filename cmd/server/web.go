package main

import (
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/comment"
	"blog/internal/moment"
	"blog/internal/og"
	"blog/internal/post"
	"blog/internal/search"
	"blog/internal/store"
	tmpl "blog/internal/template"
)

// 搜索索引与 widget 摘要共用的纯文本提取：去标签 + 压缩空白。
var (
	htmlTagRe    = regexp.MustCompile(`<[^>]+>`)
	whitespaceRe = regexp.MustCompile(`\s+`)
)

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(whitespaceRe.ReplaceAllString(s, " "))
}

// truncateText 按 rune 截断字符串，超长加省略号（widget 摘要用）。
func truncateText(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// sidebarSnapshot 侧边栏 45s 快照缓存（P2-13）。
// 快照切片在请求间共享底层数组，模板渲染只读，无数据竞争。
type sidebarSnapshot struct {
	recent         []*tmpl.RecentPost
	popular        []*tmpl.RecentPost
	latestComments []*tmpl.LatestComment
	tagCloud       []*tmpl.TagItem
	hasTopics      bool
	siteStats      *tmpl.SiteStats
}

const (
	popularLimit = 5
	sidebarTTL   = 45 * time.Second
)

// webServer 前台路由装配的共享依赖载体：按域拆分的 register* 方法挂在它上面，
// 渲染辅助（fillSidebar/fillSEO/render404 等）与侧边栏缓存也集中于此。
type webServer struct {
	site      *tmpl.Site
	st        *store.Store
	posts     *post.Service
	engine    *tmpl.Engine
	searchIdx *search.Index // 可空：FTS 建索引失败时降级
	ogGen     *og.Generator
	momentH   *moment.Handler // /notes 页近期动态，main 装配 moment 后设置

	devMode     bool
	root        string // app.Root：-dev 模式磁盘资源基准
	uploadDir   string
	notesMDPath string

	sidebarMu     sync.Mutex
	sidebarCached *sidebarSnapshot
	sidebarExpiry time.Time
}

// webDeps webServer 构造依赖（均为必填，momentH 除外——moment 装配依赖 webServer 的 fill 函数，装配后回填）。
type webDeps struct {
	Site      *tmpl.Site
	Store     *store.Store
	Posts     *post.Service
	Engine    *tmpl.Engine
	SearchIdx *search.Index
	OGGen     *og.Generator
	DevMode   bool
	Root      string
	UploadDir string
	NotesPath string
}

// newWebServer 构造前台装配载体。
func newWebServer(d webDeps) *webServer {
	return &webServer{
		site:        d.Site,
		st:          d.Store,
		posts:       d.Posts,
		engine:      d.Engine,
		searchIdx:   d.SearchIdx,
		ogGen:       d.OGGen,
		devMode:     d.DevMode,
		root:        d.Root,
		uploadDir:   d.UploadDir,
		notesMDPath: d.NotesPath,
	}
}

// cookieSecureFor Cookie Secure 标志：site.URL 为 https 且当前请求经 HTTPS 到达（直连 TLS 或反代 X-Forwarded-Proto）时置真。
// 按请求判定而非启动时定死：http://IP:port 调试访问不签 Secure，避免浏览器拒存 cookie 导致解锁态丢失。
func (s *webServer) cookieSecureFor(c *gin.Context) bool {
	if !strings.HasPrefix(s.site.URL, "https://") {
		return false
	}
	return c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
}

// fillSEO 填充 CanonicalURL 和 Description，所有前台页面渲染前调用。
func (s *webServer) fillSEO(data *tmpl.ViewData, c *gin.Context) {
	baseURL := s.baseURL(c)
	data.Page.CanonicalURL = baseURL + c.Request.URL.Path
	if data.Page.Description == "" {
		data.Page.Description = s.site.Description
	}
}

// baseURL 站点根地址：site.URL 优先，未配置时回退请求 Host（http）。
func (s *webServer) baseURL(c *gin.Context) string {
	if s.site.URL != "" {
		return s.site.URL
	}
	return "http://" + c.Request.Host
}

// invalidateSidebarCache 侧边栏快照立即失效（整站恢复后用，Spec 41）；平时靠 45s TTL 自然过期。
func (s *webServer) invalidateSidebarCache() {
	s.sidebarExpiry = time.Time{}
}

// fillSidebar 填充侧边栏动态数据（近期发布 5 篇 + 标签云前 20 + 热门 top5 + 站点统计 + 最新评论 5 条），所有前台页面渲染前调用。
// 查询失败时留空，widget 降级为空列表，不影响主内容渲染。
// P2-13：结果缓存 45s（TTL 内新文章/评论稍晚出现在侧栏，可接受）；CountByTag 合并为单次调用（原标签云与统计各查一次）。
func (s *webServer) fillSidebar(data *tmpl.ViewData) {
	s.sidebarMu.Lock()
	defer s.sidebarMu.Unlock()
	if s.sidebarCached != nil && time.Now().Before(s.sidebarExpiry) {
		data.Recent = s.sidebarCached.recent
		data.Popular = s.sidebarCached.popular
		data.LatestComments = s.sidebarCached.latestComments
		data.TagCloud = s.sidebarCached.tagCloud
		data.HasTopics = s.sidebarCached.hasTopics
		data.SiteStats = s.sidebarCached.siteStats
		return
	}
	if posts, err := s.st.ListPublishedPaged(1, 5); err == nil {
		for _, p := range posts {
			data.Recent = append(data.Recent, &tmpl.RecentPost{Title: p.Title, URL: "/post/" + p.Slug})
		}
	}
	if popular, err := s.st.ListPopular(popularLimit); err == nil && len(popular) > 0 {
		for _, p := range popular {
			data.Popular = append(data.Popular, &tmpl.RecentPost{Title: p.Title, URL: "/post/" + p.Slug})
		}
	}
	if s.site.Comments.Enabled {
		if comments, err := s.st.ListRecentComments(5); err == nil {
			for _, cm := range comments {
				data.LatestComments = append(data.LatestComments, &tmpl.LatestComment{
					Nick:    cm.Nick,
					Excerpt: truncateText(stripHTML(cm.ContentHTML), 40),
					URL:     cm.PageKey + "#comment-" + strconv.FormatInt(cm.ID, 10),
					TimeAgo: comment.TimeAgo(cm.CreatedAt),
				})
			}
		}
	}
	// 标签计数单次查询：标签云与站点统计共用（P2-13 去重）。
	tagCounts, tagErr := s.st.CountByTag()
	if tagErr == nil {
		data.TagCloud = buildTagCloud(tagCounts, 20)
	}
	// 专栏存在性：navbar「专栏」tab 显示条件（对齐原版 theme.topic.publish_list 非空）
	if topics, err := s.st.CountByTopic(); err == nil && len(topics) > 0 {
		data.HasTopics = true
	}
	var stats tmpl.SiteStats
	statsOK := true
	if n, err := s.st.CountPosts(); err == nil {
		stats.Posts = n
	} else {
		statsOK = false
	}
	if tagErr == nil {
		stats.Tags = len(tagCounts)
	} else {
		statsOK = false
	}
	if n, err := s.st.CountComments(store.CommentApproved); err == nil {
		stats.Comments = n
	} else {
		statsOK = false
	}
	if v, err := s.st.SumViews(); err == nil {
		stats.Views = v
	} else {
		statsOK = false
	}
	if statsOK {
		data.SiteStats = &stats
	}
	s.sidebarCached = &sidebarSnapshot{
		recent:         data.Recent,
		popular:        data.Popular,
		latestComments: data.LatestComments,
		tagCloud:       data.TagCloud,
		hasTopics:      data.HasTopics,
		siteStats:      data.SiteStats,
	}
	s.sidebarExpiry = time.Now().Add(sidebarTTL)
}

// buildTagCloud 标签计数转标签云条目：按数量降序（同量按名升序）取前 limit 个并标注字号。
func buildTagCloud(tagCounts map[string]int, limit int) []*tmpl.TagItem {
	items := make([]*tmpl.TagItem, 0, len(tagCounts))
	for name, n := range tagCounts {
		items = append(items, &tmpl.TagItem{Name: name, Count: n})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return items[i].Name < items[j].Name
	})
	if len(items) > limit {
		items = items[:limit]
	}
	if len(items) > 0 {
		minC, maxC := items[len(items)-1].Count, items[0].Count
		for _, item := range items {
			item.Size = tmpl.TagCloudFontSize(item.Count, minC, maxC)
		}
	}
	return items
}

// fillComments 文章页评论区 SSR 注入（评论树 + 计数）。查询失败留空，评论区降级为空列表。
func (s *webServer) fillComments(data *tmpl.ViewData, pageKey string) {
	if !s.site.Comments.Enabled {
		return
	}
	if comments, err := s.st.ListCommentsByPage(pageKey); err == nil {
		data.Comments = comment.ToViewItems(comments, &s.site.Comments)
	}
	if n, err := s.st.CountCommentsByPage(pageKey); err == nil {
		data.CommentCount = n
	}
}

// render404 渲染自定义 404 页（HTTP 404 + base 布局 Layout=404），NoRoute 与各路由未命中共用。
func (s *webServer) render404(c *gin.Context) {
	data := &tmpl.ViewData{
		Site: s.site,
		Page: &tmpl.Page{Title: "页面不存在", Layout: "404", Lang: "zh-CN"},
	}
	s.fillSidebar(data)
	html, err := s.engine.Render("layout/base", data)
	if err != nil {
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	c.Data(http.StatusNotFound, "text/html; charset=utf-8", []byte(html))
}

// render500 渲染自定义 500 页（P1-8），不暴露内部错误细节。
func (s *webServer) render500(c *gin.Context) {
	data := &tmpl.ViewData{
		Site: s.site,
		Page: &tmpl.Page{Title: "服务器错误", Layout: "500", Lang: "zh-CN"},
	}
	s.fillSidebar(data)
	html, err := s.engine.Render("layout/base", data)
	if err != nil {
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	c.Data(http.StatusInternalServerError, "text/html; charset=utf-8", []byte(html))
}

// renderBase 渲染 base 布局并写响应（归档/分类/标签页等共用）。
func (s *webServer) renderBase(c *gin.Context, data *tmpl.ViewData) {
	s.fillSidebar(data)
	s.fillSEO(data, c)
	html, err := s.engine.Render("layout/base", data)
	if err != nil {
		log.Printf("render: %v", err)
		s.render500(c)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// toCard store.Post 转文章卡片视图模型，首页/分类标签列表/相关文章共用。
// 分类配色查 site.yml article.category_color（对齐原版 category_color helper）；
// 海报 DecodePoster 非 nil 时，与 Cover 一起触发首页 photo 封面大卡。
func (s *webServer) toCard(p *store.Post) *tmpl.PostCard {
	card := &tmpl.PostCard{
		Slug:          p.Slug,
		Title:         p.Title,
		Excerpt:       p.Excerpt,
		Cover:         p.Cover,
		Date:          p.CreatedAt,
		Pinned:        p.Pinned,
		Category:      p.Category,
		CategoryStyle: tmpl.CategoryColorStyle(p.Category, s.site.Article.CategoryColor),
	}
	if pd := store.DecodePoster(p.Poster); pd != nil {
		card.Poster = &tmpl.PosterCard{Headline: pd.Headline, Topic: pd.Topic, Caption: pd.Caption, Color: pd.Color}
	}
	return card
}

// toCards store.Post 列表转首页卡片视图模型。
func (s *webServer) toCards(posts []*store.Post) []*tmpl.PostCard {
	cards := make([]*tmpl.PostCard, 0, len(posts))
	for _, p := range posts {
		cards = append(cards, s.toCard(p))
	}
	return cards
}
