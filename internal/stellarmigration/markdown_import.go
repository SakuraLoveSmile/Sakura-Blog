package stellarmigration

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"blog/internal/post"
	"blog/internal/store"
)

// MarkdownImport 是只读转换结果；SlugSuggestion 不决定编辑中记录的身份。
type MarkdownImport struct {
	Document        post.StellarDocument
	SlugSuggestion  string
	LegacyConverted bool
	Warnings        []string
}

// MissingLegacyTopicError 供导入入口请求明确创建授权；只读转换本身不新建专栏。
type MissingLegacyTopicError struct {
	Source string
	Topic  string
}

func (e *MissingLegacyTopicError) Error() string {
	return fmt.Sprintf("%s: 未找到专栏 %q；请先在「专栏」中创建，或将 topic 改为已有专栏的名称或 Slug", e.Source, e.Topic)
}

// ConvertLegacyMarkdown 显式复用整站迁移的字段与 shortcode 规则，不作为 strict 解析的 fallback。
// 空 topic 表示未分配；非空值只允许唯一 slug/name 匹配，不创建集合或修改任何记录。
func ConvertLegacyMarkdown(source []byte, filename, kind string, topics []*store.Topic) (MarkdownImport, error) {
	out := MarkdownImport{Warnings: []string{}}
	if !utf8.Valid(source) || kind != "post" && kind != "page" {
		return out, fmt.Errorf("invalid Markdown UTF-8 or kind")
	}
	raw, body, err := readLegacyDocument(string(source))
	if err != nil {
		return out, fmt.Errorf("%s: invalid front matter", filename)
	}
	for _, key := range []string{"password", "publish_at", "authors", "draft_json"} {
		_, direct := raw[key]
		_, host := get(raw, "blog.host."+key)
		if direct || host {
			return out, fmt.Errorf("%s: %s requires a supported host lifecycle contract; import rejected", filename, key)
		}
	}
	if v, exists := raw["slug"]; exists {
		text, ok := v.(string)
		if !ok || !validImportSlug(text) {
			return out, fmt.Errorf("%s: slug must be a valid single path component", filename)
		}
		out.SlugSuggestion = text
		delete(raw, "slug")
		out.LegacyConverted = true
	}
	// 旧模板常保留空 topic 占位；它没有指定归属，也不应与显式 collection 冲突。
	if value, exists := raw["topic"]; exists {
		name, text := value.(string)
		if value == nil || text && strings.TrimSpace(name) == "" {
			delete(raw, "topic")
			out.LegacyConverted = true
			out.Warnings = append(out.Warnings, "旧版 topic 为空，按未分配专栏处理。")
		}
	}
	// 显式选择 1.x 时，通用配置仍可复用 strict 解析，但正文统一应用旧版映射。
	// 例如仅含 image 的旧文档也必须保留旧 fancybox 默认值；2.0 文档应走 strict 入口。
	if !out.LegacyConverted {
		if document, err := post.ParseStellarDocument(source, post.StellarDocumentOptions{Source: filename}); err == nil {
			markdown, diagnostics, mappings := migrateMarkdown(document.Body, filename, nil, nil)
			for _, diagnostic := range diagnostics {
				if diagnostic.Blocking {
					return out, fmt.Errorf("%s: %s: %s", filename, diagnostic.Path, diagnostic.Message)
				}
			}
			if len(mappings) > 0 {
				document.Body, out.LegacyConverted = markdown, true
				out.Warnings = []string{"已转换旧版 shortcode；请确认预览后保存。"}
			}
			if err := validateImportStatus(document.Blog, filename); err != nil {
				return out, err
			}
			out.Document = document
			return out, nil
		}
	}
	for old, target := range map[string]string{"topic": "collection", "wiki": "collection", "pin": "listing.priority", "pinned": "listing.priority", "author": "article.author", "ai_label": "article.ai_label", "type": "article.style", "mathjax": "render.math", "mermaid": "render.diagrams", "references": "footer.references", "license": "footer.license", "share": "footer.share", "status": "blog.status", "poster": "blog.content.poster", "poster_headline": "blog.content.poster.headline", "poster_topic": "blog.content.poster.topic", "poster_caption": "blog.content.poster.caption", "poster_color": "blog.content.poster.color"} {
		if _, exists := raw[old]; exists {
			if importTargetExists(raw, target) {
				return out, fmt.Errorf("%s: %s conflicts with %s", filename, old, target)
			}
		}
	}
	for _, pair := range [][2]string{{"pin", "pinned"}, {"topic", "wiki"}, {"category", "categories"}} {
		if _, a := raw[pair[0]]; a {
			if _, b := raw[pair[1]]; b {
				return out, fmt.Errorf("%s: %s conflicts with %s", filename, pair[0], pair[1])
			}
		}
	}
	if _, poster := raw["poster"]; poster {
		for _, key := range []string{"poster_headline", "poster_topic", "poster_caption", "poster_color"} {
			if _, exists := raw[key]; exists {
				return out, fmt.Errorf("%s: poster conflicts with %s", filename, key)
			}
		}
	}
	if v, exists := raw["topic"]; exists {
		name, ok := v.(string)
		if !ok {
			return out, fmt.Errorf("%s: topic 必须是专栏名称或 Slug 字符串", filename)
		}
		matches := 0
		for _, topic := range topics {
			if topic.Name == name || topic.Slug == name {
				matches++
			}
		}
		if matches == 0 {
			return out, &MissingLegacyTopicError{Source: filename, Topic: name}
		}
		if matches > 1 {
			return out, fmt.Errorf("%s: topic %q 匹配到多个专栏；请改为可唯一匹配的专栏名称或 Slug", filename, name)
		}
	}
	// 仅显式导入接受旧渲染开关；不从字符串、数字或对象猜测启用状态。
	for _, field := range []struct{ old, target string }{{"mathjax", "render.math"}, {"mermaid", "render.diagrams"}} {
		if value, exists := raw[field.old]; exists {
			enabled, ok := value.(bool)
			if !ok {
				return out, fmt.Errorf("%s: %s must be a boolean", filename, field.old)
			}
			var converted any = false
			if enabled {
				converted = field.old
			}
			set(raw, field.target, converted)
			delete(raw, field.old)
		}
	}
	if value, exists := raw["type"]; exists {
		style, ok := value.(string)
		if !ok || style != "tech" && style != "story" {
			return out, fmt.Errorf("%s: type must be tech or story", filename)
		}
		set(raw, "article.style", style)
		delete(raw, "type")
	}
	if v, exists := raw["banner"]; exists {
		if image, ok := v.(string); ok {
			if importTargetExists(raw, "blog.content.banner_image") {
				return out, fmt.Errorf("%s: banner conflicts with blog.content.banner_image", filename)
			}
			set(raw, "blog.content.banner_image", image)
			delete(raw, "banner")
		}
	}
	// buildFile 的通用迁移仍保留宿主消费门禁；本入口已具备状态、海报的真实消费。
	snapshot := Snapshot{Tables: map[string]Table{"topics": {Rows: []Row{}}}}
	for _, topic := range topics {
		table := snapshot.Tables["topics"]
		table.Rows = append(table.Rows, Row{"name": topic.Name, "slug": topic.Slug})
		snapshot.Tables["topics"] = table
	}
	path := "content/pages/import.md"
	if kind == "post" {
		path = "content/posts/import.md"
	}
	candidate := buildFile(";;;\n"+encode(raw)+"\n;;;\n"+body, path, snapshot)
	for _, diagnostic := range candidate.Diagnostics {
		if diagnostic.Code == "poster_consumer" || diagnostic.Code == "file_host_fields" && diagnostic.Path == "status" {
			continue
		}
		if diagnostic.Blocking {
			return out, fmt.Errorf("%s: %s: %s", filename, diagnostic.Path, diagnostic.Message)
		}
		out.Warnings = append(out.Warnings, diagnostic.Message)
	}
	if candidate.Document == "" {
		return out, fmt.Errorf("%s: cannot safely convert Markdown", filename)
	}
	document, err := post.ParseStellarDocument([]byte(candidate.Document), post.StellarDocumentOptions{Source: filename})
	if err != nil {
		return out, err
	}
	if err := validateImportStatus(document.Blog, filename); err != nil {
		return out, err
	}
	if err := validateImportPoster(document.Blog, filename); err != nil {
		return out, err
	}
	out.Document, out.LegacyConverted = document, true
	out.Warnings = append(out.Warnings, "已转换旧版字段与已知 shortcode；请确认预览后保存。")
	return out, nil
}

