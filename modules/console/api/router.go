// Package api 提供 RESTful JSON API（/api/v1），供 Vue Console SPA 管理后台调用。
// 认证复用 admin 包的 HMAC 签名 cookie 机制，未认证返回 401 JSON 而非重定向。
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/backup"
	"blog/internal/comment"
	"blog/internal/content"
	"blog/internal/friends"
	ghpkg "blog/internal/github"
	"blog/internal/identity"
	"blog/internal/linkwatch"
	"blog/internal/module"
	"blog/internal/notify"
	"blog/internal/post"
	tmpl "blog/internal/settings"
	"blog/internal/store"
)

const (
	authCookieName = "blog_admin"       // 与 admin 包共用同一 cookie
	authCookieTTL  = 7 * 24 * time.Hour // 登录态有效期 7 天
)

// Handler API 请求处理器，持有 store、post service、站点配置、评论 handler。
type Handler struct {
	store *store.Store
	posts *post.Service
	// site 站点配置原子指针（Spec 61 阶段 3）：RebuildSite 复制-交换发布，API 侧 .Load() 读取
	site      *atomic.Pointer[tmpl.Site]
	comments  *comment.Handler
	backups   *backup.Service
	modules   *module.Manager
	notify    *notify.Service // 站内通知服务（Wave 3b），nil 时通知 API 返回空
	uploadDir string
	maxUpload int64 // 单文件上限字节

	// onLinksChanged 友链变更后的回调（用于触发 friends 聚合器即时刷新），可空。
	onLinksChanged func()
	linkWatch      *linkwatch.Watcher

	// gh GitHub 代理缓存客户端（Spec 54 友链导入用），可空。
	gh *ghpkg.Client

	// icons 图标注册表（icons.yml），素材速查端点用，可空。
	icons *tmpl.IconRegistry

	// renderSitePreview 设置页「前端预览」渲染回调（由组合入口注入，nil 时端点 501）。
	renderSitePreview func(sections map[string]json.RawMessage, page string) (string, error)
	// events is the shared content-change publisher. Console does not know
	// which module owns the derived caches that consume these events.
	events content.Events

	// 登录限流（P0-4）
	loginMu    sync.Mutex
	loginFails map[string]struct {
		n int
		t time.Time
	}

	// 注册限流（安全审计 2026-08-16）
	registerMu   sync.Mutex
	registerHits map[string]registerHit

	// PAT 限流（每令牌 60 次/分钟，与登录限流同模式）
	patMu   sync.Mutex
	patHits map[int64]patWindow
}

// registerHit 注册限流计数条目。
type registerHit struct {
	n int
	t time.Time
}

// Config API Handler 初始化参数。
type Config struct {
	Store    *store.Store
	Posts    *post.Service
	Site     *atomic.Pointer[tmpl.Site] // 站点配置原子指针（Spec 61 阶段 3）
	Comments *comment.Handler
	Backups  *backup.Service
	Modules  *module.Manager
	Notify   *notify.Service    // 站内通知服务（Wave 3b），可空
	Icons    *tmpl.IconRegistry // 图标注册表（素材速查端点），可空
	// RenderSitePreview 设置页「前端预览」渲染回调（草稿分段 → 前台整页 HTML），可空。
	RenderSitePreview func(sections map[string]json.RawMessage, page string) (string, error)
	// Events publishes generic changes such as wiki/notebook/note/settings writes.
	Events      content.Events
	UploadDir   string
	MaxUploadMB int
	LinkWatch   *linkwatch.Watcher
}

// New 创建 API handler。
func New(cfg Config) *Handler {
	maxMB := cfg.MaxUploadMB
	if maxMB <= 0 {
		maxMB = 5
	}
	return &Handler{
		store:             cfg.Store,
		posts:             cfg.Posts,
		site:              cfg.Site,
		comments:          cfg.Comments,
		backups:           cfg.Backups,
		modules:           cfg.Modules,
		notify:            cfg.Notify,
		icons:             cfg.Icons,
		renderSitePreview: cfg.RenderSitePreview,
		events:            cfg.Events,
		uploadDir:         cfg.UploadDir,
		maxUpload:         int64(maxMB) << 20,
		linkWatch:         cfg.LinkWatch,
		loginFails: make(map[string]struct {
			n int
			t time.Time
		}),
		registerHits: make(map[string]registerHit),
		patHits:      make(map[int64]patWindow),
	}
}

