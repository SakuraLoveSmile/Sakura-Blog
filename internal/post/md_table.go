package post

import "strings"

// wrapTables 给普通 Markdown 表格包一层 .md-table-scroll 横向滚动容器（1.41，
// 对齐上游 scripts/filters/lib/md_table.js 的构建期 wrap 语义）。
// goldmark 输出的 `<table>` 逐个包装；排除代码高亮表格（.highlight 容器）与
// {% table %} 标签容器（.tag-plugin.table 容器）内的表格，避免破坏其自带布局。
// 容器判定用块级 div 栈近似：goldmark 输出规范（div 不交错嵌套），足够可靠。
func wrapTables(html string) string {
	if !strings.Contains(html, "<table") {
		return html
	}
	var b strings.Builder
	excluded := 0 // 处于排除容器内的深度（.highlight / .tag-plugin.table）
	wrapped := 0  // 已插入开容器、等待对应 </table> 闭合的深度
	i := 0
	for i < len(html) {
		if html[i] != '<' {
			b.WriteByte(html[i])
			i++
			continue
		}
		end := strings.IndexByte(html[i:], '>')
		if end < 0 {
			b.WriteString(html[i:])
			break
		}
		tag := html[i : i+end+1]
		lower := strings.ToLower(tag)
		switch {
		case strings.HasPrefix(lower, "</"):
			if strings.HasPrefix(lower, "</div>") {
				if excluded > 0 {
					excluded--
				}
			} else if strings.HasPrefix(lower, "</table>") && wrapped > 0 {
				b.WriteString(tag)
				b.WriteString("</div>")
				wrapped--
				i += end + 1
				continue
			}
		case strings.HasPrefix(lower, "<table"):
			if excluded == 0 {
				b.WriteString(`<div class="md-table-scroll">`)
				wrapped++
			}
		case strings.HasPrefix(lower, "<div"):
			if isExcludedTableContainer(tag) {
				excluded++
			}
		}
		b.WriteString(tag)
		i += end + 1
	}
	return b.String()
}

// isExcludedTableContainer 判断 div 是否是需要排除的表格容器：
// .highlight（代码高亮表格）或 .tag-plugin.table（{% table %} 标签）。
func isExcludedTableContainer(tag string) bool {
	// 匹配 class 属性值（双引号/单引号），再按类集合判断
	low := strings.ToLower(tag)
	var cls string
	if i := strings.Index(low, "class="); i >= 0 {
		rest := low[i+len("class="):]
		if len(rest) > 0 && (rest[0] == '"' || rest[0] == '\'') {
			q := rest[0]
			if j := strings.IndexByte(rest[1:], q); j >= 0 {
				cls = rest[1 : 1+j]
			}
		} else if j := strings.IndexAny(rest, " >"); j >= 0 {
			cls = rest[:j]
		} else {
			cls = rest
		}
	}
	has := func(name string) bool {
		for _, f := range strings.Fields(cls) {
			if f == name {
				return true
			}
		}
		return false
	}
	if has("highlight") {
		return true
	}
	return has("tag-plugin") && has("table")
}
