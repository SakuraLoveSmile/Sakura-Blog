package routes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"blog/internal/post"
	"blog/internal/store"
)

func TestEnsureSystemNotesPageMigratesOnlyLegacyDefault(t *testing.T) {
	t.Run("legacy default gets moments shortcode", func(t *testing.T) {
		st, handler, importPath := newNotesMigrationFixture(t, "## 近期动态\n\n旧版正文\n\n{% moments %}\n")
		defer st.Close()

		if _, err := st.CreatePage(&store.Page{
			Slug: NotesSlug, Title: "笔记", ContentMD: "## 近期动态\n\n旧版正文\n", ContentHTML: "<p>旧版正文</p>",
			Status: store.StatusPublished, IsSystem: true,
		}); err != nil {
			t.Fatalf("create legacy notes page: %v", err)
		}
		if err := handler.EnsureSystemNotesPage(importPath); err != nil {
			t.Fatalf("migrate legacy notes page: %v", err)
		}
		page, err := st.GetPageBySlug(NotesSlug)
		if err != nil {
			t.Fatalf("get migrated notes page: %v", err)
		}
		if !strings.Contains(page.ContentMD, "{% moments %}") {
			t.Fatalf("legacy notes page should gain moments shortcode: %q", page.ContentMD)
		}
	})

	t.Run("custom content stays unchanged", func(t *testing.T) {
		st, handler, importPath := newNotesMigrationFixture(t, "## 近期动态\n\n旧版正文\n\n{% moments %}\n")
		defer st.Close()

		custom := "## 近期动态\n\n我修改过的正文\n"
		if _, err := st.CreatePage(&store.Page{
			Slug: NotesSlug, Title: "笔记", ContentMD: custom, ContentHTML: "<p>我修改过的正文</p>",
			Status: store.StatusPublished, IsSystem: true,
		}); err != nil {
			t.Fatalf("create custom notes page: %v", err)
		}
		if err := handler.EnsureSystemNotesPage(importPath); err != nil {
			t.Fatalf("check custom notes page: %v", err)
		}
		page, err := st.GetPageBySlug(NotesSlug)
		if err != nil {
			t.Fatalf("get custom notes page: %v", err)
		}
		if page.ContentMD != custom {
			t.Fatalf("custom notes page should remain unchanged: %q", page.ContentMD)
		}
	})
}

func newNotesMigrationFixture(t *testing.T, current string) (*store.Store, *PageHandler, string) {
	t.Helper()
	root := t.TempDir()
	st, err := store.New(filepath.Join(root, "blog.db"))
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	if err := st.EnsurePagesTable(); err != nil {
		st.Close()
		t.Fatalf("ensure pages table: %v", err)
	}
	importPath := filepath.Join(root, "notes.md")
	if err := os.WriteFile(importPath, []byte(current), 0o600); err != nil {
		st.Close()
		t.Fatalf("write notes fixture: %v", err)
	}
	return st, &PageHandler{store: st, posts: post.NewService(filepath.Join(root, "posts"))}, importPath
}
