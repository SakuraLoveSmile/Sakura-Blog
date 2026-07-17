package post

import (
	"fmt"
	"regexp"
	"strings"
)

// TOCItem 文章目录单项。Level 是 heading 级别（2/3），ID 是锚点 id，Title 是标题文本。
type TOCItem struct {
	Level int
	ID    string
	Title string
}

var (
	// headingRE 匹配 <h2>...</h2> / <h3>...</h3>，捕获级别与标题内容。
	// goldmark 输出的 heading 格式固定为 <hN>text</hN>，开闭标签数字一致；
	// RE2 不支持反向引用 \1，这里 </h[23]> 宽松匹配闭合标签（goldmark 保证一致）。
	headingRE = regexp.MustCompile(`<h([23])>(.*?)</h[23]>`)
	// headingTagRE 剥离标题内的内联标签（如 <code>）。
	headingTagRE = regexp.MustCompile(`<[^>]*>`)
	// spaceFoldRE 折叠连续空白。
	spaceFoldRE = regexp.MustCompile(`\s+`)
)

// ExtractTOC 从 goldmark 渲染后的 HTML 提取 H2/H3 目录，并给这些 heading 注入 id 锚点。
// 返回 (toc, htmlWithIDs)。无 H2/H3 时 toc 为 nil，html 原样返回。
//
// id 生成：取标题纯文本，折叠空白替换为 -，转小写；重复标题追加 -2/-3 后缀去重。
// 中文标题直接用原文做 id，浏览器原生支持 #中文 锚点（URL 里自动 percent-encode）。
func ExtractTOC(html string) ([]TOCItem, string) {
	if !headingRE.MatchString(html) {
		return nil, html
	}

	seen := map[string]int{}
	var toc []TOCItem

	// 注入 id 后的 HTML
	result := headingRE.ReplaceAllStringFunc(html, func(match string) string {
		sub := headingRE.FindStringSubmatch(match)
		level := 2
		if sub[1] == "3" {
			level = 3
		}
		title := stripTags(sub[2])
		id := slugifyHeading(title)

		// 去重：同 id 已存在则追加 -2/-3
		if n, ok := seen[id]; ok {
			n++
			seen[id] = n
			id = fmt.Sprintf("%s-%d", id, n)
		} else {
			seen[id] = 1
		}

		toc = append(toc, TOCItem{Level: level, ID: id, Title: title})
		return fmt.Sprintf(`<h%d id="%s">%s</h%d>`, level, id, sub[2], level)
	})

	if len(toc) == 0 {
		return nil, html
	}
	return toc, result
}

// BuildTOCHTML 把扁平 TOC 列表渲染成 Stellar toc widget 的嵌套 HTML：
//
//	<ol class="toc">
//	  <li class="toc-item"><a class="toc-link" href="#id">H2</a>
//	    <ol class="toc-child">
//	      <li class="toc-item"><a class="toc-link" href="#id">H3</a></li>
//	    </ol>
//	  </li>
//	</ol>
//
// H2 为顶层项，H3 作为其 toc-child 子项。状态机追踪三件事：
//   - liOpen：当前是否有一个未闭合的 <li>（H2 或 H3 的）
//   - childOpen：当前是否处于 H3 的 toc-child 内
//
// 遇到 H2：若有 childOpen 先 </li></ol> 闭合上个 H3 li + toc-child；若 liOpen 则 </li> 闭合上个 H2 li；再开新 H2 li。
// 遇到 H3：若非 childOpen 则开 toc-child（此时上个 H2 li 仍开着，作为其子列表）；若已 childOpen 则 </li> 闭合上个 H3 li；再开新 H3 li。
// 末尾按状态依次闭合。
func BuildTOCHTML(toc []TOCItem) string {
	if len(toc) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(`<ol class="toc">`)
	liOpen := false    // 当前有未闭合的 <li>
	childOpen := false // 当前处于 toc-child 内

	for _, it := range toc {
		switch it.Level {
		case 2:
			// 先收尾：若有 H3 子列表在开，闭合它（</li></ol>），此时 H2 li 仍开
			if childOpen {
				sb.WriteString(`</li></ol>`)
				childOpen = false
			}
			// 闭合上一个 H2 li（若有）
			if liOpen {
				sb.WriteString(`</li>`)
			}
			fmt.Fprintf(&sb, `<li class="toc-item"><a class="toc-link" href="#%s">%s</a>`, it.ID, it.Title)
			liOpen = true
		case 3:
			if !childOpen {
				// 进入 H3 子列表：上个 H2 li 仍开着，在其内开 toc-child
				sb.WriteString(`<ol class="toc-child">`)
				childOpen = true
			} else {
				// 闭合上一个 H3 li
				sb.WriteString(`</li>`)
			}
			fmt.Fprintf(&sb, `<li class="toc-item"><a class="toc-link" href="#%s">%s</a>`, it.ID, it.Title)
			liOpen = true
		}
	}
	// 末尾闭合：若处于 H3 子列表，</li></ol> 闭合 H3 li + toc-child；再 </li> 闭合 H2 li；最后 </ol> 闭合顶层
	if childOpen {
		sb.WriteString(`</li></ol>`)
	}
	if liOpen {
		sb.WriteString(`</li>`)
	}
	sb.WriteString(`</ol>`)
	return sb.String()
}

// stripTags 剥离 HTML 标签，保留纯文本（标题里的 <code> 等）。
func stripTags(s string) string {
	return headingTagRE.ReplaceAllString(s, "")
}

// slugifyHeading 把标题文本转成锚点 id：折叠空白为 -，转小写，去首尾 -。
// 中文保留原字符（不转拼音），仅处理空白与大小写。
func slugifyHeading(s string) string {
	s = strings.TrimSpace(s)
	s = spaceFoldRE.ReplaceAllString(s, "-")
	s = strings.ToLower(s)
	s = strings.Trim(s, "-")
	if s == "" {
		return "heading"
	}
	return s
}
