package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"blog/internal/post"
)

// ShortcodeCatalogResponse is the stable envelope for the backend-owned tag catalog.
type ShortcodeCatalogResponse struct {
	Items []post.ShortcodeCatalogEntry `json:"items"`
}

// shortcodeCatalog GET /api/v1/plugins/catalog returns the parser-owned shortcode schema.
func (h *Handler) shortcodeCatalog(c *gin.Context) {
	c.JSON(http.StatusOK, ShortcodeCatalogResponse{Items: append(post.ShortcodeCatalogStellarV2(), post.ShortcodeCatalogBlogExtensions()...)})
}
