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

// TestPostAISummaryProjectsToStellarExtensions：Stellar 2 的公开快照只读
// blog_extensions.content.ai_summary，SetAISummary 必须同步投影且保留其它扩展字段；
// 清空摘要时列与扩展同时移除。
func TestPostAISummaryProjectsToStellarExtensions(t *testing.T) {
	s := newTestStore(t)
	if err := s.EnsureSettingsTable(); err != nil {
		t.Fatalf("ensure settings: %v", err)
	}
	p := &Post{
		Slug: "canonical-ai", Title: "AI 摘要投影", Status: StatusPublished, ContentMD: "# 正文",
		StellarVersion: 2,
		StellarConfig:  `{"title":"AI 摘要投影"}`,
		BlogExtensions: `{"status":"draft","content":{"banner_image":"/banner.jpg"},"custom":{"keep":true}}`,
	}
	if _, err := s.Create(p); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.SetAISummary("canonical-ai", "canonical 摘要"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := s.GetBySlug("canonical-ai")
	if err != nil || got == nil {
		t.Fatalf("get: %v, nil=%v", err, got == nil)
	}
	if got.AISummary != "canonical 摘要" {
		t.Fatalf("ai_summary = %q, want canonical 摘要", got.AISummary)
	}
	extension, err := decodeStellarObject(got.BlogExtensions, "blog extensions")
	if err != nil {
		t.Fatalf("decode extensions: %v", err)
	}
	content, _ := extension["content"].(map[string]any)
	if content["ai_summary"] != "canonical 摘要" {
		t.Fatalf("content.ai_summary = %v, want canonical 摘要 (%s)", content["ai_summary"], got.BlogExtensions)
	}
	if content["banner_image"] != "/banner.jpg" || extension["status"] != "draft" {
		t.Fatalf("extension fields lost: %s", got.BlogExtensions)
	}
	if custom, _ := extension["custom"].(map[string]any); custom["keep"] != true {
		t.Fatalf("unknown extension keys lost: %s", got.BlogExtensions)
	}

	// 清空：列与 canonical 扩展同时移除，其它字段保留。
	if err := s.SetAISummary("canonical-ai", ""); err != nil {
		t.Fatalf("clear: %v", err)
	}
	got, _ = s.GetBySlug("canonical-ai")
	if got.AISummary != "" {
		t.Fatalf("ai_summary = %q, want empty", got.AISummary)
	}
	extension, err = decodeStellarObject(got.BlogExtensions, "blog extensions")
	if err != nil {
		t.Fatalf("decode extensions after clear: %v", err)
	}
	content, _ = extension["content"].(map[string]any)
	if _, exists := content["ai_summary"]; exists {
		t.Fatalf("content.ai_summary survived clear: %s", got.BlogExtensions)
	}
	if content["banner_image"] != "/banner.jpg" || extension["status"] != "draft" {
		t.Fatalf("clear dropped unrelated extension fields: %s", got.BlogExtensions)
	}
	if custom, _ := extension["custom"].(map[string]any); custom["keep"] != true {
		t.Fatalf("clear dropped unknown extension keys: %s", got.BlogExtensions)
	}
}

// TestMergeAISummaryExtension 覆盖直接投影的边界：空原文、非法 JSON、
// content 非对象时保留原文；摘要未变化时不制造格式漂移。
func TestMergeAISummaryExtension(t *testing.T) {
	cases := []struct {
		name, raw, summary, want string
		changed                  bool
	}{
		{"empty raw", "", "摘要", `{"content":{"ai_summary":"摘要"}}`, true},
		{"clear on empty", "", "", "", false},
		{"invalid json preserved", "{", "摘要", "{", false},
		{"non-object content preserved", `{"content":"raw"}`, "摘要", `{"content":"raw"}`, false},
		{"same value untouched", `{"content":{"ai_summary":"摘要"}}`, "摘要", `{"content":{"ai_summary":"摘要"}}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := mergeAISummaryExtension(tc.raw, tc.summary)
			if got != tc.want || changed != tc.changed {
				t.Fatalf("mergeAISummaryExtension(%q, %q) = (%q, %v), want (%q, %v)", tc.raw, tc.summary, got, changed, tc.want, tc.changed)
			}
		})
	}
}
