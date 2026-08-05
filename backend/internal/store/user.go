package store

import (
	"crypto/rand"
	"crypto/sha256"
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
	CreatedAt         time.Time `json:"created_at"`
}

// TableName Entity 接口：users 表。
func (u *User) TableName() string { return "users" }

// Columns Entity 接口：全列，首列为 id（含敏感列，登录/校验路径用）。
func (u *User) Columns() []string {
	return []string{"id", "username", "password_hash", "display_name", "email", "avatar", "totp_secret", "totp_recovery_codes", "auth_secret", "api_token_hash", "role", "created_at"}
}

// Values Entity 接口：写入值（不含 id，与 Columns()[1:] 对应）。
func (u *User) Values() []any {
	return []any{u.Username, u.PasswordHash, u.DisplayName, u.Email, u.Avatar, u.TOTPSecret, u.TOTPRecoveryCodes, u.AuthSecret, u.APITokenHash, u.Role, u.CreatedAt}
}

// SetID Entity 接口：回填自增 id。
func (u *User) SetID(id int64) { u.ID = id }

// ScanRow Entity 接口：行扫描，顺序对齐 Columns()。
func (u *User) ScanRow(row rowScanner) error {
	return row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Email, &u.Avatar, &u.TOTPSecret, &u.TOTPRecoveryCodes, &u.AuthSecret, &u.APITokenHash, &u.Role, &u.CreatedAt)
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
	return nil
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
	return u, nil
}

// GetUserByID 按 ID 查用户（含密码哈希等敏感列）。未找到返回 nil, nil。
func (s *Store) GetUserByID(id int64) (*User, error) {
	u, err := repo(s, &User{}).GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("get user by id %d: %w", id, err)
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
	if _, err := repo(s, &User{}).Create(user); err != nil {
		return fmt.Errorf("create user %s: %w", user.Username, err)
	}
	return nil
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
const userListCols = "id, username, display_name, email, avatar, role, created_at"

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
	return out, nil
}

// scanUserList 按 userListCols 顺序扫描一行用户列表记录。
func scanUserList(row rowScanner, u *User) error {
	return row.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Avatar, &u.Role, &u.CreatedAt)
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
	return out, total, nil
}

// CountUsersByRole 按角色统计用户数（防锁死：最后一个 admin 不可删除/降级）。
func (s *Store) CountUsersByRole(role string) (int, error) {
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users WHERE role = ?", role).Scan(&n); err != nil {
		return 0, fmt.Errorf("count users by role %s: %w", role, err)
	}
	return n, nil
}

// UpdateUserRole 更新用户角色并返回受影响行数（行数=0 表示用户不存在）。
func (s *Store) UpdateUserRole(userID int64, role string) error {
	res, err := s.db.Exec(`UPDATE users SET role = ? WHERE id = ?`, role, userID)
	if err != nil {
		return fmt.Errorf("update user %d role: %w", userID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("update user %d role: no rows affected", userID)
	}
	return nil
}

// DeleteUser 删除用户（用户存在且删除成功返回 nil；不存在返回错误）。
func (s *Store) DeleteUser(userID int64) error {
	res, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, userID)
	if err != nil {
		return fmt.Errorf("delete user %d: %w", userID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("delete user %d: no rows affected", userID)
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
