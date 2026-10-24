package web

import (
	"fmt"
	"html/template"
	"net/url"
	"strings"
)

// 在文档内声明上游图标变量，保留自定义 loading 图的相对路径基准。
func (r stellarTemplateRenderer) iconCSS() template.CSS {
	if r.icons == nil {
		return ""
	}
	var declarations strings.Builder
	for _, icon := range [][2]string{
		{"--icon-h3-left", "default:arrow-left"},
		{"--icon-h3-right", "default:arrow-right"},
		{"--icon-quote-left", "quot:quote-left"},
		{"--icon-quote-right", "quot:quote-right"},
		{"--icon-loading", "default:loading"},
	} {
		if !r.icons.Has(icon[1]) {
			continue
		}
		raw := r.icons.Get(icon[1])
		if strings.TrimSpace(raw) == "" {
			continue
		}
		resource := raw
		if icon[1] != "default:loading" || strings.HasPrefix(strings.TrimSpace(raw), "<svg") {
			resource = "data:image/svg+xml," + strings.ReplaceAll(url.QueryEscape(raw), "+", "%20")
		}
		fmt.Fprintf(&declarations, "%s:url(%s);", icon[0], stellarIconCSSString(resource))
	}
	if declarations.Len() == 0 {
		return ""
	}
	return template.CSS(":root{" + declarations.String() + "}")
}

func stellarIconCSSString(value string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, c := range value {
		if c == '"' || c == '\\' || c == '<' || c < 32 || c == 127 {
			fmt.Fprintf(&b, "\\%x ", c)
		} else {
			b.WriteRune(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}
