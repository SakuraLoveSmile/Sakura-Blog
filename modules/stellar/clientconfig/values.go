package clientconfig

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type object = map[string]any

func clone(v any) any {
	switch v := v.(type) {
	case object:
		r := object{}
		for k, x := range v {
			r[k] = clone(x)
		}
		return r
	case []any:
		r := make([]any, len(v))
		for i, x := range v {
			r[i] = clone(x)
		}
		return r
	case map[string]string:
		r := map[string]string{}
		for k, x := range v {
			r[k] = x
		}
		return r
	default:
		return v
	}
}
func obj(v any) object {
	if m, ok := v.(object); ok {
		return m
	}
	return object{}
}
func at(m object, path string) any {
	var v any = m
	for _, k := range strings.Split(path, ".") {
		v = obj(v)[k]
	}
	return v
}
func str(v any) string { s, _ := v.(string); return s }
func truth(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case json.Number:
		f, err := x.Float64()
		return err != nil || f != 0
	case float64:
		return x != 0
	case int:
		return x != 0
	default:
		return true
	}
}
func merged(base, override object) object {
	r := obj(clone(base))
	for k, v := range override {
		if b, ok := r[k].(object); ok {
			if o, ok := v.(object); ok {
				r[k] = merged(b, o)
				continue
			}
		}
		r[k] = clone(v)
	}
	return r
}

// 评论覆盖使用上游浅合并，null 不覆盖；options 则只在最终 provider 参数袋里合并。
func mergeComments(base, override object) object {
	r := obj(clone(base))
	for k, v := range override {
		if v != nil {
			r[k] = clone(v)
		}
	}
	return r
}

var scheme = regexp.MustCompile(`(?i)^(?:[a-z][a-z0-9+.-]*:|//)`)
var snake = regexp.MustCompile(`_([a-z])`)

func runtimeKey(s string) string {
	return snake.ReplaceAllStringFunc(s, func(t string) string { return strings.ToUpper(t[1:]) })
}
func normalizeResource(v any) any {
	if s, ok := v.(string); ok && !scheme.MatchString(s) {
		s = strings.TrimLeft(s, "/")
		for strings.HasPrefix(s, "./") {
			s = strings.TrimPrefix(s, "./")
			s = strings.TrimLeft(s, "/")
		}
		return "/" + s
	}
	return v
}

// /js 和 /css 是上游 source 位置，只有这两个固定前缀转换为本地 assets 位置。
func rewrite(v any) any {
	switch x := v.(type) {
	case string:
		if strings.HasPrefix(x, "/js/") || strings.HasPrefix(x, "/css/") {
			return "/assets" + x
		}
		return x
	case object:
		r := object{}
		for k, y := range x {
			r[k] = rewrite(y)
		}
		return r
	case []any:
		r := make([]any, len(x))
		for i, y := range x {
			r[i] = rewrite(y)
		}
		return r
	default:
		return clone(v)
	}
}
func split(input, defaults object, fields ...string) (object, object) {
	options := obj(clone(input))
	assets := object{}
	for _, field := range fields {
		v := options[field]
		if v == nil {
			v = defaults[field]
		}
		assets[runtimeKey(field)] = normalizeResource(v)
		delete(options, field)
	}
	return options, assets
}
func checkObjectFields(m object, path string) error {
	for _, key := range []string{"features", "search", "comments", "appearance", "services", "fallbacks"} {
		if v, ok := m[key]; ok && v != nil {
			if _, ok := v.(object); !ok {
				return fmt.Errorf("%s.%s must be an object", path, key)
			}
		}
	}
	return nil
}
func mapJSON(v any) object {
	b, _ := json.Marshal(v)
	r := object{}
	_ = json.Unmarshal(b, &r)
	return r
}

func mergeConfig(base, override object) object {
	r := obj(clone(base))
	for key, value := range override {
		if value == nil {
			continue
		}
		if child, ok := value.(object); ok {
			if previous, ok := r[key].(object); ok {
				r[key] = mergeConfig(previous, child)
				continue
			}
		}
		r[key] = clone(value)
	}
	return r
}
