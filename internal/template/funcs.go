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

		// icon: 对齐原版 Stellar icon helper（scripts/events/lib/utils.js 的 hexo.utils.icon）。
		// 值为 URL（以 / 、http://、https:// 开头，如原版 share:* 彩色品牌图标）时渲染为 <img src>；
		// 否则返回内联 SVG（颜色靠父元素 color + path fill="currentColor" 继承）。template.HTML 跳过转义。
		"icon": func(key string) template.HTML {
			v := reg.Get(key)
			if strings.HasPrefix(v, "/") || strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
				// P2-20：img 补 alt（取 key 末段，如 share:wechat → wechat），屏幕阅读器可辨识。
				alt := key
				if i := strings.LastIndex(key, ":"); i >= 0 {
					alt = key[i+1:]
				}
				return template.HTML(`<img src="` + v + `" alt="` + alt + `">`)
			}
			return template.HTML(v)
		},

		// __: i18n 兼容别名，委托给 Lang。
		"__": func(key string, args ...interface{}) string {
			return Lang(key, args...)
		},

		// lang: i18n 文案查询，键名对齐原版 Stellar zh-CN.yml。
		// 模板中用法：{{lang "btn.recent_publish"}}、{{lang "nav.category" .Page.Category}}
		"lang": func(key string, args ...interface{}) string {
			return Lang(key, args...)
		},

		// partial: 占位，Go 模板用 {{template "name"}} 直接嵌入，此函数仅做兼容。
		"partial": func(name string) template.HTML {
			return template.HTML(fmt.Sprintf(`<!-- partial: %s -->`, name))
		},

		// join: 辅助函数，拼接字符串切片。
		"join": strings.Join,

		// contains/split/trim: 字符串处理，logo_sub 按 | 拆 normal/hover 两段用（对齐原版 logo.ejs 的 sub.split('|')）。
		"contains": strings.Contains,
		"split":    strings.Split,
		"trim":     strings.TrimSpace,

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
