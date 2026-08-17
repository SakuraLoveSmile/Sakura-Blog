package post

import (
	"blog/internal/settings"
	"fmt"
	"html"
	"strings"
	"sync/atomic"
)

// ---------- Spec 20: 标签插件三期（video/audio/copy/poetry/rating/swiper/chat/ghcard） ----------

// renderVideo 渲染 {% video src %}，支持 bilibili/youtube 嵌入与本地视频。
func renderVideo(argsStr string) string {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"type", "bilibili", "youtube", "ratio", "width", "autoplay"}, []string{"src"})
	// 安全审计 2026-08-16：width/ratio 进 style，走数值白名单，非法回退默认
	width := safeCSSNum(named["width"])
	if width == "" {
		width = "100%"
	}
	ratio := safeCSSRatio(named["ratio"])
	if ratio == "" {
		ratio = "1.7777777777777777" // 16/9
	}
	if bv := named["bilibili"]; bv != "" {
		// 安全审计 2026-08-16：autoplay 进 iframe URL，白名单 true/false，非法回退 false
		autoplay := "false"
		if named["autoplay"] == "true" {
			autoplay = "true"
		}
		return fmt.Sprintf(`<div class="tag-plugin video-player" style="aspect-ratio:%s;max-width:%s;"><iframe src="https://player.bilibili.com/player.html?bvid=%s&autoplay=%s" scrolling="no" border="0" frameborder="no" framespacing="0" allowfullscreen="true"></iframe></div>`,
			ratio, width, html.EscapeString(bv), autoplay)
	}
	if yt := named["youtube"]; yt != "" {
		autoplay := "0"
		if named["autoplay"] == "true" || named["autoplay"] == "1" {
			autoplay = "1&mute=1"
		}
		return fmt.Sprintf(`<div class="tag-plugin video-player" style="aspect-ratio:%s;max-width:%s;"><iframe style="border:none" src="https://www.youtube.com/embed/%s?rel=0&disablekb=1&playsinline=1&autoplay=%s" picture-in-picture="true" allowfullscreen="true"></iframe></div>`,
			ratio, width, html.EscapeString(yt), autoplay)
	}
	typ := named["type"]
	if typ == "" {
		typ = "video/mp4"
	}
	// 安全审计 2026-08-16：type 用户可控，转义后再拼入属性
	return fmt.Sprintf(`<div class="tag-plugin video-player" style="max-width:%s;"><video controls preload playsinline webkit-playsinline><source src="%s" type="%s">Your browser does not support the video tag.</video></div>`,
		width, html.EscapeString(named["src"]), html.EscapeString(typ))
}

// renderAudio 渲染 {% audio src %}，支持网易云外链与本地音频。
func renderAudio(argsStr string) string {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"type", "netease", "autoplay"}, []string{"src"})
	if netease := named["netease"]; netease != "" {
		typ := named["type"]
		// 安全审计 2026-08-16：type 进 iframe URL，白名单纯数字（0/1/2），非法回退 2
		if typ == "" || safeCSSNum(typ) == "" {
			typ = "2"
		}
		auto := "0"
		if named["autoplay"] == "true" {
			auto = "1"
		}
		return fmt.Sprintf(`<div class="tag-plugin audio"><iframe src="//music.163.com/outchain/player?type=%s&id=%s&auto=%s&height=32" frameborder="no" border="0" marginwidth="0" marginheight="0" width=288px height=52></iframe></div>`,
			typ, html.EscapeString(netease), auto)
	}
	typ := named["type"]
	if typ == "" {
		typ = "audio/mp3"
	}
	// 安全审计 2026-08-16：type 用户可控，转义后再拼入属性
	return fmt.Sprintf(`<div class="tag-plugin audio"><audio controls preload><source src="%s" type="%s">Your browser does not support the audio tag.</audio></div>`,
		html.EscapeString(named["src"]), html.EscapeString(typ))
}

