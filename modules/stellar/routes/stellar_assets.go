package routes

import (
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"

	stellarweb "blog/modules/stellar/web"

	"github.com/gin-gonic/gin"
)

func (s *webServer) stellarSiteRoot() string {
	if s.site == nil || s.site.Load() == nil {
		return ""
	}
	base, err := url.Parse(s.site.Load().URL)
	if err != nil {
		return ""
	}
	return strings.TrimRight(base.Path, "/")
}
func (s *webServer) serveGeneratedStellarAsset(c *gin.Context) bool {
	requested := c.Request.URL.Path
	root := s.stellarSiteRoot()
	if s.icons == nil {
		return false
	}
	assets, err := stellarweb.GeneratedStellarAssets(s.icons)
	if err != nil {
		return false
	}
	for _, asset := range assets {
		matches := []string{"/" + asset.Path, "/assets/" + asset.Path, root + "/" + asset.Path, root + "/assets/" + asset.Path}
		for _, candidate := range matches {
			if requested != candidate {
				continue
			}
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("X-Content-Type-Options", "nosniff")
			c.Header("Cache-Control", "public, max-age=86400")
			c.Data(http.StatusOK, asset.ContentType, asset.Body)
			return true
		}
	}
	return false
}
func (s *webServer) serveSiteRootAsset(c *gin.Context) bool {
	root := s.stellarSiteRoot()
	if root == "" {
		return false
	}
	prefix := root + "/assets/"
	if !strings.HasPrefix(c.Request.URL.Path, prefix) {
		return false
	}
	relative := strings.TrimPrefix(c.Request.URL.Path, prefix)
	if !fs.ValidPath(relative) || path.Clean(relative) != relative {
		c.Status(http.StatusNotFound)
		return true
	}
	contentType := mime.TypeByExtension(path.Ext(relative))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Access-Control-Allow-Origin", "*")
	s.serveAsset(relative, contentType, nil)(c)
	return true
}
