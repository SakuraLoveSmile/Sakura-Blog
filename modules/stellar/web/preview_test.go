package web

import (
	"os"
	"strings"
	"testing"
)

func TestCollectionPreviewContract(t *testing.T) {
	html, err := os.ReadFile("build/preview.html")
	if err != nil {
		t.Fatalf("read preview: %v", err)
	}
	body := string(html)
	for _, want := range []string{
		`main.css?v=stellar200-collection`,
		`class="site-shell" id="start" data-regions="leftbar rightbar"`,
		`<div class="site-workspace">`,
		`class="site-region__surface ui-surface"`,
		`class="site-region__viewport"`,
		`class="brand-wrap"`,
		`class="widget-instance widget-instance--brand"`,
		`class="widgets widget-stack widget-stack--leftbar widget-stack--body"`,
		`class="widget-instance widget-instance--recent"`,
		`class="widgets widget-stack widget-stack--rightbar"`,
		`class="ui-collection__item ui-interactive`,
		`<div class="ui-collection" data-layout="list" data-variant="nav" data-density="compact">`,
		`<widget class="widget-wrapper recent-widget recent">`,
		`data-layout="grid" data-variant="nav"`,
		`data-layout="list" data-variant="summary"`,
		`Stellar 2.0.0 Collection`,
		`2.0.0 · 9d11978a`,
		`Collection 组件概览`,
		`两行说明用于检查文字层级、截断和 content surface 下的卡片填充。`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("preview missing %q", want)
		}
	}
	for _, legacy := range []string{"post-list", `class="item title"`, `class="link"`, `widget-wrapper post-card`} {
		if strings.Contains(body, legacy) {
			t.Errorf("preview still contains legacy collection markup %q", legacy)
		}
	}
}
