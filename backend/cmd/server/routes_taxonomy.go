package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
	tmpl "blog/internal/template"
)

// registerTaxonomyRoutes 注册聚合/索引域路由：友链、文库、笔记、归档、分类、专栏、标签。
func (s *webServer) registerTaxonomyRoutes(r *gin.Engine) {
	// 友链页：site.yml links 配置驱动（对齐 Stellar sites 卡片布局）
	r.GET("/links", func(c *gin.Context) {
		s.renderBase(c, &tmpl.ViewData{
			Site: s.site,
			Page: &tmpl.Page{Title: "友链", Layout: "links", MenuID: "friends", Lang: "zh-CN"},
		})
	})

	// 文库 /wiki（+ /wiki/tags/:tag 标签过滤），对齐 Stellar index_wiki。配置驱动（site.yml wiki 段）。
	r.GET("/wiki", s.handleWiki)
	r.GET("/wiki/", s.handleWiki)
	r.GET("/wiki/tags/:tag", s.handleWiki)
	// Spec 35a：wiki 文档三级路由（/wiki/:project 项目首页=首篇文档重定向；/wiki/:project/:slug 文档页）。
	// 注册在 /wiki/tags/:tag 之后：Gin 静态段优先，/wiki/tags/* 仍命中标签过滤页，
	// 且 handleWikiDoc 对 project=tags 做防御式 404，语义不冲突。
	r.GET("/wiki/:project", s.handleWikiProject)
	r.GET("/wiki/:project/:slug", s.handleWikiDoc)

	// 笔记(手记) /notes，对齐 xaoxuu.com/notes/：上段 content/notes.md 手记正文（quot/okr 等 shortcode），下段本站 moments 近期动态。
	r.GET("/notes", func(c *gin.Context) {
		var contentHTML template.HTML
		if raw, err := os.ReadFile(s.notesMDPath); err == nil {
			if rendered, err := s.posts.Render(string(raw)); err == nil {
				contentHTML = template.HTML(rendered)
			}
		}
		s.renderBase(c, &tmpl.ViewData{
			Site:             s.site,
			Page:             &tmpl.Page{Title: "笔记", Layout: "notes", MenuID: "notes", Lang: "zh-CN"},
			NotesContentHTML: contentHTML,
			Moments:          s.momentH.RecentItems(5),
		})
	})

	// 友链 /friends（友链卡片 + RSS 动态聚合），数据由 friends 聚合器的 /api/friends-aggregated 提供。
	r.GET("/friends", func(c *gin.Context) {
		s.renderBase(c, &tmpl.ViewData{
			Site: s.site,
			Page: &tmpl.Page{Title: "友链", Layout: "friends", MenuID: "friends", Lang: "zh-CN"},
		})
	})

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
			Site:     s.site,
			Page:     &tmpl.Page{Title: "归档", Layout: "archive", MenuID: "archive", Lang: "zh-CN"},
			Archives: years,
		})
	})

	// 分类索引页：所有分类 + 文章数（对齐 Stellar categories.ejs）
	r.GET("/categories", func(c *gin.Context) {
		counts, err := s.st.CountByCategory()
		if err != nil {
			log.Printf("count categories: %v", err)
			s.render500(c)
			return
		}
		cats := make([]*tmpl.CatItem, 0, len(counts))
		for name, n := range counts {
			cats = append(cats, &tmpl.CatItem{Name: name, Count: n})
		}
		sort.Slice(cats, func(i, j int) bool { return cats[i].Name < cats[j].Name })
		s.renderBase(c, &tmpl.ViewData{
			Site: s.site,
			Page: &tmpl.Page{Title: "分类", Layout: "categories", MenuID: "post", Lang: "zh-CN"},
			Cats: cats,
		})
	})

	// 分类文章列表页：复用首页卡片（对齐 Stellar archive.ejs 的 partial('index') 行为）
	r.GET("/categories/:name", func(c *gin.Context) {
		name := c.Param("name")
		counts, err := s.st.CountByCategory()
		if err != nil {
			log.Printf("count categories: %v", err)
			s.render500(c)
			return
		}
		if _, ok := counts[name]; !ok {
			s.render404(c)
			return
		}
		posts, err := s.st.ListByCategory(name)
		if err != nil {
			log.Printf("list posts: %v", err)
			s.render500(c)
			return
		}
		s.renderBase(c, &tmpl.ViewData{
			Site:  s.site,
			Page:  &tmpl.Page{Title: "分类：" + name, Layout: "index", MenuID: "post", Lang: "zh-CN", Category: name},
			Posts: s.toCards(posts),
		})
	})

	// 专栏索引页：专栏卡片列表（对齐 Stellar index_topic.ejs，图标缺省用主题 default.topic 占位图）
	r.GET("/topic", func(c *gin.Context) {
		counts, err := s.st.CountByTopic()
		if err != nil {
			log.Printf("count topics: %v", err)
			s.render500(c)
			return
		}
		if len(counts) == 0 {
			s.render404(c)
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
		s.renderBase(c, &tmpl.ViewData{
			Site:   s.site,
			Page:   &tmpl.Page{Title: "专栏", Layout: "topics", MenuID: "post", Lang: "zh-CN"},
			Topics: topics,
		})
	})

	// 专栏文章列表页：复用首页卡片（对齐原版 topic 文章列表）
	r.GET("/topic/:name", func(c *gin.Context) {
		name := c.Param("name")
		counts, err := s.st.CountByTopic()
		if err != nil {
			log.Printf("count topics: %v", err)
			s.render500(c)
			return
		}
		if _, ok := counts[name]; !ok {
			s.render404(c)
			return
		}
		posts, err := s.st.ListByTopic(name)
		if err != nil {
			log.Printf("list posts: %v", err)
			s.render500(c)
			return
		}
		s.renderBase(c, &tmpl.ViewData{
			Site:  s.site,
			Page:  &tmpl.Page{Title: "专栏：" + name, Layout: "index", MenuID: "post", Lang: "zh-CN", Topic: name},
			Posts: s.toCards(posts),
		})
	})

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
			items = append(items, &tmpl.TagItem{Name: name, Count: n})
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].Count != items[j].Count {
				return items[i].Count > items[j].Count
			}
			return items[i].Name < items[j].Name
		})
		s.renderBase(c, &tmpl.ViewData{
			Site:     s.site,
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
			Site:  s.site,
			Page:  &tmpl.Page{Title: "标签：" + name, Layout: "index", MenuID: "post", Lang: "zh-CN", Tag: name},
			Posts: s.toCards(posts),
		})
	})

	// 作者文章列表页（Spec 38 多作者体系）：/author/:id，对齐 Stellar author.ejs 的“全部文章”链接。
	r.GET("/author/:id", s.handleAuthor)
}

