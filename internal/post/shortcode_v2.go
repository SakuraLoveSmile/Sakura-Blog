package post

import (
	"blog/internal/plugin"
	"blog/internal/stellarv2"
	"context"
	"fmt"
	"github.com/dop251/goja"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const shortcodeV2MaxBody = 32 << 20
const shortcodeV2MaxDepth = 64

// 每次渲染独立上下文；传入快照只读，不在 Service/包全局保存页面设置或计数器。
type shortcodeV2Renderer struct {
	s              *Service
	tags           map[string]any
	services       map[string]any
	fallbacks      map[string]any
	language       string
	data           map[string]any
	page           map[string]any
	theme          map[string]any
	index          map[string]int
	depth          int
	err            error
	blogExtensions bool
	offline        bool
	katexVM        *goja.Runtime
	katexRender    goja.Callable
	katexOptions   *goja.Object
	katexDeadline  time.Time
}

func v2Object(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}
func v2String(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func (r *shortcodeV2Renderer) setting(tag, key string) string {
	return v2String(v2Object(r.tags[tag])[key])
}
func (r *shortcodeV2Renderer) next(kind string) string {
	r.index[kind]++
	return kind + "_" + strconv.Itoa(r.index[kind])
}
func v2Attr(k, v string) string {
	if v == "" {
		return ""
	}
	return ` ` + k + `="` + html.EscapeString(v) + `"`
}
func v2Attrs(args map[string]string, keys ...string) string {
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(v2Attr(k, args[k]))
	}
	return b.String()
}
func v2Compact(s string) string { return strings.ReplaceAll(s, "\n", "") }
func v2Inline(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(v2Compact(s), "<p>", ""), "</p>", "")
}
func (r *shortcodeV2Renderer) markdown(body string) string {
	r.depth++
	defer func() { r.depth-- }()
	if r.depth > shortcodeV2MaxDepth {
		r.fail("", 0, "nesting exceeds 64 levels")
		return ""
	}
	var b strings.Builder
	prepared, mathReplacements := r.prepareMath(r.expand(body))
	if err := r.s.md.Convert([]byte(prepared), &b); err != nil {
		return html.EscapeString(body)
	}
	result := b.String()
	for marker, rendered := range mathReplacements {
		result = strings.ReplaceAll(result, marker, rendered)
	}
	return result
}
func (r *shortcodeV2Renderer) icon(key, more string) string {
	if u := v2SafeURL(key); u != "" && (strings.HasPrefix(u, "/") || strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")) {
		return `<img` + more + v2Attr("src", u) + ` />`
	}
	if safeIconKey(key) == "" {
		return ""
	}
	if r.s.iconFunc != nil {
		return r.s.iconFunc(key)
	}
	return `<svg class="icon" viewBox="0 0 1 1"` + v2Attr("data-icon", key) + ` aria-hidden="true"></svg>`
}

// RenderStellarV2 显式选择官方标签语义；原 Render 仍服务未迁移预览及 Blog 扩展。
type StellarRenderContext struct {
	Theme          map[string]any
	Data           map[string]any
	Page           map[string]any
	Language       string
	BlogExtensions bool
	Offline        bool
}

func (s *Service) RenderStellarV2(body string, tags map[string]any) (string, error) {
	if tags == nil {
		return s.RenderStellarV2WithContext(body, StellarRenderContext{})
	}
	return s.RenderStellarV2WithContext(body, StellarRenderContext{Theme: map[string]any{"tags": tags}})
}

func (s *Service) RenderStellarV2WithContext(body string, renderContext StellarRenderContext) (string, error) {
	if len(body) > shortcodeV2MaxBody {
		return "", fmt.Errorf("stellar shortcode body exceeds 32 MiB")
	}
	if s.plugins != nil {
		v, e := s.plugins.Apply(context.Background(), plugin.HookShortcode, body)
		if e != nil {
			return "", e
		}
		if text, ok := v.(string); ok {
			body = text
		}
	}
	theme := v2Merge(stellarv2.DefaultTheme(), renderContext.Theme)
	language := renderContext.Language
	if language == "" {
		language = "zh-CN"
	}
	r := &shortcodeV2Renderer{s: s, tags: v2Object(theme["tags"]), services: v2Object(theme["services"]), fallbacks: v2Object(theme["fallbacks"]), theme: theme, data: renderContext.Data, page: renderContext.Page, index: map[string]int{}, language: language, blogExtensions: renderContext.BlogExtensions, offline: renderContext.Offline}
	result := wrapTables(r.markdown(body))
	if r.err != nil {
		return "", r.err
	}
	if s.plugins != nil {
		v, e := s.plugins.Apply(context.Background(), plugin.HookContentFilter, result)
		if e != nil {
			return "", e
		}
		if text, ok := v.(string); ok {
			result = text
		}
	}
	return result, nil
}

