package post

import (
	"blog/internal/settings"
	"fmt"
	"html"
	"regexp"
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

// renderTimeline 渲染 {% timeline [api:URL] [user:...] [limit:...] [hide:...] %} 节点 {% endtimeline %}。
// 有 api 时进入客户端数据服务模式（对齐原版 timeline.js 的 ds-timeline 容器）：
// 容器结构与本仓库 widgets/timeline.html 一致——class 追加 " data-service ds-timeline"，
// 属性为 data-api + 可选的 user/limit/hide（三者由 services/timeline.js 读取）。
// 原版还允许任意 key 进 data-*，本项目收窄为白名单，避免属性注入面。
// type 参数不支持：本仓库只随包发布 services/timeline.js 一个客户端服务，
// 放行其它值只会产出无人渲染的空容器。
func renderTimeline(argsStr, body string, renderMD func(string) string, tagPlugins settings.TagPluginsConfig) string {
	pairs := splitByMarker(reNodeSplit, body)

	named, _ := parseNamedPositionalArgs(argsStr, []string{"api", "user", "limit", "hide"}, nil)
	// 安全审计 2026-08-30：api 走 scheme 白名单，非法值丢弃（不进入数据服务模式）
	api := safeURL(named["api"])

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin timeline`)
	if api != "" {
		sb.WriteString(` data-service ds-timeline`)
	}
	sb.WriteString(`"`)
	if api != "" {
		fmt.Fprintf(&sb, ` data-api="%s"`, html.EscapeString(api))
	}
	// 安全审计 2026-08-30：透传属性一律转义后再拼入
	for _, key := range []string{"user", "limit", "hide"} {
		if v := named[key]; v != "" {
			fmt.Fprintf(&sb, ` %s="%s"`, key, html.EscapeString(v))
		}
	}
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

// dropdownItem 是下拉项解析结果。
type dropdownItem struct {
	icon  string
	title string
	url   string
}

// reDropdownItem 对齐原版 dropdown.js:12 的单项正则：
// 可选列表前缀（- / *）、可选前缀 icon:key、[title](url)、可选后缀 icon:key。
var reDropdownItem = regexp.MustCompile(`^\s*(?:[-*]\s+)?(?:icon:([^\s]+)\s+)?\[([^\]]+)\]\(([^)\s]+)\)(?:\s+icon:([^\s]+))?\s*$`)

func parseDropdownItems(body string) []dropdownItem {
	var items []dropdownItem
	for _, line := range strings.Split(body, "\n") {
		m := reDropdownItem.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		icon := m[1]
		if icon == "" {
			icon = m[4]
		}
		items = append(items, dropdownItem{icon: icon, title: m[2], url: m[3]})
	}
	return items
}

// renderDropdown 渲染 {% dropdown [direction:up/down] [align:left/right] [open:true] title %}
// - icon:key [title](url)
// {% enddropdown %}
// DOM 对齐原版 dropdown.js:39-62。icon 复用已注入的图标注册表；已知键渲染对应 SVG，未知键安全回退。
func renderDropdown(argsStr, body string, iconFunc func(string) string) string {
	named, positional := parseNamedPositionalArgs(argsStr, []string{"direction", "align", "open"}, []string{"title"})
	title := named["title"]
	if title == "" && len(positional) > 0 {
		title = strings.Join(positional, " ")
	}
	items := parseDropdownItems(body)
	if len(items) == 0 {
		return ""
	}
	if strings.TrimSpace(title) == "" {
		title = "菜单"
	}

	// 安全审计 2026-08-31：direction/align 走白名单，非法值回退 auto（与原版一致）
	direction := "auto"
	if named["direction"] == "up" || named["direction"] == "down" {
		direction = named["direction"]
	}
	align := "auto"
	if named["align"] == "left" || named["align"] == "right" {
		align = named["align"]
	}
	open := named["open"] == "true"

	var sb strings.Builder
	// 安全审计 2026-08-31：direction/align 白名单后转义拼入
	fmt.Fprintf(&sb, `<details class="tag-plugin dropdown" direction="%s" align="%s"`, html.EscapeString(direction), html.EscapeString(align))
	if open {
		sb.WriteString(` open`)
	}
	sb.WriteString(`>`)
	// 安全审计 2026-08-31：title 用户可控，转义后再输出（summary 的 title/aria-label 与文本三处）
	fmt.Fprintf(&sb, `<summary class="dropdown-trigger" title="%s" aria-label="%s"><span>%s</span>`, html.EscapeString(title), html.EscapeString(title), html.EscapeString(title))
	sb.WriteString(`<svg class="dropdown-arrow" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M6 9l6 6 6-6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>`)
	sb.WriteString(`</summary>`)
	sb.WriteString(`<div class="dropdown-menu ui-collection" data-ui-surface="glass" data-layout="list" data-variant="nav" data-density="compact">`)
	for _, it := range items {
		// 安全审计 2026-08-31：url 用户可控，scheme 白名单（拒 javascript:/data: 等），非法项跳过
		url := safeURL(it.url)
		if url == "" {
			continue
		}
		external := strings.Contains(url, "://")
		fmt.Fprintf(&sb, `<a class="dropdown-item ui-collection__item card-hover card-hover--spotlight" href="%s"`, html.EscapeString(url))
		if external {
			sb.WriteString(` target="_blank" rel="external nofollow noopener noreferrer">`)
		} else {
			sb.WriteString(` rel="noopener noreferrer">`)
		}
		if it.icon != "" {
			// 安全审计 2026-08-31 / 阶段 E：icon key 白名单校验，已知键渲染对应 SVG，未知键安全回退
			sb.WriteString(`<span class="ui-collection__leading">`)
			k := safeIconKey(it.icon)
			if k != "" && iconFunc != nil {
				sb.WriteString(iconFunc(k))
			} else if k != "" {
				fmt.Fprintf(&sb, `<span class="icon-missing" data-icon="%s"></span>`, html.EscapeString(k))
			}
			sb.WriteString(`</span>`)
		}
		// 安全审计 2026-08-31：title 用户可控，转义后再输出
		fmt.Fprintf(&sb, `<span class="ui-collection__content"><span class="ui-collection__title">%s</span></span></a>`, html.EscapeString(it.title))
	}
	sb.WriteString(`</div></details>`)
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
