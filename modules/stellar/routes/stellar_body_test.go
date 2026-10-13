package routes

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"blog/internal/content"
	"blog/internal/post"
	"blog/internal/store"
	stellarweb "blog/modules/stellar/web"
)

func TestStellarBodyCallbackSharesPublicWikiAndAbsoluteLinkContext(t *testing.T) {
	ws, st, _, _ := stellarRouteFixture(t)
	for _, project := range []*store.WikiProject{
		{ID: "docs", Visible: true, StellarVersion: 2, StellarConfig: `{"name":"Manual","route":{"path":"/docs"},"navigation":{"tree":["intro"]}}`},
		{ID: "private", Visible: false, StellarVersion: 2, StellarConfig: `{"name":"HIDDEN_PROJECT"}`},
	} {
		if err := st.CreateWikiProject(project); err != nil {
			t.Fatal(err)
		}
	}
	for _, doc := range []*store.WikiDoc{
		{ProjectID: "docs", Slug: "intro", Title: "Public introduction", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{"collection":{"profile":"wiki","id":"docs"}}`},
		{ProjectID: "docs", Slug: "draft", Title: "HIDDEN_DRAFT", Status: store.StatusDraft},
		{ProjectID: "private", Slug: "secret", Title: "HIDDEN_PRIVATE", Status: store.StatusPublished},
	} {
		if err := st.CreateWikidoc(doc); err != nil {
			t.Fatal(err)
		}
	}
	locked := createStellarPost(t, st, "locked", "")
	locked.Password = "pw"
	if err := st.Update(locked.Slug, locked); err != nil {
		t.Fatal(err)
	}
	input := content.StellarBodyInput{Markdown: "{% toc wiki:docs %}\n\n{% link /destination title:Destination %}", Page: map[string]any{}}
	body, err := ws.renderStellarMarkdown(input)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := ws.stellarSnapshotAt(context.Background(), ws.site.Load(), nil, true)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := ws.posts.RenderStellarV2WithContext(input.Markdown, post.StellarRenderContext{Theme: snap.input.Theme, Data: snap.input.RuntimeData, Page: input.Page, Language: stellarLanguage(snap.input, input.Page), BlogExtensions: true})
	if err != nil || body != canonical || !strings.Contains(body, "Public introduction") || !strings.Contains(body, "https://fixture.example/destination") {
		t.Fatalf("context mismatch: %v %s", err, body)
	}
	raw, _ := json.Marshal(snap.input.RuntimeData)
	if strings.Contains(string(raw), "HIDDEN_") || strings.Contains(string(raw), "locked") {
		t.Fatalf("private content leaked: %s", raw)
	}
	// 同一个只读快照同时进入 footer 和 Markdown widget，不再从配置袋补树。
	site := *ws.site.Load()
	version := *site.StellarV2
	version.Theme = stellarCopyObject(version.Theme)
	version.Theme["footer"] = map[string]any{"content": input.Markdown}
	version.Theme["rightbar"] = map[string]any{"enabled": true, "widgets": []any{"context"}}
	version.Blog = stellarCopyObject(version.Blog)
	version.Blog["data"] = map[string]any{"wiki": []any{"docs"}, "secret": "HIDDEN_DATA_SECRET", "widgets": map[string]any{"context": map[string]any{"layout": "markdown", "content": input.Markdown}}}
	site.StellarV2 = &version
	ws.site.Store(&site)
	engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	engine.SetStellarMarkdownRender(func(markdown string, ctx post.StellarRenderContext) (string, error) {
		if !strings.Contains(markdown, "toc wiki:docs") {
			return ws.posts.RenderStellarV2WithContext(markdown, ctx)
		}
		calls++
		rendered, err := ws.posts.RenderStellarV2WithContext(markdown, ctx)
		if err != nil || !strings.Contains(rendered, "Public introduction") || !strings.Contains(rendered, "https://fixture.example/destination") {
			t.Fatalf("template context absent: %v %s", err, rendered)
		}
		raw, _ := json.Marshal(ctx.Data)
		if strings.Contains(string(raw), "HIDDEN_") {
			t.Fatalf("template private data leaked: %s", raw)
		}
		return rendered, err
	})
	ws.engine = engine
	snapshots, err := ws.renderSnapshots(context.Background())
	if err != nil || len(snapshots) == 0 || calls < 2 {
		t.Fatalf("template snapshots: %v calls=%d", err, calls)
	}
}