func validImportSlug(value string) bool {
	if value == "" || strings.TrimSpace(value) != value || value == "." || value == ".." || strings.ContainsAny(value, `/<>:"\|?*`) {
		return false
	}
	for _, r := range value {
		if r < 32 {
			return false
		}
	}
	return true
}

// 目标父级为标量也属于冲突，不能由 set 静默覆盖。
func importTargetExists(raw map[string]any, path string) bool {
	parts := strings.Split(path, ".")
	current := raw
	for i, part := range parts {
		value, exists := current[part]
		if !exists {
			return false
		}
		if i == len(parts)-1 {
			return true
		}
		next, ok := value.(map[string]any)
		if !ok {
			return true
		}
		current = next
	}
	return false
}

func validateImportStatus(blog map[string]any, source string) error {
	if status, exists := blog["status"]; exists && status != store.StatusDraft && status != store.StatusPublished {
		return fmt.Errorf("%s: status must be draft or published", source)
	}
	return nil
}

func validateImportPoster(blog map[string]any, source string) error {
	v, exists := get(blog, "content.poster")
	if !exists {
		return nil
	}
	poster, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("%s: poster must be an object", source)
	}
	for key, value := range poster {
		if key != "headline" && key != "topic" && key != "caption" && key != "color" {
			return fmt.Errorf("%s: unknown poster field %s", source, key)
		}
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s: poster.%s must be a string", source, key)
		}
	}
	return nil
}
