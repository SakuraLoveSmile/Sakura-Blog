package post

import (
	"fmt"
	"regexp"
	"strings"
)

// ShortcodeEditResult is the parser-owned result used to initialize the
// Console property editor. Values only contain fields that are present in the
// shortcode, except flags which are always returned as booleans so an omitted
// flag can override a form default.
type ShortcodeEditResult struct {
	Name     string         `json:"name"`
	Values   map[string]any `json:"values"`
	Editable bool           `json:"editable"`
}

var shortcodeEditOpening = regexp.MustCompile(`(?s)^\s*{%\s*([a-zA-Z][a-zA-Z0-9_-]*)\b(.*?)\s*%}`)

// Keep this list deliberately small. A tag not in the list is still returned
// to the caller with Editable=false, so the existing raw editor remains the
// lossless fallback for every other Stellar tag.
var shortcodeEditSupported = map[string]bool{
	"note":     true,
	"button":   true,
	"checkbox": true,
	"radio":    true,
	"mark":     true,
	"folding":  true,
	"box":      true,
	"image":    true,
	"video":    true,
	"audio":    true,
	"copy":     true,
}

var shortcodeEditBlock = map[string]bool{
	"folding": true,
	"box":     true,
}

// These are parser-supported arguments which are intentionally not exposed by
// the current catalog form. Seeing one means the raw editor is safer than
// rebuilding the shortcode and dropping it.
var shortcodeEditKnownArgs = map[string]map[string]bool{
	"button":   {"color": true, "icon": true, "size": true},
	"checkbox": {"color": true, "checked": true, "symbol": true},
	"radio":    {"color": true, "checked": true, "symbol": true},
	"folding":  {"color": true, "open": true},
	"box":      {"color": true, "child": true},
	"image":    {"width": true, "height": true, "bg": true, "download": true, "padding": true, "fancybox": true, "ratio": true},
	"video":    {"type": true, "bilibili": true, "youtube": true, "ratio": true, "width": true, "autoplay": true},
	"audio":    {"type": true, "netease": true, "autoplay": true},
	"copy":     {"git": true, "prefix": true},
}

type shortcodeEditEnvelope struct {
	name  string
	args  string
	body  string
	block bool
}

func parseShortcodeEditEnvelope(raw string, name string) (shortcodeEditEnvelope, error) {
	trimmed := strings.TrimSpace(raw)
	loc := shortcodeEditOpening.FindStringSubmatchIndex(trimmed)
	if len(loc) < 6 {
		return shortcodeEditEnvelope{}, fmt.Errorf("shortcode 格式无效")
	}

	args := strings.TrimSpace(trimmed[loc[4]:loc[5]])
	rest := trimmed[loc[1]:]
	if shortcodeEditBlock[name] {
		end := regexp.MustCompile(`(?is)^([\s\S]*?)\s*{%\s*end` + regexp.QuoteMeta(name) + `\s*%}\s*$`)
		parts := end.FindStringSubmatch(rest)
		if len(parts) != 2 {
			return shortcodeEditEnvelope{}, fmt.Errorf("%s shortcode 缺少结束标签", name)
		}
		return shortcodeEditEnvelope{name: name, args: args, body: strings.TrimSpace(parts[1]), block: true}, nil
	}
	if strings.TrimSpace(rest) != "" {
		return shortcodeEditEnvelope{}, fmt.Errorf("%s shortcode 含有多余内容", name)
	}
	return shortcodeEditEnvelope{name: name, args: args}, nil
}

func shortcodeEditParams(name string) ([]ShortcodeParamSchema, bool) {
	for _, item := range ShortcodeCatalog() {
		if item.Name == name {
			return item.Params, true
		}
	}
	return nil, false
}

func shortcodeEditParamSet(params []ShortcodeParamSchema) map[string]ShortcodeParamSchema {
	set := make(map[string]ShortcodeParamSchema, len(params))
	for _, param := range params {
		set[param.Key] = param
	}
	return set
}

func parseShortcodeEditArgs(name, args string, params []ShortcodeParamSchema) (map[string]string, []string, error) {
	named := make(map[string]string)
	var positional []string
	paramSet := shortcodeEditParamSet(params)

	for _, token := range splitArgs(args) {
		matched := false
		for _, param := range params {
			if param.Role != ShortcodeParamNamed && param.Role != ShortcodeParamFlag {
				continue
			}
			if token == param.Key && param.Role == ShortcodeParamFlag {
				named[param.Key] = "true"
				matched = true
				break
			}
			prefix := param.Key + ":"
			if !strings.HasPrefix(token, prefix) {
				continue
			}
			value := strings.TrimPrefix(token, prefix)
			if param.Role == ShortcodeParamFlag && value != "true" && value != "false" {
				return nil, nil, fmt.Errorf("%s shortcode 的 %s flag 无效", name, param.Key)
			}
			named[param.Key] = value
			matched = true
			break
		}
		if matched {
			continue
		}

		// A known parser argument absent from the current UI schema must not be
		// silently discarded by buildShortcode.
		if key, _, ok := strings.Cut(token, ":"); ok && shortcodeEditKnownArgs[name][key] && paramSet[key].Key == "" {
			return nil, nil, fmt.Errorf("%s shortcode 参数 %s 暂不支持可视化编辑", name, key)
		}
		positional = append(positional, token)
	}
	return named, positional, nil
}