var v2Directive = regexp.MustCompile(`(?s){%\s*([A-Za-z][A-Za-z0-9_-]*)(.*?)%}`)

// 返回代码区段边界，包括任意长度 fence/行内 backtick；示例语法不会被展开。
func v2CodeRanges(body string) [][2]int {
	var ranges [][2]int
	fenceStart, fenceN := -1, 0
	var fenceChar byte
	off := 0
	for _, line := range strings.SplitAfter(body, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)
		n := 0
		if indent <= 3 && len(trimmed) > 0 && (trimmed[0] == '`' || trimmed[0] == '~') {
			for n < len(trimmed) && trimmed[n] == trimmed[0] {
				n++
			}
		}
		if fenceStart >= 0 {
			if n >= fenceN && trimmed[0] == fenceChar && strings.TrimSpace(trimmed[n:]) == "" {
				ranges = append(ranges, [2]int{fenceStart, off + len(line)})
				fenceStart = -1
			}
		} else if n >= 3 {
			fenceStart = off
			fenceN = n
			fenceChar = trimmed[0]
		} else if indent >= 4 || strings.HasPrefix(line, "\t") {
			ranges = append(ranges, [2]int{off, off + len(line)})
		}
		off += len(line)
	}
	if fenceStart >= 0 {
		ranges = append(ranges, [2]int{fenceStart, len(body)})
	}
	initial := append([][2]int(nil), ranges...)
	fenceIndex := 0
	for i := 0; i < len(body); i++ {
		for fenceIndex < len(initial) && i >= initial[fenceIndex][1] {
			fenceIndex++
		}
		if fenceIndex < len(initial) && i >= initial[fenceIndex][0] {
			i = initial[fenceIndex][1] - 1
			continue
		}
		if body[i] != '`' {
			continue
		}
		n := 1
		for i+n < len(body) && body[i+n] == '`' {
			n++
		}
		j := i + n
		for j < len(body) {
			if body[j] != '`' {
				j++
				continue
			}
			m := 1
			for j+m < len(body) && body[j+m] == '`' {
				m++
			}
			if m == n {
				ranges = append(ranges, [2]int{i, j + m})
				i = j + m - 1
				break
			}
			j += m
		}
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i][0] < ranges[j][0] })
	return ranges
}
func v2InCode(pos int, ranges [][2]int) bool {
	i := sort.Search(len(ranges), func(i int) bool { return ranges[i][0] > pos }) - 1
	return i >= 0 && pos < ranges[i][1]
}

