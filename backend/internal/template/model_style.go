package template

import (
	"fmt"
	"html/template"
	"strconv"
	"strings"
)

// ---------- Spec 34/52: 外观样式可配置化 ----------
//
// 对齐原版 Stellar _config.yml 的 style 段。实现策略（依据 main.css 编译产物特征）：
//   - 颜色（theme 经 --hue/--sat/--light 分量、accent、link+link-a20）与正文字号(--fsp)是 :root 变量 → 直接注入覆盖；
//   - 字体族/根字号/leftbar 背景/全站背景/header_prefix/分页箭头/代码块滚动条是编译期字面量 → 选择器级 style 覆盖；
//   - 圆角/渐变（Spec 52）：border_radius 四档 + gradient 两组注入 :root 变量（--radius-*/--gradient-*），
//     消费点（main.css）一律 var(--radius-*, 原硬编码值) fallback 形式，未配置不注入 → 零配置零差异。
// 全部字段缺省为空 → InjectCSS 输出空串 → 零配置时与现状逐像素一致（回归底线）。

// StyleConfig site.yml style 段（键名对照原版 _config.yml style）。
type StyleConfig struct {
	PrefersTheme string            `yaml:"prefers_theme" json:"prefers_theme"` // 默认主题：dark（缺省，现状）/light/auto（跟随系统）
	
	TextAlign    string            `yaml:"text_align" json:"text_align"`       // 正文段落对齐（left/justify 等，空=现状 left）
	Color        StyleColor        `yaml:"color" json:"color"`
	FontSize     StyleFontSize     `yaml:"font_size" json:"font_size"`
	FontFamily   StyleFontFamily   `yaml:"font_family" json:"font_family"`
	Leftbar      StyleLeftbar      `yaml:"leftbar" json:"leftbar"`
	Site         StyleSite         `yaml:"site" json:"site"`
	Codeblock    StyleCodeblock    `yaml:"codeblock" json:"codeblock"`
	Paginator    StylePaginator    `yaml:"paginator" json:"paginator"`
	HeaderPrefix map[string]string `yaml:"header_prefix" json:"header_prefix"` // 技术文章各级标题前缀：符号（content）或图标 URL（background-image）
	BorderRadius StyleBorderRadius `yaml:"border_radius" json:"border_radius"` // 圆角（Spec 52 四档 + 1.38 七档兼容）
	Gradient     StyleGradient     `yaml:"gradient" json:"gradient"`           // 渐变两组（Spec 52）：注入 --gradient-* 变量
	AnimatedAvatar StyleAnimatedAvatar `yaml:"animated_avatar" json:"animated_avatar"` // 1.38 头像动画组
}

// StyleBorderRadius 圆角组。Spec 52 四档（card/block/navbar/image）+ 1.38 七档（card-l/card/card-s/bar/image-l/image/image-s）兼容。
// 注入 :root 变量 --radius-*；缺省空 = 不注入，消费点回退 main.css 原硬编码值。
type StyleBorderRadius struct {
	Card   string `yaml:"card" json:"card"`     // 卡片容器（post-card/article 等）
	Block  string `yaml:"block" json:"block"`   // 块级元素（note/block 等）
	Navbar string `yaml:"navbar" json:"navbar"` // 导航栏/菜单容器
	Image  string `yaml:"image" json:"image"`   // 图片（含 .img 类）
	// 1.38 七档兼容键（card-l/card-s/bar/image-l/image-s）
	CardL  string `yaml:"card-l" json:"card-l"`
	CardS  string `yaml:"card-s" json:"card-s"`
	Bar    string `yaml:"bar" json:"bar"`
	ImageL string `yaml:"image-l" json:"image-l"`
	ImageS string `yaml:"image-s" json:"image-s"`
}

// StyleGradient 渐变组（Spec 52，1.38 键名对齐上游 start/searchbar/angle）：注入 :root 变量 --gradient-start/searchbar。
// 值为任意 CSS gradient 表达式（linear-gradient(...) 等）；缺省空 = 不注入。
type StyleGradient struct {
	Start     string `yaml:"start" json:"start"`         // 按钮/起始背景渐变（上游 style.gradient.start）
	Searchbar string `yaml:"searchbar" json:"searchbar"` // 搜索栏渐变
	Angle     string `yaml:"angle" json:"angle"`         // 主题色渐变角度（1.38 style.gradient.angle，如 210deg；菜单 grad-def SVG 用）
}

