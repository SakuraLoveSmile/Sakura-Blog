package api

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/config"
)

// login POST /api/v1/auth/login：校验密码，成功写签名 cookie 返回 JSON。
func (h *Handler) login(c *gin.Context) {
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
		return
	}

	expiry := time.Now().Add(authCookieTTL).Unix()
	value := strconv.FormatInt(expiry, 10) + "." + signAuthCookie(expiry, password)
	c.SetCookie(authCookieName, value, int(authCookieTTL.Seconds()), "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// logout POST /api/v1/auth/logout：清 cookie。
func (h *Handler) logout(c *gin.Context) {
	c.SetCookie(authCookieName, "", -1, "/", "", false, true)
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
	c.SetCookie(authCookieName, "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"ok": true, "passwordRequired": req.New != ""})
}
