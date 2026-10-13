package stellarmigration

import (
	"fmt"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"strings"
	"unicode"
)

var extensionTags = map[string]string{"about": "blog_about", "links": "blog_links", "moments": "blog_moments", "ghrepo": "blog_ghrepo", "ghuser": "blog_ghuser", "tip": "blog_tip"}
var officialTags = func() map[string]bool {
	m := map[string]bool{}
	for _, n := range strings.Fields("tabs tab box folding folders folder grid cell swiper gallery banner dropdown friends albums posters sites ghcard gist toc timeline timenode md chat checkbox radio copy emoji icon frame image link button mark navbar note poetry quot blockquote hashtag mbti okr audio video rating vote table tip reel paper u emp wavy del kbd psw blur sup sub") {
		m[n] = true
	}
	for _, n := range extensionTags {
		m[n] = true
	}
	return m
}()

// This scanner only renames the explicitly approved extension tag names. It
// never reconstructs arguments/body and skips fenced/indented/inline code.
// Ambiguous syntax is retained verbatim and blocks the candidate.
func migrateMarkdown(md, source string, ds []Diagnostic, ms []Mapping) (string, []Diagnostic, []Mapping) {
	out := strings.Builder{}
	fence := byte(0)
	fenceLen := 0
	protected := protectedCode(md)
	for start := 0; start < len(md); {
		end := strings.IndexByte(md[start:], '\n')
		if end < 0 {
			end = len(md)
		} else {
			end += start + 1
		}
		line := md[start:end]
		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)
		if indent <= 3 && len(trimmed) > 0 && (trimmed[0] == '`' || trimmed[0] == '~') {
			n := runLength(trimmed, 0, trimmed[0])
			if n >= 3 {
				if fence == 0 {
					fence = trimmed[0]
					fenceLen = n
					out.WriteString(line)
					start = end
					continue
				}
				if fence == trimmed[0] && n >= fenceLen && strings.TrimSpace(trimmed[n:]) == "" {
					fence = 0
					fenceLen = 0
					out.WriteString(line)
					start = end
					continue
				}
			}
		}
		if fence != 0 || indent >= 4 || strings.HasPrefix(line, "\t") {
			out.WriteString(line)
			start = end
			continue
		}
		for i := 0; i < len(line); {
			if protected[start+i] || !strings.HasPrefix(line[i:], "{%") || escaped(line, i) {
				out.WriteByte(line[i])
				i++
				continue
			}
			close := strings.Index(line[i+2:], "%}")
			if close < 0 {
				ds = append(ds, Diagnostic{Code: "ambiguous_shortcode", Source: source, Path: fmt.Sprintf("markdown.byte[%d]", start+i), Message: "跨行或未闭合 shortcode 不可靠转换；原文保留", Blocking: true})
				out.WriteString(line[i:])
				break
			}
			close += i + 2
			token := line[i : close+2]
			inner := line[i+2 : close]
			if !balancedQuotes(inner) {
				ds = append(ds, Diagnostic{Code: "ambiguous_shortcode", Source: source, Path: fmt.Sprintf("markdown.byte[%d]", start+i), Message: "参数引号不完整或包含不可靠关闭符，保持原文", Blocking: true})
				out.WriteString(token)
				i = close + 2
				continue
			}
			leading := len(inner) - len(strings.TrimLeftFunc(inner, unicode.IsSpace))
			rest := inner[leading:]
			nameEnd := strings.IndexFunc(rest, unicode.IsSpace)
			if nameEnd < 0 {
				nameEnd = len(rest)
			}
			name := rest[:nameEnd]
			base := strings.TrimPrefix(name, "end")
			closing := base != name
			if replacement, ok := extensionTags[base]; ok {
				if closing {
					replacement = "end" + replacement
				}
				token = "{%" + inner[:leading] + replacement + rest[nameEnd:] + "%}"
				ms = append(ms, Mapping{fmt.Sprintf("%s.markdown.byte[%d]", source, start+i), replacement, "只改批准的 Blog 扩展名字；参数、正文及空白逐字保留"})
			} else if !officialTags[base] {
				ds = append(ds, Diagnostic{Code: "unknown_shortcode", Source: source, Path: fmt.Sprintf("markdown.byte[%d]", start+i), Message: "未核实标签保持原文，须人工确认 renderer 和参数", Blocking: true})
			}
			if base == "image" && !closing && !hasParameter(rest[nameEnd:], "fancybox:") {
				token = "{%" + inner + " fancybox:false %}"
				ms = append(ms, Mapping{fmt.Sprintf("%s.markdown.byte[%d]", source, start+i), "image.fancybox:false", "显式保留旧 image 默认禁用放大；避免官方 true 改变点击行为"})
			}
			out.WriteString(token)
			i = close + 2
		}
		start = end
	}
	return out.String(), ds, ms
}
func escaped(s string, i int) bool {
	n := 0
	for j := i - 1; j >= 0 && s[j] == '\\'; j-- {
		n++
	}
	return n%2 == 1
}
func balancedQuotes(s string) bool {
	quote := byte(0)
	for i := 0; i < len(s); i++ {
		if escaped(s, i) {
			continue
		}
		if quote != 0 {
			if s[i] == quote {
				quote = 0
			}
		} else if s[i] == '\'' || s[i] == '"' {
			quote = s[i]
		}
	}
	return quote == 0
}
func hasParameter(s, key string) bool {
	quote := byte(0)
	for i := 0; i < len(s); i++ {
		if escaped(s, i) {
			continue
		}
		if quote != 0 {
			if s[i] == quote {
				quote = 0
			}
			continue
		}
		if s[i] == '\'' || s[i] == '"' {
			quote = s[i]
			continue
		}
		if (i == 0 || s[i-1] == ' ' || s[i-1] == '\t') && strings.HasPrefix(s[i:], key) {
			return true
		}
	}
	return false
}
func protectedCode(md string) []bool {
	source := []byte(md)
	protected := make([]bool, len(source))
	document := goldmark.New().Parser().Parse(text.NewReader(source))
	mark := func(start, end int) {
		if start < 0 {
			start = 0
		}
		if end > len(protected) {
			end = len(protected)
		}
		for i := start; i < end; i++ {
			protected[i] = true
		}
	}
	_ = ast.Walk(document, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
		if !enter {
			return ast.WalkContinue, nil
		}
		switch n.(type) {
		case *ast.CodeBlock, *ast.FencedCodeBlock:
			for i := 0; i < n.Lines().Len(); i++ {
				segment := n.Lines().At(i)
				mark(segment.Start, segment.Stop)
			}
		case *ast.CodeSpan:
			for child := n.FirstChild(); child != nil; child = child.NextSibling() {
				if t, ok := child.(*ast.Text); ok {
					mark(t.Segment.Start, t.Segment.Stop)
				}
			}
		}
		return ast.WalkContinue, nil
	})
	return protected
}
func runLength(s string, i int, c byte) int {
	end := i
	for end < len(s) && s[end] == c {
		end++
	}
	return end - i
}
