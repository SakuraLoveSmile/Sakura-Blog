package settings

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// TestParseHSL 颜色解析：hsl/hsla/hex → HSL 分量（表驱动）。
func TestParseHSL(t *testing.T) {
	cases := []struct {
		in                  string
		wantH, wantS, wantL float64
		wantOK              bool
	}{
		{"hsl(192 98% 55%)", 192, 98, 55, true},
		{"hsl(192, 98%, 55%)", 192, 98, 55, true},
		{"hsla(207deg 90% 54% / 1)", 207, 90, 54, true},
		{"#ffffff", 0, 0, 100, true},  // 纯白：灰阶 h/s=0 l=100
		{"#000000", 0, 0, 0, true},    // 纯黑
		{"#ff0000", 0, 100, 50, true}, // 纯红
		{"", 0, 0, 0, false},
		{"red", 0, 0, 0, false}, // 颜色关键字不支持
		{"#12", 0, 0, 0, false}, // 非法 hex
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

// TestStyleInjectCSSEmpty 零配置 → 1.43.0 原生 View Transition 默认规则。
func TestStyleInjectCSSEmpty(t *testing.T) {
	var s StyleConfig
	got := string(s.InjectCSS())
	if !strings.Contains(got, "@view-transition{navigation:auto}") || !strings.Contains(got, "animation-duration:0.2s") {
		t.Errorf("零配置应默认启用 page transition，got %q", got)
	}
}

func TestStylePageTransitionExplicitFalse(t *testing.T) {
	disabled := false
	got := string((StyleConfig{PageTransition: StylePageTransition{Enable: &disabled}}).InjectCSS())
	if !strings.Contains(got, "@view-transition{navigation:none}") || !strings.Contains(got, "animation:none!important") {
		t.Fatalf("显式关闭应覆盖导航过渡，got %q", got)
	}
	if strings.Contains(got, "navigation:auto") {
		t.Fatalf("显式关闭不应输出 auto 规则，got %q", got)
	}
}

func TestStyleAndDefaultLegacyConfigAliases(t *testing.T) {
	var cfg struct {
		Style   StyleConfig   `yaml:"style"`
		Default DefaultAssets `yaml:"default"`
	}
	if err := yaml.Unmarshal([]byte(`style:
  page_transition:
    enable: false
  font-size:
    body: 17px
    root: 18px
  font-family:
    body: Arial
  text-align: justify
  border-radius:
    card: 12px
  prefix:
    h2: '#'
default:
  loading: /assets/loading.svg
`), &cfg); err != nil {
		t.Fatalf("decode upstream style aliases: %v", err)
	}
	if cfg.Style.PageTransition.Enable == nil || *cfg.Style.PageTransition.Enable {
		t.Fatal("explicit page_transition.enable=false was not preserved")
	}
	if cfg.Style.FontSize.Body != "17px" || cfg.Style.FontSize.Root != "18px" || cfg.Style.FontFamily.Body != "Arial" || cfg.Style.TextAlign != "justify" {
		t.Fatalf("hyphenated style aliases = %+v", cfg.Style)
	}
	if cfg.Style.BorderRadius.Card != "12px" || cfg.Style.HeaderPrefix["h2"] != "#" || cfg.Default.Loading != "/assets/loading.svg" {
		t.Fatalf("style/default compatibility values = %+v / %+v", cfg.Style, cfg.Default)
	}
	if css := string(cfg.Style.InjectCSS()); strings.Contains(css, "--fsp:17px") || strings.Contains(css, "--gap-p:calc(17px") {
		t.Fatalf("legacy font-size.body must not affect 1.43 rendering: %s", css)
	}

	data, err := json.Marshal(map[string]any{
		"page_transition": map[string]any{"enable": false},
		"font-size":       map[string]any{"body": "16px"},
		"default":         map[string]any{"loading": "/loading.svg"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var style StyleConfig
	if err := json.Unmarshal(data, &style); err != nil {
		t.Fatalf("decode JSON aliases: %v", err)
	}
	if style.PageTransition.Enable == nil || *style.PageTransition.Enable || style.FontSize.Body != "16px" {
		t.Fatalf("JSON style aliases = %+v", style)
	}
}

func TestStyleAnimatedAvatarModes(t *testing.T) {
	if !(StyleAnimatedAvatar{}).Enabled() {
		t.Fatal("empty animate value should preserve the enabled default")
	}
	if (StyleAnimatedAvatar{Animate: "false"}).Enabled() {
		t.Fatal("false animate value should disable the avatar ring")
	}
	if !(StyleAnimatedAvatar{Animate: "always"}).Always() {
		t.Fatal("always animate value should select continuous animation")
	}

	if css := string((StyleConfig{AnimatedAvatar: StyleAnimatedAvatar{Animate: "false"}}).InjectCSS()); !strings.Contains(css, "display:none") {
		t.Fatalf("disabled avatar ring CSS missing: %q", css)
	}
	if css := string((StyleConfig{AnimatedAvatar: StyleAnimatedAvatar{Animate: "always"}}).InjectCSS()); !strings.Contains(css, "opacity:1!important;animation:spin") {
		t.Fatalf("always avatar ring CSS missing: %q", css)
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
		"--accent:#ff0000;",                     // accent 原样
		"--link:hsla(207deg 90% 54% / 1)",       // link 全色
		"--link-a20:hsla(207deg 90% 54% / 0.2)", // link a20 派生
	} {
		if !strings.Contains(css, want) {
			t.Errorf("颜色注入缺 %q\n完整: %s", want, css)
		}
	}
	if !strings.Contains(css, ":root{") {
		t.Errorf("颜色变量应在 :root 块内，got %s", css)
	}
}

// TestStyleInjectCSSFontAndMisc 字体/排版/代码块/分页/header_prefix 选择器级覆盖。
func TestStyleInjectCSSFontAndMisc(t *testing.T) {
	s := StyleConfig{
		FontSize:     StyleFontSize{Root: "18px", Body: "16px"},
		FontFamily:   StyleFontFamily{Body: "Arial", Code: "Menlo", Codeblock: "Consolas"},
		TextAlign:    "justify",
		Codeblock:    StyleCodeblock{Scrollbar: "6px"},
		Paginator:    StylePaginator{Prev: "/p.svg", Next: "/n.svg"},
		HeaderPrefix: map[string]string{"h2": "#", "h3": "="},
	}
	css := string(s.InjectCSS())
	for _, want := range []string{
		"html{font-size:18px;}",                          // 根字号
		"html{font-family:Arial;}",                       // body 字体
		"code{font-family:Menlo;}",                       // code 字体
		"pre,.md-text .highlight{font-family:Consolas;}", // codeblock 字体
		".md-text p:not([class]){text-align:justify;}",   // 对齐
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
	if strings.Contains(css, "--fsp:16px") || strings.Contains(css, "--gap-p:calc(16px + 4px)") {
		t.Errorf("旧 font-size.body 不应参与 1.43.0 渲染: %s", css)
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

	// leftbar 纯色模式（全站仅深色，background_color_dark 无条件生效）
	color := StyleConfig{Leftbar: StyleLeftbar{BackgroundColorDark: "#222"}}
	c2 := string(color.InjectCSS())
	if !strings.Contains(c2, ".l_left .leftbar-container{background:#222;}") {
		t.Errorf("leftbar 纯色注入错误: %s", c2)
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

// TestStyleInjectCSSRadiusGradient 圆角/渐变组注入（Spec 52）：仅显式配置时输出 :root 变量。
func TestStyleInjectCSSRadiusGradient(t *testing.T) {
	s := StyleConfig{
		BorderRadius: StyleBorderRadius{Card: "12px", Block: "6px", Navbar: "24px", Image: "4px"},
		Gradient:     StyleGradient{Start: "linear-gradient(135deg, #ff8a5c, #ff5724)", Searchbar: "linear-gradient(90deg, #f00, #00f)"},
	}
	css := string(s.InjectCSS())
	for _, want := range []string{
		"--radius-card:12px;",
		"--radius-block:6px;",
		"--radius-navbar:24px;",
		"--radius-image:4px;",
		"--gradient-start:linear-gradient(135deg, #ff8a5c, #ff5724);",
		"--gradient-searchbar:linear-gradient(90deg, #f00, #00f);",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("圆角/渐变注入缺 %q\n完整: %s", want, css)
		}
	}
	if !strings.Contains(css, ":root{") {
		t.Errorf("圆角/渐变变量应在 :root 块内，got %s", css)
	}
}
