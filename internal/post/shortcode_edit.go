package post

import (
	"fmt"
	"regexp"
	"strings"
)

// ShortcodeEditResult is the parser-owned result used to initialize the
// Console property editor. Values always contain every catalog parameter so the
// form can tell "absent" from "empty": editing an existing shortcode must never
// inject a default the author never wrote.
type ShortcodeEditResult struct {
	Name     string         `json:"name"`
	Values   map[string]any `json:"values"`
	Editable bool           `json:"editable"`
	// Variant 只对 hybrid 标签（navbar/frame）输出，标明当前 raw 是 inline 还是 block，
	// 前端据此选择包装方式，避免把旧的单行语法强行改写为块级。
	Variant string `json:"variant,omitempty"`
}

var shortcodeEditOpening = regexp.MustCompile(`(?s)^\s*{%\s*([a-zA-Z][a-zA-Z0-9_-]*)\b(.*?)\s*%}`)

// shortcodeEditURLSchemes 位置参数里可能带冒号（如链接），这些前缀不算命名参数。
var shortcodeEditURLSchemes = map[string]bool{
	"http": true, "https": true, "ftp": true, "ftps": true, "mailto": true, "tel": true,
}

// shortcodeEditColonPositional 这些位置参数的值本身含冒号（如 Iconify 图标 key），
// 不能按 key:value 判定为命名参数；只在尚未填入位置参数时适用。
var shortcodeEditColonPositional = map[string]bool{
	"icon": true,
}

// shortcodeEditPositionalFallback 旧语法把值写在位置参数里，而 schema 用 named 字段承载。
// 解析时把剩余位置参数回填到首個空字段，重建时统一输出 canonical named 形式。
var shortcodeEditPositionalFallback = map[string][]string{
	"banner": {"title", "subtitle"},
	"quot":   {"text"},
	"tip":    {"text"},
	"frame":  {"device"},
}

// shortcodeFrameInlineParams 是 frame 单行旧语法的参数布局，与块级（device/width/content）不同。
// 只在解析/生成单行 frame 时使用，不引入通用 variant schema 体系。
var shortcodeFrameInlineParams = []ShortcodeParamSchema{
	positional("device", "设备", "iphone"),
	named("img", "图片 URL", nil),
	named("video", "视频 URL", nil),
	named("focus", "焦点", nil),
	positional("alt", "替代文本", nil),
}

type shortcodeEditEnvelope struct {
	name  string
	args  string
	body  string
	block bool
}

// shortcodeEditHasClosing 判断 raw 是否带 {% endxxx %}（hybrid 标签据此区分 inline/block）。
func shortcodeEditHasClosing(raw, name string) bool {
	re, err := regexp.Compile(`(?is){%\s*end` + regexp.QuoteMeta(name) + `\s*%}`)
	if err != nil {
		return false
	}
	return re.MatchString(raw)
}

