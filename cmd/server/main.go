package main

import (
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	textTemplate "text/template" // XML 渲染用（atom.xml / sitemap.xml，别名避免与 html/template 冲突）
	"time"

	"github.com/gin-gonic/gin"

	"blog/assets"
	"blog/internal/admin"
	"blog/internal/config"
	"blog/internal/post"
	"blog/internal/store"
	tmpl "blog/internal/template"
)

// 资源加载策略：
//   - 默认（部署）：icons/templates/assets 全部走 go:embed，单二进制无外部依赖。
//   - -dev 模式（本地调试）：从磁盘读 resource/icons.yml、internal/template、assets/，
//     便于改 CSS/模板后 go run 立即生效（无需重编译 embed）。
//   - root 用于定位 site.yml 与 DB；embed 模式下 root 仅作 config/data 目录基准。
//
// 不能依赖 os.Executable()：go run 编译到临时目录，二进制路径不在 dev/。
func main() {
	var (
		root       string
		seed       bool
		devMode    bool
		iconsPath  string
		configPath string
	)
	flag.StringVar(&root, "root", "", "工作根目录（定位 config/site.yml 与 DB；空则用 cwd 或 app.yml）")
	flag.BoolVar(&seed, "seed", false, "启动时把 content/posts/*.md 导入 SQLite（按 slug UPSERT）")
	flag.BoolVar(&devMode, "dev", false, "开发模式：从磁盘读 icons/templates/assets（默认走 go:embed）")
	flag.StringVar(&iconsPath, "icons", "", "icons.yml 路径（仅 -dev 模式生效，默认 ../resource/.../icons.yml）")
	flag.StringVar(&configPath, "config", "", "app.yml 路径（默认 <root>/config/app.yml）")
	flag.Parse()

	// 应用配置：先粗读 app.yml 拿 root（环境变量 BLOG_ROOT 可覆盖），再 ResolveApp 绝对化。
	// root 解析优先级：-root flag > BLOG_ROOT > app.yml.root > cwd。
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
	// -root flag 优先于 app.yml 的 root（命令行显式指定胜出）
	if root != "" {
		app.Root = root
	}
	app = config.ResolveApp(app)
	if devMode && iconsPath == "" {
		// 开发模式默认从 resource/ 读 icons.yml（dev/ 出发是 ../resource/...）
		iconsPath = filepath.Join(app.Root, "..", "resource", "hexo-theme-stellar", "_data", "icons.yml")
	}

	// icons 加载：-dev 模式从磁盘读，否则走 embed。
	var reg *tmpl.IconRegistry
	if devMode {
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

	// 模板引擎：-dev 模式从磁盘读（改模板立即生效），否则走 embed。
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

	// 文章服务：content/posts/*.md（文件回退用 + seed 源）。embed 模式下 content 仍需磁盘，
	// 故按 root 定位；部署时 content/ 可选（有 DB 即可），不存在不报错。
	posts := post.NewService(filepath.Join(app.Root, "content", "posts"))

	// SQLite 持久化：DBPath 已在 ResolveApp 绝对化。
	st, err := store.New(app.DBPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()
	log.Printf("sqlite ready: %s", app.DBPath)

	// 站点配置：优先读 config/site.yml，缺失或解析失败回退 DefaultSite。
	siteConfigPath := filepath.Join(app.Root, "config", "site.yml")
	site, err := config.LoadSite(siteConfigPath)
	if err != nil {
		log.Printf("load site config failed (%v), fallback to DefaultSite", err)
		site = config.DefaultSite()
	} else {
		log.Printf("site config loaded from %s", siteConfigPath)
	}

	// admin handler（含 seed 能力）
	adminHandler := admin.New(st, posts, engine, site)

	// seed：-seed flag 或 app.yml seed_on_start 任一为真即执行。
	if seed || app.SeedOnStart {
		n, _ := adminHandler.SeedCount()
		if err := adminHandler.Seed(); err != nil {
			log.Fatalf("seed: %v", err)
		}
		log.Printf("seed: imported %d posts from content/posts", n)
	}

	// release 模式：关掉 Gin 调试日志（生产噪音），开发期可用 -dev 保留 debug。
	if !devMode {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()

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

	// 静态文件 /assets：-dev 模式从磁盘读（改 CSS 立即生效），否则走 embed.FS。
	if devMode {
		r.Static("/assets", filepath.Join(app.Root, "assets"))
	} else {
		sub, err := fs.Sub(assets.FS, ".")
		if err != nil {
			log.Fatalf("assets sub fs: %v", err)
		}
		r.StaticFS("/assets", http.FS(sub))
	}

	// 管理后台路由组 /admin
	adminHandler.Register(r)

	// 文章路由 /post/:slug：DB 优先（published），未命中回退文件渲染（向后兼容 Demo 2）
	r.GET("/post/:slug", func(c *gin.Context) {
		slug := c.Param("slug")

		// 优先查 DB
		if p, err := st.GetPublishedBySlug(slug); err == nil && p != nil {
			// 阅读量自增：每次访问 /post/:slug 计数 +1。失败仅记日志，不中断渲染。
			// p.Views 是 incr 前的值，展示用 p.Views+1 含本次访问（与 DB 最终值一致）。
			if err := st.IncrViews(slug); err != nil {
				log.Printf("incr views for %s: %v", slug, err)
			}
			// 填标签/分类（关联表查询），用于面包屑分类段 + footer 标签/分类段
			if err := st.FillTagsCategories([]*store.Post{p}); err != nil {
				log.Printf("fill tags/categories for %s: %v", slug, err)
			}
			// 查上下篇：prev=较新，next=较旧（对齐 Stellar read_next.ejs 语义）
			var readNext *tmpl.ReadNext
			prev, next, err := st.GetPrevNext(p)
			if err != nil {
				log.Printf("get prev/next for %s: %v", slug, err)
			} else if prev != nil || next != nil {
				readNext = &tmpl.ReadNext{}
				if prev != nil {
					readNext.Prev = toCard(prev)
				}
				if next != nil {
					readNext.Next = toCard(next)
				}
			}

			toc, htmlWithIDs := post.ExtractTOC(p.ContentHTML)
			data := &tmpl.ViewData{
				Site: site,
				Page: &tmpl.Page{
					Title:       p.Title,
					Slug:        p.Slug,
					Layout:      "post",
					Type:        "tech",
					Lang:        "zh-CN",
					MenuID:      "post",
					Date:        p.CreatedAt.Format("2006-01-02"),
					Updated:     p.UpdatedAt.Format("2006-01-02"),
					Excerpt:     p.Excerpt,
					Cover:       p.Cover,
					Tags:        p.Tags,
					Categories:  p.Categories,
					CreatedTime: p.CreatedAt,
					UpdatedTime: p.UpdatedAt,
					Views:       p.Views + 1,
				},
				Content:  template.HTML(htmlWithIDs),
				TOCHTML:  template.HTML(post.BuildTOCHTML(toc)),
				ReadNext: readNext,
			}
			html, err := engine.Render("base", data)
			if err != nil {
				c.String(http.StatusInternalServerError, "render: %v", err)
				return
			}
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
			return
		}

		// 回退：文件渲染（Demo 2 链路）
		p, err := posts.Load(slug)
		if err != nil {
			// 用 errors.Is 而非 os.IsNotExist：Load 的错误经 fmt.Errorf %w 包裹，
			// os.IsNotExist 不递归解包会漏判（实测返回 false），errors.Is 能穿透。
			if errors.Is(err, fs.ErrNotExist) {
				c.String(http.StatusNotFound, "post not found: %s", slug)
				return
			}
			c.String(http.StatusInternalServerError, "load post: %v", err)
			return
		}

		// 文件回退路径无 store 数据：Cover/Tags/Categories/ReadNext 留零值（不渲染对应区块）。
		// CreatedTime/UpdatedTime 从 frontmatter date 字符串解析，保证 <time datetime> 正确；
		// 解析失败则用零值，模板 Page.Updated 为空时已 fallback 到 Page.Date 显示。
		var createdTime time.Time
		if t, perr := time.Parse("2006-01-02", p.Date); perr == nil {
			createdTime = t
		}

		toc, htmlWithIDs := post.ExtractTOC(p.HTML)
		data := &tmpl.ViewData{
			Site: site,
			Page: &tmpl.Page{
				Title:       p.Title,
				Slug:        p.Slug,
				Layout:      "post",
				Type:        "tech",
				Lang:        "zh-CN",
				MenuID:      "post",
				Date:        p.Date,
				Excerpt:     p.Excerpt,
				CreatedTime: createdTime,
				UpdatedTime: createdTime,
			},
			Content: template.HTML(htmlWithIDs),
			TOCHTML: template.HTML(post.BuildTOCHTML(toc)),
		}

		html, err := engine.Render("base", data)
		if err != nil {
			c.String(http.StatusInternalServerError, "render: %v", err)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	})

	// 首页：渲染文章列表（navbar + post-list + paginator），对齐 Stellar layout/index.ejs。
	r.GET("/", func(c *gin.Context) {
		renderHome(c, st, engine, site, 1)
	})

	// 分页：/page/N 走同一首页渲染逻辑。
	r.GET("/page/:num", func(c *gin.Context) {
		numStr := c.Param("num")
		page, err := strconv.Atoi(numStr)
		if err != nil || page < 1 {
			c.String(http.StatusNotFound, "page not found: %s", numStr)
			return
		}
		renderHome(c, st, engine, site, page)
	})

	// 标签云：/tags
	r.GET("/tags", func(c *gin.Context) {
		renderTagsCloud(c, st, engine, site)
	})

	// 单标签文章列表：/tags/:name
	r.GET("/tags/:name", func(c *gin.Context) {
		name := c.Param("name")
		page, _ := strconv.Atoi(c.Query("page"))
		renderListByRelation(c, st, engine, site, name, true, page)
	})

	// 分类云：/categories
	r.GET("/categories", func(c *gin.Context) {
		renderCategoriesCloud(c, st, engine, site)
	})

	// 单分类文章列表：/categories/:name
	r.GET("/categories/:name", func(c *gin.Context) {
		name := c.Param("name")
		page, _ := strconv.Atoi(c.Query("page"))
		renderListByRelation(c, st, engine, site, name, false, page)
	})

	// 归档：/archives 按年分组
	r.GET("/archives", func(c *gin.Context) {
		renderArchive(c, st, engine, site)
	})

	// 站内搜索：/search?q=xxx&page=N
	r.GET("/search", func(c *gin.Context) {
		q := c.Query("q")
		page, _ := strconv.Atoi(c.Query("page"))
		renderSearch(c, st, engine, site, q, page)
	})

	// RSS feed：/atom.xml（/feed.xml 别名）
	r.GET("/atom.xml", func(c *gin.Context) { renderAtom(c, st, site) })
	r.GET("/feed.xml", func(c *gin.Context) { renderAtom(c, st, site) })

	// sitemap：/sitemap.xml
	r.GET("/sitemap.xml", func(c *gin.Context) { renderSitemap(c, st, site) })

	// 404 页面：所有未匹配路由走 Stellar 风格错误页（对齐 layout/404.ejs）。
	// 渲染 base 布局 + error_page partial，Page.Layout="404" 触发 base.html 分派。
	r.NoRoute(func(c *gin.Context) {
		data := &tmpl.ViewData{
			Site: site,
			Page: &tmpl.Page{
				Title:  "404",
				Slug:   "404",
				Layout: "404",
				Type:   "tech",
				Lang:   "zh-CN",
				MenuID: "post",
			},
			NavTabs: &tmpl.NavTabs{ShowTags: true, ShowCategories: true, ShowArchive: true, Active: "home"},
		}
		html, err := engine.Render("base", data)
		if err != nil {
			c.String(http.StatusInternalServerError, "render 404: %v", err)
			return
		}
		c.Data(http.StatusNotFound, "text/html; charset=utf-8", []byte(html))
	})

	addr := app.ListenAddr
	fmt.Printf("Stellar blog server listening on http://localhost%s\n", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("run: %v", err)
	}
}

// pageSize 首页/列表页每页文章数，对齐 Hexo index_generator per_page 默认值。
const pageSize = 10

// toCards 把 store.Post 列表转成 PostCard 视图模型。
// 空 excerpt 自动从 ContentHTML 提取（复用 post.ExcerptFromHTML）。
func toCards(posts []*store.Post) []*tmpl.PostCard {
	cards := make([]*tmpl.PostCard, 0, len(posts))
	for _, p := range posts {
		cards = append(cards, toCard(p))
	}
	return cards
}

// toCard 把单篇 store.Post 转成 PostCard 视图模型。
// read_next 上下篇导航复用此函数（只需 Slug/Title，其余字段保留默认零值）。
func toCard(p *store.Post) *tmpl.PostCard {
	excerpt := p.Excerpt
	if excerpt == "" {
		excerpt = post.ExcerptFromHTML(p.ContentHTML, 120)
	}
	return &tmpl.PostCard{
		Slug:    p.Slug,
		Title:   p.Title,
		Excerpt: excerpt,
		Cover:   p.Cover,
		Date:    p.CreatedAt,
		Views:   p.Views,
	}
}

// buildPaginator 构造分页器，Go 侧预生成每页 URL，避免模板拼接 query 被转义破坏。
//   - baseURL 空：首页 /page/N 风格（第1页是 /）
//   - baseURL 非空 + baseQuery 空：列表页 baseURL?page=N（第1页是 baseURL）
//   - baseURL 非空 + baseQuery 非空：搜索页 baseURL?baseQuery&page=N（第1页是 baseURL?baseQuery）
func buildPaginator(page, total int, baseURL, baseQuery string) *tmpl.Paginator {
	pages := make([]int, total)
	pageURLs := make([]string, total)
	for i := range pages {
		n := i + 1
		pages[i] = n
		pageURLs[i] = pageURL(baseURL, baseQuery, n)
	}
	var prev, next string
	if page > 1 {
		prev = pageURL(baseURL, baseQuery, page-1)
	}
	if page < total {
		next = pageURL(baseURL, baseQuery, page+1)
	}
	return &tmpl.Paginator{Current: page, Total: total, Pages: pages, PageURLs: pageURLs, PrevURL: prev, NextURL: next}
}

// pageURL 生成第 n 页的完整 URL，分三种风格（见 buildPaginator 注释）。
func pageURL(baseURL, baseQuery string, n int) string {
	if baseURL == "" {
		// 首页：第1页是 /，其余 /page/N
		if n == 1 {
			return "/"
		}
		return fmt.Sprintf("/page/%d", n)
	}
	// 列表/搜索页
	var b strings.Builder
	b.WriteString(baseURL)
	if n == 1 {
		if baseQuery != "" {
			b.WriteString("?")
			b.WriteString(baseQuery)
		}
		return b.String()
	}
	b.WriteString("?")
	if baseQuery != "" {
		b.WriteString(baseQuery)
		b.WriteString("&")
	}
	fmt.Fprintf(&b, "page=%d", n)
	return b.String()
}

// renderHome 渲染首页文章列表（Layout=="index"）。
// page 从 1 开始；越界（page > totalPages）回 404。文章数据走 store DB 查询。
func renderHome(c *gin.Context, st *store.Store, engine *tmpl.Engine, site *tmpl.Site, page int) {
	total, err := st.CountPublished()
	if err != nil {
		c.String(http.StatusInternalServerError, "count posts: %v", err)
		return
	}
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1 // 无文章时仍渲染空列表页（分页器 Total=1 不显示）
	}
	if page > totalPages {
		c.String(http.StatusNotFound, "page %d not found (total %d)", page, totalPages)
		return
	}

	posts, err := st.ListPublishedPaged(page, pageSize)
	if err != nil {
		c.String(http.StatusInternalServerError, "list posts: %v", err)
		return
	}

	data := &tmpl.ViewData{
		Site: site,
		Page: &tmpl.Page{
			Title:  site.Title,
			Layout: "index",
			Type:   "tech",
			Lang:   "zh-CN",
			MenuID: "post",
		},
		Posts:     toCards(posts),
		Paginator: buildPaginator(page, totalPages, "", ""),
		NavTabs:   &tmpl.NavTabs{ShowTags: true, ShowCategories: true, ShowArchive: true, Active: "home"},
	}
	renderView(c, engine, data)
}

// renderTagsCloud 渲染标签云（Layout=="tags"）。
func renderTagsCloud(c *gin.Context, st *store.Store, engine *tmpl.Engine, site *tmpl.Site) {
	tags, err := st.ListTags()
	if err != nil {
		c.String(http.StatusInternalServerError, "list tags: %v", err)
		return
	}
	items := make([]tmpl.TagItem, 0, len(tags))
	for _, t := range tags {
		items = append(items, tmpl.TagItem{Name: t.Name, Count: t.Count})
	}
	data := &tmpl.ViewData{
		Site:    site,
		Page:    &tmpl.Page{Title: "标签", Layout: "tags", Type: "tech", Lang: "zh-CN", MenuID: "post"},
		Tags:    items,
		NavTabs: &tmpl.NavTabs{ShowTags: true, ShowCategories: true, ShowArchive: true, Active: "tags"},
	}
	renderView(c, engine, data)
}

// renderCategoriesCloud 渲染分类云（Layout=="categories"）。
func renderCategoriesCloud(c *gin.Context, st *store.Store, engine *tmpl.Engine, site *tmpl.Site) {
	cats, err := st.ListCategories()
	if err != nil {
		c.String(http.StatusInternalServerError, "list categories: %v", err)
		return
	}
	items := make([]tmpl.CategoryItem, 0, len(cats))
	for _, ct := range cats {
		items = append(items, tmpl.CategoryItem{Name: ct.Name, Count: ct.Count})
	}
	data := &tmpl.ViewData{
		Site:       site,
		Page:       &tmpl.Page{Title: "分类", Layout: "categories", Type: "tech", Lang: "zh-CN", MenuID: "post"},
		Categories: items,
		NavTabs:    &tmpl.NavTabs{ShowTags: true, ShowCategories: true, ShowArchive: true, Active: "categories"},
	}
	renderView(c, engine, data)
}

// renderListByRelation 渲染单标签/单分类的文章列表（Layout=="list"）。
// isTag=true 走标签查询，false 走分类查询。name 是标签/分类名。
func renderListByRelation(c *gin.Context, st *store.Store, engine *tmpl.Engine, site *tmpl.Site, name string, isTag bool, page int) {
	if page < 1 {
		page = 1
	}
	var (
		posts     []*store.Post
		total     int
		err       error
		baseURL   string
		labelType string
	)
	if isTag {
		total, err = st.CountByTag(name)
		baseURL = "/tags/" + name
		labelType = "标签"
	} else {
		total, err = st.CountByCategory(name)
		baseURL = "/categories/" + name
		labelType = "分类"
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "count: %v", err)
		return
	}
	if total == 0 {
		c.String(http.StatusNotFound, "%s not found: %s", labelType, name)
		return
	}
	totalPages := (total + pageSize - 1) / pageSize
	if page > totalPages {
		c.String(http.StatusNotFound, "page %d not found (total %d)", page, totalPages)
		return
	}
	if isTag {
		posts, err = st.ListByTag(name, page, pageSize)
	} else {
		posts, err = st.ListByCategory(name, page, pageSize)
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "list: %v", err)
		return
	}

	nav := &tmpl.NavTabs{ShowTags: true, ShowCategories: true, ShowArchive: true}
	if isTag {
		nav.TagName = name
	} else {
		nav.CategoryName = name
	}
	data := &tmpl.ViewData{
		Site:      site,
		Page:      &tmpl.Page{Title: labelType + ":" + name, Layout: "list", Type: "tech", Lang: "zh-CN", MenuID: "post"},
		Posts:     toCards(posts),
		Paginator: buildPaginator(page, totalPages, baseURL, ""),
		NavTabs:   nav,
	}
	renderView(c, engine, data)
}

// renderArchive 渲染按年归档（Layout=="archive"）。
func renderArchive(c *gin.Context, st *store.Store, engine *tmpl.Engine, site *tmpl.Site) {
	groups, err := st.Archive()
	if err != nil {
		c.String(http.StatusInternalServerError, "archive: %v", err)
		return
	}
	archive := make([]tmpl.ArchiveGroup, 0, len(groups))
	for _, g := range groups {
		archive = append(archive, tmpl.ArchiveGroup{Year: g.Year, Posts: toCards(g.Posts)})
	}
	data := &tmpl.ViewData{
		Site:    site,
		Page:    &tmpl.Page{Title: "归档", Layout: "archive", Type: "tech", Lang: "zh-CN", MenuID: "post"},
		Archive: archive,
		NavTabs: &tmpl.NavTabs{ShowTags: true, ShowCategories: true, ShowArchive: true, Active: "archive"},
	}
	renderView(c, engine, data)
}

// renderSearch 渲染站内搜索结果页（Layout=="list"，复用 post_card + paginator）。
// q 为空时渲染空结果页（不报错，引导用户输入）。baseURL 带 q 参数，分页器 ?page=N 不丢查询词。
func renderSearch(c *gin.Context, st *store.Store, engine *tmpl.Engine, site *tmpl.Site, q string, page int) {
	q = strings.TrimSpace(q)
	if page < 1 {
		page = 1
	}

	posts, total := []*store.Post{}, 0
	if q != "" {
		var err error
		posts, total, err = st.Search(q, page, pageSize)
		if err != nil {
			c.String(http.StatusInternalServerError, "search: %v", err)
			return
		}
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1 // 无结果仍渲染空列表页
	}
	if page > totalPages {
		c.String(http.StatusNotFound, "page %d not found (total %d)", page, totalPages)
		return
	}

	title := "搜索"
	if q != "" {
		title = "搜索: " + q
	}
	data := &tmpl.ViewData{
		Site: site,
		Page: &tmpl.Page{
			Title:  title,
			Layout: "list",
			Type:   "tech",
			Lang:   "zh-CN",
			MenuID: "post",
		},
		Posts:       toCards(posts),
		Paginator:   buildPaginator(page, totalPages, "/search", "q="+url.QueryEscape(q)),
		NavTabs:     &tmpl.NavTabs{ShowTags: true, ShowCategories: true, ShowArchive: true, Active: "search"},
		SearchQuery: q,
	}
	renderView(c, engine, data)
}

// renderView 渲染 base 模板并写入响应。
func renderView(c *gin.Context, engine *tmpl.Engine, data *tmpl.ViewData) {
	html, err := engine.Render("base", data)
	if err != nil {
		c.String(http.StatusInternalServerError, "render: %v", err)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// feedMaxEntries RSS feed 最多输出的文章数。
const feedMaxEntries = 20

// feedEntry Atom feed 的单篇文章条目。
type feedEntry struct {
	Title     string
	ID        string // 永久标识，用 tag: 格式（tag:host,year:post/id），slug 变更后仍稳定
	Link      string // 文章绝对 URL
	Summary   string // 摘要纯文本
	Content   string // 正文 HTML（CDATA 包裹，已转义内部 ]]>）
	Published string // RFC3339
	Updated   string // RFC3339
}

// renderAtom 输出 Atom 1.0 feed（/atom.xml）。取最近 feedMaxEntries 篇已发布文章。
// 用 text/template 渲染，文本字段走 xmlEscape 手动转义，正文 HTML 用 CDATA 包裹，
// 并把正文内的 ]]> 替换为 ]]]]><![CDATA[> 避免 CDATA 段提前闭合。
func renderAtom(c *gin.Context, st *store.Store, site *tmpl.Site) {
	posts, err := st.ListPublished()
	if err != nil {
		c.String(http.StatusInternalServerError, "list posts for feed: %v", err)
		return
	}

	baseURL := strings.TrimRight(siteURL(site), "/")
	host := feedHost(siteURL(site))
	entries := make([]feedEntry, 0, feedMaxEntries)
	for i, p := range posts {
		if i >= feedMaxEntries {
			break
		}
		excerpt := p.Excerpt
		if excerpt == "" {
			excerpt = post.ExcerptFromHTML(p.ContentHTML, 200)
		}
		entries = append(entries, feedEntry{
			Title:     p.Title,
			ID:        fmt.Sprintf("tag:%s,2026:post/%d", host, p.ID),
			Link:      baseURL + "/post/" + p.Slug,
			Summary:   excerpt,
			Content:   strings.ReplaceAll(p.ContentHTML, "]]>", "]]]]><![CDATA[>"),
			Published: p.CreatedAt.UTC().Format(time.RFC3339),
			Updated:   p.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}

	data := struct {
		Site    *tmpl.Site
		BaseURL string
		FeedID  string // feed 级永久 ID，tag:host,year 格式
		Updated string // feed 级更新时间，取首篇 updated 或当前时间
		Entries []feedEntry
	}{
		Site:    site,
		BaseURL: baseURL,
		FeedID:  fmt.Sprintf("tag:%s,2026", host),
		Updated: feedUpdated(entries),
		Entries: entries,
	}

	var sb strings.Builder
	if err := atomTpl.Execute(&sb, data); err != nil {
		c.String(http.StatusInternalServerError, "render atom: %v", err)
		return
	}
	c.Data(http.StatusOK, "application/atom+xml; charset=utf-8", []byte(sb.String()))
}

// atomTpl Atom 1.0 模板。text/template 不自动转义，所有文本节点（含 URL/IRI）显式调 xmlEscape，
// 防止 site.URL 带 query 或 slug 含 & 时生成非法 XML；正文 HTML 用 CDATA 包裹（内部 ]]> 已在 renderAtom 转义）。
var atomTpl = textTemplate.Must(textTemplate.New("atom").Funcs(textTemplate.FuncMap{
	"xmlEscape": xmlEscape,
}).Parse(`<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>{{xmlEscape .Site.Title}}</title>
  <subtitle>{{xmlEscape .Site.Subtitle}}</subtitle>
  <link href="{{xmlEscape .BaseURL}}/atom.xml" rel="self"/>
  <link href="{{xmlEscape .BaseURL}}/"/>
  <updated>{{.Updated}}</updated>
  <id>{{xmlEscape .FeedID}}/</id>
  <author>
    <name>{{xmlEscape .Site.Author}}</name>
  </author>
  {{range .Entries}}
  <entry>
    <title>{{xmlEscape .Title}}</title>
    <link href="{{xmlEscape .Link}}"/>
    <id>{{xmlEscape .ID}}</id>
    <published>{{.Published}}</published>
    <updated>{{.Updated}}</updated>
    <summary>{{xmlEscape .Summary}}</summary>
    <content type="html"><![CDATA[{{.Content}}]]></content>
  </entry>
  {{end}}
</feed>
`))

// renderSitemap 输出 sitemap.xml。含首页/标签/分类/归档 + 全部已发布文章（lastmod=updated_at）。
func renderSitemap(c *gin.Context, st *store.Store, site *tmpl.Site) {
	posts, err := st.ListPublished()
	if err != nil {
		c.String(http.StatusInternalServerError, "list posts for sitemap: %v", err)
		return
	}
	baseURL := strings.TrimRight(siteURL(site), "/")

	type sitemapURL struct {
		Loc     string
		Lastmod string // 留空则不输出 <lastmod>
	}
	urls := []sitemapURL{
		{Loc: baseURL + "/"},
		{Loc: baseURL + "/tags"},
		{Loc: baseURL + "/categories"},
		{Loc: baseURL + "/archives"},
	}
	for _, p := range posts {
		lastmod := p.UpdatedAt.UTC().Format("2006-01-02")
		urls = append(urls, sitemapURL{Loc: baseURL + "/post/" + p.Slug, Lastmod: lastmod})
	}

	var sb strings.Builder
	if err := sitemapTpl.Execute(&sb, struct {
		URLs []sitemapURL
	}{urls}); err != nil {
		c.String(http.StatusInternalServerError, "render sitemap: %v", err)
		return
	}
	c.Data(http.StatusOK, "application/xml; charset=utf-8", []byte(sb.String()))
}

var sitemapTpl = textTemplate.Must(textTemplate.New("sitemap").Funcs(textTemplate.FuncMap{
	"xmlEscape": xmlEscape,
}).Parse(`<?xml version="1.0" encoding="utf-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
{{range .URLs}}  <url>
    <loc>{{xmlEscape .Loc}}</loc>{{if .Lastmod}}
    <lastmod>{{.Lastmod}}</lastmod>{{end}}
  </url>
{{end}}</urlset>
`))

// xmlEscape 转义 XML 文本节点（& < >）。text/template 不自动转义，手动处理。
// ' 和 " 在文本节点无需转义，仅转义 & < > 三种。
func xmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return r.Replace(s)
}

// siteURL 返回站点根 URL，配置缺失时回退到 localhost。
func siteURL(site *tmpl.Site) string {
	if site.URL != "" {
		return site.URL
	}
	return "http://localhost:8080"
}

// feedHost 从站点 URL 提取 host（用于生成 tag: scheme 的永久 ID）。
// 解析失败回退到 localhost，保证 feed 始终有合法 host。
func feedHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return "localhost"
	}
	return u.Host
}

// feedUpdated 取首篇 entry 的 updated 作为 feed 级更新时间，无文章用当前时间。
func feedUpdated(entries []feedEntry) string {
	if len(entries) > 0 && entries[0].Updated != "" {
		return entries[0].Updated
	}
	return time.Now().UTC().Format(time.RFC3339)
}
