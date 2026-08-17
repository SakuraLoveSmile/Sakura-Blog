package post

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// --- Spec 31 渲染函数（输出结构逐字对齐原版 scripts/tags/lib/*.js）---

// renderAbout 对齐 about.js：wrapper + 可选返回箭头 + 可选头像区 + about-body。
func renderAbout(argsStr, body string, renderMD func(string) string) string {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"avatar", "height", "border", "back"}, nil)
	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin about">`)
	if named["back"] != "" {
		sb.WriteString(`<a class="nav-back cap" href="/">`)
		sb.WriteString(backArrowSVG)
		sb.WriteString(`</a>`)
	}
	if avatar := named["avatar"]; avatar != "" {
		sb.WriteString(`<div class="about-header"><div class="avatar">`)
		fmt.Fprintf(&sb, `<img src="%s"`, html.EscapeString(avatar))
		// 安全审计 2026-08-16：border 进 style，走 CSS 数值白名单，非法值丢弃
		if b := safeCSSNum(named["border"]); b != "" {
			fmt.Fprintf(&sb, ` style="border-radius:%s"`, html.EscapeString(b))
		}
		if h := named["height"]; h != "" {
			fmt.Fprintf(&sb, ` height="%s"`, html.EscapeString(h))
		}
		sb.WriteString(`/></div></div>`)
	}
	sb.WriteString(`<div class="about-body fs14">`)
	sb.WriteString(strings.ReplaceAll(renderMD(body), "\n", ""))
	sb.WriteString(`</div></div>`)
	return sb.String()
}

// renderBlockquote 对齐 blockquote.js：indent 属性透传，body 渲染后去换行。
func renderBlockquote(argsStr, body string, renderMD func(string) string) string {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"indent"}, nil)
	var sb strings.Builder
	fmt.Fprintf(&sb, `<div class="tag-plugin blockquote" indent="%s">`, html.EscapeString(named["indent"]))
	sb.WriteString(strings.ReplaceAll(renderMD(body), "\n", ""))
	sb.WriteString(`</div>`)
	return sb.String()
}

// renderReel 对齐 read/reel.js：content/title/meta/body.main/date/footer 结构。
func renderReel(argsStr, body string, renderMD func(string) string) string {
	named, positional := parseNamedPositionalArgs(argsStr, []string{"author", "date", "footer"}, nil)
	title := strings.Join(positional, " ")
	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin reel">`)
	sb.WriteString(`<div class="content">`)
	sb.WriteString(`<div class="title">`)
	sb.WriteString(html.EscapeString(title)) // 布局需要：空 title 也输出容器（安全审计 2026-08-16：转义）
	sb.WriteString(`</div>`)
	if a := named["author"]; a != "" {
		// 安全审计 2026-08-16：author 用户可控，转义后再输出
		fmt.Fprintf(&sb, `<div class="meta"><span>%s</span></div>`, html.EscapeString(a))
	}
	sb.WriteString(`<div class="body"><div class="main">`)
	sb.WriteString(strings.ReplaceAll(renderMD(body), "\n", ""))
	sb.WriteString(`</div></div>`)
	if d := named["date"]; d != "" {
		// 安全审计 2026-08-16：date 用户可控，转义后再输出
		fmt.Fprintf(&sb, `<div class="date">%s</div>`, html.EscapeString(d))
	}
	sb.WriteString(`<div class="footer">`)
	sb.WriteString(html.EscapeString(named["footer"])) // 布局需要（安全审计 2026-08-16：转义）
	sb.WriteString(`</div>`)
	sb.WriteString(`</div></div>`)
	return sb.String()
}

