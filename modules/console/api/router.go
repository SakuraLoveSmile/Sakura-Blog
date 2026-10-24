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
	"blog/internal/comments"
	"blog/internal/content"
	ghpkg "blog/internal/github"
	"blog/internal/identity"
	"blog/internal/linkwatch"
	"blog/internal/module"
	"blog/internal/notify"
	"blog/internal/plugin"
	"blog/internal/post"
	"blog/internal/rag"
	tmpl "blog/internal/settings"
	attachmentstorage "blog/internal/storage"
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
	site           *atomic.Pointer[tmpl.Site]
	comments       *comments.Handler
	backups        *backup.Service
	modules        *module.Manager
	notify         *notify.Service // 站内通知服务（Wave 3b），nil 时通知 API 返回空
	storage        AttachmentStorage
	storageManager *attachmentstorage.Manager
	plugins        *plugin.Runtime
	rag            *rag.Service
	maxUpload      int64 // 单文件上限字节

	// onLinksChanged 友链变更后的回调（用于触发 friends 聚合器即时刷新），可空。
	onLinksChanged func()
	linkWatch      *linkwatch.Watcher

	// gh GitHub 代理缓存客户端（Spec 54 友链导入用），可空。
	gh *ghpkg.Client
	// generate2FA 仅用于注入安全随机数失败测试。
	generate2FA    func(int64, string) (string, error)
	stellarVersion string
	stellarOwner   string
	stellarRepo    string

	// 系统更新（Console 检查更新；v1.7.3 起移除一键更新执行链）
	appVersion string
	root       string
	updateRepo string

	// icons 图标注册表（icons.yml），素材速查端点用，可空。
	icons *tmpl.IconRegistry

	// renderSitePreview 设置页「前端预览」渲染回调（由组合入口注入，nil 时端点 501）。
	renderStellarMarkdown       func(content.StellarBodyInput) (string, error)
	renderStellarContentPreview func(content.StellarPreviewInput) (content.StellarPreviewOutput, error)
	stellarWidgetCatalog        func() (map[string]any, error)
	renderSitePreview           func(sections map[string]json.RawMessage, page string) (string, error)
	// events is the shared content-change publisher. Console does not know
	// which module owns the derived caches that consume these events.
	events content.Events

	// 登录限流（P0-4）
	loginMu    sync.Mutex
	loginFails map[string]struct {
		n int
		t time.Time
	}
}

// Config API Handler 初始化参数。
type Config struct {
	Store    *store.Store
	Posts    *post.Service
	Site     *atomic.Pointer[tmpl.Site] // 站点配置原子指针（Spec 61 阶段 3）
	Comments *comments.Handler
	Backups  *backup.Service
	Modules  *module.Manager
	Notify   *notify.Service    // 站内通知服务（Wave 3b），可空
	Icons    *tmpl.IconRegistry // 图标注册表（素材速查端点），可空
	// RenderSitePreview 设置页「前端预览」渲染回调（草稿分段 → 前台整页 HTML），可空。
	RenderStellarMarkdown       func(content.StellarBodyInput) (string, error)
	RenderStellarContentPreview func(content.StellarPreviewInput) (content.StellarPreviewOutput, error)
	StellarWidgetCatalog        func() (map[string]any, error)
	RenderSitePreview           func(sections map[string]json.RawMessage, page string) (string, error)
	// Events publishes generic changes such as wiki/settings writes.
	Events         content.Events
	UploadDir      string
	Storage        AttachmentStorage
	StorageManager *attachmentstorage.Manager
	Plugins        *plugin.Runtime
	RAG            *rag.Service
	MaxUploadMB    int
	LinkWatch      *linkwatch.Watcher
	// StellarVersion is injected by the composition root to keep Console
	// independent from the Stellar module package.
	StellarVersion string
	StellarOwner   string
	StellarRepo    string
	// 系统更新配置
	Root       string
	AppVersion string
	UpdateRepo string
}

