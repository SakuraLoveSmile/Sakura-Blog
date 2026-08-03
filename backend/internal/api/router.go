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
	"blog/internal/module"
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
	modules   *module.Manager
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
	Modules     *module.Manager
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
		store:     cfg.Store,
		posts:     cfg.Posts,
		site:      cfg.Site,
		comments:  cfg.Comments,
		backups:   cfg.Backups,
		modules:   cfg.Modules,
		uploadDir: cfg.UploadDir,
		notesPath: cfg.NotesPath,
		maxUpload: int64(maxMB) << 20,
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
		{"ratings", st.EnsureRatingsTable},          // 评分数据服务（Spec 20）
		{"votes", st.EnsureVotesTable},              // 投票数据服务（Spec 30）
		{"revisions", st.EnsureRevisionsTable},      // 文章版本历史（Spec 22）
		{"page_views", st.EnsurePageViewsTable},     // 访问统计明细（Spec 24）
		{"attachments", st.EnsureAttachmentsTable},  // 附件管理
		{"shared_posts", st.EnsureSharedPostsTable}, // 文章分享链接
		{"wikidocs", st.EnsureWikidocsTable},        // wiki 文档（Spec 35）
		{"notebooks", st.EnsureNotebooksTable},      // 笔记本（Spec 35b）
		{"notes", st.EnsureNotesTable},              // 笔记（Spec 35b）
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
	v1.POST("/auth/2fa", h.handle2FA)

	// AI 摘要读者自助生成（Spec 53，公开端点：读者临时接入自己的 API，限流/SSRF 防护在 handler 内）
	v1.POST("/ai/summary/guest", h.aiSummaryGuest)

	// 需认证路由
	authed := v1.Group("", h.apiAuth())
	{
		// 管理端全局搜索（Spec 39，Console ⌘K）
		authed.GET("/search", h.adminSearch)

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

		// AI 文章摘要（Spec 53）：保存/清空预生成摘要
		authed.PUT("/posts/:slug/ai-summary", h.updateAISummary)

		// 文章版本历史（Spec 22）
		authed.GET("/posts/:slug/revisions", h.listRevisions)
		authed.GET("/revisions/:id", h.getRevision)
		authed.POST("/revisions/:id/restore", h.restoreRevision)

		// 评论
		authed.GET("/comments", h.listComments)
		authed.PUT("/comments/:id/approve", h.approveComment)
		authed.DELETE("/comments/:id", h.deleteComment)
		authed.POST("/comments/reply", h.replyComment)
		authed.POST("/comments/batch", h.batchComments)

		// Markdown 预览（编辑器实时预览，复用前台 goldmark 渲染管线）
		authed.POST("/preview/markdown", h.previewMarkdown)
		// 发布前预览：存当前表单快照，返回 token（前台 /preview/draft/:token 渲染）
		authed.POST("/preview/post", h.createPostPreview)

		// AI 属性生成（OpenAI 兼容代理，配置在 settings "ai" 段）
		authed.POST("/ai/generate", h.aiGenerate)

		// 附件
		authed.GET("/attachments", h.listAttachments)
		authed.POST("/attachments/upload", h.uploadAttachment)
		authed.POST("/attachments/move", h.moveAttachments)
		authed.GET("/attachments/:id/refs", h.attachmentRefs)
		authed.DELETE("/attachments/:id", h.deleteAttachment)

		// 附件分组（Spec 44）
		authed.GET("/attachment-groups", h.listAttachmentGroups)
		authed.POST("/attachment-groups", h.createAttachmentGroup)
		authed.PUT("/attachment-groups/:id", h.updateAttachmentGroup)
		authed.DELETE("/attachment-groups/:id", h.deleteAttachmentGroup)

		// 页面
		authed.GET("/pages", h.listPages)
		authed.GET("/pages/:slug", h.getPage)
		authed.POST("/pages", h.createPage)
		authed.PUT("/pages/:slug", h.updatePage)
		authed.DELETE("/pages/:slug", h.deletePage)
		authed.POST("/pages/:slug/restore", h.restorePage)
		authed.DELETE("/pages/:slug/purge", h.purgePage)

		// wiki 文档（Spec 35）
		wiki := authed.Group("/wiki")
		{
			wiki.GET("/:project/docs", h.listWikidocs)
			wiki.GET("/:project/docs/:slug", h.getWikidoc)
			wiki.POST("/:project/docs", h.createWikidoc)
			wiki.PUT("/:project/docs/:slug", h.updateWikidoc)
			wiki.DELETE("/:project/docs/:slug", h.deleteWikidoc)
		}

		// 笔记本 / 笔记（Spec 35b）
		notebooks := authed.Group("/notebooks")
		{
			notebooks.GET("", h.listNotebooks)
			notebooks.POST("", h.createNotebook)
			notebooks.GET("/:id", h.getNotebook)
			notebooks.PUT("/:id", h.updateNotebook)
			notebooks.DELETE("/:id", h.deleteNotebook)
			notebooks.GET("/:id/notes", h.listNotes)
			notebooks.POST("/:id/notes", h.createNote)
		}

		// 专栏（Spec 52）
		authed.GET("/topics", h.listTopics)
		authed.POST("/topics", h.createTopic)
		authed.PUT("/topics/:id", h.updateTopic)
		authed.DELETE("/topics/:id", h.deleteTopic)
		authed.GET("/notes/:id", h.getNote)
		authed.PUT("/notes/:id", h.updateNote)
		authed.DELETE("/notes/:id", h.deleteNote)
		authed.GET("/note-tags", h.noteTags)

		// 瞬间
		authed.GET("/moments", h.listMoments)
		authed.POST("/moments", h.createMoment)
		authed.PUT("/moments/:id", h.updateMoment)
		authed.DELETE("/moments/:id", h.deleteMoment)

		// 站点设置（分段）
		authed.GET("/settings", h.getSettings)
		authed.PUT("/settings/:section", h.updateSection)
		authed.POST("/settings/notify/test", h.testNotify)

		// 功能模块管理（manifest + 启停开关）
		authed.GET("/modules", h.listModules)
		authed.PUT("/modules/:id", h.toggleModule)

		// 管理密码修改（入库，保存后旧 cookie 自动失效）
		authed.PUT("/auth/password", h.changePassword)

		// 用户资料（Spec 45）
		authed.GET("/profile", h.handleGetProfile)
		authed.PUT("/profile", h.handleUpdateProfile)

		// 2FA 管理（Spec 45）
		authed.POST("/profile/2fa/setup", h.handle2FASetup)
		authed.POST("/profile/2fa/confirm", h.handle2FAConfirm)
		authed.POST("/profile/2fa/disable", h.handle2FADisable)
		authed.GET("/profile/2fa/recovery-codes", h.handle2FARecoveryCodes)

		// 笔记页正文（content/notes.md，前台 /notes 上半部分）
		authed.GET("/notes-content", h.getNotesContent)
		authed.PUT("/notes-content", h.putNotesContent)

		// 友链
		authed.GET("/links", h.listLinks)
		authed.POST("/links", h.createLink)
		authed.PUT("/links/:id", h.updateLink)
		authed.DELETE("/links/:id", h.deleteLink)
		authed.POST("/links/check", h.checkLinks)

		// 友链 RSS 订阅阅读器
		authed.GET("/subscriptions", h.listSubscriptions)
		authed.GET("/subscriptions/articles", h.listSubscriptionArticles)
		authed.POST("/subscriptions/read", h.markSubscriptionRead)
		authed.POST("/subscriptions/read-all", h.markAllSubscriptionRead)
		authed.POST("/subscriptions/flag", h.toggleSubscriptionFlag)
		authed.POST("/subscriptions/refresh", h.refreshSubscriptions)

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
		authed.POST("/backup/restore", h.restoreBackup)
		authed.POST("/backup/restore-snapshot", h.restoreSnapshot)

		// 功能模块后台 API：每个已注册模块套自己的 Gate 中间件（禁用即 403），
		// 在认证组下注册，运行时开关只影响可用性，无需重注册路由。
		if h.modules != nil {
			deps := module.Deps{Store: h.store, Posts: h.posts, Site: h.site, Modules: h.modules}
			for _, m := range module.All() {
				id := m.Meta().ID
				grp := authed.Group("", h.modules.Gate(id))
				m.RegisterRoutes(grp, deps)
			}
		}
	}
}

