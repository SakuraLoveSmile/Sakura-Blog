package store

import (
	"strings"
	"testing"
	"time"

	"blog/internal/content"
)

// mustGetBatchPost 读取批量状态测试用的文章；未找到直接失败。
func mustGetBatchPost(t *testing.T, s *Store, slug string) *Post {
	t.Helper()
	p, err := s.GetBySlug(slug)
	if err != nil {
		t.Fatalf("get post %s: %v", slug, err)
	}
	if p == nil {
		t.Fatalf("post %s not found", slug)
	}
	return p
}

// TestBatchStatusPublishDraftRoundTrip 覆盖 publish↔draft 双向真实落库、
// skipped 不写库（updated_at 不变）以及「只改 status/updated_at」的字段边界。
func TestBatchStatusPublishDraftRoundTrip(t *testing.T) {
	s := newTestStore(t)
	alice, err := s.GetUserByUsername("alice")
	if err != nil || alice == nil {
		t.Fatalf("get alice: %v", err)
	}
	scheduledAt := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)

	// 草稿携带完整元数据：批量发布后这些字段必须原样保留。
	draft := &Post{
		Slug: "batch-draft", Title: "草稿文章", Status: StatusDraft,
		ContentMD: "# 草稿正文", ContentHTML: "<p>已渲染缓存</p>", Tags: "go,sqlite",
		Category: "技术/Go", Categories: []string{"技术/Go"}, Pinned: true,
		Password: "secret", PublishAt: scheduledAt, DraftJSON: `{"title":"未发布修改"}`,
		AuthorIDs: []int64{alice.ID},
	}
	if _, err := s.Create(draft); err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if _, err := s.Create(pubPost("batch-published", "已发布文章")); err != nil {
		t.Fatalf("create published: %v", err)
	}
	draftBefore := mustGetBatchPost(t, s, "batch-draft")
	publishedBefore := mustGetBatchPost(t, s, "batch-published")

	// 方向一：draft → published。真实变更 1 篇，已发布文章 skipped。
	succeeded, skipped, failed, err := s.SetPostsStatusBySlug([]string{"batch-draft", "batch-published"}, StatusPublished)
	if err != nil || succeeded != 1 || skipped != 1 || failed != 0 {
		t.Fatalf("publish batch = %d/%d/%d err=%v, want 1/1/0", succeeded, skipped, failed, err)
	}
	draftAfter := mustGetBatchPost(t, s, "batch-draft")
	if draftAfter.Status != StatusPublished {
		t.Fatalf("published status = %q, want published", draftAfter.Status)
	}
	if !draftAfter.UpdatedAt.After(draftBefore.UpdatedAt) {
		t.Fatalf("changed post updated_at should advance: before=%v after=%v", draftBefore.UpdatedAt, draftAfter.UpdatedAt)
	}
	// 只改 status/updated_at：正文、渲染缓存、草稿头、定时发布与作者/分类/置顶/口令保持原值。
	if draftAfter.ContentMD != draftBefore.ContentMD || draftAfter.ContentHTML != draftBefore.ContentHTML {
		t.Fatalf("batch status changed content: before=%+v after=%+v", draftBefore, draftAfter)
	}
	if draftAfter.DraftJSON != draftBefore.DraftJSON {
		t.Fatalf("batch status must not touch draft_json: before=%q after=%q", draftBefore.DraftJSON, draftAfter.DraftJSON)
	}
	if !draftAfter.PublishAt.Equal(draftBefore.PublishAt) {
		t.Fatalf("batch status changed publish_at: before=%v after=%v", draftBefore.PublishAt, draftAfter.PublishAt)
	}
	if !draftAfter.Pinned || draftAfter.Password != "secret" {
		t.Fatalf("batch status changed pinned/password: %+v", draftAfter)
	}
	if draftAfter.Author != "alice" || strings.Join(draftAfter.Authors, ",") != "alice" {
		t.Fatalf("batch status changed authors: author=%q authors=%#v", draftAfter.Author, draftAfter.Authors)
	}
	if strings.Join(draftAfter.Categories, ",") != "技术/Go" {
		t.Fatalf("batch status changed categories: %#v", draftAfter.Categories)
	}

	// 已是目标状态的文章既不写库也不刷新 updated_at。
	publishedAfter := mustGetBatchPost(t, s, "batch-published")
	if !publishedAfter.UpdatedAt.Equal(publishedBefore.UpdatedAt) {
		t.Fatalf("skipped post updated_at changed: before=%v after=%v", publishedBefore.UpdatedAt, publishedAfter.UpdatedAt)
	}
	if publishedAfter.Status != StatusPublished {
		t.Fatalf("skipped post status = %q, want published", publishedAfter.Status)
	}

	// 方向二：published → draft，两篇都真实变更。
	succeeded, skipped, failed, err = s.SetPostsStatusBySlug([]string{"batch-draft", "batch-published"}, StatusDraft)
	if err != nil || succeeded != 2 || skipped != 0 || failed != 0 {
		t.Fatalf("draft batch = %d/%d/%d err=%v, want 2/0/0", succeeded, skipped, failed, err)
	}
	for _, slug := range []string{"batch-draft", "batch-published"} {
		if got := mustGetBatchPost(t, s, slug); got.Status != StatusDraft {
			t.Fatalf("%s status = %q, want draft", slug, got.Status)
		}
	}

	// 再次设为草稿：全部 skipped，且两篇 updated_at 都不变。
	firstBefore := mustGetBatchPost(t, s, "batch-draft").UpdatedAt
	secondBefore := mustGetBatchPost(t, s, "batch-published").UpdatedAt
	succeeded, skipped, failed, err = s.SetPostsStatusBySlug([]string{"batch-draft", "batch-published"}, StatusDraft)
	if err != nil || succeeded != 0 || skipped != 2 || failed != 0 {
		t.Fatalf("second draft batch = %d/%d/%d err=%v, want 0/2/0", succeeded, skipped, failed, err)
	}
	if !mustGetBatchPost(t, s, "batch-draft").UpdatedAt.Equal(firstBefore) ||
		!mustGetBatchPost(t, s, "batch-published").UpdatedAt.Equal(secondBefore) {
		t.Fatal("skipped-only batch refreshed updated_at")
	}
}