// renderCopy 渲染 {% copy text %} / {% copy git user/repo %} 为复制按钮。
// mbti 人格卡片（1.38）。zh 标签/类型名硬编码，HTML 结构对齐上游 mbti.styl 选择器。
var mbtiRoles = map[string]string{
	"INTJ": "analyst", "INTP": "analyst", "ENTJ": "analyst", "ENTP": "analyst",
	"INFJ": "diplomat", "INFP": "diplomat", "ENFJ": "diplomat", "ENFP": "diplomat",
	"ISTJ": "sentinel", "ISFJ": "sentinel", "ESTJ": "sentinel", "ESFJ": "sentinel",
	"ISTP": "explorer", "ISFP": "explorer", "ESTP": "explorer", "ESFP": "explorer",
}
var mbtiRoleColors = map[string]string{"analyst": "purple", "diplomat": "green", "sentinel": "blue", "explorer": "yellow"}
var mbtiTypeNames = map[string]string{
	"INTJ": "建筑师", "INTP": "逻辑学家", "ENTJ": "指挥官", "ENTP": "辩论家",
	"INFJ": "提倡者", "INFP": "调停者", "ENFJ": "主人公", "ENFP": "竞选者",
	"ISTJ": "物流师", "ISFJ": "守卫者", "ESTJ": "总经理", "ESFJ": "执政官",
	"ISTP": "鉴赏家", "ISFP": "探险家", "ESTP": "企业家", "ESFP": "表演者",
}
var mbtiTraits = []struct{ key, color, left, right string }{
	{"energy", "#2196f3", "外向", "内向"},
	{"mind", "#FFBD2B", "直觉", "现实"},
	{"nature", "#3DC550", "思考", "情感"},
	{"tactics", "#9c27b0", "计划", "展望"},
	{"identity", "#F44336", "坚决", "谨慎"},
}

func mbtiAlphaHex(hex string, alpha float64) string {
	if len(hex) != 7 {
		return hex
	}
	var r, g, b int
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return fmt.Sprintf("rgba(%d,%d,%d,%g)", r, g, b, alpha)
}

