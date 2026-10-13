package post

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
)

func (r *shortcodeV2Renderer) emoji(a map[string]string) string {
	inner := ""
	src := a["url"]
	if src == "" {
		source, name := a["source"], a["name"]
		if source == "" {
			return `<span class="tag-plugin emoji"></span>`
		}
		if _, ok := a["name"]; !ok {
			name = source
			source = r.setting("emoji", "defaultSource")
		}
		template := v2String(v2Object(v2Object(r.tags["emoji"])["sources"])[source])
		if template != "" {
			src = v2SafeURL(strings.Replace(template, "{name}", name, 1))
		}
	}
	if src != "" {
		inner = `<img no-lazy="" class="inline"` + v2Attr("src", src)
		if a["url"] != "" {
			inner += v2Attr("alt", a["name"])
		}
		if a["height"] != "" {
			inner += v2Attr("style", "height:"+a["height"])
		}
		inner += `/>`
	}
	return `<span class="tag-plugin emoji">` + inner + `</span>`
}
func (r *shortcodeV2Renderer) iconTag(a map[string]string) string {
	v2Default(a, "color", r.setting("icon", "defaultColor"))
	more := ""
	if css := v2SafeStyle(a["style"]); css != "" {
		more = v2Attr("style", css)
	}
	el := `<span class="tag-plugin icon colorful"` + v2Attrs(a, "color") + `>` + r.icon(a["key"], more) + `</span>`
	if a["text"] != "" {
		el = `<div class="tag-plugin icon-wrap">` + el + `<span class="text">` + html.EscapeString(a["text"]) + `</span></div>`
	}
	return el
}
func v2SafeStyle(css string) string {
	// 支持官方自由 CSS 参数中纯表现声明；拒绝外部引用和可执行 CSS。
	var declarations []string
	for _, item := range strings.Split(css, ";") {
		key, value, ok := strings.Cut(item, ":")
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if !ok || !regexp.MustCompile(`^[a-zA-Z-]+$`).MatchString(key) || value == "" {
			continue
		}
		if strings.ContainsAny(value, `<>"'\\`) || regexp.MustCompile(`(?i)(url\s*\(|expression|javascript|@import|behavior|binding)`).MatchString(value) {
			continue
		}
		declarations = append(declarations, key+":"+value)
	}
	return strings.Join(declarations, ";")
}
func (r *shortcodeV2Renderer) copy(a map[string]string) string {
	text, ok := a["text"]
	if !ok {
		return ""
	}
	if git := a["git"]; git != "" {
		text = strings.TrimPrefix(text, "/")
		switch git {
		case "ssh":
			text = "git@github.com:" + text + ".git"
		case "gh":
			text = "gh repo clone " + text
		default:
			text = "https://github.com/" + text + ".git"
		}
	}
	id := r.next("copy")
	prefix := ""
	if a["prefix"] != "" {
		prefix = `<span>` + html.EscapeString(a["prefix"]) + `</span>`
	}
	return `<div class="tag-plugin copy">` + prefix + `<input class="copy-area"` + v2Attr("id", id) + v2Attr("value", text) + `><button class="copy-btn"` + v2Attr("onclick", `util.copy("`+id+`","`+r.localized("message.copied")+`")`) + `>` + r.icon("copy:copy", "") + `</button></div>`
}
func (r *shortcodeV2Renderer) quot(a map[string]string) string {
	el := safeQuotEl(a["el"])
	config := v2Object(r.tags["quot"])
	preset := a["icon"]
	if preset == "" {
		preset = "default"
	}
	cfg := v2Object(config[preset])
	prefix, suffix := v2String(cfg["prefix"]), v2String(cfg["suffix"])
	if a["prefix"] != "" || a["suffix"] != "" {
		prefix, suffix = a["prefix"], a["suffix"]
	} else if cfg == nil && a["icon"] != "" {
		if _, ok := v2Object(r.data["icons"])[a["icon"]]; ok {
			prefix = a["icon"]
		}
	}
	icon := func(key, position string) string {
		if key == "" {
			return `<span class="empty"></span>`
		}
		i := r.icon(key, v2Attr("class", "icon "+position))
		if a["color"] != "" {
			i = `<span class="quot-icon colorful"` + v2Attrs(a, "color") + `>` + i + `</span>`
		}
		return i
	}
	id := ""
	if strings.Contains(el, "h") {
		id = v2Attr("id", a["text"])
	}
	return `<div class="tag-plugin quot ` + el + `"><` + el + ` class="content"` + id + `>` + icon(prefix, "prefix") + `<span class="text">` + html.EscapeString(a["text"]) + `</span>` + icon(suffix, "suffix") + `</` + el + `></div>`
}

