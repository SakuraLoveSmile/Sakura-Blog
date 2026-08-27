package routes

import (
	"errors"
	"html/template"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"
)

// registerTaxonomyRoutes 注册聚合/索引域路由：友链、文库、手记、归档、分类、专栏、标签。
func (s *webServer) registerTaxonomyRoutes(r *gin.Engine) {
	// 友链卡片页 /links（内容页化）：pages 表 slug=links 系统页渲染，正文 {% links %} 直出分组卡片。
	r.GET("/links", s.handleLinksPage)

	// 文库 /wiki（+ /wiki/tags/:tag 标签过滤），对齐 Stellar index_wiki。配置驱动（site.yml wiki 段）。
	r.GET("/wiki", s.handleWiki)
	r.GET("/wiki/", s.handleWiki)
	r.GET("/wiki/tags/:tag", s.handleWiki)
	// Spec 35a：wiki 文档三级路由（/wiki/:project 项目首页=首篇文档重定向；/wiki/:project/:slug 文档页）。
	// 注册在 /wiki/tags/:tag 之后：Gin 静态段优先，/wiki/tags/* 仍命中标签过滤页，
	// 且 handleWikiDoc 对 project=tags 做防御式 404，语义不冲突。
	r.GET("/wiki/:project", s.handleWikiProject)
	r.GET("/wiki/:project/:slug", s.handleWikiDoc)

	// 笔记(手记) /notes（内容页化）：正文取 pages 表 slug=notes 系统页（控制台可编辑，
	// 首次启动自 content/notes.md 迁移）；瞬间由正文中的 {% moments %} 组件按需插入。
	r.GET("/notes", func(c *gin.Context) {
		var contentHTML template.HTML
		var pageModel *tmpl.Page
		if p, err := s.st.GetPageBySlug(NotesSlug); err == nil && p != nil && p.Status == store.StatusPublished {
			if rendered, e := renderStoredContent(s.posts, p.ContentMD, p.ContentHTML); e == nil {
				contentHTML = template.HTML(rendered)
			}
			pageModel = buildPageModel(s.site.Load(), p, "/notes", "notes")
			pageModel.Layout = "notes"
		} else if raw, err := os.ReadFile(s.notesMDPath); err == nil {
			// 兼容未迁移场景（种子失败等）：回退旧文件读取
			if rendered, e := s.posts.Render(string(raw)); e == nil {
				contentHTML = template.HTML(rendered)
			}
		}
		if pageModel == nil {
			pageModel = &tmpl.Page{Title: s.menuTitle("notes", "笔记"), Layout: "notes", MenuID: "notes", Lang: "zh-CN", Path: "/notes"}
		}
		data := &tmpl.ViewData{
			Site:             s.site.Load(),
			Page:             pageModel,
			NotesContentHTML: contentHTML,
		}
		// 右栏 TOC widget 数据：与文章页一致，从渲染正文提取目录（notes.md 的 quot el:h2 标题带 id 锚点）。
		data.TOC = stellarweb.ExtractTOCWith(string(contentHTML), s.tocOptions())
		s.renderBase(c, data)
	})

	// 友链 /friends（内容页化）：pages 表 slug=friends 系统页渲染，正文 md 控制台可编辑，
	// 友链列表经 {% friends posts:true %} shortcode 嵌入；banner/评论/TOC 同内容页。
	r.GET("/friends", s.handleFriendsPage)

	// 归档页：按年份分组的文章列表（对齐 Stellar archive.ejs）
	r.GET("/archives", func(c *gin.Context) {
		posts, err := s.st.ListPublished()
		if err != nil {
			log.Printf("list posts: %v", err)
			s.render500(c)
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
		s.renderBase(c, &tmpl.ViewData{
			Site:     s.site.Load(),
			Page:     &tmpl.Page{Title: "归档", Layout: "archive", MenuID: "archive", Lang: "zh-CN"},
			Archives: years,
		})
	})

	// 分类索引页：所有分类 + 文章数（对齐 Stellar categories.ejs）
	r.GET("/categories", func(c *gin.Context) {
		tree, err := s.st.CategoryTree()
		if err != nil {
			log.Printf("count categories: %v", err)
			s.render500(c)
			return
		}
		s.renderBase(c, &tmpl.ViewData{
			Site: s.site.Load(),
			Page: &tmpl.Page{Title: "分类", Layout: "categories", MenuID: "post", Lang: "zh-CN"},
			Cats: categoryItems(tree, s.site.Load().Article.CategoryIcon),
		})
	})

	// 分类文章列表页：复用首页卡片（对齐 Stellar archive.ejs 的 partial('index') 行为）
	r.GET("/categories/:name", func(c *gin.Context) {
		s.handleCategoryPath(c, c.Param("name"))
	})

	// 专栏索引页（Spec 52）：索引页与详情页均由 topics 表驱动；详情兼容旧的 name 路径。
	r.GET("/topic", s.handleTopicIndex)
	r.GET("/topics", s.handleTopicIndex)
	// 专栏文章列表页（Spec 52）：/topic/:slug 专栏页（专栏头 + 文章卡片流）；
	// /topic/:name 兼容旧路径 301 到 slug 路径（gin 不允许同前缀注册两个不同参数名，
	// 旧名兼容在 handleTopicDetail 内按 name 兑底处理）。
	r.GET("/topic/:slug", s.handleTopicDetail)

	// 标签索引页：标签云（按文章数 DESC）
	r.GET("/tags", func(c *gin.Context) {
		counts, err := s.st.CountByTag()
		if err != nil {
			log.Printf("count tags: %v", err)
			s.render500(c)
			return
		}
		items := make([]*tmpl.TagItem, 0, len(counts))
		for name, n := range counts {
			items = append(items, &tmpl.TagItem{Name: name, Count: n, Icon: s.site.Load().Article.TagIcon[name]})
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].Count != items[j].Count {
				return items[i].Count > items[j].Count
			}
			return items[i].Name < items[j].Name
		})
		s.renderBase(c, &tmpl.ViewData{
			Site:     s.site.Load(),
			Page:     &tmpl.Page{Title: "标签", Layout: "tags", MenuID: "post", Lang: "zh-CN"},
			TagItems: items,
		})
	})

	// 标签文章列表页：复用首页卡片
	r.GET("/tags/:name", func(c *gin.Context) {
		name := c.Param("name")
		counts, err := s.st.CountByTag()
		if err != nil {
			log.Printf("count tags: %v", err)
			s.render500(c)
			return
		}
		if _, ok := counts[name]; !ok {
			s.render404(c)
			return
		}
		posts, err := s.st.ListByTag(name)
		if err != nil {
			log.Printf("list posts: %v", err)
			s.render500(c)
			return
		}
		s.renderBase(c, &tmpl.ViewData{
			Site:  s.site.Load(),
			Page:  &tmpl.Page{Title: "标签：" + name, Layout: "index", MenuID: "post", Lang: "zh-CN", Tag: name, Type: s.site.Load().Article.Type},
			Posts: s.toCards(posts),
		})
	})

	// 作者文章列表页（Spec 38 多作者体系）：/author/:id，对齐 Stellar author.ejs 的“全部文章”链接。
	r.GET("/author/:id", s.handleAuthor)
}

