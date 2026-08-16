// Package comment 自建评论系统：公开 API（/api/comments）、Markdown 渲染管道、邮件通知。
// 架构参考 Artalk（page_key 定位、两层嵌套），视觉采用 Halo 设计语言。
package comment

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"blog/internal/notify"
	"blog/internal/store"
	"blog/internal/textutil"
	tmpl "blog/internal/template"
)

// 提交限制。
const (
	maxNickLen    = 32               // 昵称最大字符数（rune）
	maxContentLen = 2000             // 内容最大字符数（rune）
	maxLinkLen    = 200              // 网站链接最大字符数
	maxEmailLen   = 254              // 邮箱最大长度（RFC 5321 上限）
	rateLimit     = 60 * time.Second // IP 限流：同 IP 60 秒 1 条
	ipHashSalt    = "blog-comment-like-v1"
)

// AllowedBoostEmojis 是公开 Boost API 与前端保持一致的固定表情集。
var AllowedBoostEmojis = []string{"👍", "❤️", "😄", "🎉", "😕", "🚀"}

var allowedBoostEmojiSet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(AllowedBoostEmojis))
	for _, emoji := range AllowedBoostEmojis {
		m[emoji] = struct{}{}
	}
	return m
}()

// 评论图片上传限制。
const (
	maxUploadSize  = 5 << 20          // 单图 5MB
	uploadInterval = 10 * time.Second // 同 IP 上传间隔
	uploadBurst    = 10               // 窗口内单 IP 最大上传次数
	uploadWindow   = 10 * time.Minute // 限流窗口
)

// Handler 评论公开 API 处理器。
type Handler struct {
	store *store.Store
	// site 站点配置原子指针（Spec 61 阶段 3）：评论配置/站点标题随 RebuildSite 复制-交换实时可见
	site      *atomic.Pointer[tmpl.Site]
	notifier  *notify.Service // 站内通知服务（Wave 3b），nil 时跳过站内通知/Webhook
	uploadDir string          // 评论图片上传根目录，空=禁用上传端点

	mu   sync.Mutex
	last map[string]time.Time // IP 限流：IP → 上次评论时间

	uploadMu   sync.Mutex
	uploadLast map[string]time.Time   // IP → 上次上传时间（间隔限流）
	uploadHist map[string][]time.Time // IP → 窗口内上传记录（总量限流）

	// notifySem 邮件通知并发上限（Spec 62：评论刷屏时逐条 go h.notify 无界增长，
	// 用有缓冲信号量限制并发，满则丢弃本轮通知并记日志）。
	notifySem chan struct{}
}

// New 创建评论 handler。site 为站点配置原子指针（Comments 段与站点标题经 .Load() 读取）。
func New(s *store.Store, site *atomic.Pointer[tmpl.Site]) *Handler {
	return &Handler{
		store:      s,
		site:       site,
		last:       map[string]time.Time{},
		uploadLast: map[string]time.Time{},
		uploadHist: map[string][]time.Time{},
		notifySem:  make(chan struct{}, commentNotifyConcurrency),
	}
}

// SetUploadDir 注入评论图片上传根目录（未注入时上传端点返回 404）。
func (h *Handler) SetUploadDir(dir string) { h.uploadDir = dir }

// SetNotifier 注入站内通知服务（Wave 3b）。未注入时评论通知仅发邮件。
func (h *Handler) SetNotifier(n *notify.Service) {
	h.notifier = n
}

// Register 注册公开路由（同源 /api/comments）。
func (h *Handler) Register(r *gin.Engine) {
	g := r.Group("/api/comments")
	g.GET("", h.list)
	g.POST("", h.create)
	g.POST("/:id/like", h.like)
	g.POST("/:id/boost", h.boost)
	g.DELETE("/:id/boost", h.unboost)
	g.POST("/upload", h.uploadImage)
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
	cfg := h.site.Load().Comments
	boosts, err := h.store.ListBoostsByPage(pageKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "查询失败"})
		return
	}
	identity, userID, ip := h.boostViewer(c, "", "")
	c.JSON(http.StatusOK, gin.H{"ok": true, "comments": ToViewItems(comments, &cfg, BoostViewOptions{
		Boosts: boosts, Identity: identity, ViewerUserID: userID, ViewerIP: ip,
	})})
}