// StyleColor 主题色组（缺省空=main.css 内置值：theme hsl(192 98% 55%)/accent #ff5724/link hsl(207 90% 54%)）。
type StyleColor struct {
	Theme  string `yaml:"theme" json:"theme"`   // 主题色（hsl()/hex，解析为 --hue/--sat/--light 分量注入）
	Accent string `yaml:"accent" json:"accent"` // 强调色（任意 CSS 颜色，原样注入 --accent）
	Link   string `yaml:"link" json:"link"`     // 超链接色（解析为 hsla 后注入 --link + --link-a20）
}

// StyleFontSize 字号组。
type StyleFontSize struct {
	Root      string `yaml:"root" json:"root"`           // 根字号（html font-size，选择器覆盖）
	Body      string `yaml:"body" json:"body"`           // 正文字号（注入 --fsp + --gap-p/--gap-p-compact）
	Code      string `yaml:"code" json:"code"`           // 行内代码字号（1.38，选择器覆盖）
	Codeblock string `yaml:"codeblock" json:"codeblock"` // 代码块字号（1.38，选择器覆盖）
}

// StyleFontFamily 字体族组（选择器级覆盖；body 缺省仍为 LXGW 文楷方案，不回退系统字体栈）。
type StyleFontFamily struct {
	Body      string `yaml:"body" json:"body"`
	Code      string `yaml:"code" json:"code"`
	Codeblock string `yaml:"codeblock" json:"codeblock"`
}

// StyleLeftbar leftbar 背景组（缺省空=现状纯色 var(--card)，由 head.html 既有块提供；配 background_image 切换为图片模糊模式）。
type StyleLeftbar struct {
	BackgroundColorDark string `yaml:"background_color_dark" json:"background_color_dark"` // 全站仅深色模式，此色无条件生效
	BackgroundImage     string `yaml:"background_image" json:"background_image"`           // 图片 URL（不含 url()）；非空则启用图片模糊侧栏
	BlurPx              string `yaml:"blur_px" json:"blur_px"`
	BackgroundOpacity   string `yaml:"background_opacity" json:"background_opacity"`
}

// StyleSite 全站背景组（缺省空=现状无背景图、siteblur 透明；配 background_image 后启用毛玻璃遮罩）。
type StyleSite struct {
	BackgroundImage string `yaml:"background_image" json:"background_image"`
	BlurPx          string `yaml:"blur_px" json:"blur_px"`
	BlurSat         string `yaml:"blur_sat" json:"blur_sat"`
	BlurBg          string `yaml:"blur_bg" json:"blur_bg"`
}

// StyleCodeblock 代码块组。
type StyleCodeblock struct {
	Scrollbar       string `yaml:"scrollbar" json:"scrollbar"`                 // 滚动条宽度（如 4px）
	HighlightjsTheme string `yaml:"highlightjs_theme" json:"highlightjs_theme"` // 1.38 高亮主题 URL（配置对齐，运行时由 head 注入 link）
}

// StyleAnimatedAvatar 头像动画组（1.38）。
type StyleAnimatedAvatar struct {
	Animate    string `yaml:"animate" json:"animate"`       // auto / always
	Background string `yaml:"background" json:"background"` // 头像旋转背景图 URL
}

// StylePaginator 分页箭头图（选择器级覆盖 background-image）。
type StylePaginator struct {
	Prev string `yaml:"prev" json:"prev"`
	Next string `yaml:"next" json:"next"`
}

