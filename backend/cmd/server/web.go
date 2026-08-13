package main

import (
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
// Spec 33：tagCounts 存原始计数，标签云按 widget 参数在装配期构建（纯内存运算，不重查库）；
// recent/popular/latestComments 按配置最大 limit 查询，实例装配时截断。
type sidebarSnapshot struct {
	recent         []*tmpl.RecentPost
	popular        []*tmpl.RecentPost
	latestComments []*tmpl.LatestComment
	tagCloud       []*tmpl.TagItem
	tagCounts      map[string]int
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
	// site 站点配置原子指针（Spec 61 阶段 3）：RebuildSite 复制-交换发布，前台无锁读 .Load()
	site      *atomic.Pointer[tmpl.Site]
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
	Site      *atomic.Pointer[tmpl.Site]
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
	if !strings.HasPrefix(s.site.Load().URL, "https://") {
		return false
	}
	return c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
}

// fillSEO 填充 CanonicalURL 和 Description，所有前台页面渲染前调用。
func (s *webServer) fillSEO(data *tmpl.ViewData, c *gin.Context) {
	baseURL := s.baseURL(c)
	data.Page.CanonicalURL = baseURL + c.Request.URL.Path
	if data.Page.Description == "" {
		data.Page.Description = s.siteOf(data).Description
	}
}

// siteOf 当前渲染应使用的站点配置：ViewData.Site 优先（设置页「前端预览」携带草稿配置），
// 缺省回退全局 s.site（正常流量 ViewData.Site 即 s.site，零差异）。
func (s *webServer) siteOf(data *tmpl.ViewData) *tmpl.Site {
	if data != nil && data.Site != nil {
		return data.Site
	}
	return s.site.Load()
}

// baseURL 站点根地址：site.URL 优先，未配置时回退请求 Host（http）。
func (s *webServer) baseURL(c *gin.Context) string {
	if s.site.Load().URL != "" {
		return s.site.Load().URL
	}
	return "http://" + c.Request.Host
}

// invalidateSidebarCache 侧边栏快照立即失效（整站恢复后用，Spec 41）；平时靠 45s TTL 自然过期。
func (s *webServer) invalidateSidebarCache() {
	s.sidebarExpiry = time.Time{}
}

// fillSidebar 填充侧边栏动态数据（近期发布 + 标签云 + 热门 + 站点统计 + 最新评论），所有前台页面渲染前调用。
// 查询失败时留空，widget 降级为空列表，不影响主内容渲染。
// P2-13：结果缓存 45s（TTL 内新文章/评论稍晚出现在侧栏，可接受）；CountByTag 合并为单次调用（原标签云与统计各查一次）。
// Spec 33：末尾 assembleWidgets 按页面类型装配左右栏实例序列（纯配置查表 + 内存截断，
// 不产生新查询，因此缓存 key 无需页面类型维度）45s 内重复请求仍不重查库。
// buildMenuPages pages 模块已发布页面自动生成左栏菜单项：
// 追加在 menubar 配置之后；访问路径与 menubar 项重复的页面不重复显示（如 /notes /friends 已在菜单）。
func (s *webServer) buildMenuPages() []tmpl.MenuItem {
	pages, err := s.st.ListPages()
	if err != nil {
		return nil
	}
	covered := map[string]bool{}
	for _, it := range s.site.Load().Menubar.Items {
		covered[strings.TrimSuffix(it.URL, "/")] = true
	}
	var items []tmpl.MenuItem
	// ListPages 按 updated_at 降序；菜单按创建序（id 升序）更稳定，倒序遍历回正
	for i := len(pages) - 1; i >= 0; i-- {
		p := pages[i]
		if p.Status != store.StatusPublished {
			continue
		}
		u := "/" + p.Slug
		if covered[u] {
			continue
		}
		covered[u] = true
		items = append(items, tmpl.MenuItem{ID: p.Slug, Title: p.Title, URL: u, Icon: "solar:documents-bold-duotone"})
	}
	return items
}

func (s *webServer) fillSidebar(data *tmpl.ViewData) {
	// 已发布页面自动进左栏菜单（不走侧栏缓存：发布/下线需即时反映）
	data.MenuPages = s.buildMenuPages()
	s.sidebarMu.Lock()
	if s.sidebarCached != nil && time.Now().Before(s.sidebarExpiry) {
		data.Recent = s.sidebarCached.recent
		data.Popular = s.sidebarCached.popular
		data.LatestComments = s.sidebarCached.latestComments
		data.TagCloud = s.sidebarCached.tagCloud
		// 专栏（Spec 52）：已禁用（用户反馈功能多余）。HasTopics 恒为 false，不走缓存值。
		//   data.HasTopics = s.sidebarCached.hasTopics
		data.HasTopics = false
		data.SiteStats = s.sidebarCached.siteStats
		tagCounts := s.sidebarCached.tagCounts
		s.sidebarMu.Unlock()
		s.assembleWidgets(data, tagCounts)
		return
	}
	site := s.siteOf(data)
	if posts, err := s.st.ListPublishedPaged(1, s.widgetMaxLimit(site, "recent", 5)); err == nil {
		for _, p := range posts {
			data.Recent = append(data.Recent, &tmpl.RecentPost{Title: p.Title, URL: "/post/" + p.Slug})
		}
	}
	if popular, err := s.st.ListPopular(s.widgetMaxLimit(site, "popular", popularLimit)); err == nil && len(popular) > 0 {
		for _, p := range popular {
			data.Popular = append(data.Popular, &tmpl.RecentPost{Title: p.Title, URL: "/post/" + p.Slug})
		}
	}
	if site.Comments.Enabled {
		if comments, err := s.st.ListRecentComments(s.widgetMaxLimit(site, "latest_comment", 5)); err == nil {
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
	// 专栏（Spec 52）：已禁用（用户反馈功能多余）。HasTopics 恒为 false，navbar 「专栏」tab 不再显示。
	// 保留 DB 查询代码注释，后续恢复：
	//   if topics, err := s.st.CountByTopic(); err == nil && len(topics) > 0 {
	//       data.HasTopics = true
	//   }
	data.HasTopics = false
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
		tagCounts:      tagCounts,
		hasTopics:      data.HasTopics,
		siteStats:      data.SiteStats,
	}
	s.sidebarExpiry = time.Now().Add(sidebarTTL)
	s.sidebarMu.Unlock()
	s.assembleWidgets(data, tagCounts)
}

// widgetMaxLimit 扫描用户 library 中指定 layout 的 limit 参数，返回与 fallback 的最大值（查询上限）；
// 实例装配时再按各自 limit 截断，保证同 layout 多实例不同 limit 均有足量数据。
// site 参数化：设置页预览传入草稿站点，正常流量传 s.site。
func (s *webServer) widgetMaxLimit(site *tmpl.Site, layout string, fallback int) int {
	max := fallback
	for _, def := range site.Widgets.Library {
		if def.Layout() != layout {
			continue
		}
		if n := def.Int("limit", fallback); n > max {
			max = n
		}
	}
	return max
}

// assembleWidgets 按页面类型解析左右栏组件序列并逐个装配数据（Spec 33）。
// 数据全部来自 fillSidebar 已填充的字段/快照（内存截断与重排），不产生新查询。
// 页面级侧栏覆盖（CustomPageBars，仅 Layout=page 生效）：非 nil 时跳过全局 BarsFor，
// 用自定义名字列表 BuildBar，即使显式空切片也生效（= 空侧栏，不继承全局）。
func (s *webServer) assembleWidgets(data *tmpl.ViewData, tagCounts map[string]int) {
	if data.Page == nil {
		return
	}
	site := s.siteOf(data)
	cfg := site.Widgets
	var bars tmpl.PageBars
	if data.CustomPageBars != nil {
		bars = *data.CustomPageBars
	} else {
		bars = cfg.BarsFor(tmpl.PageTypeOf(data.Page.Layout))
	}
	data.LeftbarWidgets = cfg.BuildBar(bars.Leftbar, data)
	data.RightbarWidgets = cfg.BuildBar(bars.Rightbar, data)
	for _, w := range append(append([]*tmpl.WidgetInstance{}, data.LeftbarWidgets...), data.RightbarWidgets...) {
		switch w.Layout {
		case "recent":
			// wiki 文档页优先当前项目最近更新（handleWikiDoc 已按 UpdatedAt 降序截断 5），
			// 其余页面走全局 Recent（快照缓存）。
			if data.WikiRecentDocs != nil {
				w.Data = truncateRecent(data.WikiRecentDocs, w.Int("limit", 5))
			} else {
				w.Data = truncateRecent(data.Recent, w.Int("limit", 5))
			}
		case "popular":
			w.Data = truncateRecent(data.Popular, w.Int("limit", popularLimit))
		case "latest_comment":
			if n := w.Int("limit", 5); n < len(data.LatestComments) {
				w.Data = data.LatestComments[:n]
			} else {
				w.Data = data.LatestComments
			}
		case "tagcloud":
			w.Data = tmpl.BuildTagCloud(tagCounts, tmpl.TagCloudOptionsFrom(w.Params))
		case "author":
			// Spec 38：当前文章有 author 且在 Site.Authors 中命中时，装配作者卡片数据；
			// 否则不设 Data，模板走站长兜底（零配置回归）。
			if data.Page != nil && data.Page.Author != "" {
				if author, ok := site.Authors[data.Page.Author]; ok {
					w.Data = &tmpl.AuthorWidgetData{
						ID:     data.Page.Author,
						Author: author,
						Path:   "/author/" + data.Page.Author + "/",
					}
				}
			}
		case "tree":
			// Spec 35a：wiki 文档页注入项目文档树（数据仅本页有，不进侧栏快照；
			// 其余页面 Data 留空，模板不输出，零配置降级）。
			if data.WikiDocTree != nil {
				curSlug := ""
				if data.WikiDoc != nil {
					curSlug = data.WikiDoc.Slug
				}
				// 项目 icon：header 展示项目 Logo + 项目名（主题感）
				icon := ""
				if project := s.wikiProjectByID(data.WikiProjectID); project != nil {
					icon = project.Icon
				}
				w.Data = &tmpl.WikiTreeWidgetData{
					ProjectID:   data.WikiProjectID,
					ProjectName: data.WikiProjectName,
					ProjectIcon: icon,
					CurrentSlug: curSlug,
					Tree:        data.WikiDocTree,
				}
			}
		case "wiki_related":
			// 同 tag 项目推荐（仅 wiki 文档页）：与当前项目有 tag 交集、排除自身、
			// 按 Sort 降序截断 limit（默认 5）；无候选时 Data 留空模板不输出。
			if data.WikiProjectID == "" {
				break
			}
			cur := s.wikiProjectByID(data.WikiProjectID)
			if cur == nil {
				break
			}
			var projects []*tmpl.WikiProject
			for i := range s.site.Load().Wiki {
				p := s.site.Load().Wiki[i]
				if p.ID == cur.ID {
					continue
				}
				if !hasCommonTag(p.Tags, cur.Tags) {
					continue
				}
				projects = append(projects, &p)
			}
			if len(projects) == 0 {
				break
			}
			sort.SliceStable(projects, func(i, j int) bool { return projects[i].Sort > projects[j].Sort })
			if n := w.Int("limit", 5); n > 0 && n < len(projects) {
				projects = projects[:n]
			}
			w.Data = projects
			// header 显示"更多：{标签名}"：取当前项目第一个命中的共同标签（首个即优先级最高）
			tagName := ""
			for _, t := range cur.Tags {
				found := false
				for _, p := range projects {
					if hasTag(p.Tags, t) {
						tagName = t
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if tagName != "" {
				w.Params["title"] = "更多：" + tagName
			}
		case "ghrepo":
			// GitHub 仓库卡（仅 wiki 文档页且当前项目配置 repo 时生效）：
			// 从 site.yml wiki 段取 Repo，用户 library 显式配了 repo 时优先用户配置。
			if data.WikiProjectID == "" {
				break
			}
			project := s.wikiProjectByID(data.WikiProjectID)
			if project == nil || project.Repo == "" {
				break
			}
			if w.Params.Str("repo", "") == "" {
				w.Params["repo"] = project.Repo
			}
		case "tagtree":
			// Spec 35b：笔记本相关页注入标签树（数据仅本页有，不进侧栏快照；
			// 其余页面 Data 留空，模板不输出，零配置降级）。
			if data.TagTreeData != nil {
				w.Data = data.TagTreeData
			}
		case "linklist":
			// Spec 50：linklist widget 数据，从 widget params 的 items 解析。
			w.Data = parseLinkListItems(w.Params)
		}
	}
}

// tocOptions library toc 参数 → TOC 生成选项（Spec 33 参数补齐：list_number/min_depth/max_depth）。
func (s *webServer) tocOptions() tmpl.TOCOptions {
	def := s.site.Load().Widgets.Lookup("toc")
	return tmpl.TOCOptions{
		ListNumber: def.Bool("list_number", false),
		MinDepth:   def.Int("min_depth", 1),
		MaxDepth:   def.Int("max_depth", 6),
	}
}

// truncateRecent 按 limit 截断文章列表（共享底层数组，只读安全）。
func truncateRecent(list []*tmpl.RecentPost, limit int) []*tmpl.RecentPost {
	if limit > 0 && limit < len(list) {
		return list[:limit]
	}
	return list
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

// parseLinkListItems 从 widget params 解析 items 为 []LinkListItem（Spec 50）。
// items 由 yaml 解析为 []any，每项为 map[string]any{title, url, icon}。
func parseLinkListItems(params tmpl.WidgetDef) []tmpl.LinkListItem {
	raw, ok := params["items"]
	if !ok {
		return nil
	}
	rawSlice, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]tmpl.LinkListItem, 0, len(rawSlice))
	for _, r := range rawSlice {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		item := tmpl.LinkListItem{
			Title: toString(m["title"]),
			URL:   toString(m["url"]),
			Icon:  toString(m["icon"]),
		}
		out = append(out, item)
	}
	return out
}

// toString 类型安全转字符串，nil 或非 string 返回空。
func toString(v any) string {
	s, _ := v.(string)
	return s
}

// fillComments 文章页评论区 SSR 注入（评论树 + 计数）。查询失败留空，评论区降级为空列表。
// CommentPageKey 统一在此设置（模板 data-page-key 优先用它，文章页兜底 /post/{slug}）。
func (s *webServer) fillComments(data *tmpl.ViewData, pageKey string) {
	data.CommentPageKey = pageKey
	if !s.site.Load().Comments.Enabled {
		return
	}
	if comments, err := s.st.ListCommentsByPage(pageKey); err == nil {
		data.Comments = comment.ToViewItems(comments, &s.site.Load().Comments)
	}
	if n, err := s.st.CountCommentsByPage(pageKey); err == nil {
		data.CommentCount = n
	}
}

// render404 渲染自定义 404 页（HTTP 404 + base 布局 Layout=404），NoRoute 与各路由未命中共用。
func (s *webServer) render404(c *gin.Context) {
	data := &tmpl.ViewData{
		Site: s.site.Load(),
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
		Site: s.site.Load(),
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
// 分类图标查 article.category_icon（未命中回退 default:category）；
// 海报 DecodePoster 非 nil 时，与 Cover 一起触发首页 photo 封面大卡。
func (s *webServer) toCard(p *store.Post) *tmpl.PostCard {
	return s.toCardWith(s.site.Load(), p)
}

// toCardWith 同 toCard，但站点配置参数化（设置页「前端预览」用草稿站点渲染卡片）。
func (s *webServer) toCardWith(site *tmpl.Site, p *store.Post) *tmpl.PostCard {
	card := &tmpl.PostCard{
		Slug:          p.Slug,
		Title:         p.Title,
		Excerpt:       p.Excerpt,
		Cover:         p.Cover,
		Date:          p.CreatedAt,
		Pinned:        p.Pinned,
		Category:      p.Category,
		CategoryStyle: tmpl.CategoryColorStyle(p.Category, site.Article.CategoryColor),
		CategoryIcon:  tmpl.CategoryIconKey(p.Category, site.Article.CategoryIcon),
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

// toCardsWith 同 toCards，站点配置参数化（预览用）。
func (s *webServer) toCardsWith(site *tmpl.Site, posts []*store.Post) []*tmpl.PostCard {
	cards := make([]*tmpl.PostCard, 0, len(posts))
	for _, p := range posts {
		cards = append(cards, s.toCardWith(site, p))
	}
	return cards
}
