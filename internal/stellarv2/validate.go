package stellarv2

import (
	"fmt"
	"math"
	"net"
	"net/url"
	"path"
	"strconv"
	"strings"

	"golang.org/x/net/idna"
)

func text(value any) string   { s, _ := value.(string); return s }
func nonempty(value any) bool { s, ok := value.(string); return ok && len(strings.TrimSpace(s)) > 0 }
func safeNavigation(value any) bool {
	s, ok := value.(string)
	if !ok || s == "" || strings.TrimSpace(s) != s {
		return false
	}
	if strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//") {
		return true
	}
	if strings.HasPrefix(s, "#") && len(s) > 1 {
		return true
	}
	if httpURL(s) {
		return true
	}
	// 非 special scheme 的空 opaque path 在 WHATWG URL 中仍有效。
	clean := cleanURL(s)
	colon := strings.IndexByte(clean, ':')
	if colon < 0 {
		return false
	}
	if !includes([]string{"mailto", "tel"}, strings.ToLower(clean[:colon])) {
		return false
	}
	return genericNavigationURL(clean[colon+1:])
}
func genericNavigationURL(rest string) bool {
	if !strings.HasPrefix(rest, "//") {
		return true
	}
	authority := rest[2:]
	if end := strings.IndexAny(authority, "/?#"); end >= 0 {
		authority = authority[:end]
	}
	hasCredentials := strings.Contains(authority, "@")
	if at := strings.LastIndexByte(authority, '@'); at >= 0 {
		authority = authority[at+1:]
	}
	host, port := authority, ""
	hasPort := false
	if strings.HasPrefix(host, "[") {
		end := strings.IndexByte(host, ']')
		if end < 0 || net.ParseIP(host[1:end]) == nil || !strings.Contains(host[1:end], ":") {
			return false
		}
		suffix := host[end+1:]
		host = host[:end+1]
		if suffix != "" {
			if suffix[0] != ':' {
				return false
			}
			port = suffix[1:]
			hasPort = true
		}
	} else {
		if colon := strings.LastIndexByte(host, ':'); colon >= 0 {
			host, port = host[:colon], host[colon+1:]
			hasPort = true
		}
		if strings.ContainsAny(host, " #/:<>?@[\\]^|") {
			return false
		}
		for _, r := range host {
			if r <= 0x20 || r == 0x7f {
				return false
			}
		}
	}
	if host == "" && (hasCredentials || hasPort) {
		return false
	}
	if port != "" {
		if !matches(`^[0-9]+$`, port) {
			return false
		}
		digits := strings.TrimLeft(port, "0")
		if digits != "" {
			number, err := strconv.ParseUint(digits, 10, 64)
			if err != nil || number > 65535 {
				return false
			}
		}
	}
	return true
}
func cleanURL(s string) string {
	s = strings.TrimFunc(s, func(r rune) bool { return r <= 0x20 })
	return strings.NewReplacer("\t", "", "\r", "", "\n", "").Replace(s)
}

// WHATWG 的 UTS #46 使用非 STD3 模式，允许下划线和首尾连字符，
// 仍检查 Unicode 双向文本与连接字符；普通 DNS lookup profile 更严格。
var urlIDNA = idna.New(idna.MapForLookup(), idna.Transitional(false), idna.StrictDomainName(false), idna.CheckHyphens(false), idna.CheckJoiners(true), idna.BidiRule(), idna.VerifyDNSLength(false))

