package post

import (
	"blog/internal/settings"
	"fmt"
	"html"
	"strings"
	"sync/atomic"
)

// ---------- Spec 16: 标签插件二期（link/image/folders/grid/box/mark/emoji/icon/hashtag） ----------

// renderLink 渲染 {% link url [title] [desc:...] [icon:...] %} 为 .tag-plugin.link 卡片。
func renderLink(argsStr string) (string, bool) {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"icon", "desc"}, []string{"url", "title"})
	url := named["url"]
	if url == "" {
		return "", false
	}
	// 安全审计 2026-08-16：url 用户可控，校验 scheme（拒 javascript:/data:/vbscript: 等），非法不渲染
	if safeURL(url) == "" {
		return "", false
	}
	title := named["title"]
	if title == "" {
		title = url
	}
	desc := named["desc"]
	icon := named["icon"]

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin link dis-select">`)
	if desc != "" && desc != "false" {
		// rich 模式（上下结构）
		sb.WriteString(`<a class="link-card rich" title="` + html.EscapeString(title) + `" href="` + html.EscapeString(url) + `"`)
		if strings.Contains(url, "://") {
			sb.WriteString(` target="_blank" rel="external nofollow noopener noreferrer"`)
		}
		sb.WriteString(`>`)
		sb.WriteString(`<div class="top">`)
		sb.WriteString(`<div class="lazy img" data-bg="` + html.EscapeString(icon) + `"></div>`)
		sb.WriteString(`<span class="cap link footnote">` + html.EscapeString(url) + `</span>`)
		sb.WriteString(`</div>`)
		sb.WriteString(`<div class="bottom">`)
		sb.WriteString(`<span class="title">` + html.EscapeString(title) + `</span>`)
		sb.WriteString(`<span class="cap desc footnote">` + html.EscapeString(desc) + `</span>`)
		sb.WriteString(`</div>`)
	} else {
		// plain 模式（左右结构）
		sb.WriteString(`<a class="link-card plain" title="` + html.EscapeString(title) + `" href="` + html.EscapeString(url) + `"`)
		if strings.Contains(url, "://") {
			sb.WriteString(` target="_blank" rel="external nofollow noopener noreferrer"`)
		}
		sb.WriteString(`>`)
		sb.WriteString(`<div class="left">`)
		sb.WriteString(`<span class="title">` + html.EscapeString(title) + `</span>`)
		sb.WriteString(`<span class="cap link footnote">` + html.EscapeString(url) + `</span>`)
		sb.WriteString(`</div>`)
		sb.WriteString(`<div class="right">`)
		if icon != "" {
			sb.WriteString(`<div class="lazy img" data-bg="` + html.EscapeString(icon) + `"></div>`)
		} else {
			sb.WriteString(`<div class="lazy img" data-bg=""></div>`)
		}
		sb.WriteString(`</div>`)
	}
	sb.WriteString(`</a></div>`)
	return sb.String(), true
}