// JS String.split(regex) 保留捕获组且过滤空白：包括标记之前的内容，不能默默丢弃。
func v2SplitCaptured(re *regexp.Regexp, body string) []string {
	var out []string
	last := 0
	appendNonempty := func(s string) {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	for _, loc := range re.FindAllStringSubmatchIndex(body, -1) {
		appendNonempty(body[last:loc[0]])
		for i := 2; i < len(loc); i += 2 {
			if loc[i] >= 0 {
				appendNonempty(body[loc[i]:loc[i+1]])
			}
		}
		last = loc[1]
	}
	appendNonempty(body[last:])
	return out
}
func v2Pairs(re *regexp.Regexp, body string) []markerPair {
	arr := v2SplitCaptured(re, body)
	var pairs []markerPair
	for i, s := range arr {
		if i%2 == 0 {
			pairs = append(pairs, markerPair{header: s})
		} else {
			pairs[len(pairs)-1].body = s
		}
	}
	return pairs
}
func (r *shortcodeV2Renderer) tabs(a map[string]string, body string) string {
	pairs := v2Pairs(reTabSplit, body)
	if len(pairs) == 0 {
		return ""
	}
	name := r.next("tab")
	active, _ := strconv.ParseFloat(a["active"], 64)
	var nav, content strings.Builder
	for i, p := range pairs {
		id := name + "-" + strconv.Itoa(i+1)
		cl := ""
		if (active > 0 && active == float64(i+1)) || (active == 0 && i == 0) {
			cl = " active"
		}
		header := p.header
		if header == "" {
			header = name + " " + strconv.Itoa(i+1)
		}
		nav.WriteString(`<div class="tab` + cl + `"><a href="#` + id + `">` + html.EscapeString(header) + `</a></div>`)
		content.WriteString(`<div class="tab-pane` + cl + `" id="` + id + `">` + strings.TrimSpace(r.markdown(p.body)) + `</div>`)
	}
	return `<div class="tag-plugin tabs"` + v2Attrs(a, "align") + v2Attr("id", name) + `><div class="nav-tabs">` + nav.String() + `</div><div class="tab-content">` + content.String() + `</div></div>`
}
func (r *shortcodeV2Renderer) folders(a map[string]string, body string) string {
	var b strings.Builder
	b.WriteString(`<div class="tag-plugin colorful folders"` + v2Attrs(a, "color") + `>`)
	for i, p := range v2Pairs(reFolderSplit, body) {
		fmt.Fprintf(&b, `<details class="folder" index="%d"><summary>%s</summary><div class="body">%s</div></details>`, i, r.markdown(p.header), v2Compact(r.markdown(p.body)))
	}
	b.WriteString(`</div>`)
	return b.String()
}
func (r *shortcodeV2Renderer) grid(a map[string]string, body string) string {
	if _, w := a["w"]; !w {
		if _, c := a["c"]; !c {
			a["w"] = "280px"
		}
	}
	style := ""
	if a["w"] != "" {
		style = "grid-template-columns: repeat(auto-fill, minmax(" + a["w"] + ", 1fr));"
	} else if n, err := strconv.Atoi(a["c"]); err == nil && n > 0 {
		style = "grid-template-columns: repeat(" + strconv.Itoa(n) + ", 1fr);"
	}
	if a["gap"] != "" {
		style += "grid-gap:" + a["gap"] + ";"
	}
	cl := "cell"
	if a["bg"] == "card" {
		cl += " card-hover card-hover--spotlight card-hover--tilt"
	}
	var b strings.Builder
	b.WriteString(`<div class="tag-plugin grid"` + v2Attrs(a, "bg", "columns") + ` style="` + html.EscapeString(style) + `">`)
	for _, cell := range v2SplitCaptured(reCellSplit, body) {
		br := ""
		if a["br"] != "" {
			br = "border-radius:" + a["br"] + ";"
		}
		b.WriteString(`<div class="` + cl + `" style="` + br + `">` + v2Compact(r.markdown(cell)) + `</div>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}
func (r *shortcodeV2Renderer) gallery(a map[string]string, body string) string {
	v2Default(a, "size", r.setting("gallery", "size"))
	v2Default(a, "aspect_ratio", r.setting("gallery", "aspectRatio"))
	layout := "grid"
	if a["layout"] == "flow" {
		layout = "flow"
	}
	id := strings.Replace(r.next("gallery"), "_", "-", 1)
	var b strings.Builder
	b.WriteString(`<div class="tag-plugin gallery ` + layout + `-box"` + v2Attrs(a, "size", "aspect_ratio") + `>`)
	for _, line := range strings.Split(body, "\n") {
		m := reGalleryMD.FindStringSubmatch(line)
		if len(m) < 3 {
			continue
		}
		src := v2SafeURL(m[2])
		if src == "" {
			continue
		}
		b.WriteString(`<div class="` + layout + `-cell lazy-box"><img class="lazy"` + v2Attr("data-fancybox", id) + v2Attr("data-src", src) + v2Attr("alt", m[1]) + `/><div class="lazy-icon"></div><div class="image-meta">`)
		if m[1] != "" {
			b.WriteString(`<span class="image-caption">` + html.EscapeString(m[1]) + `</span>`)
		}
		b.WriteString(`</div></div>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}
func (r *shortcodeV2Renderer) swiper(a map[string]string, body string) string {
	var b strings.Builder
	b.WriteString(`<div class="tag-plugin swiper fancybox" id="swiper-api"` + v2Attrs(a, "width", "effect") + `><div class="swiper-wrapper">`)
	for _, img := range reSwiperImg.FindAllString(r.markdown(body), -1) {
		b.WriteString(`<div class="swiper-slide">` + strings.Replace(img, "<img src", "<img no-lazy src", 1) + `</div>`)
	}
	b.WriteString(`</div><div class="swiper-pagination"></div><div class="swiper-button-prev blur"></div><div class="swiper-button-next blur"></div></div>`)
	return b.String()
}
func (r *shortcodeV2Renderer) dropdown(a map[string]string, body string) string {
	items := parseDropdownItems(body)
	if a["title"] == "" || len(items) == 0 {
		return ""
	}
	direction, align := "auto", "auto"
	if a["direction"] == "up" || a["direction"] == "down" {
		direction = a["direction"]
	}
	if a["align"] == "left" || a["align"] == "right" {
		align = a["align"]
	}
	open := ""
	if a["open"] == "true" {
		open = " open"
	}
	var b strings.Builder
	b.WriteString(`<details class="tag-plugin dropdown"` + v2Attr("direction", direction) + v2Attr("align", align) + open + `><summary class="dropdown-trigger ui-interactive"` + v2Attr("title", a["title"]) + v2Attr("aria-label", a["title"]) + `><span>` + html.EscapeString(a["title"]) + `</span><span class="ui-icon dropdown-arrow" aria-hidden="true">` + r.icon("default:chevron-left", " no-lazy") + `</span></summary><div class="dropdown-menu ui-collection" data-layout="list" data-variant="nav" data-density="compact">`)
	for _, item := range items {
		u := v2SafeURL(item.url)
		if u == "" {
			continue
		}
		b.WriteString(`<a class="dropdown-item ui-collection__item ui-interactive card-hover card-hover--spotlight"` + v2Attr("href", u))
		if strings.Contains(u, "://") {
			b.WriteString(` target="_blank" rel="external nofollow noopener noreferrer">`)
		} else {
			b.WriteString(` rel="noopener noreferrer">`)
		}
		if item.icon != "" {
			b.WriteString(`<span class="ui-icon">` + r.icon(item.icon, " no-lazy") + `</span>`)
		}
		b.WriteString(`<span class="ui-collection__content"><span class="ui-collection__title">` + html.EscapeString(item.title) + `</span></span></a>`)
	}
	b.WriteString(`</div></details>`)
	return b.String()
}
func (r *shortcodeV2Renderer) read(name string, a map[string]string, body string) string {
	title, author, date, footer := html.EscapeString(a["title"]), html.EscapeString(a["author"]), html.EscapeString(a["date"]), html.EscapeString(a["footer"])
	if name == "poetry" {
		el := `<div class="tag-plugin poetry"><div class="content">`
		if title != "" {
			el += `<div class="title">` + title + `</div>`
		}
		if author != "" || date != "" {
			el += `<div class="meta">`
			if author != "" {
				el += `<span>` + author + `</span>`
			}
			if date != "" {
				el += `<span>` + date + `</span>`
			}
			el += `</div>`
		}
		el += `<div class="body">` + v2Compact(r.markdown(body)) + `</div>`
		if footer != "" {
			el += `<div class="footer">` + footer + `</div>`
		}
		return el + `</div></div>`
	}
	if name == "reel" {
		el := `<div class="tag-plugin reel"><div class="content"><div class="title">` + title + `</div>`
		if author != "" {
			el += `<div class="meta"><span>` + author + `</span></div>`
		}
		el += `<div class="body"><div class="main">` + v2Compact(r.markdown(body)) + `</div></div>`
		if date != "" {
			el += `<div class="date">` + date + `</div>`
		}
		return el + `<div class="footer">` + footer + `</div></div></div>`
	}
	var b strings.Builder
	class := "content"
	if style, ok := a["style"]; ok {
		class += " " + style
	}
	b.WriteString(`<div class="tag-plugin paper"><div` + v2Attr("class", class) + `><div class="title">` + title + `</div><div class="body">`)
	// 按官方捕获数组运行小状态机，保留 marker 之前/之间的相同过滤规则。
	parts := v2SplitCaptured(regexp.MustCompile(`<!--\s*(paragraph|section|line) (.*?)\s*-->`), body)
	kind, innerTitle := "", ""
	for _, part := range parts {
		if part == "paragraph" || part == "section" || part == "line" {
			kind = part
			continue
		}
		switch kind {
		case "paragraph":
			b.WriteString(`<div class="paragraph">` + v2Compact(r.markdown(part)) + `</div>`)
		case "line":
			if innerTitle == "" {
				innerTitle = part
			} else {
				cl := "line"
				if innerTitle == "right" {
					cl += " right"
				}
				b.WriteString(`<div class="` + cl + `">` + v2Compact(r.markdown(part)) + `</div>`)
				innerTitle = ""
			}
		case "section":
			if innerTitle == "" {
				innerTitle = part
			} else {
				b.WriteString(`<div class="section"><div class="section-title">` + html.EscapeString(innerTitle) + `</div><div class="section-content">` + v2Compact(r.markdown(part)) + `</div></div>`)
				innerTitle = ""
			}
		}
	}
	b.WriteString(`</div><div class="footer">`)
	if author != "" || date != "" {
		b.WriteString(`<div class="author-date">`)
		if author != "" {
			b.WriteString(`<span class="author">` + author + `</span>`)
		}
		if date != "" {
			b.WriteString(`<span class="date">` + date + `</span>`)
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(footer + `</div></div></div>`)
	return b.String()
}

var v2CSSValueCharacters = regexp.MustCompile(`^[A-Za-z0-9_#%.(),/+:* \t-]+$`)

func v2CSSValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 1024 || !v2CSSValueCharacters.MatchString(value) || regexp.MustCompile(`(?i)(url\s*\(|expression|javascript|@import|behavior|binding)`).MatchString(value) {
		return ""
	}
	depth := 0
	for _, c := range value {
		if c == '(' {
			depth++
		}
		if c == ')' {
			depth--
		}
		if depth < 0 {
			return ""
		}
	}
	if depth != 0 {
		return ""
	}
	return value
}