// New 创建 API handler。
func New(cfg Config) *Handler {
	maxMB := cfg.MaxUploadMB
	if maxMB <= 0 {
		maxMB = 5
	}

	// 系统更新配置默认值兜底
	repo := cfg.UpdateRepo
	if repo == "" {
		repo = updateRepoDefault
	}

	storageBackend := cfg.Storage
	if cfg.StorageManager != nil {
		storageBackend = cfg.StorageManager
	}
	return &Handler{
		store:                       cfg.Store,
		posts:                       cfg.Posts,
		site:                        cfg.Site,
		comments:                    cfg.Comments,
		backups:                     cfg.Backups,
		modules:                     cfg.Modules,
		notify:                      cfg.Notify,
		icons:                       cfg.Icons,
		renderSitePreview:           cfg.RenderSitePreview,
		renderStellarContentPreview: cfg.RenderStellarContentPreview,
		renderStellarMarkdown:       cfg.RenderStellarMarkdown,
		stellarWidgetCatalog:        cfg.StellarWidgetCatalog,
		events:                      cfg.Events,
		storage:                     attachmentStorageOrLocal(storageBackend, cfg.UploadDir),
		storageManager:              cfg.StorageManager,
		plugins:                     cfg.Plugins,
		rag:                         cfg.RAG,
		maxUpload:                   int64(maxMB) << 20,
		linkWatch:                   cfg.LinkWatch,
		stellarVersion:              cfg.StellarVersion,
		stellarOwner:                cfg.StellarOwner,
		stellarRepo:                 cfg.StellarRepo,
		appVersion:                  cfg.AppVersion,
		root:                        cfg.Root,
		updateRepo:                  repo,
		generate2FA:                 generate2FAToken,
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

func (h *Handler) publishContentChanged() {
	if h.events != nil {
		h.events.Publish(content.Event{Kind: content.ContentChanged})
	}
}

// MountDeps contains Console API dependencies beyond the shared Config.
type MountDeps struct {
	Config
	GH *ghpkg.Client // GitHub 代理缓存客户端（/api/gh/*），可空
}

// MountConsole installs only the Console-owned /api/v1 surface.
func MountConsole(r *gin.Engine, d MountDeps) (*Handler, error) {
	return mount(r, d)
}

func mount(r *gin.Engine, d MountDeps) (*Handler, error) {
	st := d.Store
	ensures := []struct {
		name string
		fn   func() error
	}{
		{"users", st.EnsureUsersTable},              // 多用户/读者注册
		{"post_authors", st.EnsurePostAuthorsTable}, // 文章 ownership（users.id）
		{"notebooks", st.EnsureNotebooksTable},
		{"notes", st.EnsureNotesTable},
		{"ratings", st.EnsureRatingsTable},                  // 评分数据服务（Spec 20）
		{"votes", st.EnsureVotesTable},                      // 投票数据服务（Spec 30）
		{"revisions", st.EnsureRevisionsTable},              // 文章版本历史（Spec 22）
		{"page_views", st.EnsurePageViewsTable},             // 访问统计明细（Spec 24）
		{"attachments", st.EnsureAttachmentsTable},          // 附件管理
		{"shared_posts", st.EnsureSharedPostsTable},         // 文章分享链接
		{"wikidocs", st.EnsureWikidocsTable},                // wiki 文档（Spec 35）
		{"wiki_projects", st.EnsureWikiProjectsTable},       // wiki 项目元数据（数据库唯一真源）
		{"login_logs", st.EnsureLoginLogsTable},             // 登录日志（Wave 3a）
		{"notifications", st.EnsureNotificationsTable},      // 站内通知（Wave 3b）
		{"pat_tokens", st.EnsurePATTable},                   // 个人访问令牌（PAT，只存 sha256 哈希）
		{"user_preferences", st.EnsureUserPreferencesTable}, // 用户级偏好（Dashboard 布局等）
		{"links", st.EnsureLinksTable},                      // 友链与巡检状态
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

	h.register(r)
	return h, nil
}

// Register 把 /api/v1 路由组挂到 r 上。
func (h *Handler) Register(r *gin.Engine) {
	h.register(r)
}

func (h *Handler) register(r *gin.Engine) {
	v1 := r.Group("/api/v1")

	// 公开路由（无需认证）
	v1.POST("/auth/login", h.login)
	v1.POST("/auth/logout", h.logout)
	v1.GET("/auth/check", h.authCheck)
	v1.POST("/auth/2fa", h.handle2FA)

	// AI 摘要读者自助生成（Spec 53，公开端点：读者临时接入自己的 API，限流/SSRF 防护在 handler 内）
	v1.POST("/ai/summary/guest", h.aiSummaryGuest)
	// RAG 前台问答：是否启用与 BYOK 策略在 handler 内判定。
	v1.POST("/rag/ask/public", h.ragAskPublic)

	// 组件库预览 Fixture 端点（Spec 63）：只服务 iframe 预览，不落库、不发外部请求，公开免鉴权以兼容 sandboxed srcdoc iframe
	v1.GET("/preview/rating", h.previewRatingFixture)
	v1.POST("/preview/rating", h.previewRatingFixtureSubmit)
	v1.GET("/preview/vote/info", h.previewVoteFixtureInfo)
	v1.POST("/preview/vote/update", h.previewVoteFixtureUpdate)
	v1.GET("/preview/friends", h.previewFriendsFixture)
	v1.GET("/preview/sites", h.previewSitesFixture)
	v1.GET("/preview/md", h.previewMDFixture)

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
	// 全链路）。现统一加 content:write 权限门禁。
	author := v1.Group("", h.apiAuth(), h.requirePermission(store.PermissionContentWrite))
	{
		// 管理端全局搜索（Spec 39，Console ⌘K）
		author.GET("/search", h.adminSearch)

		// 仪表盘
		author.GET("/dashboard/stats", h.dashboardStats)
		author.GET("/dashboard/layout", h.getDashboardLayout)
		author.PUT("/dashboard/layout", h.saveDashboardLayout)
		author.GET("/dashboard/stellar-version", h.dashboardStellarVersion)
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

		// 公开作者目录供内容编辑器使用，不授予设置管理权限。
		author.GET("/authors/catalog", h.getStellarAuthorCatalog)

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
		author.POST("/preview/widget", h.previewStellarWidget)
		// Markdown 导入（只解析内存，不落库；文章/页面编辑器共用）
		author.POST("/editor/import/markdown", h.importMarkdown)
		author.POST("/editor/import/stellar", h.importStellarMarkdown)
		author.POST("/editor/export/stellar", h.exportStellarMarkdown)
		author.POST("/editor/stellar/validate", h.validateStellarContent)
		// 站点配置预览（设置页所见即所得：草稿分段 → 前台整页 HTML）
		author.POST("/preview/site", h.previewSite)
		// 发布前预览：存当前表单快照，返回 token（前台 /preview/draft/:token 渲染）
		author.POST("/preview/post", h.createPostPreview)

		// 附件
		author.GET("/attachments", h.listAttachments)
		author.POST("/attachments/upload", h.uploadAttachment)
		author.POST("/attachments/import-url", h.importAttachmentURL)
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
			wiki.GET("/projects", h.listWikiProjects)
			wiki.GET("/projects/:id", h.getWikiProject)
			wiki.POST("/projects", h.createWikiProject)
			wiki.PUT("/projects/:id", h.updateWikiProject)
			wiki.DELETE("/projects/:id", h.deleteWikiProject)
			wiki.GET("/:project/docs", h.listWikidocs)
			wiki.GET("/:project/docs/:slug", h.getWikidoc)
			wiki.POST("/:project/docs", h.createWikidoc)
			wiki.PUT("/:project/docs/:slug", h.updateWikidoc)
			wiki.DELETE("/:project/docs/:slug", h.deleteWikidoc)
		}

		// 专栏（Spec 52）
		author.GET("/topics", h.listTopics)
		author.POST("/topics", h.createTopic)
		author.PUT("/topics/:id", h.updateTopic)
		author.DELETE("/topics/:id", h.deleteTopic)
		author.GET("/notebooks", h.listNotebooks)
		author.POST("/notebooks", h.createNotebook)
		author.GET("/notebooks/:notebook", h.getNotebook)
		author.PUT("/notebooks/:notebook", h.updateNotebook)
		author.DELETE("/notebooks/:notebook", h.deleteNotebook)
		author.GET("/notebooks/:notebook/notes", h.listNotebookNotes)
		author.POST("/notebooks/:notebook/notes", h.createNotebookNote)
		author.GET("/notebooks/:notebook/notes/:id", h.getNotebookNote)
		author.PUT("/notebooks/:notebook/notes/:id", h.updateNotebookNote)
		author.DELETE("/notebooks/:notebook/notes/:id", h.deleteNotebookNote)
		author.POST("/notebooks/:notebook/notes/:id/publish", h.publishNotebookNote)
		author.PUT("/notebooks/:notebook/notes/:id/draft", h.draftNotebookNote)

		// 瞬间模块 API：模块停用时由 manifest 对齐的 Gate 返回 403。
		momentRoutes := author
		if h.modules != nil {
			momentRoutes = author.Group("", h.modules.Gate("moments"))
		}
		momentRoutes.GET("/moments", h.listMoments)
		momentRoutes.POST("/moments", h.createMoment)
		momentRoutes.PUT("/moments/:id", h.updateMoment)
		momentRoutes.DELETE("/moments/:id", h.deleteMoment)

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
		author.POST("/categories", h.createCategory)
		author.PUT("/categories/:id", h.updateCategory)
		author.DELETE("/categories/:id", h.deleteCategoryByID)
		author.GET("/tags", h.listTags)
		author.POST("/tags/merge", h.mergeTags)
		author.PUT("/tags/:name", h.renameTag)
		author.DELETE("/tags/:name", h.deleteTag)

		// 素材速查（插件库图标画廊数据源：icons.yml 全量 key + SVG）
		author.GET("/assets/icons", h.listAssetIcons)

		// 标签插件 catalog：以 Go parser 注册表为真值，供 Console SchemaForm/插件库消费。
		author.GET("/plugins/catalog", h.shortcodeCatalog)
		author.GET("/plugins/widgets/catalog", h.getStellarWidgetCatalog)
		author.POST("/plugins/shortcode/parse", h.shortcodeEdit)
		author.POST("/plugins/shortcode/serialize", h.shortcodeSerialize)

		// 模块 manifest 对 author 角色可读；启停写操作仍在 sys 组。
		author.GET("/modules", h.listModules)
	}

	// User Center: readers may inspect their account and upgrade; article and
	// attachment surfaces require author. The write handlers reuse the Batch 4
	// ownership checks and server-side author binding.
	uc := v1.Group("/uc", h.ucAuth())
	{
		uc.GET("/me", h.ucMe)
		uc.GET("/profile", h.handleGetProfile)
		uc.PUT("/profile", h.handleUpdateProfile)
		uc.PUT("/auth/password", h.changePassword)
		uc.POST("/profile/2fa/setup", h.handle2FASetup)
		uc.POST("/profile/2fa/confirm", h.handle2FAConfirm)
		uc.POST("/profile/2fa/disable", h.handle2FADisable)
		uc.GET("/profile/2fa/recovery-codes", h.handle2FARecoveryCodes)
		uc.GET("/profile/api-token", h.apiTokenStatus)
		uc.POST("/profile/api-token", h.generateAPIToken)
		uc.DELETE("/profile/api-token", h.revokeAPIToken)
		uc.GET("/notifications", h.listNotifications)
		uc.GET("/notifications/unread-count", h.unreadNotifications)
		uc.PUT("/notifications/:id/read", h.markNotificationRead)
		uc.PUT("/notifications/read-all", h.markAllNotificationsRead)
	}
	ucAuthor := uc.Group("", h.requirePermission(store.PermissionContentWrite))
	{
		ucAuthor.GET("/posts", h.listUCPosts)
		ucAuthor.GET("/posts/:slug", h.getUCPost)
		ucAuthor.POST("/posts", h.createUCPost)
		ucAuthor.PUT("/posts/:slug", h.updateUCPost)
		ucAuthor.DELETE("/posts/:slug", h.deletePost)
		ucAuthor.PUT("/posts/:slug/draft", h.saveUCDraft)
		ucAuthor.DELETE("/posts/:slug/draft", h.discardPostDraft)
		ucAuthor.POST("/posts/:slug/publish", h.publishUCPost)
		ucAuthor.POST("/posts/:slug/restore", h.restorePost)
		ucAuthor.DELETE("/posts/:slug/purge", h.purgePost)
		ucAuthor.PUT("/posts/:slug/ai-summary", h.updateAISummary)
		ucAuthor.POST("/revisions/:id/restore", h.restoreRevision)

		ucAuthor.GET("/attachments", h.listUCAttachments)
		ucAuthor.POST("/attachments/upload", h.uploadAttachment)
		ucAuthor.POST("/attachments/import-url", h.importAttachmentURL)
		ucAuthor.DELETE("/attachments/:id", h.deleteUCAttachment)
	}

	// 系统管理组（仅 admin，且站点必须已启用密码认证——安全审计 2026-08-16）：
	// 用户管理 / 登录日志 / 站点设置 / 功能模块 / 备份导出 / SEO 模块 / 友链管理 / AI 代理。
	sys := v1.Group("", h.apiAuth(), h.requireSysAdmin())
	{
		// 用户管理（多用户体系）：列表/创建/更新/重置密码/删除
		sys.GET("/users", h.listUsers)
		sys.POST("/users", h.createUser)
		sys.POST("/users/invitations", h.createRegistrationInvite)
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
		sys.GET("/settings/stellar", h.getStellarSettings)
		sys.PUT("/settings/stellar", h.updateStellarSettings)
		sys.POST("/settings/stellar/validate", h.validateStellarSettings)
		sys.PUT("/settings/:section", h.updateSection)
		sys.POST("/settings/notify/test", h.testNotify)
		// Feedback 服务连通性自测：探测草稿 api_base 的 /healthz（仅 admin，不回传响应体）
		sys.POST("/settings/feedback/check", h.checkFeedback)
		// 主题 YAML 编辑器（Spec 60）：style 段 YAML 读写 + 热更新
		sys.GET("/settings/style/yaml", h.getStyleYAML)
		sys.PUT("/settings/style/yaml", h.updateStyleYAML)

		// 功能模块管理（启停开关；manifest 读取位于 author 组）
		sys.PUT("/modules/:id", h.toggleModule)

		// 备份与导出（下载走 cookie 认证，Console 直接 <a href> 触发）
		sys.GET("/backup/export", h.exportBackup)
		sys.GET("/backup/export/markdown", h.exportPostsMarkdown)
		sys.POST("/backup/import/markdown", h.importPostsMarkdown)
		sys.GET("/backup/status", h.backupStatus)
		sys.POST("/backup/restore", h.restoreBackup)
		sys.POST("/backup/restore-snapshot", h.restoreSnapshot)
		sys.POST("/backup/restores", h.startRestore)
		sys.GET("/backup/restores", h.listRestoreTasks)
		sys.GET("/backup/restores/:id", h.getRestoreTask)

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
		sys.GET("/rag/status", h.ragStatus)
		sys.POST("/rag/ask", h.ragAsk)
		sys.POST("/rag/rebuild", h.ragRebuild)

		// 系统版本与更新（版本识别 / 检查更新）
		sys.GET("/system/version", h.systemVersion)
		sys.GET("/system/update/check", h.systemUpdateCheck)
		sys.GET("/plugins/runtime", h.listScriptPlugins)
		sys.POST("/plugins/runtime/hook", h.runScriptPluginHook)

		// 功能模块后台 API：仅 admin；每个已注册模块套自己的 Gate 中间件（禁用即 403）。
		if h.modules != nil {
			deps := module.Deps{Store: h.store, Posts: h.posts, Storage: h.storageManager, Site: h.site, Modules: h.modules}
			for _, m := range module.All() {
				id := m.Meta().ID
				grp := sys.Group("", h.modules.Gate(id))
				m.RegisterRoutes(grp, deps)
			}
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
			h.setAuthorizationContext(c, 0, store.RoleAdmin)
			c.Next()
			return
		}

		// Bearer Token 认证（脚本/第三方调用，优先级高于 cookie）
		if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			token := strings.TrimPrefix(auth, "Bearer ")
			user, err := h.store.GetUserByAPITokenHash(store.HashAPIToken(token))
			if err != nil || user == nil || user.Status == store.UserStatusBanned {
				AbortFail(c, http.StatusUnauthorized, "API Token 无效或已撤销")
				return
			}
			if !h.setAuthorizationContext(c, user.ID, normalizeRole(user.Role)) {
				return
			}
			c.Next()
			return
		}

		if cookie, err := c.Cookie(authCookieName); err == nil {
			if userID, authSecret, expiry, role, ok := h.validAuthCookie(cookie, cookieAudAdmin); ok {
				if !h.setAuthorizationContext(c, userID, role) {
					return
				}
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
		// Public author sessions use blog_user so the UC app can share the same
		// identity cookie. Account-level endpoints (profile, notifications, 2FA,
		// PAT and password) are safe to reuse; content/system groups still apply
		// their role gates below.
		if public := identity.CurrentUser(c); public.Verified && public.ID > 0 {
			if !h.setAuthorizationContext(c, public.ID, normalizeRole(public.Role)) {
				return
			}
			c.Next()
			return
		}
		AbortFail(c, http.StatusUnauthorized, "未登录或登录已过期")
	}
}

func (h *Handler) setAuthorizationContext(c *gin.Context, userID int64, role string) bool {
	c.Set("userID", userID)
	c.Set("userRole", role)
	permissions := []string{"*"}
	if userID > 0 {
		var err error
		permissions, err = h.store.GetUserPermissions(userID)
		if err != nil {
			AbortFail(c, http.StatusInternalServerError, "读取用户权限失败")
			return false
		}
	}
	c.Set("userPermissions", permissions)
	return true
}

func contextHasPermission(c *gin.Context, permission string) bool {
	value, _ := c.Get("userPermissions")
	permissions, _ := value.([]string)
	return store.HasPermission(permissions, permission)
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
	if user == nil || user.Status == store.UserStatusBanned {
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

// requirePermission is the table-driven authorization gate.
func (h *Handler) requirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !contextHasPermission(c, permission) {
			AbortFail(c, http.StatusForbidden, "无权限：缺少所需权限")
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
		if !contextHasPermission(c, store.PermissionSystemManage) {
			AbortFail(c, http.StatusForbidden, "无权限：缺少系统管理权限")
			return
		}
		c.Next()
	}
}
