package textutil

import (
	"strings"
	"testing"
)

// TestValidEmail 安全审计 2026-08-16 回归：邮箱强校验拒 CRLF/空白/显示名，
// 该入口此前可经评论回复通知流入 To: 头构成 SMTP 注入。
func TestValidEmail(t *testing.T) {
	const maxLen = 254
	valid := []string{
		"user@example.com",
		"nick.name+tag@sub.example.org",
		"a@b.cn",
	}
	invalid := []string{
		"",
		"a@b.com\r\nBcc: evil@x.com", // SMTP 头注入载荷
		"a@b.com\r\n.",
		"a b@c.com",                         // 空白
		"Bob <a@b.com>",                     // 显示名形式
		"a@b.com>",                          // 残留尖括号
		"no-at-sign",                        // 无 @
		"@example.com",                      // 缺本地部分
		strings.Repeat("a", 250) + "@x.com", // 超长（>254）
	}
	for _, s := range valid {
		if !ValidEmail(s, maxLen) {
			t.Errorf("ValidEmail(%q) = false, want true", s)
		}
	}
	for _, s := range invalid {
		if ValidEmail(s, maxLen) {
			t.Errorf("ValidEmail(%q) = true, want false", s)
		}
	}
}
