package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// newUserStore 建库并确保 users 表。
func newUserStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := s.EnsureUsersTable(); err != nil {
		t.Fatalf("ensure users table: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// newTestUser 便捷构造一个用户（bcrypt 哈希真实生成，cost=10 可接受）。
func newTestUser(username, password string) *User {
	hash, _ := HashPassword(password)
	return &User{
		Username:     username,
		PasswordHash: hash,
		DisplayName:  "站长",
		Email:        username + "@example.com",
		AuthSecret:   GenerateAuthSecret(),
	}
}

// TestEnsureUsersTableIdempotent 重复建表应幂等（旧库不报错）。
func TestEnsureUsersTableIdempotent(t *testing.T) {
	s := newUserStore(t)
	for i := 0; i < 3; i++ {
		if err := s.EnsureUsersTable(); err != nil {
			t.Fatalf("ensure users table (iter %d): %v", i, err)
		}
	}
}

// TestUserCreateAndGet CreateUser 回填 ID，GetUserByUsername/GetUserByID 可查回全部字段。
func TestUserCreateAndGet(t *testing.T) {
	s := newUserStore(t)
	u := newTestUser("admin", "secret123")
	if err := s.CreateUser(u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if u.ID == 0 {
		t.Fatal("expected non-zero id")
	}
	if u.CreatedAt.IsZero() {
		t.Fatal("expected created_at set")
	}

	byName, err := s.GetUserByUsername("admin")
	if err != nil || byName == nil {
		t.Fatalf("get by username: %v, nil=%v", err, byName == nil)
	}
	if byName.ID != u.ID || byName.DisplayName != "站长" || byName.Email != "admin@example.com" {
		t.Fatalf("field mismatch: %+v", byName)
	}
	if byName.PasswordHash == "" || byName.AuthSecret == "" {
		t.Fatalf("sensitive columns should be loaded, got %+v", byName)
	}
	if byName.PasswordHash != u.PasswordHash || byName.AuthSecret != u.AuthSecret {
		t.Fatal("password hash / auth secret mismatch")
	}

	byID, err := s.GetUserByID(u.ID)
	if err != nil || byID == nil {
		t.Fatalf("get by id: %v, nil=%v", err, byID == nil)
	}
	if byID.Username != "admin" {
		t.Fatalf("by id username = %q, want admin", byID.Username)
	}
}

// TestUserGetMissing 未找到的用户返回 nil, nil（不报错）。
func TestUserGetMissing(t *testing.T) {
	s := newUserStore(t)
	if got, err := s.GetUserByUsername("ghost"); err != nil || got != nil {
		t.Fatalf("get missing username: %v, nil=%v", err, got == nil)
	}
	if got, err := s.GetUserByID(999); err != nil || got != nil {
		t.Fatalf("get missing id: %v, nil=%v", err, got == nil)
	}
}

// TestUserCreateDuplicateUsername username UNIQUE 约束，重复创建报错。
func TestUserCreateDuplicateUsername(t *testing.T) {
	s := newUserStore(t)
	if err := s.CreateUser(newTestUser("admin", "a")); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if err := s.CreateUser(newTestUser("admin", "b")); err == nil {
		t.Fatal("expected duplicate username error, got nil")
	}
}

// TestUserUpdate UpdateUser 更新资料字段，不动认证相关列。
func TestUserUpdate(t *testing.T) {
	s := newUserStore(t)
	u := newTestUser("admin", "secret123")
	if err := s.CreateUser(u); err != nil {
		t.Fatalf("create: %v", err)
	}
	oldHash, oldSecret := u.PasswordHash, u.AuthSecret

	u.DisplayName = "新昵称"
	u.Email = "new@example.com"
	u.Avatar = "/uploads/avatar.png"
	if err := s.UpdateUser(u); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := s.GetUserByID(u.ID)
	if err != nil || got == nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.DisplayName != "新昵称" || got.Email != "new@example.com" || got.Avatar != "/uploads/avatar.png" {
		t.Fatalf("update not applied: %+v", got)
	}
	if got.PasswordHash != oldHash || got.AuthSecret != oldSecret {
		t.Fatal("update must not touch password_hash / auth_secret")
	}

	// 不存在的用户报错
	ghost := &User{ID: 999, DisplayName: "x"}
	if err := s.UpdateUser(ghost); err == nil {
		t.Fatal("expected error updating non-existent user")
	}
}

// TestUserUpdatePassword UpdatePassword 替换哈希并轮换 auth_secret。
func TestUserUpdatePassword(t *testing.T) {
	s := newUserStore(t)
	u := newTestUser("admin", "old-pass")
	if err := s.CreateUser(u); err != nil {
		t.Fatalf("create: %v", err)
	}
	newHash, err := HashPassword("new-pass")
	if err != nil {
		t.Fatalf("hash new password: %v", err)
	}
	newSecret := GenerateAuthSecret()
	if err := s.UpdatePassword(u.ID, newHash, newSecret); err != nil {
		t.Fatalf("update password: %v", err)
	}
	got, _ := s.GetUserByID(u.ID)
	if got.PasswordHash != newHash {
		t.Fatal("password hash not updated")
	}
	if got.AuthSecret != newSecret {
		t.Fatal("auth secret not rotated")
	}
	if err := CheckPassword(got.PasswordHash, "new-pass"); err != nil {
		t.Fatalf("new password should verify: %v", err)
	}
	if err := CheckPassword(got.PasswordHash, "old-pass"); err == nil {
		t.Fatal("old password should fail")
	}
}

// TestUserUpdateTOTPSecret TOTP 密钥 + 恢复码 JSON 持久化。
func TestUserUpdateTOTPSecret(t *testing.T) {
	s := newUserStore(t)
	u := newTestUser("admin", "secret123")
	if err := s.CreateUser(u); err != nil {
		t.Fatalf("create: %v", err)
	}
	codes := []string{"AAAA-BBBB", "CCCC-DDDD"}
	if err := s.UpdateTOTPSecret(u.ID, "JBSWY3DPEHPK3PXP", codes); err != nil {
		t.Fatalf("update totp: %v", err)
	}
	got, _ := s.GetUserByID(u.ID)
	if got.TOTPSecret != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("totp secret = %q", got.TOTPSecret)
	}
	var stored []string
	if err := json.Unmarshal([]byte(got.TOTPRecoveryCodes), &stored); err != nil {
		t.Fatalf("recovery codes not valid JSON: %v", err)
	}
	if len(stored) != 2 || stored[0] != "AAAA-BBBB" || stored[1] != "CCCC-DDDD" {
		t.Fatalf("recovery codes mismatch: %#v", stored)
	}
}

// TestListUsers 列表仅返回非敏感列，按 id 升序。
func TestListUsers(t *testing.T) {
	s := newUserStore(t)
	if err := s.CreateUser(newTestUser("alice", "a")); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	if err := s.CreateUser(newTestUser("bob", "b")); err != nil {
		t.Fatalf("create bob: %v", err)
	}
	users, err := s.ListUsers()
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("want 2 users, got %d", len(users))
	}
	if users[0].Username != "alice" || users[1].Username != "bob" {
		t.Fatalf("order/name mismatch: %+v", users)
	}
	if users[0].PasswordHash != "" || users[0].AuthSecret != "" || users[0].TOTPSecret != "" {
		t.Fatal("list must not expose sensitive columns")
	}
}

// TestHashPasswordAndCheckPassword bcrypt 哈希可验证，错误密码拒绝，哈希含盐随机。
func TestHashPasswordAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if err := CheckPassword(hash, "correct horse battery staple"); err != nil {
		t.Fatalf("correct password should verify: %v", err)
	}
	if err := CheckPassword(hash, "wrong"); err == nil {
		t.Fatal("wrong password should fail")
	}
	if err := CheckPassword("not-a-bcrypt-hash", "x"); err == nil {
		t.Fatal("invalid hash should error")
	}
	// 同一密码两次哈希应不同（bcrypt 盐）
	hash2, _ := HashPassword("correct horse battery staple")
	if hash == hash2 {
		t.Fatal("bcrypt hashes should differ (salt)")
	}
}

