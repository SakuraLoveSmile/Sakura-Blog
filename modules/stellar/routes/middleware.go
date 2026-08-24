package routes

import (
	"fmt"
	"hash/fnv"
	"log"
	"net/http"
	"strings"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"

	"blog/internal/config"
	"blog/internal/httpapi"
	"blog/internal/store"
)

// securityHeaders P2-14 安全响应头：nosniff 防 MIME 嗅探、SAMEORIGIN 防点击劫持、Referrer-Policy 控制来源泄漏。
// 不加全局 CSP：前台存在内联脚本/样式（原版 Stellar 结构），严格 CSP 会破坏渲染；/uploads 的 SVG 已单独 CSP 防护（P1-11）。
func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// P3-1: Console SPA 防缓存——强制浏览器每次重新加载，确保修改即时生效
		if len(c.Request.URL.Path) >= 8 && c.Request.URL.Path[:8] == "/console" {
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		}
		c.Next()
	}
}

// assetsCacheControl P1-7 /assets 长缓存：带 ?v= 版本号的 css/js/字体改 immutable 长缓存，破缓存靠 ?v= bump。
// sw.js/manifest 不在 /assets/ 路径下，不受影响。
func assetsCacheControl() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/assets/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		}
		c.Next()
	}
}

// installMiddleware 装配全局中间件链（顺序敏感：recovery → 代理/上传上限 → gzip → 响应头 → 埋点）。
func (s *webServer) installMiddleware(r *gin.Engine, app *config.App, pv *pageViewBuffer) {
	// P1-8：自定义 500——Logger + CustomRecovery，panic 时渲染友好错误页而非裸文本。
	r.Use(gin.Logger())
	r.Use(gin.CustomRecovery(func(c *gin.Context, err interface{}) {
		log.Printf("panic recovered: %v", err)
		if c.Writer.Written() {
			c.Abort()
			return
		}
		// API 路由 panic → 统一 JSON Problem（前台仍走 render500 页，现状不变）
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			const msg = "服务器内部错误，请稍后重试"
			c.AbortWithStatusJSON(http.StatusInternalServerError, httpapi.Problem{
				Type:     "internal_error",
				Title:    http.StatusText(http.StatusInternalServerError),
				Status:   http.StatusInternalServerError,
				Detail:   msg,
				Instance: c.Request.URL.Path,
				Error:    msg,
			})
			return
		}
		s.render500(c)
		c.Abort()
	}))

	// 反向代理头：设了 trusted_proxy 才信任 X-Forwarded-For，还原客户端真实 IP。
	if app.TrustedProxy != "" {
		proxies := strings.Split(app.TrustedProxy, ",")
		for i := range proxies {
			proxies[i] = strings.TrimSpace(proxies[i])
		}
		if err := r.SetTrustedProxies(proxies); err != nil {
			log.Printf("set trusted proxies: %v", err)
		}
		r.ForwardedByClientIP = true
	} else {
		// 安全审计 2026-08-16：未配置 trusted_proxy 时显式清空信任列表——gin 默认
		// 信任 0.0.0.0/0+::/0，X-Forwarded-For 可伪造 ClientIP，绕过登录/评论/
		// guest AI 的全部 per-IP 限流。清空后仅认 RemoteAddr（直连场景正确语义）。
		if err := r.SetTrustedProxies(nil); err != nil {
			log.Printf("clear trusted proxies: %v", err)
		}
	}

	// P2-18：multipart 内存上限——跟随 max_upload_mb，防止超大表单解析撑爆内存（超出部分落临时文件）。
	r.MaxMultipartMemory = int64(app.MaxUploadMB+1) << 20

	// P1-5：gzip 压缩——对 HTML/CSS/JS/JSON/XML 等文本响应启用 gzip，注册在静态与路由之前。
	// Console 已自行协商并返回构建期 .gz；排除 /console，避免全局中间件再次包装
	// 预压缩字节并清掉 Content-Encoding，导致浏览器把 gzip 数据当作 HTML/JS 读取。
	r.Use(gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedPathsRegexs([]string{`^/console(?:/|$)`})))

	r.Use(securityHeaders())
	r.Use(assetsCacheControl())

	// 访问埋点中间件（Spec 24）：仅记录内容页 GET，排除静态/API/后台/机器人。
	r.Use(trackPageView(pv))
}

// --- 访问埋点（Spec 24） ---

// botUASubstrings 常见爬虫/机器人 UA 关键词（小写匹配）。
var botUASubstrings = []string{
	"bot", "crawl", "spider", "slurp", "mediapartners",
	"curl", "wget", "python-requests", "go-http-client",
	"java/", "scrapy", "httpclient", "feedfetcher",
}

// trackPageView 返回 Gin 中间件：仅记录内容页 GET 请求的访问明细。
// 白名单制：只记 /、/post/*、/notes、/friends、/moments、/archives 等内容路径。
// Spec 61 阶段 5：埋点改走 pageViewBuffer 异步批量（不再同步写库串行化读）。
func trackPageView(pv *pageViewBuffer) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}
		p := c.Request.URL.Path
		// 排除非内容路径
		if strings.HasPrefix(p, "/assets") || strings.HasPrefix(p, "/api") ||
			strings.HasPrefix(p, "/console") || strings.HasPrefix(p, "/sw.js") ||
			p == "/manifest.webmanifest" || p == "/favicon.svg" ||
			p == "/search.json" || p == "/atom.xml" || p == "/rss.xml" ||
			p == "/robots.txt" {
			c.Next()
			return
		}
		// 白名单：仅记录内容页
		isContent := p == "/" || p == "/notes" || p == "/friends" ||
			p == "/moments" || p == "/archives" ||
			strings.HasPrefix(p, "/post/") || strings.HasPrefix(p, "/page/") ||
			strings.HasPrefix(p, "/category/") || strings.HasPrefix(p, "/tags/")
		if !isContent {
			c.Next()
			return
		}
		// flying-pages 预取请求不计数（Spec 36）
		if c.GetHeader("X-Preload") != "" {
			c.Next()
			return
		}
		// 过滤机器人 UA
		ua := strings.ToLower(c.Request.UserAgent())
		for _, kw := range botUASubstrings {
			if strings.Contains(ua, kw) {
				c.Next()
				return
			}
		}
		// ua_hash = fnv32(UA + IP)，不存明文，隐私友好
		h := fnv.New32a()
		h.Write([]byte(c.Request.UserAgent() + c.ClientIP()))
		uaHash := fmt.Sprintf("%x", h.Sum32())

		referrer := c.Request.Referer()
		pv.push(store.PageView{Path: p, Referrer: referrer, UAHash: uaHash})
		c.Next()
	}
}
