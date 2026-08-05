package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// 多用户体系测试：用户管理 API（admin-only）+ 角色鉴权（author 可内容管理、禁系统管理）。

// createUserViaAPI 用 admin cookie 创建用户，返回响应体解码后的 userJSON。
func createUserViaAPI(t *testing.T, r *gin.Engine, cookie, username string) userJSON {
	t.Helper()
	body := fmt.Sprintf(`{"username":%q,"display_name":%q,"email":%q,"password":"secret6","role":"author"}`, username, username, username+"@example.com")
	w := do2FA(r, http.MethodPost, "/api/v1/users", body, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("create user %s: status = %d, body = %s", username, w.Code, w.Body.String())
	}
	var u userJSON
	if err := json.Unmarshal(w.Body.Bytes(), &u); err != nil {
		t.Fatalf("decode created user: %v", err)
	}
	return u
}

// loginAs 用用户名/密码登录，返回 Set-Cookie 值（认证后 cookie）。
func loginAs(t *testing.T, r *gin.Engine, username, password string) string {
	t.Helper()
	body := fmt.Sprintf(`{"username":%q,"password":%q}`, username, password)
	w := do2FA(r, http.MethodPost, "/api/v1/auth/login", body, "")
	if w.Code != http.StatusOK {
		t.Fatalf("login %s: status = %d, body = %s", username, w.Code, w.Body.String())
	}
	for _, ck := range w.Result().Cookies() {
		if ck.Name == authCookieName && ck.Value != "" {
			return ck.Value
		}
	}
	t.Fatalf("login %s: no auth cookie set", username)
	return ""
}

// TestUserManagementAdminOnly 用户管理 API：admin 可访问；author 访问 403。
func TestUserManagementAdminOnly(t *testing.T) {
	r, _, admin, _ := newFullEnv(t)
	adminCookie := authCookie(admin)

	// admin 创建 author 用户
	u := createUserViaAPI(t, r, adminCookie, "author1")

	// 列表包含新用户
	w := do2FA(r, http.MethodGet, "/api/v1/users?page=1&size=20", "", adminCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("list users: status = %d", w.Code)
	}
	var list struct {
		Items []userJSON `json:"items"`
		Total int        `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if list.Total < 2 || list.Items[0].ID != admin.ID {
		t.Errorf("list = %+v, want admin first and total >= 2", list)
	}
	if u.Role != store.RoleAuthor {
		t.Errorf("created user role = %q, want author", u.Role)
	}

	// author 登录后：内容 API 可用，系统管理 API 403
	authorCookie := loginAs(t, r, "author1", "secret6")
	w = do2FA(r, http.MethodGet, "/api/v1/posts", "", authorCookie)
	if w.Code != http.StatusOK {
		t.Errorf("author list posts: status = %d, want 200", w.Code)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/users", "", authorCookie)
	if w.Code != http.StatusForbidden {
		t.Errorf("author list users: status = %d, want 403", w.Code)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/settings", "", authorCookie)
	if w.Code != http.StatusForbidden {
		t.Errorf("author get settings: status = %d, want 403", w.Code)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/auth/login-logs", "", authorCookie)
	if w.Code != http.StatusForbidden {
		t.Errorf("author login-logs: status = %d, want 403", w.Code)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/backup/status", "", authorCookie)
	if w.Code != http.StatusForbidden {
		t.Errorf("author backup: status = %d, want 403", w.Code)
	}
}

// TestUserUpdateRoleResetPasswordDelete 角色变更/重置密码/删除用户全流程。
func TestUserUpdateRoleResetPasswordDelete(t *testing.T) {
	r, _, admin, _ := newFullEnv(t)
	adminCookie := authCookie(admin)
	u := createUserViaAPI(t, r, adminCookie, "author2")

	// 更新角色：author → admin
	w := do2FA(r, http.MethodPut, fmt.Sprintf("/api/v1/users/%d", u.ID),
		`{"display_name":"Author Two","email":"a2@example.com","role":"admin"}`, adminCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("update role: status = %d, body = %s", w.Code, w.Body.String())
	}
	var updated userJSON
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated: %v", err)
	}
	if updated.Role != store.RoleAdmin || updated.DisplayName != "Author Two" {
		t.Errorf("updated = %+v, want role=admin display_name=Author Two", updated)
	}

	// 重置密码 → 旧 cookie 失效（auth_secret 轮换），新密码可登录
	oldCookie := loginAs(t, r, "author2", "secret6")
	w = do2FA(r, http.MethodPut, fmt.Sprintf("/api/v1/users/%d/password", u.ID), `{"password":"newpass9"}`, adminCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("reset password: status = %d", w.Code)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/profile", "", oldCookie)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("old cookie after password reset: status = %d, want 401", w.Code)
	}
	loginAs(t, r, "author2", "newpass9")

	// 删除用户 → 再登录失败
	w = do2FA(r, http.MethodDelete, fmt.Sprintf("/api/v1/users/%d", u.ID), "", adminCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("delete user: status = %d", w.Code)
	}
	w = do2FA(r, http.MethodPost, "/api/v1/auth/login", `{"username":"author2","password":"newpass9"}`, "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("login deleted user: status = %d, want 401", w.Code)
	}
}

// TestUserManagementGuardrails 创建冲突/删除保护/降级保护。
func TestUserManagementGuardrails(t *testing.T) {
	r, _, admin, _ := newFullEnv(t)
	adminCookie := authCookie(admin)

	// 重复用户名 → 409
	w := do2FA(r, http.MethodPost, "/api/v1/users", `{"username":"admin","password":"secret6","role":"author"}`, adminCookie)
	if w.Code != http.StatusConflict {
		t.Errorf("duplicate username: status = %d, want 409", w.Code)
	}

	// 密码过短 → 400
	w = do2FA(r, http.MethodPost, "/api/v1/users", `{"username":"shorty","password":"123","role":"author"}`, adminCookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("short password: status = %d, want 400", w.Code)
	}

	// 非法角色 → 400
	w = do2FA(r, http.MethodPost, "/api/v1/users", `{"username":"weird","password":"secret6","role":"superuser"}`, adminCookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid role: status = %d, want 400", w.Code)
	}

	// 不能删除自己
	w = do2FA(r, http.MethodDelete, fmt.Sprintf("/api/v1/users/%d", admin.ID), "", adminCookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("delete self: status = %d, want 400", w.Code)
	}

	// 不能降级/删除最后一个 admin（当前只有 admin 一个管理员）
	w = do2FA(r, http.MethodPut, fmt.Sprintf("/api/v1/users/%d", admin.ID), `{"role":"author"}`, adminCookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("demote last admin: status = %d, want 400", w.Code)
	}

	// 不存在用户 → 404
	w = do2FA(r, http.MethodPut, "/api/v1/users/99999", `{"role":"author"}`, adminCookie)
	if w.Code != http.StatusNotFound {
		t.Errorf("update missing user: status = %d, want 404", w.Code)
	}
}