// SetOnLinksChanged 注册友链变更回调（如 friends 聚合器的 Refresh），友链增删改后调用以即时刷新 /friends。
func (h *Handler) SetOnLinksChanged(fn func()) {
	h.onLinksChanged = fn
}

func (h *Handler) publishContentChanged() {
	if h.events != nil {
		h.events.Publish(content.Event{Kind: content.ContentChanged})
	}
}

// MountDeps API 侧装配依赖：Config 之外附带公共 API 端点的可选组件。
type MountDeps struct {
	Config
	Friends   *friends.Aggregator // 友链动态聚合（/api/friends-aggregated），可空
	LinkWatch *linkwatch.Watcher  // 友链连通性/截图巡检器，可空
	GH        *ghpkg.Client       // GitHub 代理缓存客户端（/api/gh/*），可空
}

// Mount 承接 main 的 API 侧装配：建表 → /api/v1 路由组 → 友链聚合/GitHub 代理/评分公开端点。
// 组件生命周期（如 friends 聚合器的 Start）仍由调用方管理；返回 Handler 供上层继续挂回调。
func Mount(r *gin.Engine, d MountDeps) (*Handler, error) {
	return mount(r, d, true)
}

// MountConsole is the module-owned mount. It deliberately excludes public
// reader and content endpoints; modules/stellar owns those routes at runtime.
func MountConsole(r *gin.Engine, d MountDeps) (*Handler, error) {
	return mount(r, d, false)
}

func mount(r *gin.Engine, d MountDeps, includePublic bool) (*Handler, error) {
	st := d.Store
	ensures := []struct {
		name string
		fn   func() error
	}{
		{"users", st.EnsureUsersTable},                 // 多用户/读者注册
		{"ratings", st.EnsureRatingsTable},             // 评分数据服务（Spec 20）
		{"votes", st.EnsureVotesTable},                 // 投票数据服务（Spec 30）
		{"revisions", st.EnsureRevisionsTable},         // 文章版本历史（Spec 22）
		{"page_views", st.EnsurePageViewsTable},        // 访问统计明细（Spec 24）
		{"attachments", st.EnsureAttachmentsTable},     // 附件管理
		{"shared_posts", st.EnsureSharedPostsTable},    // 文章分享链接
		{"wikidocs", st.EnsureWikidocsTable},           // wiki 文档（Spec 35）
		{"notebooks", st.EnsureNotebooksTable},         // 笔记本（Spec 35b）
		{"notes", st.EnsureNotesTable},                 // 笔记（Spec 35b）
		{"login_logs", st.EnsureLoginLogsTable},        // 登录日志（Wave 3a）
		{"notifications", st.EnsureNotificationsTable}, // 站内通知（Wave 3b）
		{"pat_tokens", st.EnsurePATTable},              // 个人访问令牌（PAT，只存 sha256 哈希）
		{"links", st.EnsureLinksTable},                 // 友链与巡检状态
	}
	for _, e := range ensures {
		if err := e.fn(); err != nil {
			return nil, fmt.Errorf("ensure %s table: %w", e.name, err)
		}
	}

	h := New(d.Config)
	if d.GH != nil {
		// The Console link-import handler uses the shared client even when the
		// public GitHub HTTP routes are mounted by Stellar.
		h.gh = d.GH
	}

	if includePublic {
		// 读者身份中间件与后台 cookie 互相独立。
		r.Use(h.currentUserMiddleware())
		auth := r.Group("/api/auth")
		{
			auth.POST("/register", h.publicRegister)
			auth.POST("/login", h.publicLogin)
			auth.GET("/me", h.publicMe)
			auth.POST("/logout", h.publicLogout)
		}
	}

	h.register(r, includePublic)
	if includePublic {
		mountLegacyPublic(r, h, d)
	}
	return h, nil
}

