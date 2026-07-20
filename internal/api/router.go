// Package api 提供 RESTful JSON API（/api/v1），供 Vue Console SPA 管理后台调用。
// 认证复用 admin 包的 HMAC 签名 cookie 机制，未认证返回 401 JSON 而非重定向。
package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/comment"
	"blog/internal/post"
	"blog/internal/store"
	tmpl "blog/internal/template"
)

const (
	authCookieName = "blog_admin"       // 与 admin 包共用同一 cookie
	authCookieTTL  = 7 * 24 * time.Hour // 登录态有效期 7 天
)

// Handler API 请求处理器，持有 store、post service、站点配置、评论 handler。
type Handler struct {
	store     *store.Store
	posts     *post.Service
	site      *tmpl.Site
	comments  *comment.Handler
	uploadDir string
	maxUpload int64 // 单文件上限字节

	// onLinksChanged 友链变更后的回调（用于触发 friends 聚合器即时刷新），可空。
	onLinksChanged func()
}

// Config API Handler 初始化参数。
type Config struct {
	Store       *store.Store
	Posts       *post.Service
	Site        *tmpl.Site
	Comments    *comment.Handler
	UploadDir   string
	MaxUploadMB int
}

// New 创建 API handler。
func New(cfg Config) *Handler {
	maxMB := cfg.MaxUploadMB
	if maxMB <= 0 {
		maxMB = 5
	}
	return &Handler{
		store:     cfg.Store,
		posts:     cfg.Posts,
		site:      cfg.Site,
		comments:  cfg.Comments,
		uploadDir: cfg.UploadDir,
		maxUpload: int64(maxMB) << 20,
	}
}

// SetOnLinksChanged 注册友链变更回调（如 friends 聚合器的 Refresh），友链增删改后调用以即时刷新 /friends。
func (h *Handler) SetOnLinksChanged(fn func()) {
	h.onLinksChanged = fn
}

// Register 把 /api/v1 路由组挂到 r 上。
func (h *Handler) Register(r *gin.Engine) {
	v1 := r.Group("/api/v1")

	// 公开路由（无需认证）
	v1.POST("/auth/login", h.login)
	v1.POST("/auth/logout", h.logout)
	v1.GET("/auth/check", h.authCheck)

	// 需认证路由
	authed := v1.Group("", h.apiAuth())
	{
		// 仪表盘
		authed.GET("/dashboard/stats", h.dashboardStats)
		authed.GET("/dashboard/recent-comments", h.dashboardRecentComments)

		// 文章
		authed.GET("/posts", h.listPosts)
		authed.GET("/posts/:slug", h.getPost)
		authed.POST("/posts", h.createPost)
		authed.PUT("/posts/:slug", h.updatePost)
		authed.DELETE("/posts/:slug", h.deletePost)

		// 评论
		authed.GET("/comments", h.listComments)
		authed.PUT("/comments/:id/approve", h.approveComment)
		authed.DELETE("/comments/:id", h.deleteComment)
		authed.POST("/comments/reply", h.replyComment)

		// Markdown 预览（编辑器实时预览，复用前台 goldmark 渲染管线）
		authed.POST("/preview/markdown", h.previewMarkdown)

		// 附件
		authed.GET("/attachments", h.listAttachments)
		authed.POST("/attachments/upload", h.uploadAttachment)
		authed.DELETE("/attachments/:id", h.deleteAttachment)

		// 页面
		authed.GET("/pages", h.listPages)
		authed.GET("/pages/:slug", h.getPage)
		authed.POST("/pages", h.createPage)
		authed.PUT("/pages/:slug", h.updatePage)
		authed.DELETE("/pages/:slug", h.deletePage)

		// 瞬间
		authed.GET("/moments", h.listMoments)
		authed.POST("/moments", h.createMoment)
		authed.DELETE("/moments/:id", h.deleteMoment)

		// 站点设置（分段）
		authed.GET("/settings", h.getSettings)
		authed.PUT("/settings/:section", h.updateSection)

		// 友链
		authed.GET("/links", h.listLinks)
		authed.POST("/links", h.createLink)
		authed.PUT("/links/:id", h.updateLink)
		authed.DELETE("/links/:id", h.deleteLink)

		// 分类 / 标签
		authed.GET("/categories", h.listCategories)
		authed.PUT("/categories/:name", h.renameCategory)
		authed.DELETE("/categories/:name", h.deleteCategory)
		authed.GET("/tags", h.listTags)
		authed.POST("/tags/merge", h.mergeTags)
		authed.PUT("/tags/:name", h.renameTag)
		authed.DELETE("/tags/:name", h.deleteTag)
	}
}

// apiAuth API 鉴权中间件：密码未配置直接放行；否则校验 HMAC cookie，失败返回 401 JSON。
func (h *Handler) apiAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		password := h.site.Admin.Password
		if password == "" {
			c.Next()
			return
		}
		if cookie, err := c.Cookie(authCookieName); err == nil && validAuthCookie(cookie, password) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录或登录已过期"})
	}
}

// --- HMAC cookie 签名/验证（与 admin/auth.go 逻辑一致） ---

func signAuthCookie(expiry int64, password string) string {
	mac := hmac.New(sha256.New, []byte(password))
	mac.Write([]byte(strconv.FormatInt(expiry, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

func validAuthCookie(value, password string) bool {
	i := strings.IndexByte(value, '.')
	if i <= 0 {
		return false
	}
	expiry, err := strconv.ParseInt(value[:i], 10, 64)
	if err != nil || time.Now().Unix() > expiry {
		return false
	}
	expected := signAuthCookie(expiry, password)
	return subtle.ConstantTimeCompare([]byte(value[i+1:]), []byte(expected)) == 1
}