func shortcodeEditInitValues(params []ShortcodeParamSchema) map[string]any {
	values := make(map[string]any)
	for _, param := range params {
		if param.Role == ShortcodeParamFlag {
			values[param.Key] = false
		}
	}
	return values
}

func shortcodeEditSetNamed(values map[string]any, params []ShortcodeParamSchema, named map[string]string) {
	for _, param := range params {
		if value, ok := named[param.Key]; ok {
			if param.Role == ShortcodeParamFlag {
				values[param.Key] = value == "true"
			} else {
				values[param.Key] = value
			}
		}
	}
}

func shortcodeEditSetPositional(values map[string]any, params []ShortcodeParamSchema, positional []string, joinSingle bool) error {
	fields := make([]string, 0, len(params))
	for _, param := range params {
		if param.Role == ShortcodeParamPositional {
			fields = append(fields, param.Key)
		}
	}
	if len(positional) > len(fields) && !joinSingle {
		return fmt.Errorf("位置参数数量超出 schema")
	}
	if joinSingle && len(fields) == 1 {
		if len(positional) > 0 {
			values[fields[0]] = strings.Join(positional, " ")
		}
		return nil
	}
	for i, key := range fields {
		if i < len(positional) {
			values[key] = positional[i]
		}
	}
	return nil
}

func shortcodeEditString(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func validateShortcodeEditValues(name string, values map[string]any) error {
	require := func(keys ...string) error {
		for _, key := range keys {
			if strings.TrimSpace(shortcodeEditString(values, key)) == "" {
				return fmt.Errorf("%s shortcode 缺少必要参数 %s", name, key)
			}
		}
		return nil
	}

	switch name {
	case "button":
		return require("text", "url")
	case "checkbox", "radio", "mark", "copy":
		return require("text")
	case "image":
		return require("src")
	case "video":
		if shortcodeEditString(values, "src") == "" && shortcodeEditString(values, "bilibili") == "" && shortcodeEditString(values, "youtube") == "" {
			return fmt.Errorf("video shortcode 缺少媒体地址")
		}
	case "audio":
		if shortcodeEditString(values, "src") == "" && shortcodeEditString(values, "netease") == "" {
			return fmt.Errorf("audio shortcode 缺少媒体地址")
		}
	}
	return nil
}

// ParseShortcodeForEdit parses only the simple catalog-backed tags used by the
// first visual editor version. Unsupported tags intentionally return a normal
// result with Editable=false so callers can keep editing their raw source.
func ParseShortcodeForEdit(raw string) (ShortcodeEditResult, error) {
	loc := shortcodeEditOpening.FindStringSubmatchIndex(strings.TrimSpace(raw))
	if len(loc) < 6 {
		return ShortcodeEditResult{}, fmt.Errorf("shortcode 格式无效")
	}
	name := strings.ToLower(strings.TrimSpace(strings.TrimSpace(raw)[loc[2]:loc[3]]))
	result := ShortcodeEditResult{Name: name, Values: map[string]any{}, Editable: false}
	if !shortcodeEditSupported[name] {
		return result, nil
	}

	envelope, err := parseShortcodeEditEnvelope(raw, name)
	if err != nil {
		return ShortcodeEditResult{}, err
	}
	params, ok := shortcodeEditParams(name)
	if !ok || len(params) == 0 {
		return result, nil
	}
	named, positional, err := parseShortcodeEditArgs(name, envelope.args, params)
	if err != nil {
		return ShortcodeEditResult{}, err
	}
	values := shortcodeEditInitValues(params)
	shortcodeEditSetNamed(values, params, named)

	switch name {
	case "note":
		color, title, content := parseNoteArgs(envelope.args)
		if color != "" {
			values["color"] = color
		}
		if title != "" {
			values["title"] = title
		}
		values["content"] = content
	case "folding", "box":
		if err := shortcodeEditSetPositional(values, params, positional, true); err != nil {
			return ShortcodeEditResult{}, err
		}
		values["content"] = envelope.body
	default:
		joinSingle := name == "checkbox" || name == "radio" || name == "mark" || name == "copy"
		if err := shortcodeEditSetPositional(values, params, positional, joinSingle); err != nil {
			return ShortcodeEditResult{}, err
		}
	}
	if err := validateShortcodeEditValues(name, values); err != nil {
		return ShortcodeEditResult{}, err
	}
	result.Values = values
	result.Editable = true
	return result, nil
}
