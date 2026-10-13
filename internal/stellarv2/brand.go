package stellarv2

import (
	"fmt"
	"strings"
)

// NormalizeBrand projects only public title/link/image fields. Collection source,
// search, GitHub statistics and navigation metadata remain in the region model.
func NormalizeBrand(brand any) map[string]any {
	m, _ := regionObject(brand)
	result := pick(m, "name", "tagline", "href")
	if image, ok := regionObject(m["image"]); ok {
		projected := pick(image, "src", "variant")
		if len(projected) > 0 {
			result["image"] = projected
		}
	}
	return result
}

// SiteBrandModel adds the site source and regular style without changing caller
// configuration. False is the explicit brand disabling value.
func SiteBrandModel(globalConfig map[string]any) any {
	configured, _ := Get(globalConfig, "leftbar.brand")
	if configured == false {
		return false
	}
	result := assignWidget(configured)
	result["source"] = "site"
	if !jsTruthy(result["style"]) {
		result["style"] = "regular"
	}
	return result
}

// ShouldShowMobileBrand takes the upstream runtime profileKey (e.g. blogIndex),
// which is distinct from public snake_case profile configuration names.
func ShouldShowMobileBrand(profileKey string) bool {
	return includes([]string{"home", "blogIndex", "wikiIndex", "notebooks", "notebook"}, profileKey)
}

func jsString(value any) string {
	if value == nil {
		return "null"
	}
	switch v := value.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case []any:
		parts := make([]string, len(v))
		for i, item := range v {
			if item != nil {
				parts[i] = jsString(item)
			}
		}
		return strings.Join(parts, ",")
	case map[string]any:
		return "[object Object]"
	}
	return fmt.Sprint(value)
}

// JS replacement strings interpret $$, $&, $` and $' even for replaceAll with
// a literal search string. Preserve upstream behavior for author titles with $.
func replaceToken(value, token, replacement string) string {
	var out strings.Builder
	cursor := 0
	for {
		relative := strings.Index(value[cursor:], token)
		if relative < 0 {
			out.WriteString(value[cursor:])
			return out.String()
		}
		start := cursor + relative
		end := start + len(token)
		out.WriteString(value[cursor:start])
		for i := 0; i < len(replacement); i++ {
			if replacement[i] == '$' && i+1 < len(replacement) {
				switch replacement[i+1] {
				case '$':
					out.WriteByte('$')
					i++
					continue
				case '&':
					out.WriteString(token)
					i++
					continue
				case '`':
					out.WriteString(value[:start])
					i++
					continue
				case '\'':
					out.WriteString(value[end:])
					i++
					continue
				}
			}
			out.WriteByte(replacement[i])
		}
		cursor = end
	}
}

func ReplaceConfigTokens(value any, config map[string]any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	for _, key := range []string{"title", "subtitle", "avatar"} {
		replacement := ""
		if current := config[key]; jsTruthy(current) {
			replacement = jsString(current)
		}
		text = replaceToken(text, "{config."+key+"}", replacement)
	}
	return text
}

// ToRenderRegions follows layout-config.js: site-brand normalized global,
// defaults, profile, then supplied collection/page layers; source overrides win.
func ToRenderRegions(globalConfig, profile, defaults map[string]any, options RegionOptions) map[string]any {
	siteBrand := SiteBrandModel(globalConfig)
	normalized := map[string]any{}
	if globalConfig != nil {
		normalized = clone(globalConfig).(map[string]any)
	}
	leftbar, _ := regionObject(normalized["leftbar"])
	if leftbar == nil {
		leftbar = map[string]any{}
	}
	leftbar["brand"] = clone(siteBrand)
	normalized["leftbar"] = leftbar
	sources := map[string]any{"site": siteBrand}
	for key, value := range options.BrandSources {
		sources[key] = clone(value)
	}
	layers := []map[string]any{normalized, defaults, profile}
	layers = append(layers, options.Layers...)
	return CascadeRegions(layers, sources)
}

func ToRenderNavigation(profile map[string]any) map[string]any {
	if active := profile["activeMenu"]; active != nil {
		return map[string]any{"menu": clone(active)}
	}
	return map[string]any{}
}
func ProfilePath(value any) string {
	path, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.Trim(path, "/")
}
func GeneratorPath(value any) string {
	path := ProfilePath(value)
	if path == "" {
		return "index.html"
	}
	if matches(`\.[^/]+$`, path) {
		return path
	}
	return path + "/index.html"
}