// TestGenerateAuthSecret 生成 64 位 hex（32 字节），两次调用不同。
func TestGenerateAuthSecret(t *testing.T) {
	a := GenerateAuthSecret()
	if len(a) != 64 {
		t.Fatalf("secret length = %d, want 64", len(a))
	}
	b := GenerateAuthSecret()
	if a == b {
		t.Fatal("two generated secrets should differ")
	}
}

// TestEnsureUsersTableMigratesAPITokenColumn 老库（无 api_token_hash 列）补列迁移：
// 列缺失时 ALTER 补上，已有数据保留；再次调用幂等。
func TestEnsureUsersTableMigratesAPITokenColumn(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	// 模拟 Spec 45 老库：手工建无 api_token_hash 列的 users 表并插入数据
	if _, err := s.db.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		display_name TEXT NOT NULL DEFAULT '',
		email TEXT NOT NULL DEFAULT '',
		avatar TEXT NOT NULL DEFAULT '',
		totp_secret TEXT NOT NULL DEFAULT '',
		totp_recovery_codes TEXT NOT NULL DEFAULT '[]',
		auth_secret TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create legacy users table: %v", err)
	}
	if _, err := s.db.Exec(`INSERT INTO users (username, password_hash, auth_secret) VALUES ('admin', 'x', 'legacy-secret')`); err != nil {
		t.Fatalf("seed legacy user: %v", err)
	}

	// 迁移：EnsureUsersTable 应补上 api_token_hash 列
	if err := s.EnsureUsersTable(); err != nil {
		t.Fatalf("migrate users table: %v", err)
	}
	has, err := s.tableHasColumn("users", "api_token_hash")
	if err != nil || !has {
		t.Fatalf("api_token_hash column should exist after migrate (has=%v err=%v)", has, err)
	}

	// 存量数据保留且新列默认空
	u, err := s.GetUserByUsername("admin")
	if err != nil || u == nil {
		t.Fatalf("legacy user should survive migrate: %v / %v", u, err)
	}
	if u.APITokenHash != "" {
		t.Errorf("new column should default empty, got %q", u.APITokenHash)
	}

	// 幂等：再次调用不报错
	if err := s.EnsureUsersTable(); err != nil {
		t.Fatalf("second migrate should be idempotent: %v", err)
	}
}

