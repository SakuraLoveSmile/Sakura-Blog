package store

import (
	"fmt"
)

// Permission constants are stable extension points. Route handlers and
// frontend manifests consume these names instead of comparing role strings.
const (
	PermissionConsoleAccess     = "console:access"
	PermissionContentRead       = "content:read"
	PermissionContentWrite      = "content:write"
	PermissionContentDelete     = "content:delete"
	PermissionContentManageAny  = "content:manage:any"
	PermissionMediaRead         = "media:read"
	PermissionMediaWrite        = "media:write"
	PermissionMediaDelete       = "media:delete"
	PermissionProfileRead       = "profile:read"
	PermissionProfileWrite      = "profile:write"
	PermissionNotificationsRead = "notifications:read"
	PermissionSystemManage      = "system:manage"
	PermissionUsersManage       = "users:manage"
	PermissionModulesManage     = "modules:manage"
	PermissionPluginsManage     = "plugins:manage"
	PermissionWebhookRegister   = "webhook:register"
	PermissionPluginScript      = "plugin:script"
	PermissionNetworkFetch      = "network:fetch"
)

var roleDescriptions = map[string]string{
	RoleAdmin:  "全部系统权限",
	RoleAuthor: "文章、页面和附件管理",
	RoleReader: "账户、通知和读者功能",
}

var permissionDescriptions = map[string]string{
	"*":                         "超级管理员通配权限",
	PermissionConsoleAccess:     "进入管理端",
	PermissionContentRead:       "读取内容",
	PermissionContentWrite:      "创建和编辑内容",
	PermissionContentDelete:     "删除内容",
	PermissionContentManageAny:  "管理任意作者的内容",
	PermissionMediaRead:         "读取附件",
	PermissionMediaWrite:        "上传附件",
	PermissionMediaDelete:       "删除附件",
	PermissionProfileRead:       "读取个人资料",
	PermissionProfileWrite:      "修改个人资料",
	PermissionNotificationsRead: "读取通知",
	PermissionSystemManage:      "管理系统设置",
	PermissionUsersManage:       "管理用户",
	PermissionModulesManage:     "管理功能模块",
	PermissionPluginsManage:     "管理插件",
	PermissionWebhookRegister:   "注册通知 webhook",
	PermissionPluginScript:      "运行脚本插件",
	PermissionNetworkFetch:      "允许插件访问白名单网络",
}

var rolePermissions = map[string][]string{
	RoleReader: {
		PermissionProfileRead, PermissionProfileWrite, PermissionNotificationsRead,
	},
	RoleAuthor: {
		PermissionProfileRead, PermissionProfileWrite, PermissionNotificationsRead,
		PermissionContentRead, PermissionContentWrite, PermissionContentDelete,
		PermissionMediaRead, PermissionMediaWrite, PermissionMediaDelete,
	},
	RoleAdmin: {"*"},
}

// EnsureAuthorizationTables creates the role/permission graph and migrates
// users.role into user_roles. The legacy column is retained for old database
// readers, but all new authorization reads use the association tables.
func (s *Store) EnsureAuthorizationTables() error {
	for _, table := range []string{"roles", "permissions", "role_permissions", "user_roles"} {
		if err := s.ensureTable(table); err != nil {
			return fmt.Errorf("ensure authorization table %s: %w", table, err)
		}
	}
	for role, description := range roleDescriptions {
		if _, err := s.db.Exec(`INSERT INTO roles(name, description) VALUES(?, ?) ON CONFLICT(name) DO NOTHING`, role, description); err != nil {
			return fmt.Errorf("seed role %s: %w", role, err)
		}
	}
	for permission, description := range permissionDescriptions {
		if _, err := s.db.Exec(`INSERT INTO permissions(name, description) VALUES(?, ?) ON CONFLICT(name) DO NOTHING`, permission, description); err != nil {
			return fmt.Errorf("seed permission %s: %w", permission, err)
		}
	}
	for role, permissions := range rolePermissions {
		for _, permission := range permissions {
			if _, err := s.db.Exec(`INSERT INTO role_permissions(role, permission) VALUES(?, ?) ON CONFLICT(role, permission) DO NOTHING`, role, permission); err != nil {
				return fmt.Errorf("seed role permission %s/%s: %w", role, permission, err)
			}
		}
	}

	rows, err := s.db.Query(`SELECT id, role FROM users`)
	if err != nil {
		return fmt.Errorf("read legacy user roles: %w", err)
	}
	type legacyUserRole struct {
		id   int64
		role string
	}
	legacy := []legacyUserRole{}
	for rows.Next() {
		var user legacyUserRole
		if err := rows.Scan(&user.id, &user.role); err != nil {
			return fmt.Errorf("scan legacy user role: %w", err)
		}
		legacy = append(legacy, user)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close legacy user roles: %w", err)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, user := range legacy {
		role := user.role
		if !ValidRole(role) {
			role = RoleAuthor
		}
		if _, err := s.db.Exec(`INSERT INTO user_roles(user_id, role, is_primary) VALUES(?, ?, 1) ON CONFLICT(user_id, role) DO UPDATE SET is_primary = 1`, user.id, role); err != nil {
			return fmt.Errorf("migrate user role %d: %w", user.id, err)
		}
	}
	return nil
}

