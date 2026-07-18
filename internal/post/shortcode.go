package post

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
)

// note 标签正则：{% note [color:xxx] [title] content %}
var reNote = regexp.MustCompile(`{%\s*note\s+(.*?)\s*%}`)

// folding 标签正则（块级）：{% folding ... %} body {% endfolding %}
var reFolding = regexp.MustCompile(`(?s){%\s*folding\s+(.*?)\s*%}(.*?){%\s*endfolding\s*%}`)

// tabs 标签正则（块级）：{% tabs ... %} body {% endtabs %}
var reTabs = regexp.MustCompile(`(?s){%\s*tabs\s*(.*?)\s*%}(.*?){%\s*endtabs\s*%}`)

// timeline 标签正则（块级）：{% timeline ... %} body {% endtimeline %}
var reTimeline = regexp.MustCompile(`(?s){%\s*timeline\s*(.*?)\s*%}(.*?){%\s*endtimeline\s*%}`)

// gallery 标签正则（块级）：{% gallery ... %} body {% endgallery %}
var reGallery = regexp.MustCompile(`(?s){%\s*gallery\s*(.*?)\s*%}(.*?){%\s*endgallery\s*%}`)

// mermaid 标签正则（块级）：{% mermaid %} body {% endmermaid %}
var reMermaid = regexp.MustCompile(`(?s){%\s*mermaid\s*%}(.*?){%\s*endmermaid\s*%}`)

// button 标签正则：{% button ... %}
var reButton = regexp.MustCompile(`{%\s*button\s+(.*?)\s*%}`)

// checkbox 标签正则：{% checkbox ... %}
var reCheckbox = regexp.MustCompile(`{%\s*checkbox\s+(.*?)\s*%}`)

var (
	reTabSplit  = regexp.MustCompile(`<!--\s*tab (.*?)\s*-->`)
	reNodeSplit = regexp.MustCompile(`<!--\s*node (.*?)\s*-->`)
	reGalleryMD = regexp.MustCompile(`!\[(.*?)\]\((.*?)\)`)

	tabIndexCounter uint64
	galleryIndex    uint64
)

