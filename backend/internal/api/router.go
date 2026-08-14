// Package api 提供 RESTful JSON API（/api/v1），供 Vue Console SPA 管理后台调用。
// 认证复用 admin 包的 HMAC 签名 cookie 机制，未认证返回 401 JSON 而非重定向。
package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
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
	"blog/internal/friends"
	ghpkg "blog/internal/github"
	"blog/internal/linkwatch"
	"blog/internal/module"
	"blog/internal/notify"
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

	// renderSitePreview 设置页「前端预览」渲染回调（cmd/server 注入，nil 时端点 501）。
	renderSitePreview func(sections map[string]json.RawMessage, page string) (string, error)
	// onContentChanged 内容写路径回调（cmd/server 注入，可空）：置脏内容派生缓存。
	onContentChanged func()

	// 登录限流（P0-4）
	loginMu    sync.Mutex
	loginFails map[string]struct {
		n int
		t time.Time
	}

	// PAT 限流（每令牌 60 次/分钟，与登录限流同模式）
	patMu   sync.Mutex
	patHits map[int64]patWindow
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
	// OnContentChanged 内容写路径回调（Spec 61 阶段 5）：wikidoc/notebook/note/设置保存后
	// 通知 cmd/server 置脏 /search.json 与 /sitemap.xml 缓存，可空。
	OnContentChanged func()
	UploadDir        string
	MaxUploadMB      int
	LinkWatch        *linkwatch.Watcher
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
		onContentChanged:  cfg.OnContentChanged,
		uploadDir:         cfg.UploadDir,
		maxUpload:         int64(maxMB) << 20,
		linkWatch:         cfg.LinkWatch,
		loginFails: make(map[string]struct {
			n int
			t time.Time
		}),
		patHits: make(map[int64]patWindow),
	}
}

