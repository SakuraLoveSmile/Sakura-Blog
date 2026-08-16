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
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Avatar      string `json:"avatar"`
	Role        string `json:"role"`
	CreatedAt   string `json:"created_at"`
}

func toUserJSON(u *store.User) *userJSON {
	return &userJSON{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Email:       u.Email,
		Avatar:      u.Avatar,
		Role:        u.Role,
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
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Avatar      string `json:"avatar"`
		Password    string `json:"password"`
		Role        string `json:"role"`
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
	if !store.ValidRole(req.Role) {
		Fail(c, http.StatusBadRequest, "角色不合法")
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
		Role:         req.Role,
	}
	if err := h.store.CreateUser(user); err != nil {
		Fail(c, http.StatusConflict, "用户名已存在")
		return
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
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Avatar      string `json:"avatar"`
		Role        string `json:"role"`
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
	// 角色变更：最后一个 admin 不允许降级（防锁死）。
	if req.Role != "" && req.Role != user.Role {
		if !store.ValidRole(req.Role) {
			Fail(c, http.StatusBadRequest, "角色不合法")
			return
		}
		if user.Role == store.RoleAdmin && req.Role == store.RoleAuthor {
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
	if req.Role != "" && req.Role != user.Role {
		if err := h.store.UpdateUserRole(user.ID, req.Role); err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		user.Role = req.Role
	}
	c.JSON(http.StatusOK, toUserJSON(user))
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
	if user.Role == store.RoleAdmin {
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
