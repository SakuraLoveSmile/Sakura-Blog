package api

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/config"
)

// 登录限流参数（P0-4）。
const (
	loginRateLimit  = 5 * time.Minute // 失败计数窗口
	loginMaxFails   = 5               // 窗口内最大失败次数
	loginBlockTime  = 5 * time.Minute // 触发限流后封锁时长
)

// login POST /api/v1/auth/login：校验密码，成功写签名 cookie 返回 JSON。
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

	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	if subtle.ConstantTimeCompare([]byte(req.Password), []byte(password)) != 1 {
		h.recordLoginFail(ip)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
		return
	}

	h.clearLoginFail(ip)
	expiry := time.Now().Add(authCookieTTL).Unix()
	value := strconv.FormatInt(expiry, 10) + "." + signAuthCookie(expiry, password)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(authCookieName, value, int(authCookieTTL.Seconds()), "/", "", h.cookieSecure(), true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// logout POST /api/v1/auth/logout：清 cookie。
func (h *Handler) logout(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(authCookieName, "", -1, "/", "", h.cookieSecure(), true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// authCheck GET /api/v1/auth/check：返回当前登录状态。
func (h *Handler) authCheck(c *gin.Context) {
	password := h.site.Admin.Password
	if password == "" {
		c.JSON(http.StatusOK, gin.H{"authenticated": true, "passwordRequired": false})
		return
	}
	if cookie, err := c.Cookie(authCookieName); err == nil && validAuthCookie(cookie, password) {
		c.JSON(http.StatusOK, gin.H{"authenticated": true, "passwordRequired": true})
		return
	}
	c.JSON(http.StatusOK, gin.H{"authenticated": false, "passwordRequired": true})
}

// changePassword PUT /api/v1/auth/password：校验当前密码后将新密码存入 settings 表 admin 段。
// 新密码允许为空（=关闭登录 gate）。cookie 签名 key 随密码变化，保存后旧 cookie 全部失效。
func (h *Handler) changePassword(c *gin.Context) {
	var req struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	if subtle.ConstantTimeCompare([]byte(req.Current), []byte(h.site.Admin.Password)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "当前密码错误"})
		return
	}
	if err := config.SaveAdminPassword(h.store, h.site, req.New); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 旧 cookie 已因签名 key 变化失效，顺手清掉客户端残留
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(authCookieName, "", -1, "/", "", h.cookieSecure(), true)
	c.JSON(http.StatusOK, gin.H{"ok": true, "passwordRequired": req.New != ""})
}

// cookieSecure 由 site.URL 协议推导：https → true，http/空 → false（本地开发不被 Secure 挡住）。
func (h *Handler) cookieSecure() bool {
	return strings.HasPrefix(h.site.URL, "https://")
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
