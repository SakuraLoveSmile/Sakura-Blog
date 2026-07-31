package main

import (
	"strings"
	"testing"
	"time"

	"blog/internal/store"
)

func TestRSSItemsFromPosts(t *testing.T) {
	created := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	long := strings.Repeat("长", 100) // 300 字节 UTF-8，触发 200 字节截断
	posts := []*store.Post{
		{Slug: "a", Title: "有摘要", Excerpt: "摘要", CreatedAt: created, Author: "sakura"},
		{Slug: "b", Title: "无摘要", ContentHTML: "<p>" + long + "</p>", CreatedAt: created, Author: "unknown"},
		{Slug: "c", Title: "加密", ContentHTML: "<p>秘密</p>", Password: "pw", CreatedAt: created},
	}
	// 模拟作者解析：已知作者返回名称，否则回退站点名
	resolveAuthor := func(postAuthor string) string {
		if postAuthor == "sakura" {
			return "樱花"
		}
		return "默认站点"
	}
	items := rssItemsFromPosts(posts, "https://example.com", resolveAuthor)
	if len(items) != 3 {
		t.Fatalf("want 3 items, got %d", len(items))
	}
	if items[0].Description != "摘要" || items[0].Link != "https://example.com/post/a" || items[0].Guid != items[0].Link {
		t.Errorf("item with excerpt mismatch: %+v", items[0])
	}
	if items[0].PubDate != created.Format(time.RFC1123Z) {
		t.Errorf("pubDate mismatch: %s", items[0].PubDate)
	}
	if len(items[1].Description) > 200 {
		t.Errorf("description should be truncated to 200 bytes, got %d", len(items[1].Description))
	}
	if items[2].Description != "本文受密码保护" {
		t.Errorf("protected post must hide content: %q", items[2].Description)
	}
	// 验证 author 字段：已知作者返回解析名，未知作者回退默认
	if items[0].Author != "樱花" {
		t.Errorf("author mismatch: got %q, want %q", items[0].Author, "樱花")
	}
	if items[1].Author != "默认站点" {
		t.Errorf("unknown author fallback: got %q, want %q", items[1].Author, "默认站点")
	}
}
