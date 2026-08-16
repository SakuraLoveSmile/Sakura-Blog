package store

import (
	"path/filepath"
	"reflect"
	"testing"
)

// newNotebookStore 建库并确保 notebooks + notes 两表。
func newNotebookStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := s.EnsureNotebooksTable(); err != nil {
		t.Fatalf("ensure notebooks table: %v", err)
	}
	if err := s.EnsureNotesTable(); err != nil {
		t.Fatalf("ensure notes table: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// mkNotebook 便捷插入一个笔记本（ID 回填），返回指针供断言用。
func mkNotebook(t *testing.T, s *Store, name string, order int) *Notebook {
	t.Helper()
	nb := &Notebook{
		Name:        name,
		Description: "desc-" + name,
		Cover:       "cover-" + name,
		Order:       order,
	}
	if err := s.CreateNotebook(nb); err != nil {
		t.Fatalf("create notebook %s: %v", name, err)
	}
	return nb
}

// mkNote 便捷插入一条笔记（ID 回填），返回指针供断言用。
func mkNote(t *testing.T, s *Store, notebookID int64, title, tags string) *Note {
	t.Helper()
	n := &Note{
		NotebookID:  notebookID,
		Title:       title,
		Content:     "# " + title,
		ContentHTML: "<h1>" + title + "</h1>",
		Tags:        tags,
	}
	if err := s.CreateNote(n); err != nil {
		t.Fatalf("create note %s: %v", title, err)
	}
	return n
}

// TestEnsureNotebookTablesIdempotent 重复建 notebooks/notes 表应幂等。
func TestEnsureNotebookTablesIdempotent(t *testing.T) {
	s := newNotebookStore(t)
	for i := 0; i < 3; i++ {
		if err := s.EnsureNotebooksTable(); err != nil {
			t.Fatalf("re-ensure notebooks #%d: %v", i, err)
		}
		if err := s.EnsureNotesTable(); err != nil {
			t.Fatalf("re-ensure notes #%d: %v", i, err)
		}
	}
}

// TestCreateGetNotebook 创建后按 id / name 查回；未找到返回 nil, nil。
func TestCreateGetNotebook(t *testing.T) {
	s := newNotebookStore(t)
	nb := mkNotebook(t, s, "工作", 1)

	got, err := s.GetNotebook(nb.ID)
	if err != nil || got == nil {
		t.Fatalf("get by id: %v (nil=%v)", err, got == nil)
	}
	if got.ID != nb.ID || got.Name != "工作" || got.Order != 1 {
		t.Fatalf("field mismatch: %+v", got)
	}
	if got.Description != "desc-工作" || got.Cover != "cover-工作" {
		t.Fatalf("desc/cover mismatch: %+v", got)
	}
	if got.CreatedAt.IsZero() {
		t.Fatalf("created_at should be set: %+v", got)
	}

	byName, err := s.GetNotebookByName("工作")
	if err != nil || byName == nil || byName.ID != nb.ID {
		t.Fatalf("get by name: %v (nil=%v)", err, byName == nil)
	}

	// 未找到返回 nil, nil
	missing, err := s.GetNotebook(99999)
	if err != nil || missing != nil {
		t.Fatalf("missing id should return nil,nil: nb=%v err=%v", missing, err)
	}
	missingName, err := s.GetNotebookByName("不存在")
	if err != nil || missingName != nil {
		t.Fatalf("missing name should return nil,nil: nb=%v err=%v", missingName, err)
	}
}

// TestListNotebooksOrder 按 nb_order, id 升序。
func TestListNotebooksOrder(t *testing.T) {
	s := newNotebookStore(t)
	mkNotebook(t, s, "c", 3)
	mkNotebook(t, s, "a", 1)
	mkNotebook(t, s, "b", 2)

	nbs, err := s.ListNotebooks()
	if err != nil {
		t.Fatalf("list notebooks: %v", err)
	}
	if len(nbs) != 3 {
		t.Fatalf("want 3 notebooks, got %d", len(nbs))
	}
	for i, want := range []string{"a", "b", "c"} {
		if nbs[i].Name != want {
			t.Errorf("order[%d] want %s, got %s", i, want, nbs[i].Name)
		}
	}
}

// TestDeleteNotebookCascade 删除笔记本时级联删除其下所有笔记，其它笔记本不受影响。
func TestDeleteNotebookCascade(t *testing.T) {
	s := newNotebookStore(t)
	nb := mkNotebook(t, s, "A", 0)
	other := mkNotebook(t, s, "B", 0)
	n1 := mkNote(t, s, nb.ID, "n1", "")
	n2 := mkNote(t, s, nb.ID, "n2", "")
	keep := mkNote(t, s, other.ID, "keep", "")

	if err := s.DeleteNotebook(nb.ID); err != nil {
		t.Fatalf("delete notebook: %v", err)
	}
	// 笔记本本身已删除
	got, err := s.GetNotebook(nb.ID)
	if err != nil || got != nil {
		t.Fatalf("notebook should be gone: nb=%v err=%v", got, err)
	}
	// 其下笔记被级联删除
	for _, id := range []int64{n1.ID, n2.ID} {
		n, err := s.GetNote(id)
		if err != nil || n != nil {
			t.Fatalf("note %d should be cascaded: note=%v err=%v", id, n, err)
		}
	}
	// 其它笔记本及其笔记不受影响
	if _, total, err := s.ListNotesByNotebook(other.ID, 10, 0); err != nil || total != 1 {
		t.Fatalf("other notebook notes should remain: total=%d err=%v", total, err)
	}
	if n, err := s.GetNote(keep.ID); err != nil || n == nil {
		t.Fatalf("note keep should remain: note=%v err=%v", n, err)
	}
	// 重复删除不报错
	if err := s.DeleteNotebook(nb.ID); err != nil {
		t.Fatalf("delete nonexistent notebook should be silent: %v", err)
	}
}

// TestCreateGetNote 创建后按 id 查回；未找到返回 nil, nil。
func TestCreateGetNote(t *testing.T) {
	s := newNotebookStore(t)
	nb := mkNotebook(t, s, "A", 0)
	n := mkNote(t, s, nb.ID, "hello", "Go,Web")

	got, err := s.GetNote(n.ID)
	if err != nil || got == nil {
		t.Fatalf("get note: %v (nil=%v)", err, got == nil)
	}
	if got.ID != n.ID || got.NotebookID != nb.ID || got.Title != "hello" {
		t.Fatalf("field mismatch: %+v", got)
	}
	if got.Content != "# hello" || got.ContentHTML != "<h1>hello</h1>" || got.Tags != "Go,Web" {
		t.Fatalf("content mismatch: %+v", got)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("timestamps should be set: %+v", got)
	}

	missing, err := s.GetNote(99999)
	if err != nil || missing != nil {
		t.Fatalf("missing note should return nil,nil: note=%v err=%v", missing, err)
	}
}

// TestListNotesByNotebookPagination 分页（limit/offset）+ 总数，按最新优先。
func TestListNotesByNotebookPagination(t *testing.T) {
	s := newNotebookStore(t)
	nb := mkNotebook(t, s, "A", 0)
	other := mkNotebook(t, s, "B", 0)
	for i := 1; i <= 9; i++ {
		mkNote(t, s, nb.ID, "note"+string(rune('0'+i)), "")
	}
	mkNote(t, s, other.ID, "other-note", "")

	// 第一页：最新 3 条（id 倒序 9,8,7）
	notes, total, err := s.ListNotesByNotebook(nb.ID, 3, 0)
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	if total != 9 {
		t.Fatalf("total want 9, got %d", total)
	}
	if len(notes) != 3 {
		t.Fatalf("page 1 want 3 notes, got %d", len(notes))
	}
	for i, wantID := range []int64{9, 8, 7} {
		if notes[i].ID != wantID {
			t.Errorf("page1[%d] want id %d, got %d", i, wantID, notes[i].ID)
		}
	}

	// 第二页：6,5,4
	notes, total, err = s.ListNotesByNotebook(nb.ID, 3, 3)
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if total != 9 || len(notes) != 3 {
		t.Fatalf("page 2: total=%d len=%d", total, len(notes))
	}
	for i, wantID := range []int64{6, 5, 4} {
		if notes[i].ID != wantID {
			t.Errorf("page2[%d] want id %d, got %d", i, wantID, notes[i].ID)
		}
	}

	// 第三页：3,2,1
	notes, total, err = s.ListNotesByNotebook(nb.ID, 3, 6)
	if err != nil {
		t.Fatalf("page 3: %v", err)
	}
	if total != 9 || len(notes) != 3 {
		t.Fatalf("page 3: total=%d len=%d", total, len(notes))
	}
	for i, wantID := range []int64{3, 2, 1} {
		if notes[i].ID != wantID {
			t.Errorf("page3[%d] want id %d, got %d", i, wantID, notes[i].ID)
		}
	}

	// 越界页返回空列表，总数不变
	notes, total, err = s.ListNotesByNotebook(nb.ID, 3, 9)
	if err != nil {
		t.Fatalf("page 4: %v", err)
	}
	if total != 9 || len(notes) != 0 {
		t.Fatalf("page 4: total=%d len=%d", total, len(notes))
	}

	// 其它笔记本的笔记不混入
	_, total, err = s.ListNotesByNotebook(other.ID, 10, 0)
	if err != nil || total != 1 {
		t.Fatalf("other notebook total want 1, got %d err=%v", total, err)
	}
}

// TestUpdateNote 更新字段并验证 updated_at 自动刷新；笔记不存在报错。
func TestUpdateNote(t *testing.T) {
	s := newNotebookStore(t)
	nb := mkNotebook(t, s, "A", 0)
	n := mkNote(t, s, nb.ID, "old", "Go")
	old := n.UpdatedAt

	n.Title = "新标题"
	n.Content = "# 新内容"
	n.ContentHTML = "<h1>新内容</h1>"
	n.Tags = "Go,SQLite"
	if err := s.UpdateNote(n); err != nil {
		t.Fatalf("update: %v", err)
	}
	// 内存对象 updated_at 应被刷新（晚于创建时）
	if !n.UpdatedAt.After(old) {
		t.Fatalf("updated_at should be refreshed: old=%v new=%v", old, n.UpdatedAt)
	}

	got, err := s.GetNote(n.ID)
	if err != nil || got == nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Title != "新标题" || got.Content != "# 新内容" || got.ContentHTML != "<h1>新内容</h1>" || got.Tags != "Go,SQLite" {
		t.Fatalf("update not persisted: %+v", got)
	}
	// 库中 updated_at 不应早于创建时（驱动按秒截断，允许相等）
	if got.UpdatedAt.Before(old) {
		t.Fatalf("persisted updated_at should not be older than created: created=%v updated=%v", old, got.UpdatedAt)
	}

	// 更新不存在的笔记应报错
	if err := s.UpdateNote(&Note{ID: 99999}); err == nil {
		t.Error("update nonexistent note should error")
	}
}

// TestDeleteNote 硬删除后查不到；重复删除不报错；不影响其它笔记。
func TestDeleteNote(t *testing.T) {
	s := newNotebookStore(t)
	nb := mkNotebook(t, s, "A", 0)
	n := mkNote(t, s, nb.ID, "n1", "")
	keep := mkNote(t, s, nb.ID, "n2", "")

	if err := s.DeleteNote(n.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	got, err := s.GetNote(n.ID)
	if err != nil || got != nil {
		t.Fatalf("note should be gone: note=%v err=%v", got, err)
	}
	if k, err := s.GetNote(keep.ID); err != nil || k == nil {
		t.Fatalf("other note should remain: note=%v err=%v", k, err)
	}
	// 删除不存在的 id 静默
	if err := s.DeleteNote(n.ID); err != nil {
		t.Fatalf("delete nonexistent should be silent: %v", err)
	}
}

// TestAllNoteTags 多笔记标签聚合：去重、去空白、排序；空库返回空列表。
func TestAllNoteTags(t *testing.T) {
	s := newNotebookStore(t)
	nb := mkNotebook(t, s, "A", 0)
	mkNote(t, s, nb.ID, "n1", "Go, Web")
	mkNote(t, s, nb.ID, "n2", "Web, SQLite")
	mkNote(t, s, nb.ID, "n3", "Go, Go, SQLite,  ") // 重复 + 逗号尾巴空段
	mkNote(t, s, nb.ID, "n4", "")                  // 无标签不参与

	tags, err := s.AllNoteTags()
	if err != nil {
		t.Fatalf("all note tags: %v", err)
	}
	want := []string{"Go", "SQLite", "Web"}
	if !reflect.DeepEqual(tags, want) {
		t.Fatalf("tags want %v, got %v", want, tags)
	}

	// 空库返回空列表（无错误）
	empty := newNotebookStore(t)
	got, err := empty.AllNoteTags()
	if err != nil {
		t.Fatalf("empty all note tags: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("empty store want 0 tags, got %v", got)
	}
}

// TestCountNoteTagsByNotebook 按笔记本聚合标签计数：跨笔记本隔离、去空白、空标签跳过；空库空 map。
func TestCountNoteTagsByNotebook(t *testing.T) {
	s := newNotebookStore(t)
	a := mkNotebook(t, s, "A", 0)
	b := mkNotebook(t, s, "B", 0)
	mkNote(t, s, a.ID, "n1", "Go, Web")
	mkNote(t, s, a.ID, "n2", "Web, SQLite")
	mkNote(t, s, a.ID, "n3", "Go, Go, SQLite,  ") // 重复 + 逗号尾巴空段
	mkNote(t, s, a.ID, "n4", "")                  // 无标签不参与
	mkNote(t, s, b.ID, "m1", "Go")                // 其它笔记本隔离

	counts, err := s.CountNoteTagsByNotebook(a.ID)
	if err != nil {
		t.Fatalf("count note tags: %v", err)
	}
	want := map[string]int{"Go": 3, "Web": 2, "SQLite": 2}
	if !reflect.DeepEqual(counts, want) {
		t.Fatalf("counts want %v, got %v", want, counts)
	}

	// 无标签笔记本/空库返回空 map（无错误）
	empty := newNotebookStore(t)
	nb := mkNotebook(t, empty, "empty", 0)
	got, err := empty.CountNoteTagsByNotebook(nb.ID)
	if err != nil {
		t.Fatalf("empty count: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("empty notebook want 0 tags, got %v", got)
	}
}

// TestNotebookUniqueName 笔记本 name 唯一约束；重复创建报错。
func TestNotebookUniqueName(t *testing.T) {
	s := newNotebookStore(t)
	mkNotebook(t, s, "工作", 0)

	if err := s.CreateNotebook(&Notebook{Name: "工作", Order: 1}); err == nil {
		t.Fatal("duplicate name should violate UNIQUE constraint")
	}
	// 不同名正常
	if err := s.CreateNotebook(&Notebook{Name: "学习", Order: 2}); err != nil {
		t.Fatalf("different name: %v", err)
	}
	// 重名后按名字仍能取到第一条
	if nb, err := s.GetNotebookByName("工作"); err != nil || nb == nil {
		t.Fatalf("get by name after dup: nb=%v err=%v", nb, err)
	}
}
