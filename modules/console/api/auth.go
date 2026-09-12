package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/identity"
	"blog/internal/store"
)

const minPasswordLen = 6

// loginRequest 登录请求体（Spec 45：username 可选，空时回退旧版 admin 密码认证）。
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// --- 2FA 临时 Token 机制 ---

var pending2FA = struct {
	sync.RWMutex
	tokens map[string]pendingToken
}{tokens: make(map[string]pendingToken)}

type pendingToken struct {
	userID     int64
	authSecret string
	expiresAt  time.Time
}

func generate2FAToken(userID int64, authSecret string) (string, error) {
	return generate2FATokenFrom(rand.Reader, userID, authSecret)
}

func generate2FATokenFrom(reader io.Reader, userID int64, authSecret string) (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(reader, b); err != nil {
		return "", fmt.Errorf("read secure random bytes: %w", err)
	}
	token := base64.URLEncoding.EncodeToString(b)

	pending2FA.Lock()
	// Spec 61 阶段 3：惰性清理过期条目（沿用 loginBlocked 模式），防长期运行下 map 无限增长
	now := time.Now()
	for k, p := range pending2FA.tokens {
		if now.After(p.expiresAt) {
			delete(pending2FA.tokens, k)
		}
	}
	pending2FA.tokens[token] = pendingToken{
		userID:     userID,
		authSecret: authSecret,
		expiresAt:  now.Add(10 * time.Minute),
	}
	pending2FA.Unlock()

	return token, nil
}

func consume2FAToken(token string) (pendingToken, bool) {
	pending2FA.Lock()
	defer pending2FA.Unlock()

	p, ok := pending2FA.tokens[token]
	if !ok || time.Now().After(p.expiresAt) {
		delete(pending2FA.tokens, token) // 清理过期
		return pendingToken{}, false
	}
	delete(pending2FA.tokens, token) // 一次性使用
	return p, true
}

// 登录限流参数（P0-4）。
const (
	loginRateLimit = 5 * time.Minute // 失败计数窗口
	loginMaxFails  = 5               // 窗口内最大失败次数
	loginBlockTime = 5 * time.Minute // 触发限流后封锁时长
)

