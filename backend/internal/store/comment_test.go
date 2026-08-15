package store

import (
	"errors"
	"testing"
)

// mkComment 便捷插入一条评论，返回 id。
func mkComment(t *testing.T, s *Store, pageKey, status string, rootID int64) int64 {
	t.Helper()
	id, err := s.CreateComment(&Comment{
		PageKey:   pageKey,
		Nick:      "tester",
		Email:     "t@example.com",
		ContentMD: "hi",
		Status:    status,
		RootID:    rootID,
	})
	if err != nil {
		t.Fatalf("create comment: %v", err)
	}
	return id
}

func TestApproveCommentsEmptyIDs(t *testing.T) {
	s := newTestStore(t)
	n, err := s.ApproveComments(nil)
	if err != nil || n != 0 {
		t.Fatalf("empty ids: n=%d err=%v, want 0 nil", n, err)
	}
	n, err = s.ApproveComments([]int64{})
	if err != nil || n != 0 {
		t.Fatalf("zero-length ids: n=%d err=%v, want 0 nil", n, err)
	}
}

func TestSetCommentsStatusBatch(t *testing.T) {
	s := newTestStore(t)
	a := mkComment(t, s, "/post/a", CommentPending, 0)
	b := mkComment(t, s, "/post/a", CommentApproved, 0)

	// 批量标垃圾（混入不存在 id，只计命中数）
	n, err := s.SetCommentsStatus([]int64{a, b, 99999}, CommentSpam)
	if err != nil || n != 2 {
		t.Fatalf("set spam: n=%d err=%v, want 2 nil", n, err)
	}
	for _, id := range []int64{a, b} {
		cm, _ := s.GetComment(id)
		if cm == nil || cm.Status != CommentSpam {
			t.Fatalf("comment %d not spam: %+v", id, cm)
		}
	}
	// 批量拒绝
	if n, err = s.SetCommentsStatus([]int64{a}, CommentRejected); err != nil || n != 1 {
		t.Fatalf("set rejected: n=%d err=%v, want 1 nil", n, err)
	}
	cm, _ := s.GetComment(a)
	if cm == nil || cm.Status != CommentRejected {
		t.Fatalf("comment %d not rejected: %+v", a, cm)
	}
	// 空 ids 直接返回 0
	if n, err = s.SetCommentsStatus(nil, CommentSpam); err != nil || n != 0 {
		t.Fatalf("empty ids: n=%d err=%v, want 0 nil", n, err)
	}
}

func TestCountCommentStatuses(t *testing.T) {
	s := newTestStore(t)
	mkComment(t, s, "/post/a", CommentPending, 0)
	mkComment(t, s, "/post/a", CommentApproved, 0)
	mkComment(t, s, "/post/b", CommentApproved, 0)
	mkComment(t, s, "/post/b", CommentSpam, 0)

	counts, err := s.CountCommentStatuses()
	if err != nil {
		t.Fatalf("count statuses: %v", err)
	}
	want := map[string]int{"all": 4, CommentPending: 1, CommentApproved: 2, CommentSpam: 1}
	for k, v := range want {
		if counts[k] != v {
			t.Errorf("counts[%q] = %d, want %d (counts=%+v)", k, counts[k], v, counts)
		}
	}
	if _, ok := counts[CommentRejected]; ok {
		t.Errorf("rejected should be absent when no rows, counts=%+v", counts)
	}
}

func TestApproveCommentsBatch(t *testing.T) {
	s := newTestStore(t)
	a := mkComment(t, s, "/post/a", CommentPending, 0)
	b := mkComment(t, s, "/post/a", CommentPending, 0)
	c := mkComment(t, s, "/post/b", CommentPending, 0)

	n, err := s.ApproveComments([]int64{a, b})
	if err != nil {
		t.Fatalf("approve batch: %v", err)
	}
	if n != 2 {
		t.Fatalf("succeeded = %d, want 2", n)
	}
	for _, id := range []int64{a, b} {
		cm, _ := s.GetComment(id)
		if cm == nil || cm.Status != CommentApproved {
			t.Fatalf("comment %d not approved: %+v", id, cm)
		}
	}
	// 未选中的 c 不受影响
	cm, _ := s.GetComment(c)
	if cm == nil || cm.Status != CommentPending {
		t.Fatalf("comment %d should stay pending: %+v", c, cm)
	}
}

func TestApproveCommentsPartialMissing(t *testing.T) {
	s := newTestStore(t)
	a := mkComment(t, s, "/post/a", CommentPending, 0)

	// 混入不存在的 id：存在的照常成功，返回值只计成功数
	n, err := s.ApproveComments([]int64{a, 99999, 88888})
	if err != nil {
		t.Fatalf("approve with missing ids: %v", err)
	}
	if n != 1 {
		t.Fatalf("succeeded = %d, want 1", n)
	}
	cm, _ := s.GetComment(a)
	if cm.Status != CommentApproved {
		t.Fatalf("comment %d not approved: %+v", a, cm)
	}
}

func TestDeleteCommentsEmptyIDs(t *testing.T) {
	s := newTestStore(t)
	n, err := s.DeleteComments(nil)
	if err != nil || n != 0 {
		t.Fatalf("empty ids: n=%d err=%v, want 0 nil", n, err)
	}
}