func parseShortcodeEditEnvelope(raw, name string, block bool) (shortcodeEditEnvelope, error) {
	trimmed := strings.TrimSpace(raw)
	loc := shortcodeEditOpening.FindStringSubmatchIndex(trimmed)
	if len(loc) < 6 {
		return shortcodeEditEnvelope{}, fmt.Errorf("shortcode 格式无效")
	}

	args := strings.TrimSpace(trimmed[loc[4]:loc[5]])
	rest := trimmed[loc[1]:]
	if block {
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

// shortcodeEditSplitArgs 按 catalog schema 把开标签参数切成 named / flag / positional。
// 形如 key:value 但 key 不在 schema 中的 token 一律报错，交回源码编辑，
// 避免重建 shortcode 时把未知参数静默当成位置参数吞掉。
func shortcodeEditSplitArgs(name, args string, params []ShortcodeParamSchema) (map[string]string, []string, error) {
	named := make(map[string]string)
	var positional []string

	for _, token := range splitArgs(args) {
		key, value, hasColon := strings.Cut(token, ":")
		matched := false

		if hasColon {
			for _, param := range params {
				switch param.Role {
				case ShortcodeParamNamed:
					if param.Key == key {
						named[param.Key] = value
						matched = true
					}
				case ShortcodeParamFlag:
					if param.Key == key {
						if value != "true" && value != "false" {
							return nil, nil, fmt.Errorf("%s shortcode 的 %s flag 无效", name, param.Key)
						}
						named[param.Key] = value
						matched = true
					}
				}
				if matched {
					break
				}
			}
		} else {
			// 裸 flag：{% checkbox checked 文本 %}
			for _, param := range params {
				if param.Role == ShortcodeParamFlag && param.Key == token {
					named[param.Key] = "true"
					matched = true
					break
				}
			}
		}
		if matched {
			continue
		}
		if hasColon && !shortcodeEditURLSchemes[strings.ToLower(key)] {
			if shortcodeEditColonPositional[name] && len(positional) == 0 {
				positional = append(positional, token)
				continue
			}
			return nil, nil, fmt.Errorf("%s shortcode 参数 %s 暂不支持可视化编辑", name, key)
		}
		positional = append(positional, token)
	}
	return named, positional, nil
}

// shortcodeEditInitValues 初始化全部 schema 字段：flag=false，其余为空字符串。
func shortcodeEditInitValues(params []ShortcodeParamSchema) map[string]any {
	values := make(map[string]any, len(params))
	for _, param := range params {
		if param.Role == ShortcodeParamFlag {
			values[param.Key] = false
		} else {
			values[param.Key] = ""
		}
	}
	return values
}

func shortcodeEditApplyNamed(values map[string]any, params []ShortcodeParamSchema, named map[string]string) {
	for _, param := range params {
		value, ok := named[param.Key]
		if !ok {
			continue
		}
		if param.Role == ShortcodeParamFlag {
			values[param.Key] = value == "true"
		} else {
			values[param.Key] = value
		}
	}
}

// shortcodeEditApplyPositional 把位置参数写入 schema 的 positional 字段。
// 最后一个 positional 字段吸收剩余 token（自由文本不被截断）；
// 没有 positional 字段时原样返回剩余 token，由调用方决定归属或报错。
func shortcodeEditApplyPositional(values map[string]any, params []ShortcodeParamSchema, positional []string) []string {
	var fields []string
	for _, param := range params {
		if param.Role == ShortcodeParamPositional {
			fields = append(fields, param.Key)
		}
	}
	if len(fields) == 0 || len(positional) == 0 {
		return positional
	}

	tail := len(fields) - 1
	if len(positional) <= tail {
		for i, key := range fields {
			if i < len(positional) {
				values[key] = positional[i]
			}
		}
		return nil
	}
	for i := 0; i < tail; i++ {
		values[fields[i]] = positional[i]
	}
	values[fields[tail]] = strings.Join(positional[tail:], " ")
	return nil
}

// shortcodeEditApplyFallback 把剩余位置参数回填到 schema 的 named 字段（旧语法兼容）。
// 最后一个字段吸收剩余 token；无 fallback 定义且仍有剩余参数时返回 false。
func shortcodeEditApplyFallback(values map[string]any, name string, leftover []string) bool {
	if len(leftover) == 0 {
		return true
	}
	keys, ok := shortcodeEditPositionalFallback[name]
	if !ok {
		return false
	}
	for i, key := range keys {
		if shortcodeEditString(values, key) != "" || i >= len(leftover) {
			continue
		}
		if i == len(keys)-1 {
			values[key] = strings.Join(leftover[i:], " ")
		} else {
			values[key] = leftover[i]
		}
	}
	return true
}

// parseEmojiArgs 解析 emoji 的两种模式：
// 普通：{% emoji [source] name [height:x] %}；URL：{% emoji url:https://... [name:alt] [height:x] %}。
func parseEmojiArgs(args string) (source, name, url, height string) {
	var positional []string
	for _, tok := range splitArgs(args) {
		switch {
		case strings.HasPrefix(tok, "url:"):
			url = strings.TrimPrefix(tok, "url:")
		case strings.HasPrefix(tok, "height:"):
			height = strings.TrimPrefix(tok, "height:")
		case strings.HasPrefix(tok, "name:"):
			name = strings.TrimPrefix(tok, "name:")
		default:
			positional = append(positional, tok)
		}
	}
	if url != "" {
		if name == "" {
			name = strings.Join(positional, " ")
		}
		return "", name, url, height
	}
	switch len(positional) {
	case 0:
	case 1:
		if name == "" {
			name = positional[0]
		}
	default:
		source = positional[0]
		if name == "" {
			name = strings.Join(positional[1:], " ")
		}
	}
	return source, name, url, height
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

// ParseShortcodeForEdit 解析 catalog 内的任意标签，供 Console 属性面板初始化。
// 不在 catalog 中或语法无法无损映射的标签返回 Editable=false / 错误，
// 由调用方回退到源码编辑；绝不改动持久化内容。
func ParseShortcodeForEdit(raw string) (ShortcodeEditResult, error) {
	trimmed := strings.TrimSpace(raw)
	loc := shortcodeEditOpening.FindStringSubmatchIndex(trimmed)
	if len(loc) < 6 {
		return ShortcodeEditResult{}, fmt.Errorf("shortcode 格式无效")
	}
	name := strings.ToLower(strings.TrimSpace(trimmed[loc[2]:loc[3]]))
	result := ShortcodeEditResult{Name: name, Values: map[string]any{}, Editable: false}

	entry, ok := findShortcodeCatalogEntry(name)
	if !ok {
		return result, nil
	}

	block := entry.Type == ShortcodeBlock
	variant := ShortcodeVariantInline
	if block {
		variant = ShortcodeVariantBlock
	} else if entry.Type == ShortcodeHybrid && shortcodeEditHasClosing(trimmed, name) {
		block = true
		variant = ShortcodeVariantBlock
	}

	envelope, err := parseShortcodeEditEnvelope(trimmed, name, block)
	if err != nil {
		return ShortcodeEditResult{}, err
	}

	params := entry.Params
	if name == "frame" && !block {
		params = shortcodeFrameInlineParams
	}
	values := shortcodeEditInitValues(params)

	switch name {
	case "note":
		color, title, body := parseNoteArgs(envelope.args)
		values["color"] = color
		values["title"] = title
		values["content"] = body
	case "emoji":
		source, emojiName, url, height := parseEmojiArgs(envelope.args)
		values["source"] = source
		values["name"] = emojiName
		values["url"] = url
		values["height"] = height
	default:
		named, positional, err := shortcodeEditSplitArgs(name, envelope.args, params)
		if err != nil {
			return ShortcodeEditResult{}, err
		}
		shortcodeEditApplyNamed(values, params, named)
		leftover := shortcodeEditApplyPositional(values, params, positional)
		if block {
			for _, param := range params {
				if param.Role == ShortcodeParamContent {
					values[param.Key] = envelope.body
				}
			}
		}
		switch {
		case name == "navbar" && !block:
			// 单行 navbar 的链接列表是位置参数，按行展示在 content 里
			values["content"] = strings.Join(leftover, "\n")
		default:
			if !shortcodeEditApplyFallback(values, name, leftover) {
				return ShortcodeEditResult{}, fmt.Errorf("%s shortcode 参数无法映射到属性表单", name)
			}
		}
	}

	if err := validateShortcodeEditValues(name, values); err != nil {
		return ShortcodeEditResult{}, err
	}

	result.Values = values
	result.Editable = true
	if entry.Type == ShortcodeHybrid {
		result.Variant = string(variant)
	}
	return result, nil
}
