package seo

import (
	"fmt"
	"hash/fnv"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/module"
)

// rangeDays 从 range 查询参数解析天数（today/7d/30d/90d 或纯数字），默认 7。
func rangeDays(c *gin.Context) int {
	switch c.Query("range") {
	case "today":
		return 1
	case "7d", "":
		return 7
	case "30d":
		return 30
	case "90d":
		return 90
	default:
		if n, err := strconv.Atoi(strings.TrimSuffix(c.Query("range"), "d")); err == nil && n > 0 {
			return n
		}
		return 7
	}
}

// registerCrawler 挂爬虫记录相关端点。
func (m *Module) registerCrawler(g *gin.RouterGroup) {
	g.GET("/crawlers/analysis", m.crawlerAnalysisHandler)
	g.GET("/crawlers/logs", m.crawlerLogsHandler)
}

func (m *Module) crawlerAnalysisHandler(c *gin.Context) {
	res, err := m.deps.Store.CrawlerAnalysis(rangeDays(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (m *Module) crawlerLogsHandler(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	logs, total, err := m.deps.Store.ListCrawlerLogs(page, size, c.Query("engine"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": logs, "total": total})
}

// crawlerSkipPrefixes 埋点跳过的非内容路径前缀。
var crawlerSkipPrefixes = []string{"/assets", "/api", "/console", "/uploads", "/og/"}

// crawlerMiddleware 返回中间件：识别搜索引擎爬虫 UA 并落库。
// 受「SEO 模块启用」+「crawler.enabled 开关」双重控制，运行时实时判定。
func (m *Module) crawlerMiddleware(deps module.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}
		// 模块禁用或子开关关闭时不记录
		if deps.Modules == nil || !deps.Modules.Enabled("seo") {
			c.Next()
			return
		}
		if !loadConfig(deps.Store).Crawler.Enabled {
			c.Next()
			return
		}
		p := c.Request.URL.Path
		for _, pre := range crawlerSkipPrefixes {
			if strings.HasPrefix(p, pre) {
				c.Next()
				return
			}
		}
		if p == "/sw.js" || p == "/manifest.webmanifest" || p == "/favicon.ico" ||
			p == "/robots.txt" || p == "/sitemap.xml" || p == "/rss.xml" || p == "/atom.xml" {
			c.Next()
			return
		}
		engine := crawlerEngine(c.Request.UserAgent())
		if engine == "" {
			c.Next()
			return
		}
		// ip_hash = fnv32(IP)，不存明文
		h := fnv.New32a()
		h.Write([]byte(c.ClientIP()))
		_ = deps.Store.RecordCrawler(engine, c.Request.UserAgent(), p, fmt.Sprintf("%x", h.Sum32()))
		c.Next()
	}
}