// TestAPITokenSetAndLookup SetAPITokenHash 写入后可被 GetUserByAPITokenHash 命中，
// 置空（撤销）后不再命中。
func TestAPITokenSetAndLookup(t *testing.T) {
	s := newUserStore(t)
	u := newTestUser("admin", "secret123")
	if err := s.CreateUser(u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	token := GenerateAPIToken()
	hash := HashAPIToken(token)
	if err := s.SetAPITokenHash(u.ID, hash); err != nil {
		t.Fatalf("set api token: %v", err)
	}

	found, err := s.GetUserByAPITokenHash(hash)
	if err != nil || found == nil || found.ID != u.ID {
		t.Fatalf("lookup by token hash: %v / %+v", err, found)
	}
	// 错误哈希查不到
	if miss, err := s.GetUserByAPITokenHash("deadbeef"); err != nil || miss != nil {
		t.Fatalf("wrong hash should miss: %v / %+v", err, miss)
	}

	// 撤销：置空后不再命中
	if err := s.SetAPITokenHash(u.ID, ""); err != nil {
		t.Fatalf("revoke api token: %v", err)
	}
	if miss, err := s.GetUserByAPITokenHash(hash); err != nil || miss != nil {
		t.Fatalf("revoked token should miss: %v / %+v", err, miss)
	}
}
