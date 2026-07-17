package post

import (
	"fmt"
	"regexp"
	"strings"
)

// note 标签正则：{% note [color:xxx] [title] content %}
var reNote = regexp.MustCompile(`{%\s*note\s+(.*?)\s*%}`)

// folding 标签正则（块级）：{% folding ... %} body {% endfolding %}
var reFolding = regexp.MustCompile(`(?s){%\s*folding\s+(.*?)\s*%}(.*?){%\s*endfolding\s*%}`)

// processShortcodes 在 goldmark 渲染前预处理 note / folding 标签，返回替换后的 Markdown。
// folding body 内的 Markdown 由调用者传入 renderMD 函数单独渲染。
func processShortcodes(md string, renderMD func(string) string) string {
	// 1. 先处理 folding（块级，body 需单独渲染 Markdown）
	md = reFolding.ReplaceAllStringFunc(md, func(match string) string {
		parts := reFolding.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		argsStr := parts[1]
		body := strings.TrimSpace(parts[2])

		color, open, title := parseFoldingArgs(argsStr)

		// body 内的 Markdown 单独渲染
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

	// 2. 再处理 note（单行标签）
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
// 规则：color:xxx 可选；如果只有一个非 color 参数，视为 content（无 title）；
// 两个以上非 color 参数，第一个为 title，剩余拼接为 content。
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
		// 无内容
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