// apiAuth API 鉴权中间件：密码未配置直接放行；否则校验 HMAC cookie，失败返回 401 JSON。
// 从 cookie 解析 userID 并注入 context（userID=0 代表旧版 admin 用户）。
func (h *Handler) apiAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		password := h.site.Admin.Password
		if password == "" {
			c.Next()
			return
		}
		if cookie, err := c.Cookie(authCookieName); err == nil {
			if userID, ok := h.validAuthCookie(cookie); ok {
				c.Set("userID", userID)
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录或登录已过期"})
	}
}

// --- HMAC cookie 签名/验证（Spec 45: 迁移至 users 表 bcrypt + auth_secret 签名 key） ---

// signAuthCookie 使用 auth_secret 签名 cookie：<expiry>.<userID>.<hmac-sha256(auth_secret, expiry.userID)>
func signAuthCookie(expiry int64, userID int64, authSecret string) string {
	msg := fmt.Sprintf("%d.%d", expiry, userID)
	mac := hmac.New(sha256.New, []byte(authSecret))
	mac.Write([]byte(msg))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%s", msg, sig)
}

// validAuthCookie 解析并验证 cookie。
// 先尝试新格式（3 段：expiry.userID.sig），查 users 表获取 auth_secret 验证签名；
// 新格式失败时回退旧格式（2 段：expiry.sig，签名 key = admin password），返回 userID=0 表示旧版 admin。
func (h *Handler) validAuthCookie(value string) (int64, bool) {
	parts := strings.Split(value, ".")
	if len(parts) == 3 {
		// 新格式: expiry.userID.sig
		expiry, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || time.Now().Unix() > expiry {
			return 0, false
		}
		userID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, false
		}
		user, _ := h.store.GetUserByID(userID)
		if user == nil {
			return 0, false
		}
		msg := fmt.Sprintf("%d.%d", expiry, userID)
		mac := hmac.New(sha256.New, []byte(user.AuthSecret))
		mac.Write([]byte(msg))
		expected := hex.EncodeToString(mac.Sum(nil))
		if subtle.ConstantTimeCompare([]byte(parts[2]), []byte(expected)) == 1 {
			return userID, true
		}
		return 0, false
	}

	// 旧格式: expiry.sig（签名 key = admin password）
	if len(parts) == 2 {
		password := h.site.Admin.Password
		if password == "" {
			return 0, false
		}
		expiry, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || time.Now().Unix() > expiry {
			return 0, false
		}
		mac := hmac.New(sha256.New, []byte(password))
		mac.Write([]byte(parts[0]))
		expected := hex.EncodeToString(mac.Sum(nil))
		if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(expected)) == 1 {
			return 0, true // userID=0 代表旧版 admin
		}
	}
	return 0, false
}
