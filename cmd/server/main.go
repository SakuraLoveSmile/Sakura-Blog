package main

import (
	"context"
	"crypto/subtle"
	"encoding/xml"
	"flag"
	"fmt"
	"hash/fnv"
	"html/template"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"

	"blog/assets"
	"blog/console"
	"blog/internal/api"
	backuppkg "blog/internal/backup"
	"blog/internal/comment"
	"blog/internal/config"
	"blog/internal/feed"
	"blog/internal/friends"
	ghpkg "blog/internal/github"
	"blog/internal/moment"
	"blog/internal/og"
	"blog/internal/page"
	"blog/internal/post"
	"blog/internal/search"
	seedpkg "blog/internal/seed"
	"blog/internal/store"
	tmpl "blog/internal/template"
)

// RSS 2.0 结构体（encoding/xml 标准库）
type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel Channel  `xml:"channel"`
}

type Channel struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Items       []Item `xml:"item"`
}

type Item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Guid        string `xml:"guid"`
}

// 资源加载策略：
//   - 默认（部署）：icons/templates/assets 全部走 go:embed，单二进制无外部依赖。
//   - -dev 模式（本地调试）：从磁盘读 icons.yml、internal/template、assets/，改样式/模板即时生效。
//   - root 用于定位 config（app.yml/site.yml）、DB 与 content；embed 模式下仅作目录基准。
//
// 不能依赖 os.Executable()：go run 编译到临时目录，二进制路径不在源码目录。
func main() {
	var (
		root       string
		seed       bool
		devMode    bool
		iconsPath  string
		configPath string
	)
	flag.StringVar(&root, "root", "", "工作根目录（定位 config/DB/content；空则用 cwd 或 app.yml）")
	flag.BoolVar(&seed, "seed", false, "启动时把 content/posts/*.md 导入 SQLite（按 slug UPSERT）")
	flag.BoolVar(&devMode, "dev", false, "开发模式：从磁盘读 icons/templates/assets（默认走 go:embed）")
	flag.StringVar(&iconsPath, "icons", "", "icons.yml 路径（仅 -dev 模式生效，默认 <root>/internal/template/data/icons.yml）")
	flag.StringVar(&configPath, "config", "", "app.yml 路径（默认 <root>/config/app.yml）")
	flag.Parse()

	// 应用配置：先算 bootstrap root（-root > BLOG_ROOT > cwd），据此定位 app.yml，
	// 再 ResolveApp 应用环境变量覆盖与路径绝对化。
	bootstrapRoot := root
	if bootstrapRoot == "" {
		bootstrapRoot = os.Getenv("BLOG_ROOT")
	}
	if bootstrapRoot == "" {
		bootstrapRoot, _ = os.Getwd()
	}
	bootstrapRoot, _ = filepath.Abs(bootstrapRoot)

	appCfgPath := configPath
	if appCfgPath == "" {
		appCfgPath = filepath.Join(bootstrapRoot, "config", "app.yml")
	}
	app, err := config.LoadApp(appCfgPath)
	if err != nil {
		log.Fatalf("load app config: %v", err)
	}
	if root != "" {
		app.Root = root // -root flag 优先于 app.yml
	}
	app = config.ResolveApp(app)

	// icons 加载：-dev 从磁盘读规范副本（internal/template/data/icons.yml，与 embed 同源、已内联 share:* SVG），
	// 否则走 embed。上游 resource/.../icons.yml 的 share:* 仍是裸 URL，会导致分享图标显示为 URL 文本，故不作默认。
	var reg *tmpl.IconRegistry
	if devMode {
		if iconsPath == "" {
			iconsPath = filepath.Join(app.Root, "internal", "template", "data", "icons.yml")
		}
		reg, err = tmpl.LoadIcons(iconsPath)
		if err != nil {
			log.Fatalf("load icons (dev): %v", err)
		}
		log.Printf("loaded %d icons from %s (dev)", reg.Len(), iconsPath)
	} else {
		reg, err = tmpl.LoadIconsEmbed()
		if err != nil {
			log.Fatalf("load icons (embed): %v", err)
		}
		log.Printf("loaded %d icons from embed", reg.Len())
	}

	// 模板引擎：-dev 从磁盘读（改模板即时生效），否则走 embed。命名均为路径化（layout/base 等）。
	var engine *tmpl.Engine
	if devMode {
		engine, err = tmpl.NewEngine(filepath.Join(app.Root, "internal", "template"), reg)
		if err != nil {
			log.Fatalf("load templates (dev): %v", err)
		}
		log.Printf("templates loaded from disk (dev)")
	} else {
		engine, err = tmpl.NewEngineFromEmbed(reg)
		if err != nil {
			log.Fatalf("load templates (embed): %v", err)
		}
		log.Printf("templates loaded from embed")
	}

	// 文章服务：content/posts/*.md（文件回退用 + seed 源）。按 root 定位，部署时可选。
	posts := post.NewService(filepath.Join(app.Root, "content", "posts"))
	posts.SetIconFunc(reg.Get) // 供 quot 等 shortcode 渲染内联 SVG（复用模板 icon 同一套图标）

	// SQLite 持久化：DBPath 已在 ResolveApp 绝对化。
	st, err := store.New(app.DBPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()
	log.Printf("sqlite ready: %s", app.DBPath)

	// 搜索索引与 widget 摘要共用的纯文本提取：去标签 + 压缩空白。
	htmlTagRe := regexp.MustCompile(`<[^>]+>`)
	whitespaceRe := regexp.MustCompile(`\s+`)
	stripHTML := func(s string) string {
		s = htmlTagRe.ReplaceAllString(s, " ")
		return strings.TrimSpace(whitespaceRe.ReplaceAllString(s, " "))
	}

	// FTS5 全文索引（Spec 25）：建表 + 挂 store 写路径回调同步；失败仅记日志，
	// 搜索降级回 /search.json 前端匹配，不阻断启动。在 seed 前装配，保证导入也能同步。
	searchIdx := search.New(st.DB())
	if err := searchIdx.Ensure(); err != nil {
		log.Printf("search: ensure fts failed (%v), server search disabled", err)
		searchIdx = nil
	} else {
		// 空表回填：旧库首次启用 FTS 时全量建索引（幂等，非空不重建）。
		if n, err := searchIdx.Count(); err == nil && n == 0 {
			if published, err := st.ListPublished(); err == nil && len(published) > 0 {
				items := make([]search.Item, 0, len(published))
				for _, p := range published {
					content := stripHTML(p.ContentHTML)
					if p.IsPasswordProtected() {
						content = ""
					}
					items = append(items, search.Item{Slug: p.Slug, Title: p.Title, Content: content})
				}
				if err := searchIdx.Rebuild(items); err != nil {
					log.Printf("search: rebuild failed: %v", err)
				} else {
					log.Printf("search: fts index backfilled, %d posts", len(items))
				}
			}
		}
	}

	// OG 社交分享图生成器（Spec 27）：字体从 assets/fonts/ 加载，缓存到 data/ogcache/。
	ogCacheDir := filepath.Join(filepath.Dir(app.DBPath), "ogcache")
	var ogFontPath string
	if devMode {
		ogFontPath = filepath.Join(app.Root, "assets", "fonts", "LXGWWenKai-Regular.ttf")
	} else {
		// embed 模式：从 embed FS 读出字体写入缓存目录供 gg 加载
		if data, err := assets.FS.ReadFile("fonts/LXGWWenKai-Regular.ttf"); err == nil {
			_ = os.MkdirAll(ogCacheDir, 0o755)
			ogFontPath = filepath.Join(ogCacheDir, ".font.ttf")
			_ = os.WriteFile(ogFontPath, data, 0o644)
		}
	}
	ogGen := og.New(ogFontPath, ogCacheDir)

	// store 写路径统一回调：搜索索引同步 + OG 缓存失效。
	st.SetPostWriteHook(func(op, slug string) {
		// OG 缓存失效（所有写操作均失效）
		ogGen.Invalidate(slug)
		// 搜索索引同步
		if searchIdx == nil {
			return
		}
		switch op {
		case "delete", "purge":
			_ = searchIdx.Unindex(slug)
		default: // create / update / restore
			p, err := st.GetBySlug(slug)
			if err != nil || p == nil {
				return
			}
			if p.Status != store.StatusPublished || p.DeletedAt != "" || (!p.PublishAt.IsZero() && p.PublishAt.After(time.Now())) {
				_ = searchIdx.Unindex(slug)
				return
			}
			content := stripHTML(p.ContentHTML)
			if p.IsPasswordProtected() {
				content = ""
			}
			if err := searchIdx.IndexPost(p.Slug, p.Title, content); err != nil {
				log.Printf("search: index %s failed: %v", slug, err)
			}
		}
	})

	// 站点配置：优先读 config/site.yml，缺失/解析失败时 fallback 到 DefaultSite()
	siteConfigPath := filepath.Join(app.Root, "config", "site.yml")
	site, err := config.LoadSite(siteConfigPath)
	if err != nil {
		log.Printf("load site config failed (%v), fallback to DefaultSite", err)
		site = config.DefaultSite()
	} else {
		log.Printf("loaded site config: %s", siteConfigPath)
	}

	// 评论配置兜底：site.yml 缺字段时补默认值（Enabled 以配置为准，不强制开启）
	if site.Comments.GravatarMirror == "" {
		site.Comments.GravatarMirror = "https://weavatar.com/avatar"
	}
	if site.Comments.AdminBadge == "" {
		site.Comments.AdminBadge = "博主"
	}
	if site.Comments.Title == "" {
		site.Comments.Title = "评论"
	}

	// 站点配置迁移到 DB：Bootstrap 首次把 site.yml 导入 settings/links 表（空表才导，admin 段单独兼容补导），
	// 之后 RebuildSite 以 DB 为准回填 site（含 console 在线修改）。密码入库后 site.yml 仅作初始值。
	if err := config.Bootstrap(st, site); err != nil {
		log.Fatalf("bootstrap settings: %v", err)
	}
	if err := config.RebuildSite(st, site); err != nil {
		log.Fatalf("rebuild site from db: %v", err)
	}
	log.Printf("site config synced from db (settings/links)")

	// P0-2：生产模式 site.url 校验——release 下空 URL 会导致 canonical/OG/sitemap 回退 http，显著告警提醒配置。
	if !devMode && site.URL == "" {
		log.Printf("⚠️  WARNING: site.url is empty in release mode; canonical/OG/sitemap/feed URLs will fallback to http://<request-host>. Set url: https://your-domain in config/site.yml for production.")
	}
	// Cookie Secure 标志：site.URL 为 https 且当前请求经 HTTPS 到达（直连 TLS 或反代 X-Forwarded-Proto）时置真。
	// 按请求判定而非启动时定死：http://IP:port 调试访问不签 Secure，避免浏览器拒存 cookie 导致解锁态丢失。
	cookieSecureFor := func(c *gin.Context) bool {
		if !strings.HasPrefix(site.URL, "https://") {
			return false
		}
		return c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	}

	// 评论 handler：公开 API 注册 + 评论回复通知复用（Enabled=false 时不注册路由）
	commentHandler := comment.New(st, &site.Comments, site.Title)

	// GitHub 代理缓存（Spec 21）：内存 + 磁盘双缓存客户端，供 ghrepo/ghuser/ghcard 同步取数。
	// 必须在 seed 之前注入 posts：否则 seed 渲染时 ghClient 为 nil，卡片会降级为纯链接并存入 content_html。
	ghClient := ghpkg.New(ghpkg.Config{
		Token:    app.GitHubToken,
		CacheDir: filepath.Join(filepath.Dir(app.DBPath), "ghcache"),
	})
	posts.SetGHClient(&ghAdapter{ghClient})

	// seed：-seed flag 或 app.yml seed_on_start / BLOG_SEED 任一为真即执行导入。
	if seed || app.SeedOnStart {
		n, _ := seedpkg.Count(posts)
		if err := seedpkg.Import(st, posts); err != nil {
			log.Fatalf("seed: %v", err)
		}
		log.Printf("seed: imported %d posts from content/posts", n)
	}

	// fillSEO：填充 CanonicalURL 和 Description，所有前台页面渲染前调用。
	fillSEO := func(data *tmpl.ViewData, c *gin.Context) {
		baseURL := ""
		if site.URL != "" {
			baseURL = site.URL
		} else {
			baseURL = "http://" + c.Request.Host
		}
		data.Page.CanonicalURL = baseURL + c.Request.URL.Path
		if data.Page.Description == "" {
			data.Page.Description = site.Description
		}
	}

	// fillSidebar：填充侧边栏动态数据（近期发布 5 篇 + 标签云前 20 + 热门 top5 + 站点统计 + 最新评论 5 条），所有前台页面渲染前调用。
	// 查询失败时留空，widget 降级为空列表，不影响主内容渲染。
	// P2-13：结果缓存 45s（TTL 内新文章/评论稍晚出现在侧栏，可接受）；CountByTag 合并为单次调用（原标签云与统计各查一次）。
	// 快照切片在请求间共享底层数组，模板渲染只读，无数据竞争。
	const popularLimit = 5
	type sidebarSnapshot struct {
		recent         []*tmpl.RecentPost
		popular        []*tmpl.RecentPost
		latestComments []*tmpl.LatestComment
		tagCloud       []*tmpl.TagItem
		hasTopics      bool
		siteStats      *tmpl.SiteStats
	}
	var (
		sidebarMu     sync.Mutex
		sidebarCached *sidebarSnapshot
		sidebarExpiry time.Time
	)
	const sidebarTTL = 45 * time.Second
	fillSidebar := func(data *tmpl.ViewData) {
		sidebarMu.Lock()
		defer sidebarMu.Unlock()
		if sidebarCached != nil && time.Now().Before(sidebarExpiry) {
			data.Recent = sidebarCached.recent
			data.Popular = sidebarCached.popular
			data.LatestComments = sidebarCached.latestComments
			data.TagCloud = sidebarCached.tagCloud
			data.HasTopics = sidebarCached.hasTopics
			data.SiteStats = sidebarCached.siteStats
			return
		}
		if posts, err := st.ListPublishedPaged(1, 5); err == nil {
			for _, p := range posts {
				data.Recent = append(data.Recent, &tmpl.RecentPost{Title: p.Title, URL: "/post/" + p.Slug})
			}
		}
		if popular, err := st.ListPopular(popularLimit); err == nil && len(popular) > 0 {
			for _, p := range popular {
				data.Popular = append(data.Popular, &tmpl.RecentPost{Title: p.Title, URL: "/post/" + p.Slug})
			}
		}
		if site.Comments.Enabled {
			if comments, err := st.ListRecentComments(5); err == nil {
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
		tagCounts, tagErr := st.CountByTag()
		if tagErr == nil {
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
			if len(items) > 20 {
				items = items[:20]
			}
			if len(items) > 0 {
				minC, maxC := items[len(items)-1].Count, items[0].Count
				for _, item := range items {
					item.Size = tmpl.TagCloudFontSize(item.Count, minC, maxC)
				}
			}
			data.TagCloud = items
		}
		// 专栏存在性：navbar「专栏」tab 显示条件（对齐原版 theme.topic.publish_list 非空）
		if topics, err := st.CountByTopic(); err == nil && len(topics) > 0 {
			data.HasTopics = true
		}
		var stats tmpl.SiteStats
		statsOK := true
		if n, err := st.CountPosts(); err == nil {
			stats.Posts = n
		} else {
			statsOK = false
		}
		if tagErr == nil {
			stats.Tags = len(tagCounts)
		} else {
			statsOK = false
		}
		if n, err := st.CountComments(store.CommentApproved); err == nil {
			stats.Comments = n
		} else {
			statsOK = false
		}
		if v, err := st.SumViews(); err == nil {
			stats.Views = v
		} else {
			statsOK = false
		}
		if statsOK {
			data.SiteStats = &stats
		}
		sidebarCached = &sidebarSnapshot{
			recent:         data.Recent,
			popular:        data.Popular,
			latestComments: data.LatestComments,
			tagCloud:       data.TagCloud,
			hasTopics:      data.HasTopics,
			siteStats:      data.SiteStats,
		}
		sidebarExpiry = time.Now().Add(sidebarTTL)
	}

	// fillComments：文章页评论区 SSR 注入（评论树 + 计数）。查询失败留空，评论区降级为空列表。
	fillComments := func(data *tmpl.ViewData, pageKey string) {
		if !site.Comments.Enabled {
			return
		}
		if comments, err := st.ListCommentsByPage(pageKey); err == nil {
			data.Comments = comment.ToViewItems(comments, &site.Comments)
		}
		if n, err := st.CountCommentsByPage(pageKey); err == nil {
			data.CommentCount = n
		}
	}

	// render404：渲染自定义 404 页（HTTP 404 + base 布局 Layout=404），NoRoute 与各路由未命中共用。
	render404 := func(c *gin.Context) {
		data := &tmpl.ViewData{
			Site: site,
			Page: &tmpl.Page{Title: "页面不存在", Layout: "404", Lang: "zh-CN"},
		}
		fillSidebar(data)
		html, err := engine.Render("layout/base", data)
		if err != nil {
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
		c.Data(http.StatusNotFound, "text/html; charset=utf-8", []byte(html))
	}

	// render500：渲染自定义 500 页（P1-8），不暴露内部错误细节。
	render500 := func(c *gin.Context) {
		data := &tmpl.ViewData{
			Site: site,
			Page: &tmpl.Page{Title: "服务器错误", Layout: "500", Lang: "zh-CN"},
		}
		fillSidebar(data)
		html, err := engine.Render("layout/base", data)
		if err != nil {
			c.String(http.StatusInternalServerError, "Internal Server Error")
			return
		}
		c.Data(http.StatusInternalServerError, "text/html; charset=utf-8", []byte(html))
	}

	// toCard：store.Post 转文章卡片视图模型，首页/分类标签列表/相关文章共用。
	// 分类配色查 site.yml article.category_color（对齐原版 category_color helper）；
	// 海报 DecodePoster 非 nil 时，与 Cover 一起触发首页 photo 封面大卡。
	toCard := func(p *store.Post) *tmpl.PostCard {
		card := &tmpl.PostCard{
			Slug:          p.Slug,
			Title:         p.Title,
			Excerpt:       p.Excerpt,
			Cover:         p.Cover,
			Date:          p.CreatedAt,
			Pinned:        p.Pinned,
			Category:      p.Category,
			CategoryStyle: tmpl.CategoryColorStyle(p.Category, site.Article.CategoryColor),
		}
		if pd := store.DecodePoster(p.Poster); pd != nil {
			card.Poster = &tmpl.PosterCard{Headline: pd.Headline, Topic: pd.Topic, Caption: pd.Caption, Color: pd.Color}
		}
		return card
	}

	// release 模式：关掉 Gin 调试日志（生产噪音），-dev 保留 debug。
	if !devMode {
		gin.SetMode(gin.ReleaseMode)
	}
	// P1-8：自定义 500——gin.New() + Logger + CustomRecovery，panic 时渲染友好错误页而非裸文本。
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.CustomRecovery(func(c *gin.Context, err interface{}) {
		log.Printf("panic recovered: %v", err)
		if c.Writer.Written() {
			c.Abort()
			return
		}
		render500(c)
		c.Abort()
	}))

	// 反向代理头：设了 trusted_proxy 才信任 X-Forwarded-For，还原客户端真实 IP。
	if app.TrustedProxy != "" {
		proxies := strings.Split(app.TrustedProxy, ",")
		for i := range proxies {
			proxies[i] = strings.TrimSpace(proxies[i])
		}
		if err := r.SetTrustedProxies(proxies); err != nil {
			log.Printf("set trusted proxies: %v", err)
		}
		r.ForwardedByClientIP = true
	}

	// P2-18：multipart 内存上限——跟随 max_upload_mb，防止超大表单解析撑爆内存（超出部分落临时文件）。
	r.MaxMultipartMemory = int64(app.MaxUploadMB+1) << 20

	// P1-5：gzip 压缩——对 HTML/CSS/JS/JSON/XML 等文本响应启用 gzip，注册在静态与路由之前。
	r.Use(gzip.Gzip(gzip.DefaultCompression))

	// P2-14：安全响应头——nosniff 防 MIME 嗅探、SAMEORIGIN 防点击劫持、Referrer-Policy 控制来源泄漏。
	// 不加全局 CSP：前台存在内联脚本/样式（原版 Stellar 结构），严格 CSP 会破坏渲染；/uploads 的 SVG 已单独 CSP 防护（P1-11）。
	r.Use(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	})

	// P1-7：/assets 长缓存——带 ?v= 版本号的 css/js/字体改 immutable 长缓存，破缓存靠 ?v= bump。
	// sw.js/manifest 不在 /assets/ 路径下，不受影响。
	r.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/assets/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		}
		c.Next()
	})

	// 访问埋点中间件（Spec 24）：仅记录内容页 GET，排除静态/API/后台/机器人。
	r.Use(trackPageView(st))

	// 静态文件 /assets：-dev 从磁盘读（改 CSS 即时生效），否则走 embed.FS（css/js/fonts）。
	if devMode {
		r.Static("/assets", filepath.Join(app.Root, "assets"))
	} else {
		sub, err := fs.Sub(assets.FS, ".")
		if err != nil {
			log.Fatalf("assets sub fs: %v", err)
		}
		r.StaticFS("/assets", http.FS(sub))
	}

	// PWA 根作用域路由：SW 必须从 /sw.js 提供才能控制全站；manifest/图标映射到 embed 或磁盘。
	serveAsset := func(assetPath, contentType string, extraHeaders map[string]string) gin.HandlerFunc {
		return func(c *gin.Context) {
			var data []byte
			var err error
			if devMode {
				data, err = os.ReadFile(filepath.Join(app.Root, "assets", assetPath))
			} else {
				data, err = assets.FS.ReadFile(assetPath)
			}
			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}
			if contentType != "" {
				c.Header("Content-Type", contentType)
			}
			for k, v := range extraHeaders {
				c.Header(k, v)
			}
			c.Data(http.StatusOK, contentType, data)
		}
	}
	r.GET("/sw.js", serveAsset("js/sw.js", "application/javascript", map[string]string{
		"Service-Worker-Allowed": "/",
		"Cache-Control":          "no-cache",
	}))
	r.GET("/manifest.webmanifest", serveAsset("manifest.webmanifest", "application/manifest+json", nil))
	r.GET("/favicon.ico", serveAsset("img/favicon.ico", "image/x-icon", nil))
	r.GET("/apple-touch-icon.png", serveAsset("img/apple-touch-icon.png", "image/png", nil))

	// 上传附件静态托管（运行时数据，不走 go:embed）
	// P1-11：SVG 防护——对 .svg 响应加 CSP sandbox，禁止内嵌脚本执行。
	r.GET("/uploads/*filepath", func(c *gin.Context) {
		fp := c.Param("filepath")
		if strings.HasSuffix(strings.ToLower(fp), ".svg") {
			c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
		}
		c.File(filepath.Join(app.UploadDir, filepath.Clean(fp)))
	})

	// 备份服务：全量备份（DB 快照 + uploads + site.yml）与文章 Markdown 导出/导入。
	// 自动备份目录跟随 DB 所在 data 目录（data/backups）。
	backupSvc := backuppkg.New(backuppkg.Config{
		Store:     st,
		Posts:     posts,
		UploadDir: app.UploadDir,
		SitePath:  siteConfigPath,
		BackupDir: filepath.Join(filepath.Dir(app.DBPath), "backups"),
		Keep:      app.BackupKeep,
		Auto:      app.AutoBackup,
	})
	if app.AutoBackup {
		backupSvc.StartAuto(24 * time.Hour)
	}

	// REST API 层（/api/v1），供 Vue Console SPA 调用
	apiHandler := api.New(api.Config{
		Store:       st,
		Posts:       posts,
		Site:        site,
		Comments:    commentHandler,
		Backups:     backupSvc,
		UploadDir:   app.UploadDir,
		NotesPath:   filepath.Join(app.Root, "content", "notes.md"),
		MaxUploadMB: app.MaxUploadMB,
	})
	apiHandler.Register(r)

	// 友链动态聚合：后台定时抓取配置友链（site.yml links 带 rss）的 RSS/Atom 最新文章，
	// /api/friends-aggregated 供 /friends 页的 ds-friends_and_posts 组件异步渲染。
	friendsAgg := friends.New(site)
	friendsAgg.Start()
	friendsAgg.Register(r)
	// 友链在 console 增删改后，触发聚合器即时重建缓存，使 /friends 页即时反映。
	apiHandler.SetOnLinksChanged(friendsAgg.Refresh)

	// GitHub 代理端点（Spec 21）：/api/gh/repo（直连 api.github.com；ghClient 已在 seed 前构造并注入 posts）。
	ghpkg.NewHandler(ghClient).Register(r)

	// 评分数据服务（Spec 20）：/api/rating 公开端点。
	if err := st.EnsureRatingsTable(); err != nil {
		log.Fatalf("ensure ratings table: %v", err)
	}
	store.NewRatingHandler(st).Register(r)

	// 文章版本历史（Spec 22）：revisions 表（端点随 api.Handler 注册）。
	if err := st.EnsureRevisionsTable(); err != nil {
		log.Fatalf("ensure revisions table: %v", err)
	}

	// 访问统计明细表（Spec 24）：page_views 表 + 索引。
	if err := st.EnsurePageViewsTable(); err != nil {
		log.Fatalf("ensure page_views table: %v", err)
	}

	// Vue Console SPA 静态托管（/console/）：默认走 go:embed（单二进制），-dev 从磁盘 console/dist 读。
	// 只注册 /console/assets/* 与根级静态文件；其余 /console 路径（含 /console/ 本身与 SPA 前端路由）
	// 落到 NoRoute 返回 index.html，保证前端路由刷新不 404。
	var consoleIndex []byte
	if devMode {
		consoleDir := filepath.Join(app.Root, "console", "dist")
		if info, err := os.Stat(consoleDir); err == nil && info.IsDir() {
			r.Static("/console/assets", filepath.Join(consoleDir, "assets"))
			// 根级静态文件（favicon.svg 等）
			if entries, err := os.ReadDir(consoleDir); err == nil {
				for _, e := range entries {
					if !e.IsDir() && e.Name() != "index.html" {
						name := e.Name()
						r.StaticFile("/console/"+name, filepath.Join(consoleDir, name))
					}
				}
			}
			consoleIndex, _ = os.ReadFile(filepath.Join(consoleDir, "index.html"))
			log.Printf("console SPA loaded from %s (dev)", consoleDir)
		} else {
			log.Printf("console dist not found: %s (dev), /console/ unavailable", consoleDir)
		}
	} else {
		// embed 模式：console.DistFS（all:dist）
		assetsSub, err := fs.Sub(console.DistFS, "dist/assets")
		if err != nil {
			log.Fatalf("console assets sub fs: %v", err)
		}
		r.StaticFS("/console/assets", http.FS(assetsSub))
		// 根级静态文件（favicon.svg / icons.svg 等）
		entries, err := console.DistFS.ReadDir("dist")
		if err != nil {
			log.Fatalf("read console dist (embed): %v", err)
		}
		for _, e := range entries {
			if e.IsDir() || e.Name() == "index.html" {
				continue
			}
			name := e.Name()
			r.GET("/console/"+name, func(c *gin.Context) {
				data, err := console.DistFS.ReadFile("dist/" + name)
				if err != nil {
					c.Status(http.StatusNotFound)
					return
				}
				c.Data(http.StatusOK, consoleContentType(name), data)
			})
		}
		consoleIndex, err = console.DistFS.ReadFile("dist/index.html")
		if err != nil {
			log.Fatalf("read console index.html (embed): %v", err)
		}
		log.Printf("console SPA loaded from embed")
	}

	// 管理后台已统一到 Vue Console（/console/ + /api/v1）；服务端渲染后台 /admin 与 /admin/media 已退役。

	// 文章密码解锁
	r.POST("/post/:slug/unlock", func(c *gin.Context) {
		slug := c.Param("slug")
		p, err := st.GetPublishedBySlug(slug)
		if err != nil || p == nil || !p.IsPasswordProtected() {
			c.Redirect(http.StatusFound, "/post/"+slug)
			return
		}
		input := c.PostForm("password")
		if subtle.ConstantTimeCompare([]byte(input), []byte(p.Password)) != 1 {
			data := &tmpl.ViewData{
				Site: site,
				Page: &tmpl.Page{
					Title:       p.Title,
					Slug:        p.Slug,
					Layout:      "post-password",
					Lang:        "zh-CN",
					MenuID:      "post",
					Description: "密码错误，请重试",
				},
			}
			fillSidebar(data)
			fillSEO(data, c)
			html, err := engine.Render("layout/base", data)
			if err != nil {
				log.Printf("render: %v", err)
				render500(c)
				return
			}
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
			return
		}
		expiry := time.Now().Add(postPWCookieTTL).Unix()
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(postPWCookieName(slug), signPostPWCookie(slug, p.Password, expiry), int(postPWCookieTTL.Seconds()), "/", "", cookieSecureFor(c), true)
		c.Redirect(http.StatusFound, "/post/"+slug)
	})

	// 独立页面前台 /pages/:slug（后台管理已迁移到 Vue Console + /api/v1）
	pageHandler, err := page.New(st, posts, engine, site)
	if err != nil {
		log.Fatalf("init page: %v", err)
	}
	pageHandler.SetRender404(render404)
	pageHandler.SetFillExtra(func(data *tmpl.ViewData, c *gin.Context) {
		fillSidebar(data)
		fillSEO(data, c)
	})
	pageHandler.Register(r)

	momentHandler, err := moment.New(st, posts, engine, site, fillSidebar, fillSEO)
	if err != nil {
		log.Fatalf("init moments: %v", err)
	}
	momentHandler.Register(r)

	// 自建评论系统公开 API（/api/comments），comments.enabled 时注册
	if site.Comments.Enabled {
		commentHandler.Register(r)
	}

	// 未匹配路由：/console/* 走 SPA fallback，其余走自定义 404 页
	r.NoRoute(func(c *gin.Context) {
		if consoleIndex != nil && strings.HasPrefix(c.Request.URL.Path, "/console") {
			c.Data(http.StatusOK, "text/html; charset=utf-8", consoleIndex)
			return
		}
		render404(c)
	})

	// 文章路由 /post/:slug：DB 优先（published），未命中回退文件渲染（向后兼容 Demo 2）
	r.GET("/post/:slug", func(c *gin.Context) {
		slug := c.Param("slug")

		// 优先查 DB
		if p, err := st.GetPublishedBySlug(slug); err == nil && p != nil {
			// 密码门：未解锁则渲染口令表单（不增加阅读量、不泄漏正文）
			if p.IsPasswordProtected() {
				cookie, _ := c.Cookie(postPWCookieName(slug))
				if !validPostPWCookie(cookie, slug, p.Password) {
					data := &tmpl.ViewData{
						Site: site,
						Page: &tmpl.Page{
							Title:  p.Title,
							Slug:   p.Slug,
							Layout: "post-password",
							Lang:   "zh-CN",
							MenuID: "post",
						},
					}
					fillSidebar(data)
					fillSEO(data, c)
					html, err := engine.Render("layout/base", data)
					if err != nil {
						log.Printf("render: %v", err)
						render500(c)
						return
					}
					c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
					return
				}
			}
			// 阅读量自增：每次访问 /post/:slug 计数 +1。失败仅记日志，不中断渲染。
			if err := st.IncrViews(slug); err != nil {
				log.Printf("incr views for %s: %v", slug, err)
			}
			views, err := st.GetViews(slug)
			if err != nil {
				log.Printf("get views for %s: %v", slug, err)
			}
			data := &tmpl.ViewData{
				Site: site,
				Page: &tmpl.Page{
					Title:      p.Title,
					Slug:       p.Slug,
					Layout:     "post",
					Type:       "tech",
					Lang:       "zh-CN",
					MenuID:     "post",
					Date:       p.CreatedAt.Format("2006-01-02"),
					Excerpt:    p.Excerpt,
					Views:      views,
					Topic:      p.Topic,
					References: store.DecodeReferences(p.References),
				},
				Content: template.HTML(p.ContentHTML),
			}
			data.TOC = tmpl.ExtractTOC(p.ContentHTML)
			// 左栏「专栏：xxx」系列文章 widget（对齐 widgets/related.ejs relatedPostsInTopic）
			if p.Topic != "" {
				if series, err := st.ListByTopic(p.Topic); err == nil && len(series) > 0 {
					ts := &tmpl.TopicSeries{Name: p.Topic}
					for _, sp := range series {
						ts.Posts = append(ts.Posts, &tmpl.TopicSeriesPost{Slug: sp.Slug, Title: sp.Title, Active: sp.Slug == slug})
					}
					data.TopicSeries = ts
				}
			}
			// 上下篇导航
			if prev, err := st.GetPrevPost(slug); err == nil && prev != nil {
				data.PrevPost = &tmpl.AdjacentPost{Slug: prev.Slug, Title: prev.Title}
			}
			if next, err := st.GetNextPost(slug); err == nil && next != nil {
				data.NextPost = &tmpl.AdjacentPost{Slug: next.Slug, Title: next.Title}
			}
			// 相关文章（按标签匹配，最多 4 篇）
			if related, err := st.ListRelatedByTags(slug, p.Tags, 4); err == nil && len(related) > 0 {
				cards := make([]*tmpl.PostCard, 0, len(related))
				for _, rp := range related {
					cards = append(cards, toCard(rp))
				}
				data.RelatedPosts = cards
			}
			// 文章描述：优先 Excerpt，否则截断正文前 150 字符（去 HTML 标签）
			desc := p.Excerpt
			if desc == "" {
				d := stripHTML(p.ContentHTML)
				if len(d) > 150 {
					d = d[:150]
				}
				desc = d
			}
			data.Page.Description = desc
			fillSidebar(data)
			fillComments(data, "/post/"+slug)
			fillSEO(data, c)
			html, err := engine.Render("layout/base", data)
			if err != nil {
				log.Printf("render: %v", err)
				render500(c)
				return
			}
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
			return
		}

		// 回退：文件渲染（Demo 2 链路）
		p, err := posts.Load(slug)
		if err != nil {
			if os.IsNotExist(err) {
				render404(c)
				return
			}
			log.Printf("load post: %v", err)
			render500(c)
			return
		}

		data := &tmpl.ViewData{
			Site: site,
			Page: &tmpl.Page{
				Title:      p.Title,
				Slug:       p.Slug,
				Layout:     "post",
				Type:       "tech",
				Lang:       "zh-CN",
				MenuID:     "post",
				Date:       p.Date,
				Excerpt:    p.Excerpt,
				References: p.References,
			},
			Content: template.HTML(p.HTML),
		}

		data.TOC = tmpl.ExtractTOC(p.HTML)
		desc := p.Excerpt
		if desc == "" {
			d := stripHTML(p.HTML)
			if len(d) > 150 {
				d = d[:150]
			}
			desc = d
		}
		data.Page.Description = desc
		fillSidebar(data)
		fillComments(data, "/post/"+slug)
		fillSEO(data, c)
		html, err := engine.Render("layout/base", data)
		if err != nil {
			log.Printf("render: %v", err)
			render500(c)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	})

	// 服务端全文搜索（Spec 25）：FTS5 bm25 排序 + snippet 高亮。
	// 前端 local-search.js 优先走此端点，失败降级回 /search.json 本地匹配。
	r.GET("/api/search", func(c *gin.Context) {
		if searchIdx == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "server search disabled"})
			return
		}
		q := strings.TrimSpace(c.Query("q"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		results, err := searchIdx.Search(q, limit)
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

	// OG 社交分享图（Spec 27）：/og/:slug.png，磁盘缓存 + 写时失效。
	r.GET("/og/:slug", func(c *gin.Context) {
		slug := c.Param("slug")
		slug = strings.TrimSuffix(slug, ".png")
		if slug == "" {
			c.Status(http.StatusNotFound)
			return
		}
		p, err := st.GetBySlug(slug)
		if err != nil || p == nil || p.Status != store.StatusPublished {
			c.Status(http.StatusNotFound)
			return
		}
		data, err := ogGen.GetOrGenerate(slug, og.PostInfo{
			Title:     p.Title,
			Date:      p.CreatedAt,
			SiteTitle: site.Title,
		})
		if err != nil {
			log.Printf("og: generate %s failed: %v", slug, err)
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Header("Cache-Control", "public, max-age=86400")
		c.Data(http.StatusOK, "image/png", data)
	})

	// 本地搜索索引：全量 published 文章的标题/正文纯文本，前端 local-search.js 拉取。
	// no-store：索引需实时反映 admin 改动，禁止浏览器/CDN 缓存。
	r.GET("/search.json", func(c *gin.Context) {
		posts, err := st.ListPublished()
		if err != nil {
			log.Printf("list posts: %v", err)
			render500(c)
			return
		}
		type searchItem struct {
			Title   string `json:"title"`
			Content string `json:"content"`
			Path    string `json:"path"`
		}
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
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, items)
	})

	// 首页渲染：/ 与 /page/:num 共用。page 从 1 开始，超出末页回退到最后一页。
	renderHome := func(c *gin.Context, page int) {
		const pageSize = 10
		total, err := st.CountPublished()
		if err != nil {
			log.Printf("count posts: %v", err)
			render500(c)
			return
		}
		totalPages := (total + pageSize - 1) / pageSize
		if page < 1 {
			page = 1
		}
		if totalPages > 0 && page > totalPages {
			page = totalPages
		}
		posts, err := st.ListPublishedPaged(page, pageSize)
		if err != nil {
			log.Printf("list posts: %v", err)
			render500(c)
			return
		}
		cards := make([]*tmpl.PostCard, 0, len(posts))
		for _, p := range posts {
			cards = append(cards, toCard(p))
		}
		pages := make([]int, totalPages)
		for i := range pages {
			pages[i] = i + 1
		}
		data := &tmpl.ViewData{
			Site:      site,
			Page:      &tmpl.Page{Title: site.Title, Layout: "index", MenuID: "post", Lang: "zh-CN"},
			Posts:     cards,
			Paginator: &tmpl.Paginator{Current: page, Total: totalPages, Pages: pages},
		}
		fillSidebar(data)
		fillSEO(data, c)
		html, err := engine.Render("layout/base", data)
		if err != nil {
			log.Printf("render: %v", err)
			render500(c)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	}

	// 首页：文章列表（对齐 Stellar layout/index.ejs）
	r.GET("/", func(c *gin.Context) {
		renderHome(c, 1)
	})
	// 首页分页：/page/N，非数字或 <1 返回 404
	r.GET("/page/:num", func(c *gin.Context) {
		n, err := strconv.Atoi(c.Param("num"))
		if err != nil || n < 1 {
			render404(c)
			return
		}
		renderHome(c, n)
	})

	// renderBase：渲染 base 布局并写响应（归档/分类/标签页共用）。
	renderBase := func(c *gin.Context, data *tmpl.ViewData) {
		fillSidebar(data)
		fillSEO(data, c)
		html, err := engine.Render("layout/base", data)
		if err != nil {
			log.Printf("render: %v", err)
			render500(c)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	}

	// toCards：store.Post 列表转首页卡片视图模型。
	toCards := func(posts []*store.Post) []*tmpl.PostCard {
		cards := make([]*tmpl.PostCard, 0, len(posts))
		for _, p := range posts {
			cards = append(cards, toCard(p))
		}
		return cards
	}

	// 友链页：site.yml links 配置驱动（对齐 Stellar sites 卡片布局）
	r.GET("/links", func(c *gin.Context) {
		renderBase(c, &tmpl.ViewData{
			Site: site,
			Page: &tmpl.Page{Title: "友链", Layout: "links", MenuID: "friends", Lang: "zh-CN"},
		})
	})

	// 文库 /wiki（+ /wiki/tags/:tag 标签过滤），对齐 Stellar index_wiki。配置驱动（site.yml wiki 段）。
	wikiHandler := func(c *gin.Context) {
		tag := c.Param("tag")
		// 收集全量标签（顶部 tab，保留首见顺序）
		var tags []string
		tagSeen := map[string]bool{}
		for i := range site.Wiki {
			for _, t := range site.Wiki[i].Tags {
				if !tagSeen[t] {
					tagSeen[t] = true
					tags = append(tags, t)
				}
			}
		}
		if tag != "" && !tagSeen[tag] {
			render404(c)
			return
		}
		// 项目（可按 tag 过滤），按 Sort 降序
		projects := make([]*tmpl.WikiProject, 0, len(site.Wiki))
		for i := range site.Wiki {
			w := &site.Wiki[i]
			if tag != "" {
				matched := false
				for _, t := range w.Tags {
					if t == tag {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
			projects = append(projects, w)
		}
		sort.SliceStable(projects, func(i, j int) bool { return projects[i].Sort > projects[j].Sort })
		renderBase(c, &tmpl.ViewData{
			Site:         site,
			Page:         &tmpl.Page{Title: "文库", Layout: "wiki", MenuID: "wiki", Lang: "zh-CN", Tag: tag},
			WikiProjects: projects,
			WikiTags:     tags,
		})
	}
	r.GET("/wiki", wikiHandler)
	r.GET("/wiki/", wikiHandler)
	r.GET("/wiki/tags/:tag", wikiHandler)

	// 笔记(手记) /notes，对齐 xaoxuu.com/notes/：上段 content/notes.md 手记正文（quot/okr 等 shortcode），下段本站 moments 近期动态。
	notesMDPath := filepath.Join(app.Root, "content", "notes.md")
	r.GET("/notes", func(c *gin.Context) {
		var contentHTML template.HTML
		if raw, err := os.ReadFile(notesMDPath); err == nil {
			if rendered, err := posts.Render(string(raw)); err == nil {
				contentHTML = template.HTML(rendered)
			}
		}
		renderBase(c, &tmpl.ViewData{
			Site:             site,
			Page:             &tmpl.Page{Title: "笔记", Layout: "notes", MenuID: "notes", Lang: "zh-CN"},
			NotesContentHTML: contentHTML,
			Moments:          momentHandler.RecentItems(5),
		})
	})

	// 友链 /friends（友链卡片 + RSS 动态聚合），数据由 friendsAgg 的 /api/friends-aggregated 提供。
	r.GET("/friends", func(c *gin.Context) {
		renderBase(c, &tmpl.ViewData{
			Site: site,
			Page: &tmpl.Page{Title: "友链", Layout: "friends", MenuID: "friends", Lang: "zh-CN"},
		})
	})

	// 归档页：按年份分组的文章列表（对齐 Stellar archive.ejs）
	r.GET("/archives", func(c *gin.Context) {
		posts, err := st.ListPublished()
		if err != nil {
			log.Printf("list posts: %v", err)
			render500(c)
			return
		}
		// ListPublished 已按 created_at DESC，年份分组天然降序
		var years []*tmpl.ArchiveYear
		byYear := map[string]*tmpl.ArchiveYear{}
		for _, p := range posts {
			y := p.CreatedAt.Format("2006")
			ay, ok := byYear[y]
			if !ok {
				ay = &tmpl.ArchiveYear{Year: y}
				byYear[y] = ay
				years = append(years, ay)
			}
			title := p.Title
			if title == "" {
				title = p.CreatedAt.Format("2006-01-02") // 无标题用日期（对齐 Stellar 行为）
			}
			ay.Posts = append(ay.Posts, &tmpl.ArchiveItem{Slug: p.Slug, Title: title, Date: p.CreatedAt})
		}
		renderBase(c, &tmpl.ViewData{
			Site:     site,
			Page:     &tmpl.Page{Title: "归档", Layout: "archive", MenuID: "archive", Lang: "zh-CN"},
			Archives: years,
		})
	})

	// 分类索引页：所有分类 + 文章数（对齐 Stellar categories.ejs）
	r.GET("/categories", func(c *gin.Context) {
		counts, err := st.CountByCategory()
		if err != nil {
			log.Printf("count categories: %v", err)
			render500(c)
			return
		}
		cats := make([]*tmpl.CatItem, 0, len(counts))
		for name, n := range counts {
			cats = append(cats, &tmpl.CatItem{Name: name, Count: n})
		}
		sort.Slice(cats, func(i, j int) bool { return cats[i].Name < cats[j].Name })
		renderBase(c, &tmpl.ViewData{
			Site: site,
			Page: &tmpl.Page{Title: "分类", Layout: "categories", MenuID: "post", Lang: "zh-CN"},
			Cats: cats,
		})
	})

	// 分类文章列表页：复用首页卡片（对齐 Stellar archive.ejs 的 partial('index') 行为）
	r.GET("/categories/:name", func(c *gin.Context) {
		name := c.Param("name")
		counts, err := st.CountByCategory()
		if err != nil {
			log.Printf("count categories: %v", err)
			render500(c)
			return
		}
		if _, ok := counts[name]; !ok {
			render404(c)
			return
		}
		posts, err := st.ListByCategory(name)
		if err != nil {
			log.Printf("list posts: %v", err)
			render500(c)
			return
		}
		renderBase(c, &tmpl.ViewData{
			Site:  site,
			Page:  &tmpl.Page{Title: "分类：" + name, Layout: "index", MenuID: "post", Lang: "zh-CN", Category: name},
			Posts: toCards(posts),
		})
	})

	// 专栏索引页：专栏卡片列表（对齐 Stellar index_topic.ejs，图标缺省用主题 default.topic 占位图）
	r.GET("/topic", func(c *gin.Context) {
		counts, err := st.CountByTopic()
		if err != nil {
			log.Printf("count topics: %v", err)
			render500(c)
			return
		}
		if len(counts) == 0 {
			render404(c)
			return
		}
		topics := make([]*tmpl.TopicItem, 0, len(counts))
		for name, n := range counts {
			topics = append(topics, &tmpl.TopicItem{
				Name:  name,
				Count: n,
				Icon:  "https://gcore.jsdelivr.net/gh/cdn-x/placeholder@1.0.12/image/10433048.png",
			})
		}
		sort.Slice(topics, func(i, j int) bool { return topics[i].Name < topics[j].Name })
		renderBase(c, &tmpl.ViewData{
			Site:   site,
			Page:   &tmpl.Page{Title: "专栏", Layout: "topics", MenuID: "post", Lang: "zh-CN"},
			Topics: topics,
		})
	})

	// 专栏文章列表页：复用首页卡片（对齐原版 topic 文章列表）
	r.GET("/topic/:name", func(c *gin.Context) {
		name := c.Param("name")
		counts, err := st.CountByTopic()
		if err != nil {
			log.Printf("count topics: %v", err)
			render500(c)
			return
		}
		if _, ok := counts[name]; !ok {
			render404(c)
			return
		}
		posts, err := st.ListByTopic(name)
		if err != nil {
			log.Printf("list posts: %v", err)
			render500(c)
			return
		}
		renderBase(c, &tmpl.ViewData{
			Site:  site,
			Page:  &tmpl.Page{Title: "专栏：" + name, Layout: "index", MenuID: "post", Lang: "zh-CN", Topic: name},
			Posts: toCards(posts),
		})
	})

	// 标签索引页：标签云（按文章数 DESC）
	r.GET("/tags", func(c *gin.Context) {
		counts, err := st.CountByTag()
		if err != nil {
			log.Printf("count tags: %v", err)
			render500(c)
			return
		}
		items := make([]*tmpl.TagItem, 0, len(counts))
		for name, n := range counts {
			items = append(items, &tmpl.TagItem{Name: name, Count: n})
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].Count != items[j].Count {
				return items[i].Count > items[j].Count
			}
			return items[i].Name < items[j].Name
		})
		renderBase(c, &tmpl.ViewData{
			Site:     site,
			Page:     &tmpl.Page{Title: "标签", Layout: "tags", MenuID: "post", Lang: "zh-CN"},
			TagItems: items,
		})
	})

	// 标签文章列表页：复用首页卡片
	r.GET("/tags/:name", func(c *gin.Context) {
		name := c.Param("name")
		counts, err := st.CountByTag()
		if err != nil {
			log.Printf("count tags: %v", err)
			render500(c)
			return
		}
		if _, ok := counts[name]; !ok {
			render404(c)
			return
		}
		posts, err := st.ListByTag(name)
		if err != nil {
			log.Printf("list posts: %v", err)
			render500(c)
			return
		}
		renderBase(c, &tmpl.ViewData{
			Site:  site,
			Page:  &tmpl.Page{Title: "标签：" + name, Layout: "index", MenuID: "post", Lang: "zh-CN", Tag: name},
			Posts: toCards(posts),
		})
	})

	// Atom / JSON Feed / 分类标签 RSS / robots.txt
	feed.New(st, site).Register(r)

	// RSS 2.0 订阅输出：/rss.xml，最新 20 篇 published 文章。
	r.GET("/rss.xml", func(c *gin.Context) {
		posts, err := st.ListPublishedPaged(1, 20)
		if err != nil {
			log.Printf("list posts: %v", err)
			render500(c)
			return
		}
		siteURL := "http://" + c.Request.Host
		items := make([]Item, 0, len(posts))
		for _, p := range posts {
			desc := p.Excerpt
			if p.IsPasswordProtected() {
				desc = "本文受密码保护"
			} else if desc == "" {
				desc = stripHTML(p.ContentHTML)
				if len(desc) > 200 {
					desc = desc[:200]
				}
			}
			link := siteURL + "/post/" + p.Slug
			items = append(items, Item{
				Title:       p.Title,
				Link:        link,
				Description: desc,
				PubDate:     p.CreatedAt.Format(time.RFC1123Z),
				Guid:        link,
			})
		}
		rss := RSS{
			Version: "2.0",
			Channel: Channel{
				Title:       site.Title,
				Link:        siteURL,
				Description: site.Description,
				Items:       items,
			},
		}
		output, err := xml.MarshalIndent(rss, "", "  ")
		if err != nil {
			log.Printf("marshal rss: %v", err)
			render500(c)
			return
		}
		c.Data(http.StatusOK, "application/rss+xml; charset=utf-8", append([]byte(xml.Header), output...))
	})

	// Sitemap 输出：/sitemap.xml，包含首页、所有已发布文章、静态索引页。
	r.GET("/sitemap.xml", func(c *gin.Context) {
		baseURL := ""
		if site.URL != "" {
			baseURL = site.URL
		} else {
			baseURL = "http://" + c.Request.Host
		}

		type SitemapURL struct {
			Loc      string `xml:"loc"`
			LastMod  string `xml:"lastmod,omitempty"`
			Priority string `xml:"priority,omitempty"`
		}
		type Sitemap struct {
			XMLName xml.Name     `xml:"urlset"`
			XMLNS   string       `xml:"xmlns,attr"`
			URLs    []SitemapURL `xml:"url"`
		}

		sm := Sitemap{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9"}

		// 首页
		sm.URLs = append(sm.URLs, SitemapURL{Loc: baseURL + "/", Priority: "1.0"})

		// 所有已发布文章
		posts, err := st.ListPublished()
		if err == nil {
			for _, p := range posts {
				sm.URLs = append(sm.URLs, SitemapURL{
					Loc:     baseURL + "/post/" + p.Slug,
					LastMod: p.UpdatedAt.Format("2006-01-02"),
				})
			}
		}

		// 静态页面
		sm.URLs = append(sm.URLs, SitemapURL{Loc: baseURL + "/archives"})
		sm.URLs = append(sm.URLs, SitemapURL{Loc: baseURL + "/categories"})
		sm.URLs = append(sm.URLs, SitemapURL{Loc: baseURL + "/tags"})
		sm.URLs = append(sm.URLs, SitemapURL{Loc: baseURL + "/links"})
		sm.URLs = append(sm.URLs, SitemapURL{Loc: baseURL + "/moments"})

		output, err := xml.MarshalIndent(sm, "", "  ")
		if err != nil {
			log.Printf("marshal sitemap: %v", err)
			render500(c)
			return
		}
		c.Data(http.StatusOK, "application/xml; charset=utf-8", append([]byte(xml.Header), output...))
	})

	// P0-1：优雅退出——显式 http.Server + 信号监听，SIGINT/SIGTERM 时 drain 进行中请求后关闭 DB。
	addr := app.ListenAddr
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		fmt.Printf("Stellar blog server listening on http://localhost%s\n", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down gracefully (timeout 30s)...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	log.Println("server stopped")
}

// truncateText 按 rune 截断字符串，超长加省略号（widget 摘要用）。
func truncateText(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// ghAdapter 将 github.Client 适配为 post.GHClient 接口（解耦循环依赖）。
type ghAdapter struct{ c *ghpkg.Client }

func (a *ghAdapter) GetRepo(owner, repo string) (*post.GHRepoInfo, error) {
	info, err := a.c.GetRepo(owner, repo)
	if err != nil {
		return nil, err
	}
	return &post.GHRepoInfo{
		Name: info.Name, FullName: info.FullName, Description: info.Description,
		Stars: info.Stars, Forks: info.Forks, Language: info.Language,
		License: info.License, OwnerAvatar: info.OwnerAvatar, OwnerLogin: info.OwnerLogin, HTMLURL: info.HTMLURL,
	}, nil
}

func (a *ghAdapter) GetUser(username string) (*post.GHUserInfo, error) {
	info, err := a.c.GetUser(username)
	if err != nil {
		return nil, err
	}
	return &post.GHUserInfo{
		Login: info.Login, Name: info.Name, Bio: info.Bio, AvatarURL: info.AvatarURL,
		Followers: info.Followers, Following: info.Following, Repos: info.Repos, HTMLURL: info.HTMLURL,
	}, nil
}

// consoleContentType 按扩展名推断 console 根级静态文件的 Content-Type。
// 常见类型显式返回（不依赖运行环境 mime 数据库，alpine 精简镜像也能正确），
// 其余 fallback 到 mime.TypeByExtension，再不行用 octet-stream。
func consoleContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".svg":
		return "image/svg+xml"
	case ".js":
		return "application/javascript"
	case ".css":
		return "text/css; charset=utf-8"
	case ".png":
		return "image/png"
	case ".ico":
		return "image/x-icon"
	case ".html":
		return "text/html; charset=utf-8"
	default:
		if ct := mime.TypeByExtension(filepath.Ext(name)); ct != "" {
			return ct
		}
		return "application/octet-stream"
	}
}

// --- 访问埋点（Spec 24） ---

// botUASubstrings 常见爬虫/机器人 UA 关键词（小写匹配）。
var botUASubstrings = []string{
	"bot", "crawl", "spider", "slurp", "mediapartners",
	"curl", "wget", "python-requests", "go-http-client",
	"java/", "scrapy", "httpclient", "feedfetcher",
}

// trackPageView 返回 Gin 中间件：仅记录内容页 GET 请求的访问明细。
// 白名单制：只记 /、/post/*、/notes、/friends、/moments、/archives 等内容路径。
func trackPageView(st *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}
		p := c.Request.URL.Path
		// 排除非内容路径
		if strings.HasPrefix(p, "/assets") || strings.HasPrefix(p, "/api") ||
			strings.HasPrefix(p, "/console") || strings.HasPrefix(p, "/sw.js") ||
			p == "/manifest.webmanifest" || p == "/favicon.svg" ||
			p == "/search.json" || p == "/atom.xml" || p == "/rss.xml" ||
			p == "/robots.txt" {
			c.Next()
			return
		}
		// 白名单：仅记录内容页
		isContent := p == "/" || p == "/notes" || p == "/friends" ||
			p == "/moments" || p == "/archives" ||
			strings.HasPrefix(p, "/post/") || strings.HasPrefix(p, "/page/") ||
			strings.HasPrefix(p, "/category/") || strings.HasPrefix(p, "/tags/")
		if !isContent {
			c.Next()
			return
		}
		// 过滤机器人 UA
		ua := strings.ToLower(c.Request.UserAgent())
		for _, kw := range botUASubstrings {
			if strings.Contains(ua, kw) {
				c.Next()
				return
			}
		}
		// ua_hash = fnv32(UA + IP)，不存明文，隐私友好
		h := fnv.New32a()
		h.Write([]byte(c.Request.UserAgent() + c.ClientIP()))
		uaHash := fmt.Sprintf("%x", h.Sum32())

		referrer := c.Request.Referer()
		_ = st.RecordView(p, referrer, uaHash)
		c.Next()
	}
}
