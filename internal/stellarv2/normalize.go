package stellarv2

import (
	"regexp"
	"strings"
)

// 正则表达式只来自固定官方规则，不接受配置输入作为表达式。

func matches(pattern, value string) bool { return regexp.MustCompile(pattern).MatchString(value) }
func replacePattern(pattern, value, replacement string) string {
	return regexp.MustCompile(pattern).ReplaceAllString(value, replacement)
}
func replacePatternFunc(pattern, value string) string {
	return regexp.MustCompile(pattern).ReplaceAllStringFunc(value, func(match string) string { return strings.ToUpper(match[1:]) })
}
func submatches(pattern, value string) []string {
	return regexp.MustCompile(pattern).FindStringSubmatch(value)
}
func trimList(value any, trim func(string) string) []any {
	result := []any{}
	seen := map[string]bool{}
	for _, item := range value.([]any) {
		text := trim(item.(string))
		if text != "" && !seen[text] {
			seen[text] = true
			result = append(result, text)
		}
	}
	return result
}
func pick(m map[string]any, names ...string) map[string]any {
	result := map[string]any{}
	for _, key := range names {
		if v, ok := m[key]; ok {
			result[key] = clone(v)
		}
	}
	return result
}
func normalize(n *node, value any) any {
	switch n.Normalizer {
	case "nullable_host":
		if value == nil {
			return nil
		}
		return strings.TrimRight(replacePattern(`(?i)^https?://`, strings.TrimSpace(value.(string)), ""), "/")
	case "host_list", "trimmed_string_list":
		return trimList(value, strings.TrimSpace)
	case "origin_list":
		return trimList(value, func(s string) string { return strings.TrimRight(strings.TrimSpace(s), "/") })
	case "nullable_trimmed_string_list":
		if value == nil {
			return nil
		}
		return trimList(value, strings.TrimSpace)
	case "share_override":
		if _, ok := value.([]any); ok {
			return trimList(value, strings.TrimSpace)
		}
		return clone(value)
	case "root_relative_path":
		if value == nil {
			return nil
		}
		normalized := strings.ReplaceAll(strings.TrimSpace(value.(string)), "\\", "/")
		if normalized == "" || normalized == "/" {
			return "/"
		}
		rooted := "/" + strings.Trim(normalized, "/")
		last := rooted[strings.LastIndex(rooted, "/")+1:]
		if strings.Contains(last, ".") {
			return rooted
		}
		return rooted + "/"
	case "collection_path":
		normalized := strings.TrimLeft(strings.ReplaceAll(strings.TrimSpace(value.(string)), "\\", "/"), "/")
		return replacePattern(`/{2,}`, normalized, "/")
	case "nullable_trimmed_string":
		if value == nil {
			return nil
		}
		return strings.TrimSpace(value.(string))
	case "menu_item":
		return pick(value.(map[string]any), "id", "title", "icon", "url", "accent")
	case "footer_action", "footer_action_item":
		m := value.(map[string]any)
		switch m["type"] {
		case "spacer":
			return map[string]any{"type": "spacer"}
		case "link":
			return pick(m, "type", "icon", "title", "url")
		case "button":
			return pick(m, "type", "icon", "title", "onclick")
		default:
			return pick(m, "type", "icon", "title", "items")
		}
	case "effect":
		if value == nil {
			return nil
		}
		m := value.(map[string]any)
		result := pick(m, "type", "options")
		if runtime, ok := m["runtime"].(map[string]any); ok {
			r := map[string]any{}
			if v, exists := runtime["pause_when_hidden"]; exists {
				r["pauseWhenHidden"] = clone(v)
			}
			if v, exists := runtime["respect_reduced_motion"]; exists {
				r["respectReducedMotion"] = clone(v)
			}
			result["runtime"] = r
		}
		return result
	case "identity", "trusted_text", "array", "leftbar_background_type", "object", "parameter_bag":
		return clone(value)
	default:
		panic("unknown Stellar normalizer: " + n.Normalizer)
	}
}
