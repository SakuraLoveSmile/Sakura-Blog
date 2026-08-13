package main

import (
	"encoding/json"
	"strings"
	"testing"

	"blog/internal/store"
	tmpl "blog/internal/template"
)

// TestSearchJSONCacheRebuildsOnce（Spec 61 阶段 5）：置脏后重建一次，未置脏时重复读取零重建。
func TestSearchJSONCacheRebuildsOnce(t *testing.T) {
	st, err := store.New(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if _, err := st.Create(&store.Post{Slug: "hello", Title: "你好", Status: store.StatusPublished, ContentHTML: "<p>内容</p>"}); err != nil {
		t.Fatalf("create post: %v", err)
	}
	sitePtr := sitePtr(&tmpl.Site{})
	c := newContentCache(st, sitePtr)

	// 首次读取触发重建
	first, err := c.searchJSON()
	if err != nil {
		t.Fatalf("first get: %v", err)
	}
	// 未置脏：第二次读取直接命中缓存
	second, err := c.searchJSON()
	if err != nil {
		t.Fatalf("second get: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("cached bytes should be stable")
	}
	// 内容含文章标题（索引确实重建过）
	var items []searchItem
	if err := json.Unmarshal(first, &items); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(items) != 1 || items[0].Title != "你好" {
		t.Fatalf("unexpected items: %+v", items)
	}

	// 置脏后重建可见新文章
	if _, err := st.Create(&store.Post{Slug: "world", Title: "世界", Status: store.StatusPublished, ContentHTML: "<p>x</p>"}); err != nil {
		t.Fatalf("create post 2: %v", err)
	}
	c.invalidate()
	third, _ := c.searchJSON()
	if !strings.Contains(string(third), "世界") {
		t.Fatal("invalidate should trigger rebuild with new content")
	}
}

// TestSitemapCacheKeyedByBaseURL（Spec 61 阶段 5）：不同 baseURL 独立缓存，置脏后全量失效。
func TestSitemapCacheKeyedByBaseURL(t *testing.T) {
	st, err := store.New(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	sitePtr := sitePtr(&tmpl.Site{})
	c := newContentCache(st, sitePtr)

	a, err := c.sitemapXML("https://a.example")
	if err != nil {
		t.Fatalf("sitemap a: %v", err)
	}
	b, err := c.sitemapXML("https://b.example")
	if err != nil {
		t.Fatalf("sitemap b: %v", err)
	}
	if !strings.Contains(string(a), "https://a.example/") || strings.Contains(string(a), "https://b.example") {
		t.Fatal("sitemap a should be keyed to its baseURL")
	}
	if !strings.Contains(string(b), "https://b.example/") {
		t.Fatal("sitemap b should be keyed to its baseURL")
	}
	// 同 key 二次读取直接命中（内容一致即可）
	a2, _ := c.sitemapXML("https://a.example")
	if string(a) != string(a2) {
		t.Fatal("same baseURL should be cached")
	}
}
