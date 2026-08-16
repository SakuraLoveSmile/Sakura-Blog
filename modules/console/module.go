// Package console mounts the management REST API and Vue SPA.
package console

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/modules/console/api"
)

// MountDeps keeps Console-specific API dependencies together while allowing the
// command package to provide the Stellar 404/page fallback callback.
type MountDeps struct {
	API      api.MountDeps
	DevMode  bool
	Root     string
	Fallback gin.HandlerFunc
}

// Mount installs /api/v1, public API compatibility routes, and /console/*.
func Mount(r *gin.Engine, d MountDeps) (*api.Handler, error) {
	h, err := api.Mount(r, d.API)
	if err != nil {
		return nil, err
	}
	consoleIndex, err := setupConsoleRoutes(r, d.DevMode, d.Root)
	if err != nil {
		return nil, err
	}
	r.NoRoute(func(c *gin.Context) {
		if consoleIndex != nil && strings.HasPrefix(c.Request.URL.Path, "/console") {
			c.Data(http.StatusOK, "text/html; charset=utf-8", consoleIndex)
			return
		}
		if d.Fallback != nil {
			d.Fallback(c)
			return
		}
		c.Status(http.StatusNotFound)
	})
	return h, nil
}
