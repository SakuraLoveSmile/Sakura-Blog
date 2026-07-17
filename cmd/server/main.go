package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"

	"blog/internal/admin"
	"blog/internal/config"
	"blog/internal/post"
	"blog/internal/store"
	tmpl "blog/internal/template"
)

// dev 根目录定位策略：
//   - 优先 -root 参数（绝对路径）
//   - 否则用进程 cwd（要求在 dev/ 下启动，如 go run cmd/server/main.go）
//
// 不能依赖 os.Executable()：go run 编译到临时目录，二进制路径不在 dev/。
func main() {
	var root string
	var seed bool
	flag.StringVar(&root, "root", "", "dev 工作目录绝对路径（默认用 cwd）")
	flag.BoolVar(&seed, "seed", false, "启动时把 content/posts/*.md 导入 SQLite（按 slug UPSERT）")
	flag.Parse()

	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			log.Fatalf("getwd: %v", err)
		}
	}
	root, _ = filepath.Abs(root)

	// icons.yml 在 resource/ 下（从 dev/ 出发是 ../resource/...，已 gitignore）
	iconsPath := filepath.Join(root, "..", "resource", "hexo-theme-stellar", "_data", "icons.yml")
	reg, err := tmpl.LoadIcons(iconsPath)
	if err != nil {
		log.Fatalf("load icons: %v", err)
	}
	log.Printf("loaded %d icons from %s", reg.Len(), iconsPath)

	// 模板引擎：加载 internal/template 下所有 .html（含 admin/ 子目录，WalkDir 递归）
	engine, err := tmpl.NewEngine(filepath.Join(root, "internal", "template"), reg)
	if err != nil {
		log.Fatalf("load templates: %v", err)
	}

	// 文章服务：content/posts/*.md（文件回退用）
	posts := post.NewService(filepath.Join(root, "content", "posts"))

	// SQLite 持久化：data/blog.db（走 -root 解析，不依赖 cwd）
	dbPath := filepath.Join(root, "data", "blog.db")
	st, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()
	log.Printf("sqlite ready: %s", dbPath)

	// 站点配置：优先读 config/site.yml，缺失或解析失败回退 DefaultSite。
	configPath := filepath.Join(root, "config", "site.yml")
	site, err := config.LoadSite(configPath)
	if err != nil {
		log.Printf("load site config failed (%v), fallback to DefaultSite", err)
		site = config.DefaultSite()
	} else {
		log.Printf("site config loaded from %s", configPath)
	}

	// admin handler（含 seed 能力）
	adminHandler := admin.New(st, posts, engine, site)

	// -seed：把 content/posts/*.md 导入 DB 后继续启动
	if seed {
		n, _ := adminHandler.SeedCount()
		if err := adminHandler.Seed(); err != nil {
			log.Fatalf("seed: %v", err)
		}
		log.Printf("seed: imported %d posts from content/posts", n)
	}

	r := gin.Default()

	// 静态文件：assets/css、assets/js 映射到 /assets
	r.Static("/assets", filepath.Join(root, "assets"))

	// 管理后台路由组 /admin
	adminHandler.Register(r)

	// 文章路由 /post/:slug：DB 优先（published），未命中回退文件渲染（向后兼容 Demo 2）
	r.GET("/post/:slug", func(c *gin.Context) {
		slug := c.Param("slug")

		// 优先查 DB
		if p, err := st.GetPublishedBySlug(slug); err == nil && p != nil {
			data := &tmpl.ViewData{
				Site: site,
				Page: &tmpl.Page{
					Title:   p.Title,
					Slug:    p.Slug,
					Layout:  "post",
					Type:    "tech",
					Lang:    "zh-CN",
					MenuID:  "post",
					Date:    p.CreatedAt.Format("2006-01-02"),
					Excerpt: p.Excerpt,
				},
				Content: template.HTML(p.ContentHTML),
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
			if os.IsNotExist(err) {
				c.String(http.StatusNotFound, "post not found: %s", slug)
				return
			}
			c.String(http.StatusInternalServerError, "load post: %v", err)
			return
		}

		data := &tmpl.ViewData{
			Site: site,
			Page: &tmpl.Page{
				Title:   p.Title,
				Slug:    p.Slug,
				Layout:  "post",
				Type:    "tech",
				Lang:    "zh-CN",
				MenuID:  "post",
				Date:    p.Date,
				Excerpt: p.Excerpt,
			},
			Content: template.HTML(p.HTML),
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

	// 分页：/page/N 走同一首页渲染逻辑；/page/1 301 到 /。
	r.GET("/page/:num", func(c *gin.Context) {
		numStr := c.Param("num")
		page, err := strconv.Atoi(numStr)
		if err != nil || page < 1 {
			c.String(http.StatusNotFound, "page not found: %s", numStr)
			return
		}
		if page == 1 {
			c.Redirect(http.StatusMovedPermanently, "/")
			return
		}
		renderHome(c, st, engine, site, page)
	})

	// 归档页：/archives 301 到带斜杠版本，保持 URL 规范。
	r.GET("/archives", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/archives/")
	})

	// 归档页：按年份分组列出所有已发布文章。
	r.GET("/archives/", func(c *gin.Context) {
		posts, err := st.ListPublished()
		if err != nil {
			c.String(http.StatusInternalServerError, "list posts: %v", err)
			return
		}

		groups := make([]*tmpl.ArchiveGroup, 0)
		var current *tmpl.ArchiveGroup
		for _, p := range posts {
			year := p.CreatedAt.Year()
			if current == nil || current.Year != year {
				current = &tmpl.ArchiveGroup{Year: year}
				groups = append(groups, current)
			}
			current.Items = append(current.Items, &tmpl.ArchiveItem{
				Slug:  p.Slug,
				Title: p.Title,
				Date:  p.CreatedAt,
			})
		}

		data := &tmpl.ViewData{
		Site:          site,
		ArchiveGroups: groups,
		RecentPosts:   loadRecentPosts(st, 5),
		Page: &tmpl.Page{
			Title:  "归档",
			Layout: "archive",
			Type:   "tech",
			Lang:   "zh-CN",
			MenuID: "archive",
		},
	}

		html, err := engine.Render("base", data)
		if err != nil {
			c.String(http.StatusInternalServerError, "render: %v", err)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	})

	addr := ":8082"
	fmt.Printf("Stellar Demo 3 server listening on http://localhost%s\n", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("run: %v", err)
	}
}

// pageSize 首页每页文章数，对齐 Hexo index_generator per_page 默认值。
const pageSize = 10

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

	cards := make([]*tmpl.PostCard, 0, len(posts))
	for _, p := range posts {
		cards = append(cards, &tmpl.PostCard{
			Slug:    p.Slug,
			Title:   p.Title,
			Excerpt: p.Excerpt,
			Cover:   p.Cover,
			Date:    p.CreatedAt,
		})
	}

	pages := make([]int, totalPages)
	for i := range pages {
		pages[i] = i + 1
	}

	data := &tmpl.ViewData{
		Site:        site,
		RecentPosts: loadRecentPosts(st, 5),
		Page: &tmpl.Page{
			Title:  site.Title,
			Layout: "index",
			Type:   "tech",
			Lang:   "zh-CN",
			MenuID: "post",
		},
		Posts: cards,
		Paginator: &tmpl.Paginator{
			Current:      page,
			Total:        totalPages,
			Pages:        pages,
			VisiblePages: truncatePages(page, totalPages),
		},
	}

	html, err := engine.Render("base", data)
	if err != nil {
		c.String(http.StatusInternalServerError, "render: %v", err)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// truncatePages 截断页码序列，避免总页数过多时全量渲染。
// 返回值中 0 表示省略号占位。规则：
//   - 总页数 <= 7 时显示全部
//   - 当前页靠近开头：1 2 3 4 5 ... Total
//   - 当前页靠近末尾：1 ... Total-4 Total-3 Total-2 Total-1 Total
//   - 中间：1 ... Current-1 Current Current+1 ... Total
func truncatePages(current, total int) []int {
	if total <= 7 {
		out := make([]int, total)
		for i := range out {
			out[i] = i + 1
		}
		return out
	}
	if current <= 4 {
		return []int{1, 2, 3, 4, 5, 0, total}
	}
	if current >= total-3 {
		return []int{1, 0, total - 4, total - 3, total - 2, total - 1, total}
	}
	return []int{1, 0, current - 1, current, current + 1, 0, total}
}

// loadRecentPosts 从 DB 读取最新 limit 篇已发布文章，转为 PostCard 供侧边栏使用。
// 查询失败时返回空切片，避免影响页面整体渲染。
func loadRecentPosts(st *store.Store, limit int) []*tmpl.PostCard {
	posts, err := st.ListPublishedRecent(limit)
	if err != nil {
		return nil
	}
	cards := make([]*tmpl.PostCard, 0, len(posts))
	for _, p := range posts {
		cards = append(cards, &tmpl.PostCard{
			Slug:  p.Slug,
			Title: p.Title,
		})
	}
	return cards
}
