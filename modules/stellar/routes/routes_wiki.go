package routes

import (
	tmpl "blog/modules/stellar/viewmodel"
	"html/template"
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
	s.serveStellarCollectionIndex(c, "wiki", c.Param("tag"))
}
func (s *webServer) handleWikiProject(c *gin.Context) { s.serveStellarWiki(c, c.Param("project"), "") }
func (s *webServer) handleWikiDoc(c *gin.Context) {
	s.serveStellarWiki(c, c.Param("project"), c.Param("slug"))
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
