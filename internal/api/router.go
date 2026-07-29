// Package api 提供 RESTful JSON API（/api/v1），供 Vue Console SPA 管理后台调用。
// 认证复用 admin 包的 HMAC 签名 cookie 机制，未认证返回 401 JSON 而非重定向。
package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/backup"
	"blog/internal/comment"
	"blog/internal/friends"
	ghpkg "blog/internal/github"
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
	backups   *backup.Service
	uploadDir string
	notesPath string // content/notes.md 路径（笔记页正文）
	maxUpload int64  // 单文件上限字节

	// onLinksChanged 友链变更后的回调（用于触发 friends 聚合器即时刷新），可空。
	onLinksChanged func()

	// 登录限流（P0-4）
	loginMu    sync.Mutex
	loginFails map[string]struct {
		n int
		t time.Time
	}
}

// Config API Handler 初始化参数。
type Config struct {
	Store       *store.Store
	Posts       *post.Service
	Site        *tmpl.Site
	Comments    *comment.Handler
	Backups     *backup.Service
	UploadDir   string
	NotesPath   string
	MaxUploadMB int
}

// New 创建 API handler。
func New(cfg Config) *Handler {
	maxMB := cfg.MaxUploadMB
	if maxMB <= 0 {
		maxMB = 5
	}
	return &Handler{
		store:      cfg.Store,
		posts:      cfg.Posts,
		site:       cfg.Site,
		comments:   cfg.Comments,
		backups:    cfg.Backups,
		uploadDir:  cfg.UploadDir,
		notesPath:  cfg.NotesPath,
		maxUpload:  int64(maxMB) << 20,
		loginFails: make(map[string]struct {
			n int
			t time.Time
		}),
	}
}

// SetOnLinksChanged 注册友链变更回调（如 friends 聚合器的 Refresh），友链增删改后调用以即时刷新 /friends。
func (h *Handler) SetOnLinksChanged(fn func()) {
	h.onLinksChanged = fn
}

// MountDeps API 侧装配依赖：Config 之外附带公共 API 端点的可选组件。
type MountDeps struct {
	Config
	Friends *friends.Aggregator // 友链动态聚合（/api/friends-aggregated），可空
	GH      *ghpkg.Client       // GitHub 代理缓存客户端（/api/gh/*），可空
}

