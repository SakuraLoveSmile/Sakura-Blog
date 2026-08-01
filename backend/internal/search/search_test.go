package search

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// newTestIndex 内存级 SQLite（临时文件）+ 建好 FTS 表的索引对象。
func newTestIndex(t *testing.T) *Index {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
	ix := New(db)
	if err := ix.Ensure(); err != nil {
		t.Fatalf("ensure fts: %v", err)
	}
	return ix
}

func TestChineseHitAndSnippet(t *testing.T) {
	ix := newTestIndex(t)
	if err := ix.IndexPost("hello-go", "Go 语言入门", "这是一篇讲解 Go 语言并发模型的文章，goroutine 是核心概念。"); err != nil {
		t.Fatalf("index: %v", err)
	}
	res, err := ix.Search("并发模型", 20)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("want 1 result, got %d", len(res))
	}
	if res[0].Slug != "hello-go" || res[0].Title != "Go 语言入门" {
		t.Errorf("unexpected result: %+v", res[0])
	}
	if !strings.Contains(res[0].Snippet, `<span class="search-keyword">`) {
		t.Errorf("snippet missing highlight: %q", res[0].Snippet)
	}
	// CJK 空格应还原：snippet 中不应出现「并 发」这类切分残留
	if strings.Contains(res[0].Snippet, "并 发") {
		t.Errorf("snippet has unjoined CJK: %q", res[0].Snippet)
	}
}

func TestBM25TitleWeight(t *testing.T) {
	ix := newTestIndex(t)
	// 标题命中的文章应排在仅正文命中的前面
	if err := ix.IndexPost("body-hit", "无关标题", "正文提到 Docker 部署一次。"); err != nil {
		t.Fatal(err)
	}
	if err := ix.IndexPost("title-hit", "Docker 部署指南", "完全无关的正文内容。Docker 相关。"); err != nil {
		t.Fatal(err)
	}
	res, err := ix.Search("Docker", 20)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("want 2 results, got %d", len(res))
	}
	if res[0].Slug != "title-hit" {
		t.Errorf("title-weighted result should rank first, got %s", res[0].Slug)
	}
}

func TestProtectedContentExcluded(t *testing.T) {
	ix := newTestIndex(t)
	// 受保护文章由调用方传空正文，正文关键词不可命中，标题仍可搜
	if err := ix.IndexPost("secret", "公开的标题", ""); err != nil {
		t.Fatal(err)
	}
	res, err := ix.Search("秘密内容", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 {
		t.Errorf("protected content should not be searchable, got %d", len(res))
	}
	res, err = ix.Search("公开的标题", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Errorf("title should still be searchable, got %d", len(res))
	}
}

func TestEmptyQuery(t *testing.T) {
	ix := newTestIndex(t)
	if err := ix.IndexPost("a", "标题", "正文"); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"", "   ", "\t\n"} {
		res, err := ix.Search(q, 20)
		if err != nil {
			t.Fatalf("search %q: %v", q, err)
		}
		if len(res) != 0 {
			t.Errorf("empty query %q should return empty, got %d", q, len(res))
		}
	}
}

func TestUnindexAndReindex(t *testing.T) {
	ix := newTestIndex(t)
	if err := ix.IndexPost("p1", "标题一", "正文内容"); err != nil {
		t.Fatal(err)
	}
	// 重复索引不产生重复条目
	if err := ix.IndexPost("p1", "标题一改", "正文内容改"); err != nil {
		t.Fatal(err)
	}
	n, err := ix.Count()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("reindex should not duplicate, count=%d", n)
	}
	res, _ := ix.Search("标题一改", 20)
	if len(res) != 1 {
		t.Errorf("updated title should be searchable, got %d", len(res))
	}
	if err := ix.Unindex("p1"); err != nil {
		t.Fatal(err)
	}
	res, _ = ix.Search("标题一改", 20)
	if len(res) != 0 {
		t.Errorf("unindexed post should disappear, got %d", len(res))
	}
}

func TestRebuild(t *testing.T) {
	ix := newTestIndex(t)
	if err := ix.IndexPost("stale", "过期条目", "旧数据"); err != nil {
		t.Fatal(err)
	}
	items := []Item{
		{Slug: "a", Title: "文章甲", Content: "甲的正文"},
		{Slug: "b", Title: "文章乙", Content: "乙的正文"},
	}
	if err := ix.Rebuild(items); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	n, _ := ix.Count()
	if n != 2 {
		t.Errorf("rebuild count want 2, got %d", n)
	}
	res, _ := ix.Search("过期条目", 20)
	if len(res) != 0 {
		t.Errorf("stale entry should be gone after rebuild, got %d", len(res))
	}
}

func TestEnsureMigratesLegacyTable(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	// 模拟历史实验表：trigram 结构（无 title_raw 列）
	if _, err := db.Exec(`CREATE VIRTUAL TABLE posts_fts USING fts5(title, content_md, tokenize='trigram')`); err != nil {
		t.Fatalf("create legacy: %v", err)
	}
	ix := New(db)
	if err := ix.Ensure(); err != nil {
		t.Fatalf("ensure should migrate legacy table: %v", err)
	}
	if err := ix.IndexPost("m", "迁移后标题", "迁移后正文"); err != nil {
		t.Fatalf("index after migrate: %v", err)
	}
	res, err := ix.Search("迁移后标题", 20)
	if err != nil || len(res) != 1 {
		t.Fatalf("search after migrate: res=%d err=%v", len(res), err)
	}
}

func TestSegmentAndJoinCJK(t *testing.T) {
	seg := segmentCJK("Go语言hello世界")
	if seg != "Go 语 言 hello 世 界" {
		t.Errorf("segmentCJK unexpected: %q", seg)
	}
	joined := joinCJK("你 好 world 世 界")
	if joined != "你好 world 世界" {
		t.Errorf("joinCJK unexpected: %q", joined)
	}
	// 高亮占位符不阻断合并
	j2 := joinCJK("并 \x01发\x02 模 型")
	if j2 != "并\x01发\x02模型" {
		t.Errorf("joinCJK with markers unexpected: %q", j2)
	}
}
