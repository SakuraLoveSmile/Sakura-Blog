package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"blog/frontend/assets"
	"blog/internal/api"
	backuppkg "blog/internal/backup"
	"blog/internal/comment"
	"blog/internal/config"
	"blog/internal/friends"
	ghpkg "blog/internal/github"
	"blog/internal/linkwatch"
	"blog/internal/module"
	_ "blog/internal/module/perf" // blank import：触发性能监控模块 init() 自注册
	_ "blog/internal/module/seo"  // blank import：触发 SEO 模块 init() 自注册
	"blog/internal/moment"
	"blog/internal/notify"
	"blog/internal/og"
	ogfonts "blog/internal/og/fonts"
	"blog/internal/page"
	"blog/internal/post"
	seedpkg "blog/internal/seed"
	"blog/internal/store"
	tmpl "blog/internal/template"
)

// 资源加载策略：
//   - 默认（部署）：icons/templates/assets 全部走 go:embed，单二进制无外部依赖。
//   - -dev 模式（本地调试）：从磁盘读 icons.yml、internal/template、assets/，改样式/模板即时生效。
//   - root 用于定位 config（app.yml/site.yml）、DB 与 content；embed 模式下仅作目录基准。
//
// 不能依赖 os.Executable()：go run 编译到临时目录，二进制路径不在源码目录。
//
// 路由注册与依赖装配已按域拆分：渲染辅助与共享依赖见 web.go（webServer），
// 各域路由见 routes_*.go，API 侧装配见 internal/api 的 Mount，main 只做串接。
func main() {
	var (
		root       string
		seed       bool
		devMode    bool
		iconsPath  string
		configPath string
	)
	flag.StringVar(&root, "root", "", "工作根目录（定位 config/DB/data；空则用 cwd 或 app.yml）")
	flag.BoolVar(&seed, "seed", false, "启动时把 content/posts/*.md 导入 SQLite（按 slug UPSERT）")
	flag.BoolVar(&devMode, "dev", false, "开发模式：从磁盘读 icons/templates/assets（默认走 go:embed）")
	flag.StringVar(&iconsPath, "icons", "", "icons.yml 路径（仅 -dev 模式生效，默认 <root>/internal/template/data/icons.yml）")
	flag.StringVar(&configPath, "config", "", "app.yml 路径（默认 <root>/backend/config/app.yml）")
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
		appCfgPath = filepath.Join(bootstrapRoot, "backend", "config", "app.yml")
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
			iconsPath = filepath.Join(app.Root, "backend", "internal", "template", "data", "icons.yml")
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
		engine, err = tmpl.NewEngine(filepath.Join(app.Root, "backend", "internal", "template"), reg)
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

	// 资源版本号：内容哈希自动生成（asset_ver 模板函数消费），根治手写 ?v= 遗漏旧缓存。
	tmpl.SetAssetVersioner(newAssetVersioner(devMode, app.Root))

	// 文章服务：content/posts/*.md（文件回退用 + seed 源）。按 root 定位，部署时可选。
	posts := post.NewService(filepath.Join(app.Root, "content", "posts"))
	posts.SetIconFunc(reg.Get) // 供 quot 等 shortcode 渲染内联 SVG（复用模板 icon 同一套图标）
	// 模板内 Markdown 渲染（页脚自定义文案等）：复用文章渲染管线（shortcode + 消毒）
	tmpl.SetMarkdownRender(func(s string) template.HTML {
		h, err := posts.Render(s)
		if err != nil {
			log.Printf("render footer markdown: %v", err)
			return ""
		}
		return template.HTML(h)
	})

	// SQLite 持久化：DBPath 已在 ResolveApp 绝对化。
	st, err := store.New(app.DBPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()
	log.Printf("sqlite ready: %s", app.DBPath)

	// Spec 52 专栏迁移：建 topics 表 + posts 表加 topic_id 列 + 存量 topic 文本回填（幂等）。
	// 必须在任何 listCols 查询（setupSearchIndex 等）之前执行：listCols 已含 topic_id 列。
	if err := st.EnsureTopicsTable(); err != nil {
		log.Fatalf("ensure topics table: %v", err)
	}
	if err := st.MigrateTopicBackfill(); err != nil {
		log.Fatalf("migrate topic backfill: %v", err)
	}

	// FTS5 全文索引（Spec 25）：在 seed 前装配，保证导入也能同步。
	searchIdx := setupSearchIndex(st)

	// OG 社交分享图生成器（Spec 27）：字体走 OG 专用 embed（Spec 61 阶段 1 移出 /assets 公网），
	// dev 分支直接读磁盘，embed 分支读出字体写入缓存目录供 gg 加载，缓存到 data/ogcache/。
	ogCacheDir := filepath.Join(filepath.Dir(app.DBPath), "ogcache")
	var ogFontPath string
	if devMode {
		ogFontPath = filepath.Join(app.Root, "backend", "internal", "og", "fonts", "LXGWWenKai-Regular.ttf")
	} else {
		if data, err := ogfonts.FS.ReadFile("LXGWWenKai-Regular.ttf"); err == nil {
			_ = os.MkdirAll(ogCacheDir, 0o755)
			ogFontPath = filepath.Join(ogCacheDir, ".font.ttf")
			_ = os.WriteFile(ogFontPath, data, 0o644)
		}
	}
	ogGen := og.New(ogFontPath, ogCacheDir)

	// store 写路径统一回调：搜索索引同步 + OG 缓存失效（文章/页面/瞬间三类）。
	// 内容派生缓存（Spec 61 阶段 5）：/search.json 与 /sitemap.xml 写路径置脏。
	// 注意 ws.contentCache 在 newWebServer 中创建，此处 hook 需在 ws 之后注入——
	// store hook 由下方 newWebServer 之后统一包装（见 post/page/moment 三处 Set*WriteHook 重新赋值）。
	postWriteHook := newPostWriteHook(st, searchIdx, ogGen)
	pageWriteHook := newPageWriteHook(st, searchIdx)
	momentWriteHook := newMomentWriteHook(st, searchIdx)
	st.SetPostWriteHook(postWriteHook)
	st.SetPageWriteHook(pageWriteHook)
	st.SetMomentWriteHook(momentWriteHook)

	// 站点配置：优先读 config/site.yml，缺失/解析失败时 fallback 到 DefaultSite()
	siteConfigPath := filepath.Join(app.Root, "backend", "config", "site.yml")
	site, err := config.LoadSite(siteConfigPath)
	if err != nil {
		log.Printf("load site config failed (%v), fallback to DefaultSite", err)
		site = config.DefaultSite()
	} else {
		log.Printf("loaded site config: %s", siteConfigPath)
	}
	// Spec 61 阶段 3：站点热更新改为 atomic 复制-交换发布（RebuildSite 构建全新 site 后 Store），
	// 全部持有者（webServer/api/page/moment/feed/comment/friends）统一经此指针 .Load() 读取，消除数据竞争。
	sitePtr := &atomic.Pointer[tmpl.Site]{}
	sitePtr.Store(site)
	posts.SetLinksFunc(func() ([]tmpl.LinkGroup, string) {
		live := sitePtr.Load()
		return live.Links, live.Default.Link
	}) // 供 {% links %} shortcode 服务端直出分组友链卡片（site 经原子指针实时读取）

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
	if err := config.RebuildSite(st, sitePtr); err != nil {
		log.Fatalf("rebuild site from db: %v", err)
	}
	log.Printf("site config synced from db (settings/links)")

	// P0-2：生产模式 site.url 校验——release 下空 URL 会导致 canonical/OG/sitemap 回退 http，显著告警提醒配置。
	if !devMode && site.URL == "" {
		log.Printf("⚠️  WARNING: site.url is empty in release mode; canonical/OG/sitemap/feed URLs will fallback to http://<request-host>. Set url: https://your-domain in config/site.yml for production.")
	}

	// 评论 handler：公开 API 注册 + 评论回复通知复用（Enabled=false 时不注册路由）
	commentHandler := comment.New(st, sitePtr)
	commentHandler.SetUploadDir(app.UploadDir) // 评论图片上传（工具栏仅保留表情+上传）

	// 站内通知中心（Wave 3b）：评论事件写站内通知 + 可选 Webhook（settings 表 notify 段）。
	// 需在评论 handler 装配前注入，否则评论提交/回复不产生站内通知。
	notifySvc := notify.New(st)
	commentHandler.SetNotifier(notifySvc)

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
		// seed 新导入的 topic 文本可能未在启动迁移时建行（当时 posts 表还是旧数据），
		// 此处兜底再跑一次回填（幂等：仅处理 topic 非空且 topic_id=0 的文章）。
		if err := st.MigrateTopicBackfill(); err != nil {
			log.Fatalf("migrate topic backfill after seed: %v", err)
		}
		log.Printf("seed: imported %d posts from content/posts", n)
	}

	// 前台装配载体：渲染辅助 + 各域 register* 方法（见 web.go / routes_*.go）。
	ws := newWebServer(webDeps{
		Site:      sitePtr,
		Store:     st,
		Posts:     posts,
		Engine:    engine,
		SearchIdx: searchIdx,
		OGGen:     ogGen,
		DevMode:   devMode,
		Root:      app.Root,
		UploadDir: app.UploadDir,
		NotesPath: filepath.Join(app.Root, "content", "notes.md"),
	})
	// Spec 61 阶段 5：写路径置脏内容派生缓存（ws.contentCache 已就绪）
	st.SetPostWriteHook(func(op, slug string) { postWriteHook(op, slug); ws.contentCache.invalidate() })
	st.SetPageWriteHook(func(op, slug string) { pageWriteHook(op, slug); ws.contentCache.invalidate() })
	st.SetMomentWriteHook(func(op, slug string) { momentWriteHook(op, slug); ws.contentCache.invalidate() })

	// release 模式：关掉 Gin 调试日志（生产噪音），-dev 保留 debug。
	if !devMode {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	pvBuffer := newPageViewBuffer(st)
	ws.installMiddleware(r, app, pvBuffer)

	// 静态资源域：/assets、PWA 根作用域文件、/uploads。
	if err := ws.registerStaticRoutes(r); err != nil {
		log.Fatalf("register static routes: %v", err)
	}

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

	// 整站恢复后置链路（Spec 41）：设置/文章已随库恢复，需同步内存态与衍生缓存——
	// 站点配置热更新、FTS 全量重建、侧边栏快照失效、OG 缓存清空。
	backupSvc.SetRestoreHook(func() {
		if err := config.RebuildSite(st, sitePtr); err != nil {
			log.Printf("restore: rebuild site config: %v", err)
		}
		rebuildSearchIndex(st, searchIdx)
		ws.invalidateSidebarCache()
		ws.contentCache.invalidate() // Spec 61 阶段 5：恢复后内容派生缓存全部置脏
		clearOGCache(ogCacheDir)
	})

	// 功能模块框架：DB 启停状态表 + 注册表 + 管理器。各模块经 blank import 在 init() 自注册。
	if err := st.EnsureModuleStatesTable(); err != nil {
		log.Fatalf("ensure module_states table: %v", err)
	}
	moduleMgr, err := module.NewManager(st)
	if err != nil {
		log.Fatalf("init module manager: %v", err)
	}
	moduleDeps := module.Deps{Store: st, Posts: posts, Site: sitePtr, Modules: moduleMgr}
	for _, m := range module.All() {
		if err := m.Migrate(moduleDeps); err != nil {
			log.Fatalf("module %s migrate: %v", m.Meta().ID, err)
		}
		m.RegisterPublic(r, moduleDeps)
	}
	log.Printf("modules: %d registered", len(module.All()))

	// 友链动态聚合：后台定时抓取配置友链（site.yml links 带 rss）的 RSS/Atom 最新文章。
	friendsAgg := friends.New(sitePtr)
	// 订阅阅读器：友链 RSS 抓取后落库（feed_articles 表），保留已读/收藏/稍后状态。
	if err := st.EnsureFeedArticlesTable(); err != nil {
		log.Fatalf("ensure feed_articles table: %v", err)
	}
	friendsAgg.SetOnArticles(func(friendTitle, friendURL string, articles []friends.Post) {
		for _, p := range articles {
			if p.Title == "error" || p.Title == "empty" || p.Link == "" {
				continue
			}
			if err := st.UpsertFeedArticle(&store.FeedArticle{
				FriendTitle: friendTitle,
				FriendURL:   friendURL,
				Title:       p.Title,
				Link:        p.Link,
				Summary:     p.Summary,
				Published:   p.Published,
				GUID:        p.Link,
			}); err != nil {
				log.Printf("upsert feed article %s from %s: %v", p.Link, friendTitle, err)
			}
		}
	})
	friendsAgg.Start()
	linkWatch := linkwatch.New(st, sitePtr, app.UploadDir)
	linkWatch.Start()

	// API 侧装配（/api/v1 + 友链聚合/GitHub 代理/评分端点 + 相关表），见 internal/api.Mount。
	if _, err := api.Mount(r, api.MountDeps{
		Config: api.Config{
			Store:             st,
			Posts:             posts,
			Site:              sitePtr,
			Comments:          commentHandler,
			Backups:           backupSvc,
			Modules:           moduleMgr,
			Notify:            notifySvc,
			Icons:             reg,
			RenderSitePreview: ws.renderSitePreview,
			OnContentChanged:  ws.contentCache.invalidate,
			UploadDir:         app.UploadDir,
			MaxUploadMB:       app.MaxUploadMB,
			LinkWatch:         linkWatch,
		},
		Friends:   friendsAgg,
		LinkWatch: linkWatch,
		GH:        ghClient,
	}); err != nil {
		log.Fatalf("mount api: %v", err)
	}

	// Vue Console SPA 静态托管（/console/），SPA fallback 由下方 NoRoute 承接。
	consoleIndex, err := setupConsoleRoutes(r, devMode, app.Root)
	if err != nil {
		log.Fatalf("setup console routes: %v", err)
	}

	// 管理后台已统一到 Vue Console（/console/ + /api/v1）；服务端渲染后台 /admin 与 /admin/media 已退役。

	// 独立页面前台 /pages/:slug（后台管理已迁移到 Vue Console + /api/v1）
	pageHandler, err := page.New(st, posts, engine, sitePtr)
	if err != nil {
		log.Fatalf("init page: %v", err)
	}
	pageHandler.SetRender404(ws.render404)
	pageHandler.SetFillExtra(func(data *tmpl.ViewData, c *gin.Context) {
		ws.fillSidebar(data)
		ws.fillSEO(data, c)
	})
	pageHandler.Register(r)
	// 友链页内容页化：确保 pages 表存在 slug=friends 系统页（/friends 路由按内容页渲染）。
	if err := pageHandler.EnsureSystemFriendsPage(); err != nil {
		log.Printf("[warn] seed friends page: %v", err)
	}
	if err := pageHandler.EnsureSystemNotesPage(ws.notesMDPath); err != nil {
		log.Printf("[warn] seed notes page: %v", err)
	}
	if err := pageHandler.EnsureSystemLinksPage(); err != nil {
		log.Printf("[warn] seed links page: %v", err)
	}

	momentHandler, err := moment.New(st, posts, engine, sitePtr, ws.fillSidebar, ws.fillSEO)
	if err != nil {
		log.Fatalf("init moments: %v", err)
	}
	momentHandler.Register(r)
	ws.momentH = momentHandler // /notes 页近期动态依赖，注册 taxonomy 路由前回填

	// 自建评论系统公开 API（/api/comments），comments.enabled 时注册
	if sitePtr.Load().Comments.Enabled {
		commentHandler.Register(r)
	}

	// 前台各域路由（见 routes_*.go）。
	ws.registerPostRoutes(r)
	ws.registerHomeRoutes(r)
	ws.registerTaxonomyRoutes(r)
	ws.registerSearchRoutes(r)
	ws.registerFeedRoutes(r)

	// 未匹配路由：/console/* 走 SPA fallback → 单段 slug 查 pages 表（支持根路径 /about）→ 兜底 404
	r.NoRoute(func(c *gin.Context) {
		if consoleIndex != nil && strings.HasPrefix(c.Request.URL.Path, "/console") {
			c.Data(http.StatusOK, "text/html; charset=utf-8", consoleIndex)
			return
		}
		if pageHandler.ShowByPath(c) {
			return
		}
		ws.render404(c)
	})

	// 发布前预览：/preview/draft/:token。从 api 内存 TTL 快照取当前表单，用前台模板渲染（含 Stellar 主题/侧栏）。
	// 支持未保存新文章；token 不可猜，服务重启后失效。
	r.GET("/preview/draft/:token", func(c *gin.Context) {
		pv, ok := api.GetPostPreview(c.Param("token"))
		if !ok {
			ws.render404(c)
			return
		}
		contentHTML, err := posts.Render(pv.ContentMD)
		if err != nil {
			log.Printf("render draft preview: %v", err)
			ws.render500(c)
			return
		}
		title := pv.Title
		if strings.TrimSpace(title) == "" {
			title = "预览"
		}
		notice := template.HTML(`<div style="background:var(--card);border:1px solid var(--theme);color:var(--text-p1);padding:12px 16px;border-radius:8px;margin:0 0 20px;text-align:center;font-size:14px;">预览模式：这是未发布的草稿，仅供发布前查看</div>`)
		data := &tmpl.ViewData{
			Site: sitePtr.Load(),
			Page: &tmpl.Page{
				Title:   title,
				Slug:    pv.Slug,
				Layout:  "post",
				Type:    "tech",
				Lang:    "zh-CN",
				MenuID:  "post",
				Date:    pv.CreatedAt.Format("2006-01-02"),
				Excerpt: pv.Excerpt,
				Topic:   pv.Topic,
			},
			Content: notice + template.HTML(contentHTML),
		}
		data.TOC = tmpl.ExtractTOC(contentHTML)
		ws.fillSidebar(data)
		ws.fillSEO(data, c)
		html, err := engine.Render("layout/base", data)
		if err != nil {
			log.Printf("render draft preview: %v", err)
			ws.render500(c)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
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
	// Spec 61 阶段 5：PV 缓冲最终落库（st.Close 由 main 顶部 defer 执行，需在此之前）
	pvBuffer.close()
	log.Println("server stopped")
}

// newAssetVersioner 资源版本号解析器：内容 sha256 前 8 位 hex。
// release 走 embed（不可变，永久缓存）；dev 走磁盘（每次重算，改资源即时换版本号）。
// path 为模板中的资源路径（如 /assets/css/main.css）。
func newAssetVersioner(devMode bool, root string) func(string) string {
	rel := func(p string) string { return strings.TrimPrefix(strings.TrimPrefix(p, "/assets/"), "assets/") }
	var mu sync.Mutex
	cache := map[string]string{}
	return func(p string) string {
		if !devMode {
			mu.Lock()
			if v, ok := cache[p]; ok {
				mu.Unlock()
				return v
			}
			mu.Unlock()
		}
		var data []byte
		var err error
		if devMode {
			data, err = os.ReadFile(filepath.Join(root, "frontend", "assets", rel(p)))
		} else {
			data, err = assets.FS.ReadFile(rel(p))
		}
		if err != nil {
			return "0"
		}
		sum := sha256.Sum256(data)
		v := hex.EncodeToString(sum[:])[:8]
		if !devMode {
			mu.Lock()
			cache[p] = v
			mu.Unlock()
		}
		return v
	}
}
