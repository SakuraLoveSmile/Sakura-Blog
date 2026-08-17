package post

import (
	"blog/internal/settings"
	"fmt"
	"html"
	"strconv"
	"strings"
	"sync/atomic"
)

func renderTabs(argsStr, body string, renderMD func(string) string) string {
	pairs := splitByMarker(reTabSplit, body)
	if len(pairs) == 0 {
		return ""
	}

	named, _ := parseNamedPositionalArgs(argsStr, []string{"active", "align"}, nil)
	active := 0
	if v, ok := named["active"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			active = n
		}
	}

	tabIdx := atomic.AddUint64(&tabIndexCounter, 1)
	tabName := fmt.Sprintf("tab_%d", tabIdx)
	containerID := strings.ReplaceAll(strings.ToLower(tabName), " ", "-")

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin tabs"`)
	// 安全审计 2026-08-16：align 用户可控，class 词白名单后再拼入
	if align := safeClassWord(named["align"]); align != "" {
		fmt.Fprintf(&sb, ` align="%s"`, html.EscapeString(align))
	}
	fmt.Fprintf(&sb, ` id="%s">`, containerID)

	sb.WriteString(`<div class="nav-tabs">`)
	tabID := 0
	for _, pair := range pairs {
		tabID++
		abbr := fmt.Sprintf("%s %d", tabName, tabID)
		href := strings.ReplaceAll(strings.ToLower(abbr), " ", "-")
		isActive := (active > 0 && active == tabID) || (active == 0 && tabID == 1)
		header := pair.header
		if strings.TrimSpace(header) == "" {
			header = abbr
		}
		if isActive {
			sb.WriteString(`<div class="tab active">`)
		} else {
			sb.WriteString(`<div class="tab">`)
		}
		// 安全审计 2026-08-16：header 来自 <!-- tab xxx --> 注释，作者可控，转义后再输出
		fmt.Fprintf(&sb, `<a href="#%s">%s</a></div>`, href, html.EscapeString(header))
	}
	sb.WriteString(`</div>`)

	sb.WriteString(`<div class="tab-content">`)
	tabID = 0
	for _, pair := range pairs {
		tabID++
		abbr := fmt.Sprintf("%s %d", tabName, tabID)
		href := strings.ReplaceAll(strings.ToLower(abbr), " ", "-")
		isActive := (active > 0 && active == tabID) || (active == 0 && tabID == 1)
		content := strings.TrimSpace(renderMD(pair.body))
		if isActive {
			fmt.Fprintf(&sb, `<div class="tab-pane active" id="%s">%s</div>`, href, content)
		} else {
			fmt.Fprintf(&sb, `<div class="tab-pane" id="%s">%s</div>`, href, content)
		}
	}
	sb.WriteString(`</div></div>`)
	return sb.String()
}

// renderTable 渲染 Stellar 1.40 table 标签。style 仅允许主题提供的三种值，
// 未知值回退 scroll，避免把任意参数写入 class。
func renderTable(argsStr, body string, renderMD func(string) string) string {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"style"}, nil)
	style := named["style"]
	if style != "wrap" && style != "compact" {
		style = "scroll"
	}
	return fmt.Sprintf(`<div class="tag-plugin table table-%s">%s</div>`, style, renderMD(strings.TrimSpace(body)))
}

// renderTip 渲染带悬浮说明的行内提示。正文按 Markdown 渲染后去掉外围段落，
// 与上游 tip.js 的输出保持一致；tooltip 文案始终 HTML 转义。
func renderTip(argsStr, body string, renderMD func(string) string) string {
	named, positional := parseNamedPositionalArgs(argsStr, []string{"text"}, nil)
	text := named["text"]
	if text == "" && len(positional) > 0 {
		text = strings.Join(positional, " ")
	}
	inner := strings.ReplaceAll(renderMD(strings.TrimSpace(body)), "\n", "")
	inner = strings.ReplaceAll(inner, "<p>", "")
	inner = strings.ReplaceAll(inner, "</p>", "")
	return fmt.Sprintf(`<span class="tag-plugin tip" tabindex="0"><span class="tip-text">%s</span><span class="tip-bubble" role="tooltip">%s</span></span>`, inner, html.EscapeString(text))
}

