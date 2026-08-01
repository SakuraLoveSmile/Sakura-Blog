package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/config"
	"blog/internal/store"
)

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
	userID    int64
	expiresAt time.Time
}

func generate2FAToken(userID int64) string {
	b := make([]byte, 32)
	rand.Read(b)
	token := base64.URLEncoding.EncodeToString(b)

	pending2FA.Lock()
	pending2FA.tokens[token] = pendingToken{
		userID:    userID,
		expiresAt: time.Now().Add(10 * time.Minute),
	}
	pending2FA.Unlock()

	return token
}

func consume2FAToken(token string) (int64, bool) {
	pending2FA.Lock()
	defer pending2FA.Unlock()

	p, ok := pending2FA.tokens[token]
	if !ok || time.Now().After(p.expiresAt) {
		delete(pending2FA.tokens, token) // 清理过期
		return 0, false
	}
	delete(pending2FA.tokens, token) // 一次性使用
	return p.userID, true
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
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "尝试次数过多，请稍后再试"})
		return
	}

	password := h.site.Admin.Password
	if password == "" {
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "无需密码"})
		return
	}

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	var user *store.User

	if req.Username != "" {
		// 新流程：用户表认证
		var err error
		user, err = h.store.GetUserByUsername(req.Username)
		if err != nil || user == nil {
			h.recordLoginFail(ip)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
			return
		}
		if err := store.CheckPassword(user.PasswordHash, req.Password); err != nil {
			h.recordLoginFail(ip)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
			return
		}
	} else {
		// 向后兼容：无 username 时回退旧 admin 密码验证
		if subtle.ConstantTimeCompare([]byte(req.Password), []byte(password)) != 1 {
			h.recordLoginFail(ip)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
			return
		}
		// 旧密码匹配，检查 users 表是否有 admin 用户
		adminUser, _ := h.store.GetUserByUsername("admin")
		if adminUser != nil {
			user = adminUser // 使用 users 表 admin 用户（含 TOTP 检查）
		} else {
			// users 表为空，使用旧 cookie 格式
			h.clearLoginFail(ip)
			expiry := time.Now().Add(authCookieTTL).Unix()
			mac := hmac.New(sha256.New, []byte(password))
			mac.Write([]byte(strconv.FormatInt(expiry, 10)))
			value := strconv.FormatInt(expiry, 10) + "." + hex.EncodeToString(mac.Sum(nil))
			c.SetSameSite(http.SameSiteLaxMode)
			c.SetCookie(authCookieName, value, int(authCookieTTL.Seconds()), "/", "", h.cookieSecure(c), true)
			c.JSON(http.StatusOK, gin.H{"ok": true})
			return
		}
	}

	// 检查 TOTP
	if user != nil && user.TOTPSecret != "" {
		token := generate2FAToken(user.ID)
		c.JSON(http.StatusOK, gin.H{"require_2fa": true, "token": token})
		return
	}

	// 生成新格式 cookie
	h.clearLoginFail(ip)
	expiry := time.Now().Add(authCookieTTL).Unix()
	cookie := signAuthCookie(expiry, user.ID, user.AuthSecret)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(authCookieName, cookie, int(authCookieTTL.Seconds()), "/", "", h.cookieSecure(c), true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// logout POST /api/v1/auth/logout：清 cookie。
func (h *Handler) logout(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(authCookieName, "", -1, "/", "", h.cookieSecure(c), true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handle2FA POST /api/v1/auth/2fa：两步登录第二步，验证 TOTP code 后下发签名 cookie。
func (h *Handler) handle2FA(c *gin.Context) {
	var req struct {
		Token string `json:"token"`
		Code  string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	userID, ok := consume2FAToken(req.Token)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token 无效或已过期"})
		return
	}

	user, err := h.store.GetUserByID(userID)
	if err != nil || user == nil || !ValidateTOTP(user.TOTPSecret, req.Code) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "验证码错误"})
		return
	}

	h.clearLoginFail(c.ClientIP())
	expiry := time.Now().Add(authCookieTTL).Unix()
	cookie := signAuthCookie(expiry, user.ID, user.AuthSecret)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(authCookieName, cookie, int(authCookieTTL.Seconds()), "/", "", h.cookieSecure(c), true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleGetProfile GET /api/v1/profile：返回当前用户资料。
func (h *Handler) handleGetProfile(c *gin.Context) {
	userID := c.GetInt64("userID")
	user, err := h.store.GetUserByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":           user.ID,
		"username":     user.Username,
		"display_name": user.DisplayName,
		"email":        user.Email,
		"avatar":       user.Avatar,
		"has_2fa":      user.TOTPSecret != "",
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	user, err := h.store.GetUserByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	user.DisplayName = req.DisplayName
	user.Email = req.Email
	user.Avatar = req.Avatar
	if err := h.store.UpdateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	if req.Secret == "" || !ValidateTOTP(req.Secret, req.Code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码错误"})
		return
	}
	if err := h.store.UpdateTOTPSecret(userID, req.Secret, req.RecoveryCodes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handle2FADisable POST /api/v1/profile/2fa/disable：验证当前 TOTP code 通过后清除 2FA。
func (h *Handler) handle2FADisable(c *gin.Context) {
	userID := c.GetInt64("userID")
	user, err := h.store.GetUserByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	if !ValidateTOTP(user.TOTPSecret, req.Code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码错误"})
		return
	}
	if err := h.store.UpdateTOTPSecret(userID, "", nil); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handle2FARecoveryCodes GET /api/v1/profile/2fa/recovery-codes：返回当前用户已保存的恢复码列表。
func (h *Handler) handle2FARecoveryCodes(c *gin.Context) {
	userID := c.GetInt64("userID")
	user, err := h.store.GetUserByID(userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	var codes []string
	if err := json.Unmarshal([]byte(user.TOTPRecoveryCodes), &codes); err != nil || codes == nil {
		codes = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"recovery_codes": codes})
}

// authCheck GET /api/v1/auth/check：返回当前登录状态（兼容新旧 cookie 格式）。
func (h *Handler) authCheck(c *gin.Context) {
	if h.site.Admin.Password == "" {
		c.JSON(http.StatusOK, gin.H{"authenticated": true, "passwordRequired": false})
		return
	}
	if cookie, err := c.Cookie(authCookieName); err == nil {
		if _, ok := h.validAuthCookie(cookie); ok {
			c.JSON(http.StatusOK, gin.H{"authenticated": true, "passwordRequired": true})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"authenticated": false, "passwordRequired": true})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	if userID > 0 {
		// 新流程：用户表
		user, err := h.store.GetUserByID(userID)
		if err != nil || user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
			return
		}
		if err := store.CheckPassword(user.PasswordHash, req.Current); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "当前密码错误"})
			return
		}
		newHash, err := store.HashPassword(req.New)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "密码处理失败"})
			return
		}
		newAuthSecret := store.GenerateAuthSecret()
		if err := h.store.UpdatePassword(user.ID, newHash, newAuthSecret); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		// 向后兼容：userID=0 表示旧版 admin，使用 settings 表
		if subtle.ConstantTimeCompare([]byte(req.Current), []byte(h.site.Admin.Password)) != 1 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "当前密码错误"})
			return
		}
		if err := config.SaveAdminPassword(h.store, h.site, req.New); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	if !strings.HasPrefix(h.site.URL, "https://") {
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
