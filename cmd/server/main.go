package main

import (
	"flag"
	"fmt"
	"html/template"
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
			toc, htmlWithIDs := post.ExtractTOC(p.ContentHTML)
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
				Content: template.HTML(htmlWithIDs),
				TOCHTML: template.HTML(post.BuildTOCHTML(toc)),
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

		toc, htmlWithIDs := post.ExtractTOC(p.HTML)
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

	addr := ":8080"
	fmt.Printf("Stellar Demo 3 server listening on http://localhost%s\n", addr)
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
		excerpt := p.Excerpt
		if excerpt == "" {
			excerpt = post.ExcerptFromHTML(p.ContentHTML, 120)
		}
		cards = append(cards, &tmpl.PostCard{
			Slug:    p.Slug,
			Title:   p.Title,
			Excerpt: excerpt,
			Cover:   p.Cover,
			Date:    p.CreatedAt,
		})
	}
	return cards
}

// buildPaginator 构造分页器。baseURL 留空走首页 /page/N 风格，非空走列表页 ?page=N 风格。
func buildPaginator(page, total int, baseURL string) *tmpl.Paginator {
	pages := make([]int, total)
	for i := range pages {
		pages[i] = i + 1
	}
	return &tmpl.Paginator{Current: page, Total: total, Pages: pages, BaseURL: baseURL}
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
		Paginator: buildPaginator(page, totalPages, ""),
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
		Site: site,
		Page: &tmpl.Page{Title: "标签", Layout: "tags", Type: "tech", Lang: "zh-CN", MenuID: "post"},
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
		Site: site,
		Page: &tmpl.Page{Title: "分类", Layout: "categories", Type: "tech", Lang: "zh-CN", MenuID: "post"},
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
		Site: site,
		Page: &tmpl.Page{Title: labelType + ":" + name, Layout: "list", Type: "tech", Lang: "zh-CN", MenuID: "post"},
		Posts:     toCards(posts),
		Paginator: buildPaginator(page, totalPages, baseURL),
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
		Site: site,
		Page: &tmpl.Page{Title: "归档", Layout: "archive", Type: "tech", Lang: "zh-CN", MenuID: "post"},
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
	if q != "" && page > totalPages {
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
		Paginator:   buildPaginator(page, totalPages, "/search?q="+url.QueryEscape(q)),
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
	Title   string
	Slug    string
	Link    string // 文章绝对 URL
	Summary string // 摘要纯文本
	Content string // 正文 HTML（CDATA 包裹）
	Published string // RFC3339
	Updated   string // RFC3339
}

// renderAtom 输出 Atom 1.0 feed（/atom.xml）。取最近 feedMaxEntries 篇已发布文章。
// 用 text/template 渲染，文本字段走 xmlEscape 手动转义，正文 HTML 用 CDATA 包裹避免双重转义。
func renderAtom(c *gin.Context, st *store.Store, site *tmpl.Site) {
	posts, err := st.ListPublished()
	if err != nil {
		c.String(http.StatusInternalServerError, "list posts for feed: %v", err)
		return
	}

	baseURL := strings.TrimRight(siteURL(site), "/")
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
			Slug:      p.Slug,
			Link:      baseURL + "/post/" + p.Slug,
			Summary:   excerpt,
			Content:   p.ContentHTML,
			Published: p.CreatedAt.UTC().Format(time.RFC3339),
			Updated:   p.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}

	data := struct {
		Site    *tmpl.Site
		BaseURL string
		Updated string // feed 级更新时间，取首篇 updated 或当前时间
		Entries []feedEntry
	}{
		Site:    site,
		BaseURL: baseURL,
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

// atomTpl Atom 1.0 模板。text/template 不自动转义，文本字段显式调 xmlEscape；
// 正文 HTML 用 CDATA 包裹（CDATA 内的 ]]> 极罕见，不额外处理）。
var atomTpl = textTemplate.Must(textTemplate.New("atom").Funcs(textTemplate.FuncMap{
	"xmlEscape": xmlEscape,
}).Parse(`<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>{{xmlEscape .Site.Title}}</title>
  <subtitle>{{xmlEscape .Site.Subtitle}}</subtitle>
  <link href="{{.BaseURL}}/atom.xml" rel="self"/>
  <link href="{{.BaseURL}}/"/>
  <updated>{{.Updated}}</updated>
  <id>{{.BaseURL}}/</id>
  <author>
    <name>{{xmlEscape .Site.Author}}</name>
  </author>
  {{range .Entries}}
  <entry>
    <title>{{xmlEscape .Title}}</title>
    <link href="{{.Link}}"/>
    <id>{{.Link}}</id>
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

// feedUpdated 取首篇 entry 的 updated 作为 feed 级更新时间，无文章用当前时间。
func feedUpdated(entries []feedEntry) string {
	if len(entries) > 0 && entries[0].Updated != "" {
		return entries[0].Updated
	}
	return time.Now().UTC().Format(time.RFC3339)
}
