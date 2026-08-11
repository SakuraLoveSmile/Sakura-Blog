package template

import (
	"encoding/json"
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"time"
)

// wikiTagColors 文库卡片标签颜色表：标签名 → 文字色（对齐原版 category_color helper 的按名查色机制）。
// 原版 _config.yml 的 article.category_color 是「标签名 → hex 色」，helper 把色值写进 inline style：
//
//	--text-p2 为文字色，--theme-block 为文字色追加 alpha（#fff → #fff2、#ffffff → #ffffff20）作半透明底。
//
// 颜色值沿用既有配色（红/绿/蓝/橙），未命中返回空串保持默认灰色。
var wikiTagColors = map[string]string{
	"NEW":  "#e74c3c",
	"前端":   "#e74c3c",
	"开源项目": "#27ae60",
	"iOS":  "#3498db",
	"后端":   "#3498db",
	"主题":   "#e67e22",
}

// FuncMap 对齐 Stellar EJS 用到的 helper。
// Go html/template 对返回值做 HTML 转义，SVG/icon/partial 需返回 template.HTML 跳过转义。
func FuncMap(reg *IconRegistry) template.FuncMap {
	return template.FuncMap{
		// markdown: 渲染 Markdown 片段（页脚自定义文案等），渲染器由 SetMarkdownRender 注入；
		// 未注入时转义原样输出，保证模板可独立渲染（测试/降级）。
		"markdown": func(s string) template.HTML {
			if markdownRender != nil {
				return markdownRender(s)
			}
			return template.HTML(template.HTMLEscapeString(s))
		},
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

		// tagColorStyle: 文库卡片标签颜色 inline style（对齐原版 scripts/helpers/category_color.js）。
		// 命中颜色表 → 输出 style="--text-p2:<文字色>;--theme-block:<文字色+alpha>"
		// （.cap 的 color: var(--text-p2) 消费文字色，background: var(--theme-block) 消费半透明底）；
		// 未命中返回空串，模板不输出 style 属性，保持默认灰色。返回值用 template.HTMLAttr 跳过转义。
		"tagColorStyle": func(name string) template.HTMLAttr {
			color, ok := wikiTagColors[name]
			if !ok {
				return ""
			}
			bgColor := color
			if len(color) == 4 { // #fff → #fff2
				bgColor += "2"
			} else if len(color) == 7 { // #ffffff → #ffffff20
				bgColor += "20"
			}
			return template.HTMLAttr(`style="--text-p2:` + color + `;--theme-block:` + bgColor + `"`)
		},

		// jsonify: 将值序列化为 JSON（用于在 <script> 中安全嵌入配置对象）。
		"jsonify": func(v interface{}) template.JS {
			b, err := json.Marshal(v)
			if err != nil {
				return template.JS("null")
			}
			return template.JS(string(b))
		},

		// dict: 构造键值 map（模板内传参用，如 widgets/tree 的 tree_node 递归调用）。
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("dict: odd number of arguments")
			}
			m := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict: key %v is not a string", values[i])
				}
				m[key] = values[i+1]
			}
			return m, nil
		},

		// reMatch: 正则匹配（Spec 50 导航栏 active_rules 用）。pattern 非法时返回 false。
		"reMatch": func(pattern, s string) bool {
			ok, err := regexp.MatchString(pattern, s)
			return err == nil && ok
		},
	}
}

// markdownRender 由上层注入的 Markdown 渲染器（main.go 用 post.Service 装配）。
// 包级变量与 SetTagPlugins 同权衡：启动时写一次，渲染期只读。
var markdownRender func(string) template.HTML

// SetMarkdownRender 注入 Markdown 渲染函数（页脚自定义文案等模板内 Markdown 用）。
func SetMarkdownRender(f func(string) template.HTML) {
	markdownRender = f
}
