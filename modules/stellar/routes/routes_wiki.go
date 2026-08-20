package routes

import (
	"blog/internal/store"
	"blog/internal/textutil"
	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
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

// handleWiki 文库页：全量标签 tab + 可选 tag 过滤 + Sort 降序项目列表。
func (s *webServer) handleWiki(c *gin.Context) {
	tag := c.Param("tag")
	allProjects := s.wikiProjectsForRuntime()
	// 收集全量标签（顶部 tab，保留首见顺序）
	var tags []string
	tagSeen := map[string]bool{}
	for _, project := range allProjects {
		for _, t := range project.Tags {
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
	projects := make([]*tmpl.WikiProject, 0, len(allProjects))
	for _, source := range allProjects {
		w := *source
		// 项目卡片只展示有文档或外部入口（URL/仓库）的项目；
		// 但运行时全量项目仍保留给 wiki_related 等上下文组件。
		if !w.HasDocs && strings.TrimSpace(w.URL) == "" && strings.TrimSpace(w.Repo) == "" {
			continue
		}
		if tag != "" {
			if !wikiProjectHasTag(&w, tag) {
				continue
			}
		}
		projects = append(projects, &w)
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
	// 项目存在性校验（迁移完成后由 wiki_projects 表提供）。
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
		WikiProjectRepo:        project.Repo,
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

// wikiProjectByID 按数据库项目 ID 查项目；未完成迁移的独立测试回退旧配置。
func (s *webServer) wikiProjectByID(id string) *tmpl.WikiProject {
	if projects, err := s.st.ListWikiProjects(); err == nil {
		if marker, markerErr := s.st.GetSetting("wiki_projects_migrated"); markerErr == nil && marker == "1" {
			for _, p := range projects {
				if p.ID == id && p.Visible {
					withCounts := p
					_ = s.st.PopulateWikiProjectCounts(withCounts)
					return wikiProjectTemplate(withCounts)
				}
			}
			return nil
		}
		for _, p := range projects {
			if p.ID == id {
				return wikiProjectTemplate(p)
			}
		}
	}
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
