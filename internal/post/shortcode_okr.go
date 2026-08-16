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

// okrStatusMap 内置状态映射，对齐 Stellar tag_plugins.okr.status 默认值。
var okrStatusMap = map[string]okrStatus{
	"in_track":   {"blue", "正常"},
	"at_risk":    {"yellow", "风险"},
	"off_track":  {"red", "滞后"},
	"finished":   {"green", "完成"},
	"unfinished": {"gray", "未完成"},
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
func okrResolveStatus(status string, percent int) okrStatus {
	if st, ok := okrStatusMap[status]; ok {
		return st
	}
	if percent >= 100 {
		return okrStatusMap["finished"]
	}
	return okrStatusMap["in_track"]
}

// okrLayoutItem 生成单个 okr-item（o 或 kr），对齐 Stellar okr.js 的 layoutItem。
func okrLayoutItem(typ, index, title, note string, st okrStatus, percent int, renderMD func(string) string) string {
	noteHTML := strings.TrimSpace(renderMD(note))
	var sb strings.Builder
	// 安全审计 2026-08-16：index/title 用户可控，转义后再拼入 id 与文本
	fmt.Fprintf(&sb, `<div class="okr-item %s" id="okr-%s">`, typ, html.EscapeString(strings.ToLower(index)))
	fmt.Fprintf(&sb, `<div class="okr-left"><span class="title">%s</span></div>`, html.EscapeString(strings.ToUpper(index)))
	fmt.Fprintf(&sb, `<div class="okr-center"><span class="title">%s</span><div class="note">%s</div></div>`, html.EscapeString(title), noteHTML)
	fmt.Fprintf(&sb, `<div class="okr-right colorful" color="%s"><div class="labels"><span class="status label">%s</span><span class="status percent">%d%%</span></div><div class="progress"><div class="fill" style="width:%d%%;"></div></div></div>`, st.color, st.label, percent, percent)
	sb.WriteString(`</div>`)
	return sb.String()
}

// renderOKR 渲染 {% okr %}...{% endokr %} 为 .tag-plugin.colorful.okr。
func renderOKR(oArgs, content string, renderMD func(string) string) string {
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
		krHTML.WriteString(okrLayoutItem("kr", ki, kt, kn, okrResolveStatus(ks, kp), kp, renderMD))
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
	sb.WriteString(okrLayoutItem("o", oIndex, oTitle, oNote, okrResolveStatus(oStatus, oPercent), oPercent, renderMD))
	sb.WriteString(krHTML.String())
	sb.WriteString(`</div>`)
	return sb.String()
}

// renderQuot 渲染 {% quot el:h2 icon:hashtag text:... %} 为 .tag-plugin.quot 小标题。
func renderQuot(argsStr string, iconFunc func(string) string, tagPlugins settings.TagPluginsConfig) string {
	named, positional := parseNamedPositionalArgs(argsStr, []string{"el", "icon", "prefix", "suffix", "text"}, nil)
	// 安全审计 2026-08-16：el 标签名白名单（h1-h6/p/div/blockquote），非法回退 p，禁止任意标签名注入
	el := safeQuotEl(named["el"])
	text := named["text"]
	if text == "" {
		text = strings.Join(positional, " ")
	}
	// 前缀图标：prefix > icon 预设 > tag_plugins.quot.prefix > 默认 hashtag
	tp := tagPlugins
	prefixKey := named["prefix"]
	if prefixKey == "" {
		if ic := named["icon"]; ic == "" || ic == "default" || ic == "hashtag" {
			prefixKey = tp.Quot.Prefix
			if prefixKey == "" {
				prefixKey = "quot:hashtag"
			}
		} else {
			prefixKey = ic
		}
	}
	suffixKey := named["suffix"]
	if suffixKey == "" {
		suffixKey = tp.Quot.Suffix
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
