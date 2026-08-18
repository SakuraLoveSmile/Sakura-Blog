package seed

import (
	"testing"
)

// TestImportMissingAdditive 增量发现只补录 DB 缺失的文件：
// 已有记录（含 Console 编辑过的内容）绝不被文件覆盖。
func TestImportMissingAdditive(t *testing.T) {
	st, posts, mdDir := newSeedFixture(t)
	writePost(t, mdDir, "alpha", "title: 甲篇\n")

	n, err := ImportMissing(st, posts)
	if err != nil || n != 1 {
		t.Fatalf("first discover = %d, %v；want 1, nil", n, err)
	}

	// 模拟 Console 侧改动 DB（文件未同步），再次发现不得覆盖
	if p, err := st.GetBySlug("alpha"); err != nil || p == nil {
		t.Fatalf("get alpha: %v / %v", p, err)
	} else {
		p.Title = "甲篇·Console 改过的标题"
		if err := st.Upsert(p); err != nil {
			t.Fatalf("simulate console edit: %v", err)
		}
	}
	// 同时把文件也改掉，证明发现路径不会用文件覆盖 DB
	writePost(t, mdDir, "alpha", "title: 甲篇·文件新标题\n")

	n, err = ImportMissing(st, posts)
	if err != nil || n != 0 {
		t.Fatalf("second discover = %d, %v；want 0, nil（已存在不得重导）", n, err)
	}
	got, err := st.GetBySlug("alpha")
	if err != nil || got == nil {
		t.Fatalf("get alpha after discover: %v / %v", got, err)
	}
	if got.Title != "甲篇·Console 改过的标题" {
		t.Fatalf("发现路径覆盖了已有记录：title = %q", got.Title)
	}

	// 新增文件下一轮被发现
	writePost(t, mdDir, "beta", "title: 乙篇\n")
	n, err = ImportMissing(st, posts)
	if err != nil || n != 1 {
		t.Fatalf("third discover = %d, %v；want 1, nil", n, err)
	}
	if p, err := st.GetBySlug("beta"); err != nil || p == nil || p.Title != "乙篇" {
		t.Fatalf("beta 未被发现：%v / %v", p, err)
	}
}

// TestImportMissingDraft 发现路径同样尊重 frontmatter status: draft。
func TestImportMissingDraft(t *testing.T) {
	st, posts, mdDir := newSeedFixture(t)
	writePost(t, mdDir, "draft-one", "title: 草稿\nstatus: draft\n")

	n, err := ImportMissing(st, posts)
	if err != nil || n != 1 {
		t.Fatalf("discover = %d, %v；want 1, nil", n, err)
	}
	p, err := st.GetBySlug("draft-one")
	if err != nil || p == nil {
		t.Fatalf("get draft-one: %v / %v", p, err)
	}
	if p.Status != "draft" {
		t.Fatalf("发现的草稿状态错误：%q", p.Status)
	}
}