// renderPaper 对齐 read/paper.js：content(title/body/footer)，body 按 paragraph/section/line 标记分段。
func renderPaper(argsStr, body string, renderMD func(string) string) string {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"style", "title", "author", "date", "footer"}, nil)
	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin paper">`)
	if s := named["style"]; s != "" {
		fmt.Fprintf(&sb, `<div class="content %s">`, html.EscapeString(s))
	} else {
		sb.WriteString(`<div class="content">`)
	}
	sb.WriteString(`<div class="title">`)             // 布局需要
	sb.WriteString(html.EscapeString(named["title"])) // 安全审计 2026-08-16：title 用户可控，转义后再输出
	sb.WriteString(`</div>`)

	sb.WriteString(`<div class="body">`)
	// 按标记切分：每个标记的正文是标记结束到下一标记开始
	locs := rePaperMarker.FindAllStringSubmatchIndex(body, -1)
	for i, loc := range locs {
		typ := body[loc[2]:loc[3]]
		arg := ""
		if loc[4] >= 0 {
			arg = body[loc[4]:loc[5]]
		}
		segEnd := len(body)
		if i+1 < len(locs) {
			segEnd = locs[i+1][0]
		}
		seg := strings.TrimSpace(body[loc[1]:segEnd])
		rendered := strings.ReplaceAll(renderMD(seg), "\n", "")
		switch typ {
		case "paragraph":
			sb.WriteString(`<div class="paragraph">` + rendered + `</div>`)
		case "line":
			if arg == "right" {
				sb.WriteString(`<div class="line right">` + rendered + `</div>`)
			} else {
				sb.WriteString(`<div class="line">` + rendered + `</div>`)
			}
		case "section":
			// 安全审计 2026-08-16：section 标题来自注释标记，作者可控，转义后再输出
			sb.WriteString(`<div class="section"><div class="section-title">` + html.EscapeString(arg) + `</div><div class="section-content">` + rendered + `</div></div>`)
		}
	}
	sb.WriteString(`</div>`)

	sb.WriteString(`<div class="footer">`) // 布局需要
	if named["author"] != "" || named["date"] != "" {
		sb.WriteString(`<div class="author-date">`)
		if a := named["author"]; a != "" {
			// 安全审计 2026-08-16：author/date 用户可控，转义后再输出
			sb.WriteString(`<span class="author">` + html.EscapeString(a) + `</span>`)
		}
		if d := named["date"]; d != "" {
			sb.WriteString(`<span class="date">` + html.EscapeString(d) + `</span>`)
		}
		sb.WriteString(`</div>`)
	}
	sb.WriteString(html.EscapeString(named["footer"])) // 安全审计 2026-08-16：footer 用户可控，转义后再输出
	sb.WriteString(`</div>`)
	sb.WriteString(`</div></div>`)
	return sb.String()
}

// renderNavbar 对齐 navbar.js：[text](href) 逐个转 a.link，active 命中加 active 类。
func renderNavbar(argsStr string) (string, bool) {
	named, positional := parseNamedPositionalArgs(argsStr, []string{"active"}, nil)
	if len(positional) == 0 {
		return "", false
	}
	reMDLink := regexp.MustCompile(`\[(.*?)\]\((.*?)\)`)
	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin navbar">`)
	for _, link := range positional {
		if m := reMDLink.FindStringSubmatch(link); len(m) > 2 {
			// 安全审计 2026-08-16：href 校验 scheme（拒 javascript: 等）、text 转义后再输出
			text, href := m[1], m[2]
			if u := safeURL(href); u != "" {
				href = u
			} else {
				href = "#"
			}
			if href == named["active"] {
				fmt.Fprintf(&sb, `<a class="link active" href="%s">%s</a>`, html.EscapeString(href), html.EscapeString(text))
			} else {
				fmt.Fprintf(&sb, `<a class="link" href="%s">%s</a>`, html.EscapeString(href), html.EscapeString(text))
			}
		} else {
			fmt.Fprintf(&sb, `<a class="link" href="#%s">%s</a>`, html.EscapeString(link), html.EscapeString(link))
		}
	}
	sb.WriteString(`</div>`)
	return sb.String(), true
}

// renderNavbarBlock 渲染 Spec 49 块级文章内导航：{% navbar [active:文本] %} 每行 [text](url) {% endnavbar %}。
// active 与链接文本匹配（区别于单行版 renderNavbar 的 active 匹配 href）；非链接行忽略，不 panic 不吞正文。
func renderNavbarBlock(argsStr, body string) string {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"active"}, nil)
	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin navbar">`)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := reNavbarLink.FindStringSubmatch(line)
		if len(m) < 3 {
			continue
		}
		// 安全审计 2026-08-16：href 校验 scheme（拒 javascript: 等），非法回退锚点
		text, href := m[1], m[2]
		if u := safeURL(href); u != "" {
			href = u
		} else {
			href = "#"
		}
		if text == named["active"] {
			fmt.Fprintf(&sb, `<a class="nav-link active" href="%s">%s</a>`, html.EscapeString(href), html.EscapeString(text))
		} else {
			fmt.Fprintf(&sb, `<a class="nav-link" href="%s">%s</a>`, html.EscapeString(href), html.EscapeString(text))
		}
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

