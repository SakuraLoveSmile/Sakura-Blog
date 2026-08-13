package main

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/frontend/assets"
)

// serveAsset 返回单文件资源 handler：-dev 从磁盘读，否则走 embed FS。
// PWA 根作用域路由用（SW 必须从 /sw.js 提供才能控制全站；manifest/图标映射到 embed 或磁盘）。
func (s *webServer) serveAsset(assetPath, contentType string, extraHeaders map[string]string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var data []byte
		var err error
		if s.devMode {
			data, err = os.ReadFile(filepath.Join(s.root, "frontend", "assets", assetPath))
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
		r.Static("/assets", filepath.Join(s.root, "frontend", "assets"))
	} else {
		sub, err := fs.Sub(assets.FS, ".")
		if err != nil {
			return err
		}
		r.StaticFS("/assets", http.FS(sub))
	}

	r.GET("/sw.js", s.serveSW())
	r.GET("/manifest.webmanifest", s.serveManifest())
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

// serveSW 提供 Service Worker：缓存版本号由 settings site 段 sw_cache_version 注入（缺省 blog-v3），
// 版本变更后旧 SW 缓存自动失效（占位符替换见 sw.js 源文件）。
func (s *webServer) serveSW() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Spec 61 阶段 5：模板启动时已读入（swTemplate），此处仅做版本占位符替换
		if len(s.swTemplate) == 0 {
			c.Status(http.StatusNotFound)
			return
		}
		ver := s.site.Load().SWCacheVersion
		if ver == "" {
			ver = "blog-v3"
		}
		body := strings.ReplaceAll(string(s.swTemplate), "__BLOG_SW_VERSION__", ver)
		c.Header("Service-Worker-Allowed", "/")
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "application/javascript", []byte(body))
	}
}

// serveManifest 动态生成 PWA manifest：名称/描述取 settings site 段，主题色取 style 段 accent
// （缺省 #1c1f21 深色）；响应 no-cache 保证站点信息修改后 PWA 安装名即时同步。
func (s *webServer) serveManifest() gin.HandlerFunc {
	return func(c *gin.Context) {
		site := s.site.Load()
		name := firstNonEmpty(site.Title, site.LogoTitle, "Stellar Blog")
		shortName := firstNonEmpty(site.LogoTitle, site.Title, "Stellar")
		theme := firstNonEmpty(site.Style.Color.Accent, "#1c1f21")
		m := map[string]any{
			"name":             name,
			"short_name":       shortName,
			"description":      site.Description,
			"start_url":        "/",
			"display":          "standalone",
			"theme_color":      theme,
			"background_color": "#1c1f21",
			"icons": []map[string]string{
				{"src": "/assets/img/icon-192.png", "sizes": "192x192", "type": "image/png", "purpose": "any"},
				{"src": "/assets/img/icon-512.png", "sizes": "512x512", "type": "image/png", "purpose": "any"},
				{"src": "/assets/img/icon-512.png", "sizes": "512x512", "type": "image/png", "purpose": "maskable"},
			},
		}
		b, err := json.Marshal(m)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "application/manifest+json", b)
	}
}

// firstNonEmpty 返回第一个非空字符串，全部为空时返回空串。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
