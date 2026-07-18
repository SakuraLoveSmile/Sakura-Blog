package template

import (
	"html"
	"html/template"
	"regexp"
	"strings"
)

// tocHeadingRe 匹配 goldmark 渲染的 h2/h3（AutoHeadingID 开启后必带 id，输出单行）。
// RE2 不支持反向引用，闭合标签用 </h[23]>（标题不会嵌套，安全）。
var tocHeadingRe = regexp.MustCompile(`<h([23]) id="([^"]+)">(.*?)</h[23]>`)

// htmlTagRe 剥离标题内的行内标签（code/em/a 等），TOC 只保留纯文本。
var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

// tocEntry 提取出的标题项。
type tocEntry struct {
	level int
	id    string
	text  string
}

// ExtractTOC 从渲染后的文章 HTML 提取 h2/h3 生成嵌套目录，对齐 Stellar hexo toc helper
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
	openH2, openH3 := false, false // 当前是否有未闭合的 h2 <li> / h3 子列表 <ol>
	for _, e := range entries {
		if e.level == 2 {
			if openH3 {
				sb.WriteString(`</li></ol>`)
				openH3 = false
			}
			if openH2 {
				sb.WriteString(`</li>`)
			}
			sb.WriteString(`<li class="toc-item toc-level-2">` + link(e))
			openH2 = true
			continue
		}
		// level 3
		if !openH2 {
			// 前文无 h2，提升为顶层项（自闭合，不进嵌套）
			sb.WriteString(`<li class="toc-item toc-level-3">` + link(e) + `</li>`)
			continue
		}
		if !openH3 {
			sb.WriteString(`<ol class="toc-child">`)
			openH3 = true
		} else {
			sb.WriteString(`</li>`)
		}
		sb.WriteString(`<li class="toc-item toc-level-3">` + link(e))
	}
	if openH3 {
		sb.WriteString(`</li></ol>`)
	}
	if openH2 {
		sb.WriteString(`</li>`)
	}
	sb.WriteString(`</ol>`)
	return template.HTML(sb.String())
}
