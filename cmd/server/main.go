package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/admin"
	"blog/internal/comment"
	"blog/internal/config"
	"blog/internal/post"
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

	// 站点配置：优先读 config/site.yml，缺失/解析失败时 fallback 到 DefaultSite()
	configPath := filepath.Join(root, "config", "site.yml")
	site, err := config.LoadSite(configPath)
	if err != nil {
		log.Printf("load site config failed (%v), fallback to DefaultSite", err)
		site = config.DefaultSite()
	} else {
		log.Printf("loaded site config: %s", configPath)
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

	// 评论 handler：公开 API 注册 + admin 回复通知复用（Enabled=false 时不注册路由，仅供 admin 用）
	commentHandler := comment.New(st, &site.Comments, site.Title)

	// admin handler（含 seed 能力）
	adminHandler := admin.New(st, posts, engine, site, commentHandler)

	// -seed：把 content/posts/*.md 导入 DB 后继续启动
	if seed {
		n, _ := adminHandler.SeedCount()
		if err := adminHandler.Seed(); err != nil {
			log.Fatalf("seed: %v", err)
		}
		log.Printf("seed: imported %d posts from content/posts", n)
	}

	// 搜索索引与 widget 摘要共用的纯文本提取：去标签 + 压缩空白。
	htmlTagRe := regexp.MustCompile(`<[^>]+>`)
	whitespaceRe := regexp.MustCompile(`\s+`)
	stripHTML := func(s string) string {
		s = htmlTagRe.ReplaceAllString(s, " ")
		return strings.TrimSpace(whitespaceRe.ReplaceAllString(s, " "))
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

	// fillSidebar：填充侧边栏动态数据（近期发布 5 篇 + 标签云前 20 + 最新评论 5 条），所有前台页面渲染前调用。
	// 查询失败时留空，widget 降级为空列表，不影响主内容渲染。
	fillSidebar := func(data *tmpl.ViewData) {
		if posts, err := st.ListPublishedPaged(1, 5); err == nil {
			for _, p := range posts {
				data.Recent = append(data.Recent, &tmpl.RecentPost{Title: p.Title, URL: "/post/" + p.Slug})
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
		if counts, err := st.CountByTag(); err == nil {
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
			if len(items) > 20 {
				items = items[:20]
			}
			data.TagCloud = items
		}
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
			c.String(http.StatusInternalServerError, "render: %v", err)
			return
		}
		c.Data(http.StatusNotFound, "text/html; charset=utf-8", []byte(html))
	}

	r := gin.Default()

	// /assets 协商缓存：no-cache 让浏览器每次发条件请求，gin Static 底层
	// http.ServeContent 支持 If-Modified-Since 返回 304，既保证新鲜又省流量。
	// 必须注册在 r.Static 之前，否则静态路由先命中拿不到头。
	r.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/assets/") {
			c.Header("Cache-Control", "no-cache")
		}
		c.Next()
	})

	// 静态文件：assets/css、assets/js 映射到 /assets
	r.Static("/assets", filepath.Join(root, "assets"))

	// 管理后台路由组 /admin
	adminHandler.Register(r)

	// 自建评论系统公开 API（/api/comments），comments.enabled 时注册
	if site.Comments.Enabled {
		commentHandler.Register(r)
	}

	// 未匹配路由一律走自定义 404 页
	r.NoRoute(render404)

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
			data.TOC = tmpl.ExtractTOC(p.ContentHTML)
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
					cards = append(cards, &tmpl.PostCard{
						Slug:    rp.Slug,
						Title:   rp.Title,
						Excerpt: rp.Excerpt,
						Cover:   rp.Cover,
						Date:    rp.CreatedAt,
					})
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
				render404(c)
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
			c.String(http.StatusInternalServerError, "render: %v", err)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	})

	// 本地搜索索引：全量 published 文章的标题/正文纯文本，前端 local-search.js 拉取。
	// no-store：索引需实时反映 admin 改动，禁止浏览器/CDN 缓存。
	r.GET("/search.json", func(c *gin.Context) {
		posts, err := st.ListPublished()
		if err != nil {
			c.String(http.StatusInternalServerError, "list posts: %v", err)
			return
		}
		type searchItem struct {
			Title   string `json:"title"`
			Content string `json:"content"`
			Path    string `json:"path"`
		}
		items := make([]searchItem, 0, len(posts))
		for _, p := range posts {
			items = append(items, searchItem{
				Title:   p.Title,
				Content: stripHTML(p.ContentHTML),
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
			c.String(http.StatusInternalServerError, "count posts: %v", err)
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
			Site:      site,
			Page:      &tmpl.Page{Title: site.Title, Layout: "index", MenuID: "post", Lang: "zh-CN"},
			Posts:     cards,
			Paginator: &tmpl.Paginator{Current: page, Total: totalPages, Pages: pages},
		}
		fillSidebar(data)
		fillSEO(data, c)
		html, err := engine.Render("layout/base", data)
		if err != nil {
			c.String(http.StatusInternalServerError, "render: %v", err)
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
			c.String(http.StatusInternalServerError, "render: %v", err)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	}

	// toCards：store.Post 列表转首页卡片视图模型。
	toCards := func(posts []*store.Post) []*tmpl.PostCard {
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
		return cards
	}

	// 归档页：按年份分组的文章列表（对齐 Stellar archive.ejs）
	r.GET("/archives", func(c *gin.Context) {
		posts, err := st.ListPublished()
		if err != nil {
			c.String(http.StatusInternalServerError, "list posts: %v", err)
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
			c.String(http.StatusInternalServerError, "count categories: %v", err)
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
			c.String(http.StatusInternalServerError, "count categories: %v", err)
			return
		}
		if _, ok := counts[name]; !ok {
			render404(c)
			return
		}
		posts, err := st.ListByCategory(name)
		if err != nil {
			c.String(http.StatusInternalServerError, "list posts: %v", err)
			return
		}
		renderBase(c, &tmpl.ViewData{
			Site:  site,
			Page:  &tmpl.Page{Title: "分类：" + name, Layout: "index", MenuID: "post", Lang: "zh-CN", Category: name},
			Posts: toCards(posts),
		})
	})

	// 标签索引页：标签云（按文章数 DESC）
	r.GET("/tags", func(c *gin.Context) {
		counts, err := st.CountByTag()
		if err != nil {
			c.String(http.StatusInternalServerError, "count tags: %v", err)
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
			c.String(http.StatusInternalServerError, "count tags: %v", err)
			return
		}
		if _, ok := counts[name]; !ok {
			render404(c)
			return
		}
		posts, err := st.ListByTag(name)
		if err != nil {
			c.String(http.StatusInternalServerError, "list posts: %v", err)
			return
		}
		renderBase(c, &tmpl.ViewData{
			Site:  site,
			Page:  &tmpl.Page{Title: "标签：" + name, Layout: "index", MenuID: "post", Lang: "zh-CN", Tag: name},
			Posts: toCards(posts),
		})
	})

	// RSS 2.0 订阅输出：/rss.xml，最新 20 篇 published 文章。
	r.GET("/rss.xml", func(c *gin.Context) {
		posts, err := st.ListPublishedPaged(1, 20)
		if err != nil {
			c.String(http.StatusInternalServerError, "list posts: %v", err)
			return
		}
		siteURL := "http://" + c.Request.Host
		items := make([]Item, 0, len(posts))
		for _, p := range posts {
			desc := p.Excerpt
			if desc == "" {
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
			c.String(http.StatusInternalServerError, "marshal rss: %v", err)
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

		output, err := xml.MarshalIndent(sm, "", "  ")
		if err != nil {
			c.String(http.StatusInternalServerError, "marshal sitemap: %v", err)
			return
		}
		c.Data(http.StatusOK, "application/xml; charset=utf-8", append([]byte(xml.Header), output...))
	})

	addr := ":8081" // qoder worktree 固定 8081，与 dev 的 8080 区分
	fmt.Printf("Stellar Demo 3 server listening on http://localhost%s\n", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("run: %v", err)
	}
}

// truncateText 按 rune 截断字符串，超长加省略号（widget 摘要用）。
func truncateText(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