// renderImage 渲染 {% image src [alt] [width:...] [bg:...] [download:...] [fancybox:...] [ratio] %}。
func renderImage(argsStr string) (string, bool) {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"width", "height", "bg", "download", "padding", "fancybox", "ratio"}, []string{"src", "alt"})
	src := named["src"]
	if src == "" {
		return "", false
	}
	alt := named["alt"]

	// 安全审计 2026-08-16：width/height 走 CSS 数值白名单，非法值丢弃，防 style 注入
	var style strings.Builder
	if w := safeCSSNum(named["width"]); w != "" {
		style.WriteString("width:" + w + ";")
	}
	if h := safeCSSNum(named["height"]); h != "" {
		style.WriteString("height:" + h + ";")
	}

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin image">`)
	// image-bg
	sb.WriteString(`<div class="image-bg"`)
	bgStyle := ""
	// 安全审计 2026-08-16：bg 走颜色白名单、padding/ratio 走数值白名单，非法值丢弃
	if bg := safeColor(named["bg"]); bg != "" {
		bgStyle += "background:" + bg + ";"
	}
	if p := safeCSSNum(named["padding"]); p != "" {
		bgStyle += "padding:" + p + ";"
	}
	if r := safeCSSRatio(named["ratio"]); r != "" {
		bgStyle += "aspect-ratio:" + r + ";"
		if style.Len() > 0 {
			bgStyle += style.String()
		}
	} else if style.Len() > 0 {
		bgStyle += "width:100%;"
	}
	if bgStyle != "" {
		sb.WriteString(` style="` + bgStyle + `"`)
	}
	sb.WriteString(`>`)
	// img
	sb.WriteString(`<img class="lazy" src="` + html.EscapeString(src) + `" data-src="` + html.EscapeString(src) + `"`)
	if alt != "" {
		sb.WriteString(` alt="` + html.EscapeString(alt) + `"`)
	}
	if named["ratio"] == "" && style.Len() > 0 {
		sb.WriteString(` style="` + style.String() + `"`)
	}
	sb.WriteString(`/>`)
	// download
	if dl := named["download"]; dl != "" && dl != "false" {
		href := dl
		if dl == "true" {
			href = src
		}
		// 安全审计 2026-08-16：download 链接校验 scheme，非法值不渲染下载按钮
		if safeURL(href) != "" {
			sb.WriteString(`<a class="image-download blur" style="opacity:0" target="_blank"`)
			if alt != "" {
				sb.WriteString(` download="` + html.EscapeString(alt) + `"`)
			}
			sb.WriteString(` href="` + html.EscapeString(href) + `"><svg class="icon" style="width: 1em; height: 1em;vertical-align: middle;fill: currentColor;overflow: hidden;" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg"><path d="M561.00682908 685.55838913a111.03077546 111.03077546 0 0 1-106.8895062 0L256.23182837 487.72885783a55.96309219 55.96309219 0 0 1 79.13181253-79.18777574L450.70357448 523.88101491V181.55477937a55.96309219 55.96309219 0 0 1 111.92618438 0v344.06109173l117.07478902-117.07478901a55.96309219 55.96309219 0 0 1 79.13181252 79.18777574zM282.81429711 797.1487951h447.70473912a55.96309219 55.96309219 0 0 1 0 111.92618438H282.81429711a55.96309219 55.96309219 0 0 1 0-111.92618438z"></path></svg></a>`)
		}
	}
	sb.WriteString(`</div>`)
	// caption
	if alt != "" {
		sb.WriteString(`<div class="image-meta"><span class="image-caption center">` + html.EscapeString(alt) + `</span></div>`)
	}
	sb.WriteString(`</div>`)
	return sb.String(), true
}