func renderTimeline(body string, renderMD func(string) string, tagPlugins settings.TagPluginsConfig) string {
	pairs := splitByMarker(reNodeSplit, body)

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin timeline"`)
	// tag_plugins.timeline.max_height：限高滚动（空=不限高，现状）
	// 安全审计 2026-08-16：max-height 进 style，走 CSS 数值白名单，非法值丢弃
	if mh := safeCSSNum(tagPlugins.Timeline.MaxHeight); mh != "" {
		fmt.Fprintf(&sb, ` style="max-height:%s;overflow-y:auto"`, mh)
	}
	sb.WriteString(`>`)
	for i, pair := range pairs {
		fmt.Fprintf(&sb, `<div class="timenode" index="%d">`, i)
		sb.WriteString(`<div class="header">`)
		if h := strings.TrimSpace(pair.header); h != "" {
			// 安全审计 2026-08-16：header 来自 <!-- node xxx --> 注释，作者可控，转义后再输出
			fmt.Fprintf(&sb, `<span>%s</span>`, html.EscapeString(h))
		}
		sb.WriteString(`</div>`)
		rendered := strings.TrimSpace(renderMD(pair.body))
		fmt.Fprintf(&sb, `<div class="body fs14">%s</div>`, rendered)
		sb.WriteString(`</div>`)
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

func renderGallery(argsStr, body string, tagPlugins settings.TagPluginsConfig) string {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"layout", "size", "ratio"}, nil)
	// tag_plugins.gallery：默认布局参数（空=内置默认 grid/mix/origin）
	g := tagPlugins.Gallery

	layout := named["layout"]
	if layout == "" {
		layout = g.Layout
	}
	if layout == "" {
		layout = "grid"
	}
	layoutType := "grid"
	if layout == "flow" {
		layoutType = "flow"
	}

	size := named["size"]
	if size == "" {
		size = g.Size
	}
	if size == "" {
		size = "mix"
	}
	ratio := named["ratio"]
	if ratio == "" {
		ratio = g.Ratio
	}
	if ratio == "" {
		ratio = "origin"
	}

	idx := atomic.AddUint64(&galleryIndex, 1)

	var sb strings.Builder
	// 安全审计 2026-08-16：size/ratio 用户可控，转义后再拼入属性
	fmt.Fprintf(&sb, `<div class="tag-plugin gallery %s-box" size="%s" ratio="%s">`, layoutType, html.EscapeString(size), html.EscapeString(ratio))

	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := reGalleryMD.FindStringSubmatch(line)
		if len(m) < 3 {
			continue
		}
		alt, src := m[1], m[2]
		fmt.Fprintf(&sb, `<div class="%s-cell lazy-box">`, layoutType)
		fmt.Fprintf(&sb, `<img class="lazy" data-fancybox="gallery-%d" src="%s" data-src="%s"`, idx, html.EscapeString(src), html.EscapeString(src))
		if alt != "" {
			fmt.Fprintf(&sb, ` alt="%s"`, html.EscapeString(alt))
		}
		sb.WriteString(`/>`)
		sb.WriteString(`<div class="lazy-icon"></div>`)
		sb.WriteString(`<div class="image-meta">`)
		if alt != "" {
			fmt.Fprintf(&sb, `<span class="image-caption">%s</span>`, html.EscapeString(alt))
		}
		sb.WriteString(`</div></div>`)
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

func renderButton(argsStr string, tagPlugins settings.TagPluginsConfig) (string, bool) {
	named, positional := parseNamedPositionalArgs(argsStr, []string{"color", "icon", "size"}, []string{"text", "url"})
	text := named["text"]
	if text == "" && len(positional) > 0 {
		text = positional[0]
	}
	url := named["url"]
	if url == "" && len(positional) > 1 {
		url = positional[1]
	}
	if text == "" || url == "" {
		return "", false
	}

	// Stellar：url 含空格时，最后一段为 url，前面拼回 text
	if strings.Contains(url, " ") {
		parts := strings.Fields(url)
		url = parts[len(parts)-1]
		text = text + " " + strings.Join(parts[:len(parts)-1], " ")
	}
	// 安全审计 2026-08-16：url 用户可控，校验 scheme（拒 javascript:/data:/vbscript: 等），非法不渲染
	if safeURL(url) == "" {
		return "", false
	}

	var sb strings.Builder
	sb.WriteString(`<a class="tag-plugin colorful button"`)
	// 安全审计 2026-08-16：color/size 用户可控，白名单/转义后再拼入
	if color := safeColor(named["color"]); color != "" {
		fmt.Fprintf(&sb, ` color="%s"`, html.EscapeString(color))
	} else if c := tagPlugins.Button.DefaultColor; c != "" {
		// tag_plugins.button.default_color
		fmt.Fprintf(&sb, ` color="%s"`, html.EscapeString(c))
	}
	if size := safeClassWord(named["size"]); size != "" {
		fmt.Fprintf(&sb, ` size="%s"`, html.EscapeString(size))
	}
	fmt.Fprintf(&sb, ` title="%s" href="%s">`, html.EscapeString(text), html.EscapeString(url))
	// icon 暂不渲染 SVG（Stellar 依赖 ctx.utils.icon）
	// 安全审计 2026-08-16：text 用户可控，转义后再输出
	fmt.Fprintf(&sb, `<span>%s</span></a>`, html.EscapeString(text))
	return sb.String(), true
}

func renderCheckbox(typ, argsStr string) (string, bool) {
	named, positional := parseNamedPositionalArgs(argsStr, []string{"color", "checked", "symbol"}, nil)
	checked := named["checked"] == "true"
	// 位置参数中的裸 checked 标记先剔除，剩余 token 拼回文本（{% radio checked 文本 %}）
	var textParts []string
	for _, tok := range positional {
		if tok == "checked" {
			checked = true
			continue
		}
		textParts = append(textParts, tok)
	}
	text := strings.Join(textParts, " ")
	if text == "" {
		return "", false
	}

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin colorful checkbox"`)
	// 安全审计 2026-08-16：color/symbol 用户可控，白名单/转义后再拼入
	if color := safeColor(named["color"]); color != "" {
		fmt.Fprintf(&sb, ` color="%s"`, html.EscapeString(color))
	}
	if symbol := named["symbol"]; symbol != "" {
		fmt.Fprintf(&sb, ` symbol="%s"`, html.EscapeString(symbol))
	}
	sb.WriteString(`>`)
	// 对齐原版 checkbox.js：checkbox/radio 共用同一结构，仅 input type 不同
	fmt.Fprintf(&sb, `<input type="%s"`, typ)
	if checked {
		sb.WriteString(` checked="true"`)
	}
	// 安全审计 2026-08-16：text 用户可控，转义后再输出
	fmt.Fprintf(&sb, `/><span>%s</span></div>`, html.EscapeString(text))
	return sb.String(), true
}