// TestBatchStatusTrashedAndMissingCountAsFailed 回收站与不存在的 slug 计入 failed，
// 且回收站文章的状态与删除标记不被改动。
func TestBatchStatusTrashedAndMissingCountAsFailed(t *testing.T) {
	s := newTestStore(t)
	trashed := pubPost("batch-trashed", "已在回收站")
	trashed.Status = StatusDraft
	if _, err := s.Create(trashed); err != nil {
		t.Fatalf("create trashed: %v", err)
	}
	if err := s.Delete("batch-trashed"); err != nil {
		t.Fatalf("move to trash: %v", err)
	}
	live := pubPost("batch-live", "正常文章")
	live.Status = StatusDraft
	if _, err := s.Create(live); err != nil {
		t.Fatalf("create live: %v", err)
	}

	succeeded, skipped, failed, err := s.SetPostsStatusBySlug(
		[]string{"batch-live", "batch-trashed", "batch-missing"}, StatusPublished)
	if err != nil || succeeded != 1 || skipped != 0 || failed != 2 {
		t.Fatalf("mixed batch = %d/%d/%d err=%v, want 1/0/2", succeeded, skipped, failed, err)
	}
	got := mustGetBatchPost(t, s, "batch-trashed")
	if got.Status != StatusDraft {
		t.Fatalf("trashed post status changed to %q, want draft", got.Status)
	}
	if got.DeletedAt == "" {
		t.Fatal("trashed post deleted_at cleared by batch status")
	}
	if missing, err := s.GetBySlug("batch-missing"); err != nil || missing != nil {
		t.Fatalf("missing slug should stay absent: post=%+v err=%v", missing, err)
	}
}

