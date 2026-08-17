package routes

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"blog/internal/store"
	"blog/internal/textutil"
	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"
)

// wikiReadmePlaceholder mirrors Stellar's wiki_readme helper: an empty project
// homepage with a GitHub repo is rendered from that repository's README.
func wikiReadmePlaceholder(project *tmpl.WikiProject) (template.HTML, bool) {
	if project == nil || !validWikiRepo(project.Repo) {
		return "", false
	}
	branch := strings.TrimSpace(project.Branch)
	if branch == "" {
		branch = "HEAD"
	}
	if !validWikiBranch(branch) {
		return "", false
	}
	base := "https://raw.githubusercontent.com/" + strings.Trim(project.Repo, "/") + "/" + branch + "/"
	src := base + "README.md"
	return template.HTML(`<div class="data-service ds-mdrender" src="` + template.HTMLEscapeString(src) + `" data-base="` + template.HTMLEscapeString(base) + `" data-replace="true" data-heading="true"></div>`), true
}

func validWikiRepo(repo string) bool {
	parts := strings.Split(strings.Trim(repo, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	for _, part := range parts {
		for _, r := range part {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("-_.", r) {
				continue
			}
			return false
		}
	}
	return true
}

func validWikiBranch(branch string) bool {
	if branch == "" || strings.HasPrefix(branch, "/") || strings.HasSuffix(branch, "/") || strings.Contains(branch, "//") {
		return false
	}
	for _, part := range strings.Split(branch, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
		for _, r := range part {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("-_.", r) {
				continue
			}
			return false
		}
	}
	return true
}

// registerTaxonomyRoutes 注册聚合/索引域路由：友链、文库、笔记、归档、分类、专栏、标签。
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

	// 笔记本体系（Spec 35b）：/notebooks 列表、/notebooks/:name 笔记列表（分页 + ?tag= 过滤）、
	// /note/:id 笔记页。与 /notes（手记单页）共存：/note/* 静态段不冲突。
	r.GET("/notebooks", s.handleNotebooks)
	r.GET("/notebooks/:name", s.handleNotebook)
	r.GET("/note/:id", s.handleNote)

	// 笔记(手记) /notes（内容页化）：正文取 pages 表 slug=notes 系统页（控制台可编辑，
	// 首次启动自 content/notes.md 迁移）；下段仍为本站 moments 近期动态。
	r.GET("/notes", func(c *gin.Context) {
		var contentHTML template.HTML
		if p, err := s.st.GetPageBySlug(NotesSlug); err == nil && p != nil && p.Status == store.StatusPublished {
			if p.ContentHTML != "" {
				contentHTML = template.HTML(p.ContentHTML)
			} else if rendered, e := s.posts.Render(p.ContentMD); e == nil {
				contentHTML = template.HTML(rendered)
			}
		} else if raw, err := os.ReadFile(s.notesMDPath); err == nil {
			// 兼容未迁移场景（种子失败等）：回退旧文件读取
			if rendered, e := s.posts.Render(string(raw)); e == nil {
				contentHTML = template.HTML(rendered)
			}
		}
		data := &tmpl.ViewData{
			Site:             s.site.Load(),
			Page:             &tmpl.Page{Title: s.menuTitle("notes", "笔记"), Layout: "notes", MenuID: "notes", Lang: "zh-CN"},
			NotesContentHTML: contentHTML,
			Moments:          s.momentH.RecentItems(5),
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
		counts, err := s.st.CountByCategory()
		if err != nil {
			log.Printf("count categories: %v", err)
			s.render500(c)
			return
		}
		cats := make([]*tmpl.CatItem, 0, len(counts))
		for name, n := range counts {
			cats = append(cats, &tmpl.CatItem{Name: name, Count: n, Icon: s.site.Load().Article.CategoryIcon[name]})
		}
		sort.Slice(cats, func(i, j int) bool { return cats[i].Name < cats[j].Name })
		s.renderBase(c, &tmpl.ViewData{
			Site: s.site.Load(),
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
			Site:  s.site.Load(),
			Page:  &tmpl.Page{Title: "分类：" + name, Layout: "index", MenuID: "post", Lang: "zh-CN", Category: name, Type: s.site.Load().Article.Type},
			Posts: s.toCards(posts),
		})
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

// handleTopicIndex 专栏索引页：按 topics.sort DESC 展示专栏名称、描述、封面与已发布文章数。
// 专栏没有配置描述/封面时，使用该专栏最新文章的摘要/封面，再回退到本地预览素材。
func (s *webServer) handleTopicIndex(c *gin.Context) {
	topics, err := s.st.ListTopics()
	if err != nil {
		log.Printf("list topics: %v", err)
		s.render500(c)
		return
	}
	items := make([]*tmpl.TopicItem, 0, len(topics))
	fallbackCovers := []string{
		"/assets/img/preview/gallery-1.jpg",
		"/assets/img/preview/gallery-2.jpg",
		"/assets/img/preview/gallery-3.jpg",
		"/assets/img/preview/image-demo.jpg",
	}
	for i, t := range topics {
		n, err := s.st.CountByTopicID(t.ID)
		if err != nil {
			log.Printf("count posts of topic %d: %v", t.ID, err)
			s.render500(c)
			return
		}

		description := strings.TrimSpace(t.Description)
		cover := strings.TrimSpace(t.Cover)
		posts, err := s.st.ListPostsByTopicID(t.ID)
		if err != nil {
			log.Printf("list posts of topic %d: %v", t.ID, err)
			s.render500(c)
			return
		}
		if len(posts) > 0 {
			if description == "" {
				description = strings.TrimSpace(posts[0].Excerpt)
			}
			if cover == "" {
				cover = strings.TrimSpace(posts[0].Cover)
			}
		}
		if description == "" {
			description = fmt.Sprintf("围绕“%s”的文章与实践记录。", t.Name)
		}
		if cover == "" {
			cover = strings.TrimSpace(t.Icon)
		}
		if cover == "" {
			cover = fallbackCovers[i%len(fallbackCovers)]
		}
		slug := strings.TrimSpace(t.Slug)
		if slug == "" {
			slug = t.Name
		}
		items = append(items, &tmpl.TopicItem{
			Name:        t.Name,
			Slug:        slug,
			Count:       n,
			Description: description,
			Cover:       cover,
			Icon:        t.Icon,
			CreatedAt:   t.CreatedAt,
		})
	}
	s.renderBase(c, &tmpl.ViewData{
		Site:   s.site.Load(),
		Page:   &tmpl.Page{Title: "专栏", Layout: "topics", MenuID: "post", Lang: "zh-CN"},
		Topics: items,
	})
}

// handleTopicDetail 专栏文章列表页（Spec 52）：专栏头（名称/描述/图标/封面 + 文章数）+ 文章卡片流。
// 数据走独立 topics 表（slug 定位 + topic_id 关联文章）；slug 未命中时按 name 兑底 301 到 slug 路径。
func (s *webServer) handleTopicDetail(c *gin.Context) {
	slug := c.Param("slug")
	t, err := s.st.GetTopicBySlug(slug)
	if err != nil {
		log.Printf("get topic %q: %v", slug, err)
		s.render500(c)
		return
	}
	// /topic/:name 旧路径兼容（Spec 52）：slug 未命中时按 name 查，命中则 301 到 slug 路径。
	if t == nil {
		legacy, err := s.st.GetTopicByName(slug)
		if err != nil {
			log.Printf("get topic by name %q: %v", slug, err)
			s.render500(c)
			return
		}
		if legacy != nil {
			c.Redirect(http.StatusMovedPermanently, "/topic/"+legacy.Slug)
			return
		}
		s.render404(c)
		return
	}
	posts, err := s.st.ListPostsByTopicID(t.ID)
	if err != nil {
		log.Printf("list posts of topic %d: %v", t.ID, err)
		s.render500(c)
		return
	}
	count, err := s.st.CountByTopicID(t.ID)
	if err != nil {
		log.Printf("count posts of topic %d: %v", t.ID, err)
		s.render500(c)
		return
	}
	hue := 0
	for _, r := range t.Name {
		hue = (hue*31 + int(r)) % 360
	}
	initial := ""
	if rs := []rune(t.Name); len(rs) > 0 {
		initial = string(rs[0])
	}
	s.renderBase(c, &tmpl.ViewData{
		Site: s.site.Load(),
		Page: &tmpl.Page{
			Title:  stellarweb.Lang("nav.topic", t.Name),
			Layout: "topic",
			MenuID: "post",
			Lang:   "zh-CN",
			Topic:  t.Name,
			Type:   s.site.Load().Article.Type,
		},
		TopicDetail: &tmpl.TopicDetail{
			Slug:        t.Slug,
			Name:        t.Name,
			Description: t.Description,
			Icon:        t.Icon,
			Cover:       t.Cover,
			Count:       count,
			Hue:         hue,
			Initial:     initial,
		},
		Posts: s.toCards(posts),
	})
}

// handleLinksPage 友链卡片页（内容页化）：渲染 pages 表 slug=links 系统页。
// 正文 {% links %} 服务端直出分组友链卡片；banner/TOC 同内容页。
func (s *webServer) handleLinksPage(c *gin.Context) {
	p, err := s.st.GetPageBySlug(LinksSlug)
	if err != nil || p == nil || p.Status != store.StatusPublished {
		s.render404(c)
		return
	}
	contentHTML := p.ContentHTML
	if contentHTML == "" {
		if rendered, e := s.posts.Render(p.ContentMD); e == nil {
			contentHTML = rendered
		}
	}
	data := &tmpl.ViewData{
		Site:    s.site.Load(),
		Page:    &tmpl.Page{Title: s.menuTitle("friends", "友链"), Layout: "page", MenuID: p.Slug, Lang: "zh-CN", Slug: p.Slug},
		Content: template.HTML(contentHTML),
	}
	data.TOC = stellarweb.ExtractTOCWith(contentHTML, s.tocOptions())
	s.renderBase(c, data)
}

// menuTitle 页面标题与菜单配置联动：取 menubar 中 ID==menuID 的菜单项标题，
// 未配置时回退 fallback（避免标题硬编码与菜单改名脱节）。
func (s *webServer) menuTitle(menuID, fallback string) string {
	for _, it := range s.site.Load().Menubar.Items {
		if it.ID == menuID && it.Title != "" {
			return it.Title
		}
	}
	return fallback
}

// handleFriendsPage 友链页（内容页化）：渲染 pages 表 slug=friends 系统页。
// 正文 md 控制台可编辑（{% friends posts:true %} 嵌入友链组件）；
// banner 标题/封面/副标题取 pages.friends 设置（缺省回退页面标题）；TOC 与评论同内容页。
func (s *webServer) handleFriendsPage(c *gin.Context) {
	p, err := s.st.GetPageBySlug(FriendsSlug)
	if err != nil || p == nil || p.Status != store.StatusPublished {
		s.render404(c)
		return
	}
	contentHTML := p.ContentHTML
	if contentHTML == "" {
		if rendered, e := s.posts.Render(p.ContentMD); e == nil {
			contentHTML = rendered
		}
	}
	title := s.site.Load().Pages.Friends.Title
	if title == "" {
		title = p.Title
	}
	pg := &tmpl.Page{Title: title, Layout: "page", MenuID: "friends", Lang: "zh-CN", Slug: p.Slug}
	if hero := s.site.Load().Pages.Friends.Hero; hero != nil {
		pg.Banner = hero.Background
		pg.Subtitle = hero.Description
	}
	data := &tmpl.ViewData{Site: s.site.Load(), Page: pg, Content: template.HTML(contentHTML)}
	data.TOC = stellarweb.ExtractTOCWith(contentHTML, s.tocOptions())
	s.fillComments(data, "/friends")
	s.renderBase(c, data)
}

// handleWiki 文库页：全量标签 tab + 可选 tag 过滤 + Sort 降序项目列表。
func (s *webServer) handleWiki(c *gin.Context) {
	tag := c.Param("tag")
	// 收集全量标签（顶部 tab，保留首见顺序）
	var tags []string
	tagSeen := map[string]bool{}
	for i := range s.site.Load().Wiki {
		for _, t := range s.site.Load().Wiki[i].Tags {
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
	// 项目（可按 tag 过滤），按 Sort 降序。
	// 复制项目值而非引用 s.site.Load().Wiki 共享切片元素：HasDocs 是运行时标记，
	// 就地修改会污染配置源并引发并发竞态（多请求同时写同一元素）。
	projects := make([]*tmpl.WikiProject, 0, len(s.site.Load().Wiki))
	for i := range s.site.Load().Wiki {
		w := s.site.Load().Wiki[i]
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
		projects = append(projects, &w)
	}
	// Spec 62：HasDocs（有已发布本地文档的项目）从每项目一次 ListWikidocs 收敛为
	// 单条 GROUP BY 查询（CountPublishedWikidocsByProjects）。查询失败仅记日志：
	// HasDocs 保持 false 回退外链，不阻塞文库页渲染。
	projectIDs := make([]string, 0, len(projects))
	for _, p := range projects {
		projectIDs = append(projectIDs, p.ID)
	}
	if counts, err := s.st.CountPublishedWikidocsByProjects(projectIDs); err != nil {
		log.Printf("count wikidocs by projects: %v", err)
	} else {
		for _, p := range projects {
			p.HasDocs = counts[p.ID] > 0
		}
	}
	// Sort 降序，同值按 ID 字母序（与 Console 设置页/项目下拉的排序规则一致）
	sort.SliceStable(projects, func(i, j int) bool {
		if projects[i].Sort != projects[j].Sort {
			return projects[i].Sort > projects[j].Sort
		}
		return projects[i].ID < projects[j].ID
	})
	s.renderBase(c, &tmpl.ViewData{
		Site:         s.site.Load(),
		Page:         &tmpl.Page{Title: s.menuTitle("wiki", "文库"), Layout: "wiki", MenuID: "wiki", Lang: "zh-CN", Tag: tag},
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
		Site: s.site.Load(),
		Page: &tmpl.Page{Title: s.menuTitle("wiki", "文库"), Layout: "wiki", MenuID: "wiki", Lang: "zh-CN"},
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
	// 文档树（leftbar tree widget）：过滤未发布节点（draft 不可见）。
	// 复用上面 ListWikidocs 的 docs，纯内存构建，避免二次查询。
	tree := wikiDocTree(docs)
	// 最近更新（wiki 维度，Spec 8）：当前项目已发布文档按 UpdatedAt 降序取前 5，
	// 复用 RecentPost 结构（recent.html 模板零改动），供 assembleWidgets 的 recent widget 装配。
	recentDocs := make([]*tmpl.RecentPost, 0, len(published))
	sortedDocs := append([]*store.WikiDoc{}, published...)
	sort.SliceStable(sortedDocs, func(i, j int) bool { return sortedDocs[i].UpdatedAt.After(sortedDocs[j].UpdatedAt) })
	for i, d := range sortedDocs {
		if i >= 5 {
			break
		}
		recentDocs = append(recentDocs, &tmpl.RecentPost{Title: d.Title, URL: "/wiki/" + projectID + "/" + d.Slug})
	}
	contentHTML := template.HTML(cur.ContentHTML)
	if published[0] == cur && strings.TrimSpace(string(contentHTML)) == "" {
		if readme, ok := wikiReadmePlaceholder(project); ok {
			contentHTML = readme
		}
	}
	description := textutil.Truncate(textutil.StripHTML(string(contentHTML)), 150)
	if description == "" {
		description = textutil.Truncate(project.Description, 150)
	}
	data := &tmpl.ViewData{
		Site: s.site.Load(),
		Page: &tmpl.Page{
			Title:       cur.Title,
			Layout:      "wiki-doc",
			MenuID:      "wiki",
			Lang:        "zh-CN",
			Slug:        slug,
			Description: description,
		},
		WikiProjectID:          projectID,
		WikiProjectName:        project.DisplayTitle(),
		WikiProjectIcon:        project.Icon,
		WikiProjectDescription: project.Description,
		WikiHero:               project.Hero,
		// 搜索框项目化：data-filter 限定当前项目 + placeholder 显示项目名
		SearchFilter:      "/wiki/" + projectID + "/",
		SearchPlaceholder: "在 " + project.DisplayTitle() + " 中搜索",
		WikiRecentDocs:    recentDocs,
		WikiDoc: &tmpl.WikiDocView{
			Slug:        cur.Slug,
			Title:       cur.Title,
			ContentHTML: contentHTML,
			Updated:     cur.UpdatedAt.Format("2006-01-02"), // Spec 53 Part C：wiki dateinfo
		},
		WikiPrevDoc: prev,
		WikiNextDoc: next,
		WikiDocTree: tree,
		// 右栏 TOC widget 数据：与文章页一致，从渲染正文提取目录（标题带 id 锚点）
		TOC: stellarweb.ExtractTOCWith(string(contentHTML), s.tocOptions()),
	}
	// 评论区：page_key 为 /wiki/{project}/{slug}（fillComments 统一设置 CommentPageKey）
	s.fillComments(data, "/wiki/"+projectID+"/"+slug)
	s.renderBase(c, data)
}

// handleNotebooks 笔记本列表页（Spec 35b）：卡片网格（封面/名称/描述 + 笔记数），
// 对齐 wiki.html 的 post-list wiki 网格样式。
func (s *webServer) handleNotebooks(c *gin.Context) {
	notebooks, err := s.st.ListNotebooks()
	if err != nil {
		log.Printf("list notebooks: %v", err)
		s.render500(c)
		return
	}
	// Spec 62：笔记数从「每笔记本一次 COUNT」收敛为单条 GROUP BY 查询（CountNotesByNotebooks）。
	ids := make([]int64, 0, len(notebooks))
	for _, nb := range notebooks {
		ids = append(ids, nb.ID)
	}
	counts, err := s.st.CountNotesByNotebooks(ids)
	if err != nil {
		log.Printf("count notes of notebooks: %v", err)
		s.render500(c)
		return
	}
	views := make([]tmpl.NotebookView, 0, len(notebooks))
	for _, nb := range notebooks {
		views = append(views, tmpl.NotebookView{Notebook: *nb, NoteCount: counts[nb.ID]})
	}
	s.renderBase(c, &tmpl.ViewData{
		Site:      s.site.Load(),
		Page:      &tmpl.Page{Title: stellarweb.Lang("notebook.title"), Layout: "notebooks", Lang: "zh-CN"},
		Notebooks: views,
	})
}

// handleNotebook 笔记本详情页（Spec 35b）：笔记列表（updated_at DESC）+ 分页 + ?tag= 过滤。
// leftbar tagtree widget 数据由 TagTreeData 注入（见 assembleWidgets 的 case "tagtree"）。
func (s *webServer) handleNotebook(c *gin.Context) {
	name := c.Param("name")
	nb, err := s.st.GetNotebookByName(name)
	if err != nil {
		log.Printf("get notebook %q: %v", name, err)
		s.render500(c)
		return
	}
	if nb == nil {
		s.render404(c)
		return
	}
	// 标签聚合（tagtree 数据源）；未知 tag 过滤参数与 wiki 标签页一致走 404
	tag := c.Query("tag")
	tagCounts, err := s.st.CountNoteTagsByNotebook(nb.ID)
	if err != nil {
		log.Printf("count note tags of notebook %d: %v", nb.ID, err)
		s.render500(c)
		return
	}
	tags := tagCountItems(tagCounts)
	if tag != "" {
		matched := false
		for _, t := range tags {
			if t.Name == tag {
				matched = true
				break
			}
		}
		if !matched {
			s.render404(c)
			return
		}
	}
	// 分页参数
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	const perPage = 20
	var notes []*store.Note
	var total int
	if tag != "" {
		// Spec 62：tag 过滤下推 SQL（ListNotesByNotebookTag），替代全量取回内存过滤
		notes, _, err = s.st.ListNotesByNotebookTag(nb.ID, tag, -1, 0)
		if err != nil {
			log.Printf("list notes of notebook %d: %v", nb.ID, err)
			s.render500(c)
			return
		}
		total = len(notes)
	} else {
		// 先取总数（LIMIT 0 仅 COUNT）再 clamp 页，对齐 handleAuthor 语义
		_, total, err = s.st.ListNotesByNotebook(nb.ID, 0, 1)
		if err != nil {
			log.Printf("count notes of notebook %d: %v", nb.ID, err)
			s.render500(c)
			return
		}
	}
	totalPages := (total + perPage - 1) / perPage
	if totalPages > 0 && page > totalPages {
		page = totalPages
	}
	if tag != "" {
		// 内存分页切片
		start := (page - 1) * perPage
		if start > len(notes) {
			start = len(notes)
		}
		end := start + perPage
		if end > len(notes) {
			end = len(notes)
		}
		notes = notes[start:end]
	} else {
		notes, _, err = s.st.ListNotesByNotebook(nb.ID, perPage, (page-1)*perPage)
		if err != nil {
			log.Printf("list notes of notebook %d: %v", nb.ID, err)
			s.render500(c)
			return
		}
	}
	pages := make([]int, totalPages)
	for i := range pages {
		pages[i] = i + 1
	}
	s.renderBase(c, &tmpl.ViewData{
		Site:           s.site.Load(),
		Page:           &tmpl.Page{Title: nb.Name, Layout: "notebook", Lang: "zh-CN", Tag: tag},
		NotebookDetail: nb,
		Notes:          noteValues(notes, s.site.Load().Notebook.AutoExcerpt),
		NotesTotal:     total,
		NotesPage:      page,
		NotesPerPage:   perPage,
		Paginator:      &tmpl.Paginator{Current: page, Total: totalPages, Pages: pages},
		NotebookTags:   tags,
		TagTreeData:    &tmpl.TagTreeWidgetData{Tags: tags, ActiveTag: tag, NotebookName: nb.Name},
	})
}

// handleNote 笔记详情页（Spec 35b）：面包屑（首页/笔记本/{name}/{title}）+ 正文 + 上下篇。
// 上下篇为同笔记本内按 updated_at DESC 相邻的笔记；正文优先已渲染 ContentHTML，为空时渲染 Markdown。
func (s *webServer) handleNote(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		s.render404(c)
		return
	}
	note, err := s.st.GetNote(id)
	if err != nil {
		log.Printf("get note %d: %v", id, err)
		s.render500(c)
		return
	}
	if note == nil {
		s.render404(c)
		return
	}
	nb, err := s.st.GetNotebook(note.NotebookID)
	if err != nil {
		log.Printf("get notebook %d: %v", note.NotebookID, err)
		s.render500(c)
		return
	}
	if nb == nil {
		s.render404(c)
		return
	}
	// 正文：优先 ContentHTML，为空时渲染 Markdown 原文
	contentHTML := template.HTML(note.ContentHTML)
	if contentHTML == "" && note.Content != "" {
		if rendered, err := s.posts.Render(note.Content); err == nil {
			contentHTML = template.HTML(rendered)
		}
	}
	// 上下篇：同笔记本内按 updated_at DESC 相邻（Spec 62：两条 LIMIT 1 邻居查询
	// GetPrevNote/GetNextNote，替代全量取回内存定位）
	var prev, next *tmpl.NoteNav
	if pn, err := s.st.GetPrevNote(nb.ID, note.ID); err == nil && pn != nil {
		prev = &tmpl.NoteNav{ID: pn.ID, Title: pn.Title}
	} else if err != nil {
		log.Printf("prev note of %d: %v", note.ID, err)
	}
	if nn, err := s.st.GetNextNote(nb.ID, note.ID); err == nil && nn != nil {
		next = &tmpl.NoteNav{ID: nn.ID, Title: nn.Title}
	} else if err != nil {
		log.Printf("next note of %d: %v", note.ID, err)
	}
	// tagtree 数据源：所属笔记本标签聚合
	tagCounts, err := s.st.CountNoteTagsByNotebook(nb.ID)
	if err != nil {
		log.Printf("count note tags of notebook %d: %v", nb.ID, err)
		s.render500(c)
		return
	}
	tags := tagCountItems(tagCounts)
	s.renderBase(c, &tmpl.ViewData{
		Site: s.site.Load(),
		Page: &tmpl.Page{
			Title:       note.Title,
			Layout:      "note",
			Lang:        "zh-CN",
			Description: textutil.Truncate(textutil.StripHTML(string(contentHTML)), 150),
		},
		NoteDetail: &tmpl.NoteView{
			Note:          *note,
			ContentHTML:   contentHTML,
			NotebookName:  nb.Name,
			NotebookTitle: nb.Name,
			PrevNote:      prev,
			NextNote:      next,
		},
		NotebookTags: tags,
		TagTreeData:  &tmpl.TagTreeWidgetData{Tags: tags, NotebookName: nb.Name},
	})
}

// noteValues store.Note 指针列表 → 值列表（ViewData.Notes 用，模板直接 range）。
func noteValues(notes []*store.Note, autoExcerpt int) []store.Note {
	out := make([]store.Note, 0, len(notes))
	for _, n := range notes {
		v := *n
		if autoExcerpt > 0 {
			content := v.ContentHTML
			if content == "" {
				content = v.Content
			}
			v.Excerpt = textutil.Truncate(textutil.StripHTML(content), autoExcerpt)
		}
		out = append(out, v)
	}
	return out
}

// tagCountItems 标签计数 map → 排序的 TagCount 列表（数量降序，同量按名升序）。
func tagCountItems(counts map[string]int) []tmpl.TagCount {
	items := make([]tmpl.TagCount, 0, len(counts))
	for name, n := range counts {
		items = append(items, tmpl.TagCount{Name: name, Count: n})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return items[i].Name < items[j].Name
	})
	return items
}

// wikiProjectByID 按 ID 查 site.yml wiki 段项目（配置驱动，未配置返回 nil）。
func (s *webServer) wikiProjectByID(id string) *tmpl.WikiProject {
	return wikiProjectByIDOf(s.site.Load(), id)

}

func wikiProjectByIDOf(site *tmpl.Site, id string) *tmpl.WikiProject {
	for i := range site.Wiki {
		if site.Wiki[i].ID == id {
			return &site.Wiki[i]
		}
	}
	return nil
}

// hasTag 判断标签列表是否包含指定标签（精确匹配）。
func hasTag(tags []string, name string) bool {
	for _, t := range tags {
		if t == name {
			return true
		}
	}
	return false
}

// hasCommonTag 判断两个标签列表是否有交集（wiki_related 同 tag 推荐过滤用）。
func hasCommonTag(a, b []string) bool {
	seen := map[string]bool{}
	for _, t := range a {
		seen[t] = true
	}
	for _, t := range b {
		if seen[t] {
			return true
		}
	}
	return false
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

// wikiDocTree 从已加载的 wiki 文档构建模板树，供真实文档页与预览载体共用。
func wikiDocTree(docs []*store.WikiDoc) []tmpl.WikiTreeNode {
	return wikiTreeNodes(store.BuildDocTreeFromDocs(docs))
}

// handleAuthor 作者文章列表页（Spec 38 多作者体系）。
// 对齐 Stellar author.ejs：检查作者存在 → 分页查询文章 → 构建 ViewData → 渲染。
func (s *webServer) handleAuthor(c *gin.Context) {
	id := c.Param("id")
	// 检查作者是否存在
	author, ok := s.site.Load().Authors[id]
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
		Site: s.site.Load(),
		Page: &tmpl.Page{
			Title:  stellarweb.Lang("author.page_title", author.Name),
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
