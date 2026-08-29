package routes

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/comments"
	"blog/internal/config"
	"blog/internal/og"
	"blog/internal/post"
	"blog/internal/search"
	"blog/internal/store"
	"blog/internal/textutil"
	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"
	"blog/modules/stellar/web/assets"
)

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
	site *atomic.Pointer[tmpl.Site]
	// contentCache 内容派生产物缓存（Spec 61 阶段 5：/search.json 与 /sitemap.xml）
	contentCache *contentCache
	// swTemplate Service Worker 模板（Spec 61 阶段 5：启动读一次，避免每请求读文件；
	// dev 模式同样启动时读——改 sw.js 需重启服务生效）
	swTemplate []byte
	st         *store.Store
	posts      *post.Service
	engine     *stellarweb.Engine
	searchIdx  *search.Index // 可空：FTS 建索引失败时降级
	ogGen      *og.Generator
	momentH    *MomentHandler // /notes 页近期动态，Stellar 装配 moment 后设置

	devMode     bool
	root        string // app.Root：-dev 模式磁盘资源基准
	uploadDir   string
	notesMDPath string

	sidebarMu     sync.Mutex
	sidebarCached *sidebarSnapshot
	sidebarExpiry time.Time
}

// loadSWTemplate 启动时读一次 SW 模板（dev 走磁盘、embed 走 FS），失败返回 nil（serveSW 404）。
func loadSWTemplate(devMode bool, root string) []byte {
	var data []byte
	var err error
	if devMode {
		data, err = os.ReadFile(filepath.Join(root, "modules", "stellar", "web", "assets", "js", "sw.js"))
	} else {
		data, err = assets.FS.ReadFile("js/sw.js")
	}
	if err != nil {
		log.Printf("load sw template: %v", err)
		return nil
	}
	return data
}

// webDeps webServer 构造依赖（均为必填，momentH 除外——moment 装配依赖 webServer 的 fill 函数，装配后回填）。
type webDeps struct {
	Site      *atomic.Pointer[tmpl.Site]
	Store     *store.Store
	Posts     *post.Service
	Engine    *stellarweb.Engine
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
		site:         d.Site,
		st:           d.Store,
		posts:        d.Posts,
		engine:       d.Engine,
		searchIdx:    d.SearchIdx,
		ogGen:        d.OGGen,
		devMode:      d.DevMode,
		root:         d.Root,
		uploadDir:    d.UploadDir,
		notesMDPath:  d.NotesPath,
		contentCache: newContentCache(d.Store, d.Site),
		swTemplate:   loadSWTemplate(d.DevMode, d.Root),
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
	baseURL := s.canonicalBaseURL(data, c)
	data.Page.CanonicalURL = baseURL + c.Request.URL.Path
	if data.Page.Description == "" {
		data.Page.Description = s.siteOf(data).Description
	}
}

// canonicalBaseURL uses Stellar's originalHost rule. Official hosts are still
// recognised by the client-side warning, but canonical URLs always point at
// the configured source host.
func (s *webServer) canonicalBaseURL(data *tmpl.ViewData, c *gin.Context) string {
	site := s.siteOf(data)
	original := canonicalHost(site.SEO.Canonical.OriginalHost)
	if original == "" {
		return s.baseURL(c)
	}
	return "https://" + original
}

// canonicalHost accepts host-only values and tolerates legacy scheme/port
// values so host comparisons remain stable across local and proxy requests.
func canonicalHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err == nil && parsed.Host != "" {
		raw = parsed.Host
	} else if !strings.Contains(raw, "://") {
		if parsed, err := url.Parse("//" + raw); err == nil && parsed.Host != "" {
			raw = parsed.Host
		}
	}
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), "."))
}

// articleExcerpt applies article.auto_excerpt at render time so changing the
// setting also updates existing posts that were imported without an excerpt.
func articleExcerpt(site *tmpl.Site, explicit, contentHTML string) string {
	if strings.TrimSpace(explicit) != "" || site == nil || site.Article.AutoExcerpt <= 0 {
		return explicit
	}
	return textutil.Truncate(textutil.StripHTML(contentHTML), site.Article.AutoExcerpt)
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
	s.sidebarMu.Lock()
	defer s.sidebarMu.Unlock()
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
		items = append(items, tmpl.MenuItem{ID: p.Slug, Title: p.Title, URL: u, Icon: "default:documents"})
	}
	return items
}

