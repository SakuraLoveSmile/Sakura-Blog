package api

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
	"blog/internal/textutil"
)

// 公共读者 cookie 与签名。
const (
	userCookieName = "blog_user"
	userCookieTTL  = 30 * 24 * time.Hour // 读者登录态有效期 30 天
)

// 注册/登录校验常量。
const (
	minUsernameLen = 3
	maxUsernameLen = 20
	minPasswordLen = 6
	maxPasswordLen = 64
	maxNicknameLen = 32
	maxEmailLen    = 254
)

// ctxKeyCurrentUser Gin context key。值类型 *store.CurrentUser。
const ctxKeyCurrentUser = "current_user"

// signUserCookie 签读者 cookie：<user>.<expiry>.<userID>.<hmac(auth_secret, user.expiry.userID)>。
// audience=user 与 admin cookie 绑定区分（安全审计 2026-08-16），跨用途重放验签失败。
func signUserCookie(expiry int64, userID int64, authSecret string) string {
	return signAuthCookie(cookieAudUser, expiry, userID, authSecret)
}

// validUserCookie 验证读者 cookie，返回 (userID, authSecret, expiry, ok)。
// 与 admin cookie 签名算法相同但 audience 独立，cookie 名与 TTL 互不干扰。
func (h *Handler) validUserCookie(value string) (int64, string, int64, bool) {
	userID, authSecret, expiry, _, ok := h.validAuthCookie(value, cookieAudUser)
	if !ok {
		return 0, "", 0, false
	}
	// 读者 cookie 只接受 users 表里真实存在的用户（userID>0），不接受旧格式（userID=0）
	if userID == 0 {
		return 0, "", 0, false
	}
	return userID, authSecret, expiry, true
}

// currentUserMiddleware 把 *store.CurrentUser 注入 gin context。
// 无论 cookie 是否有效都放行，匿名用户填 ID=0、Verified=false；
// 已登录用户从 users 表取资料，Verified=true。
func (h *Handler) currentUserMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		cu := &store.CurrentUser{ID: 0, Role: "", Verified: false}
		if cookie, err := c.Cookie(userCookieName); err == nil {
			if userID, authSecret, expiry, ok := h.validUserCookie(cookie); ok {
				if u, err := h.store.GetUserByID(userID); err == nil && u != nil {
					cu.ID = u.ID
					cu.Username = u.Username
					cu.Nickname = u.DisplayName
					if cu.Nickname == "" {
						cu.Nickname = u.Username
					}
					cu.Email = u.Email
					cu.Link = ""
					cu.Avatar = u.Avatar
					cu.Role = u.Role
					cu.Verified = true
					// 滑动续期
					if time.Now().Unix()-expiry > int64(userCookieTTL.Seconds())/2 {
						newExpiry := time.Now().Add(userCookieTTL).Unix()
						c.SetSameSite(http.SameSiteLaxMode)
						c.SetCookie(userCookieName, signUserCookie(newExpiry, userID, authSecret),
							int(userCookieTTL.Seconds()), "/", "", h.cookieSecure(c), true)
					}
				}
			}
		}
		c.Set(ctxKeyCurrentUser, cu)
		c.Next()
	}
}

// GetCurrentUser 从 context 取当前用户，无则返回匿名用户。
func GetCurrentUser(c *gin.Context) *store.CurrentUser {
	if v, ok := c.Get(ctxKeyCurrentUser); ok {
		if cu, ok := v.(*store.CurrentUser); ok {
			return cu
		}
	}
	return &store.CurrentUser{ID: 0, Role: "", Verified: false}
}

// --- 公共端点（挂在 Mount 的 r.Use(currentUserMiddleware) 之后，无需额外认证） ---

// publicRegister POST /api/auth/register：reader 角色注册。
// 入参：username / password / display_name（昵称）/ email / avatar（可选）。
// 用户名唯一约束；注册成功自动下发 cookie 并返回当前用户。
// 安全审计 2026-08-16：补 per-IP 注册限流（防批量刷号）与邮箱格式强校验（拒 CRLF）。
func (h *Handler) publicRegister(c *gin.Context) {
	if !h.registerAllowed(c.ClientIP()) {
		Fail(c, http.StatusTooManyRequests, "注册过于频繁，请稍后再试")
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
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Avatar = strings.TrimSpace(req.Avatar)

	// 校验
	if utf8.RuneCountInString(req.Username) < minUsernameLen ||
		utf8.RuneCountInString(req.Username) > maxUsernameLen {
		Fail(c, http.StatusBadRequest, "用户名长度需在 3-20 字符之间")
		return
	}
	if len(req.Password) < minPasswordLen || len(req.Password) > maxPasswordLen {
		Fail(c, http.StatusBadRequest, "密码长度需在 6-64 字节之间")
		return
	}
	if utf8.RuneCountInString(req.DisplayName) > maxNicknameLen {
		Fail(c, http.StatusBadRequest, "昵称过长（最多 32 字）")
		return
	}
	// 安全审计 2026-08-16：net/mail 强校验替代"含 @ 即可"（拒 CRLF/控制字符，防后续
	// 通知链路头注入与 SMTP 命令注入）。
	if !ValidEmail(req.Email) {
		Fail(c, http.StatusBadRequest, "邮箱格式不正确")
		return
	}
	// 用户名只允许字母数字下划线中划线
	for _, r := range req.Username {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '-'
		if !ok {
			Fail(c, http.StatusBadRequest, "用户名只允许字母、数字、下划线和中划线")
			return
		}
	}

	// 用户名重复检查
	if existing, err := h.store.GetUserByUsername(req.Username); err != nil {
		Fail(c, http.StatusInternalServerError, "注册失败，请稍后再试")
		return
	} else if existing != nil {
		Fail(c, http.StatusConflict, "用户名已被占用")
		return
	}

	pwHash, err := store.HashPassword(req.Password)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "注册失败，请稍后再试")
		return
	}

	u := &store.User{
		Username:     req.Username,
		PasswordHash: pwHash,
		DisplayName:  req.DisplayName,
		Email:        req.Email,
		Avatar:       req.Avatar,
		AuthSecret:   store.GenerateAuthSecret(),
		Role:         store.RoleReader,
	}
	if err := h.store.CreateUser(u); err != nil {
		// UNIQUE 冲突兜底（防止 TOCTOU 竞争）
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			Fail(c, http.StatusConflict, "用户名已被占用")
			return
		}
		Fail(c, http.StatusInternalServerError, "注册失败，请稍后再试")
		return
	}

	// 自动登录：下发 cookie
	expiry := time.Now().Add(userCookieTTL).Unix()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(userCookieName, signUserCookie(expiry, u.ID, u.AuthSecret),
		int(userCookieTTL.Seconds()), "/", "", h.cookieSecure(c), true)

	c.JSON(http.StatusCreated, gin.H{"ok": true, "user": h.toPublicUserView(u)})
}

