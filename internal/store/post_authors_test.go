package store

import (
	"strings"
	"testing"
)

func TestPostAuthorsMigrationUsesUserIDsAndQuarantinesUnmappedRows(t *testing.T) {
	s := newTestStore(t)
	p := pubPost("author-migration", "作者迁移")
	p.Author = "alice"
	postID, err := s.Create(p)
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	alice, err := s.GetUserByUsername("alice")
	if err != nil || alice == nil {
		t.Fatalf("get alice: %v", err)
	}

	if _, err := s.DB().Exec(`DROP INDEX IF EXISTS idx_post_authors_author_order; DROP TABLE post_authors; CREATE TABLE post_authors (
		post_id INTEGER NOT NULL,
		author_id TEXT NOT NULL,
		sort_order INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY(post_id, author_id),
		FOREIGN KEY(post_id) REFERENCES posts(id) ON DELETE CASCADE
	); INSERT INTO post_authors(post_id, author_id, sort_order) VALUES (?, 'alice', 0), (?, 'ghost', 1)`, postID, postID); err != nil {
		t.Fatalf("prepare legacy post_authors: %v", err)
	}

	if err := s.EnsurePostAuthorsTable(); err != nil {
		t.Fatalf("migrate post_authors: %v", err)
	}
	if err := s.EnsurePostAuthorsTable(); err != nil {
		t.Fatalf("repeat post_authors migration: %v", err)
	}

	var typeName string
	if err := s.DB().QueryRow(`SELECT type FROM pragma_table_info('post_authors') WHERE name = 'author_id'`).Scan(&typeName); err != nil {
		t.Fatalf("read author_id type: %v", err)
	}
	if !strings.Contains(strings.ToUpper(typeName), "INT") {
		t.Fatalf("author_id type = %q, want INTEGER", typeName)
	}
	var mappedID int64
	if err := s.DB().QueryRow(`SELECT author_id FROM post_authors WHERE post_id = ?`, postID).Scan(&mappedID); err != nil {
		t.Fatalf("read mapped author: %v", err)
	}
	if mappedID != alice.ID {
		t.Fatalf("mapped author id = %d, want %d", mappedID, alice.ID)
	}
	var legacyRef string
	if err := s.DB().QueryRow(`SELECT author_ref FROM post_authors_legacy WHERE post_id = ?`, postID).Scan(&legacyRef); err != nil {
		t.Fatalf("read quarantined author: %v", err)
	}
	if legacyRef != "ghost" {
		t.Fatalf("quarantined author = %q, want ghost", legacyRef)
	}
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM post_authors WHERE post_id = ?`, postID).Scan(&count); err != nil {
		t.Fatalf("count migrated authors: %v", err)
	}
	if count != 1 {
		t.Fatalf("migrated author count = %d, want 1", count)
	}
	rows, err := s.DB().Query(`SELECT name FROM pragma_index_info('idx_post_authors_author_order') ORDER BY seqno`)
	if err != nil {
		t.Fatalf("read ownership index: %v", err)
	}
	defer rows.Close()
	var indexColumns []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan ownership index: %v", err)
		}
		indexColumns = append(indexColumns, name)
	}
	if got := strings.Join(indexColumns, ","); got != "author_id,sort_order,post_id" {
		t.Fatalf("ownership index columns = %q, want author_id,sort_order,post_id", got)
	}
	var referencedTable, fromColumn, toColumn string
	if err := s.DB().QueryRow(`SELECT "table", "from", "to" FROM pragma_foreign_key_list('post_authors') WHERE "from" = 'author_id'`).Scan(&referencedTable, &fromColumn, &toColumn); err != nil {
		t.Fatalf("read author foreign key: %v", err)
	}
	if referencedTable != "users" || fromColumn != "author_id" || toColumn != "id" {
		t.Fatalf("author foreign key = %s(%s -> %s), want users(author_id -> id)", referencedTable, fromColumn, toColumn)
	}
}