// mountLegacyPublic preserves the pre-module API contract for package callers
// and tests. The application entry point uses modules/stellar/api instead.
func mountLegacyPublic(r *gin.Engine, h *Handler, d MountDeps) {
	if d.Friends != nil {
		d.Friends.Register(r)
		h.SetOnLinksChanged(d.Friends.Refresh)
	}
	if d.GH != nil {
		ghpkg.NewHandler(d.GH).Register(r)
	}
	store.NewRatingHandler(d.Store).Register(r)
	store.NewVoteHandler(d.Store).Register(r)
}

// Register 把 /api/v1 路由组挂到 r 上。
func (h *Handler) Register(r *gin.Engine) {
	h.register(r, true)
}

// RegisterConsole mounts only the Console-owned /api/v1 routes.
func (h *Handler) RegisterConsole(r *gin.Engine) {
	h.register(r, false)
}

func (h *Handler) register(r *gin.Engine, includePublic bool) {
	v1 := r.Group("/api/v1")

	// 公开路由（无需认证）
	v1.POST("/auth/login", h.login)
	v1.POST("/auth/logout", h.logout)
	v1.GET("/auth/check", h.authCheck)
	v1.POST("/auth/2fa", h.handle2FA)

	// AI 摘要读者自助生成（Spec 53，公开端点：读者临时接入自己的 API，限流/SSRF 防护在 handler 内）
	v1.POST("/ai/summary/guest", h.aiSummaryGuest)

	// 需认证路由（账户级：资料/令牌/通知/改密，任何登录角色可用）
	authed := v1.Group("", h.apiAuth())
	{
		// API Token（Wave 3a）：生成/撤销 Bearer 认证凭证
		authed.GET("/profile/api-token", h.apiTokenStatus)
		authed.POST("/profile/api-token", h.generateAPIToken)
		authed.DELETE("/profile/api-token", h.revokeAPIToken)

		// 站内通知中心（Wave 3b）：未读优先分页 + 单条/全部已读 + 未读数
		authed.GET("/notifications", h.listNotifications)
		authed.GET("/notifications/unread-count", h.unreadNotifications)
		authed.PUT("/notifications/:id/read", h.markNotificationRead)
		authed.PUT("/notifications/read-all", h.markAllNotificationsRead)

		// 管理密码修改（入库，保存后旧 cookie 自动失效）
		authed.PUT("/auth/password", h.changePassword)

		// 个人访问令牌管理（PAT）：创建时明文仅返回一次，列表脱敏，DELETE 即吊销
		authed.GET("/tokens", h.listTokens)
		authed.POST("/tokens", h.createToken)
		authed.DELETE("/tokens/:id", h.deleteToken)

		// 用户资料（Spec 45）
		authed.GET("/profile", h.handleGetProfile)
		authed.PUT("/profile", h.handleUpdateProfile)

		// 2FA 管理（Spec 45）
		authed.POST("/profile/2fa/setup", h.handle2FASetup)
		authed.POST("/profile/2fa/confirm", h.handle2FAConfirm)
		authed.POST("/profile/2fa/disable", h.handle2FADisable)
		authed.GET("/profile/2fa/recovery-codes", h.handle2FARecoveryCodes)
	}

	// 内容管理组（author 及以上，安全审计 2026-08-16）：
	// 此前全部内容写端点仅要求登录——开放注册的 reader 可经 console 登录端点换取
	// blog_admin cookie 后写文章/页面/瞬间/友链（配合 WithUnsafe 渲染构成存储型 XSS
	// 全链路）。现统一加 requireRole(RoleAuthor) 最低角色门禁。
	author := v1.Group("", h.apiAuth(), h.requireRole(store.RoleAuthor))
	{
		// 管理端全局搜索（Spec 39，Console ⌘K）
		author.GET("/search", h.adminSearch)

		// 仪表盘
		author.GET("/dashboard/stats", h.dashboardStats)
		author.GET("/dashboard/recent-comments", h.dashboardRecentComments)
		author.GET("/dashboard/recent-posts", h.dashboardRecentPosts)
		author.GET("/dashboard/categories", h.dashboardCategories)
		author.GET("/dashboard/quick-actions", h.quickActions)
		author.PUT("/dashboard/quick-actions", h.saveQuickActions)

		// 访问统计（Spec 24）
		author.GET("/stats/trend", h.statsTrend)
		author.GET("/stats/popular", h.statsPopular)
		author.GET("/stats/referrers", h.statsReferrers)
		author.GET("/stats/hourly", h.statsHourly)

		// 文章
		author.GET("/posts", h.listPosts)
		author.GET("/posts/:slug", h.getPost)
		author.POST("/posts", h.createPost)
		author.PUT("/posts/:slug", h.updatePost)
		author.DELETE("/posts/:slug", h.deletePost)
		author.POST("/posts/:slug/restore", h.restorePost)
		author.DELETE("/posts/:slug/purge", h.purgePost)

		// 文章草稿头（已保存未发布，列表小绿点）
		author.PUT("/posts/:slug/draft", h.savePostDraft)
		author.DELETE("/posts/:slug/draft", h.discardPostDraft)
		author.POST("/posts/:slug/publish", h.publishPost)

		// AI 文章摘要（Spec 53）：保存/清空预生成摘要
		author.PUT("/posts/:slug/ai-summary", h.updateAISummary)

		// 文章版本历史（Spec 22）
		author.GET("/posts/:slug/revisions", h.listRevisions)
		author.GET("/revisions/:id", h.getRevision)
		author.POST("/revisions/:id/restore", h.restoreRevision)

		// 评论
		author.GET("/comments", h.listComments)
		author.PUT("/comments/:id/approve", h.approveComment)
		author.PUT("/comments/:id/status", h.statusComment)
		author.DELETE("/comments/:id", h.deleteComment)
		author.POST("/comments/reply", h.replyComment)
		author.POST("/comments/batch", h.batchComments)

		// Markdown 预览（编辑器实时预览，复用前台 goldmark 渲染管线）
		author.POST("/preview/markdown", h.previewMarkdown)
		// 站点配置预览（设置页所见即所得：草稿分段 → 前台整页 HTML）
		author.POST("/preview/site", h.previewSite)
		// 发布前预览：存当前表单快照，返回 token（前台 /preview/draft/:token 渲染）
		author.POST("/preview/post", h.createPostPreview)

		// 组件库预览 Fixture 端点（Spec 63）：只服务预览，不落库、不发外部请求
		author.GET("/preview/rating", h.previewRatingFixture)
		author.POST("/preview/rating", h.previewRatingFixtureSubmit)
		author.GET("/preview/vote/info", h.previewVoteFixtureInfo)
		author.POST("/preview/vote/update", h.previewVoteFixtureUpdate)
		author.GET("/preview/friends", h.previewFriendsFixture)
		author.GET("/preview/sites", h.previewSitesFixture)
		author.GET("/preview/md", h.previewMDFixture)

		// 附件
		author.GET("/attachments", h.listAttachments)
		author.POST("/attachments/upload", h.uploadAttachment)
		author.POST("/attachments/move", h.moveAttachments)
		author.GET("/attachments/:id/refs", h.attachmentRefs)
		author.DELETE("/attachments/:id", h.deleteAttachment)

		// 附件分组（Spec 44）
		author.GET("/attachment-groups", h.listAttachmentGroups)
		author.POST("/attachment-groups", h.createAttachmentGroup)
		author.PUT("/attachment-groups/:id", h.updateAttachmentGroup)
		author.DELETE("/attachment-groups/:id", h.deleteAttachmentGroup)

		// 页面
		author.GET("/pages", h.listPages)
		author.GET("/pages/:slug", h.getPage)
		author.POST("/pages", h.createPage)
		author.PUT("/pages/:slug", h.updatePage)
		author.DELETE("/pages/:slug", h.deletePage)
		author.POST("/pages/:slug/restore", h.restorePage)
		author.DELETE("/pages/:slug/purge", h.purgePage)

		// wiki 文档（Spec 35）
		wiki := author.Group("/wiki")
		{
			wiki.GET("/:project/docs", h.listWikidocs)
			wiki.GET("/:project/docs/:slug", h.getWikidoc)
			wiki.POST("/:project/docs", h.createWikidoc)
			wiki.PUT("/:project/docs/:slug", h.updateWikidoc)
			wiki.DELETE("/:project/docs/:slug", h.deleteWikidoc)
		}

		// 笔记本 / 笔记（Spec 35b）
		notebooks := author.Group("/notebooks")
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
		author.GET("/topics", h.listTopics)
		author.POST("/topics", h.createTopic)
		author.PUT("/topics/:id", h.updateTopic)
		author.DELETE("/topics/:id", h.deleteTopic)
		author.GET("/notes/:id", h.getNote)
		author.PUT("/notes/:id", h.updateNote)
		author.DELETE("/notes/:id", h.deleteNote)
		author.GET("/note-tags", h.noteTags)

		// 瞬间
		author.GET("/moments", h.listMoments)
		author.POST("/moments", h.createMoment)
		author.PUT("/moments/:id", h.updateMoment)
		author.DELETE("/moments/:id", h.deleteMoment)

		// 友链 RSS 订阅阅读器
		author.GET("/subscriptions", h.listSubscriptions)
		author.GET("/subscriptions/articles", h.listSubscriptionArticles)
		author.POST("/subscriptions/read", h.markSubscriptionRead)
		author.POST("/subscriptions/read-all", h.markAllSubscriptionRead)
		author.POST("/subscriptions/flag", h.toggleSubscriptionFlag)
		author.POST("/subscriptions/refresh", h.refreshSubscriptions)

		// 文章分享链接
		author.GET("/shares", h.listShares)
		author.GET("/posts/:slug/shares", h.listPostShares)
		author.POST("/posts/:slug/shares", h.createShare)
		author.DELETE("/shares/:id", h.deleteShare)

		// 分类 / 标签
		author.GET("/categories", h.listCategories)
		author.PUT("/categories/:name", h.renameCategory)
		author.DELETE("/categories/:name", h.deleteCategory)
		author.GET("/tags", h.listTags)
		author.POST("/tags/merge", h.mergeTags)
		author.PUT("/tags/:name", h.renameTag)
		author.DELETE("/tags/:name", h.deleteTag)

		// 素材速查（插件库图标画廊数据源：icons.yml 全量 key + SVG）
		author.GET("/assets/icons", h.listAssetIcons)
	}

	// 系统管理组（仅 admin，且站点必须已启用密码认证——安全审计 2026-08-16）：
	// 用户管理 / 登录日志 / 站点设置 / 功能模块 / 备份导出 / SEO 模块 / 友链管理 / AI 代理。
	sys := v1.Group("", h.apiAuth(), h.requireSysAdmin())
	{
		// 用户管理（多用户体系）：列表/创建/更新/重置密码/删除
		sys.GET("/users", h.listUsers)
		sys.POST("/users", h.createUser)
		sys.PUT("/users/:id", h.updateUser)
		sys.PUT("/users/:id/password", h.resetUserPassword)
		sys.DELETE("/users/:id", h.deleteUser)

		// 评论用户管理（Spec 71）：注册读者列表/封禁解封/删除/用户级评论批量处置
		sys.GET("/comment-users", h.listCommentUsers)
		sys.PUT("/comment-users/:id/status", h.setCommentUserStatus)
		sys.DELETE("/comment-users/:id", h.deleteCommentUser)
		sys.POST("/comment-users/:id/comments/bulk", h.bulkCommentUserComments)

		// 登录日志（Wave 3a）：登录/2FA 记录查询，分页 + 状态筛选
		sys.GET("/auth/login-logs", h.listLoginLogs)

		// 站点设置（分段）
		sys.GET("/settings", h.getSettings)
		sys.PUT("/settings/:section", h.updateSection)
		sys.POST("/settings/notify/test", h.testNotify)
		// 主题 YAML 编辑器（Spec 60）：style 段 YAML 读写 + 热更新
		sys.GET("/settings/style/yaml", h.getStyleYAML)
		sys.PUT("/settings/style/yaml", h.updateStyleYAML)

		// 功能模块管理（manifest + 启停开关）
		sys.GET("/modules", h.listModules)
		sys.PUT("/modules/:id", h.toggleModule)

		// 备份与导出（下载走 cookie 认证，Console 直接 <a href> 触发）
		sys.GET("/backup/export", h.exportBackup)
		sys.GET("/backup/export/markdown", h.exportPostsMarkdown)
		sys.POST("/backup/import/markdown", h.importPostsMarkdown)
		sys.GET("/backup/status", h.backupStatus)
		sys.POST("/backup/restore", h.restoreBackup)
		sys.POST("/backup/restore-snapshot", h.restoreSnapshot)

		// 友链（安全审计 2026-08-16 收进 sys）：links 表 URL 会驱动服务端出站探测/抓取
		// （linkwatch/friends），探测结果回显构成 SSRF oracle，收敛到 admin 管理。
		sys.GET("/links", h.listLinks)
		sys.POST("/links", h.createLink)
		sys.PUT("/links/:id", h.updateLink)
		sys.DELETE("/links/:id", h.deleteLink)
		sys.POST("/links/check", h.checkLinks)
		sys.POST("/links/:id/snapshot", h.snapshotLink)
		sys.POST("/links/import", h.importLinks)

		// AI 属性生成（OpenAI 兼容代理，配置在 settings "ai" 段）：
		// 消耗 admin 配置的第三方 key，仅 admin 可用（安全审计 2026-08-16 收进 sys）。
		sys.POST("/ai/generate", h.aiGenerate)

		// 功能模块后台 API：仅 admin；每个已注册模块套自己的 Gate 中间件（禁用即 403）。
		if h.modules != nil {
			deps := module.Deps{Store: h.store, Posts: h.posts, Site: h.site, Modules: h.modules}
			for _, m := range module.All() {
				id := m.Meta().ID
				grp := sys.Group("", h.modules.Gate(id))
				m.RegisterRoutes(grp, deps)
			}
		}
	}

	if includePublic {
		// Legacy direct callers retain the complete API surface. The top-level
		// Console mount uses RegisterConsole and leaves this group to Stellar.
		pub := r.Group("/api/public/v1", h.patAuth())
		{
			pub.GET("/posts", h.pubListPosts)
			pub.GET("/posts/:slug", h.pubGetPost)
			pub.GET("/pages", h.pubListPages)
			pub.GET("/categories", h.pubListCategories)
			pub.GET("/tags", h.pubListTags)
			pub.GET("/comments", h.pubListComments)
		}
	}
}

