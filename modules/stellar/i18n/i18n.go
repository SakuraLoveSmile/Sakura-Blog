// Package i18n embeds the three official Stellar 2.0.0 language catalogs.
package i18n

import (
	"embed"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
)

//go:embed *.yml
var catalogs embed.FS

var messages = loadCatalogs()

func loadCatalogs() map[string]map[string]any {
	out := make(map[string]map[string]any)
	for _, locale := range []string{"zh-CN", "zh-TW", "en"} {
		b, err := catalogs.ReadFile(locale + ".yml")
		if err != nil {
			panic(err)
		}
		var catalog map[string]any
		if err := yaml.Unmarshal(b, &catalog); err != nil {
			panic(err)
		}
		out[locale] = catalog
	}
	return out
}

// Text uses request-local language and the official zh-CN fallback. Unknown
// keys remain literal, matching Hexo's translation helper for menu labels.
func Text(locale, key string, args ...any) string {
	locale = strings.ReplaceAll(locale, "_", "-")
	switch strings.ToLower(locale) {
	case "zh-tw", "zh-hk", "zh-hant":
		locale = "zh-TW"
	case "zh", "zh-cn", "zh-hans":
		locale = "zh-CN"
	default:
		if strings.HasPrefix(strings.ToLower(locale), "en") {
			locale = "en"
		} else {
			locale = "zh-CN"
		}
	}
	for _, language := range []string{locale, "zh-CN"} {
		var value any = messages[language]
		for _, part := range strings.Split(key, ".") {
			m, ok := value.(map[string]any)
			if !ok {
				value = nil
				break
			}
			value = m[part]
		}
		if text, ok := value.(string); ok {
			for _, arg := range args {
				text = strings.Replace(text, "%s", fmt.Sprint(arg), 1)
			}
			return text
		}
	}
	return key
}
