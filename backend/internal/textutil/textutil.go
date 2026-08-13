// Package textutil 纯文本提取与截断公共工具（Spec 62：从 cmd/server、api、moment
// 三处复制收敛为单一实现）。
package textutil

import (
	"regexp"
	"strings"
)

var (
	htmlTagRe    = regexp.MustCompile(`<[^>]+>`)
	whitespaceRe = regexp.MustCompile(`\s+`)
)

// StripHTML 去除 HTML 标签并压缩空白（搜索索引与 widget 摘要共用的纯文本提取）。
func StripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(whitespaceRe.ReplaceAllString(s, " "))
}

// Truncate 按 rune 截断字符串，超长加省略号（widget 摘要用）。
func Truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
