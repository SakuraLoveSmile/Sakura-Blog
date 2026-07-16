package post

import (
	"strings"
)

// frontMatter 文章元数据，从 Markdown 文件头部的 YAML frontmatter 解析。
type frontMatter struct {
	Title   string
	Date    string
	Excerpt string
}

// parser 轻量的 frontmatter 解析器。不引入 yaml 库，只识别 key: value 行。
type parser struct{}

func newParser() parser { return parser{} }

// Parse 分离 frontmatter 与正文。
// 支持两种格式：
//  1. --- 包裹的 frontmatter（标准）
//  2. 无 frontmatter，整篇都是正文
func (p parser) Parse(src []byte) (frontMatter, string) {
	s := string(src)
	var fm frontMatter

	lines := strings.SplitN(s, "\n", -1)
	if len(lines) == 0 {
		return fm, s
	}

	// 检测 frontmatter 起始 ---
	first := strings.TrimSpace(lines[0])
	if first != "---" {
		return fm, s
	}

	// 找结束 ---
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		// 未闭合，当作无 frontmatter
		return fm, s
	}

	// 解析 frontmatter 行
	for i := 1; i < end; i++ {
		line := lines[i]
		idx := strings.Index(line, ":")
		if idx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		val = strings.Trim(val, `"'`)
		switch key {
		case "title":
			fm.Title = val
		case "date":
			fm.Date = val
		case "excerpt", "description":
			fm.Excerpt = val
		}
	}

	body := strings.Join(lines[end+1:], "\n")
	return fm, strings.TrimPrefix(body, "\n")
}