func (s *Store) GetUserRoles(userID int64) ([]string, error) {
	rows, err := s.db.Query(`SELECT role FROM user_roles WHERE user_id = ? ORDER BY is_primary DESC, role`, userID)
	if err != nil {
		return nil, fmt.Errorf("get roles for user %d: %w", userID, err)
	}
	defer rows.Close()
	roles := []string{}
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, fmt.Errorf("scan role for user %d: %w", userID, err)
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (s *Store) GetUserPermissions(userID int64) ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT rp.permission
		FROM user_roles ur JOIN role_permissions rp ON rp.role = ur.role
		WHERE ur.user_id = ? ORDER BY rp.permission`, userID)
	if err != nil {
		return nil, fmt.Errorf("get permissions for user %d: %w", userID, err)
	}
	defer rows.Close()
	permissions := []string{}
	for rows.Next() {
		var permission string
		if err := rows.Scan(&permission); err != nil {
			return nil, fmt.Errorf("scan permission for user %d: %w", userID, err)
		}
		permissions = append(permissions, permission)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return permissions, nil
}

func HasPermission(permissions []string, required string) bool {
	for _, permission := range permissions {
		if permission == "*" || permission == required {
			return true
		}
	}
	return false
}

func (s *Store) UserHasPermission(userID int64, required string) (bool, error) {
	permissions, err := s.GetUserPermissions(userID)
	if err != nil {
		return false, err
	}
	return HasPermission(permissions, required), nil
}

func (s *Store) SetUserRoles(userID int64, roles []string) error {
	if len(roles) == 0 {
		return fmt.Errorf("user %d must have at least one role", userID)
	}
	clean := make([]string, 0, len(roles))
	seen := map[string]bool{}
	for _, role := range roles {
		if !ValidRole(role) || seen[role] {
			continue
		}
		seen[role] = true
		clean = append(clean, role)
	}
	if len(clean) == 0 {
		return fmt.Errorf("user %d has no valid roles", userID)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin user role update: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM user_roles WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("clear roles for user %d: %w", userID, err)
	}
	for i, role := range clean {
		if _, err := tx.Exec(`INSERT INTO user_roles(user_id, role, is_primary) VALUES(?, ?, ?)`, userID, role, authBoolInt(i == 0)); err != nil {
			return fmt.Errorf("insert role %s for user %d: %w", role, userID, err)
		}
	}
	if _, err := tx.Exec(`UPDATE users SET role = ? WHERE id = ?`, clean[0], userID); err != nil {
		return fmt.Errorf("update legacy role for user %d: %w", userID, err)
	}
	return tx.Commit()
}

func authBoolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (s *Store) hydrateAuthorization(user *User) error {
	if user == nil {
		return nil
	}
	roles, err := s.GetUserRoles(user.ID)
	if err != nil {
		return err
	}
	if len(roles) > 0 {
		user.Roles = roles
		user.Role = roles[0]
	}
	permissions, err := s.GetUserPermissions(user.ID)
	if err != nil {
		return err
	}
	user.Permissions = permissions
	return nil
}
