package store

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := s.EnsureRevisionsTable(); err != nil {
		t.Fatalf("ensure revisions table: %v", err)
	}
	if err := s.EnsureUsersTable(); err != nil {
		t.Fatalf("ensure users table: %v", err)
	}
	for _, username := range []string{"alice", "bob", "carol", "dave", "legacy"} {
		if err := s.CreateUser(&User{Username: username, PasswordHash: "test", AuthSecret: GenerateAuthSecret(), Role: RoleAuthor}); err != nil {
			t.Fatalf("create test user %s: %v", username, err)
		}
	}
	if err := s.EnsurePostAuthorsTable(); err != nil {
		t.Fatalf("ensure post authors table: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestRevisionCreateAndGet(t *testing.T) {
	s := newTestStore(t)
	id, err := s.CreateRevision(&Revision{
		PostSlug:  "hello",
		Title:     "标题",
		Excerpt:   "摘要",
		ContentMD: "# 正文",
		Tags:      "Go",
		Category:  "tech",
		Status:    StatusPublished,
	})
	if err != nil {
		t.Fatalf("create revision: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}
	got, err := s.GetRevision(id)
	if err != nil {
		t.Fatalf("get revision: %v", err)
	}
	if got == nil {
		t.Fatal("revision not found")
	}
	if got.ContentMD != "# 正文" || got.Title != "标题" || got.Category != "tech" {
		t.Fatalf("unexpected revision content: %+v", got)
	}
	if got.WordCount == 0 {
		t.Fatal("expected word count > 0")
	}
}

func TestRevisionListExcludesContent(t *testing.T) {
	s := newTestStore(t)
	s.CreateRevision(&Revision{PostSlug: "p", Title: "t1", ContentMD: "aaaa"})
	s.CreateRevision(&Revision{PostSlug: "p", Title: "t2", ContentMD: "bbbb"})
	s.CreateRevision(&Revision{PostSlug: "other", Title: "x", ContentMD: "cccc"})

	metas, err := s.ListRevisions("p")
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(metas) != 2 {
		t.Fatalf("expected 2 metas for slug p, got %d", len(metas))
	}
	if metas[0].Title != "t2" {
		t.Fatalf("expected newest first, got %s", metas[0].Title)
	}
}

func TestRevisionPruneBoundary(t *testing.T) {
	s := newTestStore(t)
	var firstID int64
	for i := 1; i <= 21; i++ {
		id, err := s.CreateRevision(&Revision{PostSlug: "p", Title: "t", ContentMD: "v" + string(rune('a'+i))})
		if err != nil {
			t.Fatalf("create revision %d: %v", i, err)
		}
		if i == 1 {
			firstID = id
		}
	}
	metas, err := s.ListRevisions("p")
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(metas) != maxRevisionsPerPost {
		t.Fatalf("expected %d revisions after prune, got %d", maxRevisionsPerPost, len(metas))
	}
	got, err := s.GetRevision(firstID)
	if err != nil {
		t.Fatalf("get first revision: %v", err)
	}
	if got != nil {
		t.Fatal("expected oldest revision pruned, but it still exists")
	}
}

func TestRevisionDeleteByPost(t *testing.T) {
	s := newTestStore(t)
	s.CreateRevision(&Revision{PostSlug: "p", Title: "t", ContentMD: "x"})
	s.CreateRevision(&Revision{PostSlug: "p", Title: "t2", ContentMD: "y"})
	s.CreateRevision(&Revision{PostSlug: "keep", Title: "k", ContentMD: "z"})

	if err := s.DeleteRevisionsByPost("p"); err != nil {
		t.Fatalf("delete revisions by post: %v", err)
	}
	metas, _ := s.ListRevisions("p")
	if len(metas) != 0 {
		t.Fatalf("expected 0 revisions for p, got %d", len(metas))
	}
	keep, _ := s.ListRevisions("keep")
	if len(keep) != 1 {
		t.Fatalf("expected 1 revision for keep, got %d", len(keep))
	}
}

func TestRevisionUpdateSlug(t *testing.T) {
	s := newTestStore(t)
	s.CreateRevision(&Revision{PostSlug: "old", Title: "t", ContentMD: "x"})
	if err := s.UpdateRevisionSlug("old", "new"); err != nil {
		t.Fatalf("update revision slug: %v", err)
	}
	oldMetas, _ := s.ListRevisions("old")
	if len(oldMetas) != 0 {
		t.Fatalf("expected 0 revisions under old slug, got %d", len(oldMetas))
	}
	newMetas, _ := s.ListRevisions("new")
	if len(newMetas) != 1 {
		t.Fatalf("expected 1 revision under new slug, got %d", len(newMetas))
	}
}

func TestCountWords(t *testing.T) {
	if n := CountWords("hello world"); n != 10 {
		t.Fatalf("CountWords('hello world') = %d, want 10", n)
	}
	if n := CountWords("你好 世界"); n != 4 {
		t.Fatalf("CountWords('你好 世界') = %d, want 4", n)
	}
	if n := CountWords(""); n != 0 {
		t.Fatalf("CountWords('') = %d, want 0", n)
	}
}