func renderMbti(argsStr string) string {
	fields := strings.Fields(argsStr)
	if len(fields) == 0 {
		return ""
	}
	typeCode := strings.ToUpper(fields[0])
	scores := ""
	url, image := "", ""
	for _, f := range fields[1:] {
		switch {
		case strings.HasPrefix(f, "url:"):
			url = strings.TrimPrefix(f, "url:")
		case strings.HasPrefix(f, "image:"):
			image = strings.TrimPrefix(f, "image:")
		default:
			if scores == "" {
				scores = f
			}
		}
	}
	key := typeCode
	if len(key) > 4 {
		key = key[:4]
	}
	role := mbtiRoles[key]
	themeColor := mbtiRoleColors[role]
	typeName := mbtiTypeNames[key]
	if typeName == "" {
		typeName = typeCode
	}
	pcts := make([]int, 5)
	sp := strings.Split(scores, "-")
	for i := 0; i < 5; i++ {
		if i < len(sp) {
			v := 0
			fmt.Sscanf(strings.TrimSpace(sp[i]), "%d", &v)
			if v < 0 {
				v = 0
			}
			if v > 100 {
				v = 100
			}
			pcts[i] = v
		}
	}
	code := strings.ReplaceAll(typeCode, "-", "")
	sides := [5]string{}
	if len(code) > 0 && code[0] == 'E' {
		sides[0] = "left"
	} else {
		sides[0] = "right"
	}
	if len(code) > 1 && code[1] == 'N' {
		sides[1] = "left"
	} else {
		sides[1] = "right"
	}
	if len(code) > 2 && code[2] == 'T' {
		sides[2] = "left"
	} else {
		sides[2] = "right"
	}
	if len(code) > 3 && code[3] == 'J' {
		sides[3] = "left"
	} else {
		sides[3] = "right"
	}
	if len(code) > 4 && code[4] == 'T' {
		sides[4] = "right"
	} else {
		sides[4] = "left"
	}
	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin colorful note mbti"`)
	if themeColor != "" {
		sb.WriteString(` color="` + themeColor + `"`)
	}
	sb.WriteString(`>`)
	if image != "" {
		sb.WriteString(`<img class="mbti-image" src="` + html.EscapeString(image) + `" alt="" />`)
	}
	title := typeCode + " · " + typeName
	sb.WriteString(`<div class="mbti-title">`)
	// 安全审计 2026-08-16：url 校验 scheme（拒 javascript: 等），非法降级为无链接文本
	if u := safeURL(url); u != "" {
		sb.WriteString(`<a class="mbti-title-link" href="` + html.EscapeString(u) + `" target="_blank" rel="external nofollow noopener noreferrer">` + html.EscapeString(title) + `</a>`)
	} else {
		sb.WriteString(`<span class="mbti-title-text">` + html.EscapeString(title) + `</span>`)
	}
	sb.WriteString(`</div><div class="mbti-body"><div class="mbti-traits">`)
	for i, t := range mbtiTraits {
		pct := pcts[i]
		reverse := sides[i] == "left"
		position := pct
		if reverse {
			position = 100 - pct
		}
		activeSide := "right"
		if position <= 50 {
			activeSide = "left"
		}
		reversed := ""
		if sides[i] == "right" {
			reversed = " reversed"
		}
		sb.WriteString(`<div class="mbti-trait` + reversed + `" style="--theme:` + t.color + `; --theme-a30:` + mbtiAlphaHex(t.color, 0.3) + `">`)
		sb.WriteString(`<div class="mbti-labels">`)
		if activeSide == "left" {
			sb.WriteString(`<span class="mbti-label left active">` + t.left + ` <span class="pct">` + fmt.Sprintf("%d", pct) + `%</span></span>`)
			sb.WriteString(`<span class="mbti-label right">` + t.right + `</span>`)
		} else {
			sb.WriteString(`<span class="mbti-label left">` + t.left + `</span>`)
			sb.WriteString(`<span class="mbti-label right active"><span class="pct">` + fmt.Sprintf("%d", pct) + `%</span> ` + t.right + `</span>`)
		}
		sb.WriteString(`</div>`)
		sb.WriteString(`<div class="mbti-progress"><div class="fill" style="width:` + fmt.Sprintf("%d", 100-pct) + `%"></div><span class="mbti-dot" style="left:` + fmt.Sprintf("%d", position) + `%"></span></div>`)
		sb.WriteString(`</div>`)
	}
	sb.WriteString(`</div></div></div>`)
	return sb.String()
}

func renderCopy(argsStr string, tagPlugins settings.TagPluginsConfig) string {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"git", "prefix"}, []string{"text"})
	text := named["text"]
	if text == "" {
		return ""
	}
	if git := named["git"]; git != "" {
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
	id := fmt.Sprintf("copy_%d", atomic.AddUint64(&copyIndex, 1))
	// tag_plugins.copy.toast：复制成功提示文案（空=内置「复制成功」）
	toast := tagPlugins.Copy.Toast
	if toast == "" {
		toast = "复制成功"
	}
	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin copy">`)
	if prefix := named["prefix"]; prefix != "" {
		sb.WriteString(`<span>` + html.EscapeString(prefix) + `</span>`)
	}
	sb.WriteString(fmt.Sprintf(`<input class="copy-area" id="%s" value="%s">`, id, html.EscapeString(text)))
	// 安全审计 2026-08-16：toast 配置文案进入 onclick JS 字符串，转义防属性逃逸
	sb.WriteString(fmt.Sprintf(`<button class="copy-btn" onclick="util.copy(&quot;%s&quot;,&quot;%s&quot;)">`, id, html.EscapeString(toast)))
	sb.WriteString(`<svg class="icon copy-btn" viewBox="0 0 16 16" version="1.1" width="16" height="16" aria-hidden="true"><path fill-rule="evenodd" d="M5.75 1a.75.75 0 00-.75.75v3c0 .414.336.75.75.75h4.5a.75.75 0 00.75-.75v-3a.75.75 0 00-.75-.75h-4.5zm.75 3V2.5h3V4h-3zm-2.874-.467a.75.75 0 00-.752-1.298A1.75 1.75 0 002 3.75v9.5c0 .966.784 1.75 1.75 1.75h8.5A1.75 1.75 0 0014 13.25v-9.5a1.75 1.75 0 00-.874-1.515.75.75 0 10-.752 1.298.25.25 0 01.126.217v9.5a.25.25 0 01-.25.25h-8.5a.25.25 0 01-.25-.25v-9.5a.25.25 0 01.126-.217z"></path></svg>`)
	sb.WriteString(`</button></div>`)
	return sb.String()
}

