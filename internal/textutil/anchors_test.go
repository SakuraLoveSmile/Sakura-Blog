package textutil

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestStripHTMLWithAnchors 章节锚点提取：偏移与文本、无标题文档与 StripHTML 一致。
func TestStripHTMLWithAnchors(t *testing.T) {
	t.Run("锚点 offset 与文本", func(t *testing.T) {
		src := "<p>intro text</p><h2 id=\"go\">Go 语言</h2><p>正文的 Go 内容</p><h3 id=\"sub\">子节</h3><p>更多</p>"
		text, anchors := StripHTMLWithAnchors(src)
		want := "intro text Go 语言 正文的 Go 内容 子节 更多"
		if text != want {
			t.Fatalf("text = %q, want %q", text, want)
		}
		if len(anchors) != 2 {
			t.Fatalf("anchors = %+v", anchors)
		}
		if anchors[0].ID != "go" || anchors[0].Text != "Go 语言" {
			t.Fatalf("anchor0 = %+v", anchors[0])
		}
		if got := indexUTF16(text, anchors[0].Offset); !strings.HasPrefix(text[got:], "Go 语言") {
			t.Fatalf("offset0 应指向标题首字符，got %d", anchors[0].Offset)
		}
		if anchors[1].ID != "sub" || anchors[1].Text != "子节" {
			t.Fatalf("anchor1 = %+v", anchors[1])
		}
	})

	t.Run("无标题与 StripHTML 一致", func(t *testing.T) {
		src := "  <p>a <b>b</b>  c</p>\n<p>d</p> "
		text, anchors := StripHTMLWithAnchors(src)
		if want := StripHTML(src); text != want {
			t.Fatalf("text = %q, StripHTML = %q", text, want)
		}
		if len(anchors) != 0 {
			t.Fatalf("anchors = %+v", anchors)
		}
	})

	t.Run("无 id 标题也记录（文本锚点）", func(t *testing.T) {
		_, anchors := StripHTMLWithAnchors("<h2>纯标题</h2>正文")
		if len(anchors) != 1 || anchors[0].ID != "" || anchors[0].Text != "纯标题" {
			t.Fatalf("anchors = %+v", anchors)
		}
	})
}

// indexUTF16 测试辅助：UTF-16 偏移 → 字节偏移。
func indexUTF16(s string, u16 int) int {
	n, i := 0, 0
	for i < len(s) {
		_, size := utf8.DecodeRuneInString(s[i:])
		if n >= u16 {
			return i
		}
		if size == 4 {
			n += 2
		} else {
			n++
		}
		i += size
	}
	return i
}