// apiAuth API 鉴权中间件：AuthEnabled=false（无密码站点）直接放行；否则校验 HMAC cookie 或 Bearer Token，
// 失败返回 401 JSON。从 cookie/token 解析 userID 并注入 context（userID=0 代表旧版 admin 用户）。
// cookie 校验通过且剩余有效期 < TTL 一半时自动签发新 cookie（滑动续期）。
func (h *Handler) apiAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !h.site.Load().Admin.AuthEnabled {
			// 无密码站点：保持历史全放行语义，视为 admin。
			c.Set("userID", int64(0))
			c.Set("userRole", store.RoleAdmin)
			c.Next()
			return
		}

		// Bearer Token 认证（脚本/第三方调用，优先级高于 cookie）
		if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			token := strings.TrimPrefix(auth, "Bearer ")
			user, err := h.store.GetUserByAPITokenHash(store.HashAPIToken(token))
			if err != nil || user == nil {
				AbortFail(c, http.StatusUnauthorized, "API Token 无效或已撤销")
				return
			}
			c.Set("userID", user.ID)
			c.Set("userRole", normalizeRole(user.Role))
			c.Next()
			return
		}

		if cookie, err := c.Cookie(authCookieName); err == nil {
			if userID, authSecret, expiry, role, ok := h.validAuthCookie(cookie, cookieAudAdmin); ok {
				c.Set("userID", userID)
				c.Set("userRole", role)
				// 滑动续期：新格式 cookie 剩余有效期不足一半时重新签发（旧格式无 authSecret 不续期）
				if userID > 0 && authSecret != "" && expiry-time.Now().Unix() < int64(authCookieTTL.Seconds())/2 {
					newExpiry := time.Now().Add(authCookieTTL).Unix()
					c.SetSameSite(http.SameSiteLaxMode)
					c.SetCookie(authCookieName, signAuthCookie(cookieAudAdmin, newExpiry, userID, authSecret), int(authCookieTTL.Seconds()), "/", "", h.cookieSecure(c), true)
				}
				c.Next()
				return
			}
		}
		AbortFail(c, http.StatusUnauthorized, "未登录或登录已过期")
	}
}

