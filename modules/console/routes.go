package console

import (
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	console "blog/modules/console/web"
)

type consoleIndexData struct {
	plain []byte
	gzip  []byte
}

// setupConsoleRoutes Vue Console 静态托管（/console/）：默认走 go:embed（单二进制），-dev 从磁盘 modules/console/web/dist 读。
// 静态文件统一按 Accept-Encoding 协商预压缩 .gz；其余 /console 路径由 NoRoute 返回 index.html。
func setupConsoleRoutes(r *gin.Engine, devMode bool, root string) (*consoleIndexData, error) {
	if devMode {
		consoleDir := filepath.Join(root, "modules", "console", "web", "dist")
		info, err := os.Stat(consoleDir)
		if err != nil || !info.IsDir() {
			log.Printf("console dist not found: %s (dev), /console/ unavailable", consoleDir)
			return nil, nil
		}
		registerDiskStatic(r, "/console", consoleDir)
		plain, err := os.ReadFile(filepath.Join(consoleDir, "index.html"))
		if err != nil {
			return nil, err
		}
		compressed, _ := os.ReadFile(filepath.Join(consoleDir, "index.html.gz"))
		log.Printf("console SPA loaded from %s (dev)", consoleDir)
		return &consoleIndexData{plain: plain, gzip: compressed}, nil
	}

	registerEmbedStatic(r, "/console")
	plain, err := console.DistFS.ReadFile("dist/index.html")
	if err != nil {
		return nil, err
	}
	compressed, _ := console.DistFS.ReadFile("dist/index.html.gz")
	log.Printf("console SPA loaded from embed")
	return &consoleIndexData{plain: plain, gzip: compressed}, nil
}

func setupUCRoutes(r *gin.Engine, devMode bool, root string) (*consoleIndexData, error) {
	if devMode {
		ucDir := filepath.Join(root, "modules", "console", "web", "dist-uc")
		info, err := os.Stat(ucDir)
		if err != nil || !info.IsDir() {
			log.Printf("user center dist not found: %s (dev), /uc/ unavailable", ucDir)
			return nil, nil
		}
		registerDiskStatic(r, "/uc", ucDir)
		plain, err := os.ReadFile(filepath.Join(ucDir, "uc.html"))
		if err != nil {
			return nil, err
		}
		compressed, _ := os.ReadFile(filepath.Join(ucDir, "uc.html.gz"))
		log.Printf("user center SPA loaded from %s (dev)", ucDir)
		return &consoleIndexData{plain: plain, gzip: compressed}, nil
	}

	registerEmbedStatic(r, "/uc")
	plain, err := console.UCDistFS.ReadFile("dist-uc/uc.html")
	if err != nil {
		return nil, err
	}
	compressed, _ := console.UCDistFS.ReadFile("dist-uc/uc.html.gz")
	log.Printf("user center SPA loaded from embed")
	return &consoleIndexData{plain: plain, gzip: compressed}, nil
}

// setupPluginRoutes serves only explicitly addressed bundle files from the
// external plugins directory. Bundles are same-origin and never read from an
// arbitrary URL supplied by the browser.
func setupPluginRoutes(r *gin.Engine, root string) {
	pluginRoot := filepath.Join(root, "plugins")
	handler := func(c *gin.Context) {
		pluginID := c.Param("id")
		filePath := strings.TrimPrefix(c.Param("filepath"), "/")
		ext := strings.ToLower(filepath.Ext(filePath))
		if pluginID == "" || filePath == "" || (ext != ".js" && ext != ".mjs" && ext != ".css") || strings.Contains(pluginID, "..") || strings.Contains(filePath, "..") || filepath.IsAbs(filePath) {
			c.Status(http.StatusNotFound)
			return
		}
		fullPath := filepath.Join(pluginRoot, pluginID, filepath.FromSlash(filePath))
		if _, err := os.Stat(fullPath); err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.File(fullPath)
	}
	r.GET("/plugins/:id/*filepath", handler)
	r.HEAD("/plugins/:id/*filepath", handler)
}

