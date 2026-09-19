package post

import (
	"strings"
	"testing"

	"blog/internal/settings"
)

// ---------- 阶段 E: Wiki TOC 树序渲染与草稿防护 ----------

func TestTOCTagWithWiki(t *testing.T) {
	wikiDocs := map[string][]settings.WikiTreeNode{
		"flat-project": {
			{Doc: &settings.WikiDocView{Slug: "doc1", Title: "文档一"}},
			{Doc: &settings.WikiDocView{Slug: "doc2", Title: "文档二"}},
		},
		"multi-section-project": {
			{
				Doc: &settings.WikiDocView{Slug: "getting-started", Title: "入门指南"},
				Children: []settings.WikiTreeNode{
					{Doc: &settings.WikiDocView{Slug: "install", Title: "安装步骤"}},
					{Doc: &settings.WikiDocView{Slug: "quickstart", Title: "快速起步"}},
				},
			},
			{
				Doc: &settings.WikiDocView{Slug: "advanced", Title: "高级特性"},
				Children: []settings.WikiTreeNode{
					{Doc: &settings.WikiDocView{Slug: "config", Title: "详细配置"}},
				},
			},
		},
	}

	wikiProvider := func(projectID string) (bool, []settings.WikiTreeNode) {
		nodes, ok := wikiDocs[projectID]
		return ok, nodes
	}

	s := NewService(t.TempDir())
	s.SetWikiTreeProvider(wikiProvider)

	t.Run("单层扁平公开文档树渲染", func(t *testing.T) {
		out, err := s.Render("{% toc wiki:flat-project 文档目录 open:true %}")
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		for _, want := range []string{
			`<div class="tag-plugin toc">`,
			`<details class="toc" open>`,
			`<summary>文档目录</summary>`,
			`<div class="body fs14"><div class="body"><ul class="toc">`,
			`<li><a class="list-link" href="/wiki/flat-project/doc1"><span>文档一</span></a></li>`,
			`<li><a class="list-link" href="/wiki/flat-project/doc2"><span>文档二</span></a></li>`,
			`</ul></div></div>`,
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in output: %s", want, out)
			}
		}
	})

	t.Run("多分组层级树序渲染", func(t *testing.T) {
		out, err := s.Render("{% toc wiki:multi-section-project %}")
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		for _, want := range []string{
			`<div class="tag-plugin toc">`,
			`<summary>TOC</summary>`,
			`<div class="body fs14 multi">`,
			`<section><div class="header">入门指南</div><ul class="toc">`,
			`<li><a class="list-link" href="/wiki/multi-section-project/getting-started"><span>入门指南</span></a></li>`,
			`<li><a class="list-link" href="/wiki/multi-section-project/install"><span>安装步骤</span></a></li>`,
			`<li><a class="list-link" href="/wiki/multi-section-project/quickstart"><span>快速起步</span></a></li>`,
			`<section><div class="header">高级特性</div><ul class="toc">`,
			`<li><a class="list-link" href="/wiki/multi-section-project/config"><span>详细配置</span></a></li>`,
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in output: %s", want, out)
			}
		}
	})

	t.Run("项目不存在时线上渲染保持上游空结果语义", func(t *testing.T) {
		out, err := s.Render("{% toc wiki:not-exist %}")
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		if out != "" {
			t.Errorf("nonexistent wiki project should produce empty string, got: %q", out)
		}
	})

	t.Run("Console 预览对不存在的项目输出诊断提示且不暴露草稿", func(t *testing.T) {
		previewOut := renderTOCTag("wiki:not-exist 项目目录", wikiProvider, true)
		for _, want := range []string{
			`<div class="tag-plugin toc">`,
			`<summary>项目目录</summary>`,
			`Wiki 项目 &#34;not-exist&#34; 不存在`,
		} {
			if !strings.Contains(previewOut, want) {
				t.Errorf("preview diagnostic missing %q in: %s", want, previewOut)
			}
		}
	})
}

// ---------- 阶段 E: Albums / Posters 分组卡片直出与回退机制 ----------

