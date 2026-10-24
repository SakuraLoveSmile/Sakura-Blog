package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"blog/internal/config"
)

// StellarAuthorCatalogItem 仅包含供编辑器选择的公开作者资料。
type StellarAuthorCatalogItem struct {
	Name        string `json:"name"`
	Avatar      string `json:"avatar,omitempty"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
	Hidden      bool   `json:"hidden,omitempty"`
}

type StellarAuthorCatalogResponse struct {
	Authors map[string]StellarAuthorCatalogItem `json:"authors"`
}

// getStellarAuthorCatalog 读取严格配置快照，不读旧分段或运行时 Site.Authors。
func (h *Handler) getStellarAuthorCatalog(c *gin.Context) {
	loaded, err := config.LoadStellarV2(h.store)
	if err != nil {
		stellarSettingsError(c, err, http.StatusInternalServerError)
		return
	}
	runtime, err := config.ParseStellarBlogRuntime(loaded.Blog)
	if err != nil {
		stellarSettingsError(c, err, http.StatusInternalServerError)
		return
	}
	response := StellarAuthorCatalogResponse{Authors: map[string]StellarAuthorCatalogItem{}}
	for id, author := range runtime.Authors {
		response.Authors[id] = StellarAuthorCatalogItem{Name: author.Name, Avatar: author.Avatar, Description: author.Description, URL: author.URL, Hidden: author.Hidden}
	}
	c.JSON(http.StatusOK, response)
}
