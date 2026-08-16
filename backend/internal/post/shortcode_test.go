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

// TestShortcodeXSSMitigation 安全审计 2026-08-16 回归：用户可控的 shortcode 参数
// 必须经转义/白名单后再进入 HTML 输出，以下断言针对本次新增校验（转义后的期望值）。
func TestShortcodeXSSMitigation(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string // 期望出现
		bad  string // 期望绝不出现（未转义/未拦截的漏洞特征）
	}{
		{"note color 属性注入被丢弃", `{% note color:red"><script>alert(1)</script> 标题 内容 %}`,
			`class="tag-plugin colorful note"`, `<script>alert(1)</script>`},
		{"note title 文本转义", `{% note <b>标题</b> 内容 %}`,
			`<div class="title">&lt;b&gt;标题&lt;/b&gt;</div>`, `<div class="title"><b>`},
		{"folding title 转义", `{% folding open:true <img src=x> %}内容{% endfolding %}`,
			`<summary>&lt;img src=x&gt;</summary>`, `<summary><img`},
		{"[[text]] 快捷语法转义", `[[<script>alert(1)</script>]]`,
			`<u class="p tag-plugin">&lt;script&gt;`, `<u class="p tag-plugin"><script>`},
		{"{color|text} text 转义", `{red|<b>红</b>}`,
			`style="color:red">&lt;b&gt;红&lt;/b&gt;`, `color:red"><b>`},
		// scheme 非法时 button/link 不渲染（原样保留 shortcode 源文本），断言按钮卡片确实未生成
		{"button javascript: 链接不渲染", `{% button 点我 javascript:alert(1) %}`,
			`{% button`, `class="tag-plugin colorful button"`},
		{"link data: 链接不渲染", `{% link data:text/html,base64 标题 %}`,
			`{% link`, `tag-plugin link`},
		{"hashtag href javascript: 丢弃", `{% hashtag 话题 javascript:alert(1) %}`,
			`<span>话题</span>`, `href="javascript:`},
		{"checkbox text 转义", `{% checkbox <b>选项</b> %}`,
			`<span>&lt;b&gt;选项&lt;/b&gt;</span>`, `<span><b>`},
		{"quot el 白名单回退 p", `{% quot el:script/onload=alert(1) 文本 %}`,
			`<div class="tag-plugin quot p"><p class="content">`, `<script`},
		{"quot id 属性与文本转义", `{% quot el:h2 文本"><b>x</b> %}`,
			`id="文本&gt;&lt;b&gt;x&lt;/b&gt;"`, `id="文本">`},
		{"timeline header 转义", "{% timeline %}\n<!-- node <b>节点</b> -->\n正文\n{% endtimeline %}",
			`<span>&lt;b&gt;节点&lt;/b&gt;</span>`, `<span><b>`},
		{"tabs header 转义", "{% tabs %}\n<!-- tab 标签\"><script> -->\n内容\n{% endtabs %}",
			`&lt;script&gt;`, `<script>`},
		// CSS 数值整体非法时丢弃整个参数（不截取合法前缀），回退默认布局
		{"image width CSS 注入丢弃", `{% image /a.png alt width:100px;background:url(x) %}`,
			`class="image-bg">`, `background:`},
		{"image bg 非 CSS 色丢弃", `{% image /a.png alt bg:red;}body{display:none %}`,
			`class="image-bg">`, `display:none`},
		{"image download javascript: 不渲染", `{% image /a.png alt download:javascript:alert(1) %}`,
			`class="tag-plugin image"`, `image-download`},
		{"grid gap CSS 注入丢弃", `{% grid gap:8px;color:red %}单元{% endgrid %}`,
			`minmax(240px, 1fr)`, `color:red`},
		// splitArgs 会把参数中的裸引号作为引号定界符消费，故 child 值为 `><script> 标题`（引号已剥离），转义后安全
		{"box child 属性转义", `{% box child:"><script> 标题 %}内容{% endbox %}`,
			`child="&gt;&lt;script&gt; 标题"`, `<script>`},
		{"mark text 转义", `{% mark <b>高亮</b> %}`,
			`&lt;b&gt;高亮&lt;/b&gt;`, `<mark class="tag-plugin colorful mark" color="yellow"><b>`},
		{"okr color 白名单", "{% okr o1 color:red\"><script> %}\n目标\n<!-- okr kr1 percent:50 -->\nKR\n{% endokr %}",
			`class="tag-plugin colorful okr"`, `<script>`},
		{"okr index 注入转义", "{% okr o1\"><b> %}\n目标\n<!-- okr kr1 percent:50 -->\nKR\n{% endokr %}",
			`id="okr-o1&gt;&lt;b&gt;"`, `id="okr-o1"><b>`},
		{"icon key 白名单放行合法 key", `{% icon solar:hashtag-square-bold %}`,
			`<svg data-icon="solar:hashtag-square-bold"></svg>`, ``},
		{"icon key 恶意字符降级", `{% icon key"><script> %}`,
			`class="tag-plugin icon colorful"`, `<script>`},
		{"sup color 注入整体丢弃", `{% sup color:red;body{x:1 上标 %}`,
			`colorful sup">上标`, `body{x:1`},
		{"inline label text 转义", `{% u <b>下划</b> %}`,
			`<u>&lt;b&gt;下划&lt;/b&gt;</u>`, `<u><b>`},
		// chat label 颜色非法时回退默认 white/#2196f3，不截取前缀（YAML 缩进：name>=2、label 子键>=4）
		{"chat label 颜色注入回退默认", "{% chat iphone style:qq title:t %}\nalice:\n  name: 小明\n  label:\n    text: 管理员\n    textColor: red;}body{x:1\n<!-- chatcell user:alice md:hi -->\n{% endchat %}",
			`style="color:white;background:#2196f3;"`, `body{x:1`},
		{"chat style class 词白名单", `{% chat iphone style:qq" onclick=x %}\n<!-- chatcell user:a md:hi -->\n{% endchat %}`,
			`chat iphone qq group`, `onclick=x`},
		{"mbti url javascript: 降级", `{% mbti INTJ-T 93-72-78-83-51 url:javascript:alert(1) %}`,
			`mbti-title-text`, `mbti-title-link`},
		{"reel footer 转义", `{% reel 标题 footer:<script> %}正文{% endreel %}`,
			`<div class="footer">&lt;script&gt;</div>`, `<div class="footer"><script>`},
		{"paper section 标题转义", `{% paper %}\n<!-- section <b>节</b> -->\n甲\n{% endpaper %}`,
			`<div class="section-title">&lt;b&gt;节&lt;/b&gt;</div>`, `<div class="section-title"><b>`},
		{"navbar href scheme 校验", `{% navbar [x](javascript:alert(1)) %}`,
			`<a class="link" href="#">x</a>`, `javascript:`},
		{"banner bg scheme 校验", `{% banner title:t bg:javascript:alert(1) %}b{% endbanner %}`,
			`class="tag-plugin banner">`, `background-image:url(javascript`},
		{"frame alt 文本转义", `{% frame iphone11 img:/p.png <b>示例</b> %}`,
			`<span class="image-caption">&lt;b&gt;示例&lt;/b&gt;</span>`, `<span class="image-caption"><b>`},
		{"toc title 转义", `{% toc <b>目录</b> %}`,
			`<summary>&lt;b&gt;目录&lt;/b&gt;</summary>`, `<summary><b>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := runShortcodes(tc.in)
			if tc.bad != "" && strings.Contains(out, tc.bad) {
				t.Errorf("XSS leak: output contains %q: %s", tc.bad, out)
			}
			if tc.want != "" && !strings.Contains(out, tc.want) {
				t.Errorf("expected %q missing in: %s", tc.want, out)
			}
		})
	}
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

// ---------- Spec 63：组件库预览 Fixture ----------

func TestRewritePreviewDataAPIs(t *testing.T) {
	// 覆盖：rating/vote 端点改写 + data-id 隔离前缀；friends/sites 外部 URL 一律指向本地 Fixture；
	// md src 编码进查询参数（来源标识，fixture 端点不直连）；非 preview 产物不受影响。
	in := `<div class="tag-plugin ds-rating" data-api="/api/rating" data-id="demo-post"><div class="body"></div></div>` +
		`<div class="tag-plugin ds-vote" data-api="/api/vote" data-id="demo-post"></div>` +
		`<div class="tag-plugin users-wrap"><div class="data-service ds-friends" data-api="https://example.com/data.json"><div class="grid-box"></div></div></div>` +
		`<div class="tag-plugin sites-wrap"><div class="data-service ds-sites" data-api="https://example.com/sites.json"><div class="grid-box"></div></div></div>` +
		`<div class="data-service ds-mdrender" src="https://raw.githubusercontent.com/x/y/main/README.md" id="md_1"></div>`
	out := rewritePreviewDataAPIs(in)
	for _, want := range []string{
		`data-api="/api/v1/preview/rating"`,
		`data-id="preview-demo-post"`,
		`data-api="/api/v1/preview/vote"`,
		`data-api="/api/v1/preview/friends"`,
		`data-api="/api/v1/preview/sites"`,
		`src="/api/v1/preview/md?src=`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rewrite missing %q in: %s", want, out)
		}
	}
	// 原外部端点不得作为直连残留（编码进查询参数的来源标识除外）
	for _, bad := range []string{`/api/rating"`, `/api/vote"`, "example.com/data.json", "example.com/sites.json"} {
		if strings.Contains(out, bad) {
			t.Errorf("rewrite should drop %q, got: %s", bad, out)
		}
	}
	// md src 查询参数编码：原 URL 的 / 与 : 被编码，属性不被引号/& 破坏
	if !strings.Contains(out, `src="/api/v1/preview/md?src=https%3A%2F%2Fraw.githubusercontent.com%2Fx%2Fy%2Fmain%2FREADME.md"`) {
		t.Errorf("md src should be query-escaped, got: %s", out)
	}
}

