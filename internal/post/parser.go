package post

import (
	"strings"
)

// frontMatter 文章元数据，从 Markdown 文件头部的 YAML frontmatter 解析。
type frontMatter struct {
	Title    string
	Date     string
	Excerpt  string
	Cover    string
	Banner   string // 文首横幅图 URL（对齐原版 frontmatter banner 字段）
	Category string
	Topic    string // 专栏名（对齐原版 frontmatter topic 字段）
	Tags     []string
	// 海报封面（对齐原版 post.poster；轻量 parser 不支持嵌套，用平铺 key）
	PosterHeadline string
	PosterTopic    string
	PosterCaption  string
	PosterColor    string
	// 参考文献（对齐原版 article_footer.ejs page.references）
	References []string
	// 备份导出/导入扩展字段（seed 源文件一般不写，缺失时保持零值）
	Status string // draft / published，空=由导入方决定
	Pinned bool   // 置顶
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
		case "banner":
			fm.Banner = val
		case "category", "categories":
			fm.Category = val
		case "topic":
			fm.Topic = val
		case "tags":
			fm.Tags = parseList(val)
		case "poster_headline":
			fm.PosterHeadline = val
		case "poster_topic":
			fm.PosterTopic = val
		case "poster_caption":
			fm.PosterCaption = val
		case "poster_color":
			fm.PosterColor = val
		case "references":
			fm.References = parseList(val)
		case "status":
			fm.Status = val
		case "pinned":
			fm.Pinned = val == "true" || val == "1"
		}
	}

	body := strings.Join(lines[end+1:], "\n")
	return fm, strings.TrimPrefix(body, "\n")
}

// parseList 解析行内列表值，支持 `[Go, Gin]` 与 `Go, Gin` 两种写法。
// 逐个 trim 空白与引号，去空去重。多行 YAML 列表（- item）不支持（轻量 parser 的既定边界）。
func parseList(val string) []string {
	val = strings.TrimSpace(val)
	val = strings.TrimPrefix(val, "[")
	val = strings.TrimSuffix(val, "]")
	seen := map[string]bool{}
	var out []string
	for _, item := range strings.Split(val, ",") {
		item = strings.TrimSpace(item)
		item = strings.Trim(item, `"'`)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}