func TestAlbumsPostersTagWithGroup(t *testing.T) {
	groups := []settings.LinkGroup{
		{
			Title: "摄影集",
			Items: []settings.LinkItem{
				{Name: "京都之行", URL: "https://photos.example.com/kyoto", Snapshot: "https://img.example.com/kyoto.jpg"},
				{Name: "川西徒步", URL: "https://photos.example.com/tibet", Avatar: "https://img.example.com/tibet-avatar.jpg"},
			},
		},
		{
			Title: "摄影集", // 同名组，测试按顺序稳定合并
			Items: []settings.LinkItem{
				{Name: "海边日落", URL: "https://photos.example.com/sunset"}, // 无 snapshot 无 avatar，回退 defaultCover
			},
		},
	}

	defaultCover := "https://cdn.example.com/default-cover.png"

	s := NewService(t.TempDir())
	s.SetAlbumsLinksFunc(func() ([]settings.LinkGroup, string) {
		return groups, defaultCover
	})

	t.Run("albums 分组直出、封面回退与同名组合并", func(t *testing.T) {
		out, err := s.Render("{% albums 摄影集 size:m %}")
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		for _, want := range []string{
			`class="tag-plugin albums-wrap"`,
			`<div class="tag-plugin gallery grid-box" layout="grid" ratio="square" size="m">`,
			// 条目一：snapshot 优先
			`class="grid-cell album-card"`,
			`href="https://photos.example.com/kyoto"`,
			`data-src="https://img.example.com/kyoto.jpg"`,
			`<span class="image-caption">京都之行</span>`,
			// 条目二：avatar 回退
			`href="https://photos.example.com/tibet"`,
			`data-src="https://img.example.com/tibet-avatar.jpg"`,
			`<span class="image-caption">川西徒步</span>`,
			// 条目三（来自第二个同名组）：defaultCover 回退
			`href="https://photos.example.com/sunset"`,
			`data-src="https://cdn.example.com/default-cover.png"`,
			`<span class="image-caption">海边日落</span>`,
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in albums output: %s", want, out)
			}
		}
	})

	t.Run("posters 分组直出结构", func(t *testing.T) {
		out, err := s.Render("{% posters 摄影集 %}")
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		for _, want := range []string{
			`class="tag-plugin posters-wrap"`,
			`<div class="tag-plugin gallery grid-box" layout="grid" ratio="portrait" size="xs">`,
			`class="grid-cell poster-card"`,
			`href="https://photos.example.com/kyoto"`,
			`<span class="image-caption">京都之行</span>`,
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in posters output: %s", want, out)
			}
		}
	})

	t.Run("api > repo > group 优先级：指定 api 时 api 胜出", func(t *testing.T) {
		out, err := s.Render("{% albums 摄影集 api:https://data.example.com/friends.json size:s %}")
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		if !strings.Contains(out, `data-api="https://data.example.com/friends.json"`) {
			t.Errorf("api should win over group, got: %s", out)
		}
		if strings.Contains(out, "京都之行") {
			t.Errorf("api branch should not render local group items: %s", out)
		}
	})

	t.Run("未命中分组时输出空 gallery 容器", func(t *testing.T) {
		out, err := s.Render("{% albums 未知分组 size:l %}")
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		want := `<div class="tag-plugin albums-wrap"><div class="tag-plugin gallery grid-box" layout="grid" ratio="square" size="l"></div></div>`
		if out != want {
			t.Errorf("unmatched group should render empty gallery, want %q, got %q", want, out)
		}
	})
}

// ---------- 阶段 E: Shortcode 可视化编辑与双向往返 ----------

func TestShortcodeEditAlbumsAndPosters(t *testing.T) {
	t.Run("albums 解析 group 与 size", func(t *testing.T) {
		res, err := ParseShortcodeForEdit("{% albums 推荐站点 size:m %}")
		if err != nil {
			t.Fatalf("ParseShortcodeForEdit: %v", err)
		}
		if res.Name != "albums" {
			t.Errorf("expected name albums, got %q", res.Name)
		}
		if res.Values["group"] != "推荐站点" {
			t.Errorf("expected group '推荐站点', got %v", res.Values["group"])
		}
		if res.Values["size"] != "m" {
			t.Errorf("expected size 'm', got %v", res.Values["size"])
		}
	})

	t.Run("posters 解析 group", func(t *testing.T) {
		res, err := ParseShortcodeForEdit("{% posters 院线大片 %}")
		if err != nil {
			t.Fatalf("ParseShortcodeForEdit: %v", err)
		}
		if res.Name != "posters" {
			t.Errorf("expected name posters, got %q", res.Name)
		}
		if res.Values["group"] != "院线大片" {
			t.Errorf("expected group '院线大片', got %v", res.Values["group"])
		}
	})

	t.Run("未知参数报错回退源码编辑，不篡改原始内容", func(t *testing.T) {
		_, err := ParseShortcodeForEdit("{% albums group:推荐站点 unknown_param:foo %}")
		if err == nil {
			t.Fatal("expected error on unknown argument, got nil")
		}
		if !strings.Contains(err.Error(), "unknown_param") {
			t.Errorf("error message should mention unknown_param, got: %v", err)
		}
	})
}
