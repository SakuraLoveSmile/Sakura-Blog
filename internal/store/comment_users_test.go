package store

import (
	"path/filepath"
	"testing"
)

// Spec 71 评论用户管理测试：status 补列迁移 / 封禁状态更新 / 评论用户聚合查询 / 用户级批量处置。

// newUserCommentStore 建库并确保 users 表（评论相关表经 ensureCommentReactions 惰性创建）。
func newUserCommentStore(t *testing.T) *Store {
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

// seedReader 建一个 active 的 reader 用户。
func seedReader(t *testing.T, s *Store, username string) *User {
	t.Helper()
	u := newTestUser(username, "secret6")
	u.Role = RoleReader
	if err := s.CreateUser(u); err != nil {
		t.Fatalf("create reader %s: %v", username, err)
	}
	return u
}

// TestEnsureUsersTableMigratesStatusColumn 老库（无 status 列）补列迁移：
// 列缺失时 ALTER 补上且存量用户默认 active，再次调用幂等。
func TestEnsureUsersTableMigratesStatusColumn(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	// 模拟老库：无 status 列（含 role 的多用户形态）
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
		api_token_hash TEXT NOT NULL DEFAULT '',
		role TEXT NOT NULL DEFAULT 'author',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create legacy users table: %v", err)
	}
	if _, err := s.db.Exec(`INSERT INTO users (username, password_hash, auth_secret) VALUES ('legacy', 'x', 's')`); err != nil {
		t.Fatalf("seed legacy user: %v", err)
	}

	if err := s.EnsureUsersTable(); err != nil {
		t.Fatalf("migrate users table: %v", err)
	}
	has, err := s.tableHasColumn("users", "status")
	if err != nil || !has {
		t.Fatalf("status column should exist after migrate (has=%v err=%v)", has, err)
	}
	u, err := s.GetUserByUsername("legacy")
	if err != nil || u == nil {
		t.Fatalf("legacy user should survive migrate: %v / %v", u, err)
	}
	if u.Status != UserStatusActive {
		t.Errorf("legacy user status = %q, want active", u.Status)
	}
	// 幂等
	if err := s.EnsureUsersTable(); err != nil {
		t.Fatalf("second migrate should be idempotent: %v", err)
	}
}

// TestUserCreateDefaultStatus 新用户默认 active。
func TestUserCreateDefaultStatus(t *testing.T) {
	s := newUserCommentStore(t)
	u := seedReader(t, s, "reader1")
	if u.Status != UserStatusActive {
		t.Fatalf("new user status = %q, want active", u.Status)
	}
}

// TestUpdateUserStatusAndCount 封禁/解封更新与状态计数；非法值与不存在用户报错。
func TestUpdateUserStatusAndCount(t *testing.T) {
	s := newUserCommentStore(t)
	u := seedReader(t, s, "reader1")

	if err := s.UpdateUserStatus(u.ID, UserStatusBanned); err != nil {
		t.Fatalf("ban user: %v", err)
	}
	got, _ := s.GetUserByID(u.ID)
	if got == nil || got.Status != UserStatusBanned {
		t.Fatalf("status after ban = %+v, want banned", got)
	}
	if n, err := s.CountUsersByStatus(UserStatusBanned); err != nil || n != 1 {
		t.Errorf("count banned = %d (err %v), want 1", n, err)
	}
	if n, err := s.CountUsersByStatus(""); err != nil || n != 1 {
		t.Errorf("count all = %d (err %v), want 1", n, err)
	}

	if err := s.UpdateUserStatus(u.ID, UserStatusActive); err != nil {
		t.Fatalf("unban user: %v", err)
	}
	if n, err := s.CountUsersByStatus(UserStatusBanned); err != nil || n != 0 {
		t.Errorf("count banned after unban = %d, want 0", n)
	}

	if err := s.UpdateUserStatus(u.ID, "weird"); err == nil {
		t.Error("invalid status should error")
	}
	if err := s.UpdateUserStatus(99999, UserStatusBanned); err == nil {
		t.Error("missing user should error")
	}
}

// seedUserComment 落一条指定作者的评论，返回 id。
func seedUserComment(t *testing.T, s *Store, pageKey string, userID int64, status string) int64 {
	t.Helper()
	id, err := s.CreateComment(&Comment{
		PageKey: pageKey, Nick: "n", Email: "e@example.com",
		ContentMD: "hello", ContentHTML: "<p>hello</p>",
		Status: status, UserID: userID,
	})
	if err != nil {
		t.Fatalf("seed comment: %v", err)
	}
	return id
}