// InjectCSS 生成注入到 head 的样式覆盖（仅输出用户显式配置项，未配置不输出）。
// 返回 template.CSS 跳过 html/template 的 <style> 上下文转义；零配置返回空串（模板据此不渲染 style 标签）。
func (s StyleConfig) InjectCSS() template.CSS {
	var b strings.Builder

	// --- :root 变量组（颜色 + 正文字号）---
	var root strings.Builder
	if h, sat, l, ok := parseHSL(s.Color.Theme); ok {
		fmt.Fprintf(&root, "--hue:%sdeg;--sat:%s%%;--light:%s%%;", trimNum(h), trimNum(sat), trimNum(l))
	}
	if s.Color.Accent != "" {
		fmt.Fprintf(&root, "--accent:%s;", s.Color.Accent)
	}
	if h, sat, l, ok := parseHSL(s.Color.Link); ok {
		fmt.Fprintf(&root, "--link:hsla(%sdeg %s%% %s%% / 1);--link-a20:hsla(%sdeg %s%% %s%% / 0.2);",
			trimNum(h), trimNum(sat), trimNum(l), trimNum(h), trimNum(sat), trimNum(l))
	}
	if s.FontSize.Body != "" {
		// --fsp 派生 --fsh2/3/4；--gap-p/--gap-p-compact 在 main.css 为字面量 calc(17px…)，一并覆盖保持一致。
		fmt.Fprintf(&root, "--fsp:%s;--gap-p:calc(%s + 4px);--gap-p-compact:calc(%s * 0.75);",
			s.FontSize.Body, s.FontSize.Body, s.FontSize.Body)
	}
	// 圆角四档（Spec 52）：仅显式配置时输出，未配置不定义变量（消费点 var(--radius-*, 原值) fallback）
	if s.BorderRadius.Card != "" {
		fmt.Fprintf(&root, "--radius-card:%s;", s.BorderRadius.Card)
	}
	if s.BorderRadius.Block != "" {
		fmt.Fprintf(&root, "--radius-block:%s;", s.BorderRadius.Block)
	}
	if s.BorderRadius.Navbar != "" {
		fmt.Fprintf(&root, "--radius-navbar:%s;", s.BorderRadius.Navbar)
	}
	if s.BorderRadius.Image != "" {
		fmt.Fprintf(&root, "--radius-image:%s;", s.BorderRadius.Image)
	}
	// 1.38 七档兼容键：仅显式配置时输出对应 --radius-* 变量
	for _, kv := range []struct{ v, name string }{
		{s.BorderRadius.CardL, "--radius-card-l"}, {s.BorderRadius.CardS, "--radius-card-s"},
		{s.BorderRadius.Bar, "--radius-bar"}, {s.BorderRadius.ImageL, "--radius-image-l"}, {s.BorderRadius.ImageS, "--radius-image-s"},
	} {
		if kv.v != "" {
			fmt.Fprintf(&root, "%s:%s;", kv.name, kv.v)
		}
	}
	// 渐变两组（Spec 52）：仅显式配置时输出
	if s.Gradient.Start != "" {
		fmt.Fprintf(&root, "--gradient-start:%s;", s.Gradient.Start)
	}
	if s.Gradient.Searchbar != "" {
		fmt.Fprintf(&root, "--gradient-searchbar:%s;", s.Gradient.Searchbar)
	}
	if root.Len() > 0 {
		b.WriteString(":root{")
		b.WriteString(root.String())
		b.WriteString("}")
	}

	// --- 选择器级覆盖 ---
	// 根字号
	if s.FontSize.Root != "" {
		fmt.Fprintf(&b, "html{font-size:%s;}", s.FontSize.Root)
	}
	// body 字体族（覆盖 head.html 既有 LXGW 块）
	if s.FontFamily.Body != "" {
		fmt.Fprintf(&b, "html{font-family:%s;}", s.FontFamily.Body)
	}
	// 代码字体族（行内 code + 代码块 pre/figure）
	if s.FontFamily.Code != "" {
		fmt.Fprintf(&b, "code{font-family:%s;}", s.FontFamily.Code)
	}
	if s.FontFamily.Codeblock != "" {
		fmt.Fprintf(&b, "pre,.md-text .highlight{font-family:%s;}", s.FontFamily.Codeblock)
	}
	// 1.38 代码字号（行内 code + 代码块）
	if s.FontSize.Code != "" {
		fmt.Fprintf(&b, ".md-text code:not(pre code){font-size:%s;}", s.FontSize.Code)
	}
	if s.FontSize.Codeblock != "" {
		fmt.Fprintf(&b, ".md-text .highlight{font-size:%s;}", s.FontSize.Codeblock)
	}
	// 正文段落对齐
	if s.TextAlign != "" {
		fmt.Fprintf(&b, ".md-text p:not([class]){text-align:%s;}", s.TextAlign)
	}
	// 代码块滚动条宽度
	if s.Codeblock.Scrollbar != "" {
		fmt.Fprintf(&b, ".md-text .highlight >table::-webkit-scrollbar{height:%s;width:%s;}", s.Codeblock.Scrollbar, s.Codeblock.Scrollbar)
	}
	// 分页箭头
	if s.Paginator.Next != "" {
		fmt.Fprintf(&b, ".paginator-wrap .extend.next{background-image:url(%s);}", s.Paginator.Next)
	}
	if s.Paginator.Prev != "" {
		fmt.Fprintf(&b, ".paginator-wrap .extend.prev{background-image:url(%s);}", s.Paginator.Prev)
	}
	// header_prefix：技术文章标题前缀。值为 URL 时注入图标 background-image，否则伪元素 content 符号。
	for _, h := range []string{"h2", "h3", "h4", "h5"} {
		if v, ok := s.HeaderPrefix[h]; ok && v != "" {
			if isURLValue(v) {
				fmt.Fprintf(&b, ".l_body[type='tech'] article.content %s >a:first-child:before{content:'';display:inline-block;width:1em;height:1em;background:url(%s) center/contain no-repeat;}", h, v)
			} else {
				fmt.Fprintf(&b, ".l_body[type='tech'] article.content %s >a:first-child:before{content:'%s';}", h, cssContentEscape(v))
			}
		}
	}
	// leftbar 背景
	b.WriteString(s.leftbarCSS())
	// 全站背景
	b.WriteString(s.siteCSS())

	return template.CSS(b.String())
}

