package routes

import (
	"net/http"
	"testing"

	"blog/internal/store"
)

// TestStellarAISummaryFallsBackToLegacyColumn：AI 摘要只在 posts.ai_summary 列
// （Spec 53 旧写入路径）时，公开快照仍需渲染；canonical
// blog_extensions.content.ai_summary 存在时优先使用扩展值。
func TestStellarAISummaryFallsBackToLegacyColumn(t *testing.T) {
	ws, st, capture, r := stellarRouteFixture(t)
	ws.site.Load().StellarV2.Blog["ai"] = map[string]any{"summary": map[string]any{"enabled": true}}

	row := &store.Post{
		Slug: "legacy-summary", Title: "旧列摘要", Status: store.StatusPublished,
		ContentMD: "## 正文\n\n摘要回退测试", StellarVersion: 2,
		StellarConfig: `{"title":"旧列摘要"}`, BlogExtensions: "{}",
		AISummary: "列内摘要",
	}
	if _, err := st.Create(row); err != nil {
		t.Fatal(err)
	}
	if w := stellarRequest(r, "/post/legacy-summary"); w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	if capture.last.Page.AISummary != "列内摘要" {
		t.Fatalf("Page.AISummary = %q, want 列内摘要 (legacy column fallback)", capture.last.Page.AISummary)
	}

	// canonical 扩展存在时优先，不被同名的旧列覆盖。
	row.BlogExtensions = `{"content":{"ai_summary":"canonical 摘要"}}`
	if err := st.Update(row.Slug, row); err != nil {
		t.Fatal(err)
	}
	if w := stellarRequest(r, "/post/legacy-summary"); w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	if capture.last.Page.AISummary != "canonical 摘要" {
		t.Fatalf("Page.AISummary = %q, want canonical 摘要", capture.last.Page.AISummary)
	}
}
