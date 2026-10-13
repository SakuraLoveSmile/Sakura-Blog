package routes

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// registerTaxonomyRoutes 注册聚合/索引域路由：友链、文库、手记、归档、分类、专栏、标签。
func (s *webServer) registerTaxonomyRoutes(r *gin.Engine) {
	r.GET("/links", s.handleLinksPage)
	r.GET("/friends", s.handleFriendsPage)
	r.GET("/notes", func(c *gin.Context) {
		p, err := s.st.GetPublished(NotesSlug)
		if err != nil {
			stellarError(c, err)
			return
		}
		s.serveStellarPage(c, p)
	})
	r.GET("/wiki", s.handleWiki)
	r.GET("/wiki/", s.handleWiki)
	r.GET("/wiki/tags/:tag", s.handleWiki)
	r.GET("/wiki/:project", s.handleWikiProject)
	r.GET("/wiki/:project/:slug", s.handleWikiDoc)
	r.GET("/topic", s.handleTopicIndex)
	r.GET("/topics", s.handleTopicIndex)
	r.GET("/topic/:slug", s.handleTopicDetail)
	r.GET("/archives", func(c *gin.Context) { s.serveStellarTaxonomy(c, "archives", "") })
	r.GET("/categories", func(c *gin.Context) { s.serveStellarTaxonomy(c, "categories", "") })
	r.GET("/categories/:name", func(c *gin.Context) { s.handleCategoryPath(c, c.Param("name")) })
	r.GET("/tags", func(c *gin.Context) { s.serveStellarTaxonomy(c, "tags", "") })
	r.GET("/tags/:name", func(c *gin.Context) { s.serveStellarTaxonomy(c, "tags", c.Param("name")) })
	r.GET("/author/:id", s.handleAuthor)
	r.GET("/notebooks", func(c *gin.Context) { s.serveStellarCollectionIndex(c, "notebook", "") })
	r.GET("/notebooks/:notebook", func(c *gin.Context) { s.serveStellarCollection(c, "notebook", c.Param("notebook"), 1, nil) })
	r.GET("/notebooks/:notebook/:slug", func(c *gin.Context) { s.serveStellarNote(c, c.Param("notebook"), c.Param("slug")) })
}

// handleCategoryPath 是所有分类详情入口的统一路径解析逻辑。
// NoRoute 会把包含多个路径段的 /categories/a/b/ 也转交到此方法。
func (s *webServer) handleCategoryPath(c *gin.Context, path string) {
	s.serveStellarTaxonomy(c, "categories", strings.Trim(path, "/"))
}