// leftbarCSS 生成 leftbar 背景覆盖。配 background_image 时重新启用图片模糊侧栏（覆盖 head.html 纯色块）；
// 仅配 background_color_dark 时改容器纯色（全站仅深色模式，无条件生效）。
func (s StyleConfig) leftbarCSS() string {
	l := s.Leftbar
	var b strings.Builder
	if l.BackgroundImage != "" {
		blur := firstNonEmpty(l.BlurPx, "100px")
		opacity := firstNonEmpty(l.BackgroundOpacity, "0.8")
		// 重新显示 sidebg 并设图片；恢复 head.html 纯色块（Spec Task 2）关掉的 mask 淡出与玻璃高光，
		// 回到原版图片模糊模式（Spec 50 侧栏背景图验证修复）。
		fmt.Fprintf(&b, ".l_left .sidebg{display:block;background-image:url(%s);--blur-px:%s;--background-opacity:%s;}", l.BackgroundImage, blur, opacity)
		b.WriteString(".l_left .leftbar-container{background:transparent;}")
		b.WriteString(".l_left .leftbar-container >.widgets{mask:linear-gradient(#fff,90%,transparent);-webkit-mask:linear-gradient(#fff,90%,transparent);}")
		b.WriteString("@media screen and (min-width:667px){.l_left .leftbar-container:before{display:block;}}")
	} else if l.BackgroundColorDark != "" {
		fmt.Fprintf(&b, ".l_left .leftbar-container{background:%s;}", l.BackgroundColorDark)
	}
	return b.String()
}

// siteCSS 生成全站背景覆盖。仅在配 background_image 时启用（同时移除 head.html 的 siteblur 透明兜底）。
func (s StyleConfig) siteCSS() string {
	site := s.Site
	if site.BackgroundImage == "" {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "body>.sitebg{background-image:url(%s);}", site.BackgroundImage)
	blurPx := firstNonEmpty(site.BlurPx, "100px")
	blurSat := firstNonEmpty(site.BlurSat, "300%")
	blurBg := firstNonEmpty(site.BlurBg, "var(--bg-a75)")
	// 覆盖 head.html 的 siteblur 透明块，恢复毛玻璃（!important 对齐 main.css @supports 块的优先级）
	fmt.Fprintf(&b, "body>.sitebg .siteblur{background-color:%s !important;-webkit-backdrop-filter:saturate(%s) blur(%s) !important;backdrop-filter:saturate(%s) blur(%s) !important;}",
		blurBg, blurSat, blurPx, blurSat, blurPx)
	return b.String()
}

