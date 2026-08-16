package post

import (
	"strings"
	"testing"

	"blog/internal/settings"
)

// runShortcodes 便捷调用：identity renderMD + 简单 icon 桩，不依赖 goldmark/GitHub。
func runShortcodes(md string) string {
	iconFunc := func(key string) string { return `<svg data-icon="` + key + `"></svg>` }
	return processShortcodes(md, func(s string) string { return s }, func(s string) string { return s }, iconFunc, nil, nil, settings.TagPluginsConfig{})
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
		{"icon key 白名单放行合法 key", `{% icon quot:hashtag %}`,
			`<svg data-icon="quot:hashtag"></svg>`, ``},
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

// default/qq/tieba/aru/blobcat/twemoji，未知来源原样保留。