// renderFolders 渲染 {% folders [color] %}...{% endfolders %}。
func renderFolders(argsStr, body string, renderMD func(string) string) string {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"color"}, nil)

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin colorful folders"`)
	// 安全审计 2026-08-16：color 用户可控，白名单后再拼入
	if c := safeColor(named["color"]); c != "" {
		fmt.Fprintf(&sb, ` color="%s"`, html.EscapeString(c))
	}
	sb.WriteString(`>`)

	pairs := splitByMarker(reFolderSplit, body)
	for i, pair := range pairs {
		fmt.Fprintf(&sb, `<details class="folder" index="%d">`, i)
		fmt.Fprintf(&sb, `<summary>%s</summary>`, strings.TrimSpace(renderMD(pair.header)))
		rendered := strings.TrimSpace(renderMD(pair.body))
		fmt.Fprintf(&sb, `<div class="body">%s</div></details>`, rendered)
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

// renderGrid 渲染 {% grid [bg:...] [w:...] [c:...] [gap:...] [br:...] %}...{% endgrid %}。
func renderGrid(argsStr, body string, renderMD func(string) string) string {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"bg", "w", "c", "gap", "br"}, nil)
	// 安全审计 2026-08-16：w/c/gap/br 走 CSS 数值白名单、bg 走颜色白名单，非法值丢弃
	w := safeCSSNum(named["w"])
	c := safeCSSNum(named["c"])
	if w == "" && c == "" {
		w = "240px"
	}

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin grid"`)
	if bg := safeColor(named["bg"]); bg != "" {
		fmt.Fprintf(&sb, ` bg="%s"`, html.EscapeString(bg))
	}
	sb.WriteString(` style="`)
	if w != "" {
		fmt.Fprintf(&sb, `grid-template-columns: repeat(auto-fill, minmax(%s, 1fr));`, w)
	} else if c != "" {
		fmt.Fprintf(&sb, `grid-template-columns: repeat(%s, 1fr);`, c)
	}
	if gap := safeCSSNum(named["gap"]); gap != "" {
		fmt.Fprintf(&sb, `grid-gap:%s;`, gap)
	}
	sb.WriteString(`">`)

	// 切分 cells
	cells := reCellSplit.Split(body, -1)
	for _, cell := range cells {
		cell = strings.TrimSpace(cell)
		if cell == "" {
			continue
		}
		sb.WriteString(`<div class="cell"`)
		if br := safeCSSNum(named["br"]); br != "" {
			fmt.Fprintf(&sb, ` style="border-radius:%s;"`, br)
		}
		sb.WriteString(`>`)
		sb.WriteString(strings.TrimSpace(renderMD(cell)))
		sb.WriteString(`</div>`)
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

// renderBox 渲染 {% box [color:...] [child:...] title %} body {% endbox %}。
func renderBox(argsStr, body string, renderMD func(string) string) string {
	named, positional := parseNamedPositionalArgs(argsStr, []string{"color", "child"}, []string{"title"})
	title := named["title"]
	if title == "" && len(positional) > 0 {
		title = strings.Join(positional, " ")
	}

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin colorful note"`)
	// 安全审计 2026-08-16：color/child 用户可控，白名单/转义后再拼入
	if c := safeColor(named["color"]); c != "" {
		fmt.Fprintf(&sb, ` color="%s"`, html.EscapeString(c))
	}
	if ch := named["child"]; ch != "" {
		fmt.Fprintf(&sb, ` child="%s"`, html.EscapeString(ch))
	}
	sb.WriteString(`>`)
	if title != "" {
		fmt.Fprintf(&sb, `<div class="title"><strong>%s</strong></div>`, strings.TrimSpace(renderMD(title)))
	}
	rendered := strings.TrimSpace(renderMD(body))
	fmt.Fprintf(&sb, `<div class="body">%s</div></div>`, rendered)
	return sb.String()
}

// renderMark 渲染 {% mark text [color:...] %}。
func renderMark(argsStr string, tagPlugins settings.TagPluginsConfig) (string, bool) {
	named, positional := parseNamedPositionalArgs(argsStr, []string{"color"}, []string{"text"})
	text := named["text"]
	if text == "" {
		text = strings.Join(positional, " ")
	}
	if text == "" {
		return "", false
	}
	color := named["color"]
	if color == "" {
		color = tagPlugins.Mark.DefaultColor
	}
	if color == "" {
		color = "yellow"
	}
	// 安全审计 2026-08-16：color 白名单（非法丢弃该属性）、text 转义后再输出
	colorAttr := ""
	if c := safeColor(color); c != "" {
		colorAttr = fmt.Sprintf(` color="%s"`, html.EscapeString(c))
	}
	return fmt.Sprintf(`<mark class="tag-plugin colorful mark"%s>%s</mark>`, colorAttr, html.EscapeString(text)), true
}

// emojiSources 内置 emoji CDN 映射，对齐原版 _config.yml tag_plugins.emoji。
var emojiSources = map[string]string{
	"default": "https://cdn.jsdelivr.net/gh/cdn-x/emoticons@3.1.0/qq/{name}.gif",
	"qq":      "https://cdn.jsdelivr.net/gh/cdn-x/emoticons@3.1.0/qq/{name}.gif",
	"aru":     "https://cdn.jsdelivr.net/gh/cdn-x/emoticons@3.1.0/aru/{name}.gif",
	"tieba":   "https://cdn.jsdelivr.net/gh/cdn-x/emoticons@3.1.0/tieba/{name}.png",
	"blobcat": "https://cdn.jsdelivr.net/gh/cdn-x/emoticons@3.1.0/blobcat/{name}.gif",
	"twemoji": "https://cdn.jsdelivr.net/gh/twitter/twemoji@14.0.2/assets/svg/{name}.svg",
}

// renderEmoji 渲染 {% emoji [source] name [height:...] %} 或
// {% emoji url:https://... [name:alt] [height:...] %}。
// url: 分支对齐原版 scripts/tags/lib/emoji.js：直接引用外部图片，不再走 source 配置查找。
func renderEmoji(argsStr string, tagPlugins settings.TagPluginsConfig) (string, bool) {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"url", "height", "name"}, []string{"source", "name"})
	source := named["source"]
	name := named["name"]
	height := named["height"]

	// url: 优先分支：url 非空时直接以 url 作图片地址，不查 emojiSources
	if url := named["url"]; url != "" {
		var sb strings.Builder
		sb.WriteString(`<span class="tag-plugin emoji">`)
		sb.WriteString(`<img no-lazy="" class="inline" src="` + html.EscapeString(url) + `"`)
		if name != "" {
			sb.WriteString(` alt="` + html.EscapeString(name) + `"`)
		}
		if height != "" && reEmojiHeight.MatchString(height) {
			sb.WriteString(` style="height:` + html.EscapeString(height) + `"`)
		}
		sb.WriteString(`/>`)
		sb.WriteString(`</span>`)
		return sb.String(), true
	}

	if source == "" {
		return "", false
	}
	// 省略 source 时，第一个位置参数实际是 name，用默认源
	if name == "" {
		name = source
		source = "default"
	}
	tpl, ok := emojiSources[source]
	// tag_plugins.emoji：配置源优先于内置默认源
	if cfgURL := tagPlugins.Emoji[source]; cfgURL != "" {
		tpl, ok = cfgURL, true
	}
	if !ok {
		return "", false
	}
	url := strings.Replace(tpl, "{name}", name, 1)

	var sb strings.Builder
	sb.WriteString(`<span class="tag-plugin emoji">`)
	sb.WriteString(`<img no-lazy="" class="inline" src="` + html.EscapeString(url) + `"`)
	if height != "" && reEmojiHeight.MatchString(height) {
		sb.WriteString(` style="height:` + html.EscapeString(height) + `"`)
	}
	sb.WriteString(`/>`)
	sb.WriteString(`</span>`)
	return sb.String(), true
}

