package routes

import (
	"encoding/json"
	"html/template"
	"sort"

	"blog/internal/config"
	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"
)

// renderSitePreview 用草稿配置渲染前台页面（设置页「前端预览」所见即所得）：
// 浅拷贝全局 site 后应用草稿分段（config.ApplyDraftSections，与 RebuildSite 同一映射），
// 再用真实内容数据渲染 layout/base。Console 按当前编辑的设置项提取对应 DOM 区域聚焦展示。
// page 支持 index/categories/tags/post/wiki；不写 DB、不影响全局共享 site。
func (s *webServer) renderSitePreview(sections map[string]json.RawMessage, page string) (string, error) {
	draft := *s.site.Load()
	config.ApplyDraftSections(&draft, sections)

	data := &tmpl.ViewData{Site: &draft}
	switch page {
	case "categories":
		tree, err := s.st.CategoryTree()
		if err != nil {
			return "", err
		}
		data.Page = &tmpl.Page{Title: "分类", Layout: "categories", MenuID: "post", Lang: "zh-CN"}
		data.Cats = categoryItems(tree, draft.Article.CategoryIcon)
	case "tags":
		counts, err := s.st.CountByTag()
		if err != nil {
			return "", err
		}
		items := make([]*tmpl.TagItem, 0, len(counts))
		for name, n := range counts {
			items = append(items, &tmpl.TagItem{Name: name, Count: n, Icon: draft.Article.TagIcon[name]})
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].Count != items[j].Count {
				return items[i].Count > items[j].Count
			}
			return items[i].Name < items[j].Name
		})
		data.Page = &tmpl.Page{Title: "标签", Layout: "tags", MenuID: "post", Lang: "zh-CN"}
		data.TagItems = items
	case "post":
		// 首篇已发布文章（评论/文章页类设置的预览载体）；无文章时降级首页
		posts, err := s.st.ListPublishedPaged(1, 1)
		if err != nil {
			return "", err
		}
		if len(posts) == 0 {
			return s.renderPreviewIndex(&draft, data)
		}
		p := posts[0]
		data.Page = &tmpl.Page{
			Title:   p.Title,
			Slug:    p.Slug,
			Layout:  "post",
			Type:    "tech",
			Lang:    "zh-CN",
			MenuID:  "post",
			Author:  p.Author,
			Date:    p.CreatedAt.Format("2006-01-02"),
			Updated: p.UpdatedAt.Format("2006-01-02"),
			Excerpt: articleExcerpt(&draft, p.Excerpt, p.ContentHTML),
			Topic:   p.Topic,
			Banner:  p.Banner,
		}
		data.Page.AuthorLinks = resolveAuthorLinks(&draft, p)
		if len(data.Page.AuthorLinks) > 0 {
			data.Page.Author = data.Page.AuthorLinks[0].ID
			data.Page.AuthorName = data.Page.AuthorLinks[0].Name
		}
		data.Content = template.HTML(p.ContentHTML)
		data.TOC = stellarweb.ExtractTOCWith(p.ContentHTML, s.tocOptionsFrom(&draft))
		s.fillComments(data, "/post/"+p.Slug)
	case "wiki":
		// 文库页：项目列表直接取草稿 wiki 段（预览文库配置）
		var tags []string
		seen := map[string]bool{}
		for i := range draft.Wiki {
			for _, t := range draft.Wiki[i].Tags {
				if !seen[t] {
					seen[t] = true
					tags = append(tags, t)
				}
			}
		}
		projects := make([]*tmpl.WikiProject, 0, len(draft.Wiki))
		for i := range draft.Wiki {
			projects = append(projects, &draft.Wiki[i])
		}
		sort.SliceStable(projects, func(i, j int) bool {
			if projects[i].Sort != projects[j].Sort {
				return projects[i].Sort > projects[j].Sort
			}
			return projects[i].ID < projects[j].ID
		})
		data.Page = &tmpl.Page{Title: "文库", Layout: "wiki", MenuID: "wiki", Lang: "zh-CN"}
		data.WikiProjects = projects
		data.WikiTags = tags
		if len(projects) > 0 {
			project := projects[0]
			docs, err := s.st.ListWikidocs(project.ID)
			if err != nil {
				return "", err
			}
			data.WikiProjectID = project.ID
			data.WikiProjectName = project.DisplayTitle()
			data.WikiDocTree = wikiDocTree(docs)
		}
	default: // index 首页
		return s.renderPreviewIndex(&draft, data)
	}
	return s.finishPreview(&draft, data)
}

// renderPreviewIndex 首页预览数据装配（index 与无文章时 post 降级共用）。
func (s *webServer) renderPreviewIndex(draft *tmpl.Site, data *tmpl.ViewData) (string, error) {
	posts, err := s.st.ListPublishedPaged(1, 10)
	if err != nil {
		return "", err
	}
	data.Page = &tmpl.Page{Title: draft.Title, Layout: "index", MenuID: "post", Lang: "zh-CN", Type: draft.Article.Type}
	data.Posts = s.toCardsWith(draft, posts)
	return s.finishPreview(draft, data)
}

// finishPreview 预览渲染收尾：描述兜底 + 侧栏装配 + layout/base 渲染。
func (s *webServer) finishPreview(draft *tmpl.Site, data *tmpl.ViewData) (string, error) {
	// 预览不经 HTTP 请求上下文：描述直接取草稿值（fillSEO 依赖 gin.Context，此处手工兜底）
	data.Page.Description = draft.Description
	s.fillPinSlider(data)
	s.fillSidebar(data)
	return s.engine.Render("layout/base", data)
}
