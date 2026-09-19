package store

import (
	"errors"
	"strings"
	"testing"
	"time"

	"blog/internal/content"
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

func TestPostAuthorsRoundTripAndLegacyBackfill(t *testing.T) {
	s := newTestStore(t)
	p := pubPost("multi-author", "多作者")
	p.Author = "alice"
	p.Authors = []string{"alice", " bob ", "alice", "carol"}
	id, err := s.Create(p)
	if err != nil {
		t.Fatalf("create multi-author post: %v", err)
	}
	got, err := s.GetBySlug(p.Slug)
	if err != nil {
		t.Fatalf("get multi-author post: %v", err)
	}
	if strings.Join(got.Authors, ",") != "alice,bob,carol" || got.Author != "alice" {
		t.Fatalf("authors = %#v, primary = %q", got.Authors, got.Author)
	}
	if count, err := s.CountByAuthor("bob"); err != nil || count != 1 {
		t.Fatalf("count by secondary author = %d, err=%v", count, err)
	}
	if posts, err := s.ListByAuthor("carol", 0, 10); err != nil || len(posts) != 1 || posts[0].Slug != p.Slug {
		t.Fatalf("list by secondary author = %#v, err=%v", posts, err)
	}

	if err := s.Update(p.Slug, &Post{Title: "更新", Status: StatusPublished, Authors: []string{"carol", "bob"}}); err != nil {
		t.Fatalf("update authors: %v", err)
	}
	got, _ = s.GetBySlug(p.Slug)
	if strings.Join(got.Authors, ",") != "carol,bob" || got.Author != "carol" {
		t.Fatalf("updated authors = %#v, primary = %q", got.Authors, got.Author)
	}

	// 模拟旧库只有 posts.author：迁移必须幂等回填顺序 0 记录。
	if _, err := s.db.Exec(`DELETE FROM post_authors WHERE post_id = ?`, id); err != nil {
		t.Fatalf("clear association: %v", err)
	}
	if _, err := s.db.Exec(`UPDATE posts SET author = 'legacy' WHERE id = ?`, id); err != nil {
		t.Fatalf("set legacy author: %v", err)
	}
	if err := s.migrateColumns(); err != nil {
		t.Fatalf("rerun author migration: %v", err)
	}
	got, _ = s.GetBySlug(p.Slug)
	if len(got.Authors) != 1 || got.Authors[0] != "legacy" {
		t.Fatalf("legacy backfill authors = %#v", got.Authors)
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

func TestPostUpdateSlugKeepsIdentityAndMigratesAssociations(t *testing.T) {
	s := newTestStore(t)
	p := pubPost("old-slug", "旧标题")
	p.Authors = []string{"alice", "bob"}
	p.Categories = []string{"技术/Go", "生活"}
	id, err := s.Create(p)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.DB().Exec(`UPDATE posts SET views = 7 WHERE id = ?`, id); err != nil {
		t.Fatalf("set views: %v", err)
	}
	if _, err := s.CreateRevision(&Revision{PostSlug: "old-slug", Title: "更早版本", ContentMD: "old"}); err != nil {
		t.Fatalf("create revision: %v", err)
	}
	before, err := s.GetBySlug("old-slug")
	if err != nil {
		t.Fatalf("get before: %v", err)
	}
	var events []string
	s.SetPostWriteHook(func(op, slug string) { events = append(events, op+":"+slug) })

	updated := pubPost("new-slug", "新标题")
	updated.Authors = []string{"carol", "dave"}
	updated.Categories = []string{"产品"}
	if err := s.UpdateSlug("old-slug", "new-slug", updated); err != nil {
		t.Fatalf("update slug: %v", err)
	}
	after, err := s.GetBySlug("new-slug")
	if err != nil {
		t.Fatalf("get after: %v", err)
	}
	if after.ID != id || !after.CreatedAt.Equal(before.CreatedAt) || after.Views != 7 {
		t.Fatalf("identity/read count changed: before=%+v after=%+v", before, after)
	}
	if strings.Join(after.Authors, ",") != "carol,dave" || strings.Join(after.Categories, ",") != "产品" {
		t.Fatalf("associations not replaced atomically: authors=%v categories=%v", after.Authors, after.Categories)
	}
	if old, _ := s.GetBySlug("old-slug"); old != nil {
		t.Fatal("old slug should no longer resolve")
	}
	revisions, err := s.ListRevisions("new-slug")
	if err != nil || len(revisions) != 1 || revisions[0].PostSlug != "new-slug" {
		t.Fatalf("revision slug not migrated: revisions=%+v err=%v", revisions, err)
	}
	if got := strings.Join(events, ","); got != "delete:old-slug,update:new-slug" {
		t.Fatalf("write hooks = %q, want old delete + new update", got)
	}
}

func TestPostUpdateSlugConflictAndAssociationFailureRollback(t *testing.T) {
	s := newTestStore(t)
	first := pubPost("first", "第一篇")
	first.Authors = []string{"alice"}
	first.Categories = []string{"旧分类"}
	if _, err := s.Create(first); err != nil {
		t.Fatalf("create first: %v", err)
	}
	if _, err := s.Create(pubPost("occupied", "占用 slug")); err != nil {
		t.Fatalf("create occupied: %v", err)
	}
	if err := s.UpdateSlug("first", "occupied", pubPost("occupied", "不应写入")); !errors.Is(err, ErrPostSlugConflict) {
		t.Fatalf("slug conflict = %v, want ErrPostSlugConflict", err)
	}
	got, err := s.GetBySlug("first")
	if err != nil || got == nil || got.Title != "第一篇" {
		t.Fatalf("conflict changed old post: post=%+v err=%v", got, err)
	}

	if _, err := s.DB().Exec(`CREATE TRIGGER fail_post_category BEFORE INSERT ON post_categories BEGIN SELECT RAISE(ABORT, 'forced association failure'); END`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
	defer s.DB().Exec(`DROP TRIGGER fail_post_category`)
	updated := pubPost("renamed", "不应提交")
	updated.Authors = []string{"bob"}
	updated.Categories = []string{"新分类"}
	if err := s.UpdateSlug("first", "renamed", updated); err == nil {
		t.Fatal("association failure should fail the transaction")
	}
	got, err = s.GetBySlug("first")
	if err != nil || got == nil || got.Title != "第一篇" || strings.Join(got.Authors, ",") != "alice" || strings.Join(got.Categories, ",") != "旧分类" {
		t.Fatalf("association failure did not roll back: post=%+v err=%v", got, err)
	}
	if renamed, _ := s.GetBySlug("renamed"); renamed != nil {
		t.Fatal("failed slug update created a new record")
	}
}

func TestPostUpdateSlugWithRevisionIsAtomicAndPrunes(t *testing.T) {
	s := newTestStore(t)
	p := pubPost("revision-old", "旧标题")
	p.Authors = []string{"alice"}
	p.Categories = []string{"旧分类"}
	if _, err := s.Create(p); err != nil {
		t.Fatalf("create post: %v", err)
	}
	for i := 0; i < maxRevisionsPerPost; i++ {
		if _, err := s.CreateRevision(&Revision{PostSlug: p.Slug, Title: "历史", ContentMD: "历史正文"}); err != nil {
			t.Fatalf("create revision %d: %v", i, err)
		}
	}

	events := content.NewEvents()
	var eventCount int
	events.Subscribe(func(content.Event) { eventCount++ })
	s.SetContentEvents(events)
	if _, err := s.DB().Exec(`CREATE TRIGGER fail_revision_insert BEFORE INSERT ON revisions BEGIN SELECT RAISE(ABORT, 'forced revision failure'); END`); err != nil {
		t.Fatalf("create revision trigger: %v", err)
	}
	failed := pubPost("revision-new", "不应提交")
	failed.Authors = []string{"bob"}
	failed.Categories = []string{"新分类"}
	if err := s.UpdateSlugWithRevision(p.Slug, failed.Slug, failed); err == nil {
		t.Fatal("revision failure should fail the transaction")
	}
	if eventCount != 0 {
		t.Fatalf("failed save emitted %d events, want 0", eventCount)
	}
	got, err := s.GetBySlug(p.Slug)
	if err != nil || got == nil || got.Title != "旧标题" || got.ContentMD != p.ContentMD || strings.Join(got.Authors, ",") != "alice" || strings.Join(got.Categories, ",") != "旧分类" {
		t.Fatalf("failed save changed post: post=%+v err=%v", got, err)
	}
	if revisions, _ := s.ListRevisions(p.Slug); len(revisions) != maxRevisionsPerPost {
		t.Fatalf("failed save changed revisions: got %d", len(revisions))
	}
	if renamed, _ := s.GetBySlug(failed.Slug); renamed != nil {
		t.Fatal("failed save created renamed post")
	}
	if _, err := s.DB().Exec(`DROP TRIGGER fail_revision_insert`); err != nil {
		t.Fatalf("drop revision trigger: %v", err)
	}

	updated := pubPost(failed.Slug, "新标题")
	updated.ContentMD = "新正文"
	updated.Authors = []string{"bob"}
	updated.Categories = []string{"新分类"}
	if err := s.UpdateSlugWithRevision(p.Slug, updated.Slug, updated); err != nil {
		t.Fatalf("successful save: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("successful save emitted %d events, want 1", eventCount)
	}
	if revisions, _ := s.ListRevisions(updated.Slug); len(revisions) != maxRevisionsPerPost {
		t.Fatalf("revision limit = %d, want %d", len(revisions), maxRevisionsPerPost)
	}
	latest, _ := s.GetRevision(revisionIDForSlug(t, s, updated.Slug))
	if latest == nil || latest.ContentMD != p.ContentMD {
		t.Fatal("expected a retained revision after successful save")
	}

	metadataOnly := pubPost(updated.Slug, updated.Title)
	metadataOnly.ContentMD = updated.ContentMD
	metadataOnly.Tags = "new-tag"
	metadataOnly.Authors = []string{"bob"}
	metadataOnly.Categories = []string{"新分类"}
	if err := s.UpdateSlugWithRevision(updated.Slug, updated.Slug, metadataOnly); err != nil {
		t.Fatalf("metadata-only save: %v", err)
	}
	if revisions, _ := s.ListRevisions(updated.Slug); len(revisions) != maxRevisionsPerPost {
		t.Fatalf("metadata-only save created revision: got %d", len(revisions))
	}
	if eventCount != 2 {
		t.Fatalf("metadata-only save emitted %d events, want 2", eventCount)
	}
}

func revisionIDForSlug(t *testing.T, s *Store, slug string) int64 {
	t.Helper()
	rows, err := s.DB().Query(`SELECT id FROM revisions WHERE post_slug = ? ORDER BY id DESC LIMIT 1`, slug)
	if err != nil {
		t.Fatalf("query revision id: %v", err)
	}
	defer rows.Close()
	var id int64
	if !rows.Next() || rows.Scan(&id) != nil {
		t.Fatal("missing revision id")
	}
	return id
}

func TestPostDraftHead(t *testing.T) {
	s := newTestStore(t)
	s.Create(pubPost("d", "线上版"))

	// 保存草稿：不影响线上内容
	if err := s.SaveDraft("d", `{"title":"草稿版"}`); err != nil {
		t.Fatalf("save draft: %v", err)
	}
	got, _ := s.GetBySlug("d")
	if got.DraftJSON != `{"title":"草稿版"}` {
		t.Fatalf("draft_json = %q", got.DraftJSON)
	}
	if got.Title != "线上版" {
		t.Fatalf("draft 不应影响线上 title，got %q", got.Title)
	}

	// Update 不应清掉草稿列
	if err := s.Update("d", &Post{Title: "线上版v2", Status: StatusPublished}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = s.GetBySlug("d")
	if got.DraftJSON == "" {
		t.Fatal("update 不应清空 draft_json")
	}

	// 清空草稿
	if err := s.ClearDraft("d"); err != nil {
		t.Fatalf("clear draft: %v", err)
	}
	got, _ = s.GetBySlug("d")
	if got.DraftJSON != "" {
		t.Fatalf("draft_json 应为空，got %q", got.DraftJSON)
	}

	// 不存在的文章保存草稿报错
	if err := s.SaveDraft("ghost", "{}"); err == nil {
		t.Fatal("expected error saving draft for non-existent slug")
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
