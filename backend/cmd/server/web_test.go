package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"blog/internal/store"
	tmpl "blog/internal/template"
)

func TestToCard(t *testing.T) {
	site := &tmpl.Site{}
	site.Article.CategoryColor = map[string]string{"技术": "ff0000"}
	site.Article.CategoryIcon = map[string]string{"技术": "solar:folder-bold"}
	ws := newWebServer(webDeps{Site: sitePtr(site)})
	created := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	p := &store.Post{
		Slug: "hello", Title: "Hello", Excerpt: "ex", Cover: "/c.png",
		CreatedAt: created, Pinned: true, Category: "技术",
	}
	card := ws.toCard(p)
	if card.Slug != "hello" || card.Title != "Hello" || !card.Pinned || !card.Date.Equal(created) {
		t.Errorf("card basic fields mismatch: %+v", card)
	}
	if card.CategoryStyle == "" {
		t.Error("category with configured color should produce style")
	}
	if card.CategoryIcon != "solar:folder-bold" {
		t.Errorf("category with configured icon = %q, want %q", card.CategoryIcon, "solar:folder-bold")
	}
	if card.Poster != nil {
		t.Error("empty poster column should not produce poster card")
	}
	// 带海报：DecodePoster 命中触发 PosterCard；未配置图标分类回退 default:category
	p2 := &store.Post{Slug: "p2", Category: "随笔", Poster: store.EncodePoster(store.PosterData{Headline: "大字"})}
	c2 := ws.toCard(p2)
	if c2.Poster == nil || c2.Poster.Headline != "大字" {
		t.Errorf("poster card mismatch: %+v", c2.Poster)
	}
	if c2.CategoryIcon != "default:category" {
		t.Errorf("unconfigured category icon = %q, want default:category", c2.CategoryIcon)
	}
}

