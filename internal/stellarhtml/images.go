package stellarhtml

import (
	"bytes"
	"encoding/json"
	"fmt"
	stdhtml "html"
	"net/url"
	"regexp"
	"strings"
)

const LoadingImage = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAABGdBTUEAALGPC/xhBQAAADhlWElmTU0AKgAAAAgAAYdpAAQAAAABAAAAGgAAAAAAAqACAAQAAAABAAAAAaADAAQAAAABAAAAAQAAAADa6r/EAAAAC0lEQVQIHWNgAAIAAAUAAY27m/MAAAAASUVORK5CYII="

// ProcessSite 按懒加载、失败处理顺序执行，逐字保留脚本、样式、注释。
// 图片元数据增强和局部导航签名须在本函数之后执行。
func ProcessSite(content string, ctx Context) (string, error) {
	if !strings.Contains(strings.ToLower(content), "<img") {
		return content, nil
	}
	var handler string
	var ready bool
	return mapImages(content, func(tag string) (string, error) {
		tag = lazyImage(tag)
		closing := imageClosing.FindString(tag)
		attrs := parseAttributes(tag[4 : len(tag)-len(closing)])
		if findAttribute(attrs, "onerror") != nil {
			return tag, nil
		}
		source := findAttribute(attrs, "data-src")
		if source == nil {
			source = findAttribute(attrs, "src")
		}
		if source == nil || source.value == nil || *source.value == "" || strings.HasPrefix(strings.ToLower(*source.value), "data:image") {
			return tag, nil
		}
		if !ready {
			if ctx.IconData == nil {
				return "", fmt.Errorf("stellar HTML images: IconData callback required")
			}
			fallback := ctx.IconData("image:onerror")
			if fallback == "" {
				return "", fmt.Errorf("stellar HTML images: missing image:onerror fallback")
			}
			asset, err := ImageFallbackAsset(fallback)
			if err != nil {
				return "", err
			}
			if asset != nil {
				base, err := url.Parse(ctx.SiteURL)
				if err != nil || base.Host == "" {
					return "", fmt.Errorf("stellar HTML images: absolute SiteURL required for generated fallback")
				}
				root := base.EscapedPath()
				if !strings.HasSuffix(root, "/") {
					root += "/"
				}
				fallback = root + asset.Path
			}
			var b bytes.Buffer
			enc := json.NewEncoder(&b)
			enc.SetEscapeHTML(false)
			if err := enc.Encode(fallback); err != nil {
				return "", err
			}
			handler = escapeAttribute("this.onerror=null;this.src=" + strings.TrimSuffix(b.String(), "\n"))
			ready = true
		}
		return tag[:len(tag)-len(closing)] + ` onerror="` + handler + `"` + closing, nil
	})
}

type imageAttribute struct {
	name, raw  string
	value      *string
	start, end int
}

var imageAttributes = regexp.MustCompile(`([^\s=/>]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+)))?`)
var imageClosing = regexp.MustCompile(`\s*/?\s*>$`)
var noLazy = regexp.MustCompile(`(?i)\bno-lazy\b`)

func parseAttributes(inner string) []imageAttribute {
	var out []imageAttribute
	for _, m := range imageAttributes.FindAllStringSubmatchIndex(inner, -1) {
		a := imageAttribute{name: strings.ToLower(inner[m[2]:m[3]]), raw: inner[m[0]:m[1]], start: m[0], end: m[1]}
		for i := 4; i < len(m); i += 2 {
			if m[i] >= 0 {
				v := inner[m[i]:m[i+1]]
				a.value = &v
				break
			}
		}
		out = append(out, a)
	}
	return out
}
func findAttribute(attrs []imageAttribute, name string) *imageAttribute {
	for i := range attrs {
		if attrs[i].name == name {
			return &attrs[i]
		}
	}
	return nil
}
func lazyImage(tag string) string {
	lower := strings.ToLower(tag)
	if strings.Contains(lower, "data-src") || strings.Contains(lower, "srcset=") || noLazy.MatchString(tag) {
		return tag
	}
	closing := ">"
	if strings.HasSuffix(tag, "/>") {
		closing = "/>"
	}
	inner := tag[4 : len(tag)-len(closing)]
	attrs := parseAttributes(inner)
	src := findAttribute(attrs, "src")
	if src == nil || src.value == nil || *src.value == "" || strings.HasPrefix(strings.ToLower(*src.value), "data:image") {
		return tag
	}
	class := findAttribute(attrs, "class")
	classValue := ""
	if class != nil && class.value != nil {
		classValue = *class.value
	}
	if classValue != "" {
		classValue += " "
	}
	classValue += "lazy"
	var b strings.Builder
	b.WriteString("<img")
	if class == nil {
		b.WriteString(` class="` + escapeAttribute(stdhtml.UnescapeString(classValue)) + `"`)
	}
	pos := 0
	for _, a := range attrs {
		b.WriteString(inner[pos:a.start])
		switch a.name {
		case "src":
			b.WriteString(`src="` + LoadingImage + `" data-src="` + escapeAttribute(stdhtml.UnescapeString(*src.value)) + `"`)
		case "class":
			b.WriteString(`class="` + escapeAttribute(stdhtml.UnescapeString(classValue)) + `"`)
		default:
			b.WriteString(a.raw)
		}
		pos = a.end
	}
	b.WriteString(inner[pos:])
	b.WriteString(closing)
	return b.String()
}
func escapeAttribute(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;").Replace(s)
}
func tagEnd(s string, start int) int {
	var quote byte
	for i := start; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
		} else if c == '\'' || c == '"' {
			quote = c
		} else if c == '>' {
			return i
		}
	}
	return -1
}
func boundary(s string, pos int) bool {
	return pos >= len(s) || strings.ContainsRune(" \t\r\n>/", rune(s[pos]))
}
func mapImages(s string, transform func(string) (string, error)) (string, error) {
	lower := strings.ToLower(s)
	var b strings.Builder
	for i := 0; i < len(s); {
		next := strings.IndexByte(s[i:], '<')
		if next < 0 {
			b.WriteString(s[i:])
			break
		}
		next += i
		b.WriteString(s[i:next])
		i = next
		if strings.HasPrefix(s[i:], "<!--") {
			end := strings.Index(s[i:], "-->")
			if end < 0 {
				b.WriteString(s[i:])
				break
			}
			end += i + 3
			b.WriteString(s[i:end])
			i = end
			continue
		}
		inert := ""
		for _, name := range []string{"script", "style"} {
			if strings.HasPrefix(lower[i:], "<"+name) && boundary(s, i+1+len(name)) {
				inert = name
				break
			}
		}
		if inert != "" {
			openEnd := tagEnd(s, i)
			if openEnd < 0 {
				b.WriteString(s[i:])
				break
			}
			close := strings.Index(lower[openEnd+1:], "</"+inert)
			if close < 0 {
				b.WriteString(s[i:])
				break
			}
			end := strings.IndexByte(s[openEnd+1+close:], '>')
			if end < 0 {
				b.WriteString(s[i:])
				break
			}
			end += openEnd + 1 + close + 1
			b.WriteString(s[i:end])
			i = end
			continue
		}
		if strings.HasPrefix(lower[i:], "<img") && boundary(s, i+4) {
			end := tagEnd(s, i)
			if end < 0 {
				b.WriteString(s[i:])
				break
			}
			out, err := transform(s[i : end+1])
			if err != nil {
				return "", err
			}
			b.WriteString(out)
			i = end + 1
			continue
		}
		b.WriteByte('<')
		i++
	}
	return b.String(), nil
}
