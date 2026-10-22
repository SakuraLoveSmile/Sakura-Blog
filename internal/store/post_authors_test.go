package store

import (
	"encoding/json"
	"errors"
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

func TestStrictPublicPostAuthorsRemainIndependentFromUserOwnership(t *testing.T) {
	s := newTestStore(t)
	if err := s.EnsureSettingsTable(); err != nil {
		t.Fatal(err)
	}
	if err := s.PutSetting("stellar_v2", `{"version":2,"theme":{}}`); err != nil {
		t.Fatal(err)
	}
	if err := s.PutSetting("blog_extensions", `{"data":{"authors":{"alice":{"name":"Alice"},"writer":{"name":"Writer"},"bob":{"name":"Bob"}}},"private":"catalog-secret"}`); err != nil {
		t.Fatal(err)
	}
	alice, _ := s.GetUserByUsername("alice")
	bob, _ := s.GetUserByUsername("bob")
	raw := ` {"article":{"author":"alice"},"comments":{"options":{"large":9007199254740993}}} `
	blog := ` {"content":{"authors":["alice","writer"]},"opaque":{"large":9007199254740993,"flag":false,"nil":null}} `
	p := &Post{Slug: "public-authors", Title: "Title", Status: StatusPublished, AuthorIDs: []int64{alice.ID}, StellarVersion: 2, StellarConfig: raw, BlogExtensions: blog}
	if _, err := s.Create(p); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"reopen", "draft", "update"} {
		got, err := s.GetBySlug(p.Slug)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(got.Authors, ",") != "alice,writer" || got.Author != "alice" || len(got.AuthorIDs) != 1 || got.AuthorIDs[0] != alice.ID {
			t.Fatalf("%s wrong public/owner projection: %+v", operation, got)
		}
		switch operation {
		case "draft":
			draft, _ := json.Marshal(got)
			if err := s.SaveDraft(p.Slug, string(draft)); err != nil {
				t.Fatal(err)
			}
		case "update":
			got.Title = "Only title changed"
			if err := s.UpdateSlugWithRevision(p.Slug, p.Slug, got); err != nil {
				t.Fatal(err)
			}
		}
	}
	if user, err := s.GetUserByUsername("writer"); err != nil || user != nil {
		t.Fatal("public author invented a user")
	}
	got, _ := s.GetBySlug(p.Slug)
	if got.StellarConfig != raw || got.BlogExtensions != blog {
		t.Fatal("public author raw changed")
	}
	if ok, err := s.HasPostAuthor(p.Slug, bob.ID); err != nil || ok {
		t.Fatal("public author changed editor ownership")
	}
	// 公开署名刚好等于用户姓名也不在启动回填时赋予编辑权限。
	got.StellarConfig = `{"article":{"author":"bob"}}`
	got.BlogExtensions = `{"content":{"authors":["bob"]}}`
	if err := s.Update(p.Slug, got); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsurePostAuthorsTable(); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.HasPostAuthor(p.Slug, bob.ID); err != nil || ok {
		t.Fatal("restart backfill granted public author editor ownership")
	}
	before, _ := s.GetBySlug(p.Slug)
	invalid := *before
	invalid.BlogExtensions = `{"content":{"authors":["unknown-private"]}}`
	if err := s.Update(p.Slug, &invalid); !errors.Is(err, ErrStellarPublicAuthorInvalid) {
		t.Fatalf("invalid update: %v", err)
	}
	draft, _ := json.Marshal(invalid)
	if err := s.SaveDraft(p.Slug, string(draft)); !errors.Is(err, ErrStellarPublicAuthorInvalid) {
		t.Fatalf("invalid draft: %v", err)
	}
	after, _ := s.GetBySlug(p.Slug)
	if after.BlogExtensions != before.BlogExtensions || after.DraftJSON != before.DraftJSON {
		t.Fatal("invalid public author mutated stored content")
	}
}
