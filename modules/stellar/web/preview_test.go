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
		`main.css?v=stellar143-collection`,
		`class="l_left" data-ui-surface="glass"`,
		`class="l_main" id="main" data-ui-surface="content"`,
		`class="l_right" data-ui-surface="sidebar"`,
		`<div class="ui-collection" data-layout="list" data-variant="nav" data-density="compact">`,
		`<widget class="widget-wrapper recent-widget recent">`,
		`data-layout="grid" data-variant="nav"`,
		`data-layout="list" data-variant="summary"`,
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