func TestRewritePreviewDataAPIsFriendsAndPosts(t *testing.T) {
	// friends posts:true 走 ds-friends_and_posts，同样改写为本地 Fixture 端点
	in := `<div class="tag-plugin users-posts-wrap"><div class="data-service ds-friends_and_posts" data-api="/api/friends-aggregated"><div class="grid-box"></div></div></div>`
	out := rewritePreviewDataAPIs(in)
	if !strings.Contains(out, `data-api="/api/v1/preview/friends"`) {
		t.Errorf("friends_and_posts rewrite missing: %s", out)
	}
}

func TestFixtureGHClient(t *testing.T) {
	repo, err := (fixtureGHClient{}).GetRepo("vuejs", "vue")
	if err != nil {
		t.Fatalf("fixture GetRepo: %v", err)
	}
	if repo.FullName != "vuejs/vue" || repo.Stars != 12800 || repo.Language != "Go" {
		t.Errorf("fixture repo unexpected: %+v", repo)
	}
	user, err := (fixtureGHClient{}).GetUser("octocat")
	if err != nil {
		t.Fatalf("fixture GetUser: %v", err)
	}
	if user.Login != "octocat" || user.Followers != 3200 {
		t.Errorf("fixture user unexpected: %+v", user)
	}
}

func TestRenderComponentPreview(t *testing.T) {
	s := NewService(t.TempDir())
	out, err := s.RenderComponentPreview("{% ghrepo vuejs/vue %}\n\n{% rating id:demo-post %}\n\n{% friends %}\n\n{% vote 有帮助吗 id:post-1 %}")
	if err != nil {
		t.Fatalf("RenderComponentPreview: %v", err)
	}
	for _, want := range []string{
		// ghrepo 使用确定性 Fixture 数据
		`<span class="repo-name">vuejs/vue</span>`,
		`>⭐ 12800<`,
		// rating：端点改写 + preview- 隔离 ID
		`data-api="/api/v1/preview/rating"`,
		`data-id="preview-demo-post"`,
		// vote：端点改写 + preview- 隔离 ID
		`data-api="/api/v1/preview/vote"`,
		`data-id="preview-post-1"`,
		// friends：缺省端点改写为本地 Fixture
		`data-api="/api/v1/preview/friends"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("component preview missing %q in: %s", want, out)
		}
	}
}

func TestRenderComponentPreviewStaticPlugins(t *testing.T) {
	s := NewService(t.TempDir())
	out, err := s.RenderComponentPreview("{% note color:green 提示 这是提示内容 %}\n\n{% banner title:我的博客 %}\n欢迎。\n{% endbanner %}\n\n{% poetry 静夜思 author:李白 %}\n床前明月光。\n{% endpoetry %}\n\n{% frame device:iphone width:320px %}\n![s](https://example.com/s.png)\n{% endframe %}")
	if err != nil {
		t.Fatalf("RenderComponentPreview: %v", err)
	}
	for _, want := range []string{
		`class="tag-plugin colorful note"`,
		`color="green"`,
		`class="tag-plugin banner"`,
		`class="tag-plugin poetry"`,
		`device="iphone"`,
		`class="tag-plugin frame"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("static plugin preview missing %q in: %s", want, out)
		}
	}
}

