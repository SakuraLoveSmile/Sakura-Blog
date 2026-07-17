package post

import (
	"regexp"
	"strings"
)

var (
	// tagRE 匹配 HTML 标签，用于从正文 HTML 中剥离标签。
	tagRE = regexp.MustCompile(`<[^>]*>`)
	// spaceRE 匹配连续空白字符，用于折叠空白。
	spaceRE = regexp.MustCompile(`\s+`)
)

// ExcerptFromHTML 从 HTML 片段提取纯文本摘要。
// 流程：去标签 -> 解码常见 HTML 实体 -> 折叠空白 -> 按 rune 截断 maxRunes（追加 "…"）。
// maxRunes <= 0 时不截断。空 HTML 返回空串。
func ExcerptFromHTML(html string, maxRunes int) string {
	s := tagRE.ReplaceAllString(html, " ")
	s = decodeEntities(s)
	s = spaceRE.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if maxRunes > 0 {
		r := []rune(s)
		if len(r) > maxRunes {
			s = string(r[:maxRunes]) + "…"
		}
	}
	return s
}

// decodeEntities 解码摘要中常见的 HTML 实体（&amp; &lt; &gt; &quot; &#39; &#34; &nbsp;）。
// 不引入额外的 html.UnescapeString 依赖，覆盖 goldmark 常见输出即可。
func decodeEntities(s string) string {
	r := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&#34;", `"`,
		"&nbsp;", " ",
	)
	return r.Replace(s)
}
