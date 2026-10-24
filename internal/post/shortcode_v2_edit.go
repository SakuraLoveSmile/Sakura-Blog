package post

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// v2Args 在旧 tokenizer 的基础上保留空引号参数及参数边界。
// 不猜测未闭合引号；复杂原文交还源码编辑。
func v2Args(raw string) ([]string, error) {
	var tokens []string
	var b strings.Builder
	var quote byte
	started := false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			} else {
				b.WriteByte(c)
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			started = true
			continue
		}
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			if started {
				tokens = append(tokens, b.String())
				b.Reset()
				started = false
			}
			continue
		}
		started = true
		b.WriteByte(c)
	}
	if quote != 0 {
		return nil, fmt.Errorf("shortcode 参数引号未闭合")
	}
	if started {
		tokens = append(tokens, b.String())
	}
	return tokens, nil
}

func ParseShortcodeForEditStellarV2(raw string) (ShortcodeEditResult, error) {
	result := ShortcodeEditResult{Values: map[string]any{}}
	trimmed := strings.TrimSpace(raw)
	loc := shortcodeEditOpening.FindStringSubmatchIndex(trimmed)
	if len(loc) < 6 {
		return result, fmt.Errorf("shortcode 格式无效")
	}
	name := strings.ToLower(trimmed[loc[2]:loc[3]])
	result.Name = name
	entry, ok := shortcodeV2Entry(name)
	if !ok {
		return result, nil
	}
	envelope, err := parseShortcodeEditEnvelopeV2(trimmed, name, entry.Type == ShortcodeBlock)
	if err != nil {
		return result, err
	}
	// 旧版 tip/frame/navbar 同名块形式不被猜测成官方形式。
	tokens, err := v2Args(envelope.args)
	if err != nil {
		return result, err
	}
	keys := map[string]bool{}
	var positionalFields []string
	for _, p := range entry.Params {
		result.Values[p.Key] = ""
		if p.Role == ShortcodeParamNamed || p.NamedAlias {
			keys[p.Key] = true
		}
		if p.Role == ShortcodeParamPositional {
			positionalFields = append(positionalFields, p.Key)
		}
		if p.Role == ShortcodeParamContent {
			result.Values[p.Key] = envelope.body
		}
	}
	named := map[string]string{}
	var positional []string
	for _, t := range tokens {
		k, v, hasColon := strings.Cut(t, ":")
		if hasColon && keys[k] {
			if _, duplicate := named[k]; duplicate {
				return result, nil
			}
			named[k] = v
			continue
		}
		if hasColon && !strings.Contains(t, "://") && !shortcodeEditURLSchemes[k] && name != "icon" {
			return result, nil
		}
		positional = append(positional, t)
	}
	if len(positionalFields) == 0 && len(positional) > 0 {
		return result, nil
	}
	// 官方最后位置参数吸收尾部；form保存时保留其完整文本。
	if name == "emoji" && len(positional) > 0 {
		if _, ok := named["name"]; ok {
			return result, nil
		}
	}
	present := map[string]bool{}
	for k := range named {
		present[k] = true
	}
	for i, k := range positionalFields {
		if i < len(positional) {
			present[k] = true
		}
	}
	for _, p := range entry.Params {
		if p.Role == ShortcodeParamContent || present[p.Key] {
			result.Present = append(result.Present, p.Key)
		}
	}
	mapped := v2MapArgs(tokens, entry.Params)
	for k, v := range mapped {
		result.Values[k] = v
	}
	result.Editable = true
	return result, nil
}

// SerializeShortcodeEditStellarV2 不填充插入默认值，不修改用户正文；返回候选供保存与重开。
func SerializeShortcodeEditStellarV2(name string, values map[string]any, presence ...[]string) (string, error) {
	entry, ok := shortcodeV2Entry(name)
	if !ok {
		return "", fmt.Errorf("unknown Stellar v2 shortcode %s", name)
	}
	return serializeShortcodeValuesV2(entry, values, presence...)
}

