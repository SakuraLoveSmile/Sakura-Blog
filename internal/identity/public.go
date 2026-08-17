package identity

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	tmpl "blog/internal/settings"
	"blog/internal/store"
	"blog/internal/textutil"
)

const (
	UserCookieName = "blog_user"
	userCookieTTL  = 30 * 24 * time.Hour
	currentUserKey = "current_user"
	minUsernameLen = 3
	maxUsernameLen = 20
	minPasswordLen = 6
	maxPasswordLen = 64
	maxNicknameLen = 32
	maxEmailLen    = 254
	loginWindow    = 5 * time.Minute
	loginMaxFails  = 5
)

// PublicHandler owns reader sessions and the public registration/login wire
// contract. It is shared by Stellar's public routes and comment handlers.
type PublicHandler struct {
	store        *store.Store
	site         *atomic.Pointer[tmpl.Site]
	loginMu      sync.Mutex
	loginFails   map[string]loginHit
	registerMu   sync.Mutex
	registerHits map[string]registerHit
}

type loginHit struct {
	n  int
	at time.Time
}
type registerHit struct {
	n  int
	at time.Time
}

func NewPublicHandler(st *store.Store, site *atomic.Pointer[tmpl.Site]) *PublicHandler {
	return &PublicHandler{store: st, site: site, loginFails: map[string]loginHit{}, registerHits: map[string]registerHit{}}
}

// Middleware injects a verified *store.CurrentUser into the shared Gin key.
func (h *PublicHandler) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := &store.CurrentUser{}
		if value, err := c.Cookie(UserCookieName); err == nil {
			if id, secret, expiry, ok := h.validCookie(value); ok {
				if u, lookupErr := h.store.GetUserByID(id); lookupErr == nil && u != nil && u.Status != store.UserStatusBanned {
					user.ID, user.Username, user.Email, user.Avatar, user.Role = u.ID, u.Username, u.Email, u.Avatar, u.Role
					user.Nickname = u.DisplayName
					if user.Nickname == "" {
						user.Nickname = u.Username
					}
					user.Verified = true
					if time.Now().Unix()-expiry > int64(userCookieTTL.Seconds())/2 {
						newExpiry := time.Now().Add(userCookieTTL).Unix()
						c.SetSameSite(http.SameSiteLaxMode)
						c.SetCookie(UserCookieName, SignCookie(ReaderAudience, newExpiry, id, secret), int(userCookieTTL.Seconds()), "/", "", h.cookieSecure(c), true)
					}
				}
			}
		}
		c.Set(currentUserKey, user)
		c.Next()
	}
}

// CurrentUser reads the identity injected by Middleware.
func CurrentUser(c *gin.Context) *store.CurrentUser {
	if value, ok := c.Get(currentUserKey); ok {
		if user, ok := value.(*store.CurrentUser); ok {
			return user
		}
	}
	return &store.CurrentUser{}
}

// Register mounts /api/auth/* without exposing Console's implementation.
func (h *PublicHandler) Register(r *gin.Engine) {
	auth := r.Group("/api/auth")
	auth.POST("/register", h.register)
	auth.POST("/login", h.login)
	auth.GET("/me", h.me)
	auth.POST("/logout", h.logout)
}

