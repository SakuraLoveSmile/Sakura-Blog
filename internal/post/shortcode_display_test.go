package post

import (
	"strings"
	"testing"
)

// --- Spec 31 展示类标签测试（输出结构对照原版 scripts/tags/lib）---

func TestAboutTag(t *testing.T) {
	out := runShortcodes("{% about avatar:/a.png height:80px %}**hi**{% endabout %}")
	for _, want := range []string{`class="tag-plugin about"`, `about-header`, `src="/a.png"`, `height="80px"`, `about-body fs14`} {
		if !strings.Contains(out, want) {
			t.Fatalf("about output missing %q: %s", want, out)
		}
	}
}

func TestMbtiTag(t *testing.T) {
	out := runShortcodes("{% mbti INTJ-T 93-72-78-83-51 %}")
	for _, want := range []string{`class="tag-plugin colorful note mbti"`, `color="purple"`, `mbti-title`, `INTJ`, `建筑师`, `mbti-trait`, `mbti-progress`} {
		if !strings.Contains(out, want) {
			t.Fatalf("mbti output missing %q: %s", want, out)
		}
	}
}

func TestBlockquoteTag(t *testing.T) {
	out := runShortcodes("{% blockquote indent:true %}text{% endblockquote %}")
	if !strings.Contains(out, `class="tag-plugin blockquote" indent="true"`) {
		t.Fatalf("blockquote output wrong: %s", out)
	}
}

func TestNavbarTag(t *testing.T) {
	out := runShortcodes("{% navbar active:/about/ [Home](/) [About](/about/) %}")
	for _, want := range []string{`class="tag-plugin navbar"`, `<a class="link" href="/">Home</a>`, `<a class="link active" href="/about/">About</a>`} {
		if !strings.Contains(out, want) {
			t.Fatalf("navbar output missing %q: %s", want, out)
		}
	}
}

func TestBannerTagExtractsNavbar(t *testing.T) {
	src := "{% banner 标题 副标题 bg:/bg.png link:https://x.com %}\n{% navbar [Home](/) %}\n{% endbanner %}"
	out := runShortcodes(src)
	for _, want := range []string{`class="tag-plugin banner"`, `style="background-image:url(/bg.png)"`, `tag-plugin navbar`, `<div class="title">标题</div>`, `<div class="subtitle">副标题</div>`, `<a class="banner-link" href="https://x.com">`} {
		if !strings.Contains(out, want) {
			t.Fatalf("banner output missing %q: %s", want, out)
		}
	}
	if strings.Contains(out, "window.history.back") {
		t.Fatalf("banner with link should not render back button: %s", out)
	}
}

func TestFrameTag(t *testing.T) {
	out := runShortcodes("{% frame iphone11 img:/p.png 示例 %}")
	for _, want := range []string{`class="tag-plugin img-wrap"`, `id="iphone11"`, `class="img" src="/p.png"`, `image-caption`} {
		if !strings.Contains(out, want) {
			t.Fatalf("frame output missing %q: %s", want, out)
		}
	}
	raw := "{% frame img:/p.png %}"
	if got := runShortcodes(raw); got != raw {
		t.Fatalf("frame without device should stay raw, got %s", got)
	}
}

func TestMDTag(t *testing.T) {
	out := runShortcodes("{% md /notes/a.md %}")
	if !strings.Contains(out, `class="data-service ds-mdrender" src="/notes/a.md"`) || !strings.Contains(out, `data-heading="true"`) || !strings.Contains(out, `id="md_`) {
		t.Fatalf("md output wrong: %s", out)
	}
	replaced := runShortcodes(`{% md https://example.com/readme.md wrap:false %}`)
	if !strings.Contains(replaced, `data-replace="true"`) {
		t.Fatalf("md wrap:false should replace placeholder: %s", replaced)
	}
	wrapped := runShortcodes(`{% md https://example.com/readme.md wrap:true %}`)
	if strings.Contains(wrapped, `data-replace="true"`) {
		t.Fatalf("md wrap:true should retain placeholder: %s", wrapped)
	}
}

func TestTableTag(t *testing.T) {
	out := runShortcodes("{% table style:compact %}\n| A | B |\n| - | - |\n| 1 | 2 |\n{% endtable %}")
	if !strings.Contains(out, `class="tag-plugin table table-compact"`) || !strings.Contains(out, "| A | B |") {
		t.Fatalf("table output wrong: %s", out)
	}
	if got := runShortcodes("{% table style:unknown %}body{% endtable %}"); !strings.Contains(got, `table-scroll`) {
		t.Fatalf("unknown table style should fall back to scroll: %s", got)
	}
}

func TestTipTag(t *testing.T) {
	out := runShortcodes("{% tip text:说明文字 %}**正文**{% endtip %}")
	for _, want := range []string{`class="tag-plugin tip"`, `class="tip-text"`, `class="tip-bubble"`, `说明文字`} {
		if !strings.Contains(out, want) {
			t.Fatalf("tip output missing %q: %s", want, out)
		}
	}
}

