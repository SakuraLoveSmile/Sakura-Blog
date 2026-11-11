package friends

import (
	"sync/atomic"
	"testing"

	tmpl "blog/internal/settings"
)

// TestBuildBaseCollectsOrderedLabels 首枚来自 label/color，附加角标按配置顺序追加。
func TestBuildBaseCollectsOrderedLabels(t *testing.T) {
	sitePtr := &atomic.Pointer[tmpl.Site]{}
	sitePtr.Store(&tmpl.Site{Links: []tmpl.LinkGroup{{Items: []tmpl.LinkItem{{
		Name: "杜老师说", URL: "https://example.com", Label: "社区先锋", Color: "D7EDE8",
		Labels: []tmpl.LinkLabel{{Name: "老友", Color: "30B0C7"}, {Name: "朋友", Color: "#007AFF"}},
	}}}}})
	a := New(sitePtr)

	friends := a.buildBase()
	if len(friends) != 1 {
		t.Fatalf("friends = %d, want 1", len(friends))
	}
	labels := friends[0].Labels
	if len(labels) != 3 {
		t.Fatalf("labels = %+v, want 3", labels)
	}
	want := []struct{ name, color string }{{"社区先锋", "d7ede8"}, {"老友", "30b0c7"}, {"朋友", "007aff"}}
	for i, w := range want {
		if labels[i].Name != w.name || labels[i].Color != w.color {
			t.Fatalf("label %d = %+v, want %+v", i, labels[i], w)
		}
	}
	// 浅色角标应得到高亮度，供前端选择深色文字。
	if labels[0].Lightness <= 75 {
		t.Fatalf("light label lightness = %d, want > 75", labels[0].Lightness)
	}
}

// TestBuildBaseCapsLabels 配置里塞入超过上限的角标时只保留前三枚。
func TestBuildBaseCapsLabels(t *testing.T) {
	sitePtr := &atomic.Pointer[tmpl.Site]{}
	sitePtr.Store(&tmpl.Site{Links: []tmpl.LinkGroup{{Items: []tmpl.LinkItem{{
		Name: "超量", URL: "https://example.com", Label: "一", Color: "007AFF",
		Labels: []tmpl.LinkLabel{{Name: "二"}, {Name: "三"}, {Name: "四"}},
	}}}}})
	a := New(sitePtr)

	labels := a.buildBase()[0].Labels
	if len(labels) != 3 {
		t.Fatalf("labels = %d, want 3", len(labels))
	}
	if labels[2].Name != "三" {
		t.Fatalf("labels = %+v, want first three kept in order", labels)
	}
}
