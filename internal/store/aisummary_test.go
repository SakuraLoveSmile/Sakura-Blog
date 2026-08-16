package store

import "testing"

// TestPostAISummary Spec 53：ai_summary 列迁移幂等 + 读写往返 + Update 保留（COALESCE 不误清）+ SetAISummary 更新/清空/不存在报错。
func TestPostAISummary(t *testing.T) {
	s := newTestStore(t)

	// 迁移幂等：newTestStore 已迁移过一次，再次调用不应报错（ai_summary 列已存在）
	if err := s.migrateColumns(); err != nil {
		t.Fatalf("migrate idempotent: %v", err)
	}

	p := pubPost("ai-post", "AI 摘要测试")
	p.AISummary = "初始摘要"
	if _, err := s.Create(p); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := s.GetBySlug("ai-post")
	if err != nil || got == nil {
		t.Fatalf("get: %v, nil=%v", err, got == nil)
	}
	if got.AISummary != "初始摘要" {
		t.Fatalf("ai_summary = %q, want 初始摘要", got.AISummary)
	}

	// 常规 Update 不携带 ai_summary 时应保留原值（seed/Upsert 写路径不误清）
	if err := s.Update("ai-post", &Post{Title: "AI 摘要测试", Status: StatusPublished}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = s.GetBySlug("ai-post")
	if got.AISummary != "初始摘要" {
		t.Fatalf("update should preserve ai_summary, got %q", got.AISummary)
	}

	// SetAISummary 覆盖写入
	if err := s.SetAISummary("ai-post", "新摘要"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, _ = s.GetBySlug("ai-post")
	if got.AISummary != "新摘要" {
		t.Fatalf("ai_summary = %q, want 新摘要", got.AISummary)
	}

	// 传空串即清空
	if err := s.SetAISummary("ai-post", ""); err != nil {
		t.Fatalf("clear: %v", err)
	}
	got, _ = s.GetBySlug("ai-post")
	if got.AISummary != "" {
		t.Fatalf("ai_summary = %q, want empty", got.AISummary)
	}

	// 文章不存在报错
	if err := s.SetAISummary("ghost", "x"); err == nil {
		t.Fatal("expected error for non-existent slug")
	}
}