// categoryItems converts the Store tree to the recursive template model.
func categoryItems(nodes []*store.CategoryNode, icons map[string]string) []*tmpl.CatItem {
	items := make([]*tmpl.CatItem, 0, len(nodes))
	for _, n := range nodes {
		icon := icons[n.Path]
		if icon == "" {
			icon = icons[n.Name]
		}
		items = append(items, &tmpl.CatItem{
			ID: n.ID, Name: n.Name, Path: n.Path, ParentID: n.ParentID, Count: n.Count, Icon: icon,
			Children: categoryItems(n.Children, icons),
		})
	}
	return items
}

// handleCategoryPath 是所有分类详情入口的统一路径解析逻辑。
// NoRoute 会把包含多个路径段的 /categories/a/b/ 也转交到此方法。
func (s *webServer) handleCategoryPath(c *gin.Context, path string) {
	path = strings.Trim(path, "/")
	if path == "" || strings.HasSuffix(path, "/rss.xml") {
		s.render404(c)
		return
	}
	posts, err := s.st.ListByCategoryPath(path)
	if err != nil {
		if errors.Is(err, store.ErrCategoryNotFound) {
			s.render404(c)
			return
		}
		log.Printf("list posts by category: %v", err)
		s.render500(c)
		return
	}
	s.renderBase(c, &tmpl.ViewData{
		Site:  s.site.Load(),
		Page:  &tmpl.Page{Title: "分类：" + path, Layout: "index", MenuID: "post", Lang: "zh-CN", Category: path, Type: s.site.Load().Article.Type},
		Posts: s.toCards(posts),
	})
}