// renderBanner 对齐 banner.js：bg 图 + top（返回键/navbar 行）+ bottom（avatar/标题区）+ 可选整卡链接。
// body 内嵌的 navbar 标签由管线先行展开，这里按原版逻辑提取含 "tag-plugin navbar" 的行。
func renderBanner(argsStr, body string, renderMD func(string) string) string {
	named, positional := parseNamedPositionalArgs(argsStr, []string{"bg", "avatar", "link", "title", "subtitle"}, nil)
	title := named["title"]
	subtitle := named["subtitle"]
	// 兼容 Spec 31 旧语法：title/subtitle 位置参数
	if title == "" && len(positional) > 0 {
		title = positional[0]
	}
	if subtitle == "" && len(positional) > 1 {
		subtitle = strings.Join(positional[1:], " ")
	}
	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin banner"`)
	// 安全审计 2026-08-16：bg 进 style url()，校验 scheme 后再转义拼入
	if bg := safeURL(named["bg"]); bg != "" {
		fmt.Fprintf(&sb, ` style="background-image:url(%s)"`, html.EscapeString(bg))
	}
	sb.WriteString(`>`)
	// 安全审计 2026-08-16：link 校验 scheme（拒 javascript: 等），非法不输出整卡链接
	link := safeURL(named["link"])
	if link != "" {
		fmt.Fprintf(&sb, `<a class="banner-link" href="%s">`, html.EscapeString(link))
	}
	sb.WriteString(`<div class="banner-content">`)
	if avatar := named["avatar"]; avatar != "" {
		fmt.Fprintf(&sb, `<div class="avatar"><img src="%s"></div>`, html.EscapeString(avatar))
	}
	if title != "" {
		fmt.Fprintf(&sb, `<div class="title">%s</div>`, html.EscapeString(title))
	}
	if subtitle != "" {
		fmt.Fprintf(&sb, `<div class="subtitle">%s</div>`, html.EscapeString(subtitle))
	}
	fmt.Fprintf(&sb, `<div class="body fs14">%s</div>`, strings.ReplaceAll(renderMD(body), "\n", ""))
	sb.WriteString(`</div>`)
	if link != "" {
		sb.WriteString(`</a>`)
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

// renderAlbumsPosters 对齐 albums.js / posters.js：api/repo 驱动 ds-friends 服务填充 gallery。
// group 模式依赖站点 links 配置（shortcode 层不可得），输出空 grid 容器（与原版 group 缺失行为一致）。
func renderAlbumsPosters(kind, argsStr string) string {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"repo", "api", "size"}, nil)
	size := named["size"]
	if size == "" {
		if kind == "albums" {
			size = "s"
		} else {
			size = "xs" // posters 缺省 xs（对齐原版）
		}
	}
	api := named["api"]
	if api == "" && named["repo"] != "" {
		api = "https://raw.githubusercontent.com/" + named["repo"] + "/output/v2/data.json"
	}
	ratio := "square"
	if kind == "posters" {
		ratio = "portrait"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, `<div class="tag-plugin %s-wrap">`, kind)
	if api != "" {
		fmt.Fprintf(&sb, `<div class="data-service ds-friends" data-api="%s"><div class="tag-plugin gallery grid-box" layout="grid" ratio="square" size="%s"></div></div>`,
			html.EscapeString(api), html.EscapeString(size))
	} else {
		fmt.Fprintf(&sb, `<div class="tag-plugin gallery grid-box" layout="grid" ratio="%s" size="%s"></div>`, ratio, html.EscapeString(size))
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

// renderFrame 对齐 frame.js：video 优先，img 次之；(img|video) 与 device 缺一原样输出。
func renderFrame(argsStr string) (string, bool) {
	named, positional := parseNamedPositionalArgs(argsStr, []string{"focus", "img", "video"}, nil)
	device, alt := "", ""
	if len(positional) > 0 {
		device = positional[0]
	}
	if len(positional) > 1 {
		alt = strings.Join(positional[1:], " ")
	}
	img, video := named["img"], named["video"]
	if (img == "" && video == "") || device == "" {
		return "", false
	}
	focusAttr := ""
	if f := named["focus"]; f != "" {
		focusAttr = fmt.Sprintf(`focus="%s"`, html.EscapeString(f))
	}
	var sb strings.Builder
	if video != "" {
		sb.WriteString(`<div class="tag-plugin video-wrap">`)
		fmt.Fprintf(&sb, `<div class="frame-wrap" id="%s"%s>`, html.EscapeString(device), focusAttr)
		sb.WriteString(`<video`)
		if img != "" {
			fmt.Fprintf(&sb, ` poster="%s"`, html.EscapeString(img))
		}
		sb.WriteString(` playsinline="" muted="" loop="" autoplay="" preload="metadata">`)
		fmt.Fprintf(&sb, `<source src="%s" type="video/mp4">`, html.EscapeString(video))
		sb.WriteString(`</video>`)
		sb.WriteString(`<div class="frame"></div>`)
		sb.WriteString(`</div></div>`)
	} else {
		sb.WriteString(`<div class="tag-plugin img-wrap">`)
		fmt.Fprintf(&sb, `<div class="frame-wrap" id="%s"%s>`, html.EscapeString(device), focusAttr)
		fmt.Fprintf(&sb, `<img class="img" src="%s"`, html.EscapeString(img))
		if alt != "" {
			fmt.Fprintf(&sb, ` alt="%s"`, html.EscapeString(alt))
		}
		sb.WriteString(`/>`)
		sb.WriteString(`<div class="frame"></div>`)
		sb.WriteString(`</div>`)
		if alt != "" {
			// 安全审计 2026-08-16：alt 文本用户可控，转义后再输出
			fmt.Fprintf(&sb, `<span class="image-caption">%s</span>`, html.EscapeString(alt))
		}
		sb.WriteString(`</div>`)
	}
	return sb.String(), true
}

// renderFrameBlock 渲染 Spec 49 块级设备框架：{% frame [device:iphone/ipad/macbook] [width:宽度] %} 媒体 {% endframe %}。
// body 渲染后提取 <img>/<video>/<iframe> 嵌入纯 CSS 设备壳；device 缺失或 body 无媒体时原样输出容错。
func renderFrameBlock(argsStr, body string, renderMD func(string) string) (string, bool) {
	named, positional := parseNamedPositionalArgs(argsStr, []string{"device", "width"}, nil)
	device := named["device"]
	if device == "" && len(positional) > 0 {
		device = positional[0]
	}
	if device == "" {
		return "", false
	}
	media := reFrameMedia.FindString(renderMD(body))
	if media == "" {
		return "", false
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, `<div class="tag-plugin frame" device="%s"`, html.EscapeString(device))
	// 安全审计 2026-08-16：width 进 style，走 CSS 数值白名单，非法值丢弃
	if w := safeCSSNum(named["width"]); w != "" {
		fmt.Fprintf(&sb, ` style="max-width:%s"`, html.EscapeString(w))
	}
	sb.WriteString(`>`)
	fmt.Fprintf(&sb, `<div class="frame-device %s">`, html.EscapeString(device))
	sb.WriteString(`<div class="frame-notch"></div>`)
	sb.WriteString(`<div class="frame-content">` + media + `</div>`)
	sb.WriteString(`</div></div>`)
	return sb.String(), true
}

// renderTOCTag 对齐 toc.js：details 外壳；wiki 参数命中但本项目无 wiki 文档树（Spec 35）时，
// 对齐原版 proj==undefined 分支返回空字符串。
func renderTOCTag(argsStr string) string {
	named, positional := parseNamedPositionalArgs(argsStr, []string{"wiki", "open", "display"}, nil)
	if named["wiki"] != "" {
		return "" // wiki 树数据未建（Spec 35），对齐原版项目缺失行为
	}
	title := strings.Join(positional, " ")
	if title == "" {
		title = "TOC"
	}
	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin toc"`)
	if d := named["display"]; d != "" {
		fmt.Fprintf(&sb, ` display="%s"`, html.EscapeString(d))
		if d == "mobile" {
			sb.WriteString(` style="display:none"`)
		}
	}
	sb.WriteString(`>`)
	sb.WriteString(`<details class="toc"`)
	if named["open"] != "false" {
		sb.WriteString(` open`)
	}
	sb.WriteString(`>`)
	// 安全审计 2026-08-16：title 用户可控，转义后再输出
	fmt.Fprintf(&sb, `<summary>%s</summary>`, html.EscapeString(title))
	sb.WriteString(`</details></div>`)
	return sb.String()
}