// TestListCommentUsersAggregation 评论用户聚合：评论数/最后评论时间正确，
// 无评论用户 comment_count=0 且殿后；status/keyword 过滤生效；非 reader 不出现。
func TestListCommentUsersAggregation(t *testing.T) {
	s := newUserCommentStore(t)
	active := seedReader(t, s, "active-reader")
	silent := seedReader(t, s, "silent-reader")
	// admin 不应出现在评论用户列表
	admin := newTestUser("admin", "secret6")
	admin.Role = RoleAdmin
	if err := s.CreateUser(admin); err != nil {
		t.Fatalf("create admin: %v", err)
	}

	seedUserComment(t, s, "/post/a", active.ID, CommentApproved)
	seedUserComment(t, s, "/post/b", active.ID, CommentPending)
	// 匿名评论（user_id=0）不计入任何用户
	seedUserComment(t, s, "/post/a", 0, CommentApproved)

	items, total, err := s.ListCommentUsers(1, 20, "", "")
	if err != nil {
		t.Fatalf("list comment users: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2 (readers only)", total)
	}
	if items[0].Username != "active-reader" {
		t.Errorf("first item = %q, want active-reader (most recent comment first)", items[0].Username)
	}
	if items[0].CommentCount != 2 || items[0].LastCommentAt.IsZero() {
		t.Errorf("active-reader aggregate = count:%d last:%v, want 2 and non-zero", items[0].CommentCount, items[0].LastCommentAt)
	}
	if items[1].Username != "silent-reader" || items[1].CommentCount != 0 || !items[1].LastCommentAt.IsZero() {
		t.Errorf("silent-reader aggregate = %+v, want count 0 and zero time", items[1])
	}

	// keyword 过滤
	items, total, err = s.ListCommentUsers(1, 20, "silent", "")
	if err != nil || total != 1 || items[0].Username != "silent-reader" {
		t.Errorf("keyword filter: total=%d items=%+v err=%v", total, items, err)
	}

	// status 过滤（当前无封禁用户）
	_, total, err = s.ListCommentUsers(1, 20, "", UserStatusBanned)
	if err != nil || total != 0 {
		t.Errorf("banned filter: total=%d err=%v, want 0", total, err)
	}
	if err := s.UpdateUserStatus(silent.ID, UserStatusBanned); err != nil {
		t.Fatalf("ban silent: %v", err)
	}
	items, total, err = s.ListCommentUsers(1, 20, "", UserStatusBanned)
	if err != nil || total != 1 || items[0].Username != "silent-reader" {
		t.Errorf("banned filter after ban: total=%d items=%+v err=%v", total, items, err)
	}

	// CountCommentUsers
	if n, err := s.CountCommentUsers(""); err != nil || n != 2 {
		t.Errorf("count all readers = %d (err %v), want 2", n, err)
	}
	if n, err := s.CountCommentUsers(UserStatusBanned); err != nil || n != 1 {
		t.Errorf("count banned readers = %d (err %v), want 1", n, err)
	}
}

// TestBulkCommentsByUser 用户级批量处置：approve/spam 只改该用户评论；
// delete 级联其根评论下的子评论并清理 boosts，他人评论不受影响。
func TestBulkCommentsByUser(t *testing.T) {
	s := newUserCommentStore(t)
	spammer := seedReader(t, s, "spammer")
	other := seedReader(t, s, "other")

	rootID := seedUserComment(t, s, "/post/a", spammer.ID, CommentApproved)
	seedUserComment(t, s, "/post/b", spammer.ID, CommentPending)
	otherComment := seedUserComment(t, s, "/post/a", other.ID, CommentApproved)

	// 他人回复挂在 spammer 的根评论下（级联删除目标）
	childID, err := s.CreateComment(&Comment{
		PageKey: "/post/a", Nick: "o", Email: "o@example.com",
		ContentMD: "reply", ContentHTML: "<p>reply</p>",
		Status: CommentApproved, UserID: other.ID,
		ParentID: rootID, RootID: rootID,
	})
	if err != nil {
		t.Fatalf("seed child comment: %v", err)
	}
	// spammer 评论上的 boost（delete 时应被清理）
	if _, _, err := s.AddBoost(rootID, "ip:viewer", "👍", "", BoostMeta{IP: "203.0.113.9"}); err != nil {
		t.Fatalf("seed boost: %v", err)
	}

	// spam：只影响 spammer 的 2 条
	n, err := s.BulkCommentsByUser(spammer.ID, "spam")
	if err != nil || n != 2 {
		t.Fatalf("bulk spam: n=%d err=%v, want 2", n, err)
	}
	if cm, _ := s.GetComment(otherComment); cm == nil || cm.Status != CommentApproved {
		t.Errorf("other's comment should stay approved, got %+v", cm)
	}
	if cm, _ := s.GetComment(rootID); cm == nil || cm.Status != CommentSpam {
		t.Errorf("spammer root should be spam, got %+v", cm)
	}

	// approve：恢复
	if n, err := s.BulkCommentsByUser(spammer.ID, "approve"); err != nil || n != 2 {
		t.Fatalf("bulk approve: n=%d err=%v, want 2", n, err)
	}

	// delete：spammer 的 2 条 + 级联子评论 1 条 = 3；boosts 清理；他人根评论保留
	n, err = s.BulkCommentsByUser(spammer.ID, "delete")
	if err != nil || n != 3 {
		t.Fatalf("bulk delete: n=%d err=%v, want 3 (2 own + 1 cascade child)", n, err)
	}
	if cm, _ := s.GetComment(rootID); cm != nil {
		t.Errorf("spammer root should be deleted")
	}
	if cm, _ := s.GetComment(childID); cm != nil {
		t.Errorf("cascade child under spammer root should be deleted")
	}
	if cm, _ := s.GetComment(otherComment); cm == nil {
		t.Errorf("other's root comment should survive")
	}
	if n, err := s.CountBoostsByComment(rootID); err != nil || n != 0 {
		t.Errorf("boosts of deleted comment should be cleaned, got %d (err %v)", n, err)
	}

	// 非法 action / 非法用户
	if _, err := s.BulkCommentsByUser(spammer.ID, "reject"); err == nil {
		t.Error("invalid action should error")
	}
	if _, err := s.BulkCommentsByUser(0, "spam"); err == nil {
		t.Error("invalid user id should error")
	}
}