// Mount 承接 main 的 API 侧装配：建表 → /api/v1 路由组 → 友链聚合/GitHub 代理/评分公开端点。
// 组件生命周期（如 friends 聚合器的 Start）仍由调用方管理；返回 Handler 供上层继续挂回调。
func Mount(r *gin.Engine, d MountDeps) (*Handler, error) {
	st := d.Store
	ensures := []struct {
		name string
		fn   func() error
	}{
		{"ratings", st.EnsureRatingsTable},           // 评分数据服务（Spec 20）
		{"votes", st.EnsureVotesTable},               // 投票数据服务（Spec 30）
		{"revisions", st.EnsureRevisionsTable},       // 文章版本历史（Spec 22）
		{"page_views", st.EnsurePageViewsTable},      // 访问统计明细（Spec 24）
		{"attachments", st.EnsureAttachmentsTable},   // 附件管理
		{"shared_posts", st.EnsureSharedPostsTable},  // 文章分享链接
	}
	for _, e := range ensures {
		if err := e.fn(); err != nil {
			return nil, fmt.Errorf("ensure %s table: %w", e.name, err)
		}
	}

	h := New(d.Config)
	h.Register(r)

	// 友链动态聚合：/api/friends-aggregated 供 /friends 页异步渲染；
	// 友链在 console 增删改后触发聚合器即时重建缓存。
	if d.Friends != nil {
		d.Friends.Register(r)
		h.SetOnLinksChanged(d.Friends.Refresh)
	}

	// GitHub 代理端点（Spec 21）：/api/gh/repo（直连 api.github.com）。
	if d.GH != nil {
		ghpkg.NewHandler(d.GH).Register(r)
	}

	// 评分公开端点（Spec 20）：/api/rating。
	store.NewRatingHandler(st).Register(r)

	// 投票公开端点（Spec 30）：/api/vote/info、/api/vote/update。
	store.NewVoteHandler(st).Register(r)
	return h, nil
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
		authed.GET("/dashboard/quick-actions", h.quickActions)
		authed.PUT("/dashboard/quick-actions", h.saveQuickActions)

		// 访问统计（Spec 24）
		authed.GET("/stats/trend", h.statsTrend)
		authed.GET("/stats/popular", h.statsPopular)
		authed.GET("/stats/referrers", h.statsReferrers)
		authed.GET("/stats/hourly", h.statsHourly)

		// 文章
		authed.GET("/posts", h.listPosts)
		authed.GET("/posts/:slug", h.getPost)
		authed.POST("/posts", h.createPost)
		authed.PUT("/posts/:slug", h.updatePost)
		authed.DELETE("/posts/:slug", h.deletePost)
		authed.POST("/posts/:slug/restore", h.restorePost)
		authed.DELETE("/posts/:slug/purge", h.purgePost)

		// 文章草稿头（已保存未发布，列表小绿点）
		authed.PUT("/posts/:slug/draft", h.savePostDraft)
		authed.DELETE("/posts/:slug/draft", h.discardPostDraft)
		authed.POST("/posts/:slug/publish", h.publishPost)

		// 文章版本历史（Spec 22）
		authed.GET("/posts/:slug/revisions", h.listRevisions)
		authed.GET("/revisions/:id", h.getRevision)
		authed.POST("/revisions/:id/restore", h.restoreRevision)

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
		authed.GET("/attachments/:id/refs", h.attachmentRefs)
		authed.DELETE("/attachments/:id", h.deleteAttachment)

		// 页面
		authed.GET("/pages", h.listPages)
		authed.GET("/pages/:slug", h.getPage)
		authed.POST("/pages", h.createPage)
		authed.PUT("/pages/:slug", h.updatePage)
		authed.DELETE("/pages/:slug", h.deletePage)
		authed.POST("/pages/:slug/restore", h.restorePage)
		authed.DELETE("/pages/:slug/purge", h.purgePage)

		// 瞬间
		authed.GET("/moments", h.listMoments)
		authed.POST("/moments", h.createMoment)
		authed.DELETE("/moments/:id", h.deleteMoment)

		// 站点设置（分段）
		authed.GET("/settings", h.getSettings)
		authed.PUT("/settings/:section", h.updateSection)
		authed.POST("/settings/notify/test", h.testNotify)

		// 管理密码修改（入库，保存后旧 cookie 自动失效）
		authed.PUT("/auth/password", h.changePassword)

		// 笔记页正文（content/notes.md，前台 /notes 上半部分）
		authed.GET("/notes-content", h.getNotesContent)
		authed.PUT("/notes-content", h.putNotesContent)

		// 友链
		authed.GET("/links", h.listLinks)
		authed.POST("/links", h.createLink)
		authed.PUT("/links/:id", h.updateLink)
		authed.DELETE("/links/:id", h.deleteLink)

		// 文章分享链接
		authed.GET("/shares", h.listShares)
		authed.GET("/posts/:slug/shares", h.listPostShares)
		authed.POST("/posts/:slug/shares", h.createShare)
		authed.DELETE("/shares/:id", h.deleteShare)

		// 分类 / 标签
		authed.GET("/categories", h.listCategories)
		authed.PUT("/categories/:name", h.renameCategory)
		authed.DELETE("/categories/:name", h.deleteCategory)
		authed.GET("/tags", h.listTags)
		authed.POST("/tags/merge", h.mergeTags)
		authed.PUT("/tags/:name", h.renameTag)
		authed.DELETE("/tags/:name", h.deleteTag)

		// 备份与导出（下载走 cookie 认证，Console 直接 <a href> 触发）
		authed.GET("/backup/export", h.exportBackup)
		authed.GET("/backup/export/markdown", h.exportPostsMarkdown)
		authed.POST("/backup/import/markdown", h.importPostsMarkdown)
		authed.GET("/backup/status", h.backupStatus)
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
