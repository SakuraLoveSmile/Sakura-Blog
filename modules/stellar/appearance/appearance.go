// Package appearance 消费 Stellar 2.0 规范化后的完整外观配置。
// CSS 公式和选择器来自锁定的 9d11978 source/css；生产 Go 不调用 Node/Hexo。
package appearance

import (
	"html/template"
	"math"
	"sort"
	"strconv"
	"strings"
)

type declarations map[string]string

func rule(b *strings.Builder, selector string, d declarations) {
	if len(d) == 0 {
		return
	}
	b.WriteString(selector)
	b.WriteByte('{')
	keys := make([]string, 0, len(d))
	for k := range d {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte(':')
		b.WriteString(d[k])
		b.WriteByte(';')
	}
	b.WriteByte('}')
}

// Build 接收 stellarv2.ParseTheme 的完整 camelCase 运行时结果。
// 返回样式必须放在所有主样式、选配和最终外观层之后。
// 现代颜色使用浏览器相对 HSL；未定义变量保持 CSS 原有无效值行为，不偷偷换色。
func Build(theme map[string]any) (template.CSS, error) {
	return BuildWithOptions(theme, Options{})
}

// BuildWithOptions 在生产部署指定的固定资源基址上生成样式，不依赖文章路径。
func BuildWithOptions(theme map[string]any, options Options) (template.CSS, error) {
	rootURL, cssURLBase, err := resourceBases(options)
	if err != nil {
		return "", err
	}
	r := reader{theme: theme}
	var b strings.Builder
	root := declarations{}
	preset := r.choice("appearance.preset", "card", "glass", "minimal", "flat")
	r.choice("appearance.colorScheme", "auto", "light", "dark") // data-theme 由调用方和运行时管理。
	primary := r.text("appearance.colors.primary")
	accent := r.text("appearance.colors.accent")
	link := r.text("appearance.colors.link")
	sourceTokens(root, primary)
	root["--accent"] = colorLiteral(accent)
	root["--link"] = colorAlpha(link, "1")
	root["--link-a20"] = colorAlpha(link, "0.2")
	for _, pair := range [][2]string{
		{"appearance.gradients.primaryAction", "--gradient-start"}, {"appearance.gradients.searchBar", "--gradient-searchbar"},
		{"appearance.typography.fontFamily.body", "--font-family-body"}, {"appearance.typography.fontFamily.code", "--font-family-code"},
	} {
		root[pair[1]] = r.text(pair[0])
	}
	root["--font-smoothing"] = r.choice("appearance.typography.fontSmoothing", "auto", "none", "antialiased")
	root["--content-align"] = r.choice("appearance.typography.contentAlign", "left", "center", "right", "justify")
	for _, pair := range [][2]string{{"root", "--font-size-root"}, {"inlineCode", "--font-size-inline-code"}, {"codeBlock", "--font-size-code-block"}} {
		root[pair[1]] = r.length("appearance.typography.fontSize." + pair[0])
	}
	for _, weight := range []string{"100", "200", "300", "400", "500", "600", "700", "800", "900"} {
		root["--font-weight-"+weight] = num(r.number("appearance.typography.fontWeight."+weight, 1, 1000))
	}
	root["--corner-shape"] = r.text("appearance.shape.corner")
	for _, pair := range [][2]string{{"cardLarge", "--radius-card-l"}, {"card", "--radius-card"}, {"cardSmall", "--radius-card-s"}, {"bar", "--radius-bar"}, {"imageLarge", "--radius-image-l"}, {"image", "--radius-image"}, {"imageSmall", "--radius-image-s"}} {
		root[pair[1]] = r.length("appearance.shape.radius." + pair[0])
	}
	root["--code-scrollbar-width"] = r.length("appearance.codeBlock.scrollbarWidth")
	root["--checkbox-pointer-events"] = "none"
	if r.boolean("tags.checkbox.interactive") {
		root["--checkbox-pointer-events"] = "auto"
	}
	root["--note-border"] = "none"
	if r.boolean("tags.note.border") {
		root["--note-border"] = "1px solid var(--theme-border)"
	}
	highlight := resolvedResource(r.resource("appearance.codeBlock.highlightStylesheet"), rootURL)
	if highlight != "" && !options.SkipHighlightStylesheet {
		b.WriteString("@import ")
		b.WriteString(cssURL(highlight))
		b.WriteByte(';')
	}
	rule(&b, ":root", root)
	palette(&b, primary, accent)
	directConsumers(&b)
	// 上游 list.styl / pin-slider.styl 的编译期配置，动态主题在主样式之后覆盖。
	coverRatio := strconv.FormatFloat(r.number("article.listing.coverRatio", math.SmallestNonzeroFloat64, math.MaxFloat64), 'f', -1, 64)
	for _, selector := range []string{".post-list .post-card .post-cover img", ".post-list .post-card.post.photo .cover img", ".pin-slide.post-slide"} {
		rule(&b, selector, declarations{"aspect-ratio": coverRatio})
	}
	// story-title() 原来直接使用编译期强调色，需要覆盖到配置变量。
	rule(&b, "body[data-article-style='story'] .md-text.content h2:not([class]):before,body[data-article-style='story'] .md-text.content h2:not([class]):after,.post-list.topic .post-card.topic article .topic-title:before,.post-list.topic .post-card.topic article .topic-title:after", declarations{"background": "var(--accent)"})
	pageImage := resolvedResource(r.resource("appearance.backgrounds.page.image"), cssURLBase)
	page := declarations{"background-image": "none"}
	if pageImage != "" {
		page["background-image"] = cssURL(pageImage)
	}
	rule(&b, "body > .site-background", page)
	saturation := r.text("appearance.backgrounds.page.backdrop.saturation")
	if !percentagePattern.MatchString(saturation) {
		r.fail("appearance.backgrounds.page.backdrop.saturation", "CSS percentage")
	}
	rule(&b, "body > .site-background .site-background__blur", declarations{"--blur-px": r.length("appearance.backgrounds.page.backdrop.radius"), "--blur-sat": saturation, "--blur-bg": r.text("appearance.backgrounds.page.backdrop.overlay")})
	border := "var(--block-border)"
	if pageImage != "" {
		border = "transparent"
	}
	rule(&b, ".cmt-body .artalk,.cmt-body .atk-layer-wrap,.cmt-body .artalk.atk-dark-mode,.cmt-body .atk-layer-wrap.atk-dark-mode", declarations{"--at-color-border": border})
	background(&b, &r, preset, cssURLBase)
	if r.err != nil {
		return "", r.err
	}
	return template.CSS(b.String()), nil
}
func sourceTokens(d declarations, source string) {
	d["--stellar-source-color"] = source
	if c, ok := parseColor(source); ok {
		d["--hue"] = num(c.h) + "deg"
		d["--sat"] = num(c.s) + "%"
		d["--light"] = num(c.l) + "%"
		d["--alpha"] = num(c.a)
		for _, a := range []string{"1", "0.1", "0.2", "0.3"} {
			k := "--theme"
			if a != "1" {
				k += "-a" + strings.ReplaceAll(strings.TrimPrefix(a, "0."), ".", "") + "0"
			}
			d[k] = "hsla(var(--hue) var(--sat) var(--light) / " + a + ")"
		}
	} else {
		// 纯 CSS 无法把浏览器变量拆成独立数值通道；清除旧编译通道，
		// 并由相对色完整覆盖其派生 token 与直接消费者。
		d["--hue"] = "initial"
		d["--sat"] = "initial"
		d["--light"] = "initial"
		d["--alpha"] = "initial"
		d["--theme"] = relative("var(--stellar-source-color)", "s", "l", "1")
		d["--theme-a10"] = relative("var(--stellar-source-color)", "s", "l", "0.1")
		d["--theme-a20"] = relative("var(--stellar-source-color)", "s", "l", "0.2")
		d["--theme-a30"] = relative("var(--stellar-source-color)", "s", "l", "0.3")
	}
}
func colorful(source string, dark bool) declarations {
	d := declarations{}
	sourceTokens(d, source)
	c, numeric := parseColor(source)
	hsla := func(s, l, a string) string {
		if numeric {
			return "hsla(" + num(c.h) + "deg " + s + " " + l + " / " + a + ")"
		}
		if a == "var(--alpha)" {
			a = "alpha"
		}
		return relative("var(--stellar-source-color)", s, l, a)
	}
	border, block, mark, code := [2]string{"50%", "80%"}, [2]string{"90%", "92%"}, [2]string{"95%", "80%"}, [2]string{"50%", "95%"}
	textS, textL, pS, pL, linkS, linkL := "55%", "16%", "90%", "20%", "95%", "32%"
	if dark {
		border = [2]string{"40%", "24%"}
		block = [2]string{"24%", "24%"}
		mark = [2]string{"50%", "32%"}
		code = [2]string{"12%", "24%"}
		textS, textL, pS, pL, linkS, linkL = "50%", "75%", "80%", "80%", "80%", "60%"
	}
	for _, v := range []struct {
		name  string
		color [2]string
	}{{"--theme-border", border}, {"--theme-block", block}, {"--theme-mark", mark}, {"--theme-codeblock", code}} {
		d[v.name] = hsla(v.color[0], v.color[1], "var(--alpha)")
	}
	d["--text"] = hsla(textS, textL, "1")
	for i, a := range []string{"1", "0.8", "0.6", "0.4"} {
		d["--text-p"+num(float64(i+1))] = hsla(pS, pL, a)
	}
	d["--link"] = hsla(linkS, linkL, "1")
	d["--link-a20"] = hsla(linkS, linkL, "0.2")
	return d
}
func palette(b *strings.Builder, primary, accent string) {
	entries := [][2]string{{"red", "#f44336"}, {"orange", "#fa6400"}, {"amber", "#ffbd2b"}, {"yellow", "#ffe659"}, {"green", "#3dc550"}, {"cyan", "#1bcdfc"}, {"blue", "#2196f3"}, {"purple", "#9c27b0"}, {"theme", primary}, {"accent", accent}}
	// 保持 x-set-theme-colorful 的级联顺序：先暗色 tag，再命名色。
	for _, entry := range entries {
		rule(b, ".colorful[color='"+entry[0]+"']", colorful(entry[1], false))
	}
	emitDark := func(prefix string) {
		rule(b, prefix+" .tag-plugin.tag", colorful(primary, true))
		for _, entry := range entries {
			rule(b, prefix+" .colorful[color='"+entry[0]+"']", colorful(entry[1], true))
		}
	}
	emitDark(":root[data-theme='dark']")
	b.WriteString("@media (prefers-color-scheme:dark){")
	emitDark(":root:not([data-theme])")
	b.WriteByte('}')
	// warning/error 自带数值色相；light/dark 只改 --theme，
	// 必须让直接 HSLA 消费者继续继承上层来源色相。
	for _, entry := range [][2]string{{"warning", "#ffe659"}, {"error", "#f44336"}} {
		rule(b, ".colorful[color='"+entry[0]+"']", declarations{"--stellar-source-color": entry[1]})
	}
	rule(b, ".tag-plugin.ds-vote .body button.vote-up", declarations{"--stellar-source-color": "#3dc550"})
	rule(b, ".tag-plugin.ds-vote .body button.vote-down", declarations{"--stellar-source-color": "#f44336"})
}
func directConsumers(b *strings.Builder) {
	for _, v := range []struct{ selector, property, alpha string }{
		{".related-wrap#comments .cmt-title p a", "--link-a20", "0.2"},
		{".md-text .tag-plugin.copy span", "background", "0.2"},
		{"body[data-article-style='story'] .md-text.content h3:not([class]):before,body[data-article-style='story'] .md-text.content h3:not([class]):after", "background-color", "0.5"},
		{"body[data-article-style='story'] .md-text.content h4:not([class]):before", "background", "0.3"},
		{"body[data-article-style='story'] .md-text.content h4:not([class]):hover:before", "background", "1"},
		{"body[data-article-style='story'] .md-text.content>blockquote:before,.md-text.content .tag-plugin.blockquote:before,body[data-article-style='story'] .md-text.content>blockquote:after,.md-text.content .tag-plugin.blockquote:after", "background-color", "0.25"},
		{"body[data-article-style='story'] .md-text.content>blockquote:hover:before,.md-text.content .tag-plugin.blockquote:hover:before,body[data-article-style='story'] .md-text.content>blockquote:hover:after,.md-text.content .tag-plugin.blockquote:hover:after", "background-color", "0.5"},
		{".highlight .code .copy-btn:hover", "color", "1"},
	} {
		rule(b, v.selector, declarations{v.property: relative("var(--stellar-source-color)", "s", "l", v.alpha)})
	}
}