func TestRenderComponentPreviewInteractivePlugins(t *testing.T) {
	s := NewService(t.TempDir())
	out, err := s.RenderComponentPreview("{% tabs %}\n<!-- tab 标签一 -->\n第一页。\n<!-- tab 标签二 -->\n第二页。\n{% endtabs %}\n\n{% folding color:cyan open:true 详情 %}\n折叠内容。\n{% endfolding %}\n\n{% swiper %}\n![](https://example.com/1.png)\n![](https://example.com/2.png)\n{% endswiper %}")
	if err != nil {
		t.Fatalf("RenderComponentPreview: %v", err)
	}
	for _, want := range []string{
		`class="tag-plugin tabs"`,
		`<details class="tag-plugin colorful folding"`,
		`class="tag-plugin swiper fancybox"`,
		`<div class="swiper-slide">`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("interactive plugin preview missing %q in: %s", want, out)
		}
	}
}

func TestRenderComponentPreviewExternalPlugins(t *testing.T) {
	s := NewService(t.TempDir())
	out, err := s.RenderComponentPreview("{% mermaid %}\ngraph TB\n A --> B\n{% endmermaid %}\n\n{% md https://raw.githubusercontent.com/xaoxuu/hexo-theme-stellar/main/README.md %}")
	if err != nil {
		t.Fatalf("RenderComponentPreview: %v", err)
	}
	// mermaid：源码保留在容器内（客户端 math-mermaid-loader 负责渲染，失败降级为源码）
	if !strings.Contains(out, `<div class="mermaid">graph TB`) {
		t.Errorf("mermaid fixture missing: %s", out)
	}
	// md：src 改写为本地 Fixture 端点
	if !strings.Contains(out, `class="data-service ds-mdrender" src="/api/v1/preview/md?src=`) {
		t.Errorf("md fixture rewrite missing: %s", out)
	}
}

