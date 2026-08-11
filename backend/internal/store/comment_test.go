package store

import "testing"

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
