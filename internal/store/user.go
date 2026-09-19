package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// 用户角色常量（多用户体系）：admin=超级管理员（全部权限+用户管理），author=作者（内容管理），reader=注册读者（评论带验证标识）。
const (
	RoleAdmin  = "admin"
	RoleAuthor = "author"
	RoleReader = "reader"
)

// ValidRole 判断角色值是否合法。
func ValidRole(role string) bool {
	return role == RoleAdmin || role == RoleAuthor || role == RoleReader
}

// 用户状态常量（Spec 71 评论用户管理）：active=正常，banned=封禁（无法登录与评论）。
const (
	UserStatusActive = "active"
	UserStatusBanned = "banned"
)

// ValidUserStatus 判断用户状态值是否合法。
func ValidUserStatus(status string) bool {
	return status == UserStatusActive || status == UserStatusBanned
}

// User 用户模型（多用户：admin 管理全部，author 管理内容）。
type User struct {
	ID                int64     `json:"id"`
	Username          string    `json:"username"`
	PasswordHash      string    `json:"-"`
	DisplayName       string    `json:"display_name"`
	Email             string    `json:"email"`
	Avatar            string    `json:"avatar"`
	TOTPSecret        string    `json:"-"`
	TOTPRecoveryCodes string    `json:"-"` // JSON array of strings
	AuthSecret        string    `json:"-"` // 随机 32 字节 hex，cookie 签名密钥
	APITokenHash      string    `json:"-"` // API Token 的 sha256 hex，空=未生成
	Role              string    `json:"role"`
	Roles             []string  `json:"roles,omitempty"`
	Permissions       []string  `json:"permissions,omitempty"`
	Status            string    `json:"status"` // active / banned（Spec 71）
	CreatedAt         time.Time `json:"created_at"`
}

// TableName Entity 接口：users 表。
func (u *User) TableName() string { return "users" }

// Columns Entity 接口：全列，首列为 id（含敏感列，登录/校验路径用）。
func (u *User) Columns() []string {
	return []string{"id", "username", "password_hash", "display_name", "email", "avatar", "totp_secret", "totp_recovery_codes", "auth_secret", "api_token_hash", "role", "status", "created_at"}
}

// Values Entity 接口：写入值（不含 id，与 Columns()[1:] 对应）。
func (u *User) Values() []any {
	return []any{u.Username, u.PasswordHash, u.DisplayName, u.Email, u.Avatar, u.TOTPSecret, u.TOTPRecoveryCodes, u.AuthSecret, u.APITokenHash, u.Role, u.Status, u.CreatedAt}
}

// SetID Entity 接口：回填自增 id。
func (u *User) SetID(id int64) { u.ID = id }

// ScanRow Entity 接口：行扫描，顺序对齐 Columns()。
func (u *User) ScanRow(row rowScanner) error {
	return row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Email, &u.Avatar, &u.TOTPSecret, &u.TOTPRecoveryCodes, &u.AuthSecret, &u.APITokenHash, &u.Role, &u.Status, &u.CreatedAt)
}

// EnsureUsersTable 独立建 users 表（统一走 schema.go 注册表）。
// 老库补列迁移：api_token_hash 为后续新增列，建表后检查缺失则 ALTER 补上（幂等）。
func (s *Store) EnsureUsersTable() error {
	if err := s.ensureTable("users"); err != nil {
		return err
	}
	for _, col := range []struct{ name, ddl string }{
		{"api_token_hash", `ALTER TABLE users ADD COLUMN api_token_hash TEXT NOT NULL DEFAULT ''`},
		{"role", `ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'author'`},
		{"status", `ALTER TABLE users ADD COLUMN status TEXT NOT NULL DEFAULT 'active'`},
	} {
		has, err := s.tableHasColumn("users", col.name)
		if err != nil {
			return err
		}
		if !has {
			if _, err := s.db.Exec(col.ddl); err != nil {
				return fmt.Errorf("migrate users add %s: %w", col.name, err)
			}
			// role 列补迁移：老库既有用户全部视为管理员（多用户引入前能登录的账号即管理员）。
			if col.name == "role" {
				if _, err := s.db.Exec(`UPDATE users SET role = ? WHERE role = ?`, RoleAdmin, RoleAuthor); err != nil {
					return fmt.Errorf("migrate users role to admin: %w", err)
				}
			}
		}
	}
	return s.EnsureAuthorizationTables()
}

