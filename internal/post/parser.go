package post

import (
	"fmt"
	"reflect"
	"strings"

	yaml "github.com/goccy/go-yaml"
)

// frontMatter 文章元数据，从 Markdown 文件头部的 YAML frontmatter 解析。
type frontMatter struct {
	Title      string
	Date       string
	Excerpt    string
	Cover      string
	Banner     string // 文首横幅图 URL（对齐原版 frontmatter banner 字段）
	Category   string
	Categories []string
	Topic      string // 专栏名（对齐原版 frontmatter topic 字段）
	Tags       []string
	AILabel    string // AI 成分标签（1.41）：manual/reviewed/polished/generated
	// 海报封面（对齐原版 post.poster；轻量 parser 不支持嵌套，用平铺 key）
	PosterHeadline string
	PosterTopic    string
	PosterCaption  string
	PosterColor    string
	// 参考文献（对齐原版 article_footer.ejs page.references）
	References []string
	// 备份导出/导入扩展字段（seed 源文件一般不写，缺失时保持零值）
	Status  string   // draft / published，空=由导入方决定
	Pinned  bool     // 置顶
	Authors []string // 作者 ID 列表，按 frontmatter 顺序
	Author  string   // 第一位作者，兼容旧 frontmatter
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

	// 使用项目已有 YAML 依赖解析完整 frontmatter，支持单行列表、YAML 多行列表
	// 和带引号的值。未知字段被忽略，保留当前导入器的宽松兼容行为。
	var raw map[string]any
	frontmatter := strings.Join(lines[1:end], "\n")
	if err := yaml.Unmarshal([]byte(frontmatter), &raw); err != nil {
		return fm, strings.Join(lines[end+1:], "\n")
	}
	for key, value := range raw {
		val := yamlString(value)
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
		case "category":
			fm.Category = val
		case "categories":
			fm.Categories = yamlStrings(value)
		case "topic":
			fm.Topic = val
		case "tags":
			fm.Tags = yamlStrings(value)
		case "ai_label":
			fm.AILabel = strings.TrimSpace(val)
		case "poster_headline":
			fm.PosterHeadline = val
		case "poster_topic":
			fm.PosterTopic = val
		case "poster_caption":
			fm.PosterCaption = val
		case "poster_color":
			fm.PosterColor = val
		case "references":
			fm.References = yamlStrings(value)
		case "status":
			fm.Status = val
		case "pinned":
			fm.Pinned = strings.EqualFold(val, "true") || val == "1"
		case "author":
			fm.Author = strings.TrimSpace(val)
			if fm.Author != "" && len(fm.Authors) == 0 {
				fm.Authors = []string{fm.Author}
			}
		case "authors":
			fm.Authors = yamlStrings(value)
			if len(fm.Authors) > 0 {
				fm.Author = fm.Authors[0]
			}
		}
	}
	if len(fm.Categories) == 0 && strings.TrimSpace(fm.Category) != "" {
		fm.Categories = []string{strings.TrimSpace(fm.Category)}
	}
	if len(fm.Categories) > 0 {
		fm.Category = fm.Categories[0]
	}
	if len(fm.Authors) > 0 {
		fm.Author = fm.Authors[0]
	} else if fm.Author != "" {
		fm.Authors = []string{fm.Author}
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

func yamlString(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

// yamlStrings 统一把标量、[a,b] 和 YAML block sequence 转为有序去重列表。
func yamlStrings(value any) []string {
	if value == nil {
		return nil
	}
	if s, ok := value.(string); ok {
		return parseList(s)
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return parseList(yamlString(value))
	}
	seen := map[string]bool{}
	out := make([]string, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		item := strings.Trim(yamlString(rv.Index(i).Interface()), `"'`)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}