func (s *webServer) fillSidebar(data *tmpl.ViewData) {
	if ai, err := config.LoadAI(s.st); err == nil {
		data.QAEnabled = ai.QAOn()
		data.QAAllowUserLLM = ai.QAUserLLMOn()
	}
	// 已发布页面自动进左栏菜单（不走侧栏缓存：发布/下线需即时反映）
	data.MenuPages = s.buildMenuPages()
	s.sidebarMu.Lock()
	if s.sidebarCached != nil && time.Now().Before(s.sidebarExpiry) {
		data.Recent = s.sidebarCached.recent
		data.Popular = s.sidebarCached.popular
		data.LatestComments = s.sidebarCached.latestComments
		data.TagCloud = s.sidebarCached.tagCloud
		data.HasTopics = s.sidebarCached.hasTopics
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
		if recent, err := s.st.ListRecentComments(s.widgetMaxLimit(site, "latest_comment", 5)); err == nil {
			for _, cm := range recent {
				data.LatestComments = append(data.LatestComments, &tmpl.LatestComment{
					Nick:    cm.Nick,
					Excerpt: comments.ExcerptHTML(cm.ContentHTML, 40),
					URL:     cm.PageKey + "#comment-" + strconv.FormatInt(cm.ID, 10),
					TimeAgo: comments.TimeAgo(cm.CreatedAt),
				})
			}
		}
	}
	// 标签计数单次查询：标签云与站点统计共用（P2-13 去重）。
	tagCounts, tagErr := s.st.CountByTag()
	if tagErr == nil {
		data.TagCloud = buildTagCloud(tagCounts, 20)
	}
	// 专栏入口随 topics 表存在性显示，专栏内文章列表仍由详情页按 topic_id 查询。
	if topics, err := s.st.ListTopics(); err == nil && len(topics) > 0 {
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
			// 多作者文章的作者卡片以第一位作者为头像/简介，名称与链接展示完整作者列表。
			if data.Page != nil && len(data.Page.AuthorLinks) > 0 {
				first := data.Page.AuthorLinks[0]
				w.Data = &tmpl.AuthorWidgetData{
					ID:      first.ID,
					Author:  tmpl.AuthorDef{Name: first.Name, Avatar: first.Avatar, Description: first.Description, URL: first.URL},
					Path:    "/author/" + first.ID + "/",
					Authors: append([]tmpl.AuthorLink(nil), data.Page.AuthorLinks...),
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
			for _, source := range s.wikiProjectsForRuntime() {
				p := *source
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
		case "linklist":
			// Spec 50：linklist widget 数据，从 widget params 的 items 解析。
			w.Data = parseLinkListItems(w.Params)
		}
	}
}

// tocOptions library toc 参数 → TOC 生成选项（Spec 33 参数补齐：list_number/min_depth/max_depth）。
func (s *webServer) tocOptions() stellarweb.TOCOptions {
	return s.tocOptionsFrom(s.site.Load())
}

// tocOptionsFrom 使用指定站点配置读取 toc 参数，设置页预览可据此联动草稿。
func (s *webServer) tocOptionsFrom(site *tmpl.Site) stellarweb.TOCOptions {
	def := site.Widgets.Lookup("toc")
	return stellarweb.TOCOptions{
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
	if list, err := s.st.ListCommentsByPage(pageKey); err == nil {
		boosts, _ := s.st.ListBoostsByPage(pageKey)
		data.Comments = comments.ToViewItemsWithOptions(list, &s.site.Load().Comments, comments.BoostViewOptions{Boosts: boosts})
		// Spec 62：ListCommentsByPage 返回全量，计数直接 len 派生，省一次 COUNT 查询。
		data.CommentCount = len(list)
	} else {
		// 列表查询失败时单独尝试计数（仍失败则留 0，评论区降级为空）
		if n, err := s.st.CountCommentsByPage(pageKey); err == nil {
			data.CommentCount = n
		}
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
	s.fillPinSlider(data)
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

// fillPinSlider 装配 1.40 置顶轮播数据。文章列表的 flat 模式由数据库 pinned 排序承担，
// carousel 模式则在列表前抽出置顶项，避免卡片重复出现。
func (s *webServer) fillPinSlider(data *tmpl.ViewData) {
	if data == nil || data.Page == nil || data.Site == nil || data.Site.Article.EffectivePinStyle() == "flat" {
		return
	}
	if data.Page.Layout == "index" && data.Paginator != nil && data.Paginator.Current > 1 {
		return
	}
	data.PinSliderActive = pinSliderReplacesPosts(data)
	switch data.Page.Layout {
	case "index", "archive", "categories", "tags", "topics":
		posts, err := s.st.ListPublished()
		if err != nil {
			return
		}
		for _, p := range posts {
			if p.Pinned {
				data.PinnedPosts = append(data.PinnedPosts, s.toCardWith(data.Site, p))
			}
		}
	case "wiki":
		for _, project := range s.wikiProjectsForRuntime() {
			if project.Pin {
				data.PinnedWiki = append(data.PinnedWiki, project)
			}
		}
	}
}

func pinSliderReplacesPosts(data *tmpl.ViewData) bool {
	if data == nil || data.Page == nil || data.Page.Layout != "index" || data.Page.Category != "" || data.Page.Tag != "" {
		return false
	}
	return data.Paginator == nil || data.Paginator.Current == 1
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
		Excerpt:       articleExcerpt(site, p.Excerpt, p.ContentHTML),
		Description:   articleExcerpt(site, p.Excerpt, p.ContentHTML),
		Cover:         p.Cover,
		Date:          p.CreatedAt,
		Pinned:        p.Pinned,
		Category:      p.Category,
		CategoryStyle: tmpl.CategoryColorStyle(p.Category, site.Article.CategoryColor),
		CategoryIcon:  tmpl.CategoryIconKey(p.Category, site.Article.CategoryIcon),
		Tags:          tmpl.ParseTags(p.Tags),
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
