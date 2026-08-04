package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User 用户模型（单主账户，表结构为未来多用户预留）。
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
	CreatedAt         time.Time `json:"created_at"`
}

// TableName Entity 接口：users 表。
func (u *User) TableName() string { return "users" }

// Columns Entity 接口：全列，首列为 id（含敏感列，登录/校验路径用）。
func (u *User) Columns() []string {
	return []string{"id", "username", "password_hash", "display_name", "email", "avatar", "totp_secret", "totp_recovery_codes", "auth_secret", "created_at"}
}

// Values Entity 接口：写入值（不含 id，与 Columns()[1:] 对应）。
func (u *User) Values() []any {
	return []any{u.Username, u.PasswordHash, u.DisplayName, u.Email, u.Avatar, u.TOTPSecret, u.TOTPRecoveryCodes, u.AuthSecret, u.CreatedAt}
}

// SetID Entity 接口：回填自增 id。
func (u *User) SetID(id int64) { u.ID = id }

// ScanRow Entity 接口：行扫描，顺序对齐 Columns()。
func (u *User) ScanRow(row rowScanner) error {
	return row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Email, &u.Avatar, &u.TOTPSecret, &u.TOTPRecoveryCodes, &u.AuthSecret, &u.CreatedAt)
}

// EnsureUsersTable 独立建 users 表（统一走 schema.go 注册表）。
func (s *Store) EnsureUsersTable() error {
	return s.ensureTable("users")
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

// ListUsers 返回全部用户（仅非敏感列，供管理端展示）。
func (s *Store) ListUsers() ([]*User, error) {
	rows, err := s.db.Query("SELECT id, username, display_name, email, avatar, created_at FROM users ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Avatar, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return out, nil
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