func registerDiskStatic(r *gin.Engine, prefix, distRoot string) {
	registerStaticHandler(r, prefix+"/assets/*filepath", func(name string) ([]byte, error) {
		return os.ReadFile(filepath.Join(distRoot, "assets", name))
	}, func(name string) ([]byte, error) {
		return os.ReadFile(filepath.Join(distRoot, "assets", name+".gz"))
	})
	if entries, err := os.ReadDir(distRoot); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || entry.Name() == "index.html" || entry.Name() == ".gitkeep" || strings.HasSuffix(entry.Name(), ".gz") {
				continue
			}
			name := entry.Name()
			registerStaticHandler(r, prefix+"/"+name, func(_ string) ([]byte, error) {
				return os.ReadFile(filepath.Join(distRoot, name))
			}, func(_ string) ([]byte, error) {
				return os.ReadFile(filepath.Join(distRoot, name+".gz"))
			})
		}
	}
}

func registerEmbedStatic(r *gin.Engine, prefix string) {
	fs := console.DistFS
	root := "dist"
	if prefix == "/uc" {
		fs = console.UCDistFS
		root = "dist-uc"
	}
	registerStaticHandler(r, prefix+"/assets/*filepath", func(name string) ([]byte, error) {
		return fs.ReadFile(root + "/assets/" + name)
	}, func(name string) ([]byte, error) {
		return fs.ReadFile(root + "/assets/" + name + ".gz")
	})
	entries, err := fs.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "index.html" || entry.Name() == ".gitkeep" || strings.HasSuffix(entry.Name(), ".gz") {
			continue
		}
		name := entry.Name()
		registerStaticHandler(r, prefix+"/"+name, func(_ string) ([]byte, error) {
			return fs.ReadFile(root + "/" + name)
		}, func(_ string) ([]byte, error) {
			return fs.ReadFile(root + "/" + name + ".gz")
		})
	}
}

type staticReader func(name string) ([]byte, error)

func registerStaticHandler(r *gin.Engine, route string, plain staticReader, compressed staticReader) {
	immutable := strings.Contains(route, "/assets/")
	handler := func(c *gin.Context) {
		name := strings.TrimPrefix(c.Param("filepath"), "/")
		if name == "" {
			name = filepath.Base(route)
		}
		if strings.Contains(name, "..") || filepath.IsAbs(name) {
			c.Status(http.StatusNotFound)
			return
		}
		data, err := plain(name)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		if immutable {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		}
		gzipData, _ := compressed(name)
		writeConsoleData(c, consoleContentType(name), data, gzipData)
	}
	r.GET(route, handler)
	r.HEAD(route, handler)
}

func writeConsoleData(c *gin.Context, contentType string, plain, compressed []byte) {
	if c.Writer.Header().Get("Cache-Control") == "" {
		c.Header("Cache-Control", "no-cache")
	}
	if len(compressed) > 0 && acceptsGzip(c.GetHeader("Accept-Encoding")) {
		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")
		c.Header("Content-Length", strconv.Itoa(len(compressed)))
		c.Data(http.StatusOK, contentType, compressed)
		return
	}
	c.Header("Vary", "Accept-Encoding")
	c.Header("Content-Length", strconv.Itoa(len(plain)))
	c.Data(http.StatusOK, contentType, plain)
}

func acceptsGzip(header string) bool {
	for _, raw := range strings.Split(header, ",") {
		parts := strings.Split(raw, ";")
		encoding := strings.TrimSpace(strings.ToLower(parts[0]))
		if encoding != "gzip" && encoding != "*" {
			continue
		}
		quality := 1.0
		for _, parameter := range parts[1:] {
			keyValue := strings.SplitN(strings.TrimSpace(parameter), "=", 2)
			if len(keyValue) == 2 && strings.EqualFold(keyValue[0], "q") {
				if value, err := strconv.ParseFloat(keyValue[1], 64); err == nil {
					quality = value
				}
			}
		}
		return quality > 0
	}
	return false
}

// consoleContentType 按扩展名推断 console 根级静态文件的 Content-Type。
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
