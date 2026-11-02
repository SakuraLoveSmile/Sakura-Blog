package web

import (
	"strings"
	"testing"

	"blog/internal/stellarmodels"
)

// Spec 53：AI 摘要是文章内容导读，只在文章/文档内容页渲染；
// 独立页面（layout=page，如 /friends、/about）不出现摘要块与读者自助面板。
func TestStellarBlogAISummaryOnlyOnArticleContent(t *testing.T) {
	r := stellarTemplateRenderer{}
	enableSummary := func(d *StellarRenderData, enabled, allowUserAPI bool) {
		d.Stellar.Data["blog"] = map[string]any{"ai": map[string]any{"summary": map[string]any{"enabled": enabled, "allow_user_api": allowUserAPI}}}
	}
	articleModel := func(profile string) *stellarmodels.PageModel {
		return &stellarmodels.PageModel{
			Collection: map[string]any{"profile": profile},
			Item:       map[string]any{"title": "Article", "content": "<p>Article body</p>", "presentation": map[string]any{}},
			Render:     map[string]any{"article": map[string]any{"tags": []any{}, "footer": map[string]any{}}},
		}
	}

	// 独立页面：开关开启、存在已存摘要，也不渲染摘要块与自助面板。
	page := stellarTemplateFixture(t)
	page.Page.AISummary = "Saved summary"
	enableSummary(page, true, true)
	if got := string(r.blogSummary(page)); got != "" {
		t.Fatalf("standalone page rendered AI summary: %s", got)
	}
	if source := renderBlogFixture(t, page); strings.Contains(source, `id="ai-summary"`) || strings.Contains(source, "Saved summary") || strings.Contains(source, "data-user-api") {
		t.Fatal("standalone page rendered AI summary block")
	}

	// 文章页：保留已存摘要与读者自助面板。
	article := stellarTemplateFixture(t)
	article.Stellar.Layout, article.Stellar.Profile = "post", "post"
	article.Stellar.Model = articleModel("post")
	article.Page.AISummary = "Saved summary"
	enableSummary(article, true, true)
	summary := string(r.blogSummary(article))
	for _, want := range []string{`id="ai-summary"`, `data-user-api="1"`, "Saved summary", Lang("ai.summary.title"), Lang("ai.summary.open")} {
		if !strings.Contains(summary, want) {
			t.Fatalf("article summary missing %s", want)
		}
	}
	if source := renderBlogFixture(t, article); !strings.Contains(source, `id="ai-summary"`) || !strings.Contains(source, "Saved summary") {
		t.Fatal("article page lost AI summary")
	}

	// Wiki 文档与笔记文章沿用文章内容页语义，保留摘要块。
	for _, layout := range []string{"wiki-doc", "note"} {
		article.Stellar.Layout = layout
		if !strings.Contains(string(r.blogSummary(article)), `id="ai-summary"`) {
			t.Fatalf("%s lost AI summary", layout)
		}
	}

	// 关闭 AI 摘要开关时不渲染（原有行为）。
	article.Stellar.Layout = "post"
	enableSummary(article, false, true)
	if got := string(r.blogSummary(article)); got != "" {
		t.Fatalf("disabled AI summary rendered: %s", got)
	}
}
