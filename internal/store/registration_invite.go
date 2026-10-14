package store

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrInvalidRegistrationInvite 不区分无效、过期或已使用，避免暴露邀请码状态。
var ErrInvalidRegistrationInvite = errors.New("invalid registration invite")

// CreateRegistrationInvite 仅返回一次明文，持久化其 SHA256 摘要和审计信息。
// 有效期由管理员入口选择；建表只通过正常 EnsureSchema 完成。
// 创建者和使用者 ID 是保留的审计记录，用户删除不会清除消费状态。
func (s *Store) CreateRegistrationInvite(createdBy int64, expiresAt time.Time) (string, error) {
	now := time.Now().UTC()
	if expiresAt.UnixMilli() <= now.UnixMilli() {
		return "", fmt.Errorf("registration invite expiry must be in the future")
	}
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("generate registration invite: %w", err)
	}
	code := base64.RawURLEncoding.EncodeToString(secret)
	result, err := s.db.Exec(`INSERT INTO registration_invites(code_hash, created_by, created_at, expires_at)
		SELECT ?, id, ?, ? FROM users WHERE id = ? AND status = ?
		AND EXISTS (SELECT 1 FROM user_roles WHERE user_id = users.id AND role = ?)`,
		registrationInviteHash(code), now.UnixMilli(), expiresAt.UnixMilli(), createdBy, UserStatusActive, RoleAdmin)
	if err != nil {
		return "", fmt.Errorf("create registration invite: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return "", fmt.Errorf("create registration invite rows affected: %w", err)
	}
	if n != 1 {
		return "", fmt.Errorf("registration invite creator must be an active administrator")
	}
	return code, nil
}

// CreateUserWithRegistrationInvite 原子创建读者及角色关系，并消耗一次性邀请码。
// 先写入领取条件以串行化多个连接竞争；创建或角色写入失败均回滚领取。
func (s *Store) CreateUserWithRegistrationInvite(user *User, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return ErrInvalidRegistrationInvite
	}
	if user == nil {
		return fmt.Errorf("registration user is required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin registration: %w", err)
	}
	defer tx.Rollback()
	hash := registrationInviteHash(code)
	// SQLite 在取得写锁后计算当前时间，避免等待竞争锁期间已过期的码被领取。
	result, err := tx.Exec(`UPDATE registration_invites SET used_at = CAST(unixepoch('subsec') * 1000 AS INTEGER)
		WHERE code_hash = ? AND used_at IS NULL AND expires_at > CAST(unixepoch('subsec') * 1000 AS INTEGER)`, hash)
	if err != nil {
		return fmt.Errorf("claim registration invite: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("claim registration invite rows affected: %w", err)
	}
	if n != 1 {
		return ErrInvalidRegistrationInvite
	}
	reader := *user
	reader.ID, reader.CreatedAt = 0, time.Now().UTC()
	reader.Role, reader.Status = RoleReader, UserStatusActive
	reader.Roles = []string{RoleReader}
	reader.Permissions = append([]string(nil), rolePermissions[RoleReader]...)
	reader.TOTPSecret, reader.TOTPRecoveryCodes, reader.APITokenHash = "", "[]", ""
	columns := reader.Columns()[1:]
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",")
	result, err = tx.Exec("INSERT INTO users ("+strings.Join(columns, ",")+") VALUES ("+placeholders+")", reader.Values()...)
	if err != nil {
		return fmt.Errorf("create registration user: %w", err)
	}
	reader.ID, err = result.LastInsertId()
	if err != nil {
		return fmt.Errorf("registration user id: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO user_roles(user_id, role, is_primary) VALUES(?, ?, 1)`, reader.ID, RoleReader); err != nil {
		return fmt.Errorf("create registration reader role: %w", err)
	}
	if _, err := tx.Exec(`UPDATE registration_invites SET used_by = ? WHERE code_hash = ?`, reader.ID, hash); err != nil {
		return fmt.Errorf("record registration invite user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit registration: %w", err)
	}
	*user = reader
	return nil
}

func registrationInviteHash(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}
