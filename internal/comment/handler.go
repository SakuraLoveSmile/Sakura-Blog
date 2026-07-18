// Package comment 自建评论系统：公开 API（/api/comments）、Markdown 渲染管道、邮件通知。
// 架构参考 Artalk（page_key 定位、两层嵌套），视觉采用 Halo 设计语言。
package comment

import (
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
	tmpl "blog/internal/template"
)

// 提交限制。
const (
	maxNickLen    = 32               // 昵称最大字符数（rune）
	maxContentLen = 2000             // 内容最大字符数（rune）
	maxLinkLen    = 200              // 网站链接最大字符数
	rateLimit     = 60 * time.Second // IP 限流：同 IP 60 秒 1 条
)

// Handler 评论公开 API 处理器。
type Handler struct {
	store     *store.Store
	cfg       *tmpl.CommentsConfig
	siteTitle string // 邮件主题用

	mu   sync.Mutex
	last map[string]time.Time // IP 限流：IP → 上次评论时间
}

// New 创建评论 handler。cfg 指向站点配置的 Comments 段（运行期只读）。
func New(s *store.Store, cfg *tmpl.CommentsConfig, siteTitle string) *Handler {
	return &Handler{
		store:     s,
		cfg:       cfg,
		siteTitle: siteTitle,
		last:      map[string]time.Time{},
	}
}

// Register 注册公开路由（同源 /api/comments）。
func (h *Handler) Register(r *gin.Engine) {
	g := r.Group("/api/comments")
	g.GET("", h.list)
	g.POST("", h.create)
}

// list GET /api/comments?page_key= → 评论树 JSON（顶级 + children 两层，仅已发布）。
func (h *Handler) list(c *gin.Context) {
	pageKey := strings.TrimSpace(c.Query("page_key"))
	if pageKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "page_key 不能为空"})
		return
	}
	comments, err := h.store.ListCommentsByPage(pageKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "comments": ToViewItems(comments, h.cfg)})
}

// createReq 评论提交体。Website 是蜜罐字段：表单中隐藏，正常用户留空，机器人填了即丢弃。
type createReq struct {
	PageKey  string `json:"page_key"`
	Nick     string `json:"nick"`
	Email    string `json:"email"`
	Link     string `json:"link"`
	Content  string `json:"content"`
	ParentID int64  `json:"parent_id"`
	Website  string `json:"website"`
}

// create POST /api/comments：校验 → 蜜罐 → 限流 → 渲染消毒 → 入库 → 异步邮件通知。
func (h *Handler) create(c *gin.Context) {
	var req createReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "请求格式错误"})
		return
	}

	req.Nick = strings.TrimSpace(req.Nick)
	req.Email = strings.TrimSpace(req.Email)
	req.Link = strings.TrimSpace(req.Link)
	req.Content = strings.TrimSpace(req.Content)
	req.PageKey = strings.TrimSpace(req.PageKey)

	// 基础校验
	if req.PageKey == "" || !strings.HasPrefix(req.PageKey, "/") {
		h.reject(c, "page_key 非法")
		return
	}
	if req.Nick == "" {
		h.reject(c, "昵称不能为空")
		return
	}
	if utf8.RuneCountInString(req.Nick) > maxNickLen {
		h.reject(c, "昵称过长（最多 32 字）")
		return
	}
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		h.reject(c, "邮箱格式不正确")
		return
	}
	if req.Content == "" {
		h.reject(c, "评论内容不能为空")
		return
	}
	if utf8.RuneCountInString(req.Content) > maxContentLen {
		h.reject(c, "评论过长（最多 2000 字）")
		return
	}
	if req.Link != "" {
		if !strings.HasPrefix(req.Link, "http://") && !strings.HasPrefix(req.Link, "https://") {
			req.Link = "https://" + req.Link
		}
		if utf8.RuneCountInString(req.Link) > maxLinkLen {
			h.reject(c, "网站链接过长")
			return
		}
	}

	// 蜜罐：隐藏字段被填 = 机器人，静默丢弃（返回成功假象，不暴露检测信号）
	if req.Website != "" {
		c.JSON(http.StatusOK, gin.H{"ok": true, "status": store.CommentApproved})
		return
	}

	// 博主身份匹配（邮箱与 admin_email 一致），博主豁免限流且直接发布
	isAdmin := h.cfg.AdminEmail != "" && strings.EqualFold(req.Email, h.cfg.AdminEmail)

	// IP 限流：同 IP 60 秒 1 条
	ip := c.ClientIP()
	if !isAdmin && !h.allow(ip) {
		c.JSON(http.StatusTooManyRequests, gin.H{"ok": false, "error": "评论太频繁，请稍后再试"})
		return
	}

	// 父评论校验 + root_id 计算（两层嵌套：回复子评论时挂到同一根下）
	var parent *store.Comment
	var rootID int64
	if req.ParentID != 0 {
		p, err := h.store.GetComment(req.ParentID)
		if err != nil || p == nil || p.PageKey != req.PageKey {
			h.reject(c, "被回复的评论不存在")
			return
		}
		parent = p
		if p.RootID == 0 {
			rootID = p.ID
		} else {
			rootID = p.RootID
		}
	}

	// Markdown 渲染 + bluemonday 消毒（包级共享管道）
	html, err := Render(req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "内容渲染失败"})
		return
	}

	status := store.CommentPending
	if !h.cfg.Moderation || isAdmin {
		status = store.CommentApproved
	}

	cm := &store.Comment{
		PageKey:     req.PageKey,
		Nick:        req.Nick,
		Email:       req.Email,
		Link:        req.Link,
		ContentMD:   req.Content,
		ContentHTML: html,
		ParentID:    req.ParentID,
		RootID:      rootID,
		Status:      status,
		IsAdmin:     isAdmin,
		IP:          ip,
		UA:          c.Request.UserAgent(),
	}
	if _, err := h.store.CreateComment(cm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "评论保存失败"})
		return
	}

	// 异步邮件通知（SMTP 未配置时内部跳过，失败仅记日志）
	go h.notify(cm, parent)

	resp := gin.H{"ok": true, "status": status}
	if status == store.CommentApproved {
		// 返回视图模型，前端无刷新插入列表
		resp["comment"] = ToViewItem(cm, h.cfg)
	}
	c.JSON(http.StatusOK, resp)
}

// NotifyReply 管理员后台回复后触发通知（异步）。admin 包调用，规则同 notify。
func (h *Handler) NotifyReply(cm, parent *store.Comment) {
	go h.notify(cm, parent)
}

// reject 400 参数错误统一出口。
func (h *Handler) reject(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": msg})
}

// allow IP 限流：同 IP 60 秒 1 条。顺便清理过期条目，防 map 无限增长。
func (h *Handler) allow(ip string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	for k, t := range h.last {
		if now.Sub(t) > rateLimit {
			delete(h.last, k)
		}
	}
	if t, ok := h.last[ip]; ok && now.Sub(t) < rateLimit {
		return false
	}
	h.last[ip] = now
	return true
}