// --- HMAC cookie 签名/验证（Spec 45: 迁移至 users 表 bcrypt + auth_secret 签名 key） ---

// cookie audience（安全审计 2026-08-16）：admin cookie（blog_admin，console/API）与
// 读者 cookie（blog_user，前台）此前共用同一签名算法与 per-user auth_secret，cookie 名
// 不参与 HMAC——读者 cookie 改名即可冒充 admin 会话。现将 audience 绑定进签名消息，
// 跨用途重放立即验签失败。副作用：升级后所有旧格式 cookie 失效，需重新登录。
const (
	cookieAudAdmin = "admin"
	cookieAudUser  = "user"
)

// signAuthCookie 使用 auth_secret 签名 cookie：<aud>.<expiry>.<userID>.<hmac-sha256(auth_secret, aud.expiry.userID)>
func signAuthCookie(aud string, expiry int64, userID int64, authSecret string) string {
	return identity.SignCookie(identity.Audience(aud), expiry, userID, authSecret)
}

// validAuthCookie 解析并验证 cookie（安全审计 2026-08-16 起 4 段格式）：
// aud.expiry.userID.sig，aud 必须与调用方期望一致（admin/user），查 users 表取
// auth_secret 验证 HMAC 签名。旧 3 段格式无 audience，一律拒绝。
// 返回 (userID, authSecret, expiry, role, ok)；role 为登录用户角色。
func (h *Handler) validAuthCookie(value, aud string) (int64, string, int64, string, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 4 || parts[0] != aud {
		return 0, "", 0, "", false
	}
	expiry, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, "", 0, "", false
	}
	userID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return 0, "", 0, "", false
	}
	user, _ := h.store.GetUserByID(userID)
	if user == nil {
		return 0, "", 0, "", false
	}
	if _, _, ok := identity.VerifyCookie(value, identity.Audience(aud), user.AuthSecret, time.Now()); ok {
		return userID, user.AuthSecret, expiry, normalizeRole(user.Role), true
	}
	return 0, "", 0, "", false
}

