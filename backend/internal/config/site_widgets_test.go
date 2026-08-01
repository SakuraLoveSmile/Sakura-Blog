package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadSiteWidgetsSection site.yml widgets 段解析：library 任意参数键值 + pages 序列（Spec 33）。
// goccy/go-yaml 需把嵌套 map 解为 WidgetDef(map[string]any)，数值/布尔类型经访问器归一。
func TestLoadSiteWidgetsSection(t *testing.T) {
	yml := `
title: t
widgets:
  library:
    recent:
      layout: recent
      limit: 10
    ghuser:
      layout: ghuser
      username: xaoxuu
      avatar: true
  pages:
    post:
      leftbar: [recent, related]
      rightbar: [ghuser, toc]
`
	path := filepath.Join(t.TempDir(), "site.yml")
	if err := os.WriteFile(path, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadSite(path)
	if err != nil {
		t.Fatalf("LoadSite: %v", err)
	}
	recent := s.Widgets.Library["recent"]
	if recent == nil {
		t.Fatal("widgets.library.recent 未解析")
	}
	if got := recent.Layout(); got != "recent" {
		t.Errorf("recent.layout = %q, want recent", got)
	}
	if got := recent.Int("limit", 0); got != 10 {
		t.Errorf("recent.limit = %d, want 10（yaml 数值类型经 Int 归一）", got)
	}
	ghuser := s.Widgets.Library["ghuser"]
	if got := ghuser.Str("username", ""); got != "xaoxuu" {
		t.Errorf("ghuser.username = %q, want xaoxuu", got)
	}
	if got := ghuser.Bool("avatar", false); !got {
		t.Errorf("ghuser.avatar = false, want true")
	}
	post, ok := s.Widgets.Pages["post"]
	if !ok {
		t.Fatal("widgets.pages.post 未解析")
	}
	if len(post.Leftbar) != 2 || post.Leftbar[0] != "recent" || post.Leftbar[1] != "related" {
		t.Errorf("post.leftbar = %v, want [recent related]", post.Leftbar)
	}
	if len(post.Rightbar) != 2 || post.Rightbar[0] != "ghuser" {
		t.Errorf("post.rightbar = %v, want [ghuser toc]", post.Rightbar)
	}
}

// TestLoadSiteWidgetsAbsent 未写 widgets 段：零值配置（BarsFor 回退内置缺省，缺省注入在 template 层单测覆盖）。
func TestLoadSiteWidgetsAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "site.yml")
	if err := os.WriteFile(path, []byte("title: t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadSite(path)
	if err != nil {
		t.Fatalf("LoadSite: %v", err)
	}
	if len(s.Widgets.Library) != 0 || len(s.Widgets.Pages) != 0 {
		t.Errorf("未配置 widgets 段应为零值，got %+v", s.Widgets)
	}
	bars := s.Widgets.BarsFor("post")
	if len(bars.Leftbar) == 0 || len(bars.Rightbar) == 0 {
		t.Errorf("零值配置 BarsFor 应回退内置缺省，got %+v", bars)
	}
}