func TestRenderComponentPreviewLinksFixture(t *testing.T) {
	// {% links %} 在 Fixture 模式下服务端直出确定性分组（不依赖站点 links 配置注入）
	s := NewService(t.TempDir())
	out, err := s.RenderComponentPreview("{% links %}")
	if err != nil {
		t.Fatalf("RenderComponentPreview: %v", err)
	}
	for _, want := range []string{
		`<h2>推荐站点</h2>`,
		`<h2>小伙伴</h2>`,
		`<span class="title">Fixture Blog</span>`,
		`<span class="title">Alice</span>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("links fixture missing %q in: %s", want, out)
		}
	}
}

// ---------- Spec 64：暗色预览收尾（本地资源 / emoji / TOC / albums-posters variant） ----------

func TestRenderComponentPreviewEmojiLocalized(t *testing.T) {
	// component-library 模式：内置 emoji 源（qq/tieba/twemoji）CDN 地址改写为本地素材；
	// url: 分支的真实外链不受影响。
	s := NewService(t.TempDir())
	out, err := s.RenderComponentPreview("开心 {% emoji aini %} 点赞 {% emoji twemoji 1f44d %} 滑稽 {% emoji tieba huaji %}")
	if err != nil {
		t.Fatalf("RenderComponentPreview: %v", err)
	}
	for _, want := range []string{
		`src="/assets/img/preview/emoji/qq-aini.gif"`,
		`src="/assets/img/preview/emoji/twemoji-1f44d.svg"`,
		`src="/assets/img/preview/emoji/tieba-huaji.png"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("emoji fixture missing %q in: %s", want, out)
		}
	}
	if strings.Contains(out, "cdn.jsdelivr.net") {
		t.Errorf("emoji fixture should not keep CDN url: %s", out)
	}
	// url: 分支保持用户提供的真实 URL
	outURL, err := s.RenderComponentPreview("{% emoji url:https://example.com/x.png 名称 %}")
	if err != nil {
		t.Fatalf("RenderComponentPreview url: %v", err)
	}
	if !strings.Contains(outURL, `src="https://example.com/x.png"`) {
		t.Errorf("url: branch should keep real url: %s", outURL)
	}
	// 无 profile 的普通渲染保持 CDN 源（生产行为不变）
	outNormal, err := s.Render("开心 {% emoji aini %}")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(outNormal, "cdn.jsdelivr.net/gh/cdn-x/emoticons@3.1.0/qq/aini.gif") {
		t.Errorf("normal render should keep CDN emoji url: %s", outNormal)
	}
}

