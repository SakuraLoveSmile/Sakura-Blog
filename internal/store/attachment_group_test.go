package store

import (
	"path/filepath"
	"testing"
)

// newAttachStore 建库并确保 attachments + attachment_groups 表（Spec 44）。
func newAttachStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := s.EnsureAttachmentsTable(); err != nil {
		t.Fatalf("ensure attachments table: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// mkAttach 便捷插入一条附件（可指定分组），返回 id。
func mkAttach(t *testing.T, s *Store, filename, mime string, groupID int64) int64 {
	t.Helper()
	id, err := s.CreateAttachment(&Attachment{
		Filename: filename,
		Path:     "2026/07/" + filename,
		URL:      "/uploads/2026/07/" + filename,
		Mime:     mime,
		Size:     100,
		GroupID:  groupID,
	})
	if err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	return id
}

// TestEnsureAttachmentsTableIdempotent 重复建表 + 迁移应幂等（旧库不报错）。
func TestEnsureAttachmentsTableIdempotent(t *testing.T) {
	s := newAttachStore(t)
	for i := 0; i < 3; i++ {
		if err := s.EnsureAttachmentsTable(); err != nil {
			t.Fatalf("re-ensure #%d: %v", i, err)
		}
	}
	// group_id 列可用：插入带分组的附件不报错
	mkAttach(t, s, "a.png", "image/png", UngroupedGroupID)
}

func TestAttachmentsCanBeListedByOwner(t *testing.T) {
	s := newAttachStore(t)
	mk := func(ownerID int64, name string) {
		t.Helper()
		if _, err := s.CreateAttachment(&Attachment{
			Filename: name, Path: "2026/08/" + name, URL: "/uploads/2026/08/" + name,
			Mime: "image/png", Size: 10, OwnerID: ownerID,
		}); err != nil {
			t.Fatalf("create attachment %s: %v", name, err)
		}
	}
	mk(11, "mine.png")
	mk(22, "other.png")
	items, err := s.ListAttachmentsByOwner(11, 1, 20, "", "image", nil)
	if err != nil || len(items) != 1 || items[0].Filename != "mine.png" {
		t.Fatalf("owner attachments = %+v, err=%v", items, err)
	}
	count, err := s.CountAttachmentsByOwner(22, "", "image", nil)
	if err != nil || count != 1 {
		t.Fatalf("owner count = %d, err=%v", count, err)
	}
}

func TestAttachmentGroupCRUD(t *testing.T) {
	s := newAttachStore(t)

	id, err := s.CreateAttachmentGroup(&AttachmentGroup{Name: "封面", Order: 1})
	if err != nil || id == 0 {
		t.Fatalf("create group: id=%d err=%v", id, err)
	}

	groups, err := s.ListAttachmentGroups()
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	if len(groups) != 1 || groups[0].Name != "封面" || groups[0].Order != 1 {
		t.Fatalf("unexpected groups: %+v", groups)
	}
	if groups[0].Count != 0 {
		t.Fatalf("new group count = %d, want 0", groups[0].Count)
	}

	// 重命名 + 调排序
	if err := s.UpdateAttachmentGroup(id, "素材", 5); err != nil {
		t.Fatalf("update group: %v", err)
	}
	groups, _ = s.ListAttachmentGroups()
	if groups[0].Name != "素材" || groups[0].Order != 5 {
		t.Fatalf("update not applied: %+v", groups[0])
	}

	// 更新不存在的组应报错
	if err := s.UpdateAttachmentGroup(99999, "x", 0); err == nil {
		t.Error("update nonexistent group should error")
	}
}

// TestDeleteGroupResetsAttachments 删组后组内附件归零（group_id=0），文件记录仍在。
func TestDeleteGroupResetsAttachments(t *testing.T) {
	s := newAttachStore(t)
	gid, _ := s.CreateAttachmentGroup(&AttachmentGroup{Name: "组A"})
	a := mkAttach(t, s, "a.png", "image/png", gid)
	b := mkAttach(t, s, "b.png", "image/png", gid)
	other := mkAttach(t, s, "c.png", "image/png", UngroupedGroupID)

	if err := s.DeleteAttachmentGroup(gid); err != nil {
		t.Fatalf("delete group: %v", err)
	}

	// 组删除
	if ok, _ := s.AttachmentGroupExists(gid); ok {
		t.Error("group should be deleted")
	}
	// 组内附件归未分组，记录仍在
	for _, id := range []int64{a, b} {
		att, _ := s.GetAttachment(id)
		if att == nil {
			t.Fatalf("attachment %d should survive group deletion", id)
		}
		if att.GroupID != UngroupedGroupID {
			t.Errorf("attachment %d group_id = %d, want 0", id, att.GroupID)
		}
	}
	// 未分组附件不受影响
	if att, _ := s.GetAttachment(other); att == nil || att.GroupID != UngroupedGroupID {
		t.Error("ungrouped attachment should be unaffected")
	}

	// 删除不存在的组应报错
	if err := s.DeleteAttachmentGroup(88888); err == nil {
		t.Error("delete nonexistent group should error")
	}
}

// TestMoveAttachmentsIdempotent 移动幂等：重复移动到同一组结果一致。
func TestMoveAttachmentsIdempotent(t *testing.T) {
	s := newAttachStore(t)
	g1, _ := s.CreateAttachmentGroup(&AttachmentGroup{Name: "G1"})
	g2, _ := s.CreateAttachmentGroup(&AttachmentGroup{Name: "G2"})
	a := mkAttach(t, s, "a.png", "image/png", UngroupedGroupID)
	b := mkAttach(t, s, "b.png", "image/png", UngroupedGroupID)

	// 首次移动
	n, err := s.MoveAttachments([]int64{a, b}, g1)
	if err != nil || n != 2 {
		t.Fatalf("first move: n=%d err=%v, want 2", n, err)
	}
	// 重复移动到同一组：仍匹配 2 条，结果不变（幂等）
	n, err = s.MoveAttachments([]int64{a, b}, g1)
	if err != nil || n != 2 {
		t.Fatalf("repeat move: n=%d err=%v, want 2", n, err)
	}
	for _, id := range []int64{a, b} {
		if att, _ := s.GetAttachment(id); att == nil || att.GroupID != g1 {
			t.Errorf("attachment %d not in g1: %+v", id, att)
		}
	}

	// 移动到另一组
	if _, err := s.MoveAttachments([]int64{a}, g2); err != nil {
		t.Fatalf("move to g2: %v", err)
	}
	if att, _ := s.GetAttachment(a); att.GroupID != g2 {
		t.Errorf("a should be in g2, got %d", att.GroupID)
	}

	// 移出到未分组
	if _, err := s.MoveAttachments([]int64{a}, UngroupedGroupID); err != nil {
		t.Fatalf("move to ungrouped: %v", err)
	}
	if att, _ := s.GetAttachment(a); att.GroupID != UngroupedGroupID {
		t.Errorf("a should be ungrouped, got %d", att.GroupID)
	}
}

// TestMoveAttachmentsEmpty 空 ids 返回 0，无副作用。
func TestMoveAttachmentsEmpty(t *testing.T) {
	s := newAttachStore(t)
	n, err := s.MoveAttachments(nil, 1)
	if err != nil || n != 0 {
		t.Fatalf("nil ids: n=%d err=%v, want 0 nil", n, err)
	}
	n, err = s.MoveAttachments([]int64{}, 1)
	if err != nil || n != 0 {
		t.Fatalf("empty ids: n=%d err=%v, want 0 nil", n, err)
	}
}

// TestListAttachmentsGroupFilter 三重筛选：分组 + 类型 + 关键词叠加。
func TestListAttachmentsGroupFilter(t *testing.T) {
	s := newAttachStore(t)
	g1, _ := s.CreateAttachmentGroup(&AttachmentGroup{Name: "G1"})

	mkAttach(t, s, "cover.png", "image/png", g1)     // g1 图片
	mkAttach(t, s, "doc.pdf", "application/pdf", g1) // g1 非图片
	mkAttach(t, s, "loose.png", "image/png", UngroupedGroupID)

	// group=g1 全部 → 2 条
	if got := countList(t, s, "", "", &g1); got != 2 {
		t.Errorf("group filter: got %d, want 2", got)
	}
	// group=g1 + 仅图片 → 1 条
	if got := countList(t, s, "", "image", &g1); got != 1 {
		t.Errorf("group+image: got %d, want 1", got)
	}
	// group=g1 + 图片 + 关键词 cover → 1 条
	if got := countList(t, s, "cover", "image", &g1); got != 1 {
		t.Errorf("triple filter: got %d, want 1", got)
	}
	// group=g1 + 关键词 loose → 0 条（loose 在未分组）
	if got := countList(t, s, "loose", "", &g1); got != 0 {
		t.Errorf("cross-group keyword: got %d, want 0", got)
	}
	// 未分组 → 1 条
	ung := UngroupedGroupID
	if got := countList(t, s, "", "", &ung); got != 1 {
		t.Errorf("ungrouped filter: got %d, want 1", got)
	}
	// nil（全部）→ 3 条
	if got := countList(t, s, "", "", nil); got != 3 {
		t.Errorf("no group filter: got %d, want 3", got)
	}
}

// countList 校验 List 与 Count 一致并返回条数。
func countList(t *testing.T, s *Store, keyword, kind string, groupID *int64) int {
	t.Helper()
	items, err := s.ListAttachments(1, 100, keyword, kind, groupID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	cnt, err := s.CountAttachments(keyword, kind, groupID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if cnt != len(items) {
		t.Errorf("count(%d) != len(list)(%d) for kw=%q kind=%q", cnt, len(items), keyword, kind)
	}
	return len(items)
}

// TestListAttachmentGroupsCount 列表回填组内附件数。
func TestListAttachmentGroupsCount(t *testing.T) {
	s := newAttachStore(t)
	g1, _ := s.CreateAttachmentGroup(&AttachmentGroup{Name: "G1", Order: 2})
	g2, _ := s.CreateAttachmentGroup(&AttachmentGroup{Name: "G2", Order: 1})
	mkAttach(t, s, "a.png", "image/png", g1)
	mkAttach(t, s, "b.png", "image/png", g1)
	mkAttach(t, s, "c.png", "image/png", g2)

	groups, err := s.ListAttachmentGroups()
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	// 按 order 升序：G2(order1) 在前
	if len(groups) != 2 || groups[0].ID != g2 {
		t.Fatalf("order not applied: %+v", groups)
	}
	counts := map[int64]int{}
	for _, g := range groups {
		counts[g.ID] = g.Count
	}
	if counts[g1] != 2 || counts[g2] != 1 {
		t.Errorf("counts g1=%d g2=%d, want 2/1", counts[g1], counts[g2])
	}
}
