package template

import (
	"html"
	"html/template"
	"regexp"
	"strings"
)

// tocHeadingRe 匹配 goldmark 渲染的 h1-h6（AutoHeadingID 开启后必带 id，输出单行）。
// RE2 不支持反向引用，闭合标签用 </h[1-6]>（标题不嵌套，安全）。
var tocHeadingRe = regexp.MustCompile(`<h([1-6]) id="([^"]+)">(.*?)</h[1-6]>`)

// htmlTagRe 剥离标题内的行内标签（code/em/a 等），TOC 只保留纯文本。
var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

// tocEntry 提取出的标题项。
type tocEntry struct {
	level int
	id    string
	text  string
}

// ExtractTOC 从渲染后的文章 HTML 提取 h1-h6 生成嵌套目录，对齐 Stellar hexo toc helper
// 的输出结构（子级 <ol class="toc-child"> 嵌在父级 <li> 内、紧跟 a.toc-link，
// CSS 的 .toc-child 缩进与 a.toc-link+ol 折叠选择器都依赖此结构）。
// 无标题时返回空串，调用方据此不渲染 TOC widget。
func ExtractTOC(contentHTML string) template.HTML {
	matches := tocHeadingRe.FindAllStringSubmatch(contentHTML, -1)
	entries := make([]tocEntry, 0, len(matches))
	for _, m := range matches {
		text := htmlTagRe.ReplaceAllString(m[3], "")
		text = strings.TrimSpace(html.UnescapeString(text))
		if text == "" {
			continue
		}
		entries = append(entries, tocEntry{level: int(m[1][0] - '0'), id: m[2], text: text})
	}
	if len(entries) == 0 {
		return ""
	}

	link := func(e tocEntry) string {
		return `<a class="toc-link" href="#` + html.EscapeString(e.id) + `"><span class="toc-text">` + html.EscapeString(e.text) + `</span></a>`
	}

	var sb strings.Builder
	sb.WriteString(`<ol class="toc">`)

	// 通用栈式嵌套：openLevels 跟踪当前打开的层级栈
	openLevels := []int{}

	for _, e := range entries {
		level := e.level

		if len(openLevels) == 0 {
			// 第一个条目
			openLevels = append(openLevels, level)
			sb.WriteString(`<li class="toc-item toc-level-` + string(rune('0'+level)) + `">` + link(e))
			continue
		}

		topLevel := openLevels[len(openLevels)-1]

		if level == topLevel {
			// 同级别：关闭上一个 li，开新 li
			sb.WriteString(`</li>`)
			sb.WriteString(`<li class="toc-item toc-level-` + string(rune('0'+level)) + `">` + link(e))
		} else if level > topLevel {
			// 更深层级：入栈，开子列表
			openLevels = append(openLevels, level)
			sb.WriteString(`<ol class="toc-child">`)
			sb.WriteString(`<li class="toc-item toc-level-` + string(rune('0'+level)) + `">` + link(e))
		} else {
			// 更浅层级：出栈直到匹配或栈空
			for len(openLevels) > 0 && openLevels[len(openLevels)-1] > level {
				sb.WriteString(`</li></ol>`)
				openLevels = openLevels[:len(openLevels)-1]
			}
			if len(openLevels) > 0 && openLevels[len(openLevels)-1] == level {
				// 回到同级
				sb.WriteString(`</li>`)
				sb.WriteString(`<li class="toc-item toc-level-` + string(rune('0'+level)) + `">` + link(e))
			} else {
				// 栈空或没有匹配级别，作为新的顶层项
				openLevels = append(openLevels, level)
				sb.WriteString(`<li class="toc-item toc-level-` + string(rune('0'+level)) + `">` + link(e))
			}
		}
	}

	// 关闭所有打开的标签
	for len(openLevels) > 0 {
		sb.WriteString(`</li>`)
		if len(openLevels) > 1 {
			sb.WriteString(`</ol>`)
		}
		openLevels = openLevels[:len(openLevels)-1]
	}

	sb.WriteString(`</ol>`)
	return template.HTML(sb.String())
}