// normalizeRole 角色归一化：非法/空值回退作者（最小权限）。
func normalizeRole(role string) string {
	if store.ValidRole(role) {
		return role
	}
	return store.RoleAuthor
}

// roleLevel 角色层级（安全审计 2026-08-16）：admin(3) > author(2) > reader(1)。
func roleLevel(role string) int {
	switch role {
	case store.RoleAdmin:
		return 3
	case store.RoleAuthor:
		return 2
	default:
		return 1
	}
}

// requireRole 角色校验中间件（最低角色语义，安全审计 2026-08-16）：
// 当前登录用户角色层级 >= role 才放行（admin 通过 author 门禁，反向不行）。
func (h *Handler) requireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		got, _ := c.Get("userRole")
		gotRole, _ := got.(string)
		if roleLevel(gotRole) < roleLevel(role) {
			AbortFail(c, http.StatusForbidden, "无权限：需要更高角色")
			return
		}
		c.Next()
	}
}

// requireSysAdmin 系统管理组门禁（安全审计 2026-08-16）：
// admin 角色 + 站点必须已启用密码认证。无密码站点（AuthEnabled=false）的历史
// 全放行语义不再覆盖破坏性系统端点（备份恢复/用户管理/设置等）——未设密码时
// 这些端点一律 403，倒逼先设置 admin 密码再使用系统管理功能。
func (h *Handler) requireSysAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !h.site.Load().Admin.AuthEnabled {
			AbortFail(c, http.StatusForbidden, "系统管理端点需先设置 admin 密码")
			return
		}
		got, _ := c.Get("userRole")
		gotRole, _ := got.(string)
		if roleLevel(gotRole) < roleLevel(store.RoleAdmin) {
			AbortFail(c, http.StatusForbidden, "无权限：需要管理员角色")
			return
		}
		c.Next()
	}
}
