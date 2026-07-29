package main

import (
	"strings"
	"testing"

	"blog/internal/store"
)

func TestSearchItemsFromPosts(t *testing.T) {
	posts := []*store.Post{
		{Slug: "open", Title: "公开文章", ContentHTML: "<p>正文内容</p>"},
		{Slug: "locked", Title: "加密文章", ContentHTML: "<p>秘密</p>", Password: "pw"},
	}
	items := searchItemsFromPosts(posts)
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d", len(items))
	}
	if items[0].Path != "/post/open" || items[0].Content != "正文内容" {
		t.Errorf("open item mismatch: %+v", items[0])
	}
	// 受密码保护文章：标题保留、正文不泄漏
	if items[1].Title != "加密文章" || items[1].Content != "" {
		t.Errorf("protected post must not leak content: %+v", items[1])
	}
	if strings.Contains(items[1].Content, "秘密") {
		t.Error("protected content leaked")
	}
}
