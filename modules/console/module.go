// Package console mounts the management REST API and Vue SPA.
package console

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	ghpkg "blog/internal/github"
	"blog/modules/console/api"
)

// APIConfig is the Console-facing shared service configuration. The concrete
// API package remains behind this top-level module boundary.
type APIConfig = api.Config

// MountDeps keeps Console-specific API dependencies together while allowing the
// command package to provide the Stellar 404/page fallback callback.
type MountDeps struct {
	API      APIConfig
	GH       *ghpkg.Client
	DevMode  bool
	Root     string
	Fallback gin.HandlerFunc
}

// Mount installs Console-owned /api/v1 routes and /console/*.
func Mount(r *gin.Engine, d MountDeps) (*api.Handler, error) {
	// Public reader, rating, vote, friend and GitHub endpoints belong to
	// Stellar. The Console module mounts only its /api/v1 and /console surface.
	h, err := api.MountConsole(r, api.MountDeps{Config: d.API, GH: d.GH})
	if err != nil {
		return nil, err
	}
	consoleIndex, err := setupConsoleRoutes(r, d.DevMode, d.Root)
	if err != nil {
		return nil, err
	}
	r.NoRoute(func(c *gin.Context) {
		if consoleIndex != nil && strings.HasPrefix(c.Request.URL.Path, "/console") {
			writeConsoleData(c, "text/html; charset=utf-8", consoleIndex.plain, consoleIndex.gzip)
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