// renderRating 渲染 {% rating id %} 为评分组件，对接本地 /api/rating 数据服务。
func renderRating(argsStr string, iconFunc func(string) string) string {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"id", "icon"}, []string{"title"})
	id := named["id"]
	if id == "" {
		id = "default"
	}
	iconKey := named["icon"]
	if iconKey == "" {
		iconKey = "rating:star"
	}
	star := ""
	if iconFunc != nil {
		star = iconFunc(iconKey)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<div class="tag-plugin ds-rating" data-api="/api/rating" data-id="%s">`, html.EscapeString(id)))
	if title := named["title"]; title != "" {
		sb.WriteString(`<div class="header"><span>` + html.EscapeString(title) + `</span></div>`)
	}
	sb.WriteString(`<div class="body">`)
	for i := 1; i <= 5; i++ {
		sb.WriteString(fmt.Sprintf(`<button class="star" data-value="%d">%s</button>`, i, star))
	}
	sb.WriteString(`<span class="avg">(0.0)</span></div>`)
	sb.WriteString(`<div class="footer"><span class="count">0</span><span class="suffix">人已评分</span></div>`)
	sb.WriteString(`</div>`)
	return sb.String()
}

// renderGHCard 渲染 {% ghcard user/repo %} / {% ghcard user %}。
// 复用原生 GitHub API 卡片渲染（renderGHRepo/renderGHUser）：自托管、带内存/磁盘缓存与降级，
// 不再依赖第三方 github-readme-stats 实例（其公共实例常被暂停导致裂图）。
// github-readme-stats 专有的主题/配色参数（theme/*_color 等）在原生卡片下无意义，解析后忽略。

func renderPoetry(argsStr, body string, renderMD func(string) string) string {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"author", "date", "footer"}, []string{"title"})
	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin poetry"><div class="content">`)
	if title := named["title"]; title != "" {
		sb.WriteString(`<div class="title">` + html.EscapeString(title) + `</div>`)
	}
	if named["author"] != "" || named["date"] != "" {
		sb.WriteString(`<div class="meta">`)
		if a := named["author"]; a != "" {
			sb.WriteString(`<span>` + html.EscapeString(a) + `</span>`)
		}
		if d := named["date"]; d != "" {
			sb.WriteString(`<span>` + html.EscapeString(d) + `</span>`)
		}
		sb.WriteString(`</div>`)
	}
	rendered := strings.ReplaceAll(renderMD(body), "\n", "")
	sb.WriteString(`<div class="body">` + rendered + `</div>`)
	if footer := named["footer"]; footer != "" {
		sb.WriteString(`<div class="footer">` + html.EscapeString(footer) + `</div>`)
	}
	sb.WriteString(`</div></div>`)
	return sb.String()
}

// renderSwiper 渲染 {% swiper [width:] [effect:] %} imgs {% endswiper %} 为图片轮播。
func renderSwiper(argsStr, body string, renderMD func(string) string) string {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"width", "effect"}, nil)
	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin swiper fancybox" id="swiper-api"`)
	if w := named["width"]; w != "" {
		sb.WriteString(` width="` + html.EscapeString(w) + `"`)
		// Spec 49：width 语义解析——max→容器 max-width 像素值（800px），min→容器 max-width 百分比（配合 slide 25% 迷你轮播）
		switch w {
		case "max":
			sb.WriteString(` style="max-width:800px"`)
		case "min":
			sb.WriteString(` style="max-width:100%"`)
		}
	}
	if e := named["effect"]; e != "" {
		sb.WriteString(` effect="` + html.EscapeString(e) + `"`)
	}
	sb.WriteString(`><div class="swiper-wrapper">`)
	for _, img := range reSwiperImg.FindAllString(renderMD(body), -1) {
		img = strings.Replace(img, "<img src", "<img no-lazy src", 1)
		sb.WriteString(`<div class="swiper-slide">` + img + `</div>`)
	}
	sb.WriteString(`</div><div class="swiper-pagination"></div><div class="swiper-button-prev blur"></div><div class="swiper-button-next blur"></div></div>`)
	return sb.String()
}

