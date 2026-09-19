package post

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	tmpl "blog/internal/settings"
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

var validGallerySizes = map[string]bool{
	"xs": true, "s": true, "m": true, "l": true, "xl": true, "mix": true,
}

func safeGHRepo(repo string) string {
	repo = strings.TrimSpace(repo)
	repo = strings.TrimPrefix(repo, "https://github.com/")
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	for _, part := range parts {
		for _, r := range part {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
				return ""
			}
		}
	}
	return parts[0] + "/" + parts[1]
}

// renderAlbumsPosters 兼容缺省调用的包装函数。
func renderAlbumsPosters(kind, argsStr string) string {
	return renderAlbumsPostersHTML(kind, argsStr, nil, "")
}

// renderAlbumsPostersHTML 对齐 albums.js / posters.js：
// 优先级保持 api > repo > group。
// 1. api: 直接挂载 ds-friends 数据服务容器（ratio 强制 square，对齐上游）
// 2. repo: 转换为 raw.githubusercontent.com 数据服务 URL
// 3. group: 匹配友链分组标题直出卡片流；同名组按既有顺序合并；名称映射标题、快照映射封面、回退头像与默认封面
// 4. 无匹配或空组：输出空 gallery grid-box 容器（对齐原版）
func renderAlbumsPostersHTML(kind, argsStr string, groups []tmpl.LinkGroup, defaultCover string) string {
	named, positional := parseNamedPositionalArgs(argsStr, []string{"repo", "api", "size"}, []string{"group"})
	group := named["group"]
	if group == "" && len(positional) > 0 {
		group = strings.Join(positional, " ")
	}

	size := named["size"]
	if !validGallerySizes[size] {
		if kind == "albums" {
			size = "s"
		} else {
			size = "xs" // posters 缺省 xs（对齐原版）
		}
	}

	api := named["api"]
	if api == "" && named["repo"] != "" {
		repo := safeGHRepo(named["repo"])
		if repo != "" {
			api = "https://raw.githubusercontent.com/" + repo + "/output/v2/data.json"
		}
	}

	ratio := "square"
	cardClass := "album-card"
	if kind == "posters" {
		ratio = "portrait"
		cardClass = "poster-card"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, `<div class="tag-plugin %s-wrap">`, kind)

	if api != "" {
		// api / repo 分支：对齐上游 albums.js / posters.js，强制 ratio="square"
		fmt.Fprintf(&sb, `<div class="data-service ds-friends" data-api="%s"><div class="tag-plugin gallery grid-box" layout="grid" ratio="square" size="%s"></div></div>`,
			html.EscapeString(api), html.EscapeString(size))
	} else if group != "" {
		// group 分支：同名组按既有顺序合并
		var matchedItems []tmpl.LinkItem
		for _, g := range groups {
			if g.Title == group {
				matchedItems = append(matchedItems, g.Items...)
			}
		}

		fmt.Fprintf(&sb, `<div class="tag-plugin gallery grid-box" layout="grid" ratio="%s" size="%s">`, ratio, html.EscapeString(size))
		for _, item := range matchedItems {
			url := safeURL(item.URL)
			if url == "" {
				continue
			}
			cover := item.Snapshot
			if cover == "" {
				cover = item.Avatar
			}
			if cover == "" {
				cover = defaultCover
			}
			coverURL := safeURL(cover)
			fallbackCover := safeURL(defaultCover)
			if fallbackCover == "" {
				fallbackCover = coverURL
			}

			external := strings.Contains(url, "://")
			fmt.Fprintf(&sb, `<div class="grid-cell %s">`, cardClass)
			if external {
				fmt.Fprintf(&sb, `<a class="card-link lazy-box" target="_blank" rel="external nofollow noopener noreferrer" href="%s">`, html.EscapeString(url))
			} else {
				fmt.Fprintf(&sb, `<a class="card-link lazy-box" rel="noopener noreferrer" href="%s">`, html.EscapeString(url))
			}
			fmt.Fprintf(&sb, `<img class="lazy" data-src="%s" onerror="javascript:this.removeAttribute(&quot;data-src&quot;);this.src=&quot;%s&quot;;"/>`,
				html.EscapeString(coverURL), html.EscapeString(fallbackCover))
			sb.WriteString(`<div class="lazy-icon"></div>`)
			sb.WriteString(`<div class="image-meta">`)
			if item.Name != "" {
				fmt.Fprintf(&sb, `<span class="image-caption">%s</span>`, html.EscapeString(item.Name))
			}
			sb.WriteString(`</div></a></div>`)
		}
		sb.WriteString(`</div>`)
	} else {
		// 缺省空容器
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

// renderTOCTag 对齐 toc.js：
// 1. 无 wiki 参数时渲染 details 目录外壳（由前端/布局填充正文 TOC 结构）。
// 2. 指定 wiki 参数时：
//   - 通过 wikiTreeProvider 查询指定项目的已发布公开文档树；
//   - 项目不存在或无公开文档：生产模式输出空字符串（对齐原版 proj==undefined 行为）；预览模式输出安全诊断；
//   - 项目存在且有公开文档：严格按树序（深度优先前序遍历）渲染上游 toc.js DOM 结构。
func renderTOCTag(argsStr string, wikiTreeProvider func(string) (bool, []tmpl.WikiTreeNode), isPreview bool) string {
	named, positional := parseNamedPositionalArgs(argsStr, []string{"wiki", "open", "display"}, nil)
	title := strings.Join(positional, " ")
	if title == "" {
		title = "TOC"
	}

	wikiProject := named["wiki"]
	if wikiProject != "" {
		var exists bool
		var nodes []tmpl.WikiTreeNode
		if wikiTreeProvider != nil {
			exists, nodes = wikiTreeProvider(wikiProject)
		}
		if !exists {
			if isPreview {
				return renderTOCDiagnostic(title, fmt.Sprintf("Wiki 项目 %q 不存在", wikiProject), named["display"], named["open"])
			}
			return "" // 对齐原版 proj == undefined 行为：输出空
		}
		if len(nodes) == 0 {
			if isPreview {
				return renderTOCDiagnostic(title, fmt.Sprintf("Wiki 项目 %q 暂无公开文档", wikiProject), named["display"], named["open"])
			}
			return "" // 无已发布公开文档输出空
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
		fmt.Fprintf(&sb, `<summary>%s</summary>`, html.EscapeString(title))

		hasSections := false
		for _, n := range nodes {
			if len(n.Children) > 0 {
				hasSections = true
				break
			}
		}

		if hasSections {
			sb.WriteString(`<div class="body fs14 multi">`)
			for _, sec := range nodes {
				secTitle := ""
				if sec.Doc != nil {
					secTitle = sec.Doc.Title
				}
				sb.WriteString(`<section>`)
				fmt.Fprintf(&sb, `<div class="header">%s</div>`, html.EscapeString(secTitle))
				sb.WriteString(`<ul class="toc">`)
				if sec.Doc != nil && sec.Doc.Slug != "" {
					fmt.Fprintf(&sb, `<li><a class="list-link" href="/wiki/%s/%s"><span>%s</span></a></li>`,
						html.EscapeString(wikiProject), html.EscapeString(sec.Doc.Slug), html.EscapeString(sec.Doc.Title))
				}
				var renderChildren func([]tmpl.WikiTreeNode)
				renderChildren = func(children []tmpl.WikiTreeNode) {
					for _, c := range children {
						if c.Doc == nil || c.Doc.Slug == "" {
							continue
						}
						fmt.Fprintf(&sb, `<li><a class="list-link" href="/wiki/%s/%s"><span>%s</span></a></li>`,
							html.EscapeString(wikiProject), html.EscapeString(c.Doc.Slug), html.EscapeString(c.Doc.Title))
						renderChildren(c.Children)
					}
				}
				renderChildren(sec.Children)
				sb.WriteString(`</ul></section>`)
			}
			sb.WriteString(`</div>`)
		} else {
			sb.WriteString(`<div class="body fs14"><div class="body"><ul class="toc">`)
			for _, n := range nodes {
				if n.Doc == nil || n.Doc.Slug == "" {
					continue
				}
				fmt.Fprintf(&sb, `<li><a class="list-link" href="/wiki/%s/%s"><span>%s</span></a></li>`,
					html.EscapeString(wikiProject), html.EscapeString(n.Doc.Slug), html.EscapeString(n.Doc.Title))
			}
			sb.WriteString(`</ul></div></div>`)
		}

		sb.WriteString(`</details></div>`)
		return sb.String()
	}

	// 无 wiki 参数：普通文章目录外壳
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
	fmt.Fprintf(&sb, `<summary>%s</summary>`, html.EscapeString(title))
	sb.WriteString(`</details></div>`)
	return sb.String()
}

func renderTOCDiagnostic(title, msg, display, open string) string {
	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin toc"`)
	if display != "" {
		fmt.Fprintf(&sb, ` display="%s"`, html.EscapeString(display))
	}
	sb.WriteString(`><details class="toc"`)
	if open != "false" {
		sb.WriteString(` open`)
	}
	sb.WriteString(`>`)
	fmt.Fprintf(&sb, `<summary>%s</summary>`, html.EscapeString(title))
	fmt.Fprintf(&sb, `<div class="body fs14" style="color:var(--text-p3,#98989d);padding:0.5rem 1rem">%s</div>`, html.EscapeString(msg))
	sb.WriteString(`</details></div>`)
	return sb.String()
}
