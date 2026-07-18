package post

import (
	"regexp"
	"strconv"
	"strings"
)

// goldmark 会把 \(...\)、\[...\] 里的反斜杠当转义吃掉，块级 $$...$$ 里的 \, 等同理。
// 前端 KaTeX auto-render 需要定界符完整出现在 HTML 中，因此在 goldmark 前做轻量保护：
// - $$...$$ 包进 <div>，块级 HTML 内反斜杠不会被再处理
// - \(...\)、\[...\] 及含反斜杠的 $...$ 用 &#92; 编码反斜杠后包进 span/div

var (
	reBlockDollar  = regexp.MustCompile(`(?s)\$\$(.+?)\$\$`)
	reBracketMath  = regexp.MustCompile(`(?s)\\\[((?:\\.|[^\]])*?)\\\]`)
	reParenMath    = regexp.MustCompile(`\\\(((?:\\.|[^\)])*?)\\\)`)
	reInlineDollar = regexp.MustCompile(`\$((?:\\.|[^$\\\n])*(?:\\(?:\\.|[^$\\\n])*)+)\$`)
)

func encodeBackslashes(s string) string {
	return strings.ReplaceAll(s, `\`, `&#92;`)
}

func protectMathDelimiters(src string) string {
	var parts []string
	placeholder := func(html string) string {
		id := len(parts)
		parts = append(parts, html)
		return "\x00MATH" + strconv.Itoa(id) + "\x00"
	}

	src = reBlockDollar.ReplaceAllStringFunc(src, func(m string) string {
		return placeholder("<div>" + m + "</div>")
	})
	src = reBracketMath.ReplaceAllStringFunc(src, func(m string) string {
		return placeholder("<div>" + encodeBackslashes(m) + "</div>")
	})
	src = reParenMath.ReplaceAllStringFunc(src, func(m string) string {
		return placeholder("<span>" + encodeBackslashes(m) + "</span>")
	})
	src = reInlineDollar.ReplaceAllStringFunc(src, func(m string) string {
		if !strings.Contains(m, `\`) {
			return m
		}
		return placeholder("<span>" + encodeBackslashes(m) + "</span>")
	})
	for i, p := range parts {
		src = strings.ReplaceAll(src, "\x00MATH"+strconv.Itoa(i)+"\x00", p)
	}
	return src
}
