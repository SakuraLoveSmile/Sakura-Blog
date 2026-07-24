package store

import (
	"testing"
	"time"
)

// 便捷构造一篇已发布文章。
func pubPost(slug, title string) *Post {
	return &Post{Slug: slug, Title: title, Status: StatusPublished, ContentMD: "# " + title}
}

func TestPostCreateAndGet(t *testing.T) {
	s := newTestStore(t)
	p := pubPost("hello", "你好世界")
	p.Tags = "Go,Gin"
	p.Category = "tech"
	id, err := s.Create(p)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}
	got, err := s.GetBySlug("hello")
	if err != nil || got == nil {
		t.Fatalf("get by slug: %v, nil=%v", err, got == nil)
	}
	if got.Title != "你好世界" || got.Tags != "Go,Gin" || got.Category != "tech" {
		t.Fatalf("field mismatch: %+v", got)
	}
	if got.Status != StatusPublished {
		t.Fatalf("status = %s, want published", got.Status)
	}
}

func TestPostCreateDuplicateSlug(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Create(pubPost("dup", "a")); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := s.Create(pubPost("dup", "b")); err == nil {
		t.Fatal("expected duplicate slug error, got nil")
	}
}

func TestPostUpdate(t *testing.T) {
	s := newTestStore(t)
	s.Create(pubPost("u", "old"))
	if err := s.Update("u", &Post{Title: "new", Category: "life", Status: StatusPublished}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := s.GetBySlug("u")
	if got.Title != "new" || got.Category != "life" {
		t.Fatalf("update not applied: %+v", got)
	}
	if err := s.Update("ghost", &Post{Title: "x"}); err == nil {
		t.Fatal("expected error updating non-existent slug")
	}
}

func TestListPublishedFilters(t *testing.T) {
	s := newTestStore(t)
	s.Create(pubPost("pub", "已发布"))
	s.Create(&Post{Slug: "draft", Title: "草稿", Status: StatusDraft})
	// 软删除文章不列出
	s.Create(pubPost("del", "已删除"))
	s.Delete("del")
	// 未来定时发布不列出
	future := pubPost("future", "未到点")
	future.PublishAt = time.Now().Add(24 * time.Hour)
	s.Create(future)
	// 已到点定时发布应列出
	past := pubPost("past", "已到点")
	past.PublishAt = time.Now().Add(-time.Hour)
	s.Create(past)

	list, err := s.ListPublished()
	if err != nil {
		t.Fatalf("list published: %v", err)
	}
	got := map[string]bool{}
	for _, p := range list {
		got[p.Slug] = true
	}
	if !got["pub"] || !got["past"] {
		t.Fatalf("expected pub & past listed, got %v", got)
	}
	if got["draft"] || got["del"] || got["future"] {
		t.Fatalf("draft/deleted/future must be filtered, got %v", got)
	}
}

func TestListPublishedPinnedFirst(t *testing.T) {
	s := newTestStore(t)
	old := pubPost("old", "旧文")
	old.CreatedAt = time.Now().Add(-48 * time.Hour)
	s.Create(old)
	pinned := pubPost("pinned", "置顶")
	pinned.Pinned = true
	pinned.CreatedAt = time.Now().Add(-72 * time.Hour) // 更旧但置顶
	s.Create(pinned)

	list, _ := s.ListPublished()
	if len(list) < 2 || list[0].Slug != "pinned" {
		t.Fatalf("expected pinned first, got %+v", list)
	}
}

func TestPostTrashRestorePurge(t *testing.T) {
	s := newTestStore(t)
	s.Create(pubPost("t1", "回收站"))

	if err := s.Delete("t1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	trashed, _ := s.ListTrashed()
	if len(trashed) != 1 || trashed[0].Slug != "t1" {
		t.Fatalf("expected t1 in trash, got %+v", trashed)
	}
	// GetBySlug 是管理视角（含回收站），GetPublishedBySlug 是前台视角（排除回收站）。
	if got, _ := s.GetBySlug("t1"); got == nil {
		t.Fatal("GetBySlug (admin view) should still see trashed post")
	}
	if got, _ := s.GetPublishedBySlug("t1"); got != nil {
		t.Fatal("GetPublishedBySlug must exclude trashed post")
	}

	if err := s.RestorePost("t1"); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if got, _ := s.GetPublishedBySlug("t1"); got == nil {
		t.Fatal("restored post should be visible again")
	}

	s.Delete("t1")
	if err := s.PurgePost("t1"); err != nil {
		t.Fatalf("purge: %v", err)
	}
	trashed, _ = s.ListTrashed()
	if len(trashed) != 0 {
		t.Fatalf("expected empty trash after purge, got %+v", trashed)
	}
}

func TestCountByTagAndCategory(t *testing.T) {
	s := newTestStore(t)
	a := pubPost("a", "A")
	a.Tags = "Go,Gin"
	a.Category = "tech"
	s.Create(a)
	b := pubPost("b", "B")
	b.Tags = "Go"
	b.Category = "tech"
	s.Create(b)
	c := pubPost("c", "C")
	c.Tags = "生活"
	c.Category = "life"
	s.Create(c)

	tags, err := s.CountByTag()
	if err != nil {
		t.Fatalf("count by tag: %v", err)
	}
	if tags["Go"] != 2 || tags["Gin"] != 1 || tags["生活"] != 1 {
		t.Fatalf("unexpected tag counts: %v", tags)
	}

	cats, err := s.CountByCategory()
	if err != nil {
		t.Fatalf("count by category: %v", err)
	}
	if cats["tech"] != 2 || cats["life"] != 1 {
		t.Fatalf("unexpected category counts: %v", cats)
	}

	byTag, _ := s.ListByTag("Go")
	if len(byTag) != 2 {
		t.Fatalf("ListByTag(Go) = %d, want 2", len(byTag))
	}
	byCat, _ := s.ListByCategory("tech")
	if len(byCat) != 2 {
		t.Fatalf("ListByCategory(tech) = %d, want 2", len(byCat))
	}
}

func TestListPublishedPaged(t *testing.T) {
	s := newTestStore(t)
	// 截断到秒：避免纳秒+时区格式导致 SQLite datetime() 解析失效、排序不稳定。
	base := time.Now().UTC().Truncate(time.Second)
	for i := 0; i < 7; i++ {
		p := pubPost(string(rune('a'+i)), "p")
		p.CreatedAt = base.Add(time.Duration(i) * time.Minute)
		s.Create(p)
	}
	page1, _ := s.ListPublishedPaged(1, 5)
	if len(page1) != 5 {
		t.Fatalf("page1 = %d, want 5", len(page1))
	}
	page2, _ := s.ListPublishedPaged(2, 5)
	if len(page2) != 2 {
		t.Fatalf("page2 = %d, want 2", len(page2))
	}
	if n, _ := s.CountPublished(); n != 7 {
		t.Fatalf("count = %d, want 7", n)
	}
	// 分页不重叠（按 created_at 倒序，g 最新在 page1 首位）
	if page1[0].Slug != "g" || page2[0].Slug == page1[0].Slug {
		t.Fatalf("pagination overlap or wrong order: %s / %s", page1[0].Slug, page2[0].Slug)
	}
}