// SetOnLinksChanged 注册友链变更回调（如 friends 聚合器的 Refresh），友链增删改后调用以即时刷新 /friends。
func (h *Handler) SetOnLinksChanged(fn func()) {
	h.onLinksChanged = fn
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

	// 读者身份中间件（挂在引擎根级：comment handler 的 /api/comments、SSR handler 的文章页均受益）
	r.Use(h.currentUserMiddleware())

	// 公共身份端点（与后台 /api/v1/auth 解耦，cookie 名独立）
	auth := r.Group("/api/auth")
	{
		auth.POST("/register", h.publicRegister)
		auth.POST("/login", h.publicLogin)
		auth.GET("/me", h.publicMe)
		auth.POST("/logout", h.publicLogout)
	}

	h.Register(r)

	// 友链动态聚合：/api/friends-aggregated 供 /friends 页异步渲染；
	// 友链在 console 增删改后触发聚合器即时重建缓存。
	if d.Friends != nil {
		d.Friends.Register(r)
		h.SetOnLinksChanged(d.Friends.Refresh)
	}

	// GitHub 代理端点（Spec 21）：/api/gh/repo（直连 api.github.com）。
	// Spec 54：同一 client 注入 Handler 供友链导入（POST /links/import）拉取 issues。
	if d.GH != nil {
		h.gh = d.GH
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

		// API Token（Wave 3a）：生成/撤销 Bearer 认证凭证
		authed.GET("/profile/api-token", h.apiTokenStatus)
		authed.POST("/profile/api-token", h.generateAPIToken)
		authed.DELETE("/profile/api-token", h.revokeAPIToken)

		// 站内通知中心（Wave 3b）：未读优先分页 + 单条/全部已读 + 未读数
		authed.GET("/notifications", h.listNotifications)
		authed.GET("/notifications/unread-count", h.unreadNotifications)
		authed.PUT("/notifications/:id/read", h.markNotificationRead)
		authed.PUT("/notifications/read-all", h.markAllNotificationsRead)

		// 仪表盘
		authed.GET("/dashboard/stats", h.dashboardStats)
		authed.GET("/dashboard/recent-comments", h.dashboardRecentComments)
		authed.GET("/dashboard/recent-posts", h.dashboardRecentPosts)
		authed.GET("/dashboard/categories", h.dashboardCategories)
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
		authed.PUT("/comments/:id/status", h.statusComment)
		authed.DELETE("/comments/:id", h.deleteComment)
		authed.POST("/comments/reply", h.replyComment)
		authed.POST("/comments/batch", h.batchComments)

		// Markdown 预览（编辑器实时预览，复用前台 goldmark 渲染管线）
		authed.POST("/preview/markdown", h.previewMarkdown)
		// 站点配置预览（设置页所见即所得：草稿分段 → 前台整页 HTML）
		authed.POST("/preview/site", h.previewSite)
		// 发布前预览：存当前表单快照，返回 token（前台 /preview/draft/:token 渲染）
		authed.POST("/preview/post", h.createPostPreview)

		// 组件库预览 Fixture 端点（Spec 63）：只服务预览，不落库、不发外部请求
		authed.GET("/preview/rating", h.previewRatingFixture)
		authed.POST("/preview/rating", h.previewRatingFixtureSubmit)
		authed.GET("/preview/vote/info", h.previewVoteFixtureInfo)
		authed.POST("/preview/vote/update", h.previewVoteFixtureUpdate)
		authed.GET("/preview/friends", h.previewFriendsFixture)
		authed.GET("/preview/sites", h.previewSitesFixture)
		authed.GET("/preview/md", h.previewMDFixture)

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

		// 友链
		authed.GET("/links", h.listLinks)
		authed.POST("/links", h.createLink)
		authed.PUT("/links/:id", h.updateLink)
		authed.DELETE("/links/:id", h.deleteLink)
		authed.POST("/links/check", h.checkLinks)
		authed.POST("/links/:id/snapshot", h.snapshotLink)
		authed.POST("/links/import", h.importLinks)

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

		// 素材速查（插件库图标画廊数据源：icons.yml 全量 key + SVG）
		authed.GET("/assets/icons", h.listAssetIcons)

	}

	// 系统管理组（仅 admin）：用户管理 / 登录日志 / 站点设置 / 功能模块 / 备份导出 / SEO 模块。
	sys := v1.Group("", h.apiAuth(), h.requireRole(store.RoleAdmin))
	{
		// 用户管理（多用户体系）：列表/创建/更新/重置密码/删除
		sys.GET("/users", h.listUsers)
		sys.POST("/users", h.createUser)
		sys.PUT("/users/:id", h.updateUser)
		sys.PUT("/users/:id/password", h.resetUserPassword)
		sys.DELETE("/users/:id", h.deleteUser)

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

	// 公开只读 Content API（PAT 令牌认证，与 cookie 主链路完全分离）：
	// 无论站点密码是否配置都必须校验 Bearer，无任何写端点。
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
			if userID, authSecret, expiry, role, ok := h.validAuthCookie(cookie); ok {
				c.Set("userID", userID)
				c.Set("userRole", role)
				// 滑动续期：新格式 cookie 剩余有效期不足一半时重新签发（旧格式无 authSecret 不续期）
				if userID > 0 && authSecret != "" && expiry-time.Now().Unix() < int64(authCookieTTL.Seconds())/2 {
					newExpiry := time.Now().Add(authCookieTTL).Unix()
					c.SetSameSite(http.SameSiteLaxMode)
					c.SetCookie(authCookieName, signAuthCookie(newExpiry, userID, authSecret), int(authCookieTTL.Seconds()), "/", "", h.cookieSecure(c), true)
				}
				c.Next()
				return
			}
		}
		AbortFail(c, http.StatusUnauthorized, "未登录或登录已过期")
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

// validAuthCookie 解析并验证 cookie（Spec 61 阶段 4 起仅新格式）：
// 3 段 expiry.userID.sig，查 users 表取 auth_secret 验证 HMAC 签名。
// 旧格式（2 段明文密码 HMAC）已随 admin 明文密码移除而删除。
// 返回 (userID, authSecret, expiry, role, ok)；role 为登录用户角色。
func (h *Handler) validAuthCookie(value string) (int64, string, int64, string, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return 0, "", 0, "", false
	}
	expiry, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().Unix() > expiry {
		return 0, "", 0, "", false
	}
	userID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, "", 0, "", false
	}
	user, _ := h.store.GetUserByID(userID)
	if user == nil {
		return 0, "", 0, "", false
	}
	msg := fmt.Sprintf("%d.%d", expiry, userID)
	mac := hmac.New(sha256.New, []byte(user.AuthSecret))
	mac.Write([]byte(msg))
	expected := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(parts[2]), []byte(expected)) == 1 {
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

// requireRole 角色校验中间件：当前登录用户需具备指定角色，否则 403。
func (h *Handler) requireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		got, _ := c.Get("userRole")
		if got != role {
			AbortFail(c, http.StatusForbidden, "无权限：需要管理员角色")
			return
		}
		c.Next()
	}
}
