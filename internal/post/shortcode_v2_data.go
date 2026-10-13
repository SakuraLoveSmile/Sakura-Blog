package post

import (
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
)

func v2List(v any) []any {
	switch v := v.(type) {
	case []any:
		return v
	case []map[string]any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = v[i]
		}
		return out
	}
	return nil
}
func v2First(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := v2String(item[key]); value != "" {
			return value
		}
	}
	return ""
}
func (r *shortcodeV2Renderer) collection(name string, a map[string]string) string {
	raw := strings.TrimRight(v2String(v2Object(r.services["github"])["rawUrl"]), "/")
	api := a["api"]
	if api == "" && safeGHRepo(a["repo"]) != "" {
		api = raw + "/" + a["repo"] + "/output/v2/data.json"
	}
	wrapper := ""
	service := "friends"
	switch name {
	case "friends":
		wrapper = "users-wrap"
		if a["posts"] != "" {
			wrapper = "users-posts-wrap"
			service = "friends_and_posts"
		}
	case "sites":
		wrapper = "sites-wrap"
		service = "sites"
	default:
		wrapper = name + "-wrap"
	}
	var b strings.Builder
	b.WriteString(`<div class="tag-plugin ` + wrapper + `">`)
	gallery := name == "albums" || name == "posters"
	size := a["size"]
	if _, ok := a["size"]; !ok {
		if name == "posters" {
			size = "xs"
		} else {
			size = "s"
		}
	}
	grid := `<div class="grid-box">`
	if gallery {
		ratio := "square"
		if name == "posters" && api == "" {
			ratio = "portrait"
		}
		grid = `<div class="tag-plugin gallery grid-box" layout="grid" aspect_ratio="` + ratio + `"` + v2Attr("size", size) + `>`
	}
	if api != "" {
		b.WriteString(`<div class="data-service ds-` + service + `"` + v2Attr("data-api", api) + `>` + grid + `</div></div>`)
	} else if a["group"] != "" {
		b.WriteString(grid)
		links := v2Object(r.data["links"])
		for _, value := range v2List(links[a["group"]]) {
			item := v2Object(value)
			u := v2SafeURL(v2String(item["url"]))
			title := v2String(item["title"])
			if u == "" || (!gallery && title == "") {
				continue
			}
			b.WriteString(r.collectionCard(name, item, u, title))
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}
func v2LazyImage(source, fallback, cl string) string {
	source = v2SafeURL(source)
	fallback = v2SafeURL(fallback)
	// 自有失败处理消费 data-fallback；不拼作者可控事件处理器。
	return `<img class="lazy` + cl + `"` + v2Attr("data-src", source) + v2Attr("data-fallback", fallback) + `/><div class="lazy-icon"></div>`
}
func (r *shortcodeV2Renderer) collectionCard(kind string, item map[string]any, u, title string) string {
	icon := v2SafeURL(v2First(item, "appicon", "icon", "avatar"))
	avatar := v2String(r.fallbacks["avatar"])
	coverFallback := v2String(r.fallbacks["cover"])
	linkFallback := v2String(r.fallbacks["linkCard"])
	switch kind {
	case "friends":
		if icon == "" {
			icon = avatar
		}
		return `<div class="grid-cell user-card"><a class="card-link" target="_blank" rel="external nofollow noopener noreferrer"` + v2Attr("href", u) + `><div class="lazy-box icon">` + v2LazyImage(icon, avatar, "") + `</div><div class="name"><span>` + html.EscapeString(title) + `</span></div></a></div>`
	case "albums", "posters":
		cover := v2SafeURL(v2First(item, "cover", "appicon", "icon", "avatar"))
		if cover == "" {
			cover = coverFallback
		}
		caption := ""
		if title != "" {
			caption = `<span class="image-caption">` + html.EscapeString(title) + `</span>`
		}
		card := strings.TrimSuffix(kind, "s")
		return `<div class="grid-cell ` + card + `-card"><a class="card-link lazy-box" target="_blank" rel="external nofollow noopener noreferrer"` + v2Attr("href", u) + `>` + v2LazyImage(cover, coverFallback, "") + `<div class="image-meta">` + caption + `</div></a></div>`
	case "sites":
		cover := v2SafeURL(v2First(item, "cover", "snapshot", "screenshot"))
		if cover == "" {
			cover = "https://image.thum.io/get/width/1280/crop/720/" + u
		}
		api := ""
		if icon == "" {
			api = strings.Replace(r.endpoint("siteInfo"), "{href}", u, 1)
			icon = linkFallback
		}
		description := v2First(item, "description", "url")
		return `<div class="grid-cell site-card"><a class="card-link"` + v2Attr("data-siteinfo-api", api) + ` target="_blank" rel="external nofollow noopener noreferrer"` + v2Attr("href", u) + `><div class="lazy-box snapshot">` + v2LazyImage(cover, coverFallback, "") + `</div><div class="info"><div class="lazy-box icon">` + v2LazyImage(icon, icon, " siteinfo-icon") + `</div><span class="title">` + html.EscapeString(title) + `</span><span class="desc">` + html.EscapeString(description) + `</span></div></a></div>`
	}
	return ""
}

var v2TimelineAttribute = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

func (r *shortcodeV2Renderer) timeline(raw, body string) string {
	a := map[string]string{}
	for _, token := range splitArgs(raw) {
		key, value, ok := strings.Cut(token, ":")
		if ok {
			a[key] = value
		}
	}
	kind := a["type"]
	if kind == "" {
		kind = "timeline"
	}
	kind = safeClassWord(kind)
	class, attrs := "", ""
	if api := v2SafeURL(a["api"]); api != "" && kind != "" {
		class = " data-service ds-" + kind
		a["data-api"] = api
		delete(a, "api")
		var keys []string
		for k := range a {
			if k != "type" && v2TimelineAttribute.MatchString(k) && !strings.HasPrefix(strings.ToLower(k), "on") && k != "style" && k != "srcdoc" && k != "class" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		attrs = v2Attrs(a, keys...)
	}
	var b strings.Builder
	b.WriteString(`<div class="tag-plugin timeline` + class + `"` + attrs + `>`)
	for i, p := range v2Pairs(reNodeSplit, body) {
		head := ""
		if p.header != "" {
			head = `<span>` + v2Inline(r.markdown(p.header)) + `</span>`
		}
		fmt.Fprintf(&b, `<div class="timenode" index="%d"><div class="header">%s</div><div class="body fs14">%s</div></div>`, i, head, v2Compact(r.markdown(p.body)))
	}
	b.WriteString(`</div>`)
	return b.String()
}
func (r *shortcodeV2Renderer) toc(a map[string]string) string {
	project := v2Object(v2Object(v2Object(r.data["wiki"])["tree"])[a["wiki"]])
	if a["wiki"] != "" && project == nil {
		return ""
	}
	style := ""
	if a["display"] == "mobile" {
		style = ` style="display:none"`
	}
	open := " open"
	if a["open"] == "false" {
		open = ""
	}
	title := a["title"]
	if title == "" {
		title = "TOC"
	}
	el := `<div class="tag-plugin toc"` + v2Attrs(a, "display") + style + `><details class="toc"` + open + `><summary>` + html.EscapeString(title) + `</summary>`
	tree := func(pages any) string {
		var b strings.Builder
		b.WriteString(`<ul class="toc">`)
		for _, value := range v2List(pages) {
			page := v2Object(value)
			path := v2SafeURL(v2String(page["path"]))
			b.WriteString(`<li><a class="list-link"` + v2Attr("href", path) + `><span>` + html.EscapeString(v2String(page["title"])) + `</span></a></li>`)
		}
		b.WriteString(`</ul>`)
		return b.String()
	}
	if project != nil {
		sections := v2List(project["sections"])
		if len(sections) > 1 {
			el += `<div class="body fs14 multi">`
			for _, value := range sections {
				section := v2Object(value)
				el += `<section><div class="header">` + html.EscapeString(v2String(section["title"])) + `</div>` + tree(section["pages"]) + `</section>`
			}
			el += `</div>`
		} else {
			el += `<div class="body fs14"><div class="body">` + tree(project["pages"]) + `</div></div>`
		}
	}
	return el + `</details></div>`
}
