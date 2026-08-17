package post

import (
	"strings"
	"testing"
)

// --- Spec 51 装饰标签测试（poetry / folders）---

func TestPoetryTag(t *testing.T) {
	out := runShortcodes("{% poetry 静夜思 author:李白 date:唐 footer:录自《唐诗三百首》 %}床前明月光\n疑是地上霜{% endpoetry %}")
	for _, want := range []string{
		`class="tag-plugin poetry"`,
		`<div class="title">静夜思</div>`,
		`<div class="meta"><span>李白</span><span>唐</span></div>`,
		`<div class="body">床前明月光疑是地上霜</div>`,
		`<div class="footer">录自《唐诗三百首》</div>`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("poetry output missing %q: %s", want, out)
		}
	}
	// 无参数时 title/meta 不输出，卡片骨架仍在
	bare := runShortcodes("{% poetry %}正文{% endpoetry %}")
	if !strings.Contains(bare, `class="tag-plugin poetry"`) || strings.Contains(bare, "<div class=\"title\">") {
		t.Fatalf("poetry bare output wrong: %s", bare)
	}
}

func TestFoldersTag(t *testing.T) {
	src := "{% folders color:blue %}\n<!-- folder 第一章 -->\n甲。\n<!-- folder 第二章 -->\n乙。\n{% endfolders %}"
	out := runShortcodes(src)
	for _, want := range []string{
		`class="tag-plugin colorful folders"`,
		`color="blue"`,
		`<details class="folder" index="0">`,
		`<summary>第一章</summary>`,
		`<div class="body">甲。</div>`,
		`<details class="folder" index="1">`,
		`<summary>第二章</summary>`,
		`<div class="body">乙。</div>`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("folders output missing %q: %s", want, out)
		}
	}
}

func TestEmojiSources(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"缺省源走 QQ", "{% emoji aini %}", "emoticons@3.1.0/qq/aini.gif"},
		{"显式 qq 源", "{% emoji qq weixiao %}", "emoticons@3.1.0/qq/weixiao.gif"},
		{"tieba 源为 png", "{% emoji tieba huaji %}", "emoticons@3.1.0/tieba/huaji.png"},
		{"aru 源", "{% emoji aru aru0010 %}", "emoticons@3.1.0/aru/aru0010.gif"},
		{"blobcat 源", "{% emoji blobcat aww %}", "emoticons@3.1.0/blobcat/aww.gif"},
		{"twemoji 源", "{% emoji twemoji 1f44d %}", "twemoji@14.0.2/assets/svg/1f44d.svg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := runShortcodes(tt.in)
			if !strings.Contains(out, tt.want) {
				t.Errorf("emoji output missing %q: %s", tt.want, out)
			}
		})
	}
	// 未知来源不渲染为表情
	if out := runShortcodes("{% emoji nosuch aini %}"); strings.Contains(out, "emoticons") {
		t.Errorf("unknown source should not render: %s", out)
	}
}

// TestEmojiURL 对齐原版 scripts/tags/lib/emoji.js 的 url: 分支：
// {% emoji url:https://... [name:alt] [height:...] %}，url 非空时直接引用外部图片，
// 不再走 source 配置查找；无 url 时保持旧行为不变。
func TestEmojiURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"url 基本", "{% emoji url:https://example.com/emoji.png %}",
			`<span class="tag-plugin emoji"><img no-lazy="" class="inline" src="https://example.com/emoji.png"/>`},
		{"url+name(alt)", "{% emoji url:https://example.com/emoji.png name:alt %}",
			`src="https://example.com/emoji.png" alt="alt"`},
		{"url+height", "{% emoji url:https://example.com/emoji.png height:2em %}",
			`src="https://example.com/emoji.png" style="height:2em"`},
		{"url 含 ? 与 & 特殊字符", "{% emoji url:https://example.com/a.png?size=1&mode=2 %}",
			`src="https://example.com/a.png?size=1&amp;mode=2"`},
		{"url 分支不输出 alt（无 name）", "{% emoji url:https://example.com/emoji.png %}",
			`emoji.png"/>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := runShortcodes(tt.in)
			if !strings.Contains(out, tt.want) {
				t.Errorf("emoji url output missing %q: %s", tt.want, out)
			}
		})
	}
	// 无 alt 时不输出 alt 属性
	if out := runShortcodes("{% emoji url:https://example.com/emoji.png %}"); strings.Contains(out, `alt=`) {
		t.Errorf("emoji url without name should not output alt: %s", out)
	}
	// url 中裸 & 须转义为 &amp;，防 XSS
	if out := runShortcodes("{% emoji url:https://example.com/a.png?x=1&y=2 %}"); strings.Contains(out, `x=1&y=2"`) {
		t.Errorf("emoji url should escape ampersand: %s", out)
	}
	// url 中 < > 须转义，防 HTML 注入
	if out := runShortcodes("{% emoji url:https://example.com/<script> %}"); strings.Contains(out, `<script>`) {
		t.Errorf("emoji url should escape angle brackets: %s", out)
	}
	// height CSS 注入防御：style 值必须转义，不可引入额外 CSS 属性
	for _, tc := range []struct {
		name string
		in   string
	}{{"url 分支 height 含 background", `{% emoji url:https://e.png height:1em;background:url(https://x.png) %}`},
		{"source 分支 height 含 background", `{% emoji qq aini height:1em;background:url(https://x.png) %}`},
		{"url 分支 height 含引号", `{% emoji url:https://e.png height:1em" x="y %}`},
		{"source 分支 height 含引号", `{% emoji qq aini height:1em" x="y %}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := runShortcodes(tc.in)
			if strings.Contains(out, `background:`) {
				t.Errorf("height CSS injection: output contains unexpected background: %s", out)
			}
		})
	}

	// 回归：无 url 时旧行为不变（走 source 配置查找，不输出 alt）
	out := runShortcodes("{% emoji qq weixiao height:1.75em %}")
	for _, want := range []string{`src="https://cdn.jsdelivr.net/gh/cdn-x/emoticons@3.1.0/qq/weixiao.gif"`, `style="height:1.75em"`} {
		if !strings.Contains(out, want) {
			t.Errorf("emoji source regression missing %q: %s", want, out)
		}
	}
	if strings.Contains(out, `alt=`) {
		t.Errorf("emoji source branch should not output alt: %s", out)
	}
}