func TestDeleteCommentsBatchCascade(t *testing.T) {
	s := newTestStore(t)
	root := mkComment(t, s, "/post/a", CommentApproved, 0)
	child := mkComment(t, s, "/post/a", CommentApproved, root) // root 的子评论，应级联删除
	other := mkComment(t, s, "/post/b", CommentPending, 0)

	n, err := s.DeleteComments([]int64{root})
	if err != nil {
		t.Fatalf("delete batch: %v", err)
	}
	// 报数只计请求 ids 的命中数，级联的子评论不计入
	if n != 1 {
		t.Fatalf("succeeded = %d, want 1", n)
	}
	for _, id := range []int64{root, child} {
		cm, _ := s.GetComment(id)
		if cm != nil {
			t.Fatalf("comment %d should be deleted: %+v", id, cm)
		}
	}
	cm, _ := s.GetComment(other)
	if cm == nil {
		t.Fatalf("comment %d should survive", other)
	}
}

func TestDeleteCommentsPartialMissing(t *testing.T) {
	s := newTestStore(t)
	a := mkComment(t, s, "/post/a", CommentApproved, 0)
	b := mkComment(t, s, "/post/a", CommentPending, 0)

	n, err := s.DeleteComments([]int64{a, 77777})
	if err != nil {
		t.Fatalf("delete with missing ids: %v", err)
	}
	if n != 1 {
		t.Fatalf("succeeded = %d, want 1", n)
	}
	if cm, _ := s.GetComment(a); cm != nil {
		t.Fatalf("comment %d should be deleted", a)
	}
	if cm, _ := s.GetComment(b); cm == nil {
		t.Fatalf("comment %d should survive", b)
	}
}

func TestCommentBoostCreateUpdateDeleteAndList(t *testing.T) {
	s := newTestStore(t)
	id, err := s.CreateComment(&Comment{
		PageKey: "/post/boost", Nick: "author", Email: "author@example.com",
		ContentMD: "hello", Status: CommentApproved, IP: "203.0.113.1",
	})
	if err != nil {
		t.Fatalf("create comment: %v", err)
	}
	meta := BoostMeta{Nick: "reader", Email: "reader@example.com", IP: "203.0.113.2", IPHash: "hash-1"}
	first, created, err := s.AddBoost(id, "ip:hash-1", "👍", "赞", meta)
	if err != nil || !created || first.Text != "赞" {
		t.Fatalf("first boost: item=%+v created=%v err=%v", first, created, err)
	}
	second, created, err := s.AddBoost(id, "ip:hash-1", "🎉", "更新", meta)
	if err != nil || created || second.ID != first.ID || second.Emoji != "🎉" || second.Text != "更新" {
		t.Fatalf("updated boost: item=%+v created=%v err=%v", second, created, err)
	}
	if ok, err := s.HasBoost(id, "ip:hash-1"); err != nil || !ok {
		t.Fatalf("has boost: ok=%v err=%v", ok, err)
	}
	if n, err := s.CountBoostsByComment(id); err != nil || n != 1 {
		t.Fatalf("count boosts: n=%d err=%v", n, err)
	}
	page, err := s.ListBoostsByPage("/post/boost")
	if err != nil || len(page[id]) != 1 || page[id][0].Emoji != "🎉" {
		t.Fatalf("list boosts: page=%+v err=%v", page, err)
	}
	removed, err := s.DeleteBoost(id, "ip:hash-1")
	if err != nil || !removed {
		t.Fatalf("delete boost: removed=%v err=%v", removed, err)
	}
	if removed, err = s.DeleteBoost(id, "ip:hash-1"); err != nil || removed {
		t.Fatalf("delete missing boost: removed=%v err=%v", removed, err)
	}
}

func TestCommentBoostSelfAndLimit(t *testing.T) {
	s := newTestStore(t)
	id, err := s.CreateComment(&Comment{
		PageKey: "/post/boost", Nick: "author", Email: "author@example.com",
		ContentMD: "hello", Status: CommentApproved, UserID: 42, IP: "203.0.113.3",
	})
	if err != nil {
		t.Fatalf("create comment: %v", err)
	}
	if _, _, err := s.AddBoost(id, "u:42", "👍", "", BoostMeta{UserID: 42, IP: "203.0.113.4"}); !errors.Is(err, ErrBoostSelf) {
		t.Fatalf("user self boost err=%v, want ErrBoostSelf", err)
	}
	if _, _, err := s.AddBoost(id, "ip:author", "👍", "", BoostMeta{IP: "203.0.113.3"}); !errors.Is(err, ErrBoostSelf) {
		t.Fatalf("ip self boost err=%v, want ErrBoostSelf", err)
	}
	for i := 0; i < 50; i++ {
		identity := "ip:limit-" + string(rune('a'+i))
		_, _, err := s.AddBoost(id, identity, "👍", "", BoostMeta{IP: "198.51.100." + string(rune('1'+i))})
		if err != nil {
			t.Fatalf("fill boost %d: %v", i, err)
		}
	}
	if n, err := s.CountBoostsByComment(id); err != nil || n != 50 {
		t.Fatalf("full count: n=%d err=%v", n, err)
	}
	if _, _, err := s.AddBoost(id, "ip:overflow", "👍", "", BoostMeta{IP: "198.51.100.200"}); !errors.Is(err, ErrBoostLimit) {
		t.Fatalf("overflow err=%v, want ErrBoostLimit", err)
	}
	// 已有身份在满额后仍允许更新。
	if _, _, err := s.AddBoost(id, "ip:limit-a", "🚀", "edit", BoostMeta{IP: "198.51.100.1"}); err != nil {
		t.Fatalf("update at limit: %v", err)
	}
}
