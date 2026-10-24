package appearance

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

type reader struct {
	theme map[string]any
	err   error
}

func (r *reader) fail(path, expected string) {
	if r.err == nil {
		r.err = fmt.Errorf("stellar appearance %s: expected %s", path, expected)
	}
}
func (r *reader) get(path string) any {
	var cur any = r.theme
	for _, key := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			r.fail(path, "normalized configuration field")
			return nil
		}
		v, ok := m[key]
		if !ok {
			r.fail(path, "normalized configuration field")
			return nil
		}
		cur = v
	}
	return cur
}
func (r *reader) text(path string) string {
	v := r.get(path)
	s, ok := v.(string)
	if !ok {
		r.fail(path, "string")
		return ""
	}
	if !safeValue(s) {
		r.fail(path, "safe CSS value")
		return ""
	}
	return s
}
func (r *reader) choice(path string, values ...string) string {
	s := r.text(path)
	for _, v := range values {
		if s == v {
			return s
		}
	}
	r.fail(path, "one of "+strings.Join(values, ", "))
	return ""
}
func (r *reader) number(path string, min, max float64) float64 {
	var n float64
	switch v := r.get(path).(type) {
	case float64:
		n = v
	case int:
		n = float64(v)
	case int64:
		n = float64(v)
	case json.Number:
		var e error
		n, e = v.Float64()
		if e != nil {
			r.fail(path, "finite number")
		}
	default:
		r.fail(path, "finite number")
	}
	if math.IsNaN(n) || math.IsInf(n, 0) || n < min || n > max {
		r.fail(path, "number within range")
	}
	return n
}
func (r *reader) boolean(path string) bool {
	v, ok := r.get(path).(bool)
	if !ok {
		r.fail(path, "boolean")
	}
	return v
}

var lengthPattern = regexp.MustCompile(`^(?:0|(?:\d+|\d*\.\d+)(?:px|rem|em|%|vw|vh|vmin|vmax|ch|ex|cm|mm|in|pt|pc))$`)
var percentagePattern = regexp.MustCompile(`^(?:\d+|\d*\.\d+)%$`)

func (r *reader) length(path string) string {
	s := r.text(path)
	if !lengthPattern.MatchString(s) {
		r.fail(path, "CSS length")
	}
	return s
}
func (r *reader) resource(path string) string {
	v := r.get(path)
	if v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok || s == "" || strings.TrimSpace(s) != s || strings.ContainsAny(s, "'\"\\<>\n\r") {
		r.fail(path, "safe image/stylesheet resource")
		return ""
	}
	for _, c := range s {
		if unicode.IsControl(c) {
			r.fail(path, "safe image/stylesheet resource")
			return ""
		}
	}
	if strings.HasPrefix(strings.ToLower(s), "data:image/") {
		return s
	}
	u, e := url.Parse(s)
	if e != nil || u.Scheme != "" && (strings.ToLower(u.Scheme) != "https" && strings.ToLower(u.Scheme) != "http" || u.Host == "") {
		r.fail(path, "safe image/stylesheet resource")
		return ""
	}
	for _, part := range strings.Split(u.Path, "/") {
		if part == ".." {
			r.fail(path, "safe image/stylesheet resource")
			return ""
		}
	}
	return s
}
func (r *reader) colors(path string) []string {
	var values []any
	switch a := r.get(path).(type) {
	case []any:
		values = a
	case []string:
		for _, s := range a {
			values = append(values, s)
		}
	default:
		r.fail(path, "four CSS colors")
	}
	if len(values) != 4 {
		r.fail(path, "four CSS colors")
		return nil
	}
	result := make([]string, 4)
	for i, v := range values {
		s, ok := v.(string)
		if !ok || !safeValue(s) {
			r.fail(path+"."+strconv.Itoa(i), "safe CSS color")
		}
		result[i] = s
	}
	return result
}

// 输入已通过 schema；转为 template.CSS 之前仍拒绝逃逸、注释绕过及未闭合 token。
func safeValue(s string) bool {
	if s == "" || strings.TrimSpace(s) != s || strings.ContainsAny(s, ";{}<>@") || strings.Contains(s, "/*") || strings.Contains(s, "*/") {
		return false
	}
	lower := strings.ToLower(s)
	if strings.Contains(lower, "url(") || regexp.MustCompile(`(?i)(?:url|expression)\s*\(`).MatchString(s) {
		return false
	}
	depth := 0
	var quote rune
	escaped := false
	for _, c := range s {
		if unicode.IsControl(c) {
			return false
		}
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\\' {
			return false
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		if c == '(' {
			depth++
		}
		if c == ')' {
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return quote == 0 && depth == 0 && !escaped
}
func cssURL(s string) string { return "url(" + strconv.Quote(s) + ")" }
