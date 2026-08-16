package textutil

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Anchor 内容标题锚点（1.41 搜索章节化）：ID 为标题元素的 id 属性（跳转锚点），
// Text 为标题文本，Offset 为标题首字符在提取纯文本中的偏移——单位是 UTF-16 code unit，
// 与 JS String.slice/length 的计数一致（Go rune 对 BMP 外字符少计，前端切片会错位）。
type Anchor struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Offset int    `json:"offset"`
}

var headingOpenRe = regexp.MustCompile(`(?i)^<h[1-6]\b`)

// idAttrRe 提取标签内 id 属性值（双引号/单引号）。
var idAttrRe = regexp.MustCompile(`(?i)\bid\s*=\s*("([^"]*)"|'([^']*)')`)

func tagID(tag string) string {
	if m := idAttrRe.FindStringSubmatch(tag); m != nil {
		if m[2] != "" {
			return m[2]
		}
		return m[3]
	}
	return ""
}

// StripHTMLWithAnchors 提取纯文本（空白语义对齐 StripHTML：标签→空格、连续空白压一格、
// 首尾 trim）同时记录 h1-h6 标题锚点及偏移。搜索索引（/search.json anchors 字段）专用；
// 与 StripHTML 的输出在无标题文档上逐字一致。
func StripHTMLWithAnchors(s string) (string, []Anchor) {
	var b strings.Builder
	var anchors []Anchor
	var cur *Anchor
	u16 := 0 // 已输出文本的 UTF-16 长度（offset 基准）
	pendingSpace := false

	flushSpace := func() {
		if pendingSpace {
			if b.Len() > 0 {
				b.WriteByte(' ')
				u16++
				// 标题文本内部空格同样压缩累积，保持 Text 与主文本一致
				if cur != nil && cur.Offset >= 0 && cur.Text != "" {
					cur.Text += " "
				}
			} // 空文本时丢弃前导空格（≈ TrimSpace 左侧语义）
			pendingSpace = false
		}
	}

	for i := 0; i < len(s); {
		if s[i] == '<' {
			end := strings.IndexByte(s[i:], '>')
			if end < 0 {
				break // 未闭合的 '<' 按文本处理不现实，直接截断（与 StripHTML 正则行为一致）
			}
			tag := s[i : i+end+1]
			if headingOpenRe.MatchString(tag) {
				cur = &Anchor{ID: tagID(tag), Offset: -1}
			} else if cur != nil && strings.HasPrefix(strings.ToLower(tag), "</h") {
				if cur.ID != "" || cur.Text != "" {
					anchors = append(anchors, *cur)
				}
				cur = nil
			}
			// 标签按 StripHTML 语义折算为一个空格（连续空白在 flushSpace 去重）
			pendingSpace = true
			i += end + 1
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			i++
			continue
		}
		if unicode.IsSpace(r) {
			pendingSpace = true
			i += size
			continue
		}
		flushSpace()
		if cur != nil {
			if cur.Offset < 0 {
				cur.Offset = u16
			}
			cur.Text += string(r)
		}
		b.WriteRune(r)
		if r <= 0xFFFF {
			u16++
		} else {
			u16 += 2
		}
		i += size
	}
	// 尾部 pendingSpace 未 flush（首部 flushSpace 因 b.Len()==0 丢弃）≈ TrimSpace 语义
	return b.String(), anchors
}

// Utf16Len 返回字符串的 UTF-16 code unit 长度（与 JS String.length 一致），
// 用于把字节偏移换算到 Anchor.Offset 的基准。
func Utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r <= 0xFFFF {
			n++
		} else {
			n += 2
		}
	}
	return n
}
