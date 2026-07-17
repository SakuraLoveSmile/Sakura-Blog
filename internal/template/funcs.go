package template

import (
	"fmt"
	"html/template"
	"strings"
	"time"
)

// FuncMap 对齐 Stellar EJS 用到的 helper。
// Go html/template 对返回值做 HTML 转义，SVG/icon/partial 需返回 template.HTML 跳过转义。
func FuncMap(reg *IconRegistry) template.FuncMap {
	return template.FuncMap{
		// url_for: Stellar 的 url_for，在无 root 前缀场景直接返回原路径。
		"url_for": func(path string) string {
			if path == "" {
				return "/"
			}
			return path
		},

		// date: 对齐 Hexo moment(date, format)。默认 Go time 格式。
		"date": func(t interface{}, layout string) string {
			var tt time.Time
			switch v := t.(type) {
			case time.Time:
				tt = v
			case *time.Time:
				if v == nil {
					return ""
				}
				tt = *v
			case string:
				if parsed, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
					tt = parsed
				} else if parsed, err := time.Parse("2006-01-02", v); err == nil {
					tt = parsed
				} else {
					return v
				}
			default:
				return fmt.Sprintf("%v", v)
			}
			if layout == "" {
				layout = "2006-01-02"
			}
			return tt.Format(layout)
		},

		// icon: 对齐 Stellar icon helper，返回内联 SVG（template.HTML 跳过转义）。
		// 颜色靠父元素 color + SVG path 的 fill="currentColor" 继承，不能用 <img src>。
		"icon": func(key string) template.HTML {
			return template.HTML(reg.Get(key))
		},

		// __: i18n，Demo 阶段直接返回原串。
		"__": func(key string) string {
			return key
		},

		// partial: 占位，Go 模板用 {{template "name"}} 直接嵌入，此函数仅做兼容。
		"partial": func(name string) template.HTML {
			return template.HTML(fmt.Sprintf(`<!-- partial: %s -->`, name))
		},

		// join: 辅助函数，拼接字符串切片。
		"join": strings.Join,

		// add/sub: 整数加减，分页器页码运算用（Go template 无内置算术）。
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },

		// safe: 把字符串标记为 HTML，用于注入 goldmark 渲染后的文章正文。
		"safe": func(s string) template.HTML {
			return template.HTML(s)
		},

		// htmlAttr: 把字符串标记为 HTML attribute 值。
		"htmlAttr": func(s string) template.HTMLAttr {
			return template.HTMLAttr(s)
		},
	}
}
