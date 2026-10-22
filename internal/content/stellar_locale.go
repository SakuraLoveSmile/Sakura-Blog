package content

import "strings"

// ResolveStellarLanguage 沿宿主 Hexo 语言优先级，不读取主题中不存在的 language。
func ResolveStellarLanguage(page, blog map[string]any) string {
	first := func(value any) string {
		switch value := value.(type) {
		case string:
			return strings.TrimSpace(value)
		case []any:
			for _, item := range value {
				if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
					return strings.TrimSpace(text)
				}
			}
		case []string:
			for _, text := range value {
				if strings.TrimSpace(text) != "" {
					return strings.TrimSpace(text)
				}
			}
		}
		return ""
	}
	for _, value := range []any{page["lang"], page["language"]} {
		if language := first(value); language != "" {
			return language
		}
	}
	site, _ := blog["site"].(map[string]any)
	if language := first(site["language"]); language != "" {
		return language
	}
	return "zh-CN"
}