// processShortcodes 在 goldmark 渲染前预处理 Stellar 标签插件，返回替换后的 Markdown。
// 块级 body 内的 Markdown 由调用者传入 renderMD 函数单独渲染。
func processShortcodes(md string, renderMD func(string) string) string {
	// 1. mermaid（块级，body 为原文，不渲染 Markdown）
	md = reMermaid.ReplaceAllStringFunc(md, func(match string) string {
		parts := reMermaid.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		body := strings.TrimSpace(parts[1])
		return fmt.Sprintf(`<div class="mermaid">%s</div>`, html.EscapeString(body))
	})

	// 2. folding（块级，body 需单独渲染 Markdown）
	md = reFolding.ReplaceAllStringFunc(md, func(match string) string {
		parts := reFolding.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		argsStr := parts[1]
		body := strings.TrimSpace(parts[2])

		color, open, title := parseFoldingArgs(argsStr)
		renderedBody := renderMD(body)

		var sb strings.Builder
		sb.WriteString(`<details class="tag-plugin colorful folding"`)
		if color != "" {
			fmt.Fprintf(&sb, ` color="%s"`, color)
		}
		if open {
			sb.WriteString(` open`)
		}
		sb.WriteString(`>`)
		if title != "" {
			fmt.Fprintf(&sb, `<summary>%s</summary>`, title)
		}
		fmt.Fprintf(&sb, `<div class="body">%s</div>`, renderedBody)
		sb.WriteString(`</details>`)
		return sb.String()
	})

	// 3. timeline（块级）
	md = reTimeline.ReplaceAllStringFunc(md, func(match string) string {
		parts := reTimeline.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		body := strings.TrimSpace(parts[2])
		return renderTimeline(body, renderMD)
	})

	// 4. gallery（块级）
	md = reGallery.ReplaceAllStringFunc(md, func(match string) string {
		parts := reGallery.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		argsStr := parts[1]
		body := strings.TrimSpace(parts[2])
		return renderGallery(argsStr, body)
	})

	// 5. tabs（块级）
	md = reTabs.ReplaceAllStringFunc(md, func(match string) string {
		parts := reTabs.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		argsStr := parts[1]
		body := strings.TrimSpace(parts[2])
		return renderTabs(argsStr, body, renderMD)
	})

	// 6. button（单行）
	md = reButton.ReplaceAllStringFunc(md, func(match string) string {
		parts := reButton.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		el, ok := renderButton(parts[1])
		if !ok {
			return match
		}
		return el
	})

	// 7. checkbox（单行）
	md = reCheckbox.ReplaceAllStringFunc(md, func(match string) string {
		parts := reCheckbox.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		el, ok := renderCheckbox(parts[1])
		if !ok {
			return match
		}
		return el
	})

	// 8. note（单行）
	md = reNote.ReplaceAllStringFunc(md, func(match string) string {
		parts := reNote.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		argsStr := parts[1]
		color, title, content := parseNoteArgs(argsStr)

		var sb strings.Builder
		sb.WriteString(`<div class="tag-plugin colorful note"`)
		if color != "" {
			fmt.Fprintf(&sb, ` color="%s"`, color)
		}
		sb.WriteString(`>`)
		if title != "" {
			fmt.Fprintf(&sb, `<div class="title">%s</div>`, title)
		}
		fmt.Fprintf(&sb, `<div class="body">%s</div>`, content)
		sb.WriteString(`</div>`)
		return sb.String()
	})

	return md
}

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
	if align := named["align"]; align != "" {
		fmt.Fprintf(&sb, ` align="%s"`, align)
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
		fmt.Fprintf(&sb, `<a href="#%s">%s</a></div>`, href, header)
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

func renderTimeline(body string, renderMD func(string) string) string {
	pairs := splitByMarker(reNodeSplit, body)

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin timeline">`)
	for i, pair := range pairs {
		fmt.Fprintf(&sb, `<div class="timenode" index="%d">`, i)
		sb.WriteString(`<div class="header">`)
		if h := strings.TrimSpace(pair.header); h != "" {
			fmt.Fprintf(&sb, `<span>%s</span>`, h)
		}
		sb.WriteString(`</div>`)
		rendered := strings.TrimSpace(renderMD(pair.body))
		fmt.Fprintf(&sb, `<div class="body fs14">%s</div>`, rendered)
		sb.WriteString(`</div>`)
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

func renderGallery(argsStr, body string) string {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"layout", "size", "ratio"}, nil)

	layout := named["layout"]
	if layout == "" {
		layout = "grid"
	}
	layoutType := "grid"
	if layout == "flow" {
		layoutType = "flow"
	}

	size := named["size"]
	if size == "" {
		size = "mix"
	}
	ratio := named["ratio"]
	if ratio == "" {
		ratio = "origin"
	}

	idx := atomic.AddUint64(&galleryIndex, 1)

	var sb strings.Builder
	fmt.Fprintf(&sb, `<div class="tag-plugin gallery %s-box" size="%s" ratio="%s">`, layoutType, size, ratio)

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
		fmt.Fprintf(&sb, `<img class="lazy" data-fancybox="gallery-%d" data-src="%s"`, idx, html.EscapeString(src))
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

func renderButton(argsStr string) (string, bool) {
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

	var sb strings.Builder
	sb.WriteString(`<a class="tag-plugin colorful button"`)
	if color := named["color"]; color != "" {
		fmt.Fprintf(&sb, ` color="%s"`, color)
	}
	if size := named["size"]; size != "" {
		fmt.Fprintf(&sb, ` size="%s"`, size)
	}
	fmt.Fprintf(&sb, ` title="%s" href="%s">`, html.EscapeString(text), html.EscapeString(url))
	// icon 暂不渲染 SVG（Stellar 依赖 ctx.utils.icon）
	fmt.Fprintf(&sb, `<span>%s</span></a>`, text)
	return sb.String(), true
}

func renderCheckbox(argsStr string) (string, bool) {
	named, positional := parseNamedPositionalArgs(argsStr, []string{"color", "checked", "symbol"}, []string{"text"})
	text := named["text"]
	checked := named["checked"] == "true"
	var textParts []string
	for _, tok := range positional {
		if tok == "checked" {
			checked = true
			continue
		}
		textParts = append(textParts, tok)
	}
	if text == "" && len(textParts) > 0 {
		text = strings.Join(textParts, " ")
	}
	if text == "" {
		return "", false
	}

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin colorful checkbox"`)
	if color := named["color"]; color != "" {
		fmt.Fprintf(&sb, ` color="%s"`, color)
	}
	if symbol := named["symbol"]; symbol != "" {
		fmt.Fprintf(&sb, ` symbol="%s"`, symbol)
	}
	sb.WriteString(`>`)
	sb.WriteString(`<input type="checkbox"`)
	if checked {
		sb.WriteString(` checked="true"`)
	}
	fmt.Fprintf(&sb, `/><span>%s</span></div>`, text)
	return sb.String(), true
}

type markerPair struct {
	header string
	body   string
}

// splitByMarker 按 <!-- marker xxx --> 注释切分内容为 header/body 对（对齐 Stellar tabs.js / timeline.js）。
func splitByMarker(re *regexp.Regexp, content string) []markerPair {
	indices := re.FindAllStringSubmatchIndex(content, -1)
	if len(indices) == 0 {
		return nil
	}

	pairs := make([]markerPair, 0, len(indices))
	for i, loc := range indices {
		header := content[loc[2]:loc[3]]
		bodyStart := loc[1]
		bodyEnd := len(content)
		if i+1 < len(indices) {
			bodyEnd = indices[i+1][0]
		}
		pairs = append(pairs, markerPair{
			header: header,
			body:   content[bodyStart:bodyEnd],
		})
	}
	return pairs
}

// parseNamedPositionalArgs 解析 key:value 命名参数与位置参数（对齐 Stellar ctx.args.map）。
func parseNamedPositionalArgs(args string, namedKeys, positionalKeys []string) (map[string]string, []string) {
	namedSet := make(map[string]struct{}, len(namedKeys))
	for _, k := range namedKeys {
		namedSet[k] = struct{}{}
	}

	result := make(map[string]string)
	var positional []string

	for _, tok := range splitArgs(args) {
		isNamed := false
		for k := range namedSet {
			if strings.HasPrefix(tok, k+":") {
				result[k] = strings.TrimPrefix(tok, k+":")
				isNamed = true
				break
			}
		}
		if !isNamed {
			positional = append(positional, tok)
		}
	}

	for i, k := range positionalKeys {
		if i < len(positional) {
			result[k] = positional[i]
		}
	}
	return result, positional
}

// parseFoldingArgs 解析 folding 开标签参数：[color:xxx] [open:true/false] title
func parseFoldingArgs(args string) (color string, open bool, title string) {
	args = strings.TrimSpace(args)
	var remaining []string

	for _, tok := range splitArgs(args) {
		if strings.HasPrefix(tok, "color:") {
			color = strings.TrimPrefix(tok, "color:")
		} else if strings.HasPrefix(tok, "open:") {
			open = strings.TrimPrefix(tok, "open:") == "true"
		} else {
			remaining = append(remaining, tok)
		}
	}

	title = strings.Join(remaining, " ")
	return
}

// parseNoteArgs 解析 note 标签参数：[color:xxx] [title] content
func parseNoteArgs(args string) (color, title, content string) {
	args = strings.TrimSpace(args)
	var remaining []string

	for _, tok := range splitArgs(args) {
		if strings.HasPrefix(tok, "color:") {
			color = strings.TrimPrefix(tok, "color:")
		} else {
			remaining = append(remaining, tok)
		}
	}

	switch len(remaining) {
	case 0:
	case 1:
		content = remaining[0]
	default:
		title = remaining[0]
		content = strings.Join(remaining[1:], " ")
	}
	return
}

// splitArgs 按空白切分参数，但保留引号内的空格。
func splitArgs(s string) []string {
	var tokens []string
	var cur strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote {
			if c == quoteChar {
				inQuote = false
			} else {
				cur.WriteByte(c)
			}
		} else if c == '"' || c == '\'' {
			inQuote = true
			quoteChar = c
		} else if c == ' ' || c == '\t' {
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		} else {
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}
