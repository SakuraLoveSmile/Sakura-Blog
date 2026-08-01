package main

import (
	"testing"
	"time"

	"blog/internal/store"
	tmpl "blog/internal/template"
)

func TestTruncateText(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"短文本", 40, "短文本"},
		{"一二三四五", 3, "一二三…"}, // 按 rune 截断，多字节安全
		{"", 5, ""},
	}
	for _, c := range cases {
		if got := truncateText(c.in, c.n); got != c.want {
			t.Errorf("truncateText(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}

func TestStripHTML(t *testing.T) {
	got := stripHTML("<p>hello   <b>world</b></p>\n\n<span>!</span>")
	if got != "hello world !" {
		t.Errorf("stripHTML = %q, want %q", got, "hello world !")
	}
}

func TestToCard(t *testing.T) {
	site := &tmpl.Site{}
	site.Article.CategoryColor = map[string]string{"技术": "ff0000"}
	ws := newWebServer(webDeps{Site: site})
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
	if card.Poster != nil {
		t.Error("empty poster column should not produce poster card")
	}
	// 带海报：DecodePoster 命中触发 PosterCard
	p2 := &store.Post{Slug: "p2", Poster: store.EncodePoster(store.PosterData{Headline: "大字"})}
	if c2 := ws.toCard(p2); c2.Poster == nil || c2.Poster.Headline != "大字" {
		t.Errorf("poster card mismatch: %+v", c2.Poster)
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
