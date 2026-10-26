package post

import (
	nethtml "golang.org/x/net/html"
	"strings"
	"testing"
)

func TestStellarV2MarkerBlocksKeepNestedBoundaries(t *testing.T) {
	cases := []struct {
		name, source, container, item string
		items                         int
	}{
		{"tabs", "{% tabs %}\n<!-- tab Outer -->\n{% tabs %}\n<!-- tab Inner -->\nInner body\n<!-- tab Inner second -->\nSecond inner body\n{% endtabs %}\n<!-- tab Outer second -->\nSecond outer body\n{% endtabs %}", `class="tag-plugin tabs"`, `class="tab-pane`, 4},
		{"folders", "{% folders %}\n<!-- folder Outer -->\n{% folders %}\n<!-- folder Inner -->\nInner body\n<!-- folder Inner second -->\nSecond inner body\n{% endfolders %}\n<!-- folder Outer second -->\nSecond outer body\n{% endfolders %}", `class="tag-plugin colorful folders"`, `class="folder"`, 4},
		{"grid", "{% grid %}\n<!-- cell -->\n{% grid %}\n<!-- cell -->\nInner body\n<!-- cell -->\nSecond inner body\n{% endgrid %}\n<!-- cell -->\nSecond outer body\n{% endgrid %}", `class="tag-plugin grid"`, `class="cell"`, 4},
		{"timeline", "{% timeline %}\n<!-- node Outer -->\n{% timeline %}\n<!-- node Inner -->\nInner body\n<!-- node Inner second -->\nSecond inner body\n{% endtimeline %}\n<!-- node Outer second -->\nSecond outer body\n{% endtimeline %}", `class="tag-plugin timeline"`, `class="timenode"`, 4},
		{"paper", "{% paper %}\n<!-- section Outer -->\n{% paper %}\n<!-- section Inner -->\nInner body\n<!-- section Inner second -->\nSecond inner body\n{% endpaper %}\n<!-- section Outer second -->\nSecond outer body\n{% endpaper %}", `class="tag-plugin paper"`, `class="section"`, 4},
		{"okr", "{% okr o1 %}\nOuter objective\n<!-- okr kr1 percent:50 -->\nOuter result\n{% okr o2 %}\nInner objective\n<!-- okr kr2 percent:25 -->\nInner body\n{% endokr %}\n<!-- okr kr3 percent:100 -->\nSecond outer body\n{% endokr %}", `class="tag-plugin colorful okr"`, `class="okr-item kr"`, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := NewService("").RenderStellarV2WithContext(c.source, StellarRenderContext{})
			if err != nil {
				t.Fatalf("valid nested %s must render: %v", c.name, err)
			}
			if strings.Count(got, c.container) != 2 || strings.Count(got, c.item) != c.items {
				t.Fatalf("outer block consumed nested markers: %s", got)
			}
			assertStellarV2NestedMarkerDOM(t, got, strings.TrimSuffix(strings.TrimPrefix(c.container, `class="`), `"`))
			for _, text := range []string{"Inner body", "Second outer body"} {
				if !strings.Contains(got, text) {
					t.Fatalf("nested content %q disappeared: %s", text, got)
				}
			}
		})
	}
}

func TestStellarV2MarkersInsideCodeRemainExamples(t *testing.T) {
	for _, code := range []string{
		"```html\n<!-- tab Example -->\n```",
		"~~~html\n<!-- tab Example -->\n~~~",
		"`<!-- tab Example -->`",
		"``<!-- tab Example -->``",
		"    <!-- tab Example -->",
	} {
		t.Run(code, func(t *testing.T) {
			source := "{% tabs %}\n<!-- tab Actual -->\n\n" + code + "\n\nAfter example\n{% endtabs %}"
			got, err := NewService("").RenderStellarV2(source, nil)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(got, `class="tab-pane`) != 1 || !strings.Contains(got, "Example") || !strings.Contains(got, "After example") {
				t.Fatalf("code marker changed tabs or discarded code: %s", got)
			}
		})
	}
}

func TestStellarV2MarkerInsideInlineTagRemainsArgument(t *testing.T) {
	source := "{% tabs %}\n<!-- tab Actual -->\n{% note Title '<!-- tab Example -->' %}\n{% endtabs %}"
	got, err := NewService("").RenderStellarV2(source, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(got, `class="tab-pane`) != 1 || !strings.Contains(got, `class="tag-plugin colorful note"`) || !strings.Contains(got, "<!-- tab Example -->") {
		t.Fatalf("inline shortcode argument became an outer marker: %s", got)
	}
}

func TestStellarV2MarkerInsideDifferentBlockRemainsInChild(t *testing.T) {
	source := "{% tabs %}\n<!-- tab Outer -->\n{% box Child %}\n<!-- tab Child comment -->\nChild body\n{% endbox %}\n<!-- tab Second -->\nSecond outer body\n{% endtabs %}"
	got, err := NewService("").RenderStellarV2(source, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(got, `class="tab-pane`) != 2 || !strings.Contains(got, "<!-- tab Child comment -->") || !strings.Contains(got, "Child body") || !strings.Contains(got, "Second outer body") {
		t.Fatalf("outer tabs consumed the child block's comment: %s", got)
	}
}

func TestStellarV2ChatCodeMarkersRemainExamples(t *testing.T) {
	for _, code := range []string{
		"```html\n<!-- chatcell user:alice md:Example -->\n```",
		"~~~html\n<!-- chatcell user:alice md:Example -->\n~~~",
		"`<!-- chatcell user:alice md:Example -->`",
		"    <!-- chatcell user:alice md:Example -->",
	} {
		t.Run(code, func(t *testing.T) {
			source := "{% chat title:Chat %}\nalice:\n  name: Alice\n<!-- chatcell user:alice md:Actual -->\n" + code + "\n<!-- chatcell user:alice md:After example -->\n{% endchat %}"
			got, err := NewService("").RenderStellarV2(source, nil)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(got, `class="chatcell`) != 2 || !strings.Contains(got, "Actual") || !strings.Contains(got, "After example") || strings.Contains(got, ">Example<") {
				t.Fatalf("chat activated a code example: %s", got)
			}
		})
	}
}

// 检查真实 DOM 归属，避免只有容器数量正确但内容已被 HTML 解析器移到错误层级。
func assertStellarV2NestedMarkerDOM(t *testing.T, source, class string) {
	t.Helper()
	doc, err := nethtml.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	matches := func(n *nethtml.Node) bool {
		if n.Type != nethtml.ElementNode {
			return false
		}
		for _, a := range n.Attr {
			if a.Key == "class" && a.Val == class {
				return true
			}
		}
		return false
	}
	var containers []*nethtml.Node
	owners := map[string]*nethtml.Node{}
	var visit func(*nethtml.Node, *nethtml.Node)
	visit = func(n, owner *nethtml.Node) {
		if matches(n) {
			containers = append(containers, n)
			owner = n
		}
		if n.Type == nethtml.TextNode {
			for _, text := range []string{"Inner body", "Second outer body"} {
				if strings.Contains(n.Data, text) {
					owners[text] = owner
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c, owner)
		}
	}
	visit(doc, nil)
	if len(containers) != 2 || owners["Inner body"] != containers[1] || owners["Second outer body"] != containers[0] {
		t.Fatalf("nested content escaped its owning %s container: %s", class, source)
	}
	for p := containers[1].Parent; p != nil; p = p.Parent {
		if p == containers[0] {
			return
		}
	}
	t.Fatalf("nested %s container became a sibling: %s", class, source)
}
