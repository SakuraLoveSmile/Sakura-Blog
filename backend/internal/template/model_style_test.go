package template

import (
	"strings"
	"testing"
)

// TestParseHSL 颜色解析：hsl/hsla/hex → HSL 分量（表驱动）。
func TestParseHSL(t *testing.T) {
	cases := []struct {
		in            string
		wantH, wantS, wantL float64
		wantOK        bool
	}{
		{"hsl(192 98% 55%)", 192, 98, 55, true},
		{"hsl(192, 98%, 55%)", 192, 98, 55, true},
		{"hsla(207deg 90% 54% / 1)", 207, 90, 54, true},
		{"#ffffff", 0, 0, 100, true}, // 纯白：灰阶 h/s=0 l=100
		{"#000000", 0, 0, 0, true},   // 纯黑
		{"#ff0000", 0, 100, 50, true}, // 纯红
		{"", 0, 0, 0, false},
		{"red", 0, 0, 0, false},       // 颜色关键字不支持
		{"#12", 0, 0, 0, false},       // 非法 hex
	}
	for _, c := range cases {
		h, s, l, ok := parseHSL(c.in)
		if ok != c.wantOK {
			t.Errorf("parseHSL(%q) ok=%v, want %v", c.in, ok, c.wantOK)
			continue
		}
		if ok && (h != c.wantH || s != c.wantS || l != c.wantL) {
			t.Errorf("parseHSL(%q) = (%g,%g,%g), want (%g,%g,%g)", c.in, h, s, l, c.wantH, c.wantS, c.wantL)
		}
	}
}

// TestStyleInjectCSSEmpty 零配置 → 空注入（像素回归底线）。
func TestStyleInjectCSSEmpty(t *testing.T) {
	var s StyleConfig
	if got := string(s.InjectCSS()); got != "" {
		t.Errorf("零配置 InjectCSS 应为空，got %q", got)
	}
	if got := s.EffectivePrefersTheme(); got != "auto" {
		t.Errorf("零配置 prefers_theme = %q, want auto", got)
	}
}

// TestStyleInjectCSSColors 颜色组注入：theme 拆分量、accent 原样、link 双变量。
func TestStyleInjectCSSColors(t *testing.T) {
	s := StyleConfig{Color: StyleColor{
		Theme:  "hsl(14 100% 57%)",
		Accent: "#ff0000",
		Link:   "hsl(207 90% 54%)",
	}}
	css := string(s.InjectCSS())
	for _, want := range []string{
		"--hue:14deg", "--sat:100%", "--light:57%", // theme 分量
		"--accent:#ff0000;",                          // accent 原样
		"--link:hsla(207deg 90% 54% / 1)",            // link 全色
		"--link-a20:hsla(207deg 90% 54% / 0.2)",      // link a20 派生
	} {
		if !strings.Contains(css, want) {
			t.Errorf("颜色注入缺 %q\n完整: %s", want, css)
		}
	}
	if !strings.HasPrefix(css, ":root{") {
		t.Errorf("颜色变量应在 :root 块内，got %s", css)
	}
}