func TestPinSliderActiveOnlyOnHomeFirstPage(t *testing.T) {
	cases := []struct {
		name string
		data *tmpl.ViewData
		want bool
	}{
		{
			name: "home first page",
			data: &tmpl.ViewData{Page: &tmpl.Page{Layout: "index"}, Paginator: &tmpl.Paginator{Current: 1}},
			want: true,
		},
		{
			name: "home second page",
			data: &tmpl.ViewData{Page: &tmpl.Page{Layout: "index"}, Paginator: &tmpl.Paginator{Current: 2}},
			want: false,
		},
		{
			name: "category list",
			data: &tmpl.ViewData{Page: &tmpl.Page{Layout: "index", Category: "技术"}},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pinSliderReplacesPosts(tc.data); got != tc.want {
				t.Fatalf("pinSliderReplacesPosts = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBuildTagCloud(t *testing.T) {
	counts := map[string]int{"a": 1, "b": 3, "c": 3, "d": 2}
	items := buildTagCloud(counts, 3)
	if len(items) != 3 {
		t.Fatalf("limit not applied: got %d items", len(items))
	}
	// 数量降序，同量按名升序：b(3) c(3) d(2)
	if items[0].Name != "b" || items[1].Name != "c" || items[2].Name != "d" {
		t.Errorf("order mismatch: %s %s %s", items[0].Name, items[1].Name, items[2].Name)
	}
	for _, it := range items {
		if it.Size == 0 {
			t.Errorf("tag %s should have font size", it.Name)
		}
	}
}

// TestRenderSitePreview 设置页「前端预览」：草稿分段应用到 site 副本并渲染整页，
// 不影响全局共享 site（所见即所得不落库）。
func TestRenderSitePreview(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	eng, err := tmpl.NewEngineFromEmbed(&tmpl.IconRegistry{})
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	live := &tmpl.Site{Title: "线上标题"}
	ws := newWebServer(webDeps{Site: sitePtr(live), Store: st, Engine: eng})

	sections := map[string]json.RawMessage{
		"site": json.RawMessage(`{"title":"草稿标题"}`),
	}
	html, err := ws.renderSitePreview(sections, "index")
	if err != nil {
		t.Fatalf("renderSitePreview: %v", err)
	}
	if !strings.Contains(html, "草稿标题") {
		t.Errorf("preview html should contain draft title, got %d bytes", len(html))
	}
	// 全局 site 不被草稿污染
	if live.Title != "线上标题" {
		t.Errorf("live site mutated by preview: %q", live.Title)
	}
}

func TestRenderSitePreviewWidgetsOnlyKeepsLiveSiteSections(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	eng, err := tmpl.NewEngineFromEmbed(&tmpl.IconRegistry{})
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	live := &tmpl.Site{Title: "线上标题", Comments: tmpl.CommentsConfig{Enabled: true}}
	ws := newWebServer(webDeps{Site: sitePtr(live), Store: st, Engine: eng})

	html, err := ws.renderSitePreview(map[string]json.RawMessage{
		"widgets": json.RawMessage(`{"library":{"__preview__":{"layout":"recent"}},"pages":{"index":{"leftbar":["__preview__"]}}}`),
	}, "index")
	if err != nil {
		t.Fatalf("renderSitePreview widgets-only: %v", err)
	}
	if !strings.Contains(html, "线上标题") {
		t.Errorf("widgets-only preview should retain site title")
	}
	if live.Comments.Enabled != true {
		t.Error("widgets-only preview mutated live comments")
	}
}

func TestRenderSitePreviewWikiUsesFirstProjectTreeAndRelatedData(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.EnsureWikidocsTable(); err != nil {
		t.Fatalf("ensure wikidocs: %v", err)
	}
	if err := st.CreateWikidoc(&store.WikiDoc{
		ProjectID: "first", Slug: "intro", Title: "首个项目文档", Order: 1, Status: store.StatusPublished,
	}); err != nil {
		t.Fatalf("create wiki doc: %v", err)
	}
	eng, err := tmpl.NewEngineFromEmbed(&tmpl.IconRegistry{})
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	first := tmpl.WikiProject{ID: "first", Title: "首个项目", Tags: []string{"go"}, Sort: 10}
	second := tmpl.WikiProject{ID: "second", Title: "相关项目", Tags: []string{"go"}, Sort: 1}
	site := &tmpl.Site{
		Title: "文库预览",
		Wiki:  []tmpl.WikiProject{first, second},
		Widgets: tmpl.WidgetsConfig{
			Library: map[string]tmpl.WidgetDef{
				"tree":         {"layout": "tree"},
				"wiki_related": {"layout": "wiki_related"},
			},
			Pages: map[string]tmpl.PageBars{
				"wiki": {Leftbar: []string{"tree", "wiki_related"}},
			},
		},
	}
	ws := newWebServer(webDeps{Site: sitePtr(site), Store: st, Engine: eng})

	html, err := ws.renderSitePreview(nil, "wiki")
	if err != nil {
		t.Fatalf("renderSitePreview wiki: %v", err)
	}
	for _, want := range []string{"首个项目文档", "相关项目", `id="data-tree"`} {
		if !strings.Contains(html, want) {
			t.Errorf("wiki preview missing %q", want)
		}
	}
}

func TestRenderSitePreviewNotebooksUsesFirstNotebookTagTree(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.EnsureNotebooksTable(); err != nil {
		t.Fatalf("ensure notebooks: %v", err)
	}
	if err := st.EnsureNotesTable(); err != nil {
		t.Fatalf("ensure notes: %v", err)
	}
	nb := &store.Notebook{Name: "go-notes", Order: 1}
	if err := st.CreateNotebook(nb); err != nil {
		t.Fatalf("create notebook: %v", err)
	}
	if err := st.CreateNote(&store.Note{NotebookID: nb.ID, Title: "Go 入门", Tags: "Go,后端"}); err != nil {
		t.Fatalf("create note: %v", err)
	}
	eng, err := tmpl.NewEngineFromEmbed(&tmpl.IconRegistry{})
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	site := &tmpl.Site{
		Title: "笔记预览",
		Widgets: tmpl.WidgetsConfig{
			Library: map[string]tmpl.WidgetDef{"tagtree": {"layout": "tagtree"}},
			Pages:   map[string]tmpl.PageBars{"notebooks": {Leftbar: []string{"tagtree"}}},
		},
	}
	ws := newWebServer(webDeps{Site: sitePtr(site), Store: st, Engine: eng})

	html, err := ws.renderSitePreview(nil, "notebooks")
	if err != nil {
		t.Fatalf("renderSitePreview notebooks: %v", err)
	}
	for _, want := range []string{"go-notes", "Go", `id="data-tagtree"`} {
		if !strings.Contains(html, want) {
			t.Errorf("notebooks preview missing %q", want)
		}
	}
}

func TestRenderSitePreviewPostUsesDraftTOCDepth(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if _, err := st.Create(&store.Post{
		Slug: "toc-preview", Title: "TOC 预览", Status: store.StatusPublished,
		ContentHTML: `<h1 id="h1">一级标题</h1><h2 id="h2">二级标题</h2>`,
	}); err != nil {
		t.Fatalf("create post: %v", err)
	}
	eng, err := tmpl.NewEngineFromEmbed(&tmpl.IconRegistry{})
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	site := &tmpl.Site{
		Title: "TOC 预览",
		Widgets: tmpl.WidgetsConfig{
			Library: map[string]tmpl.WidgetDef{"toc": {"layout": "toc", "min_depth": 1}},
			Pages:   map[string]tmpl.PageBars{"post": {Rightbar: []string{"toc"}}},
		},
	}
	ws := newWebServer(webDeps{Site: sitePtr(site), Store: st, Engine: eng})

	html, err := ws.renderSitePreview(map[string]json.RawMessage{
		"widgets": json.RawMessage(`{"library":{"toc":{"layout":"toc","min_depth":2}},"pages":{"post":{"rightbar":["toc"]}}}`),
	}, "post")
	if err != nil {
		t.Fatalf("renderSitePreview post: %v", err)
	}
	if strings.Contains(html, `href="#h1"`) || !strings.Contains(html, `href="#h2"`) {
		t.Errorf("draft toc min_depth=2 not applied: %s", html)
	}
}