func (h *PublicHandler) register(c *gin.Context) {
	if !h.registerAllowed(c.ClientIP()) {
		jsonError(c, http.StatusTooManyRequests, "注册过于频繁，请稍后再试")
		return
	}
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Avatar      string `json:"avatar"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	req.Username, req.Password, req.DisplayName = strings.TrimSpace(req.Username), strings.TrimSpace(req.Password), strings.TrimSpace(req.DisplayName)
	req.Email, req.Avatar = strings.TrimSpace(strings.ToLower(req.Email)), strings.TrimSpace(req.Avatar)
	if utf8.RuneCountInString(req.Username) < minUsernameLen || utf8.RuneCountInString(req.Username) > maxUsernameLen {
		jsonError(c, http.StatusBadRequest, "用户名长度需在 3-20 字符之间")
		return
	}
	if len(req.Password) < minPasswordLen || len(req.Password) > maxPasswordLen {
		jsonError(c, http.StatusBadRequest, "密码长度需在 6-64 字节之间")
		return
	}
	if utf8.RuneCountInString(req.DisplayName) > maxNicknameLen {
		jsonError(c, http.StatusBadRequest, "昵称过长（最多 32 字）")
		return
	}
	if !textutil.ValidEmail(req.Email, maxEmailLen) {
		jsonError(c, http.StatusBadRequest, "邮箱格式不正确")
		return
	}
	for _, char := range req.Username {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-') {
			jsonError(c, http.StatusBadRequest, "用户名只允许字母、数字、下划线和中划线")
			return
		}
	}
	existing, err := h.store.GetUserByUsername(req.Username)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "注册失败，请稍后再试")
		return
	}
	if existing != nil {
		jsonError(c, http.StatusConflict, "用户名已被占用")
		return
	}
	hash, err := store.HashPassword(req.Password)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "注册失败，请稍后再试")
		return
	}
	u := &store.User{Username: req.Username, PasswordHash: hash, DisplayName: req.DisplayName, Email: req.Email, Avatar: req.Avatar, AuthSecret: store.GenerateAuthSecret(), Role: store.RoleReader}
	if err := h.store.CreateUser(u); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			jsonError(c, http.StatusConflict, "用户名已被占用")
		} else {
			jsonError(c, http.StatusInternalServerError, "注册失败，请稍后再试")
		}
		return
	}
	h.setUserCookie(c, u)
	c.JSON(http.StatusCreated, gin.H{"ok": true, "user": publicUser(u)})
}

func (h *PublicHandler) login(c *gin.Context) {
	ip := c.ClientIP()
	if h.loginBlocked(ip) {
		jsonError(c, http.StatusTooManyRequests, "尝试次数过多，请稍后再试")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	req.Username, req.Password = strings.TrimSpace(req.Username), strings.TrimSpace(req.Password)
	if req.Username == "" || req.Password == "" {
		jsonError(c, http.StatusUnauthorized, "用户名或密码不能为空")
		return
	}
	u, err := h.store.GetUserByUsername(req.Username)
	if err != nil || u == nil || store.CheckPassword(u.PasswordHash, req.Password) != nil {
		h.recordLoginFail(ip)
		jsonError(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if u.Role != store.RoleReader {
		jsonError(c, http.StatusForbidden, "管理账号请从后台登录")
		return
	}
	if u.Status == store.UserStatusBanned {
		jsonError(c, http.StatusForbidden, "该账号已被封禁")
		return
	}
	h.clearLoginFail(ip)
	h.setUserCookie(c, u)
	c.JSON(http.StatusOK, gin.H{"ok": true, "user": publicUser(u)})
}

func (h *PublicHandler) me(c *gin.Context) {
	u := CurrentUser(c)
	c.JSON(http.StatusOK, gin.H{"id": u.ID, "username": u.Username, "nickname": u.Nickname, "email": u.Email, "avatar": u.Avatar, "role": u.Role, "verified": u.Verified, "is_admin": u.Role == store.RoleAdmin})
}

func (h *PublicHandler) logout(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(UserCookieName, "", -1, "/", "", h.cookieSecure(c), true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *PublicHandler) setUserCookie(c *gin.Context, u *store.User) {
	expiry := time.Now().Add(userCookieTTL).Unix()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(UserCookieName, SignCookie(ReaderAudience, expiry, u.ID, u.AuthSecret), int(userCookieTTL.Seconds()), "/", "", h.cookieSecure(c), true)
}

func (h *PublicHandler) validCookie(value string) (int64, string, int64, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 4 || parts[0] != string(ReaderAudience) {
		return 0, "", 0, false
	}
	id := int64(0)
	if _, err := fmt.Sscan(parts[2], &id); err != nil || id <= 0 {
		return 0, "", 0, false
	}
	u, err := h.store.GetUserByID(id)
	if err != nil || u == nil {
		return 0, "", 0, false
	}
	_, expiry, ok := VerifyCookie(value, ReaderAudience, u.AuthSecret, time.Now())
	return id, u.AuthSecret, expiry, ok
}

func (h *PublicHandler) cookieSecure(c *gin.Context) bool {
	if h.site == nil || h.site.Load() == nil || !strings.HasPrefix(h.site.Load().URL, "https://") {
		return false
	}
	return c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
}

func (h *PublicHandler) loginBlocked(ip string) bool {
	h.loginMu.Lock()
	defer h.loginMu.Unlock()
	now := time.Now()
	for key, hit := range h.loginFails {
		if now.Sub(hit.at) > loginWindow {
			delete(h.loginFails, key)
		}
	}
	return h.loginFails[ip].n >= loginMaxFails && now.Sub(h.loginFails[ip].at) < loginWindow
}
func (h *PublicHandler) recordLoginFail(ip string) {
	h.loginMu.Lock()
	defer h.loginMu.Unlock()
	hit := h.loginFails[ip]
	if time.Since(hit.at) > loginWindow {
		hit.n = 0
	}
	hit.n++
	hit.at = time.Now()
	h.loginFails[ip] = hit
}
func (h *PublicHandler) clearLoginFail(ip string) {
	h.loginMu.Lock()
	delete(h.loginFails, ip)
	h.loginMu.Unlock()
}
func (h *PublicHandler) registerAllowed(ip string) bool {
	h.registerMu.Lock()
	defer h.registerMu.Unlock()
	now := time.Now()
	hit := h.registerHits[ip]
	if now.Sub(hit.at) > time.Hour {
		hit.n = 0
	}
	if hit.n >= 5 {
		return false
	}
	hit.n++
	hit.at = now
	h.registerHits[ip] = hit
	return true
}

func publicUser(u *store.User) map[string]any {
	nickname := u.DisplayName
	if nickname == "" {
		nickname = u.Username
	}
	return map[string]any{"id": u.ID, "username": u.Username, "nickname": nickname, "email": u.Email, "avatar": u.Avatar, "role": u.Role, "verified": true, "is_admin": u.Role == store.RoleAdmin}
}
func jsonError(c *gin.Context, status int, message string) { c.JSON(status, gin.H{"error": message}) }