// TestStyleInjectCSSFontAndMisc 字体/排版/代码块/分页/header_prefix 选择器级覆盖。
func TestStyleInjectCSSFontAndMisc(t *testing.T) {
	no := false
	s := StyleConfig{
		FontSize:   StyleFontSize{Root: "18px", Body: "16px"},
		FontFamily: StyleFontFamily{Body: "Arial", Code: "Menlo", Codeblock: "Consolas"},
		TextAlign:  "justify",
		SmoothScroll: &no,
		Codeblock:  StyleCodeblock{Scrollbar: "6px"},
		Paginator:  StylePaginator{Prev: "/p.svg", Next: "/n.svg"},
		HeaderPrefix: map[string]string{"h2": "#", "h3": "="},
	}
	css := string(s.InjectCSS())
	for _, want := range []string{
		"--fsp:16px;",                                   // 正文字号
		"--gap-p:calc(16px + 4px)",                      // gap 跟随
		"html{font-size:18px;}",                          // 根字号
		"html{font-family:Arial;}",                       // body 字体
		"code{font-family:Menlo;}",                       // code 字体
		"pre,.md-text .highlight{font-family:Consolas;}", // codeblock 字体
		".md-text p:not([class]){text-align:justify;}",   // 对齐
		"html{scroll-behavior:auto;}",                    // 关平滑滚动
		"::-webkit-scrollbar{height:6px;width:6px;}",     // 代码块滚动条
		".extend.next{background-image:url(/n.svg);}",    // 分页 next
		".extend.prev{background-image:url(/p.svg);}",    // 分页 prev
		"h2 >a:first-child:before{content:'#';}",         // header_prefix h2
		"h3 >a:first-child:before{content:'=';}",         // header_prefix h3
	} {
		if !strings.Contains(css, want) {
			t.Errorf("缺 %q\n完整: %s", want, css)
		}
	}
}

// TestStyleInjectCSSBackgrounds leftbar 图片模式 / 全站背景毛玻璃。
func TestStyleInjectCSSBackgrounds(t *testing.T) {
	// leftbar 图片模式：重启 sidebg + 恢复 head.html 纯色块关掉的 mask 淡出与玻璃高光（Spec 50）
	img := StyleConfig{Leftbar: StyleLeftbar{BackgroundImage: "/bg.jpg", BlurPx: "80px", BackgroundOpacity: "0.7"}}
	c1 := string(img.InjectCSS())
	for _, want := range []string{
		".l_left .sidebg{display:block;background-image:url(/bg.jpg);--blur-px:80px;--background-opacity:0.7;}",
		".l_left .leftbar-container{background:transparent;}",
		".l_left .leftbar-container >.widgets{mask:linear-gradient(#fff,90%,transparent);-webkit-mask:linear-gradient(#fff,90%,transparent);}",
		"@media screen and (min-width:667px){.l_left .leftbar-container:before{display:block;}}",
	} {
		if !strings.Contains(c1, want) {
			t.Errorf("leftbar 图片模式缺 %q\n完整: %s", want, c1)
		}
	}

	// leftbar 纯色模式（亮/暗分轨）
	color := StyleConfig{Leftbar: StyleLeftbar{BackgroundColorLight: "#eee", BackgroundColorDark: "#222"}}
	c2 := string(color.InjectCSS())
	if !strings.Contains(c2, ".l_left .leftbar-container{background:#eee;}") ||
		!strings.Contains(c2, `:root[data-theme="dark"] .l_left .leftbar-container{background:#222;}`) {
		t.Errorf("leftbar 纯色分轨错误: %s", c2)
	}

	// 全站背景毛玻璃
	site := StyleConfig{Site: StyleSite{BackgroundImage: "/site.webp"}}
	c3 := string(site.InjectCSS())
	if !strings.Contains(c3, "body>.sitebg{background-image:url(/site.webp);}") ||
		!strings.Contains(c3, "backdrop-filter:saturate(300%) blur(100px) !important") {
		t.Errorf("全站背景注入错误: %s", c3)
	}
	// 未配全站背景图 → 不输出 sitebg（保持 head.html 透明兜底）
	if strings.Contains(string(color.InjectCSS()), "body>.sitebg{") {
		t.Errorf("未配背景图不应输出 sitebg")
	}
}

// TestEffectivePrefersTheme 明暗偏好规范化。
func TestEffectivePrefersTheme(t *testing.T) {
	cases := map[string]string{"light": "light", "dark": "dark", "auto": "auto", "": "auto", "xxx": "auto"}
	for in, want := range cases {
		if got := (StyleConfig{PrefersTheme: in}).EffectivePrefersTheme(); got != want {
			t.Errorf("EffectivePrefersTheme(%q) = %q, want %q", in, got, want)
		}
	}
}
