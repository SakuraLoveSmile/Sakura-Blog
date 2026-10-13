package post

import (
	"html"
	"net/url"
	"regexp"
	"strings"
)

func (r *shortcodeV2Renderer) navbar(a map[string]string) string {
	if a["links"] == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div class="tag-plugin navbar">`)
	for _, link := range strings.Split(a["links"], " ") {
		m := reNavbarLink.FindStringSubmatch(link)
		href, text := "#"+link, link
		if len(m) > 2 {
			href = v2SafeURL(m[2])
			text = m[1]
		}
		if href == "" {
			continue
		}
		cl := "link"
		if href == a["active"] {
			cl += " active"
		}
		b.WriteString(`<a class="` + cl + `"` + v2Attr("href", href) + `>` + html.EscapeString(text) + `</a>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}
func (r *shortcodeV2Renderer) banner(a map[string]string, body string) string {
	bg := a["bg"]
	cl := ""
	style := ""
	if bg == "" {
		cl = " no-bg"
	} else {
		style = v2Attr("style", "--bg-url:url('"+strings.ReplaceAll(bg, "'", "%27")+"')")
	}
	var b strings.Builder
	b.WriteString(`<div class="tag-plugin banner` + cl + `"` + style + `>`)
	adaptive := ""
	if bg != "" {
		b.WriteString(`<img class="lazy bg"` + v2Attr("data-src", bg) + `>`)
		adaptive = ` data-text-adaptive="split"`
	}
	b.WriteString(`<div class="content"` + adaptive + `><div class="top">`)
	if a["link"] != "" {
		b.WriteString(`<div></div>`)
	} else {
		b.WriteString(`<button class="back cap" onclick="window.history.back()">` + r.icon("default:arrow-left", "") + `</button>`)
	}
	expanded := r.expand(body)
	for _, row := range strings.Split(expanded, "\n") {
		if strings.Contains(row, "tag-plugin navbar") {
			b.WriteString(row)
			break
		}
	}
	b.WriteString(`</div><div class="bottom">`)
	if a["avatar"] != "" {
		b.WriteString(`<img class="avatar"` + v2Attr("src", a["avatar"]) + `>`)
	}
	if a["title"] != "" || a["subtitle"] != "" {
		b.WriteString(`<div class="text-area">`)
		for _, k := range []string{"title", "subtitle"} {
			if a[k] != "" {
				b.WriteString(`<div class="text ` + k + `">` + html.EscapeString(a[k]) + `</div>`)
			}
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`</div></div>`)
	if a["link"] != "" {
		b.WriteString(`<a class="banner-link"` + v2Attr("href", a["link"]) + `></a>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}
func (r *shortcodeV2Renderer) frame(a map[string]string) string {
	if a["device"] == "" || (a["img"] == "" && a["video"] == "") {
		return ""
	}
	kind := "img"
	if a["video"] != "" {
		kind = "video"
	}
	el := `<div class="tag-plugin ` + kind + `-wrap"><div class="frame-wrap"` + v2Attr("id", a["device"]) + v2Attr("focus", a["focus"]) + `>`
	if kind == "video" {
		el += `<video` + v2Attr("poster", a["img"]) + ` playsinline="" muted="" loop="" autoplay="" preload="metadata"><source` + v2Attr("src", a["video"]) + ` type="video/mp4"></video>`
	} else {
		el += `<img class="img"` + v2Attr("src", a["img"]) + v2Attr("alt", a["alt"]) + `/>`
	}
	el += `<div class="frame"></div></div>`
	if kind == "img" && a["alt"] != "" {
		el += `<span class="image-caption">` + html.EscapeString(a["alt"]) + `</span>`
	}
	return el + `</div>`
}

const v2LoadingImage = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAABGdBTUEAALGPC/xhBQAAADhlWElmTU0AKgAAAAgAAYdpAAQAAAABAAAAGgAAAAAAAqACAAQAAAABAAAAAaADAAQAAAABAAAAAQAAAADa6r/EAAAAC0lEQVQIHWNgAAIAAAUAAY27m/MAAAAASUVORK5CYII="

func (r *shortcodeV2Renderer) image(a map[string]string) string {
	style := ""
	if a["width"] != "" {
		style += "width:" + a["width"] + ";"
	}
	if a["height"] != "" {
		style += "height:" + a["height"] + ";"
	}
	fancy, fancyHref := true, ""
	if a["fancybox"] == "false" {
		fancy = false
	} else if a["fancybox"] != "" && a["fancybox"] != "true" {
		fancyHref = v2SafeURL(a["fancybox"])
	}
	img := `<img class="lazy"` + v2Attr("src", v2LoadingImage) + v2Attr("data-src", a["src"]) + v2Attr("alt", a["alt"])
	if fancy && fancyHref == "" {
		img += ` data-fancybox="true"`
	}
	if a["ratio"] == "" && style != "" {
		img += v2Attr("style", style)
	}
	img += `/><div class="lazy-icon"></div>`
	if fancyHref != "" {
		img = `<a data-fancybox` + v2Attr("data-caption", a["alt"]) + v2Attr("href", fancyHref) + `>` + img + `</a>`
	}
	bgStyle := ""
	if bg := v2CSSValue(a["bg"]); bg != "" {
		bgStyle += "background:" + bg + ";"
	}
	if a["padding"] != "" {
		bgStyle += "padding:" + a["padding"] + ";"
	}
	if a["ratio"] != "" {
		bgStyle += "aspect-ratio:" + a["ratio"] + ";" + style
	} else if style != "" {
		bgStyle += "width:100%;"
	}
	el := `<div class="tag-plugin image"><div class="image-bg"` + v2Attr("style", bgStyle) + `>` + img
	if download := a["download"]; download != "" {
		if download == "true" {
			download = a["src"]
		}
		download = v2SafeURL(download)
		if download != "" {
			el += `<a class="image-download blur" style="opacity:0" target="_blank"` + v2Attr("download", a["alt"]) + v2Attr("href", download) + `>` + r.icon("image:download", "") + `</a>`
		}
	}
	el += `</div>`
	if a["alt"] != "" {
		el += `<div class="image-meta"><span class="image-caption center">` + html.EscapeString(a["alt"]) + `</span></div>`
	}
	return el + `</div>`
}
func (r *shortcodeV2Renderer) link(a map[string]string) string {
	if a["url"] == "" {
		return ""
	}
	absolute := r.fullURL(a["url"])
	api := strings.Replace(r.endpoint("siteInfo"), "{href}", absolute, 1)
	var autofill []string
	if a["title"] == "" {
		autofill = append(autofill, "title")
	}
	if a["icon"] == "" {
		autofill = append(autofill, "icon")
	}
	if a["desc"] == "" {
		autofill = append(autofill, "desc")
	}
	kind := "plain"
	if a["desc"] != "" {
		kind = "rich"
	}
	el := `<div class="tag-plugin link dis-select"><a class="link-card ` + kind + ` card-hover card-hover--spotlight card-hover--tilt" title="` + html.EscapeString(a["title"]) + `"` + v2Attr("href", a["url"])
	if strings.Contains(a["url"], "://") {
		el += ` target="_blank" rel="external nofollow noopener noreferrer"`
	}
	el += ` cardlink` + v2Attr("data-api", api) + ` autofill="` + strings.Join(autofill, ",") + `">`
	icon := v2SafeURL(a["icon"])
	if icon == "" {
		icon = v2SafeURL(v2String(r.fallbacks["linkCard"]))
	}
	image := `<div class="lazy img"` + v2Attr("data-bg", icon) + `></div>`
	title := a["title"]
	if title == "" {
		title = a["url"]
	}
	titleHTML := `<span class="title">` + html.EscapeString(title) + `</span>`
	linkHTML := `<span class="cap link footnote">` + html.EscapeString(absolute) + `</span>`
	if kind == "rich" {
		el += `<div class="top">` + image + linkHTML + `</div><div class="bottom">` + titleHTML + `<span class="cap desc footnote">` + html.EscapeString(a["desc"]) + `</span></div>`
	} else {
		el += `<div class="left">` + titleHTML + linkHTML + `</div><div class="right">` + image + `</div>`
	}
	return el + `</a></div>`
}
func (r *shortcodeV2Renderer) fullURL(path string) string {
	if strings.Contains(path, "://") {
		return path
	}
	base := v2String(r.page["url"])
	if base == "" {
		base = v2String(r.data["siteURL"])
	}
	if u, e := url.Parse(base); e == nil && u.IsAbs() {
		if p, e := url.Parse(path); e == nil {
			return u.ResolveReference(p).String()
		}
	}
	return path
}
func (r *shortcodeV2Renderer) ghcard(a map[string]string) string {
	repo := a["repo"]
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)?$`).MatchString(repo) {
		return ""
	}
	base := strings.TrimRight(r.endpoint("githubCard"), "/")
	if base == "" {
		return ""
	}
	var image string
	if owner, name, ok := strings.Cut(repo, "/"); ok {
		image = base + "/api/pin/?username=" + url.QueryEscape(owner) + "&repo=" + url.QueryEscape(name)
	} else {
		image = base + "/api/?username=" + url.QueryEscape(repo)
	}
	params := []string{"show_owner", "theme", "title_color", "text_color", "icon_color", "bg_color", "hide_border", "cache_seconds", "locale"}
	image += "&"
	var pairs []string
	for _, p := range params {
		if a[p] != "" {
			pairs = append(pairs, p+"="+url.QueryEscape(a[p]))
		}
	}
	image += strings.Join(pairs, "&")
	if a["show_owner"] == "" {
		image += "&show_owner=true"
	}
	return `<div class="tag-plugin ghcard"><a class="ghcard" rel="external nofollow noopener noreferrer"` + v2Attr("href", "https://github.com/"+repo) + `><img` + v2Attr("src", image) + `/></a></div>`
}
func (r *shortcodeV2Renderer) reaction(name string, a map[string]string) string {
	endpoint := r.endpoint(name)
	disabled := ""
	cl := ""
	if endpoint == "" {
		cl = " is-disabled"
		disabled = " disabled"
	}
	id := a["id"]
	if id == "" {
		id = "default"
	}
	var b strings.Builder
	b.WriteString(`<div class="tag-plugin ds-` + name + cl + `"` + v2Attr("data-api", endpoint) + v2Attr("data-id", id) + `>`)
	if a["title"] != "" {
		b.WriteString(`<div class="header"><span>` + html.EscapeString(a["title"]) + `</span></div>`)
	}
	b.WriteString(`<div class="body">`)
	if name == "rating" {
		key := a["icon"]
		if key == "" {
			key = "rating:star"
		}
		for _, value := range []string{"1", "2", "3", "4", "5"} {
			b.WriteString(`<button class="star" data-value="` + value + `"` + disabled + `>` + r.icon(key, "") + `</button>`)
		}
		b.WriteString(`<span class="avg">(0.0)</span></div><div class="footer"><span class="count">0</span><span class="suffix">人已评分</span></div>`)
	} else {
		yes, no := a["yes"], a["no"]
		if yes == "" {
			yes = "vote:thumbsup"
		}
		if no == "" {
			no = "vote:thumbsdown"
		}
		b.WriteString(`<button class="vote-up"` + disabled + `>` + r.icon(yes, "") + ` <span class="up">0</span></button><button class="vote-down"` + disabled + `>` + r.icon(no, "") + ` <span class="down">0</span></button></div>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}
func (r *shortcodeV2Renderer) remoteMD(a map[string]string) string {
	src := a["src"]
	base := ""
	rawURL := v2String(v2Object(r.services["github"])["rawUrl"])
	configured, _ := url.Parse(rawURL)
	u, e := url.Parse(src)
	if e == nil && u.IsAbs() && (u.Hostname() == "raw.githubusercontent.com" || (configured != nil && u.Scheme == configured.Scheme && u.Host == configured.Host)) {
		segs := strings.FieldsFunc(u.EscapedPath(), func(ch rune) bool { return ch == '/' })
		if len(segs) >= 3 {
			origin := u.Scheme + "://" + u.Host
			if configured != nil && configured.IsAbs() {
				origin = configured.Scheme + "://" + configured.Host
			}
			dir := strings.Join(segs[:3], "/") + "/"
			base = origin + "/" + dir
			src = base + strings.Join(segs[3:], "/")
		}
	}
	replace := ""
	if a["wrap"] == "false" {
		replace = ` data-replace="true"`
	}
	return `<div class="data-service ds-mdrender"` + v2Attr("src", src) + v2Attr("data-base", base) + replace + ` data-heading="true"` + v2Attr("id", r.next("md")) + `></div>`
}

func (r *shortcodeV2Renderer) video(a map[string]string) string {
	width, hasWidth := a["width"]
	if !hasWidth {
		width = "100%"
	}
	ratio := a["ratio"]
	if ratio == "" {
		ratio = "1.7777777777777777"
	}
	wrapper := `<div class="tag-plugin video-player"` + v2Attr("style", "aspect-ratio:"+ratio+";max-width:"+width+";") + `>`
	if a["bilibili"] != "" {
		auto := a["autoplay"]
		if auto == "" {
			auto = "false"
		}
		src := "https://player.bilibili.com/player.html?bvid=" + url.QueryEscape(a["bilibili"]) + "&autoplay=" + url.QueryEscape(auto)
		return wrapper + `<iframe` + v2Attr("src", src) + ` scrolling="no" border="0" frameborder="no" framespacing="0" allowfullscreen="true"></iframe></div>`
	}
	if a["youtube"] != "" {
		auto := "0"
		if a["autoplay"] == "true" || a["autoplay"] == "1" {
			auto = "1&mute=1"
		}
		src := "https://www.youtube.com/embed/" + url.PathEscape(a["youtube"]) + "?rel=0&disablekb=1&playsinline=1&autoplay=" + auto
		return wrapper + `<iframe style="border:none"` + v2Attr("src", src) + ` picture-in-picture="true" allowfullscreen="true"></iframe></div>`
	}
	mime := a["type"]
	if mime == "" {
		mime = "video/mp4"
	}
	return `<div class="tag-plugin video-player"` + v2Attr("style", "max-width:"+width+";") + `><video controls preload playsinline webkit-playsinline><source src="` + html.EscapeString(a["src"]) + `"` + v2Attr("type", mime) + `>Your browser does not support the video tag.</video></div>`
}
func (r *shortcodeV2Renderer) audio(a map[string]string) string {
	mime := a["type"]
	if a["netease"] != "" {
		if mime == "" {
			mime = "2"
		}
		auto := "0"
		if a["autoplay"] == "true" {
			auto = "1"
		}
		src := "//music.163.com/outchain/player?type=" + url.QueryEscape(mime) + "&id=" + url.QueryEscape(a["netease"]) + "&auto=" + auto + "&height=32"
		return `<div class="tag-plugin audio"><iframe` + v2Attr("src", src) + ` frameborder="no" border="0" marginwidth="0" marginheight="0" width="288px" height="52"></iframe></div>`
	}
	if mime == "" {
		mime = "audio/mp3"
	}
	return `<div class="tag-plugin audio"><audio controls preload><source src="` + html.EscapeString(a["src"]) + `"` + v2Attr("type", mime) + `>Your browser does not support the audio tag.</audio></div>`
}
