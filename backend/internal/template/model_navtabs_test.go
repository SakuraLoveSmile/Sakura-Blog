package template

import (
	"testing"

	"github.com/goccy/go-yaml"
)

// TestNavTabsUnmarshalOldShape 旧形状（sequence）向后兼容：nav_tabs: [{title,url,external}]。
func TestNavTabsUnmarshalOldShape(t *testing.T) {
	src := `
nav_tabs:
  - title: 友链
    url: /friends/
    external: true
  - title: GitHub
    url: https://github.com/example
`
	var s Site
	if err := yaml.Unmarshal([]byte(src), &s); err != nil {
		t.Fatalf("旧形状解析失败: %v", err)
	}
	if s.NavTabs.EnableCustom {
		t.Errorf("旧形状 enable_custom 应为 false")
	}
	if len(s.NavTabs.Items) != 2 {
		t.Fatalf("旧形状 items 应为 2，got %d", len(s.NavTabs.Items))
	}
	first := s.NavTabs.Items[0]
	if first.Title != "友链" || first.URL != "/friends/" || !first.External {
		t.Errorf("旧形状第 1 项解析错误: %+v", first)
	}
	if s.NavTabs.Items[1].Title != "GitHub" || s.NavTabs.Items[1].External {
		t.Errorf("旧形状第 2 项解析错误: %+v", s.NavTabs.Items[1])
	}
}

// TestNavTabsUnmarshalNewShape 新形状（mapping）：nav_tabs: {enable_custom, items}（Spec 50）。
func TestNavTabsUnmarshalNewShape(t *testing.T) {
	src := `
nav_tabs:
  enable_custom: true
  items:
    - title: 友链
      url: /friends/
      icon: solar:planet-bold-duotone
      active_rules: ['/friends/']
    - title: 技术
      url: /categories/技术/
      active_rules: ['^/categories/']
`
	var s Site
	if err := yaml.Unmarshal([]byte(src), &s); err != nil {
		t.Fatalf("新形状解析失败: %v", err)
	}
	if !s.NavTabs.EnableCustom {
		t.Errorf("新形状 enable_custom 应为 true")
	}
	if len(s.NavTabs.Items) != 2 {
		t.Fatalf("新形状 items 应为 2，got %d", len(s.NavTabs.Items))
	}
	first := s.NavTabs.Items[0]
	if first.Title != "友链" || first.Icon != "solar:planet-bold-duotone" {
		t.Errorf("新形状第 1 项 icon 解析错误: %+v", first)
	}
	if len(first.ActiveRules) != 1 || first.ActiveRules[0] != "/friends/" {
		t.Errorf("新形状第 1 项 active_rules 解析错误: %+v", first.ActiveRules)
	}
	if len(s.NavTabs.Items[1].ActiveRules) != 1 {
		t.Errorf("新形状第 2 项 active_rules 解析错误: %+v", s.NavTabs.Items[1].ActiveRules)
	}
}

// TestNavTabIsActive 高亮规则：URL 精确匹配缺省 / 前缀 / 正则 / 空 Path 安全。
func TestNavTabIsActive(t *testing.T) {
	exact := NavTabItem{URL: "/friends/"}
	if !exact.IsActive(&Page{Path: "/friends/"}) {
		t.Errorf("URL 精确匹配应高亮")
	}
	if exact.IsActive(&Page{Path: "/friends/2"}) {
		t.Errorf("URL 精确匹配不应命中子路径")
	}
	if exact.IsActive(nil) || exact.IsActive(&Page{}) {
		t.Errorf("nil/空 Path 应返回 false")
	}

	prefix := NavTabItem{URL: "/categories/", ActiveRules: []string{"/categories/"}}
	if !prefix.IsActive(&Page{Path: "/categories/技术/"}) {
		t.Errorf("前缀规则应命中 /categories/技术/")
	}
	if prefix.IsActive(&Page{Path: "/categoriesx"}) {
		t.Errorf("前缀规则不应命中 /categoriesx")
	}

	regex := NavTabItem{URL: "/x", ActiveRules: []string{"^/post/\\d+"}}
	if !regex.IsActive(&Page{Path: "/post/123"}) {
		t.Errorf("正则规则应命中 /post/123")
	}
	if regex.IsActive(&Page{Path: "/post/abc"}) {
		t.Errorf("正则规则不应命中 /post/abc")
	}

	// 非法正则回退为前缀比较（不 panic）
	bad := NavTabItem{URL: "/x", ActiveRules: []string{"[invalid"}}
	if !bad.IsActive(&Page{Path: "[invalid"}) {
		t.Errorf("非法正则按前缀比较应命中同串前缀")
	}
	if bad.IsActive(&Page{Path: "/other"}) {
		t.Errorf("非法正则不应命中无关路径")
	}

	// 多规则任一命中
	multi := NavTabItem{URL: "/x", ActiveRules: []string{"/a/", "/b/"}}
	if !multi.IsActive(&Page{Path: "/b/deep"}) {
		t.Errorf("多规则任一命中即高亮")
	}
	if multi.IsActive(&Page{Path: "/c/"}) {
		t.Errorf("多规则全部不命中应返回 false")
	}
}