// cssContentEscape 转义 content 值里的单引号与反斜杠，防止注入闭合。
func cssContentEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `'`, `\'`)
}

// isURLValue 判断 header_prefix 值是否为图标 URL（http(s):// 或 / 开头）。
func isURLValue(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "/")
}

// firstNonEmpty 返回首个非空串。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// trimNum 去掉浮点尾随 0（192.0 → 192），生成紧凑的 CSS 数值。
func trimNum(f float64) string {
	s := strconv.FormatFloat(f, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	return strings.TrimRight(s, ".")
}

// parseHSL 解析颜色为 HSL 分量（h 角度 deg，s/l 百分比）。支持 hsl()/hsla()/#rgb/#rrggbb；无法解析返回 ok=false。
func parseHSL(s string) (h, sPct, lPct float64, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, 0, false
	}
	if strings.HasPrefix(s, "#") {
		r, g, bb, ok2 := parseHexRGB(s)
		if !ok2 {
			return 0, 0, 0, false
		}
		h, sPct, lPct = rgbToHSL(r, g, bb)
		return h, sPct, lPct, true
	}
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "hsl") {
		// 提取括号内数值：hsl(192 98% 55%) / hsl(192,98%,55%) / hsla(192 98% 55% / 1)
		open := strings.Index(s, "(")
		close := strings.LastIndex(s, ")")
		if open < 0 || close <= open {
			return 0, 0, 0, false
		}
		inner := s[open+1 : close]
		if slash := strings.Index(inner, "/"); slash >= 0 {
			inner = inner[:slash] // 去掉 alpha 部分
		}
		inner = strings.ReplaceAll(inner, ",", " ")
		fields := strings.Fields(inner)
		if len(fields) < 3 {
			return 0, 0, 0, false
		}
		hh, e1 := strconv.ParseFloat(strings.TrimSuffix(strings.ToLower(fields[0]), "deg"), 64)
		ss, e2 := strconv.ParseFloat(strings.TrimSuffix(fields[1], "%"), 64)
		ll, e3 := strconv.ParseFloat(strings.TrimSuffix(fields[2], "%"), 64)
		if e1 != nil || e2 != nil || e3 != nil {
			return 0, 0, 0, false
		}
		return hh, ss, ll, true
	}
	return 0, 0, 0, false
}

// parseHexRGB 解析 #rgb / #rrggbb 为 0-255 分量。
func parseHexRGB(s string) (r, g, b int, ok bool) {
	s = strings.TrimPrefix(s, "#")
	switch len(s) {
	case 3:
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	case 6:
	default:
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return int(v >> 16), int(v >> 8 & 0xff), int(v & 0xff), true
}

// rgbToHSL 0-255 RGB → HSL（h∈[0,360) deg，s/l∈[0,100] %），标准转换。
func rgbToHSL(ri, gi, bi int) (h, s, l float64) {
	r, g, b := float64(ri)/255, float64(gi)/255, float64(bi)/255
	max, min := maxOf(r, g, b), minOf(r, g, b)
	l = (max + min) / 2
	if max == min {
		return 0, 0, round1(l * 100) // 灰阶：无色相无饱和
	}
	d := max - min
	if l > 0.5 {
		s = d / (2 - max - min)
	} else {
		s = d / (max + min)
	}
	switch max {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	return round1(h), round1(s * 100), round1(l * 100)
}

func maxOf(a, b, c float64) float64 {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	return m
}

func minOf(a, b, c float64) float64 {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

// round1 保留一位小数，抑制浮点噪声。
func round1(f float64) float64 {
	return float64(int(f*10+0.5)) / 10
}
