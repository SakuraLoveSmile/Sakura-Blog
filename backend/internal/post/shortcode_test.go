package post

import (
	"strings"
	"testing"
)

// runShortcodes 便捷调用：identity renderMD + 简单 icon 桩，不依赖 goldmark/GitHub。
func runShortcodes(md string) string {
	iconFunc := func(key string) string { return `<svg data-icon="` + key + `"></svg>` }
	return processShortcodes(md, func(s string) string { return s }, func(s string) string { return s }, iconFunc, nil, nil)
}

func TestRadioAndCheckboxCoexist(t *testing.T) {
	out := runShortcodes("{% checkbox checked:true 选项A %}\n{% radio color:cyan 选项B %}")
	if !strings.Contains(out, `<input type="checkbox" checked="true"/><span>选项A</span>`) {
		t.Errorf("checkbox not rendered: %s", out)
	}
	if !strings.Contains(out, `<input type="radio"/><span>选项B</span>`) {
		t.Errorf("radio not rendered: %s", out)
	}
	if !strings.Contains(out, `color="cyan"`) {
		t.Errorf("radio color attr missing: %s", out)
	}
}

func TestRadioCheckedPositional(t *testing.T) {
	out := runShortcodes("{% radio checked 已选中 %}")
	if !strings.Contains(out, `<input type="radio" checked="true"/><span>已选中</span>`) {
		t.Errorf("radio checked not rendered: %s", out)
	}
}

func TestVoteRender(t *testing.T) {
	out := runShortcodes("{% vote id:post-1 这篇文章有用吗 %}")
	for _, want := range []string{
		`class="tag-plugin ds-vote"`,
		`data-api="/api/vote"`,
		`data-id="post-1"`,
		`<div class="header"><span>这篇文章有用吗</span></div>`,
		`<button class="vote-up"><svg data-icon="vote:thumbsup"></svg> <span class="up">0</span></button>`,
		`<button class="vote-down"><svg data-icon="vote:thumbsdown"></svg> <span class="down">0</span></button>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("vote output missing %q: %s", want, out)
		}
	}
}

func TestVoteDefaultID(t *testing.T) {
	out := runShortcodes("{% vote %}")
	if !strings.Contains(out, `data-id="default"`) {
		t.Errorf("vote default id missing: %s", out)
	}
}

func TestFriendsRender(t *testing.T) {
	// 缺省指向本地友链聚合端点
	out := runShortcodes("{% friends %}")
	if !strings.Contains(out, `<div class="tag-plugin users-wrap"><div class="data-service ds-friends" data-api="/api/friends"><div class="grid-box"></div></div></div>`) {
		t.Errorf("friends default output mismatch: %s", out)
	}
	// api 参数透传
	out = runShortcodes("{% friends api:https://example.com/data.json %}")
	if !strings.Contains(out, `data-api="https://example.com/data.json"`) {
		t.Errorf("friends api not passed through: %s", out)
	}
}

func TestSitesRender(t *testing.T) {
	out := runShortcodes("{% sites api:https://example.com/sites.json %}")
	if !strings.Contains(out, `<div class="tag-plugin sites-wrap"><div class="data-service ds-sites" data-api="https://example.com/sites.json"><div class="grid-box"></div></div></div>`) {
		t.Errorf("sites output mismatch: %s", out)
	}
	// api 缺失原样输出（语法容错）
	raw := "{% sites %}"
	if got := runShortcodes(raw); got != raw {
		t.Errorf("sites without api should stay raw, got: %s", got)
	}
}

func TestInlineLabels(t *testing.T) {
	out := runShortcodes("{% u 下划线 %} {% emp 着重 %} {% wavy 波浪 %} {% del 删除 %} {% kbd Ctrl %} {% psw 密码 %} {% blur 模糊 %}")
	for _, want := range []string{
		`<u>下划线</u>`, `<emp>着重</emp>`, `<wavy>波浪</wavy>`, `<del>删除</del>`,
		`<kbd>Ctrl</kbd>`, `<psw>密码</psw>`, `<blur>模糊</blur>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("inline label missing %q: %s", want, out)
		}
	}
}

func TestInlineSupSub(t *testing.T) {
	out := runShortcodes("{% sup color:red 上标 %} 与 {% sub 下标 %}")
	if !strings.Contains(out, `<sup class="tag-plugin colorful sup" color="red">上标</sup>`) {
		t.Errorf("sup output mismatch: %s", out)
	}
	if !strings.Contains(out, `<sub class="tag-plugin colorful sub">下标</sub>`) {
		t.Errorf("sub output mismatch: %s", out)
	}
}

func TestInlineLabelsSkipCode(t *testing.T) {
	md := "{% u 正文生效 %}\n\n```\n{% u 代码块不动 %}\n```\n\n行内 `{% kbd X %}` 也不动"
	out := runShortcodes(md)
	if !strings.Contains(out, `<u>正文生效</u>`) {
		t.Errorf("inline label in body not rendered: %s", out)
	}
	if !strings.Contains(out, "{% u 代码块不动 %}") {
		t.Errorf("fenced code should stay raw: %s", out)
	}
	if !strings.Contains(out, "`{% kbd X %}`") {
		t.Errorf("inline code should stay raw: %s", out)
	}
}

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
	if !strings.Contains(out, `class="data-service ds-mdrender" src="/notes/a.md"`) || !strings.Contains(out, `id="md_`) {
		t.Fatalf("md output wrong: %s", out)
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
// default/qq/tieba/aru/blobcat/twemoji，未知来源原样保留。
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