func TestRenderComponentPreviewTOCFixture(t *testing.T) {
	// component-library 模式：{% toc %} 的 details 壳内填充确定性两级目录；
	// 无 profile 时保持空壳（生产行为不变）。
	s := NewService(t.TempDir())
	out, err := s.RenderComponentPreview("{% toc 本文目录 %}")
	if err != nil {
		t.Fatalf("RenderComponentPreview: %v", err)
	}
	for _, want := range []string{
		`<details class="toc" open><summary>本文目录</summary>`,
		`<ol class="toc"`,
		`第一章 背景`,
		`1.1 项目动机`,
		`第二章 方案`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("toc fixture missing %q in: %s", want, out)
		}
	}
	outNormal, err := s.Render("{% toc 本文目录 %}")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(outNormal, "第一章 背景") {
		t.Errorf("normal render should keep empty toc shell: %s", outNormal)
	}
}

func TestRenderComponentPreviewGHFixtureLocalAvatar(t *testing.T) {
	// GitHub Fixture 头像为本地素材，链接保留真实 GitHub 地址。
	s := NewService(t.TempDir())
	out, err := s.RenderComponentPreview("{% ghrepo vuejs/vue %}\n\n{% ghuser octocat %}")
	if err != nil {
		t.Fatalf("RenderComponentPreview: %v", err)
	}
	if !strings.Contains(out, `/assets/img/preview/avatar-gh-vuejs.svg?v=4&s=40`) {
		t.Errorf("ghrepo avatar should be local with resize params: %s", out)
	}
	if !strings.Contains(out, `/assets/img/preview/avatar-gh-octocat.svg?v=4&s=64`) {
		t.Errorf("ghuser avatar should be local with resize params: %s", out)
	}
	if strings.Contains(out, "avatars.githubusercontent.com") {
		t.Errorf("fixture should not hit github avatar cdn: %s", out)
	}
	if !strings.Contains(out, `https://github.com/vuejs/vue`) {
		t.Errorf("fixture should keep real github link: %s", out)
	}
}