func (r *shortcodeV2Renderer) expand(body string) string {
	locations := v2Directive.FindAllStringSubmatchIndex(body, -1)
	code := v2CodeRanges(body)
	// Nunjucks 在标签开符之后找不到 %} 时失败；代码示例不参与此检查。
	for offset := 0; offset < len(body); {
		relative := strings.Index(body[offset:], "{%")
		if relative < 0 {
			break
		}
		start := offset + relative
		if v2InCode(start, code) {
			offset = start + 2
			continue
		}
		relativeEnd := strings.Index(body[start+2:], "%}")
		if relativeEnd < 0 {
			r.fail("", 1+strings.Count(body[:start], "\n"), "unterminated tag: missing %}")
			return ""
		}
		offset = start + 2 + relativeEnd + 2
	}
	closingTags := r.matchBlocks(body, locations, code)
	if r.err != nil {
		return ""
	}
	var out strings.Builder
	cursor := 0
	for i := 0; i < len(locations); i++ {
		l := locations[i]
		if l[0] < cursor || v2InCode(l[0], code) {
			continue
		}
		name := strings.ToLower(body[l[2]:l[3]])
		entry, ok := shortcodeV2Entry(name)
		blog := false
		if !ok && r.blogExtensions {
			entry, ok = shortcodeBlogEntry(name)
			blog = ok
		}
		if !ok {
			r.fail(name, 1+strings.Count(body[:l[0]], "\n"), "unknown tag or unexpected closing tag")
			continue
		}
		argsRaw := strings.TrimSpace(body[l[4]:l[5]])
		content := ""
		end := l[1]
		if entry.Type == ShortcodeBlock {
			closing, found := closingTags[i]
			if !found {
				r.fail(name, 1+strings.Count(body[:l[0]], "\n"), "missing end"+name)
				continue
			}
			content = body[l[1]:locations[closing][0]]
			end = locations[closing][1]
			i = closing
		}
		tokens, e := v2Args(argsRaw)
		if e != nil {
			r.fail(name, 1+strings.Count(body[:l[0]], "\n"), e.Error())
			continue
		}
		args := v2MapArgs(tokens, entry.Params)
		value := ""
		if blog {
			value = r.renderBlog(name, argsRaw, content)
		} else {
			value = r.render(name, args, content, argsRaw)
		}
		out.WriteString(body[cursor:l[0]])
		out.WriteString(value)
		cursor = end
	}
	out.WriteString(body[cursor:])
	return out.String()
}

