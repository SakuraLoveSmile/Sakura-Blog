package store

import (
	"path/filepath"
	"testing"
)

// newTopicStore 建库 + topics 表 + posts.topic_id 列迁移（测试 Create 需要列存在）。
func newTopicStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := s.EnsureTopicsTable(); err != nil {
		t.Fatalf("ensure topics table: %v", err)
	}
	if err := s.MigrateTopicBackfill(); err != nil {
		t.Fatalf("migrate topic backfill: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// mkTopic 便捷插入一个专栏（ID 回填），返回指针供断言用。
func mkTopic(t *testing.T, s *Store, name, slug string, sort int) *Topic {
	t.Helper()
	tp := &Topic{Name: name, Slug: slug, Sort: sort}
	if err := s.CreateTopic(tp); err != nil {
		t.Fatalf("create topic %s: %v", name, err)
	}
	return tp
}

// TestEnsureTopicsTableIdempotent 重复建 topics 表应幂等（含索引）。
func TestEnsureTopicsTableIdempotent(t *testing.T) {
	s := newTopicStore(t)
	for i := 0; i < 3; i++ {
		if err := s.EnsureTopicsTable(); err != nil {
			t.Fatalf("re-ensure topics #%d: %v", i, err)
		}
	}
}

// TestTopicCreateGet 创建后按 id / slug / name 查回；未找到返回 nil, nil。
func TestTopicCreateGet(t *testing.T) {
	s := newTopicStore(t)
	tp := mkTopic(t, s, "Go 专栏", "go", 1)

	byID, err := s.GetTopicByID(tp.ID)
	if err != nil || byID == nil {
		t.Fatalf("get by id: %v (nil=%v)", err, byID == nil)
	}
	if byID.Name != "Go 专栏" || byID.Slug != "go" || byID.Sort != 1 {
		t.Fatalf("field mismatch: %+v", byID)
	}
	bySlug, err := s.GetTopicBySlug("go")
	if err != nil || bySlug == nil || bySlug.ID != tp.ID {
		t.Fatalf("get by slug: %v nil=%v", err, bySlug == nil)
	}
	byName, err := s.GetTopicByName("Go 专栏")
	if err != nil || byName == nil || byName.ID != tp.ID {
		t.Fatalf("get by name: %v nil=%v", err, byName == nil)
	}
	if got, _ := s.GetTopicBySlug("ghost"); got != nil {
		t.Fatalf("get ghost slug should be nil, got %+v", got)
	}
}

// TestTopicCreateDuplicate 双唯一约束：name / slug 重复均报错（API 层映射 409）。
func TestTopicCreateDuplicate(t *testing.T) {
	s := newTopicStore(t)
	mkTopic(t, s, "专栏A", "a", 0)
	if err := s.CreateTopic(&Topic{Name: "专栏A", Slug: "b"}); err == nil {
		t.Fatal("duplicate name should error")
	}
	if err := s.CreateTopic(&Topic{Name: "专栏B", Slug: "a"}); err == nil {
		t.Fatal("duplicate slug should error")
	}
}

// TestTopicUpdate 更新字段并刷新 updated_at；专栏不存在时报错。
func TestTopicUpdate(t *testing.T) {
	s := newTopicStore(t)
	tp := mkTopic(t, s, "旧名", "old", 0)
	tp.Name, tp.Slug, tp.Description = "新名", "new", "desc"
	if err := s.UpdateTopic(tp); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := s.GetTopicByID(tp.ID)
	if got.Name != "新名" || got.Slug != "new" || got.Description != "desc" {
		t.Fatalf("update not applied: %+v", got)
	}
	if err := s.UpdateTopic(&Topic{ID: 99999, Name: "x"}); err == nil {
		t.Fatal("update non-existent should error")
	}
}

// TestTopicDeleteProtection 删除保护：有关联文章时 ErrTopicInUse；解除关联后可删。
func TestTopicDeleteProtection(t *testing.T) {
	s := newTopicStore(t)
	tp := mkTopic(t, s, "Go 专栏", "go", 0)
	p := pubPost("topic-post", "一篇专栏文章")
	p.Topic = "Go 专栏"
	if _, err := s.Create(p); err != nil {
		t.Fatalf("create post: %v", err)
	}
	if err := s.MigrateTopicBackfill(); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	if err := s.DeleteTopic(tp.ID); err != ErrTopicInUse {
		t.Fatalf("delete with posts = %v, want ErrTopicInUse", err)
	}
	if n, _ := s.CountByTopicID(tp.ID); n != 1 {
		t.Fatalf("count published = %d, want 1", n)
	}

	// 解除关联后可删除
	p.TopicID = 0
	if err := s.Update(p.Slug, p); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if err := s.DeleteTopic(tp.ID); err != nil {
		t.Fatalf("delete after unlink: %v", err)
	}
	if got, _ := s.GetTopicByID(tp.ID); got != nil {
		t.Fatalf("topic should be deleted, got %+v", got)
	}
}

// TestTopicListOrder ListTopics 按 sort DESC + 创建时间线性（时间倒序）。
func TestTopicListOrder(t *testing.T) {
	s := newTopicStore(t)
	a := mkTopic(t, s, "A", "a", 0) // 先建
	b := mkTopic(t, s, "B", "b", 0) // 后建
	c := mkTopic(t, s, "C", "c", 2) // sort 最高

	got, err := s.ListTopics()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].ID != c.ID {
		t.Fatalf("first should be sort-high C (id=%d), got id=%d", c.ID, got[0].ID)
	}
	if got[1].ID != b.ID || got[2].ID != a.ID {
		t.Fatalf("time order wrong: %d, %d", got[1].ID, got[2].ID)
	}
}

// TestMigrateTopicBackfill 存量 topic 文本 → topics 行 + 回填 topic_id；幂等二次执行无变化。
func TestMigrateTopicBackfill(t *testing.T) {
	s := newTopicStore(t)
	mkTopic(t, s, "已有专栏", "existing", 0) // 预置同名专栏：回填应复用不重建

	posts := []*Post{
		{Slug: "p1", Title: "一", Status: StatusPublished, ContentMD: "# 一", Topic: "Go 专栏"},
		{Slug: "p2", Title: "二", Status: StatusPublished, ContentMD: "# 二", Topic: "Go 专栏"},
		{Slug: "p3", Title: "三", Status: StatusPublished, ContentMD: "# 三", Topic: "前端"},
	}
	for _, p := range posts {
		if _, err := s.Create(p); err != nil {
			t.Fatalf("create %s: %v", p.Slug, err)
		}
	}

	if err := s.MigrateTopicBackfill(); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	// Go 专栏：新建行 + 两篇文章关联
	goTopic, err := s.GetTopicByName("Go 专栏")
	if err != nil || goTopic == nil {
		t.Fatalf("get Go 专栏: %v nil=%v", err, goTopic == nil)
	}
	list, err := s.ListPostsByTopicID(goTopic.ID)
	if err != nil || len(list) != 2 {
		t.Fatalf("list posts of Go 专栏: %v len=%d, want 2", err, len(list))
	}
	if n, _ := s.CountAllByTopicID(goTopic.ID); n != 2 {
		t.Fatalf("count all = %d, want 2", n)
	}

	// 预置同名专栏未重建
	if existing, _ := s.GetTopicByName("已有专栏"); existing == nil {
		t.Fatal("pre-existing topic should be kept")
	}

	// 幂等：再次迁移不新增行、不重复回填
	if err := s.MigrateTopicBackfill(); err != nil {
		t.Fatalf("re-backfill: %v", err)
	}
	if n, _ := s.CountAllByTopicID(goTopic.ID); n != 2 {
		t.Fatalf("after re-backfill count = %d, want 2", n)
	}
}
