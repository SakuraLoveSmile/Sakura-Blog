package main

import (
	"html/template"
	"log"
	"os"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"

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