// renderIcon 渲染 {% icon key [color:...] [style:...] %}。
func renderIcon(argsStr string, iconFunc func(string) string, tagPlugins settings.TagPluginsConfig) (string, bool) {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"color", "style"}, []string{"key", "text"})
	key := named["key"]
	if key == "" {
		return "", false
	}
	color := named["color"]
	if color == "" {
		color = tagPlugins.Icon.DefaultColor
	}
	if color == "" {
		color = "blue"
	}
	text := named["text"]

	var sb strings.Builder
	if text != "" {
		sb.WriteString(`<div class="tag-plugin icon-wrap">`)
	}
	// 安全审计 2026-08-16：color 用户可控，白名单后再拼入
	fmt.Fprintf(&sb, `<span class="tag-plugin icon colorful" color="%s">`, html.EscapeString(safeColor(color)))
	// 安全审计 2026-08-16：key 用户可控，格式白名单后再进入 iconFunc（非法降级空图标）
	if iconFunc != nil {
		if k := safeIconKey(key); k != "" {
			sb.WriteString(iconFunc(k))
		}
	}
	sb.WriteString(`</span>`)
	if text != "" {
		// 安全审计 2026-08-16：text 用户可控，转义后再输出
		fmt.Fprintf(&sb, `<span class="text">%s</span></div>`, html.EscapeString(text))
	}
	return sb.String(), true
}

// hashtagColors 话题标签循环色板，对齐原版 hashtag.js。
var hashtagColors = []string{"red", "orange", "yellow", "green", "cyan", "blue", "purple"}

// hashtagSVG 话题标签图标，对齐原版 hashtag.js 内联 SVG。
const hashtagSVG = `<svg t="1701408144765" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" width="200" height="200"><path d="M426.6 64.8c34.8 5.8 58.4 38.8 52.6 73.6l-19.6 117.6h190.2l23-138.6c5.8-34.8 38.8-58.4 73.6-52.6s58.4 38.8 52.6 73.6l-19.4 117.6H896c35.4 0 64 28.6 64 64s-28.6 64-64 64h-137.8l-42.6 256H832c35.4 0 64 28.6 64 64s-28.6 64-64 64h-137.8l-23 138.6c-5.8 34.8-38.8 58.4-73.6 52.6s-58.4-38.8-52.6-73.6l19.6-117.4h-190.4l-23 138.6c-5.8 34.8-38.8 58.4-73.6 52.6s-58.4-38.8-52.6-73.6l19.4-117.8H128c-35.4 0-64-28.6-64-64s28.6-64 64-64h137.8l42.6-256H192c-35.4 0-64-28.6-64-64s28.6-64 64-64h137.8l23-138.6c5.8-34.8 38.8-58.4 73.6-52.6z m11.6 319.2l-42.6 256h190.2l42.6-256h-190.2z"></path></svg>`

// renderHashtag 渲染 {% hashtag text [href] [color:...] %}。
func renderHashtag(argsStr string, tagPlugins settings.TagPluginsConfig) (string, bool) {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"color"}, []string{"text", "href"})
	text := named["text"]
	if text == "" {
		return "", false
	}
	href := named["href"]
	color := named["color"]
	if color == "" {
		// tag_plugins.hashtag.default_color：非空时固定默认色，空=循环色板（现状）
		color = tagPlugins.Hashtag.DefaultColor
	}
	if color == "" {
		idx := atomic.AddUint64(&hashtagColorIdx, 1) - 1
		color = hashtagColors[idx%uint64(len(hashtagColors))]
	}

	var sb strings.Builder
	sb.WriteString(`<a class="tag-plugin colorful hashtag"`)
	// 安全审计 2026-08-16：color 白名单后再拼入
	fmt.Fprintf(&sb, ` color="%s"`, html.EscapeString(safeColor(color)))
	// 安全审计 2026-08-16：href 校验 scheme（拒 javascript: 等），非法丢弃 href；text 转义后再输出
	if href != "" {
		if u := safeURL(href); u != "" {
			fmt.Fprintf(&sb, ` href="%s"`, html.EscapeString(u))
		}
	}
	sb.WriteString(`>`)
	sb.WriteString(hashtagSVG)
	fmt.Fprintf(&sb, `<span>%s</span>`, html.EscapeString(text))
	sb.WriteString(`</a>`)
	return sb.String(), true
}
