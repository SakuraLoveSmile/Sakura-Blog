// Package textutil 纯文本提取与截断公共工具（Spec 62：从组合入口、api、moment
// 三处复制收敛为单一实现）。
package textutil

import (
	"net/mail"
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

// ValidEmail 邮箱强校验（安全审计 2026-08-16）：
// net/mail.ParseAddress 解析 + 只接受裸地址（无显示名/注释）+ 拒绝 CR/LF/空白。
// 此前评论/注册仅校验"含 @"，含 CRLF 的邮箱可经回复通知流入 To: 头与
// SMTP Rcpt 命令，构成邮件头注入；所有入库邮箱入口统一走本函数。
func ValidEmail(s string, maxLen int) bool {
	if s == "" || len(s) > maxLen {
		return false
	}
	if strings.ContainsAny(s, "\r\n \t") {
		return false
	}
	addr, err := mail.ParseAddress(s)
	return err == nil && addr != nil && addr.Address == s
}