func TestRewritePreviewDataAPIsAlbumsPostersVariant(t *testing.T) {
	// albums/posters 的 ds-friends data-api 改写为带 variant 的本地 Fixture 端点；
	// 普通 friends 改写不带 variant。
	in := `<div class="tag-plugin albums-wrap"><div class="data-service ds-friends" data-api="https://example.com/data.json"><div class="tag-plugin gallery grid-box"></div></div></div>` +
		`<div class="tag-plugin posters-wrap"><div class="data-service ds-friends" data-api="https://example.com/posters.json"><div class="tag-plugin gallery grid-box"></div></div></div>` +
		`<div class="tag-plugin users-wrap"><div class="data-service ds-friends" data-api="/api/friends"><div class="grid-box"></div></div></div>`
	out := rewritePreviewDataAPIs(in)
	for _, want := range []string{
		`data-api="/api/v1/preview/friends?variant=albums"`,
		`data-api="/api/v1/preview/friends?variant=posters"`,
		`data-api="/api/v1/preview/friends"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rewrite missing %q in: %s", want, out)
		}
	}
	if strings.Contains(out, "example.com") {
		t.Errorf("rewrite should drop external api url: %s", out)
	}
}

func TestRenderComponentPreviewDefaultExamplesOffline(t *testing.T) {
	// 代表性默认示例经 component-library 渲染后，资源属性（src/data-src/data-api/source）
	// 不得残留公网地址（href 链接除外）；emoji/md/albums/sites 均被本地化改写。
	s := NewService(t.TempDir())
	examples := []string{
		"{% gallery %}\n![a](/assets/img/preview/gallery-1.jpg)\n{% endgallery %}",
		"{% banner title:t bg:/assets/img/preview/banner-bg.jpg avatar:/assets/img/preview/avatar-blog.svg %}b{% endbanner %}",
		"{% frame device:iphone width:320px %}\n![s](/assets/img/preview/frame-shot.jpg)\n{% endframe %}",
		"{% about avatar:/assets/img/preview/avatar-blog.svg %}a{% endabout %}",
		"{% image /assets/img/preview/image-demo.jpg 图 width:80% download:true %}",
		"{% video /assets/media/preview/sample-video.mp4 %}",
		"{% audio /assets/media/preview/sample-audio.mp3 %}",
		"{% swiper %}\n![](/assets/img/preview/swiper-1.jpg)\n{% endswiper %}",
		"{% md /assets/md/preview/example.md %}",
		"{% albums api:https://example.com/data.json size:m %}",
		"{% posters api:https://example.com/posters.json size:s %}",
		"{% sites api:https://example.com/sites.json %}",
		"{% chat iphone style:qq title:群 me:b %}\nalice:\n name: 小明\n avatar: /assets/img/preview/avatar-xiaoming.svg\n<!-- chatcell user:alice md:hi -->\n{% endchat %}",
		"{% emoji aini %}",
		"{% toc 目录 %}",
		"{% mermaid %}\ngraph TB\n A[开始] --> B[结束]\n{% endmermaid %}",
	}
	for _, ex := range examples {
		out, err := s.RenderComponentPreview(ex)
		if err != nil {
			t.Fatalf("RenderComponentPreview(%q): %v", ex, err)
		}
		for _, attr := range []string{`src="http`, `data-src="http`, `data-api="http`, `srcset="http`, `<source src="http`} {
			if strings.Contains(out, attr) {
				t.Errorf("example %q leaves external %s in: %s", ex, attr, out)
			}
		}
	}
}
