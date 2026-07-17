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

	addr := ":8080"
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
		Site: site,
		Page: &tmpl.Page{
			Title:  site.Title,
			Layout: "index",
			Type:   "tech",
			Lang:   "zh-CN",
			MenuID: "post",
		},
		Posts:     cards,
		Paginator: &tmpl.Paginator{Current: page, Total: totalPages, Pages: pages},
	}

	html, err := engine.Render("base", data)
	if err != nil {
		c.String(http.StatusInternalServerError, "render: %v", err)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}