type boostReq struct {
	Emoji string `json:"emoji"`
	Text  string `json:"text"`
	Nick  string `json:"nick"`
	Email string `json:"email"`
}

// boostViewer 统一解析登录/匿名 Boost 身份。匿名身份只使用 IP hash 去重，
// 昵称和邮箱作为发布者快照写入 Boost，不参与 identity key。
func (h *Handler) boostViewer(c *gin.Context, nick, email string) (identity string, userID int64, ip string) {
	ip = c.ClientIP()
	sum := sha256.Sum256([]byte(ip + ipHashSalt))
	ipHash := hex.EncodeToString(sum[:])
	if v, ok := c.Get(ctxKeyCurrentUser); ok {
		if cu, ok := v.(*store.CurrentUser); ok && cu.ID > 0 {
			return "u:" + strconv.FormatInt(cu.ID, 10), cu.ID, ip
		}
	}
	return "ip:" + ipHash, 0, ip
}

func (h *Handler) boostMeta(c *gin.Context, req boostReq) (string, store.BoostMeta, int64, error) {
	identity, userID, ip := h.boostViewer(c, req.Nick, req.Email)
	sum := sha256.Sum256([]byte(ip + ipHashSalt))
	ipHash := hex.EncodeToString(sum[:])
	if v, ok := c.Get(ctxKeyCurrentUser); ok {
		if cu, ok := v.(*store.CurrentUser); ok && cu.ID > 0 {
			nick := strings.TrimSpace(cu.Nickname)
			if nick == "" {
				nick = strings.TrimSpace(cu.Username)
			}
			return identity, store.BoostMeta{UserID: userID, Nick: nick, Email: strings.TrimSpace(cu.Email), IP: ip, IPHash: ipHash}, userID, nil
		}
	}
	req.Nick = strings.TrimSpace(req.Nick)
	req.Email = strings.TrimSpace(req.Email)
	if req.Nick == "" {
		return "", store.BoostMeta{}, 0, errors.New("昵称不能为空")
	}
	if utf8.RuneCountInString(req.Nick) > maxNickLen {
		return "", store.BoostMeta{}, 0, errors.New("昵称过长（最多 32 字）")
	}
	// 安全审计 2026-08-16：net/mail 强校验替代"含 @ 即可"（拒 CRLF/控制字符）——
	// 该邮箱会经回复通知拼入 To: 头与 SMTP Rcpt 命令，弱校验构成邮件头注入。
	if !textutil.ValidEmail(req.Email, maxEmailLen) {
		return "", store.BoostMeta{}, 0, errors.New("邮箱格式不正确")
	}
	return identity, store.BoostMeta{Nick: req.Nick, Email: req.Email, IP: ip, IPHash: ipHash}, 0, nil
}

// boost POST /api/comments/:id/boost 创建或更新当前身份的 Boost。
func (h *Handler) boost(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "非法评论 id"})
		return
	}
	var req boostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "请求格式错误"})
		return
	}
	req.Emoji = strings.TrimSpace(req.Emoji)
	req.Text = strings.TrimSpace(req.Text)
	if _, ok := allowedBoostEmojiSet[req.Emoji]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "表情不在允许列表中"})
		return
	}
	if utf8.RuneCountInString(req.Text) > 16 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "回应文字最多 16 字"})
		return
	}
	identity, meta, _, err := h.boostMeta(c, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": err.Error()})
		return
	}
	existing, err := h.store.HasBoost(id, identity)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "回应失败"})
		return
	}
	if !existing && !h.allow("boost:"+c.ClientIP()) {
		c.JSON(http.StatusTooManyRequests, gin.H{"ok": false, "error": "操作太频繁，请稍后再试"})
		return
	}
	boost, created, err := h.store.AddBoost(id, identity, req.Emoji, req.Text, meta)
	if errors.Is(err, store.ErrCommentNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "评论不存在"})
		return
	}
	if errors.Is(err, store.ErrBoostSelf) {
		c.JSON(http.StatusForbidden, gin.H{"ok": false, "error": "不能回应自己的评论"})
		return
	}
	if errors.Is(err, store.ErrBoostLimit) {
		c.JSON(http.StatusConflict, gin.H{"ok": false, "error": "这条评论的回应已满"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "回应失败"})
		return
	}
	cfg := h.site.Load().Comments
	item := boostItemFromStore(boost, &cfg, identity)
	c.JSON(http.StatusOK, gin.H{"ok": true, "created": created, "boost": item})
}

