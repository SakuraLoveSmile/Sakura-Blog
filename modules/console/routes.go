package console

import (
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	console "blog/modules/console/web"
)

// setupConsoleRoutes Vue Console SPA 静态托管（/console/）：默认走 go:embed（单二进制），-dev 从磁盘 modules/console/web/dist 读。
// 只注册 /console/assets/* 与根级静态文件；其余 /console 路径（含 /console/ 本身与 SPA 前端路由）
// 由调用方在 NoRoute 返回 index.html，保证前端路由刷新不 404。
// 返回 index.html 内容（nil 表示 console 不可用，dev 模式缺 dist 不报错）。
func setupConsoleRoutes(r *gin.Engine, devMode bool, root string) ([]byte, error) {
	if devMode {
		consoleDir := filepath.Join(root, "modules", "console", "web", "dist")
		info, err := os.Stat(consoleDir)
		if err != nil || !info.IsDir() {
			log.Printf("console dist not found: %s (dev), /console/ unavailable", consoleDir)
			return nil, nil
		}
		r.Static("/console/assets", filepath.Join(consoleDir, "assets"))
		// 根级静态文件（favicon.svg 等）
		if entries, err := os.ReadDir(consoleDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && e.Name() != "index.html" {
					name := e.Name()
					r.StaticFile("/console/"+name, filepath.Join(consoleDir, name))
				}
			}
		}
		consoleIndex, _ := os.ReadFile(filepath.Join(consoleDir, "index.html"))
		log.Printf("console SPA loaded from %s (dev)", consoleDir)
		return consoleIndex, nil
	}

	// embed 模式：console.DistFS（all:dist）
	assetsSub, err := fs.Sub(console.DistFS, "dist/assets")
	if err != nil {
		return nil, err
	}
	r.StaticFS("/console/assets", http.FS(assetsSub))
	// 根级静态文件（favicon.svg / icons.svg 等）
	entries, err := console.DistFS.ReadDir("dist")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || e.Name() == "index.html" {
			continue
		}
		name := e.Name()
		r.GET("/console/"+name, func(c *gin.Context) {
			data, err := console.DistFS.ReadFile("dist/" + name)
			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}
			c.Data(http.StatusOK, consoleContentType(name), data)
		})
	}
	consoleIndex, err := console.DistFS.ReadFile("dist/index.html")
	if err != nil {
		return nil, err
	}
	log.Printf("console SPA loaded from embed")
	return consoleIndex, nil
}

// consoleContentType 按扩展名推断 console 根级静态文件的 Content-Type。
// 常见类型显式返回（不依赖运行环境 mime 数据库，alpine 精简镜像也能正确），
// 其余 fallback 到 mime.TypeByExtension，再不行用 octet-stream。
func consoleContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".svg":
		return "image/svg+xml"
	case ".js":
		return "application/javascript"
	case ".css":
		return "text/css; charset=utf-8"
	case ".png":
		return "image/png"
	case ".ico":
		return "image/x-icon"
	case ".html":
		return "text/html; charset=utf-8"
	default:
		if ct := mime.TypeByExtension(filepath.Ext(name)); ct != "" {
			return ct
		}
		return "application/octet-stream"
	}
}