// chatUser 聊天参与者（从 body 顶部 YAML 块解析）。
type chatUser struct {
	Name           string
	Avatar         string
	LabelText      string
	LabelTextColor string
	LabelBgColor   string
}

// chatCell 聊天单元格（chattip 时间提示 / chatcell 消息）。
type chatCell struct {
	kind   string // chattip | chatcell
	text   string // chattip 文本
	params map[string]string
}

// parseChatUsers 解析 body 顶部（首个 <!-- 注释之前）的简易 YAML 用户块。
func parseChatUsers(body string) map[string]chatUser {
	users := make(map[string]chatUser)
	yamlBlock := body
	if idx := strings.Index(body, "<!--"); idx >= 0 {
		yamlBlock = body[:idx]
	}
	var cur string
	inLabel := false
	for _, line := range strings.Split(yamlBlock, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent == 0 && strings.HasSuffix(trimmed, ":") {
			cur = strings.TrimSuffix(trimmed, ":")
			users[cur] = chatUser{}
			inLabel = false
			continue
		}
		if cur == "" {
			continue
		}
		kv := strings.SplitN(trimmed, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		u := users[cur]
		if indent >= 4 && inLabel {
			switch key {
			case "text":
				u.LabelText = val
			case "textColor":
				u.LabelTextColor = val
			case "bgColor":
				u.LabelBgColor = val
			}
		} else if indent >= 2 {
			if key == "label" && val == "" {
				inLabel = true
			} else {
				inLabel = false
				switch key {
				case "name":
					u.Name = val
				case "avatar":
					u.Avatar = val
				}
			}
		}
		users[cur] = u
	}
	return users
}

// parseChatCells 按顺序提取 <!-- chattip ... --> / <!-- chatcell ... --> 单元格。
func parseChatCells(body string) []chatCell {
	var cells []chatCell
	for _, m := range reChatComment.FindAllStringSubmatch(body, -1) {
		content := strings.TrimSpace(m[1])
		switch {
		case strings.HasPrefix(content, "chattip "):
			cells = append(cells, chatCell{kind: "chattip", text: strings.TrimSpace(strings.TrimPrefix(content, "chattip "))})
		case strings.HasPrefix(content, "chatcell "):
			cells = append(cells, chatCell{kind: "chatcell", params: parseChatCellParams(strings.TrimPrefix(content, "chatcell "))})
		}
	}
	return cells
}

// parseChatCellParams 解析 chatcell 的 key:value 参数（值可含空格，遇下一个 key 截断）。
func parseChatCellParams(s string) map[string]string {
	params := make(map[string]string)
	locs := reChatParam.FindAllStringSubmatchIndex(s, -1)
	for i, loc := range locs {
		key := s[loc[2]:loc[3]]
		valStart := loc[1]
		valEnd := len(s)
		if i+1 < len(locs) {
			valEnd = locs[i+1][0]
		}
		params[key] = strings.TrimSpace(s[valStart:valEnd])
	}
	return params
}

// renderChat 渲染 {% chat device [style:] [title:] [scene:] [me:] %}（文字消息 + 时间提示子集）。
func renderChat(argsStr, body string, renderMD func(string) string) string {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"style", "title", "scene", "me", "labelColorStyle"}, []string{"device"})
	// 安全审计 2026-08-16：style/scene 拼入 class，走 class 词白名单，非法回退默认
	style := safeClassWord(named["style"])
	if style == "" {
		style = "qq"
	}
	scene := safeClassWord(named["scene"])
	if scene == "" {
		scene = "group"
	}
	me := named["me"]
	users := parseChatUsers(body)
	cells := parseChatCells(body)

	msgCount := 0
	for _, c := range cells {
		if c.kind == "chatcell" {
			msgCount++
		}
	}

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin chat ` + html.EscapeString(named["device"]) + ` ` + style + ` ` + scene + `">`)
	// 导航栏
	sb.WriteString(`<div class="chat-navbar"><div class="left-items">`)
	sb.WriteString(`<div class="back-btn"><svg t="1725176176548" class="icon back" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="992" width="200" height="200"><path d="M347.008 574.379a85.547 85.547 0 0 1 0-124.758l285.483-267.477a42.39 42.39 0 0 1 60.074 2.07c16.022 17.237 15.104 44.223-2.069 60.309L405.013 512l285.483 267.477c17.173 16.086 18.09 43.094 2.07 60.31a42.39 42.39 0 0 1-60.075 2.069L347.008 574.379z" p-id="993"></path></svg></div>`)
	sb.WriteString(fmt.Sprintf(`<div class="news-num">%d</div>`, msgCount))
	sb.WriteString(`<div class="title">` + html.EscapeString(named["title"]) + `</div>`)
	sb.WriteString(`</div><div class="right-items"><div class="more-btn"><svg t="1725176222076" class="icon more" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="1146" width="200" height="200"><path d="M106.667 170.667A42.667 42.667 0 0 1 149.333 128h725.334a42.667 42.667 0 1 1 0 85.333H149.333a42.667 42.667 0 0 1-42.666-42.666z m0 341.333a42.667 42.667 0 0 1 42.666-42.667h725.334a42.667 42.667 0 1 1 0 85.334H149.333A42.667 42.667 0 0 1 106.667 512z m0 341.333a42.667 42.667 0 0 1 42.666-42.666h725.334a42.667 42.667 0 1 1 0 85.333H149.333a42.667 42.667 0 0 1-42.666-42.667z" p-id="1147"></path></svg></div></div></div>`)
	// 消息区
	sb.WriteString(`<div class="content">`)
	for _, c := range cells {
		if c.kind == "chattip" {
			sb.WriteString(`<div class="chattip">` + html.EscapeString(c.text) + `</div>`)
			continue
		}
		// Spec 49：多媒体 cell（type:image/voice/video/file/link）仅输出类型占位，不渲染实际内容防破版
		if t := c.params["type"]; t != "" {
			fmt.Fprintf(&sb, `<div class="chatcell unsupported">[%s]</div>`, html.EscapeString(t))
			continue
		}
		u := users[c.params["user"]]
		sb.WriteString(`<div class="chatcell`)
		// Spec 49：scene=group 群聊模式全部左对齐（不输出 right 类）
		if scene != "group" && me != "" && me == c.params["user"] {
			sb.WriteString(` right`)
		}
		sb.WriteString(`">`)
		sb.WriteString(`<div class="user-avatar"><img lazy src="` + html.EscapeString(u.Avatar) + `"></div>`)
		sb.WriteString(`<div class="user-main"><div class="header">`)
		if u.LabelText != "" {
			// 安全审计 2026-08-16：textColor/bgColor 拼 style，走颜色白名单，非法回退默认
			textColor := safeColor(u.LabelTextColor)
			if textColor == "" {
				textColor = "white"
			}
			bgColor := safeColor(u.LabelBgColor)
			if bgColor == "" {
				bgColor = "#2196f3"
			}
			sb.WriteString(`<span class="label hand-color" style="color:` + html.EscapeString(textColor) + `;background:` + html.EscapeString(bgColor) + `;">` + html.EscapeString(u.LabelText) + `</span>`)
		}
		sb.WriteString(`<span class="name">` + html.EscapeString(u.Name) + `</span></div>`)
		rendered := strings.ReplaceAll(renderMD(c.params["md"]), "\n", "")
		sb.WriteString(`<div class="talk md">` + rendered + `</div>`)
		sb.WriteString(`</div></div>`)
	}
	sb.WriteString(`</div>`)
	// 底部输入栏（qq 样式）
	if style == "qq" {
		sb.WriteString(`<div class="bottom"><div class="input"><div class="input-text"></div><div class="submit-btn">发送</div></div></div>`)
	}
	sb.WriteString(`</div>`)
	return sb.String()
}