func httpURL(value any) bool {
	s, ok := value.(string)
	if !ok {
		return false
	}
	// 上游使用 WHATWG new URL：无基址的 HTTP(S) 是 special URL，
	// 允许 https:example.com、反斜杠和规范化后的 host；net/url 不等价。
	s = cleanURL(s)
	colon := strings.IndexByte(s, ':')
	if colon < 0 || !includes([]string{"http", "https"}, strings.ToLower(s[:colon])) {
		return false
	}
	rest := strings.TrimLeft(strings.ReplaceAll(s[colon+1:], "\\", "/"), "/")
	end := strings.IndexAny(rest, "/?#")
	if end >= 0 {
		rest = rest[:end]
	}
	if at := strings.LastIndexByte(rest, '@'); at >= 0 {
		rest = rest[at+1:]
	}
	if rest == "" {
		return false
	}
	host, port := rest, ""
	if strings.HasPrefix(rest, "[") {
		close := strings.IndexByte(rest, ']')
		if close < 0 {
			return false
		}
		ip := rest[1:close]
		if !strings.Contains(ip, ":") || net.ParseIP(ip) == nil {
			return false
		}
		suffix := rest[close+1:]
		if suffix != "" {
			if !strings.HasPrefix(suffix, ":") {
				return false
			}
			port = suffix[1:]
		}
		host = rest[:close+1]
	} else {
		if i := strings.LastIndexByte(rest, ':'); i >= 0 {
			host, port = rest[:i], rest[i+1:]
		}
		decoded, err := url.PathUnescape(host)
		if err != nil {
			return false
		}
		host, err = urlIDNA.ToASCII(decoded)
		if err != nil {
			return false
		}
		if host == "" || strings.ContainsAny(host, "\x00\t\n\r #/:<>?@[\\]^|%") {
			return false
		}
		for _, c := range host {
			if c <= 0x20 || c == 0x7f {
				return false
			}
		}
		if !validIPv4Domain(host) {
			return false
		}
	}
	if port != "" {
		if !matches(`^[0-9]+$`, port) {
			return false
		}
		n, err := strconv.ParseUint(strings.TrimLeft(port, "0"), 10, 64)
		if strings.TrimLeft(port, "0") == "" {
			n, err = 0, nil
		}
		if err != nil || n > 65535 {
			return false
		}
	}
	return host != ""
}
func ipv4Number(s string) (uint64, bool) {
	if s == "" {
		return 0, false
	}
	base := 10
	if len(s) >= 2 && strings.EqualFold(s[:2], "0x") {
		s = s[2:]
		base = 16
	} else if len(s) >= 2 && s[0] == '0' {
		s = s[1:]
		base = 8
	}
	if s == "" {
		return 0, true
	}
	pattern := `^[0-9]+$`
	if base == 8 {
		pattern = `^[0-7]+$`
	} else if base == 16 {
		pattern = `(?i)^[0-9a-f]+$`
	}
	if !matches(pattern, s) {
		return 0, false
	}
	n, err := strconv.ParseUint(s, base, 64)
	return n, err == nil
}
func validIPv4Domain(host string) bool {
	parts := strings.Split(strings.TrimSuffix(host, "."), ".")
	last := parts[len(parts)-1]
	_, number := ipv4Number(last)
	// “ends in a number” 先识别数值语法；即使十六进制大到
	// uint64 溢出也属于 IP 候选，随后必须报无效，而不是当域名接受。
	if !number && !matches(`^[0-9]+$`, last) && !matches(`(?i)^0x[0-9a-f]*$`, last) {
		return true
	}
	if len(parts) > 4 {
		return false
	}
	for i, part := range parts {
		n, ok := ipv4Number(part)
		if !ok {
			return false
		}
		limit := uint64(256)
		if i == len(parts)-1 {
			limit = uint64(1) << uint(8*(5-len(parts)))
		}
		if n >= limit {
			return false
		}
	}
	return true
}
func cssColor(value any) bool {
	s, ok := value.(string)
	if !ok || s == "" || strings.TrimSpace(s) != s || strings.ContainsAny(s, ";{}<>") {
		return false
	}
	return matches(`(?i)^#[0-9a-f]{3,4}(?:[0-9a-f]{3,4})?$`, s) || matches(`(?i)^(?:rgb|rgba|hsl|hsla|hwb|lab|lch|oklab|oklch|color|color-mix)\([^\n]+\)$`, s) || matches(`(?i)^var\(--[a-z0-9_-]+(?:\s*,[^\n]+)?\)$`, s) || matches(`(?i)^[a-z]+$`, s)
}
func cssLength(value any) bool {
	s, ok := value.(string)
	if !ok || strings.TrimSpace(s) != s || strings.ContainsAny(s, ";{}<>\n\r") {
		return false
	}
	return s == "0" || matches(`^(?:\d+|\d*\.\d+)(?:px|rem|em|%|vw|vh|vmin|vmax|ch|ex|cm|mm|in|pt|pc)$`, s)
}
func safeCSS(value any) bool {
	s, ok := value.(string)
	return ok && s != "" && strings.TrimSpace(s) == s && !strings.ContainsAny(s, ";{}<\n\r") && !matches(`(?i)url\s*\(`, s)
}
func resource(value any) bool {
	s, ok := value.(string)
	if !ok || s == "" || strings.TrimSpace(s) != s || strings.ContainsAny(s, "'\"\\<>\n\r") || matches(`(?i)^url\s*\(`, s) {
		return false
	}
	if matches(`(?i)^data:image/[a-z0-9.+-]+(?:;[a-z0-9=.+-]+)*(?:;base64)?,`, s) {
		return true
	}
	if strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//") && !includes(strings.Split(s, "/"), "..") {
		return true
	}
	if httpURL(s) {
		return true
	}
	return !matches(`(?i)^[a-z][a-z0-9+.-]*:`, s) && !includes(strings.Split(s, "/"), "..")
}
func nonnegative(value any) bool {
	n, ok := number(value)
	return ok && !math.IsNaN(n) && !math.IsInf(n, 0) && math.Trunc(n) == n && n >= 0
}
func (p *parser) validate(n *node, input any, field string) {
	if n.Validator == "" {
		return
	}
	invalid := func(expected string) { p.issue(n, "invalid_value", field, input, expected) }
	nullable := strings.HasPrefix(n.Validator, "nullable_")
	if nullable && input == nil {
		return
	}
	switch n.Validator {
	case "non_empty_string", "nullable_non_empty_string":
		if !nonempty(input) {
			invalid("non-empty string")
		}
	case "string_tree":
		entries := map[string]any{field: input}
		if m, ok := input.(map[string]any); ok {
			entries = map[string]any{}
			for k, v := range m {
				entries[dot(field, k)] = v
			}
		}
		for _, key := range keys(entries) {
			items, ok := entries[key].([]any)
			if !ok {
				p.issue(n, "invalid_type", key, entries[key], "array")
				continue
			}
			for i, item := range items {
				if typeOf(item) != "string" {
					p.issue(n, "invalid_type", fmt.Sprintf("%s[%d]", key, i), item, "string")
				}
			}
		}
	case "effect":
		p.effect(n, input, field)
	case "brand":
		p.brand(n, input, field)
	case "safe_navigation_url", "nullable_safe_navigation_url":
		if !safeNavigation(input) {
			invalid("safe navigable URL or root-relative path")
		}
	case "nullable_template_navigation_url":
		candidate := replacePattern(`(?i)\{[a-z][a-z0-9_-]*\.[a-z][a-z0-9_-]*\}`, text(input), "token")
		if typeOf(input) != "string" || (candidate != "token" && !safeNavigation(candidate)) {
			invalid("safe navigable URL, root-relative path, or supported template URL")
		}
	case "absolute_http_url", "nullable_absolute_http_url":
		if !httpURL(input) {
			invalid("absolute HTTP(S) URL")
		}
	case "css_color", "nullable_css_color":
		if !cssColor(input) {
			invalid("valid CSS color")
		}
	case "css_length":
		if !cssLength(input) {
			invalid("valid CSS length")
		}
	case "css_percentage":
		s, ok := input.(string)
		if !ok || strings.TrimSpace(s) != s || !matches(`^(?:\d+|\d*\.\d+)%$`, s) {
			invalid("non-negative CSS percentage")
		}
	case "css_font_family":
		if !safeCSS(input) {
			invalid("safe CSS font-family list")
		}
	case "css_selector":
		if !safeCSS(input) {
			invalid("safe CSS selector")
		}
	case "css_gradient":
		s, ok := input.(string)
		if !ok || strings.TrimSpace(s) != s || strings.ContainsAny(s, ";{}<>\n\r") || !matches(`(?i)^(?:repeating-)?(?:linear|radial|conic)-gradient\([\s\S]+\)$`, s) {
			invalid("valid CSS gradient")
		}
	case "sidebar_gradient_colors":
		items, _ := input.([]any)
		if len(items) != 4 {
			invalid("exactly 4 CSS colors")
		}
	case "corner_shape":
		if !matches(`^(?:round|scoop|bevel|notch|square|superellipse\((?:\d+|\d*\.\d+)\))$`, text(input)) {
			invalid("supported CSS corner-shape")
		}
	case "resource", "nullable_resource":
		if !resource(input) {
			expected := "safe Resource"
			if nullable {
				expected = "null or safe Resource"
			}
			invalid(expected)
		}
	case "non_negative_integer", "nullable_non_negative_integer":
		if !nonnegative(input) {
			expected := "non-negative integer"
			if nullable {
				expected = "null or non-negative integer"
			}
			invalid(expected)
		}
	case "non_empty_record_keys":
		m, _ := input.(map[string]any)
		for key := range m {
			if strings.TrimSpace(key) == "" {
				invalid("record with non-empty keys")
			}
		}
	case "license_value", "license_override":
		override := n.Validator == "license_override"
		if override && (input == nil || input == true) {
			return
		}
		if input == false || (typeOf(input) == "string" && text(input) != "") {
			return
		}
		if override {
			invalid("null, boolean, or non-empty string")
		} else {
			invalid("false or non-empty string")
		}
	case "share_override":
		if input != nil && typeOf(input) != "boolean" && typeOf(input) != "array" {
			invalid("null, boolean, or provider array")
		}
	case "kebab_id", "nullable_kebab_id":
		if !matches(`^[a-z0-9]+(?:-[a-z0-9]+)*$`, text(input)) {
			invalid("non-empty kebab-case id")
		}
	case "menu_items":
		p.menu(n, input, field)
	case "footer_actions":
		p.actions(n, input, field)
	case "navigation_tabs":
		items, _ := input.([]any)
		for i, item := range items {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text(m["title"]) == "" {
				p.issue(n, "invalid_value", fmt.Sprintf("%s[%d].title", field, i), valueOrUndefined(m, "title"), "non-empty string")
			}
			if !safeNavigation(m["url"]) {
				p.issue(n, "invalid_value", fmt.Sprintf("%s[%d].url", field, i), valueOrUndefined(m, "url"), "safe navigable URL or root-relative path")
			}
		}
	case "emoji_template":
		if !strings.Contains(text(input), "{name}") {
			invalid("URL template containing {name}")
		}
	case "emoji_sources":
		m, _ := input.(map[string]any)
		sources, _ := m["sources"].(map[string]any)
		selected, ok := m["default_source"].(string)
		_, exists := sources[selected]
		if !ok || !exists {
			p.issue(n, "invalid_value", dot(field, "default_source"), valueOrUndefined(m, "default_source"), "key declared in emoji.sources")
		}
	case "github_repository":
		if !matches(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`, text(input)) {
			invalid("GitHub owner/repository")
		}
	case "contributor_repositories":
		seen := map[string]bool{}
		items, _ := input.([]any)
		for i, item := range items {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			prefix, ok := m["source_prefix"].(string)
			if !ok {
				continue
			}
			prefix = strings.TrimPrefix(strings.ReplaceAll(prefix, "\\", "/"), "./")
			if seen[prefix] {
				p.issue(n, "invalid_value", fmt.Sprintf("%s[%d].source_prefix", field, i), prefix, "unique source_prefix")
			}
			seen[prefix] = true
		}
	case "diagrams_override":
		if input != false && input != "mermaid" && typeOf(input) != "object" {
			invalid("false, mermaid, or Mermaid options object")
		}
	case "safe_relative_path":
		s := text(input)
		unix := strings.ReplaceAll(s, "\\", "/")
		normalized := path.Clean(unix)
		if s == "" || strings.HasPrefix(unix, "/") || matches(`^[A-Za-z]:`, unix) || normalized == "." || includes(strings.Split(unix, "/"), "..") {
			invalid("safe non-empty relative path")
		}
	default:
		panic("unknown Stellar validator: " + n.Validator)
	}
}
func (p *parser) brand(n *node, input any, field string) {
	if input == nil || input == false {
		return
	}
	m, ok := input.(map[string]any)
	if !ok {
		p.issue(n, "invalid_value", field, input, "false or Brand object")
		return
	}
	for _, key := range keys(m) {
		if n.Properties[key] == nil {
			p.issue(n, "unknown_field", dot(field, key), m[key], strings.Join(n.PropertyOrder, " | "))
		}
	}
	for _, key := range []string{"name", "tagline"} {
		if v := m[key]; v != nil && typeOf(v) != "string" {
			p.issue(n, "invalid_type", dot(field, key), v, "string | null")
		}
	}
	if n.Properties["search"] != nil {
		if v := m["search"]; v != nil && typeOf(v) != "boolean" {
			p.issue(n, "invalid_type", dot(field, "search"), v, "boolean")
		}
	}
	if style := n.Properties["style"]; style != nil {
		if v := m["style"]; v != nil {
			if typeOf(v) != "string" {
				p.issue(n, "invalid_type", dot(field, "style"), v, "string")
			} else if style.Values != nil && !valuesInclude(style.Values, v) {
				// 官方品牌校验器直接 Array.join：这里的 null 显示为空；
				// 通用枚举校验器则明确写成 null，保留两条原始诊断。
				parts := make([]string, len(style.Values))
				for i, value := range style.Values {
					if value != nil {
						parts[i] = fmt.Sprint(value)
					}
				}
				p.issue(n, "invalid_value", dot(field, "style"), v, strings.Join(parts, " | "))
			}
		}
	}
	if v := m["href"]; v != nil && !safeNavigation(v) {
		p.issue(n, "invalid_value", dot(field, "href"), v, "safe navigable URL or root-relative path")
	}
	if name, ok := m["name"].(string); ok {
		if matches(`^\[[\s\S]*\]\([\s\S]*\)$`, strings.TrimSpace(name)) {
			p.issue(n, "invalid_value", dot(field, "name"), name, "plain text without Markdown links")
		}
		if strings.ContainsAny(name, "<>") {
			p.issue(n, "invalid_value", dot(field, "name"), name, "plain text without HTML")
		}
	}
	image := m["image"]
	if image == nil {
		return
	}
	im, ok := image.(map[string]any)
	if !ok {
		p.issue(n, "invalid_type", dot(field, "image"), image, "object")
		return
	}
	for _, key := range keys(im) {
		if !includes([]string{"src", "variant"}, key) {
			p.issue(n, "unknown_field", dot(field, "image."+key), im[key], "src | variant")
		}
	}
	if v := im["src"]; v != nil && (typeOf(v) != "string" || text(v) == "") {
		p.issue(n, "invalid_value", dot(field, "image.src"), v, "null or non-empty string")
	}
	if v := im["variant"]; v != nil && !valuesInclude([]any{"avatar", "icon", "plain"}, v) {
		p.issue(n, "invalid_value", dot(field, "image.variant"), v, "avatar | icon | plain")
	}
	if src, ok := im["src"].(string); ok && matches(`^\[[\s\S]*\]\([\s\S]*\)$`, strings.TrimSpace(src)) {
		p.issue(n, "invalid_value", dot(field, "image.src"), src, "plain image URL")
	}
}
func (p *parser) menu(n *node, input any, field string) {
	items, ok := input.([]any)
	if !ok {
		return
	}
	ids := map[string]bool{}
	for i, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		base := fmt.Sprintf("%s[%d]", field, i)
		if typ := m["type"]; typ != nil && typ != "link" {
			p.issue(n, "invalid_value", base+".type", typ, "link")
		}
		if id, ok := m["id"].(string); ok {
			if ids[id] {
				p.issue(n, "invalid_value", base+".id", id, "unique menu id")
			}
			ids[id] = true
		}
		if text(m["title"]) == "" && text(m["icon"]) == "" {
			p.issue(n, "invalid_value", base, m, "non-empty title or icon")
		}
		if text(m["id"]) == "" {
			p.issue(n, "invalid_value", base+".id", valueOrUndefined(m, "id"), "non-empty kebab-case id")
		}
		if text(m["url"]) == "" {
			p.issue(n, "invalid_value", base+".url", valueOrUndefined(m, "url"), "safe navigable URL")
		}
	}
}
func (p *parser) actions(n *node, input any, field string) {
	items, ok := input.([]any)
	if !ok {
		return
	}
	var action func(map[string]any, string, bool)
	action = func(m map[string]any, base string, nested bool) {
		typ := text(m["type"])
		if !includes([]string{"link", "button", "dropdown", "spacer"}, typ) {
			p.issue(n, "invalid_value", base+".type", valueOrUndefined(m, "type"), "link | button | dropdown | spacer")
			return
		}
		if nested && !includes([]string{"link", "button"}, typ) {
			p.issue(n, "invalid_value", base+".type", m["type"], "link | button")
			return
		}
		allowed := []string{"type"}
		switch typ {
		case "link":
			allowed = []string{"type", "icon", "title", "url"}
		case "button":
			allowed = []string{"type", "icon", "title", "onclick"}
		case "dropdown":
			allowed = []string{"type", "icon", "title", "items"}
		}
		for _, key := range keys(m) {
			if !includes(allowed, key) {
				p.issue(n, "unknown_field", base+"."+key, m[key], strings.Join(allowed, " | "))
			}
		}
		if typ == "link" && !safeNavigation(m["url"]) {
			p.issue(n, "invalid_value", base+".url", valueOrUndefined(m, "url"), "safe navigable URL or root-relative path")
		}
		if typ == "button" {
			for _, key := range []string{"title", "icon", "onclick"} {
				if key == "icon" && nested {
					continue
				}
				if text(m[key]) == "" {
					p.issue(n, "missing_field", base+"."+key, valueOrUndefined(m, key), "non-empty string")
				}
			}
		}
		if typ == "dropdown" {
			if text(m["title"]) == "" && text(m["icon"]) == "" {
				p.issue(n, "invalid_value", base, m, "non-empty title or icon")
			}
			children, ok := m["items"].([]any)
			if !ok || len(children) == 0 {
				p.issue(n, "invalid_value", base+".items", valueOrUndefined(m, "items"), "non-empty array")
			} else {
				for i, child := range children {
					if c, ok := child.(map[string]any); ok {
						action(c, fmt.Sprintf("%s.items[%d]", base, i), true)
					}
				}
			}
		}
	}
	for i, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		base := fmt.Sprintf("%s[%d]", field, i)
		action(m, base, false)
		if m["type"] == "link" && text(m["icon"]) == "" {
			p.issue(n, "missing_field", base+".icon", valueOrUndefined(m, "icon"), "non-empty string")
		}
	}
}
func (p *parser) effect(n *node, input any, field string) {
	if input == nil {
		return
	}
	m, _ := input.(map[string]any)
	for _, key := range keys(m) {
		if !includes([]string{"type", "options", "runtime"}, key) {
			p.issue(n, "unknown_field", field+"."+key, m[key], "type | options | runtime")
		}
	}
	var definition *effectDefinition
	ids := []string{}
	for i := range schemas.Effects {
		ids = append(ids, schemas.Effects[i].ID)
		if schemas.Effects[i].ID == text(m["type"]) {
			definition = &schemas.Effects[i]
		}
	}
	if text(m["type"]) == "" {
		p.issue(n, "missing_field", field+".type", valueOrUndefined(m, "type"), "non-empty string")
	} else if definition == nil {
		p.issue(n, "invalid_value", field+".type", m["type"], strings.Join(ids, " | "))
	}
	if options := m["options"]; options != nil {
		bag, ok := options.(map[string]any)
		if !ok {
			p.issue(n, "invalid_type", field+".options", options, "object")
		} else if definition != nil {
			for _, key := range keys(bag) {
				value := bag[key]
				rule, exists := definition.Options[key]
				base := field + ".options." + key
				if !exists {
					p.issue(n, "unknown_field", base, value, "known "+definition.Label+" option")
					continue
				}
				code, expected := "", ""
				numberValid := func(v any) bool { num, ok := number(v); return ok && !math.IsNaN(num) && !math.IsInf(num, 0) }
				switch rule.Type {
				case "number":
					if !numberValid(value) {
						code = "invalid_type"
						expected = "finite number"
					}
				case "nullable_positive_number":
					num, _ := number(value)
					if value != nil && (!numberValid(value) || num <= 0) {
						code = "invalid_value"
						expected = "positive finite number or null"
					}
				case "nullable_string":
					if value != nil && !nonempty(value) {
						code = "invalid_value"
						expected = "non-empty string or null"
					}
				case "boolean":
					if typeOf(value) != "boolean" {
						code = "invalid_type"
						expected = "boolean"
					}
				case "number_array":
					items, ok := value.([]any)
					valid := ok
					for _, item := range items {
						if !numberValid(item) {
							valid = false
						}
					}
					if !valid {
						code = "invalid_type"
						expected = "number[]"
					}
				case "enum":
					if typeOf(value) != "string" || !valuesInclude(rule.Values, value) {
						code = "invalid_value"
						expected = expectedValues(rule.Values)
					}
				case "hex_color":
					if typeOf(value) != "string" || !matches(`(?i)^#?[a-f\d]{6}$`, text(value)) {
						code = "invalid_value"
						expected = "six-digit hexadecimal color"
					}
				case "hex_color_array":
					minItems := 1
					if rule.MinItems != nil {
						minItems = *rule.MinItems
					}
					items, ok := value.([]any)
					valid := ok && len(items) >= minItems && len(items) <= 8
					for _, item := range items {
						if typeOf(item) != "string" || !matches(`(?i)^#?[a-f\d]{6}$`, text(item)) {
							valid = false
						}
					}
					if !valid {
						code = "invalid_value"
						expected = fmt.Sprintf("%d to eight six-digit hexadecimal colors", minItems)
					}
				default:
					panic("unknown Stellar effect option: " + rule.Type)
				}
				if code != "" {
					p.issue(n, code, base, value, expected)
				}
			}
		}
	}
	if runtime := m["runtime"]; runtime != nil {
		bag, ok := runtime.(map[string]any)
		if !ok {
			p.issue(n, "invalid_type", field+".runtime", runtime, "object")
		} else {
			for _, key := range keys(bag) {
				if !includes([]string{"pause_when_hidden", "respect_reduced_motion"}, key) {
					p.issue(n, "unknown_field", field+".runtime."+key, bag[key], "pause_when_hidden | respect_reduced_motion")
				} else if typeOf(bag[key]) != "boolean" {
					p.issue(n, "invalid_type", field+".runtime."+key, bag[key], "boolean")
				}
			}
		}
	}
}