func serializeShortcodeValuesV2(entry ShortcodeCatalogEntry, values map[string]any, presence ...[]string) (string, error) {
	name := entry.Name
	fields := map[string]ShortcodeParamSchema{}
	for _, p := range entry.Params {
		fields[p.Key] = p
	}
	for k := range values {
		if _, ok := fields[k]; !ok {
			return "", fmt.Errorf("unknown %s parameter %s", name, k)
		}
	}
	present := map[string]bool{}
	exact := len(presence) > 0
	if exact {
		for _, key := range presence[0] {
			if _, ok := fields[key]; !ok {
				return "", fmt.Errorf("unknown %s presence key %s", name, key)
			}
			present[key] = true
		}
	}
	var args []string
	var positional []string
	var positionalPresent []bool
	body := ""
	quote := func(v string) (string, error) {
		if strings.ContainsAny(v, "\r\n") || strings.Contains(v, "%}") {
			return "", fmt.Errorf("参数含换行或shortcode结束符，请使用源码编辑")
		}
		if v == "" {
			return `""`, nil
		}
		if !strings.ContainsAny(v, " \t\"'") {
			return v, nil
		}
		if !strings.Contains(v, `"`) {
			return `"` + v + `"`, nil
		}
		if !strings.Contains(v, "'") {
			return "'" + v + "'", nil
		}
		return "", fmt.Errorf("参数同时含单双引号，请使用源码编辑")
	}
	for _, p := range entry.Params {
		value, exists := values[p.Key]
		text := v2String(value)
		if p.Role == ShortcodeParamContent {
			body = text
			continue
		}
		emojiNamedMode := name == "emoji" && v2String(values["url"]) != "" && (!exact || !present["source"])
		if p.Role == ShortcodeParamPositional && !(emojiNamedMode && p.NamedAlias) {
			if emojiNamedMode {
				continue
			}
			positional = append(positional, text)
			positionalPresent = append(positionalPresent, !exact || present[p.Key])
			continue
		}
		if exact && !present[p.Key] {
			continue
		}
		if !exists || (text == "" && !exact) {
			continue
		}
		if b, ok := value.(bool); ok {
			text = strconv.FormatBool(b)
		}
		serialized, e := quote(text)
		if e != nil {
			return "", e
		}
		args = append(args, p.Key+":"+serialized)
	}
	// 只去掉尾部省略字段；中间空位置参数必须保留，防止后续值移到前面。
	for len(positional) > 0 && positional[len(positional)-1] == "" && (!exact || !positionalPresent[len(positional)-1]) {
		positional = positional[:len(positional)-1]
	}
	for _, p := range positional {
		serialized, e := quote(p)
		if e != nil {
			return "", e
		}
		args = append(args, serialized)
	}
	opening := "{% " + name
	if len(args) > 0 {
		opening += " " + strings.Join(args, " ")
	}
	opening += " %}"
	if entry.Type == ShortcodeBlock {
		return opening + "\n" + body + "\n{% end" + name + " %}", nil
	}
	return opening, nil
}

func parseShortcodeEditEnvelopeV2(raw, name string, block bool) (shortcodeEditEnvelope, error) {
	trimmed := strings.TrimSpace(raw)
	loc := shortcodeEditOpening.FindStringSubmatchIndex(trimmed)
	if len(loc) < 6 {
		return shortcodeEditEnvelope{}, fmt.Errorf("shortcode 格式无效")
	}
	args := strings.TrimSpace(trimmed[loc[4]:loc[5]])
	rest := trimmed[loc[1]:]
	if !block {
		if strings.TrimSpace(rest) != "" {
			return shortcodeEditEnvelope{}, fmt.Errorf("%s shortcode 含有多余内容", name)
		}
		return shortcodeEditEnvelope{name: name, args: args}, nil
	}
	end := regexp.MustCompile(`(?is){%\s*end` + regexp.QuoteMeta(name) + `\s*%}\s*$`)
	closing := end.FindStringIndex(rest)
	if closing == nil {
		return shortcodeEditEnvelope{}, fmt.Errorf("%s shortcode 缺少结束标签", name)
	}
	body := rest[:closing[0]]
	body = strings.TrimPrefix(strings.TrimPrefix(body, "\r\n"), "\n")
	body = strings.TrimSuffix(strings.TrimSuffix(body, "\r\n"), "\n")
	return shortcodeEditEnvelope{name: name, args: args, body: body, block: true}, nil
}

// Blog扩展入口独立命名；调用方还须验证站点启用开关。
func ParseShortcodeForEditBlogExtension(raw string) (ShortcodeEditResult, error) {
	trimmed := strings.TrimSpace(raw)
	loc := shortcodeEditOpening.FindStringSubmatchIndex(trimmed)
	if len(loc) < 6 {
		return ShortcodeEditResult{}, fmt.Errorf("shortcode 格式无效")
	}
	name := strings.ToLower(trimmed[loc[2]:loc[3]])
	e, ok := shortcodeBlogEntry(name)
	if !ok {
		return ShortcodeEditResult{Name: name, Values: map[string]any{}}, nil
	}
	envelope, err := parseShortcodeEditEnvelopeV2(trimmed, name, e.Type == ShortcodeBlock)
	if err != nil {
		return ShortcodeEditResult{Name: name, Values: map[string]any{}}, err
	}
	legacyName := strings.TrimPrefix(name, "blog_")
	candidate := "{% " + legacyName + " " + envelope.args + " %}"
	if e.Type == ShortcodeBlock {
		candidate += "\n" + envelope.body + "\n{% end" + legacyName + " %}"
	}
	result, err := ParseShortcodeForEdit(candidate)
	result.Name = name
	if result.Editable {
		named, pos, err2 := shortcodeEditSplitArgs(legacyName, envelope.args, e.Params)
		if err2 != nil {
			return result, err2
		}
		posIndex := 0
		for _, p := range e.Params {
			_, present := named[p.Key]
			if p.Role == ShortcodeParamPositional && posIndex < len(pos) {
				present = true
				posIndex++
			}
			if p.Role == ShortcodeParamContent {
				present = true
				result.Values[p.Key] = envelope.body
			}
			if present {
				result.Present = append(result.Present, p.Key)
			}
		}
	}
	return result, err
}
func SerializeShortcodeEditBlogExtension(name string, values map[string]any, presence ...[]string) (string, error) {
	e, ok := shortcodeBlogEntry(name)
	if !ok {
		return "", fmt.Errorf("unknown Blog extension %s", name)
	}
	return serializeShortcodeValuesV2(e, values, presence...)
}
