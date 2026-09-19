package post

import (
	"blog/internal/settings"
	"fmt"
	"html"
	"strconv"
	"strings"
)

type okrStatus struct {
	color string
	label string
}

// parseOKRMeta 解析 "o1 percent:50 status:in_track color:blue"：首 token 为 index，其余为命名参数。
func parseOKRMeta(s string) (index string, percent int, hasPercent bool, status, color string) {
	for i, tok := range splitArgs(s) {
		if i == 0 {
			index = tok
			continue
		}
		switch {
		case strings.HasPrefix(tok, "percent:"):
			if n, err := strconv.Atoi(strings.TrimPrefix(tok, "percent:")); err == nil {
				percent = n
				hasPercent = true
			}
		case strings.HasPrefix(tok, "status:"):
			status = strings.TrimPrefix(tok, "status:")
		case strings.HasPrefix(tok, "color:"):
			color = strings.TrimPrefix(tok, "color:")
		}
	}
	return
}

// okrSplitTitleNote 首行非空为 title，其余为 note。
func okrSplitTitleNote(s string) (title, note string) {
	var lines []string
	for _, ln := range strings.Split(strings.TrimSpace(s), "\n") {
		if strings.TrimSpace(ln) != "" {
			lines = append(lines, strings.TrimSpace(ln))
		}
	}
	if len(lines) == 0 {
		return "", ""
	}
	return lines[0], strings.Join(lines[1:], "\n")
}

// okrResolveStatus 给定 status/percent 解析出配色与文案；status 缺省时按 percent 推断。
func okrResolveStatus(status string, percent int, tagPlugins settings.TagPluginsConfig) okrStatus {
	if configured, ok := tagPlugins.OKR.Status[status]; ok {
		return okrStatus{color: configured.Color, label: configured.Label}
	}
	if percent >= 100 {
		configured := tagPlugins.OKR.Status["finished"]
		return okrStatus{color: configured.Color, label: configured.Label}
	}
	configured := tagPlugins.OKR.Status["in_track"]
	return okrStatus{color: configured.Color, label: configured.Label}
}

// okrLayoutItem 生成单个 okr-item（o 或 kr），对齐 Stellar okr.js 的 layoutItem。
func okrLayoutItem(typ, index, title, note string, st okrStatus, percent int, renderMD func(string) string) string {
	noteHTML := strings.TrimSpace(renderMD(note))
	var sb strings.Builder
	// 安全审计 2026-08-16：index/title 用户可控，转义后再拼入 id 与文本
	fmt.Fprintf(&sb, `<div class="okr-item %s" id="okr-%s">`, typ, html.EscapeString(strings.ToLower(index)))
	fmt.Fprintf(&sb, `<div class="okr-left"><span class="title">%s</span></div>`, html.EscapeString(strings.ToUpper(index)))
	fmt.Fprintf(&sb, `<div class="okr-center"><span class="title">%s</span><div class="note">%s</div></div>`, html.EscapeString(title), noteHTML)
	color := safeColor(st.color)
	fmt.Fprintf(&sb, `<div class="okr-right colorful" color="%s"><div class="labels"><span class="status label">%s</span><span class="status percent">%d%%</span></div><div class="progress"><div class="fill" style="width:%d%%;"></div></div></div>`, html.EscapeString(color), html.EscapeString(st.label), percent, percent)
	sb.WriteString(`</div>`)
	return sb.String()
}

// renderOKR 渲染 {% okr %}...{% endokr %} 为 .tag-plugin.colorful.okr。
func renderOKR(oArgs, content string, renderMD func(string) string, tagPlugins settings.TagPluginsConfig) string {
	tagPlugins.FillDefaults()
	oIndex, oPercent, oHasPercent, oStatus, oColor := parseOKRMeta(oArgs)

	type krItem struct{ meta, body string }
	var krs []krItem
	oBody := content
	if locs := reOKRItem.FindAllStringSubmatchIndex(content, -1); len(locs) > 0 {
		oBody = content[:locs[0][0]]
		for i, loc := range locs {
			meta := content[loc[2]:loc[3]]
			bodyStart := loc[1]
			bodyEnd := len(content)
			if i+1 < len(locs) {
				bodyEnd = locs[i+1][0]
			}
			krs = append(krs, krItem{meta: meta, body: content[bodyStart:bodyEnd]})
		}
	}

	var krHTML strings.Builder
	sum, cnt := 0, 0
	for _, kr := range krs {
		ki, kp, _, ks, _ := parseOKRMeta(kr.meta)
		kt, kn := okrSplitTitleNote(kr.body)
		sum += kp
		cnt++
		krHTML.WriteString(okrLayoutItem("kr", ki, kt, kn, okrResolveStatus(ks, kp, tagPlugins), kp, renderMD))
	}
	if !oHasPercent && cnt > 0 {
		oPercent = sum / cnt
	}
	oTitle, oNote := okrSplitTitleNote(oBody)

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin colorful okr"`)
	// 安全审计 2026-08-16：color 用户可控，白名单后再拼入
	if oColor != "" {
		if c := safeColor(oColor); c != "" {
			fmt.Fprintf(&sb, ` color="%s"`, html.EscapeString(c))
		}
	}
	sb.WriteString(`>`)
	sb.WriteString(okrLayoutItem("o", oIndex, oTitle, oNote, okrResolveStatus(oStatus, oPercent, tagPlugins), oPercent, renderMD))
	sb.WriteString(krHTML.String())
	sb.WriteString(`</div>`)
	return sb.String()
}

// renderQuot 渲染 {% quot el:h2 icon:hashtag text:... %} 为 .tag-plugin.quot 小标题。
func renderQuot(argsStr string, iconFunc func(string) string, tagPlugins settings.TagPluginsConfig) string {
	tagPlugins.FillDefaults()
	named, positional := parseNamedPositionalArgs(argsStr, []string{"el", "icon", "prefix", "suffix", "text"}, nil)
	// 安全审计 2026-08-16：el 标签名白名单（h1-h6/p/div/blockquote），非法回退 p，禁止任意标签名注入
	el := safeQuotEl(named["el"])
	text := named["text"]
	if text == "" {
		text = strings.Join(positional, " ")
	}
	// 上游语义：prefix 或 suffix 任一显式传入时，二者共同覆盖预设；否则由 icon 选择预设。
	preset := tagPlugins.QuotPreset(named["icon"])
	prefixKey, suffixKey := preset.Prefix, preset.Suffix
	if named["prefix"] != "" || named["suffix"] != "" {
		prefixKey, suffixKey = named["prefix"], named["suffix"]
	}

	iconHTML := func(key string) string {
		// 安全审计 2026-08-16：icon key 用户可控，格式白名单后再进入 iconFunc
		key = safeIconKey(key)
		if key == "" || iconFunc == nil {
			return `<span class="empty"></span>`
		}
		return iconFunc(key)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, `<div class="tag-plugin quot %s">`, el)
	// 安全审计 2026-08-16：text 用户可控，id 属性与文本均转义后再输出
	if strings.Contains(el, "h") {
		fmt.Fprintf(&sb, `<%s class="content" id="%s">`, el, html.EscapeString(text))
	} else {
		fmt.Fprintf(&sb, `<%s class="content">`, el)
	}
	sb.WriteString(iconHTML(prefixKey))
	fmt.Fprintf(&sb, `<span class="text">%s</span>`, html.EscapeString(text))
	sb.WriteString(iconHTML(suffixKey))
	fmt.Fprintf(&sb, `</%s></div>`, el)
	return sb.String()
}