// unboost DELETE /api/comments/:id/boost 删除当前身份自己的 Boost。
func (h *Handler) unboost(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "非法评论 id"})
		return
	}
	identity, _, _ := h.boostViewer(c, "", "")
	removed, err := h.store.DeleteBoost(id, identity)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "删除回应失败"})
		return
	}
	if !removed {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "未找到自己的回应"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "removed": true})
}

// like POST /api/comments/:id/like → IP 去重点赞，返回 {ok, likes, liked}。
func (h *Handler) like(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "非法评论 id"})
		return
	}

	ip := c.ClientIP()
	sum := sha256.Sum256([]byte(ip + ipHashSalt))
	ipHash := hex.EncodeToString(sum[:])

	// 复用 allow(ip)：限流窗口内仍允许「已赞过」回读（liked:false），避免挡住重复点赞约定。
	if !h.allow(ip) {
		has, likes, err := h.store.HasCommentLike(id, ipHash)
		if errors.Is(err, store.ErrCommentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "评论不存在"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "点赞失败"})
			return
		}
		if has {
			c.JSON(http.StatusOK, gin.H{"ok": true, "likes": likes, "liked": false})
			return
		}
		c.JSON(http.StatusTooManyRequests, gin.H{"ok": false, "error": "操作太频繁，请稍后再试"})
		return
	}

	liked, likes, err := h.store.LikeComment(id, ipHash)
	if errors.Is(err, store.ErrCommentNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "评论不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "点赞失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "likes": likes, "liked": liked})
}

// ctxKeyCurrentUser 与 api 包共用的 Gin context key（*store.CurrentUser）。
const ctxKeyCurrentUser = "current_user"

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

	// 已登录用户：昵称/邮箱/验证状态强制用账户资料，防冒名
	var userID int64
	var verified bool
	if v, ok := c.Get(ctxKeyCurrentUser); ok {
		if cu, ok := v.(*store.CurrentUser); ok && cu.ID > 0 {
			userID = cu.ID
			verified = true
			if cu.Nickname != "" {
				req.Nick = cu.Nickname
			} else if cu.Username != "" {
				req.Nick = cu.Username
			}
			if cu.Email != "" {
				req.Email = cu.Email
			}
			// 个人网站：注册用户暂无扩展字段，沿用用户输入或留空
		}
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
	// 安全审计 2026-08-16：net/mail 强校验（拒 CRLF），防回复通知 To: 头注入。
	if !textutil.ValidEmail(req.Email, maxEmailLen) {
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
	isAdmin := h.site.Load().Comments.AdminEmail != "" && strings.EqualFold(req.Email, h.site.Load().Comments.AdminEmail)
	// 已注册管理员/作者也视为博主身份（若邮箱匹配 admin_email 已覆盖）
	if userID > 0 && (verified && !isAdmin) {
		// 非 admin 邮箱但已登录：保持普通 verified 用户，isAdmin 仍由 admin_email 判定
	}

	// IP 限流：同 IP 60 秒 1 条
	ip := c.ClientIP()
	if !isAdmin && !verified && !h.allow(ip) {
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
	if !h.site.Load().Comments.Moderation || isAdmin {
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
		UserID:      userID,
		Verified:    verified,
		IP:          ip,
		UA:          c.Request.UserAgent(),
	}
	if _, err := h.store.CreateComment(cm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "评论保存失败"})
		return
	}

	// 异步邮件通知（SMTP 未配置时内部跳过，失败仅记日志；并发受 notifySem 限流）
	h.launchNotify(cm, parent)

	resp := gin.H{"ok": true, "status": status}
	if status == store.CommentApproved {
		// 返回视图模型，前端无刷新插入列表
		cfg := h.site.Load().Comments
		resp["comment"] = ToViewItem(cm, &cfg)
	}
	c.JSON(http.StatusOK, resp)
}

