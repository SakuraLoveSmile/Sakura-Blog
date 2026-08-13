package seed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"blog/internal/post"
	"blog/internal/store"
)

// writePost 写一篇 frontmatter 文章到 mdDir。
func writePost(t *testing.T, mdDir, slug, frontmatter string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(mdDir, slug+".md"), []byte("---\n"+frontmatter+"---\n\n正文内容"), 0o644); err != nil {
		t.Fatalf("write %s: %v", slug, err)
	}
}

// newSeedFixture 临时 store + 临时 md 目录 + post.Service（构造在 fixture 内返回 mdDir）。
func newSeedFixture(t *testing.T) (*store.Store, *post.Service, string) {
	t.Helper()
	st, err := store.New(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	mdDir := t.TempDir()
	return st, post.NewService(mdDir), mdDir
}

// TestImportIdempotent -seed 幂等：跑两遍不产生重复记录，内容字段被更新。
func TestImportIdempotent(t *testing.T) {
	st, posts, mdDir := newSeedFixture(t)
	writePost(t, mdDir, "hello", "title: 你好\n")
	if err := Import(st, posts); err != nil {
		t.Fatalf("first import: %v", err)
	}
	first, err := st.GetBySlug("hello")
	if err != nil || first == nil {
		t.Fatalf("get after first import: %v / %v", first, err)
	}
	if err := Import(st, posts); err != nil {
		t.Fatalf("second import: %v", err)
	}
	second, err := st.GetBySlug("hello")
	if err != nil || second == nil {
		t.Fatalf("get after second import: %v / %v", second, err)
	}
	if second.ID != first.ID {
		t.Fatalf("id changed after re-import: %d -> %d（重复导入应幂等）", first.ID, second.ID)
	}
	if second.Title != "你好" || second.ContentMD == "" {
		t.Fatalf("content not imported: %+v", second)
	}
}

// TestImportDraftSemantics frontmatter status: draft 不得被发布（曾修两次的 bug 点）。
func TestImportDraftSemantics(t *testing.T) {
	st, posts, mdDir := newSeedFixture(t)
	writePost(t, mdDir, "drafty", "title: 草稿\nstatus: draft\n")
	writePost(t, mdDir, "pub", "title: 已发布\n")
	if err := Import(st, posts); err != nil {
		t.Fatalf("import: %v", err)
	}
	d, err := st.GetBySlug("drafty")
	if err != nil || d == nil {
		t.Fatalf("get drafty: %v / %v", d, err)
	}
	if d.Status != store.StatusDraft {
		t.Fatalf("draft status = %q, want %q", d.Status, store.StatusDraft)
	}
	p, err := st.GetBySlug("pub")
	if err != nil || p == nil {
		t.Fatalf("get pub: %v / %v", p, err)
	}
	if p.Status != store.StatusPublished {
		t.Fatalf("published status = %q", p.Status)
	}
}

// TestImportPinnedMapping frontmatter pinned: true 必须映射到 DB 置顶（曾漏映射的 bug 点）。
func TestImportPinnedMapping(t *testing.T) {
	st, posts, mdDir := newSeedFixture(t)
	writePost(t, mdDir, "top", "title: 置顶\npinned: true\n")
	if err := Import(st, posts); err != nil {
		t.Fatalf("import: %v", err)
	}
	p, err := st.GetBySlug("top")
	if err != nil || p == nil {
		t.Fatalf("get top: %v / %v", p, err)
	}
	if !p.Pinned {
		t.Fatal("pinned: true 未映射到 DB 记录")
	}
}

// TestImportTagsAndTopic tags 逗号规范化 + topic 关联独立表。
func TestImportTagsAndTopic(t *testing.T) {
	st, posts, mdDir := newSeedFixture(t)
	if err := st.EnsureTopicsTable(); err != nil {
		t.Fatalf("ensure topics: %v", err)
	}
	if err := st.CreateTopic(&store.Topic{Name: "Go 学习"}); err != nil {
		t.Fatalf("create topic: %v", err)
	}
	writePost(t, mdDir, "gopost", "title: Go 文章\ncategory: 技术\ntags: Go, Gin\ntopic: Go 学习\n")
	if err := Import(st, posts); err != nil {
		t.Fatalf("import: %v", err)
	}
	p, err := st.GetBySlug("gopost")
	if err != nil || p == nil {
		t.Fatalf("get gopost: %v / %v", p, err)
	}
	if p.Category != "技术" {
		t.Fatalf("category = %q", p.Category)
	}
	if !strings.Contains(p.Tags, "Go") || !strings.Contains(p.Tags, "Gin") {
		t.Fatalf("tags = %q", p.Tags)
	}
	tp, err := st.GetTopicByName("Go 学习")
	if err != nil || tp == nil {
		t.Fatalf("get topic: %v / %v", tp, err)
	}
	if p.TopicID != tp.ID {
		t.Fatalf("topic_id = %d, want %d", p.TopicID, tp.ID)
	}
}