// TestBatchStatusInputValidation 空列表安全返回零计数；非法目标状态返回错误。
func TestBatchStatusInputValidation(t *testing.T) {
	s := newTestStore(t)
	if succeeded, skipped, failed, err := s.SetPostsStatusBySlug(nil, StatusPublished); err != nil || succeeded != 0 || skipped != 0 || failed != 0 {
		t.Fatalf("empty list = %d/%d/%d err=%v, want 0/0/0 nil", succeeded, skipped, failed, err)
	}
	for _, status := range []string{"", "trashed", "Published", "publish"} {
		if _, _, _, err := s.SetPostsStatusBySlug([]string{"any"}, status); err == nil {
			t.Fatalf("status %q should be rejected", status)
		}
	}
}

// TestBatchStatusWriteEventPerChangedSlug 只有真实变更的 slug 触发写路径事件。
func TestBatchStatusWriteEventPerChangedSlug(t *testing.T) {
	s := newTestStore(t)
	for _, slug := range []string{"batch-one", "batch-two"} {
		if _, err := s.Create(&Post{Slug: slug, Title: slug, Status: StatusDraft}); err != nil {
			t.Fatalf("create %s: %v", slug, err)
		}
	}
	if _, err := s.Create(pubPost("batch-already", "已是发布态")); err != nil {
		t.Fatalf("create already: %v", err)
	}

	var events []content.Event
	bus := content.NewEvents()
	bus.Subscribe(func(event content.Event) { events = append(events, event) })
	s.SetContentEvents(bus)

	succeeded, skipped, failed, err := s.SetPostsStatusBySlug(
		[]string{"batch-one", "batch-two", "batch-already", "batch-missing"}, StatusPublished)
	if err != nil || succeeded != 2 || skipped != 1 || failed != 1 {
		t.Fatalf("batch = %d/%d/%d err=%v, want 2/1/1", succeeded, skipped, failed, err)
	}
	if len(events) != 2 {
		t.Fatalf("write events = %d, want 2: %+v", len(events), events)
	}
	changed := map[string]bool{}
	for _, event := range events {
		if event.Kind != content.PostChanged || event.Op != "update" {
			t.Fatalf("unexpected event: %+v", event)
		}
		changed[event.Slug] = true
	}
	if !changed["batch-one"] || !changed["batch-two"] || changed["batch-already"] || changed["batch-missing"] {
		t.Fatalf("event slugs = %#v, want only batch-one/batch-two", changed)
	}
}

// TestBatchStatusRollbackOnUpdateFailure 事务内更新失败时整单回滚：
// 状态保持不变，也不触发任何写路径事件。
func TestBatchStatusRollbackOnUpdateFailure(t *testing.T) {
	s := newTestStore(t)
	for _, slug := range []string{"batch-rollback-a", "batch-rollback-b"} {
		if _, err := s.Create(pubPost(slug, slug)); err != nil {
			t.Fatalf("create %s: %v", slug, err)
		}
	}
	var events int
	bus := content.NewEvents()
	bus.Subscribe(func(content.Event) { events++ })
	s.SetContentEvents(bus)

	if _, err := s.DB().Exec(`CREATE TRIGGER fail_post_status_update BEFORE UPDATE OF status ON posts
		BEGIN SELECT RAISE(ABORT, 'forced status failure'); END`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}
	if _, _, _, err := s.SetPostsStatusBySlug([]string{"batch-rollback-a", "batch-rollback-b"}, StatusDraft); err == nil {
		t.Fatal("forced update failure should surface as error")
	}
	for _, slug := range []string{"batch-rollback-a", "batch-rollback-b"} {
		if got := mustGetBatchPost(t, s, slug); got.Status != StatusPublished {
			t.Fatalf("%s status = %q after rollback, want published", slug, got.Status)
		}
	}
	if events != 0 {
		t.Fatalf("failed batch emitted %d write events, want 0", events)
	}
}
