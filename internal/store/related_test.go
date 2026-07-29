package store

import (
	"testing"
	"time"
)

// relPost 构造指定创建时间的已发布文章。
func relPost(slug, category, tags string, day int) *Post {
	return &Post{
		Slug:      slug,
		Title:     "标题-" + slug,
		Status:    StatusPublished,
		Category:  category,
		Tags:      tags,
		ContentMD: "# " + slug,
		CreatedAt: time.Date(2026, 1, day, 12, 0, 0, 0, time.UTC),
	}
}

func mustCreate(t *testing.T, s *Store, p *Post) {
	t.Helper()
	if _, err := s.Create(p); err != nil {
		t.Fatalf("create %s: %v", p.Slug, err)
	}
}

func TestGetAdjacentPostSameCategoryFirst(t *testing.T) {
	s := newTestStore(t)
	mustCreate(t, s, relPost("a", "tech", "", 1))
	mustCreate(t, s, relPost("b", "life", "", 2))
	mustCreate(t, s, relPost("c", "tech", "", 3)) // 当前文章
	mustCreate(t, s, relPost("d", "life", "", 4))
	mustCreate(t, s, relPost("e", "tech", "", 5))

	// 同分类优先：c 的较早邻篇应是 a（跳过 life 的 b），较新邻篇应是 e（跳过 life 的 d）
	older, err := s.GetAdjacentPost("c", "tech", false)
	if err != nil {
		t.Fatalf("older: %v", err)
	}
	if older == nil || older.Slug != "a" {
		t.Fatalf("older = %v, want a", older)
	}
	newer, err := s.GetAdjacentPost("c", "tech", true)
	if err != nil {
		t.Fatalf("newer: %v", err)
	}
	if newer == nil || newer.Slug != "e" {
		t.Fatalf("newer = %v, want e", newer)
	}
}

func TestGetAdjacentPostFallbackGlobal(t *testing.T) {
	s := newTestStore(t)
	mustCreate(t, s, relPost("a", "life", "", 1))
	mustCreate(t, s, relPost("b", "tech", "", 2)) // 当前文章：同分类无邻篇
	mustCreate(t, s, relPost("c", "life", "", 3))

	// tech 分类内无邻篇，回退全局时间序
	older, err := s.GetAdjacentPost("b", "tech", false)
	if err != nil {
		t.Fatalf("older: %v", err)
	}
	if older == nil || older.Slug != "a" {
		t.Fatalf("older = %v, want a（全局回退）", older)
	}
	newer, err := s.GetAdjacentPost("b", "tech", true)
	if err != nil {
		t.Fatalf("newer: %v", err)
	}
	if newer == nil || newer.Slug != "c" {
		t.Fatalf("newer = %v, want c（全局回退）", newer)
	}

	// 无分类文章直接走全局
	older2, err := s.GetAdjacentPost("c", "", false)
	if err != nil {
		t.Fatalf("older2: %v", err)
	}
	if older2 == nil || older2.Slug != "b" {
		t.Fatalf("older2 = %v, want b", older2)
	}
}

func TestGetAdjacentPostExcludesProtectedAndUnpublished(t *testing.T) {
	s := newTestStore(t)
	mustCreate(t, s, relPost("open", "tech", "", 1))
	locked := relPost("locked", "tech", "", 2)
	locked.Password = "secret"
	mustCreate(t, s, locked)
	draft := relPost("draft", "tech", "", 3)
	draft.Status = StatusDraft
	mustCreate(t, s, draft)
	mustCreate(t, s, relPost("cur", "tech", "", 4))

	// cur 的较早邻篇应跳过加密的 locked 与草稿 draft，落在 open
	older, err := s.GetAdjacentPost("cur", "tech", false)
	if err != nil {
		t.Fatalf("older: %v", err)
	}
	if older == nil || older.Slug != "open" {
		t.Fatalf("older = %v, want open（排除加密/草稿）", older)
	}
	// 较新方向无可公开邻篇
	newer, err := s.GetAdjacentPost("cur", "tech", true)
	if err != nil {
		t.Fatalf("newer: %v", err)
	}
	if newer != nil {
		t.Fatalf("newer = %v, want nil", newer)
	}
}

func TestListRelatedPostsOrderByCommonTags(t *testing.T) {
	s := newTestStore(t)
	mustCreate(t, s, relPost("cur", "", "Go,Web,SQLite", 10))
	mustCreate(t, s, relPost("one-tag-old", "", "Go", 1))
	mustCreate(t, s, relPost("one-tag-new", "", "Web", 2))
	mustCreate(t, s, relPost("two-tags", "", "Go,SQLite", 3))
	mustCreate(t, s, relPost("no-common", "", "Rust", 4))

	got, err := s.ListRelatedPosts("cur", "Go,Web,SQLite", 5)
	if err != nil {
		t.Fatalf("list related: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3（排除自身与无共同标签）", len(got))
	}
	// 共同标签数 desc：two-tags(2) 最前；同为 1 个共同标签时时间 desc：one-tag-new > one-tag-old
	want := []string{"two-tags", "one-tag-new", "one-tag-old"}
	for i, w := range want {
		if got[i].Slug != w {
			t.Fatalf("got[%d] = %s, want %s", i, got[i].Slug, w)
		}
	}
}

func TestListRelatedPostsExcludesProtectedAndLimit(t *testing.T) {
	s := newTestStore(t)
	mustCreate(t, s, relPost("cur", "", "Go", 10))
	locked := relPost("locked", "", "Go", 5)
	locked.Password = "secret"
	mustCreate(t, s, locked)
	draft := relPost("draft", "", "Go", 6)
	draft.Status = StatusDraft
	mustCreate(t, s, draft)
	mustCreate(t, s, relPost("r1", "", "Go", 1))
	mustCreate(t, s, relPost("r2", "", "Go", 2))
	mustCreate(t, s, relPost("r3", "", "Go", 3))

	got, err := s.ListRelatedPosts("cur", "Go", 2)
	if err != nil {
		t.Fatalf("list related: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2（limit 生效）", len(got))
	}
	for _, p := range got {
		if p.Slug == "locked" || p.Slug == "draft" {
			t.Fatalf("加密/草稿文章泄漏: %s", p.Slug)
		}
	}
	if got[0].Slug != "r3" || got[1].Slug != "r2" {
		t.Fatalf("got = [%s %s], want [r3 r2]", got[0].Slug, got[1].Slug)
	}
}

func TestListRelatedPostsEmptyInput(t *testing.T) {
	s := newTestStore(t)
	mustCreate(t, s, relPost("cur", "", "", 1))

	if got, err := s.ListRelatedPosts("cur", "", 5); err != nil || got != nil {
		t.Fatalf("空标签应返回 nil: got=%v err=%v", got, err)
	}
	if got, err := s.ListRelatedPosts("cur", "Go", 0); err != nil || got != nil {
		t.Fatalf("limit=0 应返回 nil: got=%v err=%v", got, err)
	}
}