// 等价 utils.js args.map：最后一个位置字段吸收尾部，未知命名字段属于位置文本。
func v2MapArgs(tokens []string, params []ShortcodeParamSchema) map[string]string {
	args := map[string]string{}
	keys := map[string]bool{}
	var positionalKeys, other []string
	for _, p := range params {
		if p.Role == ShortcodeParamNamed || p.Role == ShortcodeParamFlag || p.NamedAlias {
			keys[p.Key] = true
		}
		if p.Role == ShortcodeParamPositional {
			positionalKeys = append(positionalKeys, p.Key)
		}
	}
	for _, t := range tokens {
		k, v, ok := strings.Cut(strings.TrimSpace(t), ":")
		if ok && keys[k] {
			args[k] = v
		} else {
			other = append(other, t)
		}
	}
	for i, k := range positionalKeys {
		if i >= len(other) {
			if len(other) > 0 {
				delete(args, k)
			}
			continue
		}
		if i == len(positionalKeys)-1 {
			args[k] = strings.Join(other[i:], " ")
		} else {
			args[k] = other[i]
		}
	}
	return args
}
func v2Default(args map[string]string, key, value string) {
	if _, ok := args[key]; !ok {
		args[key] = value
	}
}
func (r *shortcodeV2Renderer) endpoint(service string) string {
	m := v2Object(r.services[service])
	p := v2String(m["provider"])
	if p == "" {
		return ""
	}
	return v2SafeURL(v2String(v2Object(m[p])["endpoint"]))
}
func v2SafeArgs(args map[string]string) {
	for _, k := range []string{"url", "href", "src", "img", "video", "image", "avatar", "bg", "api"} {
		if v, ok := args[k]; ok {
			args[k] = v2SafeURL(v)
		}
	}
	for _, k := range []string{"width", "height", "padding", "gap", "br", "w"} {
		if v, ok := args[k]; ok {
			args[k] = v2CSSValue(v)
		}
	}
	if v, ok := args["ratio"]; ok {
		args["ratio"] = v2CSSValue(v)
	}
	if v, ok := args["color"]; ok {
		args["color"] = safeColor(v)
	}
}
func (r *shortcodeV2Renderer) render(name string, a map[string]string, body, raw string) string {
	swiperWidth, imageBackground := a["width"], a["bg"]
	v2SafeArgs(a)
	if name == "swiper" {
		a["width"] = swiperWidth
	}
	if name == "image" {
		a["bg"] = v2CSSValue(imageBackground)
	}
	switch name {
	case "u", "emp", "wavy", "del", "kbd", "psw", "blur":
		return `<` + name + `>` + html.EscapeString(a["text"]) + `</` + name + `>`
	case "sup", "sub":
		return `<` + name + ` class="tag-plugin colorful ` + name + `"` + v2Attrs(a, "color") + `>` + html.EscapeString(a["text"]) + `</` + name + `>`
	case "tip":
		return `<span class="tag-plugin tip" tabindex="0"><span class="tip-text">` + v2Inline(r.markdown(a["text"])) + `</span><span class="tip-bubble" role="tooltip">` + html.EscapeString(a["pop"]) + `</span></span>`
	case "note", "box":
		v2Default(a, "color", r.setting("note", "defaultColor"))
		title := a["title"]
		inner := body
		if name == "note" {
			inner = a["content"]
			if inner == "" {
				inner = title
				title = ""
			}
		}
		head := ""
		if title != "" {
			t := html.EscapeString(title)
			if name == "box" {
				t = "<strong>" + r.markdown(title) + "</strong>"
			}
			head = `<div class="title">` + t + `</div>`
		}
		return `<div class="tag-plugin colorful note"` + v2Attrs(a, "color", "child") + `>` + head + `<div class="body">` + v2Compact(r.markdown(inner)) + `</div></div>`
	case "folding":
		open := ""
		if a["open"] == "true" {
			open = " open"
		}
		return `<details class="tag-plugin colorful folding"` + v2Attrs(a, "color", "child") + open + `><summary>` + r.markdown(a["title"]) + `</summary><div class="body">` + strings.ReplaceAll(r.markdown(body), "\n", " ") + `</div></details>`
	case "checkbox", "radio":
		checked := ""
		if a["checked"] == "true" {
			checked = ` checked="true"`
		}
		cl := ""
		if name == "radio" {
			cl = ` class="ui-radio"`
		}
		return `<div class="tag-plugin colorful checkbox"` + v2Attrs(a, "color", "symbol") + `><input` + cl + ` type="` + name + `"` + checked + `/><span>` + html.EscapeString(a["text"]) + `</span></div>`
	case "button":
		if a["text"] == "" || a["url"] == "" {
			return ""
		}
		if strings.Contains(a["url"], " ") {
			p := strings.Split(a["url"], " ")
			a["text"] += " " + strings.Join(p[:len(p)-1], " ")
			a["url"] = v2SafeURL(p[len(p)-1])
		}
		if a["url"] == "" {
			return ""
		}
		v2Default(a, "color", r.setting("button", "defaultColor"))
		icon := ""
		if a["icon"] != "" {
			icon = r.icon(a["icon"], "")
		}
		return `<a class="tag-plugin colorful button"` + v2Attrs(a, "color", "size") + v2Attr("title", a["text"]) + v2Attr("href", a["url"]) + `>` + icon + `<span>` + html.EscapeString(a["text"]) + `</span></a>`
	case "mark":
		v2Default(a, "color", r.setting("mark", "defaultColor"))
		tip := ""
		if a["tip"] != "" {
			tip = `<span class="mark-tip">` + html.EscapeString(a["tip"]) + `</span>`
		}
		return `<mark class="tag-plugin colorful mark"` + v2Attrs(a, "color") + `>` + html.EscapeString(a["text"]) + tip + `</mark>`
	case "hashtag":
		if _, ok := a["color"]; !ok {
			a["color"] = r.setting("hashtag", "defaultColor")
			if a["color"] == "" {
				colors := []string{"red", "orange", "yellow", "green", "cyan", "blue", "purple"}
				a["color"] = colors[r.index["hashtag"]%len(colors)]
				r.index["hashtag"]++
			}
		}
		return `<a class="tag-plugin colorful hashtag"` + v2Attrs(a, "color", "href") + `>` + r.icon("hashtag:hashtag", "") + `<span>` + html.EscapeString(a["text"]) + `</span></a>`
	case "emoji":
		return r.emoji(a)
	case "icon":
		return r.iconTag(a)
	case "copy":
		return r.copy(a)
	case "quot":
		return r.quot(a)
	case "tabs":
		return r.tabs(a, body)
	case "folders":
		return r.folders(a, body)
	case "grid":
		return r.grid(a, body)
	case "gallery":
		return r.gallery(a, body)
	case "swiper":
		return r.swiper(a, body)
	case "dropdown":
		return r.dropdown(a, body)
	case "table":
		style := a["style"]
		if style != "wrap" && style != "compact" {
			style = "scroll"
		}
		return `<div class="tag-plugin table table-` + style + `">` + r.markdown(body) + `</div>`
	case "blockquote":
		indent := "undefined"
		if v, ok := a["indent"]; ok {
			indent = v
		}
		return `<div class="tag-plugin blockquote"` + v2Attr("indent", indent) + `>` + v2Compact(r.markdown(body)) + `</div>`
	case "poetry", "reel", "paper":
		return r.read(name, a, body)
	case "navbar":
		return r.navbar(a)
	case "banner":
		return r.banner(a, body)
	case "frame":
		return r.frame(a)
	case "image":
		return r.image(a)
	case "video":
		return r.video(a)
	case "audio":
		return r.audio(a)
	case "link":
		return r.link(a)
	case "rating", "vote":
		return r.reaction(name, a)
	case "ghcard":
		return r.ghcard(a)
	case "md":
		return r.remoteMD(a)
	case "friends", "sites", "albums", "posters":
		return r.collection(name, a)
	case "toc":
		return r.toc(a)
	case "timeline":
		return r.timeline(raw, body)
	case "okr":
		return r.okr(a, body)
	case "mbti":
		return r.mbti(a)
	case "chat":
		return r.chat(a, body)
	case "gist":
		if !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9]+$`).MatchString(a["gist"]) {
			return `<span class="stellar-tag-error" data-tag="gist">Invalid Gist owner/id</span>`
		}
		q := url.Values{"gist": {a["gist"]}}
		if a["file"] != "" {
			q.Set("file", a["file"])
		}
		return `<iframe class="stellar-gist-embed" data-stellar-gist style="width:100%;height:200px;border:0"` + v2Attr("src", "/stellar/embed/gist?"+q.Encode()) + ` sandbox="allow-scripts" loading="lazy" title="GitHub Gist"></iframe>`
	}
	return ""
}

func v2Merge(base, overlay map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		if m, ok := v.(map[string]any); ok {
			out[k] = v2Merge(v2Object(base[k]), m)
		} else {
			out[k] = v
		}
	}
	return out
}

func v2SafeURL(value string) string {
	// 浏览器会忽略scheme内的ASCII控制符；先拒绝，避免java\tscript绕过旧scheme正则。
	for _, c := range value {
		if c < 32 || c == 127 {
			return ""
		}
	}
	return safeURL(value)
}

func (r *shortcodeV2Renderer) fail(tag string, line int, message string) {
	if r.err == nil {
		r.err = fmt.Errorf("stellar v2 tag %s at line %d: %s", tag, line, message)
	}
}
func shortcodeBlogEntry(name string) (ShortcodeCatalogEntry, bool) {
	if !strings.HasPrefix(name, "blog_") {
		return ShortcodeCatalogEntry{}, false
	}
	legacy := strings.TrimPrefix(name, "blog_")
	switch legacy {
	case "about", "links", "moments", "ghrepo", "ghuser", "tip":
	default:
		return ShortcodeCatalogEntry{}, false
	}
	e, ok := findShortcodeCatalogEntry(legacy)
	e.Name = name
	return e, ok
}
func ShortcodeCatalogBlogExtensions() []ShortcodeCatalogEntry {
	var out []ShortcodeCatalogEntry
	for _, name := range []string{"blog_about", "blog_links", "blog_moments", "blog_ghrepo", "blog_ghuser", "blog_tip"} {
		e, _ := shortcodeBlogEntry(name)
		e.Signature = strings.ReplaceAll(e.Signature, "{% "+strings.TrimPrefix(name, "blog_"), "{% "+name)
		e.Example = strings.ReplaceAll(e.Example, "{% "+strings.TrimPrefix(name, "blog_"), "{% "+name)
		e.Signature = strings.ReplaceAll(e.Signature, "{% end"+strings.TrimPrefix(name, "blog_"), "{% end"+name)
		e.Example = strings.ReplaceAll(e.Example, "{% end"+strings.TrimPrefix(name, "blog_"), "{% end"+name)
		out = append(out, e)
	}
	return out
}
func (r *shortcodeV2Renderer) renderBlog(name, args, body string) string {
	var rendered string
	switch name {
	case "blog_about":
		rendered = renderAbout(args, body, r.markdown)
	case "blog_tip":
		rendered = renderTip(args, body, r.markdown)
	case "blog_links":
		rendered = r.s.renderLinks()
	case "blog_moments":
		rendered = r.s.renderMoments(args)
	case "blog_ghrepo":
		if r.offline {
			rendered = r.offlineGitHub("repo", args)
		} else {
			rendered = renderGHRepo(args, r.s.ghClient)
		}
	case "blog_ghuser":
		if r.offline {
			rendered = r.offlineGitHub("user", args)
		} else {
			rendered = renderGHUser(args, r.s.ghClient)
		}
	}
	return `<div` + v2Attr("data-blog-service", strings.TrimPrefix(name, "blog_")) + `>` + rendered + `</div>`
}

func (r *shortcodeV2Renderer) matchBlocks(body string, locations [][]int, code [][2]int) map[int]int {
	closings := map[int]int{}
	var stack []int
	if len(locations) > 131072 {
		r.fail("", 1, "directive count exceeds 131072")
		return closings
	}
	lookup := func(name string) (ShortcodeCatalogEntry, bool) {
		e, ok := shortcodeV2Entry(name)
		if !ok && r.blogExtensions {
			return shortcodeBlogEntry(name)
		}
		return e, ok
	}
	for i, l := range locations {
		if v2InCode(l[0], code) {
			continue
		}
		name := strings.ToLower(body[l[2]:l[3]])
		line := 0
		if strings.HasPrefix(name, "end") {
			if len(stack) == 0 {
				r.fail(name, 1+strings.Count(body[:l[0]], "\n"), "unexpected closing tag")
				return closings
			}
			opening := stack[len(stack)-1]
			p := locations[opening]
			expected := "end" + strings.ToLower(body[p[2]:p[3]])
			if expected != name {
				r.fail(name, 1+strings.Count(body[:l[0]], "\n"), "expected "+expected)
				return closings
			}
			closings[opening] = i
			stack = stack[:len(stack)-1]
			continue
		}
		e, ok := lookup(name)
		if !ok {
			line = 1 + strings.Count(body[:l[0]], "\n")
			r.fail(name, line, "unknown tag")
			return closings
		}
		if e.Type == ShortcodeBlock {
			stack = append(stack, i)
			if len(stack) > shortcodeV2MaxDepth {
				r.fail(name, 1+strings.Count(body[:l[0]], "\n"), "nesting exceeds 64 levels")
				return closings
			}
		}
	}
	if len(stack) > 0 {
		l := locations[stack[0]]
		name := strings.ToLower(body[l[2]:l[3]])
		r.fail(name, 1+strings.Count(body[:l[0]], "\n"), "missing end"+name)
	}
	return closings
}

func (r *shortcodeV2Renderer) offlineGitHub(kind, args string) string {
	path := strings.TrimSpace(args)
	path = strings.TrimPrefix(path, "https://github.com/")
	if kind == "user" {
		path = strings.TrimPrefix(path, "@")
	}
	if (kind == "repo" && safeGHRepo(path) == "") || (kind == "user" && !regexp.MustCompile(`^[A-Za-z0-9_.-]+$`).MatchString(path)) {
		return `<span class="stellar-tag-error">Invalid GitHub identifier</span>`
	}
	base := strings.TrimRight(v2SafeURL(v2String(v2Object(r.services["github"])["apiUrl"])), "/")
	endpoint := base + "/users/" + url.PathEscape(path)
	field := "login"
	if kind == "repo" {
		endpoint = base + "/repos/" + path
		field = "full_name"
	}
	return `<div class="tag-plugin ghcard data-service ds-ghinfo ds-gh` + kind + `"` + v2Attr("data-api", endpoint) + `><a class="ghcard" type="link" id="html_url"` + v2Attr("href", "https://github.com/"+path) + ` target="_blank" rel="external nofollow noopener noreferrer"><span type="text"` + v2Attr("id", field) + `>` + html.EscapeString(path) + `</span><span type="text" id="description"></span></a></div>`
}
