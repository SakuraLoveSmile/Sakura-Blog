package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// 用户管理 API（多用户体系）：全部为 admin-only（路由注册在 sys 组）。

// userJSON 用户列表/详情响应（非敏感字段）。
type userJSON struct {
	ID          int64    `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	Email       string   `json:"email"`
	Avatar      string   `json:"avatar"`
	Role        string   `json:"role"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	Status      string   `json:"status"`
	CreatedAt   string   `json:"created_at"`
}

func toUserJSON(u *store.User) *userJSON {
	return &userJSON{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Email:       u.Email,
		Avatar:      u.Avatar,
		Role:        u.Role,
		Roles:       append([]string(nil), u.Roles...), Permissions: append([]string(nil), u.Permissions...),
		Status:      u.Status,
		CreatedAt:   u.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

// listUsers GET /api/v1/users：分页查询用户（keyword 匹配用户名/显示名/邮箱）。
func (h *Handler) listUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	keyword := strings.TrimSpace(c.Query("keyword"))

	users, total, err := h.store.ListUsersPage(page, size, keyword)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]*userJSON, 0, len(users))
	for _, u := range users {
		items = append(items, toUserJSON(u))
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "size": size})
}

// createUser POST /api/v1/users：创建用户（username 唯一，密码至少 6 位，角色必选）。
func (h *Handler) createUser(c *gin.Context) {
	var req struct {
		Username    string   `json:"username"`
		DisplayName string   `json:"display_name"`
		Email       string   `json:"email"`
		Avatar      string   `json:"avatar"`
		Password    string   `json:"password"`
		Role        string   `json:"role"`
		Roles       []string `json:"roles"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" {
		Fail(c, http.StatusBadRequest, "用户名不能为空")
		return
	}
	if len([]rune(username)) > 64 {
		Fail(c, http.StatusBadRequest, "用户名过长（最多 64 字符）")
		return
	}
	if len(req.Password) < 6 {
		Fail(c, http.StatusBadRequest, "密码至少 6 位")
		return
	}
	roles := requestedRoles(req.Roles, req.Role)
	if len(roles) == 0 {
		Fail(c, http.StatusBadRequest, "至少需要一个角色")
		return
	}
	for _, role := range roles {
		if !store.ValidRole(role) {
			Fail(c, http.StatusBadRequest, "角色不合法")
			return
		}
	}
	if len(roles) > 4 {
		Fail(c, http.StatusBadRequest, "角色数量过多")
		return
	}
	hash, err := store.HashPassword(req.Password)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "密码处理失败")
		return
	}
	user := &store.User{
		Username:     username,
		PasswordHash: hash,
		DisplayName:  strings.TrimSpace(req.DisplayName),
		Email:        strings.TrimSpace(req.Email),
		Avatar:       strings.TrimSpace(req.Avatar),
		AuthSecret:   store.GenerateAuthSecret(),
		Role:         roles[0],
	}
	if err := h.store.CreateUser(user); err != nil {
		Fail(c, http.StatusConflict, "用户名已存在")
		return
	}
	if len(roles) > 1 {
		if err := h.store.SetUserRoles(user.ID, roles); err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		refreshed, err := h.store.GetUserByID(user.ID)
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		user = refreshed
	}
	c.JSON(http.StatusOK, toUserJSON(user))
}

// updateUser PUT /api/v1/users/:id：更新用户资料与角色（不允许降级最后一个 admin）。
func (h *Handler) updateUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		Fail(c, http.StatusBadRequest, "用户 ID 不合法")
		return
	}
	var req struct {
		DisplayName string   `json:"display_name"`
		Email       string   `json:"email"`
		Avatar      string   `json:"avatar"`
		Role        string   `json:"role"`
		Roles       []string `json:"roles"`
		Status      string   `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	user, err := h.store.GetUserByID(id)
	if err != nil || user == nil {
		Fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	req.Status = strings.TrimSpace(req.Status)
	if req.Status != "" && req.Status != user.Status {
		if !store.ValidUserStatus(req.Status) {
			Fail(c, http.StatusBadRequest, "用户状态不合法")
			return
		}
		if req.Status == store.UserStatusBanned && userHasRole(user, store.RoleAdmin) {
			n, err := h.store.CountUsersByRole(store.RoleAdmin)
			if err != nil {
				Fail(c, http.StatusInternalServerError, err.Error())
				return
			}
			if n <= 1 {
				Fail(c, http.StatusBadRequest, "不能封禁最后一个管理员")
				return
			}
		}
		if err := h.store.UpdateUserStatus(user.ID, req.Status); err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		user.Status = req.Status
	}
	roles := requestedRoles(req.Roles, req.Role)
	roleChanged := len(roles) > 0 && !sameRoles(user.Roles, roles)
	// 角色变更：最后一个 admin 不允许降级（防锁死）。
	if roleChanged {
		for _, role := range roles {
			if !store.ValidRole(role) {
				Fail(c, http.StatusBadRequest, "角色不合法")
				return
			}
		}
		if userHasRole(user, store.RoleAdmin) && !containsRole(roles, store.RoleAdmin) {
			n, err := h.store.CountUsersByRole(store.RoleAdmin)
			if err != nil {
				Fail(c, http.StatusInternalServerError, err.Error())
				return
			}
			if n <= 1 {
				Fail(c, http.StatusBadRequest, "不能降级最后一个管理员")
				return
			}
		}
	}
	user.DisplayName = strings.TrimSpace(req.DisplayName)
	user.Email = strings.TrimSpace(req.Email)
	user.Avatar = strings.TrimSpace(req.Avatar)
	if err := h.store.UpdateUser(user); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if roleChanged {
		if err := h.store.SetUserRoles(user.ID, roles); err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		refreshed, err := h.store.GetUserByID(user.ID)
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		user = refreshed
	}
	c.JSON(http.StatusOK, toUserJSON(user))
}

func requestedRoles(roles []string, primary string) []string {
	if len(roles) == 0 && strings.TrimSpace(primary) != "" {
		roles = []string{primary}
	}
	out := make([]string, 0, len(roles))
	seen := map[string]bool{}
	for _, role := range roles {
		role = strings.TrimSpace(role)
		if role != "" && !seen[role] {
			seen[role] = true
			out = append(out, role)
		}
	}
	return out
}

func containsRole(roles []string, target string) bool {
	for _, role := range roles {
		if role == target {
			return true
		}
	}
	return false
}

func sameRoles(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// resetUserPassword PUT /api/v1/users/:id/password：管理员重置用户密码，
// 轮换 auth_secret 使该用户所有旧 cookie 立即失效。
func (h *Handler) resetUserPassword(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		Fail(c, http.StatusBadRequest, "用户 ID 不合法")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if len(req.Password) < 6 {
		Fail(c, http.StatusBadRequest, "密码至少 6 位")
		return
	}
	user, err := h.store.GetUserByID(id)
	if err != nil || user == nil {
		Fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	hash, err := store.HashPassword(req.Password)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "密码处理失败")
		return
	}
	if err := h.store.UpdatePassword(user.ID, hash, store.GenerateAuthSecret()); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// deleteUser DELETE /api/v1/users/:id：删除用户（禁止删除自己与最后一个 admin）。
func (h *Handler) deleteUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		Fail(c, http.StatusBadRequest, "用户 ID 不合法")
		return
	}
	currentID := c.GetInt64("userID")
	if id == currentID {
		Fail(c, http.StatusBadRequest, "不能删除当前登录用户")
		return
	}
	user, err := h.store.GetUserByID(id)
	if err != nil || user == nil {
		Fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	if userHasRole(user, store.RoleAdmin) {
		n, err := h.store.CountUsersByRole(store.RoleAdmin)
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		if n <= 1 {
			Fail(c, http.StatusBadRequest, "不能删除最后一个管理员")
			return
		}
	}
	if err := h.store.DeleteUser(id); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func userHasRole(user *store.User, role string) bool {
	if user == nil {
		return false
	}
	return containsRole(user.Roles, role) || (len(user.Roles) == 0 && user.Role == role)
}
