package store

import (
	"path/filepath"
	"testing"
)

// newWikidocStore 建库并确保 wikidocs 表。
func newWikidocStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := s.EnsureWikidocsTable(); err != nil {
		t.Fatalf("ensure wikidocs table: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// mkWikidoc 便捷插入一条文档（ID 回填），返回指针供断言用。
func mkWikidoc(t *testing.T, s *Store, projectID, slug string, order int, parent int64) *WikiDoc {
	t.Helper()
	d := &WikiDoc{
		ProjectID:   projectID,
		Slug:        slug,
		Title:       slug,
		Order:       order,
		Parent:      parent,
		ContentMD:   "# " + slug,
		ContentHTML: "<h1>" + slug + "</h1>",
		Status:      StatusPublished,
	}
	if err := s.CreateWikidoc(d); err != nil {
		t.Fatalf("create wikidoc %s/%s: %v", projectID, slug, err)
	}
	return d
}

// findNode 在树中按 slug 找节点（不进入深层嵌套查找，用于 3 层树断言）。
func findNode(nodes []TreeNode, slug string) *TreeNode {
	for i := range nodes {
		if nodes[i].Doc.Slug == slug {
			return &nodes[i]
		}
	}
	return nil
}

// TestEnsureWikidocsTableIdempotent 重复建表应幂等。
func TestEnsureWikidocsTableIdempotent(t *testing.T) {
	s := newWikidocStore(t)
	for i := 0; i < 3; i++ {
		if err := s.EnsureWikidocsTable(); err != nil {
			t.Fatalf("re-ensure #%d: %v", i, err)
		}
	}
}

// TestCreateGetWikidocBySlug 创建后按 project_id + slug 查回；未找到返回 nil, nil。
func TestCreateGetWikidocBySlug(t *testing.T) {
	s := newWikidocStore(t)
	d := mkWikidoc(t, s, "go", "install", 0, 0)

	got, err := s.GetWikidocBySlug("go", "install")
	if err != nil || got == nil {
		t.Fatalf("get by slug: %v (nil=%v)", err, got == nil)
	}
	if got.ID != d.ID || got.ProjectID != "go" || got.Slug != "install" {
		t.Fatalf("field mismatch: %+v", got)
	}
	if got.Title != "install" || got.ContentMD != "# install" || got.ContentHTML != "<h1>install</h1>" {
		t.Fatalf("content mismatch: %+v", got)
	}
	if got.Status != StatusPublished {
		t.Fatalf("status want published, got %q", got.Status)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("timestamps should be set: %+v", got)
	}

	// 未找到返回 nil, nil
	missing, err := s.GetWikidocBySlug("go", "nope")
	if err != nil || missing != nil {
		t.Fatalf("missing slug should return nil,nil: doc=%v err=%v", missing, err)
	}
	// 同 slug 跨 project 隔离
	other, err := s.GetWikidocBySlug("rust", "install")
	if err != nil || other != nil {
		t.Fatalf("other project should not match: doc=%v err=%v", other, err)
	}
}

// TestGetWikidocByID 按 id 查回；未找到返回 nil, nil。
func TestGetWikidocByID(t *testing.T) {
	s := newWikidocStore(t)
	d := mkWikidoc(t, s, "go", "a", 0, 0)

	got, err := s.GetWikidocByID(d.ID)
	if err != nil || got == nil {
		t.Fatalf("get by id: %v (nil=%v)", err, got == nil)
	}
	if got.Slug != "a" {
		t.Fatalf("slug want a, got %q", got.Slug)
	}

	missing, err := s.GetWikidocByID(99999)
	if err != nil || missing != nil {
		t.Fatalf("missing id should return nil,nil: doc=%v err=%v", missing, err)
	}
}

// TestListWikidocsFilterByProject 按 project_id 过滤 + 按 doc_order, id 升序。
func TestListWikidocsFilterByProject(t *testing.T) {
	s := newWikidocStore(t)
	mkWikidoc(t, s, "go", "c", 3, 0)
	mkWikidoc(t, s, "go", "a", 1, 0)
	mkWikidoc(t, s, "go", "b", 2, 0)
	mkWikidoc(t, s, "rust", "x", 0, 0)

	docs, err := s.ListWikidocs("go")
	if err != nil {
		t.Fatalf("list go: %v", err)
	}
	if len(docs) != 3 {
		t.Fatalf("want 3 docs, got %d", len(docs))
	}
	for i, want := range []string{"a", "b", "c"} {
		if docs[i].Slug != want {
			t.Errorf("order[%d] want %s, got %s", i, want, docs[i].Slug)
		}
	}

	other, err := s.ListWikidocs("rust")
	if err != nil {
		t.Fatalf("list rust: %v", err)
	}
	if len(other) != 1 || other[0].Slug != "x" {
		t.Fatalf("rust docs mismatch: %+v", other)
	}
}

// TestUpdateWikidoc 更新字段并验证 updated_at 自动刷新；文档不存在报错。
func TestUpdateWikidoc(t *testing.T) {
	s := newWikidocStore(t)
	d := mkWikidoc(t, s, "go", "a", 0, 0)
	old := d.UpdatedAt

	d.Title = "新标题"
	d.Order = 5
	d.Parent = 0
	d.ContentMD = "# 新内容"
	d.ContentHTML = "<h1>新内容</h1>"
	d.Status = StatusDraft
	if err := s.UpdateWikidoc(d); err != nil {
		t.Fatalf("update: %v", err)
	}
	// 内存对象 updated_at 应被刷新（晚于创建时）
	if !d.UpdatedAt.After(old) {
		t.Fatalf("updated_at should be refreshed: old=%v new=%v", old, d.UpdatedAt)
	}

	got, err := s.GetWikidocByID(d.ID)
	if err != nil || got == nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Title != "新标题" || got.ContentMD != "# 新内容" || got.ContentHTML != "<h1>新内容</h1>" {
		t.Fatalf("update not persisted: %+v", got)
	}
	if got.Order != 5 || got.Status != StatusDraft {
		t.Fatalf("order/status not persisted: %+v", got)
	}
	// 库中 updated_at 不应早于创建时（驱动按秒截断，允许相等）
	if got.UpdatedAt.Before(old) {
		t.Fatalf("persisted updated_at should not be older than created: created=%v updated=%v", old, got.UpdatedAt)
	}

	// 更新不存在的文档应报错
	if err := s.UpdateWikidoc(&WikiDoc{ID: 99999}); err == nil {
		t.Error("update nonexistent doc should error")
	}
}

// TestDeleteWikidoc 硬删除后查不到；重复删除不报错。
func TestDeleteWikidoc(t *testing.T) {
	s := newWikidocStore(t)
	d := mkWikidoc(t, s, "go", "a", 0, 0)

	if err := s.DeleteWikidoc(d.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	got, err := s.GetWikidocByID(d.ID)
	if err != nil || got != nil {
		t.Fatalf("doc should be gone: doc=%v err=%v", got, err)
	}
	// 删除不存在的 id 静默
	if err := s.DeleteWikidoc(d.ID); err != nil {
		t.Fatalf("delete nonexistent should be silent: %v", err)
	}
}

// TestWikidocUniqueConstraint 同 project 内 slug 唯一；跨 project 允许同名。
func TestWikidocUniqueConstraint(t *testing.T) {
	s := newWikidocStore(t)
	mkWikidoc(t, s, "go", "install", 0, 0)

	if err := s.CreateWikidoc(&WikiDoc{ProjectID: "go", Slug: "install", Title: "dup"}); err == nil {
		t.Fatal("same project + slug should violate UNIQUE constraint")
	}
	// 同 project 不同 slug 正常
	if err := s.CreateWikidoc(&WikiDoc{ProjectID: "go", Slug: "guide", Title: "g"}); err != nil {
		t.Fatalf("different slug in same project: %v", err)
	}
	// 不同 project 同 slug 正常
	if err := s.CreateWikidoc(&WikiDoc{ProjectID: "rust", Slug: "install", Title: "r"}); err != nil {
		t.Fatalf("same slug in other project: %v", err)
	}
}

// TestBuildDocTree 3 层树组装：层级正确、第 4 层截断、孤儿丢弃、跨 project 隔离。
func TestBuildDocTree(t *testing.T) {
	s := newWikidocStore(t)
	// wiki project：3 层 + 1 个第 4 层 + 1 个孤儿（parent 指向不存在的 99999）
	a := mkWikidoc(t, s, "wiki", "a", 1, 0)
	mkWikidoc(t, s, "wiki", "b", 2, 0)
	a1 := mkWikidoc(t, s, "wiki", "a1", 1, a.ID)
	mkWikidoc(t, s, "wiki", "a2", 2, a.ID)
	a1x := mkWikidoc(t, s, "wiki", "a1x", 1, a1.ID)
	mkWikidoc(t, s, "wiki", "a1xy", 1, a1x.ID)  // 第 4 层，应被截断
	mkWikidoc(t, s, "wiki", "orphan", 1, 99999) // 孤儿，应被丢弃
	// 其它 project 不应混入
	mkWikidoc(t, s, "other", "x", 0, 0)

	tree, err := s.BuildDocTree("wiki")
	if err != nil {
		t.Fatalf("build tree: %v", err)
	}
	if len(tree) != 2 {
		t.Fatalf("want 2 roots, got %d", len(tree))
	}
	if tree[0].Doc.Slug != "a" || tree[1].Doc.Slug != "b" {
		t.Fatalf("root order mismatch: %s / %s", tree[0].Doc.Slug, tree[1].Doc.Slug)
	}

	// 第 2 层：a 下挂 a1、a2
	na := findNode(tree, "a")
	if na == nil || len(na.Children) != 2 {
		t.Fatalf("a children want 2, got %v", na)
	}
	if na.Children[0].Doc.Slug != "a1" || na.Children[1].Doc.Slug != "a2" {
		t.Fatalf("a children order mismatch: %s / %s", na.Children[0].Doc.Slug, na.Children[1].Doc.Slug)
	}

	// 第 3 层：a1 下挂 a1x
	na1 := findNode(na.Children, "a1")
	if na1 == nil || len(na1.Children) != 1 || na1.Children[0].Doc.Slug != "a1x" {
		t.Fatalf("a1 children want [a1x], got %v", na1)
	}

	// 第 4 层截断：a1x 无 children
	na1x := findNode(na1.Children, "a1x")
	if na1x == nil {
		t.Fatal("a1x should exist in tree")
	}
	if len(na1x.Children) != 0 {
		t.Fatalf("a1x (depth 4 root) should have no children, got %d", len(na1x.Children))
	}

	// 孤儿不出现；b 无子节点
	nb := findNode(tree, "b")
	if nb == nil || len(nb.Children) != 0 {
		t.Fatalf("b should be leaf, got %v", nb)
	}
	if findNode(tree, "orphan") != nil || findNode(tree, "a1xy") != nil {
		t.Fatal("orphan / depth-4 doc should not appear in tree")
	}

	// 其它 project 树不含 wiki 文档
	other, err := s.BuildDocTree("other")
	if err != nil {
		t.Fatalf("build other tree: %v", err)
	}
	if len(other) != 1 || other[0].Doc.Slug != "x" {
		t.Fatalf("other tree mismatch: %+v", other)
	}
}

// TestWikidocDeletePromotesChildren 删除父文档时，其直接子文档 parent 批量置 0 升为顶级。
func TestWikidocDeletePromotesChildren(t *testing.T) {
	s := newWikidocStore(t)
	a := mkWikidoc(t, s, "wiki", "a", 1, 0)
	a1 := mkWikidoc(t, s, "wiki", "a1", 1, a.ID)
	a2 := mkWikidoc(t, s, "wiki", "a2", 2, a.ID)
	a1x := mkWikidoc(t, s, "wiki", "a1x", 1, a1.ID) // 孙子文档，parent 指向 a1 不受影响
	b := mkWikidoc(t, s, "wiki", "b", 2, 0)         // 无关顶级文档

	if err := s.DeleteWikidoc(a.ID); err != nil {
		t.Fatalf("delete parent: %v", err)
	}

	// 被删文档本身不存在
	gone, err := s.GetWikidocByID(a.ID)
	if err != nil || gone != nil {
		t.Fatalf("deleted doc should be gone: doc=%v err=%v", gone, err)
	}
	// 直接子文档升为顶级
	c1, err := s.GetWikidocByID(a1.ID)
	if err != nil || c1 == nil {
		t.Fatalf("child a1 should remain: %v", err)
	}
	if c1.Parent != 0 {
		t.Fatalf("child a1 parent want 0 (promoted), got %d", c1.Parent)
	}
	c2, err := s.GetWikidocByID(a2.ID)
	if err != nil || c2 == nil {
		t.Fatalf("child a2 should remain: %v", err)
	}
	if c2.Parent != 0 {
		t.Fatalf("child a2 parent want 0 (promoted), got %d", c2.Parent)
	}
	// 孙子文档的 parent 链不受影响（仍指向 a1）
	g, err := s.GetWikidocByID(a1x.ID)
	if err != nil || g == nil {
		t.Fatalf("grandchild a1x should remain: %v", err)
	}
	if g.Parent != a1.ID {
		t.Fatalf("grandchild a1x parent want %d, got %d", a1.ID, g.Parent)
	}
	// 无关文档不受影响
	bgot, err := s.GetWikidocByID(b.ID)
	if err != nil || bgot == nil || bgot.Parent != 0 {
		t.Fatalf("unrelated doc b should be intact: doc=%v err=%v", bgot, err)
	}
	// 删除后树中 a1/a2 为顶级
	tree, err := s.BuildDocTree("wiki")
	if err != nil {
		t.Fatalf("build tree: %v", err)
	}
	if len(tree) != 3 {
		t.Fatalf("want 3 roots after delete, got %d", len(tree))
	}
	if findNode(tree, "a1") == nil || findNode(tree, "a2") == nil {
		t.Fatal("promoted children should appear as roots")
	}
}