// publicLogin POST /api/auth/login：读者登录。
// 只接受 role=reader（防止 admin/author 通过公共端点登录泄露权限模型）；
// 安全审计 2026-08-16：补 per-IP 失败限流（复用登录限流窗口）与角色校验的实际执行
// （此前仅注释声明未实现）。
func (h *Handler) publicLogin(c *gin.Context) {
	ip := c.ClientIP()
	if h.loginBlocked(ip) {
		Fail(c, http.StatusTooManyRequests, "尝试次数过多，请稍后再试")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)

	if req.Username == "" || req.Password == "" {
		Fail(c, http.StatusUnauthorized, "用户名或密码不能为空")
		return
	}

	user, err := h.store.GetUserByUsername(req.Username)
	if err != nil || user == nil {
		h.recordLoginFail(ip)
		Fail(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if err := store.CheckPassword(user.PasswordHash, req.Password); err != nil {
		h.recordLoginFail(ip)
		Fail(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	// 角色校验：公共端点仅服务 reader，admin/author 走 /api/v1/auth/login。
	if normalizeRole(user.Role) != store.RoleReader {
		h.recordLoginFail(ip)
		Fail(c, http.StatusForbidden, "管理账号请从后台登录")
		return
	}

	h.clearLoginFail(ip)
	expiry := time.Now().Add(userCookieTTL).Unix()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(userCookieName, signUserCookie(expiry, user.ID, user.AuthSecret),
		int(userCookieTTL.Seconds()), "/", "", h.cookieSecure(c), true)

	c.JSON(http.StatusOK, gin.H{"ok": true, "user": h.toPublicUserView(user)})
}

// --- 注册限流（安全审计 2026-08-16）---

// 注册限流参数：每 IP 1 小时窗口内最多 5 次注册。
const (
	registerWindow   = time.Hour
	registerMaxHits  = 5
	registerCooldown = time.Hour // 触发后冷却时长
)

// registerAllowed 检查该 IP 是否仍可注册，未超限时顺带计数。惰性清理过期条目。
func (h *Handler) registerAllowed(ip string) bool {
	h.registerMu.Lock()
	defer h.registerMu.Unlock()
	now := time.Now()
	for k, v := range h.registerHits {
		if now.Sub(v.t) > registerCooldown {
			delete(h.registerHits, k)
		}
	}
	v := h.registerHits[ip]
	if now.Sub(v.t) > registerWindow {
		v.n = 0
	}
	if v.n >= registerMaxHits {
		return false
	}
	v.n++
	v.t = now
	h.registerHits[ip] = v
	return true
}

// ValidEmail 邮箱校验（安全审计 2026-08-16）：委托 textutil.ValidEmail 共享实现。
func ValidEmail(s string) bool {
	return textutil.ValidEmail(s, maxEmailLen)
}

// publicMe GET /api/auth/me：返回当前用户（匿名或已登录）。
// 永远返回 200；匿名用户 id=0、verified=false。
func (h *Handler) publicMe(c *gin.Context) {
	cu := GetCurrentUser(c)
	c.JSON(http.StatusOK, gin.H{
		"id":       cu.ID,
		"username": cu.Username,
		"nickname": cu.Nickname,
		"email":    cu.Email,
		"avatar":   cu.Avatar,
		"role":     cu.Role,
		"verified": cu.Verified,
		"is_admin": cu.Role == store.RoleAdmin,
	})
}

// publicLogout POST /api/auth/logout：清读者 cookie。
func (h *Handler) publicLogout(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(userCookieName, "", -1, "/", "", h.cookieSecure(c), true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// toPublicUserView 把 store.User 转成对外展示模型（不含敏感字段）。
func (h *Handler) toPublicUserView(u *store.User) map[string]any {
	nick := u.DisplayName
	if nick == "" {
		nick = u.Username
	}
	return map[string]any{
		"id":       u.ID,
		"username": u.Username,
		"nickname": nick,
		"email":    u.Email,
		"avatar":   u.Avatar,
		"role":     u.Role,
		"verified": true,
		"is_admin": u.Role == store.RoleAdmin,
	}
}
