package post

import (
	"fmt"
	"regexp"
	"strings"
)

// TOCItem 文章目录单项。Level 是 heading 级别（2/3），ID 是锚点 id，Title 是标题纯文本。
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
	// idSafeRE 锚点 id 只保留字母、数字、中文、连字符，其余剔除，避免破坏 HTML 属性
	// （标题含 " & < > 等字符时 slugify 后仍可能残留，需在此兜底）。
	idSafeRE = regexp.MustCompile(`[^\p{L}\p{N}-]+`)
)

// ExtractTOC 从 goldmark 渲染后的 HTML 提取 H2/H3 目录，并给这些 heading 注入 id 锚点。
// 返回 (toc, htmlWithIDs)。无 H2/H3 时 toc 为 nil，html 原样返回。
//
// title 经 stripTags + 实体解码得到纯文本，用于 TOC 显示与 id 生成。
// id 由 slugifyHeading 生成并经 idSafeRE 兜底剔除不安全字符；重复标题追加 -2/-3 后缀去重。
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
		title := plainText(sub[2])
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
// H2 为顶层项，H3 作为其 toc-child 子项。Title 输出前做 HTML 转义，防止标题含 < & " 等破坏 HTML。
// 状态机：liOpen 追踪当前是否有未闭合 <li>，childOpen 追踪是否处于 toc-child 内。
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
			fmt.Fprintf(&sb, `<li class="toc-item"><a class="toc-link" href="#%s">%s</a>`, it.ID, htmlEscape(it.Title))
			liOpen = true
		case 3:
			if childOpen {
				// 已在 toc-child 内：闭合上一个 H3 li，开新的
				sb.WriteString(`</li>`)
			} else if liOpen {
				// 上个 H2 li 仍开着，在其内开 toc-child
				sb.WriteString(`<ol class="toc-child">`)
				childOpen = true
			}
			// 否则（无前置 H2）：H3 直接作顶层项，不开 toc-child
			fmt.Fprintf(&sb, `<li class="toc-item"><a class="toc-link" href="#%s">%s</a>`, it.ID, htmlEscape(it.Title))
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

// plainText 把标题 HTML 片段转成纯文本：剥离标签后解码 HTML 实体。
// 顺序必须是先剥离标签再解码，否则解码出的 < > 会被误当标签剥离。
func plainText(s string) string {
	s = headingTagRE.ReplaceAllString(s, "")
	return decodeEntities(s)
}

// slugifyHeading 把标题文本转成锚点 id：折叠空白为 -，转小写，剔除非安全字符，去首尾 -。
// 中文保留原字符（不转拼音）。idSafeRE 兜底剔除 " & < 等会破坏 HTML 属性的字符。
func slugifyHeading(s string) string {
	s = strings.TrimSpace(s)
	s = spaceFoldRE.ReplaceAllString(s, "-")
	s = strings.ToLower(s)
	s = idSafeRE.ReplaceAllString(s, "")
	s = strings.Trim(s, "-")
	if s == "" {
		return "heading"
	}
	return s
}

// htmlEscape 转义 HTML 文本节点（& < > " '），供 TOC 链接文本安全输出。
func htmlEscape(s string) string {
	r := strings.NewReplacer(
		`&`, "&amp;",
		`<`, "&lt;",
		`>`, "&gt;",
		`"`, "&#34;",
		`'`, "&#39;",
	)
	return r.Replace(s)
}
