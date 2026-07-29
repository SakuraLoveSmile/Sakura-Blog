package main

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/assets"
)

// serveAsset 返回单文件资源 handler：-dev 从磁盘读，否则走 embed FS。
// PWA 根作用域路由用（SW 必须从 /sw.js 提供才能控制全站；manifest/图标映射到 embed 或磁盘）。
func (s *webServer) serveAsset(assetPath, contentType string, extraHeaders map[string]string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var data []byte
		var err error
		if s.devMode {
			data, err = os.ReadFile(filepath.Join(s.root, "assets", assetPath))
		} else {
			data, err = assets.FS.ReadFile(assetPath)
		}
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		if contentType != "" {
			c.Header("Content-Type", contentType)
		}
		for k, v := range extraHeaders {
			c.Header(k, v)
		}
		c.Data(http.StatusOK, contentType, data)
	}
}

// registerStaticRoutes 注册静态资源域路由：/assets、PWA 根作用域文件、/uploads。
func (s *webServer) registerStaticRoutes(r *gin.Engine) error {
	// 静态文件 /assets：-dev 从磁盘读（改 CSS 即时生效），否则走 embed.FS（css/js/fonts）。
	if s.devMode {
		r.Static("/assets", filepath.Join(s.root, "assets"))
	} else {
		sub, err := fs.Sub(assets.FS, ".")
		if err != nil {
			return err
		}
		r.StaticFS("/assets", http.FS(sub))
	}

	r.GET("/sw.js", s.serveAsset("js/sw.js", "application/javascript", map[string]string{
		"Service-Worker-Allowed": "/",
		"Cache-Control":          "no-cache",
	}))
	r.GET("/manifest.webmanifest", s.serveAsset("manifest.webmanifest", "application/manifest+json", nil))
	r.GET("/favicon.ico", s.serveAsset("img/favicon.ico", "image/x-icon", nil))
	r.GET("/apple-touch-icon.png", s.serveAsset("img/apple-touch-icon.png", "image/png", nil))

	// 上传附件静态托管（运行时数据，不走 go:embed）
	// P1-11：SVG 防护——对 .svg 响应加 CSP sandbox，禁止内嵌脚本执行。
	r.GET("/uploads/*filepath", func(c *gin.Context) {
		fp := c.Param("filepath")
		if strings.HasSuffix(strings.ToLower(fp), ".svg") {
			c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
		}
		c.File(filepath.Join(s.uploadDir, filepath.Clean(fp)))
	})
	return nil
}