func TestTimelineTitleStaysLiteral(t *testing.T) {
	out := runShortcodes("{% timeline %}\n<!-- node **版本 1** -->\n正文\n{% endtimeline %}")
	if !strings.Contains(out, `<span>**版本 1**</span>`) || strings.Contains(out, "<strong>版本 1</strong>") {
		t.Fatalf("timeline title must not parse Markdown: %s", out)
	}
}

func TestTOCTag(t *testing.T) {
	// wiki 参数命中：本项目 wiki 树未建（Spec 35），对齐原版 proj 缺失行为输出空
	if got := runShortcodes("{% toc wiki:stellar %}"); got != "" {
		t.Fatalf("toc with wiki should render empty, got %q", got)
	}
	out := runShortcodes("{% toc 目录 open:true %}")
	for _, want := range []string{`class="tag-plugin toc"`, `<details class="toc" open>`, `<summary>目录</summary>`} {
		if !strings.Contains(out, want) {
			t.Fatalf("toc output missing %q: %s", want, out)
		}
	}
}

func TestAlbumsPostersTag(t *testing.T) {
	out := runShortcodes("{% albums repo:owner/repo size:m %}")
	for _, want := range []string{`class="tag-plugin albums-wrap"`, `ds-friends`, `data-api="https://raw.githubusercontent.com/owner/repo/output/v2/data.json"`, `size="m"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("albums output missing %q: %s", want, out)
		}
	}
	out2 := runShortcodes("{% posters api:https://e.com/d.json %}")
	for _, want := range []string{`class="tag-plugin posters-wrap"`, `data-api="https://e.com/d.json"`, `size="xs"`} {
		if !strings.Contains(out2, want) {
			t.Fatalf("posters output missing %q: %s", want, out2)
		}
	}
}

func TestReelTag(t *testing.T) {
	out := runShortcodes("{% reel 标题 author:作者 date:2026 footer:完 %}正文{% endreel %}")
	for _, want := range []string{`class="tag-plugin reel"`, `<div class="title">标题</div>`, `<div class="meta"><span>作者</span></div>`, `<div class="date">2026</div>`, `<div class="footer">完</div>`} {
		if !strings.Contains(out, want) {
			t.Fatalf("reel output missing %q: %s", want, out)
		}
	}
}

func TestPaperTag(t *testing.T) {
	src := "{% paper style:underline title:标题 author:作者 %}\n<!-- section 第一节 -->\n甲\n<!-- paragraph -->\n乙\n<!-- line right -->\n丙\n{% endpaper %}"
	out := runShortcodes(src)
	for _, want := range []string{`class="tag-plugin paper"`, `class="content underline"`, `<div class="section-title">第一节</div>`, `class="paragraph"`, `class="line right"`, `<span class="author">作者</span>`} {
		if !strings.Contains(out, want) {
			t.Fatalf("paper output missing %q: %s", want, out)
		}
	}
}

// --- Spec 49/块级设备框架与 note 行内 Markdown 测试 ---
// 这两个用例必须走 Service.Render 真实 goldmark 管线：
// runShortcodes 的 identity renderMD 无法把 ![alt](url) 转成 <img>、无法把 **bold** 转成 <strong>。

// TestFrameBlockTag 块级设备框架（Spec 49）：body 的 ![alt](url) 经 renderInlineMD 渲染成 <img> 后嵌入设备壳。
func TestFrameBlockTag(t *testing.T) {
	s := NewService("")
	out, err := s.Render("{% frame device:iphone width:320px %}![img](url){% endframe %}")
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	for _, want := range []string{
		`class="tag-plugin frame"`,
		`device="iphone"`,
		`max-width:320px`,
		`class="frame-device iphone"`,
		`<img`,
		`src="url"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("frame block output missing %q: %s", want, out)
		}
	}
	// body 无媒体时容错原样输出（不吞内容）
	raw := "{% frame device:iphone %}no media{% endframe %}"
	if got, _ := s.Render(raw); !strings.Contains(got, "{% frame device:iphone %}") {
		t.Fatalf("frame without media should stay raw, got %s", got)
	}
}

// TestNoteMarkdownBody note 标签 body 走 renderInlineMD：**bold** 应渲染成 <strong>（回归保护）。
func TestNoteMarkdownBody(t *testing.T) {
	s := NewService("")
	out, err := s.Render("{% note color:blue **bold** %}")
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	for _, want := range []string{
		`class="tag-plugin colorful note"`,
		`color="blue"`,
		`<div class="body">`,
		`<strong>bold</strong>`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("note output missing %q: %s", want, out)
		}
	}
}

// TestGalleryImgAttrs gallery 图片同时输出 src 与 data-src（懒加载回退），并带 data-fancybox 分组（回归保护）。
func TestGalleryImgAttrs(t *testing.T) {
	s := NewService("")
	out, err := s.Render("{% gallery %}\n![img](https://example.com/test.jpg)\n{% endgallery %}")
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	for _, want := range []string{
		`src="https://example.com/test.jpg"`,
		`data-src="https://example.com/test.jpg"`,
		`data-fancybox="gallery-1"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("gallery output missing %q: %s", want, out)
		}
	}
}

// TestEmojiSources 对齐原版 _config.yml tag_plugins.emoji 的五种来源：
