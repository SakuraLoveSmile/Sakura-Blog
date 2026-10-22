package stellarmigration

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"blog/internal/stellarv2"
	"blog/internal/store"
)

// UnsupportedMarkdownParameter 只描述参数位置与处理后果，不包含原始值。
type UnsupportedMarkdownParameter struct {
	Source        string `json:"source"`
	Path          string `json:"path"`
	Field         string `json:"field"`
	Code          string `json:"code"`
	Message       string `json:"message"`
	RequiresDraft bool   `json:"requires_draft"`
}

type UnsupportedMarkdownParametersError struct {
	Parameters []UnsupportedMarkdownParameter
}

func (e *UnsupportedMarkdownParametersError) Error() string {
	return "文章包含不支持的参数；请修复参数，或明确选择忽略后继续导入"
}

// LegacyMarkdownImportOptions 让预检与转换共享同一候选目录和精确忽略授权。
type LegacyMarkdownImportOptions struct {
	Topics            []*store.Topic
	IgnoredParameters []string
	// nil 表示未提供目录；非 nil 空集合表示站点确实没有可用公开作者。
	PublicAuthorIDs map[string]bool
}

// InspectLegacyMarkdownParameters 保留无站点上下文的检查入口。
func InspectLegacyMarkdownParameters(source []byte, filename string) ([]UnsupportedMarkdownParameter, error) {
	return InspectLegacyMarkdownParametersWithOptions(source, filename, LegacyMarkdownImportOptions{})
}

// InspectLegacyMarkdownParametersWithOptions 使用 strict schema 识别真正未知/已移除的字段。
// Blog 是开放扩展袋；仅宿主生命周期字段需要单独确认，不扫描其余扩展。
func InspectLegacyMarkdownParametersWithOptions(source []byte, filename string, options LegacyMarkdownImportOptions) ([]UnsupportedMarkdownParameter, error) {
	if !utf8.Valid(source) {
		return nil, fmt.Errorf("invalid Markdown UTF-8")
	}
	raw, _, err := readLegacyDocument(string(source))
	if err != nil {
		return nil, fmt.Errorf("%s: invalid front matter", filename)
	}
	return unsupportedMarkdownParameters(raw, filename, options.PublicAuthorIDs), nil
}

