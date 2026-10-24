package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestValidateRestoreFromRejectsEmptyDatabase(t *testing.T) {
	s := newForeignKeyStore(t)
	empty := filepath.Join(t.TempDir(), "empty.db")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.ValidateRestoreFrom(empty); err == nil || !strings.Contains(err.Error(), "no application tables") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateRestoreFromRejectsNewColumnWithoutLiveMutation(t *testing.T) {
	s := newForeignKeyStore(t)
	if err := s.Upsert(&Post{Slug: "stable", Title: "before", Status: StatusPublished}); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "newer.db")
	if err := s.BackupTo(src); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+src)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE posts ADD COLUMN future TEXT`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	if err := s.ValidateRestoreFrom(src); err == nil || !strings.Contains(err.Error(), "schema is newer") {
		t.Fatalf("error = %v", err)
	}
	p, err := s.GetBySlug("stable")
	if err != nil || p == nil || p.Title != "before" {
		t.Fatalf("live DB changed: %+v %v", p, err)
	}
}

func TestRestoreRejectsUnknownTablesIncludingIndexLikeNames(t *testing.T) {
	for _, ddl := range []string{
		`CREATE TABLE future_business(id INTEGER PRIMARY KEY)`,
		`CREATE TABLE posts_fts_future_business(id INTEGER PRIMARY KEY)`,
		`CREATE TABLE rag_chunks_fts_future_business(id INTEGER PRIMARY KEY)`,
		`CREATE VIRTUAL TABLE future_fts USING fts5(content)`,
	} {
		t.Run(ddl, func(t *testing.T) {
			live := newForeignKeyStore(t)
			if err := live.Upsert(&Post{Slug: "stable", Title: "before", Status: StatusDraft}); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(t.TempDir(), "newer.db")
			if err := live.BackupTo(source); err != nil {
				t.Fatal(err)
			}
			db, err := sql.Open("sqlite", source)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(ddl); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			if err := live.RestoreFrom(source); err == nil || !strings.Contains(err.Error(), "schema is newer") {
				t.Fatalf("unknown table accepted: %v", err)
			}
			p, err := live.GetBySlug("stable")
			if err != nil || p == nil || p.Title != "before" || p.Status != StatusDraft {
				t.Fatalf("live content changed: post=%+v err=%v", p, err)
			}
		})
	}
}

func TestRestoreFromValidatesForeignKeysBeforeChangingLiveDB(t *testing.T) {
	s := newForeignKeyStore(t)
	if err := s.EnsureUsersTable(); err != nil {
		t.Fatal(err)
	}
	if err := s.Upsert(&Post{Slug: "stable", Title: "before", Status: StatusPublished}); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "bad-fk.db")
	if err := s.BackupTo(src); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+src+"?_pragma=foreign_keys(OFF)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO user_roles(user_id, role, is_primary) VALUES(999999, 'author', 1)`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	if err := s.RestoreFrom(src); err == nil {
		t.Fatal("invalid foreign key source should be rejected")
	}
	p, err := s.GetBySlug("stable")
	if err != nil || p == nil || p.Title != "before" {
		t.Fatalf("live DB changed: %+v %v", p, err)
	}
}

func TestOpenForRecoveryDoesNotCreateSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.db")
	s, err := OpenForRecovery(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("recovery open created %d tables", count)
	}
}

func TestValidateRestoreRejectsMissingLegacyContentTable(t *testing.T) {
	live := newForeignKeyStore(t)
	if err := live.Upsert(&Post{Slug: "keep", Title: "original", Status: StatusPublished}); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "incomplete.db")
	if err := live.BackupTo(source); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP TABLE posts`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := live.ValidateRestoreFrom(source); err == nil || !strings.Contains(err.Error(), "legacy core table") {
		t.Fatalf("incomplete content backup accepted: %v", err)
	}
	post, err := live.GetBySlug("keep")
	if err != nil || post == nil || post.Title != "original" {
		t.Fatalf("live content changed: %v", err)
	}
}
