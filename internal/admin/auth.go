// /admin 密码 gate：表单登录 + HMAC 签名 cookie（无状态，重启不失效）。
// 密码配置在 site.yml admin.password；留空则中间件直接放行（本地开发模式）。
package admin

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	tmpl "blog/internal/template"
)

const (
	authCookieName = "blog_admin"       // 签名 cookie 名
	authCookieTTL  = 7 * 24 * time.Hour // 登录态有效期 7 天
	loginRateLimit = 10 * time.Second   // 同 IP 登录失败最小间隔（防爆破）
)

// AuthMiddleware 导出的 /admin 鉴权中间件，供 media 等独立包复用。
// 密码未配置直接放行；否则校验 HMAC 签名 cookie，缺失/无效/过期 303 跳登录页。
func AuthMiddleware(site *tmpl.Site) gin.HandlerFunc {
	return func(c *gin.Context) {
		password := site.Admin.Password
		if password == "" {
			c.Next()
			return
		}
		if cookie, err := c.Cookie(authCookieName); err == nil && validAuthCookie(cookie, password) {
			c.Next()
			return
		}
		from := c.Request.URL.RequestURI()
		c.Redirect(http.StatusSeeOther, "/admin/login?from="+from)
		c.Abort()
	}
}

// authRequired /admin 路由组中间件，内部复用 AuthMiddleware。
func (h *Handler) authRequired() gin.HandlerFunc {
	return AuthMiddleware(h.site)
}

// loginForm GET /admin/login：渲染登录页。已登录直接回 /admin；密码未配置提示无需登录。
func (h *Handler) loginForm(c *gin.Context) {
	if h.site.Admin.Password == "" {
		c.Redirect(http.StatusSeeOther, "/admin")
		return
	}
	if cookie, err := c.Cookie(authCookieName); err == nil && validAuthCookie(cookie, h.site.Admin.Password) {
		c.Redirect(http.StatusSeeOther, safeFrom(c.Query("from")))
		return
	}
	h.renderLogin(c, "", c.Query("from"))
}

// login POST /admin/login：校验密码（常量时间比较），成功写签名 cookie 并回跳 from。
// 失败按 IP 限流（10s 一次）防爆破，回显错误。
func (h *Handler) login(c *gin.Context) {
	password := h.site.Admin.Password
	if password == "" {
		c.Redirect(http.StatusSeeOther, "/admin")
		return
	}
	from := c.PostForm("from")
	ip := c.ClientIP()
	if !h.loginLimiter.allow(ip) {
		h.renderLogin(c, "尝试太频繁，请稍后再试", from)
		return
	}
	input := c.PostForm("password")
	if subtle.ConstantTimeCompare([]byte(input), []byte(password)) != 1 {
		h.renderLogin(c, "密码错误", from)
		return
	}
	expiry := time.Now().Add(authCookieTTL).Unix()
	value := strconv.FormatInt(expiry, 10) + "." + signAuthCookie(expiry, password)
	// HttpOnly 防 XSS 读取；SameSite=Lax 允许顶级导航带 cookie（从登录页跳转回来）
	c.SetCookie(authCookieName, value, int(authCookieTTL.Seconds()), "/admin", "", false, true)
	c.Redirect(http.StatusSeeOther, safeFrom(from))
}

// logout POST /admin/logout：清 cookie 回登录页。
func (h *Handler) logout(c *gin.Context) {
	c.SetCookie(authCookieName, "", -1, "/admin", "", false, true)
	if h.site.Admin.Password == "" {
		c.Redirect(http.StatusSeeOther, "/admin")
		return
	}
	c.Redirect(http.StatusSeeOther, "/admin/login")
}

// renderLogin 渲染登录页（独立模板 login，不套 base 布局）。from 为登录成功后的回跳地址。
func (h *Handler) renderLogin(c *gin.Context, errMsg, from string) {
	data := &tmpl.ViewData{
		Site:  h.site,
		Page:  &tmpl.Page{Title: "登录", MenuID: "admin"},
		Admin: &tmpl.AdminData{LoginError: errMsg, LoginFrom: safeFrom(from)},
	}
	html, err := h.engine.Render("admin/login", data)
	if err != nil {
		c.String(http.StatusInternalServerError, "render login: %v", err)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// signAuthCookie 计算 cookie 签名：HMAC-SHA256(key=password, msg=expiry)。
// 密码即密钥，改密码后所有旧 cookie 立即失效。
func signAuthCookie(expiry int64, password string) string {
	mac := hmac.New(sha256.New, []byte(password))
	mac.Write([]byte(strconv.FormatInt(expiry, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

// validAuthCookie 校验 cookie 值（{expiry}.{signature} 格式）：签名合法且未过期。
func validAuthCookie(value, password string) bool {
	i := strings.IndexByte(value, '.')
	if i <= 0 {
		return false
	}
	expiry, err := strconv.ParseInt(value[:i], 10, 64)
	if err != nil || time.Now().Unix() > expiry {
		return false
	}
	expected := signAuthCookie(expiry, password)
	return subtle.ConstantTimeCompare([]byte(value[i+1:]), []byte(expected)) == 1
}

// safeFrom 过滤回跳地址：只允许站内 /admin 开头路径，防开放重定向。
func safeFrom(from string) string {
	if !strings.HasPrefix(from, "/admin") || strings.HasPrefix(from, "//") {
		return "/admin"
	}
	return from
}

// ipLimiter 登录失败限流：同 IP 最小间隔 loginRateLimit。顺便清理过期条目防 map 无限增长。
type ipLimiter struct {
	mu   sync.Mutex
	last map[string]time.Time
}

func newIPLimiter() *ipLimiter {
	return &ipLimiter{last: map[string]time.Time{}}
}

// allow 返回该 IP 当前是否可尝试登录；通过时记录本次时间。
func (l *ipLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for k, t := range l.last {
		if now.Sub(t) > loginRateLimit {
			delete(l.last, k)
		}
	}
	if t, ok := l.last[ip]; ok && now.Sub(t) < loginRateLimit {
		return false
	}
	l.last[ip] = now
	return true
}