// handleWiki 文库页：全量标签 tab + 可选 tag 过滤 + Sort 降序项目列表。
func (s *webServer) handleWiki(c *gin.Context) {
	tag := c.Param("tag")
	// 收集全量标签（顶部 tab，保留首见顺序）
	var tags []string
	tagSeen := map[string]bool{}
	for i := range s.site.Wiki {
		for _, t := range s.site.Wiki[i].Tags {
			if !tagSeen[t] {
				tagSeen[t] = true
				tags = append(tags, t)
			}
		}
	}
	if tag != "" && !tagSeen[tag] {
		s.render404(c)
		return
	}
	// 项目（可按 tag 过滤），按 Sort 降序
	projects := make([]*tmpl.WikiProject, 0, len(s.site.Wiki))
	for i := range s.site.Wiki {
		w := &s.site.Wiki[i]
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
	s.renderBase(c, &tmpl.ViewData{
		Site:         s.site,
		Page:         &tmpl.Page{Title: "文库", Layout: "wiki", MenuID: "wiki", Lang: "zh-CN", Tag: tag},
		WikiProjects: projects,
		WikiTags:     tags,
	})
}

// handleWikiProject 项目首页（Spec 35a）：重定向到该项目第一篇已发布文档（/wiki/:project/:slug）。
// 无已发布文档时渲染文库布局空态（复用 wiki.html 的 wiki.empty 提示）。
func (s *webServer) handleWikiProject(c *gin.Context) {
	projectID := c.Param("project")
	// 路由安全：/wiki/tags 与 /wiki/tags/:tag 已由 handleWiki 处理，防御式拦截避免语义混淆。
	if projectID == "tags" {
		s.render404(c)
		return
	}
	// 项目存在性校验（配置驱动：site.yml wiki 段未配置的项目不可访问）
	if s.wikiProjectByID(projectID) == nil {
		s.render404(c)
		return
	}
	docs, err := s.st.ListWikidocs(projectID)
	if err != nil {
		log.Printf("list wikidocs %s: %v", projectID, err)
		s.render500(c)
		return
	}
	// 第一个已发布文档 → 项目首页即文档页
	for _, d := range docs {
		if d.Status == store.StatusPublished {
			c.Redirect(http.StatusFound, "/wiki/"+projectID+"/"+d.Slug)
			return
		}
	}
	// 无已发布文档：渲染项目首页空态（复用 wiki 布局）
	s.renderBase(c, &tmpl.ViewData{
		Site: s.site,
		Page: &tmpl.Page{Title: "文库", Layout: "wiki", MenuID: "wiki", Lang: "zh-CN"},
	})
}

// handleWikiDoc 文档页（Spec 35a）：渲染 wiki 文档（wiki 维度面包屑 + 正文 + 上下篇），
// leftbar tree widget 经 assembleWidgets 注入（数据仅本页有，不进侧栏快照）。
func (s *webServer) handleWikiDoc(c *gin.Context) {
	projectID := c.Param("project")
	slug := c.Param("slug")
	// 路由安全：/wiki/tags/:tag 已注册（静态段优先），本路由不会收到 project=tags；
	// 防御式校验避免与标签过滤页语义冲突。
	if projectID == "tags" {
		s.render404(c)
		return
	}
	project := s.wikiProjectByID(projectID)
	if project == nil {
		s.render404(c)
		return
	}
	docs, err := s.st.ListWikidocs(projectID)
	if err != nil {
		log.Printf("list wikidocs %s: %v", projectID, err)
		s.render500(c)
		return
	}
	// 当前文档：仅已发布可访问（draft 走 404）；published 序列按 doc_order,id 升序
	// （ListWikidocs 保证），同列表内相邻即上下篇。
	var cur *store.WikiDoc
	var published []*store.WikiDoc
	for _, d := range docs {
		if d.Status != store.StatusPublished {
			continue
		}
		published = append(published, d)
		if d.Slug == slug {
			cur = d
		}
	}
	if cur == nil {
		s.render404(c)
		return
	}
	var prev, next *tmpl.WikiDocNav
	for i, d := range published {
		if d.Slug != slug {
			continue
		}
		if i > 0 {
			prev = &tmpl.WikiDocNav{Slug: published[i-1].Slug, Title: published[i-1].Title}
		}
		if i+1 < len(published) {
			next = &tmpl.WikiDocNav{Slug: published[i+1].Slug, Title: published[i+1].Title}
		}
		break
	}
	// 文档树（leftbar tree widget）：过滤未发布节点（draft 不可见）
	tree, err := s.st.BuildDocTree(projectID)
	if err != nil {
		log.Printf("build doc tree %s: %v", projectID, err)
		s.render500(c)
		return
	}
	s.renderBase(c, &tmpl.ViewData{
		Site: s.site,
		Page: &tmpl.Page{
			Title:       cur.Title,
			Layout:      "wiki-doc",
			MenuID:      "wiki",
			Lang:        "zh-CN",
			Slug:        slug,
			Description: truncateText(stripHTML(cur.ContentHTML), 150),
		},
		WikiProjectID:   projectID,
		WikiProjectName: project.DisplayTitle(),
		WikiDoc: &tmpl.WikiDocView{
			Slug:        cur.Slug,
			Title:       cur.Title,
			ContentHTML: template.HTML(cur.ContentHTML),
		},
		WikiPrevDoc: prev,
		WikiNextDoc: next,
		WikiDocTree: wikiTreeNodes(tree),
	})
}

// wikiProjectByID 按 ID 查 site.yml wiki 段项目（配置驱动，未配置返回 nil）。
func (s *webServer) wikiProjectByID(id string) *tmpl.WikiProject {
	for i := range s.site.Wiki {
		if s.site.Wiki[i].ID == id {
			return &s.site.Wiki[i]
		}
	}
	return nil
}

// wikiTreeNodes store.TreeNode → 模板树节点（跳过未发布文档；draft 节点及其子树不可见）。
func wikiTreeNodes(nodes []store.TreeNode) []tmpl.WikiTreeNode {
	out := make([]tmpl.WikiTreeNode, 0, len(nodes))
	for _, n := range nodes {
		if n.Doc == nil || n.Doc.Status != store.StatusPublished {
			continue
		}
		out = append(out, tmpl.WikiTreeNode{
			Doc: &tmpl.WikiDocView{
				Slug:  n.Doc.Slug,
				Title: n.Doc.Title,
			},
			Children: wikiTreeNodes(n.Children),
		})
	}
	return out
}

// handleAuthor 作者文章列表页（Spec 38 多作者体系）。
// 对齐 Stellar author.ejs：检查作者存在 → 分页查询文章 → 构建 ViewData → 渲染。
func (s *webServer) handleAuthor(c *gin.Context) {
	id := c.Param("id")
	// 检查作者是否存在
	author, ok := s.site.Authors[id]
	if !ok {
		s.render404(c)
		return
	}
	// 分页参数
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	const pageSize = 10
	total, err := s.st.CountByAuthor(id)
	if err != nil {
		log.Printf("count posts by author %s: %v", id, err)
		s.render500(c)
		return
	}
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages > 0 && page > totalPages {
		page = totalPages
	}
	offset := (page - 1) * pageSize
	posts, err := s.st.ListByAuthor(id, offset, pageSize)
	if err != nil {
		log.Printf("list posts by author %s: %v", id, err)
		s.render500(c)
		return
	}
	pages := make([]int, totalPages)
	for i := range pages {
		pages[i] = i + 1
	}
	data := &tmpl.ViewData{
		Site: s.site,
		Page: &tmpl.Page{
			Title:  tmpl.Lang("author.page_title", author.Name),
			Layout: "author",
			MenuID: "post",
			Lang:   "zh-CN",
			Author: id,
		},
		Posts:         s.toCards(posts),
		Paginator:     &tmpl.Paginator{Current: page, Total: totalPages, Pages: pages},
		AuthorProfile: &author,
	}
	s.fillSidebar(data)
	s.fillSEO(data, c)
	html, err := s.engine.Render("layout/base", data)
	if err != nil {
		log.Printf("render: %v", err)
		s.render500(c)
		return
	}
	c.Data(200, "text/html; charset=utf-8", []byte(html))
}