// tableHasColumn 检查表是否存在指定列（PRAGMA table_info 查询）。
func (s *Store) tableHasColumn(table, column string) (bool, error) {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, fmt.Errorf("table info %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, pk int
		var name, ctype string
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			return false, fmt.Errorf("scan table info %s: %w", table, err)
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

// GetUserByUsername 按用户名查用户（含密码哈希等敏感列）。未找到返回 nil, nil。
func (s *Store) GetUserByUsername(username string) (*User, error) {
	u, err := repo(s, &User{}).GetOne("username = ?", username)
	if err != nil {
		return nil, fmt.Errorf("get user by username %s: %w", username, err)
	}
	if err := s.hydrateAuthorization(u); err != nil {
		return nil, err
	}
	return u, nil
}

// GetUserByID 按 ID 查用户（含密码哈希等敏感列）。未找到返回 nil, nil。
func (s *Store) GetUserByID(id int64) (*User, error) {
	u, err := repo(s, &User{}).GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("get user by id %d: %w", id, err)
	}
	if err := s.hydrateAuthorization(u); err != nil {
		return nil, err
	}
	return u, nil
}

// CreateUser 新建用户，LastInsertId 回填 user.ID。用户名冲突（UNIQUE）返回错误。
func (s *Store) CreateUser(user *User) error {
	now := time.Now().UTC()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	if user.TOTPRecoveryCodes == "" {
		user.TOTPRecoveryCodes = "[]"
	}
	if !ValidRole(user.Role) {
		user.Role = RoleAuthor // 未显式指定角色的新用户一律按作者（最小权限）
	}
	if user.Status == "" {
		user.Status = UserStatusActive
	}
	if _, err := repo(s, &User{}).Create(user); err != nil {
		return fmt.Errorf("create user %s: %w", user.Username, err)
	}
	if err := s.EnsureAuthorizationTables(); err != nil {
		return err
	}
	return s.SetUserRoles(user.ID, []string{user.Role})
}

// UpdateUser 更新用户资料字段（display_name/email/avatar），不动认证相关列。
func (s *Store) UpdateUser(user *User) error {
	res, err := s.db.Exec(`UPDATE users SET display_name = ?, email = ?, avatar = ? WHERE id = ?`,
		user.DisplayName, user.Email, user.Avatar, user.ID)
	if err != nil {
		return fmt.Errorf("update user %d: %w", user.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("update user %d: no rows affected", user.ID)
	}
	return nil
}

// UpdatePassword 更新密码哈希并轮换 auth_secret（cookie 签名密钥）。
func (s *Store) UpdatePassword(userID int64, newHash, newAuthSecret string) error {
	if _, err := s.db.Exec(`UPDATE users SET password_hash = ?, auth_secret = ? WHERE id = ?`,
		newHash, newAuthSecret, userID); err != nil {
		return fmt.Errorf("update password for user %d: %w", userID, err)
	}
	return nil
}

// UpdateTOTPSecret 更新 TOTP 密钥与恢复码（恢复码 JSON 序列化存储）。
func (s *Store) UpdateTOTPSecret(userID int64, secret string, codes []string) error {
	codesJSON := "[]"
	if b, err := json.Marshal(codes); err == nil {
		codesJSON = string(b)
	}
	if _, err := s.db.Exec(`UPDATE users SET totp_secret = ?, totp_recovery_codes = ? WHERE id = ?`,
		secret, codesJSON, userID); err != nil {
		return fmt.Errorf("update totp for user %d: %w", userID, err)
	}
	return nil
}

// userListCols 管理端列表列（不含密码/2FA/签名密钥等敏感列）。
const userListCols = "id, username, display_name, email, avatar, role, status, created_at"

// ListUsers 返回全部用户（仅非敏感列，供管理端展示）。
func (s *Store) ListUsers() ([]*User, error) {
	rows, err := s.db.Query("SELECT " + userListCols + " FROM users ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		u := &User{}
		if err := scanUserList(rows, u); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close users: %w", err)
	}
	for _, user := range out {
		if err := s.hydrateAuthorization(user); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// scanUserList 按 userListCols 顺序扫描一行用户列表记录。
func scanUserList(row rowScanner, u *User) error {
	return row.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Avatar, &u.Role, &u.Status, &u.CreatedAt)
}

// ListUsersPage 分页查询用户（keyword 匹配 username/display_name/email，空=全部）。
// 仅返回非敏感列；按 id 升序。
func (s *Store) ListUsersPage(page, size int, keyword string) ([]*User, int, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	where := ""
	var args []any
	if kw := strings.TrimSpace(keyword); kw != "" {
		where = " WHERE username LIKE ? OR display_name LIKE ? OR email LIKE ?"
		like := "%" + kw + "%"
		args = append(args, like, like, like)
	}
	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}
	rows, err := s.db.Query("SELECT "+userListCols+" FROM users"+where+" ORDER BY id LIMIT ? OFFSET ?",
		append(args, size, (page-1)*size)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list users page: %w", err)
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		u := &User{}
		if err := scanUserList(rows, u); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate users: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, 0, fmt.Errorf("close users: %w", err)
	}
	for _, user := range out {
		if err := s.hydrateAuthorization(user); err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

// CountUsersByRole 按角色统计用户数（防锁死：最后一个 admin 不可删除/降级）。
func (s *Store) CountUsersByRole(role string) (int, error) {
	var n int
	if err := s.db.QueryRow("SELECT COUNT(DISTINCT user_id) FROM user_roles WHERE role = ?", role).Scan(&n); err != nil {
		return 0, fmt.Errorf("count users by role %s: %w", role, err)
	}
	return n, nil
}

// UpdateUserRole 更新用户角色并返回受影响行数（行数=0 表示用户不存在）。
func (s *Store) UpdateUserRole(userID int64, role string) error {
	if !ValidRole(role) {
		return fmt.Errorf("invalid role %q", role)
	}
	if err := s.EnsureAuthorizationTables(); err != nil {
		return err
	}
	return s.SetUserRoles(userID, []string{role})
}

// DeleteUser 删除用户（用户存在且删除成功返回 nil；不存在返回错误）。
func (s *Store) DeleteUser(userID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin delete user %d: %w", userID, err)
	}
	defer tx.Rollback()

	// post_authors deliberately uses ON DELETE RESTRICT so article ownership
	// cannot disappear accidentally. User deletion is the explicit ownership
	// removal path: clear only the association and keep posts.author unchanged
	// so existing article bylines remain visible.
	var authorsTable int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'post_authors'`).Scan(&authorsTable); err != nil {
		return fmt.Errorf("check post_authors before deleting user %d: %w", userID, err)
	}
	if authorsTable > 0 {
		if _, err := tx.Exec(`DELETE FROM post_authors WHERE author_id = ?`, userID); err != nil {
			return fmt.Errorf("delete post authors for user %d: %w", userID, err)
		}
	}

	res, err := tx.Exec(`DELETE FROM users WHERE id = ?`, userID)
	if err != nil {
		return fmt.Errorf("delete user %d: %w", userID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("delete user %d: no rows affected", userID)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete user %d: %w", userID, err)
	}
	return nil
}

// HashPassword bcrypt 哈希（cost=10）。
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	return string(hash), err
}

// CheckPassword 校验明文密码与 bcrypt 哈希是否匹配。
func CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// GenerateAuthSecret 生成 32 字节随机数的 hex 串（cookie 签名密钥）。
// crypto/rand.Read 永不返回错误（Go 1.24+），无需处理返回值。
func GenerateAuthSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GenerateAPIToken 生成 API Token 明文（32 字节随机数 base64url，无填充）。
// 明文只展示一次：入库前必须先用 HashAPIToken 转为 sha256 hex。
func GenerateAPIToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// HashAPIToken 计算 API Token 的 sha256 hex（存储与比对均用哈希，库泄漏不暴露明文）。
func HashAPIToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// SetAPITokenHash 写入/轮换/撤销 API Token 哈希（传空串即撤销）。
func (s *Store) SetAPITokenHash(userID int64, hash string) error {
	if _, err := s.db.Exec(`UPDATE users SET api_token_hash = ? WHERE id = ?`, hash, userID); err != nil {
		return fmt.Errorf("set api token for user %d: %w", userID, err)
	}
	return nil
}

// GetUserByAPITokenHash 按 API Token 哈希查用户（Bearer 认证用）。未命中返回 nil, nil。
func (s *Store) GetUserByAPITokenHash(hash string) (*User, error) {
	u, err := repo(s, &User{}).GetOne("api_token_hash = ?", hash)
	if err != nil {
		return nil, fmt.Errorf("get user by api token: %w", err)
	}
	return u, nil
}

// UpdateUserStatus 更新用户状态（active/banned）并返回受影响行数（0=用户不存在）。
// 当状态更新为 banned 时，事务内同步轮换 auth_secret 并清空 api_token_hash，
// 使既有会话 cookie 与 Bearer 令牌即时失效；解封后历史凭据不可复用。
func (s *Store) UpdateUserStatus(userID int64, status string) error {
	if !ValidUserStatus(status) {
		return fmt.Errorf("invalid user status %q", status)
	}
	var res sql.Result
	var err error
	if status == UserStatusBanned {
		newSecret := GenerateAuthSecret()
		res, err = s.db.Exec(`UPDATE users SET status = ?, auth_secret = ?, api_token_hash = '' WHERE id = ?`, status, newSecret, userID)
	} else {
		res, err = s.db.Exec(`UPDATE users SET status = ? WHERE id = ?`, status, userID)
	}
	if err != nil {
		return fmt.Errorf("update user %d status: %w", userID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("update user %d status: no rows affected", userID)
	}
	return nil
}

// CountUsersByStatus 按状态统计用户数（status 空串=全部）。
func (s *Store) CountUsersByStatus(status string) (int, error) {
	var n int
	var err error
	if status == "" {
		err = s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&n)
	} else {
		err = s.db.QueryRow("SELECT COUNT(*) FROM users WHERE status = ?", status).Scan(&n)
	}
	if err != nil {
		return 0, fmt.Errorf("count users by status %q: %w", status, err)
	}
	return n, nil
}

// CommentUser 评论用户列表行（reader 用户 + 评论聚合统计，Spec 71）。
type CommentUser struct {
	User
	CommentCount  int       // 名下评论总数
	LastCommentAt time.Time // 最后评论时间，零值=从未评论
}

// parseCommentTime 解析 MAX(created_at) 聚合结果：聚合函数丢失列类型声明，
// modernc 驱动回退为字符串（time.Time 入库为 Go 默认 String() 格式，含单调钟后缀），
// 需剪掉 " m=..." 后多格式兼容解析；解析失败返回零值。
func parseCommentTime(s string) time.Time {
	if i := strings.Index(s, " m="); i >= 0 {
		s = s[:i]
	}
	layouts := []string{
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999999-07:00",
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// CountCommentUsers 统计注册读者数（status 空串=全部，非法值按全部计）。
func (s *Store) CountCommentUsers(status string) (int, error) {
	query := "SELECT COUNT(*) FROM users u JOIN user_roles ur ON ur.user_id = u.id AND ur.is_primary = 1 WHERE ur.role = ?"
	args := []any{RoleReader}
	if ValidUserStatus(status) {
		query += " AND status = ?"
		args = append(args, status)
	}
	var n int
	if err := s.db.QueryRow(query, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("count comment users by status %q: %w", status, err)
	}
	return n, nil
}

// ListCommentUsers 分页查询注册读者（role=reader）及其评论聚合（LEFT JOIN comments）。
// status 空串=全部；keyword 匹配 username/display_name/email。
// 排序：最后评论时间倒序（从未评论者殿后），同级按 id 升序。
func (s *Store) ListCommentUsers(page, size int, keyword, status string) ([]*CommentUser, int, error) {
	if err := s.ensureCommentReactions(); err != nil { // 依赖 comments.user_id 补列迁移
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	conds := []string{"ur.role = ?", "ur.is_primary = 1"}
	args := []any{RoleReader}
	if ValidUserStatus(status) {
		conds = append(conds, "u.status = ?")
		args = append(args, status)
	}
	if kw := strings.TrimSpace(keyword); kw != "" {
		conds = append(conds, "(u.username LIKE ? OR u.display_name LIKE ? OR u.email LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like, like)
	}
	where := " WHERE " + strings.Join(conds, " AND ")

	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users u JOIN user_roles ur ON ur.user_id = u.id"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count comment users: %w", err)
	}
	rows, err := s.db.Query(`SELECT u.id, u.username, u.display_name, u.email, u.avatar, ur.role, u.status, u.created_at,
		COUNT(cm.id) AS comment_count, MAX(cm.created_at) AS last_comment_at
		FROM users u JOIN user_roles ur ON ur.user_id = u.id LEFT JOIN comments cm ON cm.user_id = u.id`+where+`
		GROUP BY u.id
		ORDER BY last_comment_at IS NULL, last_comment_at DESC, u.id ASC
		LIMIT ? OFFSET ?`,
		append(args, size, (page-1)*size)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list comment users: %w", err)
	}
	defer rows.Close()
	var out []*CommentUser
	for rows.Next() {
		cu := &CommentUser{}
		var last sql.NullString
		if err := rows.Scan(&cu.ID, &cu.Username, &cu.DisplayName, &cu.Email, &cu.Avatar, &cu.Role, &cu.Status, &cu.CreatedAt,
			&cu.CommentCount, &last); err != nil {
			return nil, 0, fmt.Errorf("scan comment user: %w", err)
		}
		if last.Valid {
			cu.LastCommentAt = parseCommentTime(last.String)
		}
		out = append(out, cu)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate comment users: %w", err)
	}
	return out, total, nil
}

// CurrentUser 注入到请求上下文的当前用户信息（匿名或已登录）。
// 被 api / comment 两个包共享，避免循环依赖。
type CurrentUser struct {
	ID       int64  // 0=匿名
	Username string // 匿名空串
	Nickname string // display_name 或 username
	Email    string // 匿名空串
	Link     string // 预留
	Avatar   string // 匿名空串
	Role     string // 空=匿名，否则 Role*
	Verified bool   // true=已登录注册用户（评论带认证徽章）
}
