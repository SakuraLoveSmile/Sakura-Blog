package store

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestNewEnablesSQLiteForeignKeys(t *testing.T) {
	s := newForeignKeyStore(t)

	var enabled int
	if err := s.DB().QueryRow(`PRAGMA foreign_keys`).Scan(&enabled); err != nil {
		t.Fatalf("read foreign_keys pragma: %v", err)
	}
	if enabled != 1 {
		t.Fatalf("foreign_keys = %d, want 1", enabled)
	}
}

func TestSQLiteForeignKeysRejectMissingParent(t *testing.T) {
	s := newForeignKeyStore(t)
	if err := s.EnsureUsersTable(); err != nil {
		t.Fatalf("ensure users: %v", err)
	}

	_, err := s.DB().Exec(`INSERT INTO user_roles(user_id, role, is_primary) VALUES(?, ?, ?)`, 999999, RoleAuthor, 1)
	if err == nil {
		t.Fatal("inserting a user role for a missing user should fail")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "foreign key") {
		t.Fatalf("missing-parent error = %v, want foreign key error", err)
	}
}

func TestSQLiteForeignKeysCascadePostAssociations(t *testing.T) {
	s := newForeignKeyStore(t)
	if err := s.EnsureUsersTable(); err != nil {
		t.Fatalf("ensure users: %v", err)
	}
	p := pubPost("foreign-key-cascade", "外键级联")
	postID, err := s.Create(p)
	if err != nil {
		t.Fatalf("create post: %v", err)
	}

	var categoryID int64
	if err := s.DB().QueryRow(`INSERT INTO categories(name, parent_id) VALUES(?, 0) RETURNING id`, "外键测试").Scan(&categoryID); err != nil {
		t.Fatalf("create category: %v", err)
	}
	if _, err := s.DB().Exec(`INSERT INTO post_categories(post_id, category_id, position) VALUES(?, ?, 0)`, postID, categoryID); err != nil {
		t.Fatalf("create post category association: %v", err)
	}

	if _, err := s.DB().Exec(`DELETE FROM posts WHERE id = ?`, postID); err != nil {
		t.Fatalf("delete post: %v", err)
	}
	var associations int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM post_categories WHERE post_id = ?`, postID).Scan(&associations); err != nil {
		t.Fatalf("count post categories: %v", err)
	}
	if associations != 0 {
		t.Fatalf("post category associations = %d, want 0 after cascade", associations)
	}
}

func TestNewMigratesLegacyDatabaseWithForeignKeysEnabled(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	if _, err := legacy.Exec(`CREATE TABLE posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slug TEXT NOT NULL UNIQUE,
		title TEXT NOT NULL DEFAULT '',
		excerpt TEXT NOT NULL DEFAULT '',
		category TEXT NOT NULL DEFAULT '',
		tags TEXT NOT NULL DEFAULT '',
		content_md TEXT NOT NULL DEFAULT '',
		content_html TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'draft',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		legacy.Close()
		t.Fatalf("create legacy posts: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("open and migrate legacy db: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	var enabled int
	if err := s.DB().QueryRow(`PRAGMA foreign_keys`).Scan(&enabled); err != nil {
		t.Fatalf("read foreign_keys pragma: %v", err)
	}
	if enabled != 1 {
		t.Fatalf("foreign_keys after legacy migration = %d, want 1", enabled)
	}
	if ok, err := s.tableHasColumn("posts", "draft_json"); err != nil || !ok {
		t.Fatalf("legacy posts migration draft_json: present=%v err=%v", ok, err)
	}
}

func TestNewSchemaCanCreateAllRegisteredTables(t *testing.T) {
	s := newForeignKeyStore(t)
	if err := s.EnsureSchema(); err != nil {
		t.Fatalf("ensure complete schema: %v", err)
	}
	if err := s.EnsureUsersTable(); err != nil {
		t.Fatalf("ensure users and authorization schema: %v", err)
	}
	if err := s.EnsureSchema(); err != nil {
		t.Fatalf("repeat complete schema: %v", err)
	}
}

func newForeignKeyStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
