package post

import (
	"fmt"
	"github.com/goccy/go-yaml"
	"html"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var v2ChatParam = regexp.MustCompile(`(?:^|\s)(user|tag|md|quote|image|emoji|source|voice|type|video|link|from|file):`)

func v2ChatParams(raw string) map[string]string {
	out := map[string]string{}
	locs := v2ChatParam.FindAllStringSubmatchIndex(raw, -1)
	for i, l := range locs {
		end := len(raw)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		out[raw[l[2]:l[3]]] = strings.TrimSpace(raw[l[1]:end])
	}
	return out
}

func (r *shortcodeV2Renderer) chat(a map[string]string, body string) string {
	prefix := body
	if i := strings.Index(body, "<!--"); i >= 0 {
		prefix = body[:i]
	}
	users := map[string]map[string]any{}
	if err := yaml.Unmarshal([]byte(prefix), &users); err != nil {
		return `<span class="stellar-tag-error" data-tag="chat">Invalid chat users YAML</span>`
	}
	type cell struct {
		kind, text string
		params     map[string]string
	}
	var cells []cell
	tagCells := map[string]map[string]string{}
	count := 0
	for _, m := range reChatComment.FindAllStringSubmatch(body, -1) {
		kind, text, ok := strings.Cut(strings.TrimSpace(m[1]), " ")
		if !ok {
			continue
		}
		if kind == "chattip" {
			cells = append(cells, cell{kind: kind, text: text})
		} else if kind == "chatcell" {
			p := v2ChatParams(text)
			cells = append(cells, cell{kind: kind, params: p})
			if p["tag"] != "" {
				tagCells[p["tag"]] = p
			}
			count++
		}
	}
	style := safeClassWord(a["style"])
	if style == "" {
		style = "qq"
	}
	cl := safeClassWord(a["device"])
	if cl != "" {
		cl = " " + cl
	}
	cl += " " + style
	if scene := safeClassWord(a["scene"]); scene != "" {
		cl += " " + scene
	}
	var b strings.Builder
	b.WriteString(`<div class="tag-plugin chat` + cl + `">`)
	dynamic := a["labelColorStyle"] == "" || a["labelColorStyle"] == "dynamic"
	colors := map[string]int{}
	chatID := strings.Replace(r.next("chat"), "_", "-", 1)
	if dynamic {
		var light, dark strings.Builder
		for _, c := range cells {
			if c.kind != "chatcell" {
				continue
			}
			key := c.params["user"]
			if _, ok := colors[key]; ok {
				continue
			}
			label := v2Object(users[key]["label"])
			if hue, ok := v2ColorHue(v2String(label["bgColor"])); ok {
				index := len(colors)
				colors[key] = index
				light.WriteString(fmt.Sprintf(`--%s-label-bg-%d:hsl(%gdeg,55%%,75%%);--%s-label-bg-2-%d:hsl(%gdeg,55%%,60%%);--%s-label-text-color-%d:hsl(%gdeg,50%%,40%%);`, chatID, index, hue, chatID, index, hue, chatID, index, hue))
				dark.WriteString(fmt.Sprintf(`--%s-label-bg-%d:hsl(%gdeg,40%%,26%%);--%s-label-bg-2-%d:hsl(%gdeg,40%%,40%%);--%s-label-text-color-%d:hsl(%gdeg,60%%,60%%);`, chatID, index, hue, chatID, index, hue, chatID, index, hue))
			}
		}
		if light.Len() > 0 {
			b.WriteString(`<style` + v2Attr("id", chatID) + `>:root{` + light.String() + `}@media(prefers-color-scheme:dark){:root:not([color-scheme]){` + dark.String() + `}}[color-scheme='dark']{` + dark.String() + `}</style>`)
		}
	}
	icons := func(keys ...string) string {
		var out strings.Builder
		for _, key := range keys {
			out.WriteString(r.icon("chat:"+key, ""))
		}
		return out.String()
	}
	if a["device"] != "" {
		b.WriteString(`<div class="device-image"></div><div class="status-bar"><div class="left-items"><span class="time">14:36</span></div><div class="right-items">` + icons("earphone", "bluetooth", "signal", "wifi", "battery") + `</div></div>`)
	}
	title := a["title"]
	if _, present := a["title"]; !present {
		title = "undefined"
	}
	more := "nav-more-qq"
	if style == "wechat" {
		more = "nav-more-wechat"
	}
	fmt.Fprintf(&b, `<div class="chat-navbar"><div class="left-items"><div class="back-btn">%s</div><div class="news-num">%d</div><div class="title">%s</div></div><div class="right-items"><div class="more-btn">%s</div></div></div><div class="content">`, icons("back"), count, html.EscapeString(title), icons(more))
	quote := func(p map[string]string, wechat bool) string {
		quoted := tagCells[p["quote"]]
		if quoted == nil {
			return ""
		}
		user := users[quoted["user"]]
		name := html.EscapeString(v2String(user["name"]))
		md, image := quoted["md"], v2SafeURL(quoted["image"])
		if md == "" && image == "" {
			return ""
		}
		attrs := v2Attr("quotedCellTag", p["quote"])
		if wechat {
			if md != "" {
				return `<div class="quote"` + attrs + `><div>` + name + `:</div><div>` + v2Compact(r.markdown(md)) + `</div></div>`
			}
			return `<div class="quote image-chat"` + attrs + `><span>` + name + `: </span><img lazy` + v2Attr("src", image) + `></div>`
		}
		inner := `<span>` + v2Compact(r.markdown(md)) + `</span>`
		if md == "" {
			inner = `<img lazy` + v2Attr("src", image) + `>`
		}
		return `<div class="quote"` + attrs + `><div class="quote-user"><span>` + name + `</span>` + icons("arrow-up") + `</div><div class="quote-content">` + inner + `</div></div>`
	}
	for _, c := range cells {
		if c.kind == "chattip" {
			b.WriteString(`<div class="chattip">` + html.EscapeString(c.text) + `</div>`)
			continue
		}
		p := c.params
		user := users[p["user"]]
		if user == nil {
			b.WriteString(`<div class="chatcell unsupported">Unknown chat user</div>`)
			continue
		}
		right := ""
		if a["me"] != "" && a["me"] == p["user"] {
			right = " right"
		}
		id := ""
		if p["tag"] != "" {
			id = v2Attr("id", "quote-"+p["tag"])
		}
		b.WriteString(`<div class="chatcell` + right + `"` + id + `><div class="user-avatar"><img lazy` + v2Attr("src", v2SafeURL(v2String(user["avatar"]))) + `></div><div class="user-main"><div class="header">`)
		label := v2Object(user["label"])
		if label != nil {
			labelStyle := ""
			labelClass := "hand-color"
			if dynamic {
				labelClass = "dynamic-color"
				if i, ok := colors[p["user"]]; ok {
					labelStyle = fmt.Sprintf("color:var(--%s-label-text-color-%d);background:var(--%s-label-bg-%d);", chatID, i, chatID, i)
				}
			} else {
				if color := safeColor(v2String(label["textColor"])); color != "" {
					labelStyle += "color:" + color + ";"
				}
				if color := safeColor(v2String(label["bgColor"])); color != "" {
					labelStyle += "background:" + color + ";"
				}
			}
			b.WriteString(`<span class="label ` + labelClass + `"` + v2Attr("style", labelStyle) + `>` + html.EscapeString(v2String(label["text"])) + `</span>`)
		}
		b.WriteString(`<span class="name">` + html.EscapeString(v2String(user["name"])) + `</span></div>`)
		switch {
		case p["md"] != "":
			b.WriteString(`<div class="talk md">`)
			md := p["md"]
			if p["quote"] != "" && style == "qq" {
				b.WriteString(quote(p, false))
				if quoted := tagCells[p["quote"]]; quoted != nil {
					md = "@" + v2String(users[quoted["user"]]["name"]) + " " + md
				}
			}
			b.WriteString(v2Compact(r.markdown(md)))
		case p["image"] != "":
			b.WriteString(`<div class="talk image"><img lazy fancybox="true"` + v2Attr("src", v2SafeURL(p["image"])) + `>`)
		case p["emoji"] != "":
			source, ok := p["source"]
			if !ok {
				source = r.setting("emoji", "defaultSource")
			}
			template := v2String(v2Object(v2Object(r.tags["emoji"])["sources"])[source])
			src := p["emoji"]
			if source != "" && template != "" {
				src = strings.Replace(template, "{name}", src, 1)
			}
			b.WriteString(`<div class="talk emoji"><img lazy` + v2Attr("src", v2SafeURL(src)) + `>`)
		case p["voice"] != "":
			b.WriteString(`<div class="talk voice">`)
			if style == "qq" {
				b.WriteString(`<div class="voice-btns"><div class="pause-btn" style="display:none;">` + icons("pause") + `</div><div class="play-btn" style="display:flex;">` + icons("play") + `</div></div><canvas class="voice-wave` + right + `"></canvas>`)
			} else if style == "wechat" {
				b.WriteString(`<div class="wechat-voice"><div class="voice-circle first"></div><div class="voice-circle second"></div><div class="voice-circle third"></div></div>`)
			}
			if style == "qq" || style == "wechat" {
				mime := p["type"]
				if mime == "" {
					mime = "audio/mp3"
				}
				b.WriteString(`<audio class="` + style + `" preload="auto"><source` + v2Attr("src", v2SafeURL(p["voice"])) + v2Attr("type", mime) + `>Your browser does not support the audio in chat tag.</audio><div class="voice-metas"><span class="voice-seconds"></span></div>`)
				if style == "wechat" {
					b.WriteString(`<div class="voice-placeholder"></div>`)
				}
			}
		case p["video"] != "":
			mime := p["type"]
			if mime == "" {
				mime = "video/mp4"
			}
			b.WriteString(`<div class="talk video"><div class="video-btns"><div class="pause-btn" style="display:none;">` + icons("pause") + `</div><div class="play-btn" style="display:flex;">` + icons("play") + `</div></div><video preload disablePictureInPicture><source` + v2Attr("src", v2SafeURL(p["video"])) + v2Attr("type", mime) + `>Your browser does not support the video tag.</video>`)
		case p["link"] != "":
			link := v2SafeURL(p["link"])
			from := p["from"]
			if from == "" {
				from = "QQ小程序"
			}
			external := ""
			if strings.Contains(link, "://") {
				external = ` target="_blank" rel="external nofollow noopener noreferrer"`
			}
			fromKeys := map[string]string{"Google浏览器": "google", "Safari浏览器": "safari", "IE浏览器": "ie", "UC浏览器": "uc", "QQ浏览器": "qq", "百度浏览器": "baidu", "Firefox浏览器": "firefox", "360浏览器": "360", "QQ小程序": "qq-mini"}
			fromIcon := ""
			if key := fromKeys[from]; key != "" {
				fromIcon = icons(key)
			}
			b.WriteString(`<div class="talk link"><a class="link-card rich"` + v2Attr("href", link) + external + v2Attr("data-api", "https://siteinfo.listentothewind.cn/api/v1?url="+link) + ` cardlink autofill="title,icon,desc"><div class="top"><span class="title">` + html.EscapeString(link) + `</span></div><div class="center"><div class="left"><span class="cap desc fs12"></span></div><div class="right"><div class="lazy img"` + v2Attr("data-bg", v2SafeURL(v2String(r.fallbacks["linkCard"]))) + `></div></div></div><div class="bottom">` + fromIcon + `<span class="cap from fs12">&nbsp;` + html.EscapeString(from) + `</span></div></a>`)
		case p["file"] != "":
			file := v2SafeURL(p["file"])
			b.WriteString(`<div class="talk chat-file"` + v2Attr("url", file) + `><div class="content"><div class="top"><div class="left"><span>` + html.EscapeString(file) + `</span></div><div class="right">` + icons("file-"+v2ChatFileType(file)) + `</div></div><div class="bottom"><span class="desc cap fs12 file-size"></span></div></div><div class="download-btn">` + icons("download") + `</div>`)
		default:
			b.WriteString(`<div class="talk">`)
		}
		b.WriteString(`</div>`)
		if p["md"] != "" && p["quote"] != "" && style == "wechat" {
			b.WriteString(quote(p, true))
		}
		b.WriteString(`</div></div>`)
	}
	b.WriteString(`</div>`)
	if style == "qq" {
		b.WriteString(`<div class="bottom"><div class="input"><div class="input-text"></div><div class="submit-btn">发送</div></div><div class="icons">` + icons("voice-qq", "photos", "camera", "red-envelope", "smile-qq", "more-qq") + `</div></div>`)
	} else if style == "wechat" {
		b.WriteString(`<div class="bottom">` + icons("voice-wechat") + `<div class="input-text"></div>` + icons("smile-wechat", "more-wechat") + `</div>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}
func v2ChatFileType(path string) string {
	extension := path[strings.LastIndex(path, ".")+1:]
	groups := map[string]string{"word": "doc docx", "ppt": "ppt pptx", "excel": "csv xls xlsx", "pdf": "pdf", "txt": "txt md", "archive": "tar rar zip Z gz bz2 xz 7z", "code": "java c cpp py h js html css sql php", "video": "mp4 mov avi flv asf wmv", "photo": "jpg png webp jpeg svg gif", "voice": "mp3 wma rm wav midi ape flac", "config": "ini conf config xml yaml toml json yml plist", "database": "mdb mdf myd db dbf wdb sqlite", "3d": "max obj stp stl dae fbx x3d", "exe": "exe bat app sh so cmd command", "link": "lnk"}
	for kind, values := range groups {
		for _, v := range strings.Fields(values) {
			if v == extension {
				return kind
			}
		}
	}
	return "unknown"
}
func v2ColorHue(color string) (float64, bool) {
	color = strings.TrimSpace(color)
	if strings.HasPrefix(color, "hsl(") || strings.HasPrefix(color, "hsla(") {
		parts := strings.Split(strings.TrimSuffix(color[strings.Index(color, "(")+1:], ")"), ",")
		if len(parts) < 3 {
			return 0, false
		}
		h, e := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		if e != nil || h < 0 || h > 360 {
			return 0, false
		}
		for _, p := range parts[1:3] {
			n, e := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(p), "%"), 64)
			if e != nil || n < 0 || n > 100 {
				return 0, false
			}
		}
		return h, true
	}
	var red, green, blue float64
	if strings.HasPrefix(color, "#") && (len(color) == 4 || len(color) == 7) {
		hex := strings.TrimPrefix(color, "#")
		if len(hex) == 3 {
			hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
		}
		v, e := strconv.ParseUint(hex, 16, 24)
		if e != nil {
			return 0, false
		}
		red = float64(v >> 16)
		green = float64((v >> 8) & 255)
		blue = float64(v & 255)
	} else if reSafeColorRGB.MatchString(color) {
		var values []float64
		for _, n := range regexp.MustCompile(`[\d.]+`).FindAllString(color, -1) {
			v, _ := strconv.ParseFloat(n, 64)
			values = append(values, v)
		}
		if len(values) < 3 {
			return 0, false
		}
		red, green, blue = values[0], values[1], values[2]
	} else {
		return 0, false
	}
	if red > 255 || green > 255 || blue > 255 {
		return 0, false
	}
	maximum := math.Max(red, math.Max(green, blue))
	minimum := math.Min(red, math.Min(green, blue))
	diff := maximum - minimum
	if diff == 0 {
		return 0, true
	}
	h := 0.0
	switch maximum {
	case red:
		h = (green - blue) / diff
	case green:
		h = 2 + (blue-red)/diff
	default:
		h = 4 + (red-green)/diff
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h, true
}