// login POST /api/v1/auth/login：校验密码，成功写签名 cookie 返回 JSON。
// Spec 45：支持 username 用户表认证 + TOTP 2FA，向后兼容无 username 时旧 admin 密码认证。
func (h *Handler) login(c *gin.Context) {
	ip := c.ClientIP()

	// P0-4：登录限流——同 IP 短窗口内失败超阈返回 429。
	if h.loginBlocked(ip) {
		Fail(c, http.StatusTooManyRequests, "尝试次数过多，请稍后再试")
		return
	}

	if !h.site.Load().Admin.AuthEnabled {
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "无需密码"})
		return
	}

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}

	var user *store.User

	// Spec 61 阶段 4：登录一律走 users 表 bcrypt 校验；无 username 时默认 admin 用户
	// （旧版明文密码比对与旧格式 cookie 已移除）。
	username := req.Username
	if username == "" {
		username = "admin"
	}
	var err error
	user, err = h.store.GetUserByUsername(username)
	if err != nil || user == nil {
		h.recordLoginFail(ip)
		h.logLogin(c, 0, username, ip, "fail", "password")
		Fail(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if err := store.CheckPassword(user.PasswordHash, req.Password); err != nil {
		h.recordLoginFail(ip)
		h.logLogin(c, user.ID, username, ip, "fail", "password")
		Fail(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if user.Status == store.UserStatusBanned {
		h.recordLoginFail(ip)
		h.logLogin(c, user.ID, username, ip, "fail", "password")
		Fail(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	// 安全审计 2026-08-16：console 登录端点拒绝 reader 角色——读者凭据不应换取
	// blog_admin 会话（否则开放注册账号即可进入全部 authed 组内容写端点）。
	canConsole, permissionErr := h.store.UserHasPermission(user.ID, store.PermissionContentWrite)
	if permissionErr != nil {
		Fail(c, http.StatusInternalServerError, "读取用户权限失败")
		return
	}
	if !canConsole {
		h.logLogin(c, user.ID, username, ip, "fail", "password")
		Fail(c, http.StatusForbidden, "读者账号请从前台登录")
		return
	}

	// 检查 TOTP
	if user != nil && user.TOTPSecret != "" {
		generateToken := h.generate2FA
		if generateToken == nil {
			generateToken = generate2FAToken
		}
		token, err := generateToken(user.ID, user.AuthSecret)
		if err != nil {
			Fail(c, http.StatusInternalServerError, "生成 2FA token 失败")
			return
		}
		c.JSON(http.StatusOK, gin.H{"require_2fa": true, "token": token})
		return
	}

	// 生成新格式 cookie
	h.clearLoginFail(ip)
	h.logLogin(c, user.ID, user.Username, ip, "success", "password")
	expiry := time.Now().Add(authCookieTTL).Unix()
	cookie := signAuthCookie(cookieAudAdmin, expiry, user.ID, user.AuthSecret)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(authCookieName, cookie, int(authCookieTTL.Seconds()), "/", "", h.cookieSecure(c), true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// logout POST /api/v1/auth/logout：清 cookie。
func (h *Handler) logout(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(authCookieName, "", -1, "/", "", h.cookieSecure(c), true)
	// UC 与公共身份共用 blog_user cookie；Console profile 的复用登出也应
	// 一并结束公共会话，避免跳转到 /uc 后仍被旧 cookie 自动登录。
	c.SetCookie(identity.UserCookieName, "", -1, "/", "", h.cookieSecure(c), true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handle2FA POST /api/v1/auth/2fa：两步登录第二步，验证 TOTP code 后下发签名 cookie。
func (h *Handler) handle2FA(c *gin.Context) {
	var req struct {
		Token string `json:"token"`
		Code  string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}

	pending, ok := consume2FAToken(req.Token)
	if !ok {
		h.logLogin(c, 0, "", c.ClientIP(), "fail", "2fa")
		Fail(c, http.StatusUnauthorized, "token 无效或已过期")
		return
	}

	user, err := h.store.GetUserByID(pending.userID)
	if err != nil || user == nil || user.Status == store.UserStatusBanned || subtle.ConstantTimeCompare([]byte(user.AuthSecret), []byte(pending.authSecret)) != 1 || !ValidateTOTP(user.TOTPSecret, req.Code) {
		h.logLogin(c, pending.userID, "", c.ClientIP(), "fail", "2fa")
		Fail(c, http.StatusUnauthorized, "验证码错误")
		return
	}

	// 安全审计 2026-08-16：与 login 一致，读者不经 console 2FA 链路换 admin cookie。
	canConsole, permissionErr := h.store.UserHasPermission(user.ID, store.PermissionContentWrite)
	if permissionErr != nil {
		Fail(c, http.StatusInternalServerError, "读取用户权限失败")
		return
	}
	if !canConsole {
		h.logLogin(c, user.ID, user.Username, c.ClientIP(), "fail", "2fa")
		Fail(c, http.StatusForbidden, "读者账号请从前台登录")
		return
	}

	h.clearLoginFail(c.ClientIP())
	h.logLogin(c, user.ID, user.Username, c.ClientIP(), "success", "2fa")
	expiry := time.Now().Add(authCookieTTL).Unix()
	cookie := signAuthCookie(cookieAudAdmin, expiry, user.ID, user.AuthSecret)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(authCookieName, cookie, int(authCookieTTL.Seconds()), "/", "", h.cookieSecure(c), true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type profileResponse struct {
	ID          int64    `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	Email       string   `json:"email"`
	Avatar      string   `json:"avatar"`
	Has2FA      bool     `json:"has_2fa"`
	Role        string   `json:"role"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

// handleGetProfile GET /api/v1/profile：返回当前用户资料。
func (h *Handler) handleGetProfile(c *gin.Context) {
	userID := c.GetInt64("userID")
	user, err := h.store.GetUserByID(userID)
	if err != nil || user == nil {
		Fail(c, http.StatusNotFound, "user not found")
		return
	}
	c.JSON(http.StatusOK, profileResponse{
		ID: user.ID, Username: user.Username, DisplayName: user.DisplayName,
		Email: user.Email, Avatar: user.Avatar, Has2FA: user.TOTPSecret != "",
		Role: normalizeRole(user.Role), Roles: user.Roles, Permissions: user.Permissions,
	})
}

// handleUpdateProfile PUT /api/v1/profile：更新当前用户资料。
func (h *Handler) handleUpdateProfile(c *gin.Context) {
	userID := c.GetInt64("userID")

	var req struct {
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Avatar      string `json:"avatar"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}

	user, err := h.store.GetUserByID(userID)
	if err != nil || user == nil {
		Fail(c, http.StatusNotFound, "user not found")
		return
	}

	user.DisplayName = req.DisplayName
	user.Email = req.Email
	user.Avatar = req.Avatar
	if err := h.store.UpdateUser(user); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handle2FASetup POST /api/v1/profile/2fa/setup：生成新 TOTP secret 与 8 个恢复码，
// 暂不写入 DB（等 confirm 验证通过后才启用）。
func (h *Handler) handle2FASetup(c *gin.Context) {
	userID := c.GetInt64("userID")
	user, err := h.store.GetUserByID(userID)
	if err != nil || user == nil {
		Fail(c, http.StatusNotFound, "user not found")
		return
	}

	secret := GenerateSecret()
	recoveryCodes := GenerateRecoveryCodes(8)
	c.JSON(http.StatusOK, gin.H{
		"secret":         secret,
		"auth_url":       TOTPAuthURL(secret, user.Username, "Stellar Blog"),
		"recovery_codes": recoveryCodes,
	})
}

// handle2FAConfirm POST /api/v1/profile/2fa/confirm：验证 TOTP code 通过后写入 secret 与恢复码启用 2FA。
func (h *Handler) handle2FAConfirm(c *gin.Context) {
	userID := c.GetInt64("userID")

	var req struct {
		Secret        string   `json:"secret"`
		Code          string   `json:"code"`
		RecoveryCodes []string `json:"recovery_codes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}

	if req.Secret == "" || !ValidateTOTP(req.Secret, req.Code) {
		Fail(c, http.StatusBadRequest, "验证码错误")
		return
	}
	if err := h.store.UpdateTOTPSecret(userID, req.Secret, req.RecoveryCodes); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handle2FADisable POST /api/v1/profile/2fa/disable：验证当前 TOTP code 通过后清除 2FA。
func (h *Handler) handle2FADisable(c *gin.Context) {
	userID := c.GetInt64("userID")
	user, err := h.store.GetUserByID(userID)
	if err != nil || user == nil {
		Fail(c, http.StatusNotFound, "user not found")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}

	if !ValidateTOTP(user.TOTPSecret, req.Code) {
		Fail(c, http.StatusBadRequest, "验证码错误")
		return
	}
	if err := h.store.UpdateTOTPSecret(userID, "", nil); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handle2FARecoveryCodes GET /api/v1/profile/2fa/recovery-codes：返回当前用户已保存的恢复码列表。
func (h *Handler) handle2FARecoveryCodes(c *gin.Context) {
	userID := c.GetInt64("userID")
	user, err := h.store.GetUserByID(userID)
	if err != nil || user == nil {
		Fail(c, http.StatusNotFound, "user not found")
		return
	}

	var codes []string
	if err := json.Unmarshal([]byte(user.TOTPRecoveryCodes), &codes); err != nil || codes == nil {
		codes = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"recovery_codes": codes})
}

type authCheckResponse struct {
	Authenticated           bool     `json:"authenticated"`
	PasswordRequired        bool     `json:"passwordRequired"`
	Role                    string   `json:"role"`
	Permissions             []string `json:"permissions"`
	ConsoleAccessDisallowed bool     `json:"console_access_disallowed,omitempty"`
}

// authCheck GET /api/v1/auth/check：返回当前登录状态（兼容新旧 cookie 格式）。
func (h *Handler) authCheck(c *gin.Context) {
	if !h.site.Load().Admin.AuthEnabled {
		c.JSON(http.StatusOK, authCheckResponse{Authenticated: true, PasswordRequired: false, Role: store.RoleAdmin, Permissions: []string{"*"}})
		return
	}
	if cookie, err := c.Cookie(authCookieName); err == nil {
		if userID, _, _, role, ok := h.validAuthCookie(cookie, cookieAudAdmin); ok {
			permissions, _ := h.store.GetUserPermissions(userID)
			c.JSON(http.StatusOK, authCheckResponse{Authenticated: true, PasswordRequired: true, Role: role, Permissions: permissions})
			return
		}
	}
	if public := identity.CurrentUser(c); public.Verified {
		permissions, _ := h.store.GetUserPermissions(public.ID)
		if store.HasPermission(permissions, store.PermissionContentWrite) && !store.HasPermission(permissions, store.PermissionConsoleAccess) {
			c.JSON(http.StatusOK, authCheckResponse{Authenticated: true, PasswordRequired: true, Role: public.Role, Permissions: permissions, ConsoleAccessDisallowed: true})
			return
		}
	}
	c.JSON(http.StatusOK, authCheckResponse{Authenticated: false, PasswordRequired: true, Role: "", Permissions: []string{}})
}

// changePassword PUT /api/v1/auth/password：Spec 45 迁移至 users 表。
// 从 context 获取 userID，查用户表校验旧密码，更新密码并轮换 auth_secret 使旧 cookie 失效。
// userID=0 时回退旧版 settings 表 admin 密码更新。
func (h *Handler) changePassword(c *gin.Context) {
	userID := c.GetInt64("userID")

	var req struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}

	// 安全审计 2026-08-16：新密码补最小长度校验（此前可设空密码，与
	// createUser/resetUserPassword 的 len>=6 约束对齐）。
	if len(req.New) < minPasswordLen {
		Fail(c, http.StatusBadRequest, "新密码长度至少 6 位")
		return
	}

	if userID > 0 {
		// 新流程：用户表
		user, err := h.store.GetUserByID(userID)
		if err != nil || user == nil {
			Fail(c, http.StatusUnauthorized, "用户不存在")
			return
		}
		if err := store.CheckPassword(user.PasswordHash, req.Current); err != nil {
			Fail(c, http.StatusUnauthorized, "当前密码错误")
			return
		}
		newHash, err := store.HashPassword(req.New)
		if err != nil {
			Fail(c, http.StatusInternalServerError, "密码处理失败")
			return
		}
		newAuthSecret := store.GenerateAuthSecret()
		if err := h.store.UpdatePassword(user.ID, newHash, newAuthSecret); err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		// Spec 61 阶段 4：userID=0 回退路径改走 users 表 admin（旧 settings 明文回写链路已删除）
		adminUser, err := h.store.GetUserByUsername("admin")
		if err != nil || adminUser == nil {
			Fail(c, http.StatusUnauthorized, "用户不存在")
			return
		}
		if err := store.CheckPassword(adminUser.PasswordHash, req.Current); err != nil {
			Fail(c, http.StatusUnauthorized, "当前密码错误")
			return
		}
		newHash, err := store.HashPassword(req.New)
		if err != nil {
			Fail(c, http.StatusInternalServerError, "密码处理失败")
			return
		}
		newAuthSecret := store.GenerateAuthSecret()
		if err := h.store.UpdatePassword(adminUser.ID, newHash, newAuthSecret); err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// 旧 cookie 已因签名 key 变化失效，顺手清掉客户端残留
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(authCookieName, "", -1, "/", "", h.cookieSecure(c), true)
	c.JSON(http.StatusOK, gin.H{"ok": true, "passwordRequired": req.New != ""})
}

// cookieSecure 由 site.URL 与当前请求共同推导：site.url 为 https 且请求经 HTTPS 到达
// （直连 TLS 或反代 X-Forwarded-Proto=https）时为 true。
// 按请求判定而非启动时定死：http://IP:port 调试访问签非 Secure，
// 避免浏览器拒存 Secure cookie 导致登录态刷新即失。
func (h *Handler) cookieSecure(c *gin.Context) bool {
	if !strings.HasPrefix(h.site.Load().URL, "https://") {
		return false
	}
	return c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
}

// loginBlocked 检查 IP 是否因连续失败超阈而被限流。顺便清理过期条目防 map 膨胀。
func (h *Handler) loginBlocked(ip string) bool {
	h.loginMu.Lock()
	defer h.loginMu.Unlock()
	now := time.Now()
	for k, v := range h.loginFails {
		if now.Sub(v.t) > loginBlockTime {
			delete(h.loginFails, k)
		}
	}
	if v, ok := h.loginFails[ip]; ok && v.n >= loginMaxFails && now.Sub(v.t) < loginBlockTime {
		return true
	}
	return false
}

// recordLoginFail 记录一次登录失败。
func (h *Handler) recordLoginFail(ip string) {
	h.loginMu.Lock()
	defer h.loginMu.Unlock()
	v := h.loginFails[ip]
	if time.Since(v.t) > loginRateLimit {
		v.n = 0 // 窗口过期重置
	}
	v.n++
	v.t = time.Now()
	h.loginFails[ip] = v
}

// clearLoginFail 登录成功后清零该 IP 的失败计数。
func (h *Handler) clearLoginFail(ip string) {
	h.loginMu.Lock()
	defer h.loginMu.Unlock()
	delete(h.loginFails, ip)
}

// logLogin 写一条登录日志（Wave 3a）。日志落库失败不阻塞登录主流程（静默降级）。
func (h *Handler) logLogin(c *gin.Context, userID int64, username, ip, status, source string) {
	if h.store == nil {
		return
	}
	ua := c.GetHeader("User-Agent")
	if len(ua) > 200 {
		ua = ua[:200] // UA 截断防超长
	}
	_ = h.store.CreateLoginLog(userID, username, ip, ua, status, source)
}

// listLoginLogs GET /api/v1/auth/login-logs：分页查询登录日志（认证组）。
// 支持 status 过滤（success/fail，空=全部）；时间倒序。
func (h *Handler) listLoginLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	status := strings.TrimSpace(c.Query("status"))

	items, total, err := h.store.ListLoginLogs(page, size, status)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if items == nil {
		items = []*store.LoginLog{}
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "size": size})
}

// apiTokenStatus GET /api/v1/profile/api-token：返回当前用户是否已启用 API Token。
func (h *Handler) apiTokenStatus(c *gin.Context) {
	userID := c.GetInt64("userID")
	user, err := h.store.GetUserByID(userID)
	if err != nil || user == nil {
		Fail(c, http.StatusNotFound, "user not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"has_token": user.APITokenHash != "", "created_at": user.CreatedAt})
}

// generateAPIToken POST /api/v1/profile/api-token：生成新 API Token。
// 明文仅本次响应返回一次（前端提示用户立即保存）；已有旧 token 先撤销再轮换。
func (h *Handler) generateAPIToken(c *gin.Context) {
	userID := c.GetInt64("userID")
	user, err := h.store.GetUserByID(userID)
	if err != nil || user == nil {
		Fail(c, http.StatusNotFound, "user not found")
		return
	}

	token := store.GenerateAPIToken()
	if err := h.store.SetAPITokenHash(userID, store.HashAPIToken(token)); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "token": token, "username": user.Username})
}

// revokeAPIToken DELETE /api/v1/profile/api-token：撤销 API Token（置空哈希，Bearer 立即失效）。
func (h *Handler) revokeAPIToken(c *gin.Context) {
	userID := c.GetInt64("userID")
	if err := h.store.SetAPITokenHash(userID, ""); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
