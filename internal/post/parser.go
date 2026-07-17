package post

import (
	"strings"

	"github.com/goccy/go-yaml"
)

// frontMatter 文章元数据，从 Markdown 文件头部的 YAML frontmatter 解析。
type frontMatter struct {
	Title      string
	Date       string
	Excerpt    string
	Cover      string
	Tags       []string
	Categories []string
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
		case "cover":
			fm.Cover = val
		case "tags":
			fm.Tags = parseList(val, lines, &i, end)
		case "categories":
			fm.Categories = parseList(val, lines, &i, end)
		}
	}

	body := strings.Join(lines[end+1:], "\n")
	return fm, strings.TrimPrefix(body, "\n")
}

// parseList 解析 front-matter 的列表字段（tags/categories）。
// 支持两种写法：
//  1. 行内：tags: [Go, Gin] 或 tags: Go,Gin（val 非空）
//  2. 块式：tags:\n  - Go\n  - Gin（val 为空，后续行以 - 或 * 开头）
// i 指向当前行，块式时向前推进 i 跳过已消费的列表项行。end 是 front-matter 结束行号。
func parseList(val string, lines []string, i *int, end int) []string {
	val = strings.TrimSpace(val)
	if val != "" {
		// 行内列表：尝试 YAML 解析 [a,b]；失败则按逗号切分
		var arr []string
		if err := yaml.Unmarshal([]byte(val), &arr); err == nil {
			return cleanList(arr)
		}
		return cleanList(strings.Split(val, ","))
	}
	// 块式：收集后续以 - 或 * 开头的行
	var out []string
	for j := *i + 1; j < end; j++ {
		s := strings.TrimSpace(lines[j])
		if s == "" {
			continue
		}
		if !strings.HasPrefix(s, "-") && !strings.HasPrefix(s, "*") {
			break // 不是列表项，停止
		}
		item := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(s, "-"), "*"))
		item = strings.Trim(item, `"'`)
		if item != "" {
			out = append(out, item)
		}
		*i = j // 推进外层循环游标
	}
	return out
}

// cleanList 去空白和空项。
func cleanList(items []string) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		it = strings.TrimSpace(strings.Trim(it, `"'`))
		if it != "" {
			out = append(out, it)
		}
	}
	return out
}