// NotifyReply 管理员后台回复后触发通知（异步）。admin 包调用，规则同 notify。
func (h *Handler) NotifyReply(cm, parent *store.Comment) {
	h.launchNotify(cm, parent)
}

// commentNotifyConcurrency 邮件通知最大并发数（Spec 62）。
const commentNotifyConcurrency = 5

// launchNotify 限流启动异步通知：并发满时丢弃本轮并记日志（评论刷屏时防 goroutine 无界增长）。
func (h *Handler) launchNotify(cm, parent *store.Comment) {
	select {
	case h.notifySem <- struct{}{}:
		go func() {
			defer func() { <-h.notifySem }()
			h.notify(cm, parent)
		}()
	default:
		log.Printf("comment notify: 通知并发已满(%d)，丢弃本轮通知", commentNotifyConcurrency)
	}
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

// allowUpload 上传限流：同 IP 间隔 10s + 窗口内总量上限，双 map 顺手清理过期条目。
func (h *Handler) allowUpload(ip string) bool {
	h.uploadMu.Lock()
	defer h.uploadMu.Unlock()
	now := time.Now()
	if t, ok := h.uploadLast[ip]; ok && now.Sub(t) < uploadInterval {
		return false
	}
	hist := h.uploadHist[ip]
	fresh := hist[:0]
	for _, t := range hist {
		if now.Sub(t) < uploadWindow {
			fresh = append(fresh, t)
		}
	}
	if len(fresh) >= uploadBurst {
		return false
	}
	h.uploadHist[ip] = append(fresh, now)
	h.uploadLast[ip] = now
	return true
}

// sniffImageExt 魔数识别图片类型（不信任扩展名/MIME），返回存储用扩展名。
func sniffImageExt(head []byte) (string, bool) {
	switch {
	case len(head) >= 8 && head[0] == 0x89 && head[1] == 'P' && head[2] == 'N' && head[3] == 'G':
		return ".png", true
	case len(head) >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF:
		return ".jpg", true
	case len(head) >= 6 && (strings.HasPrefix(string(head[:6]), "GIF87a") || strings.HasPrefix(string(head[:6]), "GIF89a")):
		return ".gif", true
	case len(head) >= 12 && string(head[0:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		return ".webp", true
	}
	return "", false
}

// randHex 生成 n 位随机十六进制文件名片段。
func randHex(n int) string {
	b := make([]byte, (n+1)/2)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))[:n]
	}
	return hex.EncodeToString(b)[:n]
}

// uploadImage POST /api/comments/upload：评论图片上传（公开但限流），
// 魔数校验 png/jpeg/gif/webp，存 uploadDir/yyyy/mm/ 随机名，返回 {ok, url}。
func (h *Handler) uploadImage(c *gin.Context) {
	if h.uploadDir == "" {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "上传未启用"})
		return
	}
	if !h.allowUpload(c.ClientIP()) {
		c.JSON(http.StatusTooManyRequests, gin.H{"ok": false, "error": "上传太频繁，请稍后再试"})
		return
	}
	// Spec 61 阶段 3：解析 multipart 前先限制请求体总大小，超大请求在解析阶段即被拒绝（不落临时文件）
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize+64<<10)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "图片不能超过 5MB"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "未选择文件"})
		return
	}
	defer file.Close()
	if header.Size > maxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "图片不能超过 5MB"})
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxUploadSize+1))
	if err != nil || len(data) > maxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "图片不能超过 5MB"})
		return
	}
	ext, ok := sniffImageExt(data)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "仅支持 png/jpeg/gif/webp 图片"})
		return
	}
	relPath := filepath.Join(time.Now().Format("2006/01"), randHex(16)+ext)
	absPath := filepath.Join(h.uploadDir, relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "创建目录失败"})
		return
	}
	if err := os.WriteFile(absPath, data, 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "写入文件失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "url": "/uploads/" + filepath.ToSlash(relPath)})
}