func unsupportedMarkdownParameters(raw map[string]any, filename string, publicAuthorIDs map[string]bool) []UnsupportedMarkdownParameter {
	out := []UnsupportedMarkdownParameter{}
	// 仅旧署名别名可明确忽略。类型错误、显式目标及父级冲突仍由转换器硬拒绝。
	if author, ok := raw["author"].(string); ok && author != "" && publicAuthorIDs != nil && !publicAuthorIDs[author] && !importTargetExists(raw, "article.author") {
		out = append(out, UnsupportedMarkdownParameter{Source: filename, Path: "/author", Field: "author", Code: "unresolved_public_author", Message: "此旧版作者未匹配站点公开作者目录；忽略后使用站点默认署名，并保存为草稿，编辑权限保持不变。", RequiresDraft: true})
	}
	for _, key := range []string{"password", "publish_at", "authors", "draft_json"} {
		paths := [][]string{{key}, {"blog", "host", key}}
		for _, parts := range paths {
			if _, exists := markdownParameterAt(raw, parts); exists {
				out = append(out, UnsupportedMarkdownParameter{Source: filename, Path: markdownParameterPointer(parts), Field: strings.Join(parts, "."), Code: "unsupported_host_lifecycle", Message: "此权限或生命周期参数不支持导入；忽略后文章将保存为草稿，且不应用该参数。", RequiresDraft: true})
			}
		}
	}
	// 已有合法迁移继续由原转换器负责，包括类型、冲突和专栏校验。
	canonical := map[string]any{}
	mapped := map[string]bool{}
	for _, key := range strings.Fields("slug topic wiki pin pinned category author ai_label type mathjax mermaid references license share status poster poster_headline poster_topic poster_caption poster_color password publish_at authors draft_json blog") {
		mapped[key] = true
	}
	for key, value := range raw {
		if mapped[key] {
			continue
		}
		if key == "banner" {
			if _, ok := value.(string); ok {
				continue
			}
		}
		canonical[key] = value
	}
	out = append(out, collectMarkdownParameterSchemaIssues(canonical, filename, nil, nil)...)
	// 旧别名不进入上游 schema，但其值仍须按目标字段严格校验。诊断只反映射
	// 已识别的原始节点；类型错误和别名/目标冲突不能借忽略授权消除。
	for _, alias := range []string{"references", "license", "share"} {
		value, exists := raw[alias]
		target := "footer." + alias
		if !exists || importTargetExists(raw, target) {
			continue
		}
		projection := map[string]any{"footer": map[string]any{alias: value}}
		out = append(out, collectMarkdownParameterSchemaIssues(projection, filename, []string{alias}, []string{"footer", alias})...)
	}
	// 旧 poster 是宿主对象别名，使用与实际消费校验相同的字段名单；开放
	// blog 扩展袋不参与扫描。已有冲突由转换器硬阻断，不提供删除别名的选择。
	if poster, ok := raw["poster"].(map[string]any); ok && !importTargetExists(raw, "blog.content.poster") {
		conflict := false
		for _, alias := range []string{"poster_headline", "poster_topic", "poster_caption", "poster_color"} {
			_, exists := raw[alias]
			conflict = conflict || exists
		}
		if !conflict {
			for _, key := range sortedKeys(poster) {
				if !isImportPosterField(key) {
					out = append(out, UnsupportedMarkdownParameter{Source: filename, Path: markdownParameterPointer([]string{"poster", key}), Field: "poster." + key, Code: "unknown_field", Message: "此参数不受支持；忽略后仅移除该参数，正文保持原样。"})
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func collectMarkdownParameterSchemaIssues(canonical map[string]any, filename string, sourcePrefix, targetPrefix []string) []UnsupportedMarkdownParameter {
	out := []UnsupportedMarkdownParameter{}
	issues := markdownUnsupportedSchemaIssues(canonical)
	var walk func(any, []string, string)
	walk = func(value any, parts []string, field string) {
		if codes := issues[field]; len(parts) > 0 && len(codes) > 0 {
			parent, _ := markdownParameterAt(canonical, parts[:len(parts)-1])
			if _, objectParameter := parent.(map[string]any); objectParameter {
				// Schema 的可读 path 不是 JSON pointer。以单一原始分支复验，避免 a.b
				// 与 {a:{b:...}} 等歧义把合法字段误标为可删除。
				isolated := markdownParameterBranch(canonical, parts).(map[string]any)
				proven := markdownUnsupportedSchemaIssues(isolated)[field]
				for _, code := range []string{"unknown_field", "removed_field"} {
					if codes[code] && proven[code] {
						sourceParts, sourceField := parts, field
						if len(targetPrefix) > 0 {
							if len(parts) < len(targetPrefix) {
								continue
							}
							matches := true
							for i, part := range targetPrefix {
								matches = matches && parts[i] == part
							}
							if !matches {
								continue
							}
							sourceParts = append(append([]string{}, sourcePrefix...), parts[len(targetPrefix):]...)
							sourceField = strings.Join(sourcePrefix, ".") + strings.TrimPrefix(field, strings.Join(targetPrefix, "."))
						}
						out = append(out, UnsupportedMarkdownParameter{Source: filename, Path: markdownParameterPointer(sourceParts), Field: sourceField, Code: code, Message: "此参数不受支持；忽略后仅移除该参数，正文保持原样。"})
						return // 未知对象整体忽略，不把内部字段再次授权。
					}
				}
			}
		}
		switch current := value.(type) {
		case map[string]any:
			for _, key := range sortedKeys(current) {
				child := key
				if field != "" {
					child = field + "." + key
				}
				walk(current[key], append(append([]string{}, parts...), key), child)
			}
		case []any:
			for i, child := range current {
				walk(child, append(append([]string{}, parts...), strconv.Itoa(i)), fmt.Sprintf("%s[%d]", field, i))
			}
		}
	}
	walk(canonical, nil, "")
	return out
}

func markdownUnsupportedSchemaIssues(raw map[string]any) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	_, err := stellarv2.ParsePage(raw, stellarv2.Options{Source: "legacy-import"})
	var schema *stellarv2.SchemaError
	if errors.As(err, &schema) {
		for _, issue := range schema.Issues {
			if issue.Code != "unknown_field" && issue.Code != "removed_field" {
				continue
			}
			if out[issue.Path] == nil {
				out[issue.Path] = map[string]bool{}
			}
			out[issue.Path][issue.Code] = true
		}
	}
	return out
}

func markdownParameterPointer(parts []string) string {
	escaped := make([]string, len(parts))
	for i, part := range parts {
		escaped[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~", "~0"), "/", "~1")
	}
	return "/" + strings.Join(escaped, "/")
}

func markdownParameterAt(raw any, parts []string) (any, bool) {
	if len(parts) == 0 {
		return raw, true
	}
	switch current := raw.(type) {
	case map[string]any:
		value, exists := current[parts[0]]
		if !exists {
			return nil, false
		}
		return markdownParameterAt(value, parts[1:])
	case []any:
		i, err := strconv.Atoi(parts[0])
		if err != nil || i < 0 || i >= len(current) {
			return nil, false
		}
		return markdownParameterAt(current[i], parts[1:])
	}
	return nil, false
}

// 构造保留数组下标的单分支，不让同名 dotted path 的其他节点影响证明。
func markdownParameterBranch(value any, parts []string) any {
	return markdownParameterBranchAt(value, parts, nil)
}

func markdownParameterBranchAt(value any, parts, prefix []string) any {
	if len(parts) == 0 {
		return value
	}
	switch current := value.(type) {
	case map[string]any:
		out := map[string]any{parts[0]: markdownParameterBranchAt(current[parts[0]], parts[1:], append(append([]string{}, prefix...), parts[0]))}
		// Page 的 footer_actions 先按 type 选择允许字段；缺少真实判别值会提前
		// 返回，无法证明未知键。只保留该动作对象已有的 type，不带入其他分支。
		if markdownFooterActionPath(prefix) && parts[0] != "type" {
			if typ, exists := current["type"]; exists {
				out["type"] = typ
			}
		}
		return out
	case []any:
		i, _ := strconv.Atoi(parts[0])
		out := make([]any, i+1)
		out[i] = markdownParameterBranchAt(current[i], parts[1:], append(append([]string{}, prefix...), parts[0]))
		return out
	}
	return nil
}

// Page schema 中仅 leftbar.footer.actions 及其 dropdown 子项按 type 提前返回。
// prefix 是真实树路径，不能用含点号的可读诊断路径判定动作上下文。
func markdownFooterActionPath(prefix []string) bool {
	if len(prefix) != 4 && len(prefix) != 6 {
		return false
	}
	if prefix[0] != "leftbar" || prefix[1] != "footer" || prefix[2] != "actions" {
		return false
	}
	if index, err := strconv.Atoi(prefix[3]); err != nil || index < 0 {
		return false
	}
	if len(prefix) == 6 {
		if prefix[4] != "items" {
			return false
		}
		if index, err := strconv.Atoi(prefix[5]); err != nil || index < 0 {
			return false
		}
	}
	return true
}

// ConvertLegacyMarkdownWithIgnoredParameters 每次重新识别并只移除明确授权的节点。
func ConvertLegacyMarkdownWithIgnoredParameters(source []byte, filename, kind string, topics []*store.Topic, ignored []string) (MarkdownImport, error) {
	return ConvertLegacyMarkdownWithOptions(source, filename, kind, LegacyMarkdownImportOptions{Topics: topics, IgnoredParameters: ignored})
}

// ConvertLegacyMarkdownWithOptions 每次依据候选目录重新证明忽略项，拒绝过时授权。
func ConvertLegacyMarkdownWithOptions(source []byte, filename, kind string, options LegacyMarkdownImportOptions) (MarkdownImport, error) {
	out := MarkdownImport{Warnings: []string{}}
	if !utf8.Valid(source) || kind != "post" && kind != "page" {
		return out, fmt.Errorf("invalid Markdown UTF-8 or kind")
	}
	raw, body, err := readLegacyDocument(string(source))
	if err != nil {
		return out, fmt.Errorf("%s: invalid front matter", filename)
	}
	problems := unsupportedMarkdownParameters(raw, filename, options.PublicAuthorIDs)
	allowed := map[string]UnsupportedMarkdownParameter{}
	for _, problem := range problems {
		allowed[problem.Path] = problem
	}
	selected := map[string]bool{}
	requiresDraft := false
	for _, path := range options.IgnoredParameters {
		problem, exists := allowed[path]
		if !exists || selected[path] {
			return out, fmt.Errorf("无效的不支持参数忽略授权")
		}
		selected[path] = true
		requiresDraft = requiresDraft || problem.RequiresDraft
	}
	remaining := []UnsupportedMarkdownParameter{}
	for _, problem := range problems {
		if !selected[problem.Path] {
			remaining = append(remaining, problem)
		}
	}
	if len(remaining) > 0 {
		return out, &UnsupportedMarkdownParametersError{Parameters: remaining}
	}
	for path := range selected {
		encodedParts := strings.Split(strings.TrimPrefix(path, "/"), "/")
		parts := make([]string, len(encodedParts))
		for i, part := range encodedParts {
			parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		}
		parent, _ := markdownParameterAt(raw, parts[:len(parts)-1])
		// 只允许删除对象参数，绝不删除列表项。
		object, ok := parent.(map[string]any)
		if !ok {
			return out, fmt.Errorf("无效的不支持参数忽略授权")
		}
		delete(object, parts[len(parts)-1])
	}
	// 已移除的生命周期值不归档；同时处理旧 status，防止冲突和隐式发布。
	if requiresDraft {
		if status, exists := raw["status"]; exists {
			if status != store.StatusDraft && status != store.StatusPublished {
				return out, fmt.Errorf("%s: status must be draft or published", filename)
			}
			if importTargetExists(raw, "blog.status") {
				return out, fmt.Errorf("%s: status conflicts with blog.status", filename)
			}
		}
		if status, exists := get(raw, "blog.status"); exists && status != store.StatusDraft && status != store.StatusPublished {
			return out, fmt.Errorf("%s: status must be draft or published", filename)
		}
		delete(raw, "status")
		if blog, exists := raw["blog"]; exists {
			if _, ok := blog.(map[string]any); !ok {
				return out, fmt.Errorf("%s: blog extension must be an object", filename)
			}
		}
		set(raw, "blog.status", store.StatusDraft)
	}
	// 后续所有快路径只读清理后的内容，不能从 source 重新引入被移除的参数。
	cleaned := []byte(";;;\n" + encode(raw) + "\n;;;\n" + body)
	return convertLegacyMarkdown(cleaned, filename, kind, options.Topics)
}
