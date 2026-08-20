package store

import (
	"errors"
	"strings"
	"testing"
)

func TestCategoryTreeAndAssociations(t *testing.T) {
	s := newTestStore(t)
	a := pubPost("a", "a")
	a.Categories = []string{"产研/技术/前端", "产品"}
	if _, err := s.Create(a); err != nil {
		t.Fatal(err)
	}
	b := pubPost("b", "b")
	b.Categories = []string{"产研/设计开发"}
	if _, err := s.Create(b); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetBySlug("a")
	if err != nil || strings.Join(got.Categories, "|") != "产研/技术/前端|产品" {
		t.Fatalf("categories = %#v, err=%v", got.Categories, err)
	}
	tree, err := s.CategoryTree()
	if err != nil || len(tree) != 2 || tree[0].Path != "产品" || tree[1].Path != "产研" {
		t.Fatalf("tree = %#v, err=%v", tree, err)
	}
	var leaf *CategoryNode
	for _, child := range tree[1].Children {
		if child.Path == "产研/技术" {
			leaf = child
		}
	}
	if tree[1].Count != 2 || len(tree[1].Children) != 2 || leaf == nil || len(leaf.Children) != 1 || leaf.Children[0].Path != "产研/技术/前端" {
		t.Fatalf("tree shape/count = %#v", tree)
	}
	posts, err := s.ListByCategoryPath("产研")
	if err != nil || len(posts) != 2 {
		t.Fatalf("subtree posts = %d, err=%v", len(posts), err)
	}
	if _, err := s.CreateCategory("错误/路径", 0); !errors.Is(err, ErrCategoryInvalid) {
		t.Fatalf("invalid category error = %v", err)
	}
	if err := s.DeleteCategoryByID(tree[1].ID); !errors.Is(err, ErrCategoryConflict) {
		t.Fatalf("non-empty parent delete = %v", err)
	}
}

func TestCategoryMigrationIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.db.Exec(`INSERT INTO posts (slug, title, category, status, deleted_at) VALUES ('legacy', '旧', '旧分类', 'published', '')`); err != nil {
		t.Fatal(err)
	}
	if err := s.migrateColumns(); err != nil {
		t.Fatal(err)
	}
	if err := s.migrateColumns(); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetBySlug("legacy")
	if err != nil || got == nil || strings.Join(got.Categories, ",") != "旧分类" {
		t.Fatalf("migrated categories = %#v, err=%v", got.Categories, err)
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM categories WHERE name = '旧分类'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("category rows = %d, err=%v", n, err)
	}
}

func TestLegacyCategoryCreateAssociation(t *testing.T) {
	s := newTestStore(t)
	p := pubPost("legacy-create", "legacy")
	p.Category = "后端"
	if _, err := s.Create(p); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM post_categories pc JOIN categories c ON c.id = pc.category_id WHERE c.name = '后端'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("association count = %d, err=%v", n, err)
	}
}
