package post

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
)

// note 标签正则：{% note [color:xxx] [title] content %}
var reNote = regexp.MustCompile(`{%\s*note\s+(.*?)\s*%}`)

// folding 标签正则（块级）：{% folding ... %} body {% endfolding %}
var reFolding = regexp.MustCompile(`(?s){%\s*folding\s+(.*?)\s*%}(.*?){%\s*endfolding\s*%}`)

// tabs 标签正则（块级）：{% tabs ... %} body {% endtabs %}
var reTabs = regexp.MustCompile(`(?s){%\s*tabs\s*(.*?)\s*%}(.*?){%\s*endtabs\s*%}`)

// timeline 标签正则（块级）：{% timeline ... %} body {% endtimeline %}
var reTimeline = regexp.MustCompile(`(?s){%\s*timeline\s*(.*?)\s*%}(.*?){%\s*endtimeline\s*%}`)

// gallery 标签正则（块级）：{% gallery ... %} body {% endgallery %}
var reGallery = regexp.MustCompile(`(?s){%\s*gallery\s*(.*?)\s*%}(.*?){%\s*endgallery\s*%}`)

// mermaid 标签正则（块级）：{% mermaid %} body {% endmermaid %}
var reMermaid = regexp.MustCompile(`(?s){%\s*mermaid\s*%}(.*?){%\s*endmermaid\s*%}`)

// button 标签正则：{% button ... %}
var reButton = regexp.MustCompile(`{%\s*button\s+(.*?)\s*%}`)

// checkbox 标签正则：{% checkbox ... %}
var reCheckbox = regexp.MustCompile(`{%\s*checkbox\s+(.*?)\s*%}`)

// quot 标签正则（单行）：{% quot ... %}
var reQuot = regexp.MustCompile(`{%\s*quot\s+(.*?)\s*%}`)

// okr 标签正则（块级）：{% okr ... %} body {% endokr %}
var reOKR = regexp.MustCompile(`(?s){%\s*okr\s+(.*?)\s*%}(.*?){%\s*endokr\s*%}`)

// okr 内部 KR 分隔符：<!-- okr krN ... -->
var reOKRItem = regexp.MustCompile(`<!--\s*okr\s+(.*?)\s*-->`)

// folders 标签正则（块级）：{% folders [color] %} body {% endfolders %}
var reFolders = regexp.MustCompile(`(?s){%\s*folders\s*(.*?)\s*%}(.*?){%\s*endfolders\s*%}`)

// grid 标签正则（块级）：{% grid ... %} body {% endgrid %}
var reGrid = regexp.MustCompile(`(?s){%\s*grid\s*(.*?)\s*%}(.*?){%\s*endgrid\s*%}`)

// box 标签正则（块级）：{% box ... %} body {% endbox %}
var reBox = regexp.MustCompile(`(?s){%\s*box\s+(.*?)\s*%}(.*?){%\s*endbox\s*%}`)

// link 标签正则（单行）：{% link url [title] [desc:...] [icon:...] %}
var reLink = regexp.MustCompile(`{%\s*link\s+(.*?)\s*%}`)

// image 标签正则（单行）：{% image src [alt] [width:...] [bg:...] [download:...] [fancybox:...] [ratio] %}
var reImage = regexp.MustCompile(`{%\s*image\s+(.*?)\s*%}`)

// mark 标签正则（单行）：{% mark text [color:...] %}
var reMark = regexp.MustCompile(`{%\s*mark\s+(.*?)\s*%}`)

// emoji 标签正则（单行）：{% emoji [source] name [height:...] %}
var reEmoji = regexp.MustCompile(`{%\s*emoji\s+(.*?)\s*%}`)

// icon 标签正则（单行）：{% icon key [color:...] [style:...] %}
var reIcon = regexp.MustCompile(`{%\s*icon\s+(.*?)\s*%}`)

// hashtag 标签正则（单行）：{% hashtag text [href] [color:...] %}
var reHashtag = regexp.MustCompile(`{%\s*hashtag\s+(.*?)\s*%}`)

// ghrepo 标签正则（单行）：{% ghrepo user/repo %}
var reGHRepo = regexp.MustCompile(`{%\s*ghrepo\s+(.*?)\s*%}`)

// ghuser 标签正则（单行）：{% ghuser username %}
var reGHUser = regexp.MustCompile(`{%\s*ghuser\s+(.*?)\s*%}`)

// video 标签正则（单行）：{% video src %}
var reVideo = regexp.MustCompile(`{%\s*video\s+(.*?)\s*%}`)

// audio 标签正则（单行）：{% audio src %}
var reAudio = regexp.MustCompile(`{%\s*audio\s+(.*?)\s*%}`)

// copy 标签正则（单行）：{% copy text %}
var reCopy = regexp.MustCompile(`{%\s*copy\s+(.*?)\s*%}`)

// rating 标签正则（单行）：{% rating id %}
var reRating = regexp.MustCompile(`{%\s*rating\s+(.*?)\s*%}`)

// ghcard 标签正则（单行）：{% ghcard user/repo %}
var reGHCard = regexp.MustCompile(`{%\s*ghcard\s+(.*?)\s*%}`)

// poetry 标签正则（块级）：{% poetry ... %} body {% endpoetry %}
var rePoetry = regexp.MustCompile(`(?s){%\s*poetry\s*(.*?)\s*%}(.*?){%\s*endpoetry\s*%}`)

// swiper 标签正则（块级）：{% swiper ... %} body {% endswiper %}
var reSwiper = regexp.MustCompile(`(?s){%\s*swiper\s*(.*?)\s*%}(.*?){%\s*endswiper\s*%}`)

// chat 标签正则（块级）：{% chat ... %} body {% endchat %}
var reChat = regexp.MustCompile(`(?s){%\s*chat\s+(.*?)\s*%}(.*?){%\s*endchat\s*%}`)

var (
	reTabSplit    = regexp.MustCompile(`<!--\s*tab (.*?)\s*-->`)
	reNodeSplit   = regexp.MustCompile(`<!--\s*node (.*?)\s*-->`)
	reFolderSplit = regexp.MustCompile(`<!--\s*folder (.*?)\s*-->`)
	reCellSplit   = regexp.MustCompile(`<!--\s*cell(.*?)-->`)
	reGalleryMD   = regexp.MustCompile(`!\[(.*?)\]\((.*?)\)`)

	// swiper 从渲染后的 Markdown 中提取 img 标签
	reSwiperImg = regexp.MustCompile(`(?i)<img(.*?)src="(.*?)"(.*?)>`)
	// chat 注释单元格：<!-- chattip ... --> / <!-- chatcell ... -->
	reChatComment = regexp.MustCompile(`(?s)<!--\s*(.*?)\s*-->`)
	// chat 单元格参数：user:/md:/tag:/quote:
	reChatParam = regexp.MustCompile(`(?:^|\s)(user|md|tag|quote):`)

	tabIndexCounter  uint64
	galleryIndex     uint64
	hashtagColorIdx  uint64
	copyIndex        uint64
)

// processShortcodes 在 goldmark 渲染前预处理 Stellar 标签插件，返回替换后的 Markdown。
// 块级 body 内的 Markdown 由调用者传入 renderMD 函数单独渲染。
func processShortcodes(md string, renderMD func(string) string, iconFunc func(string) string, ghClient GHClient) string {
	// 1. mermaid（块级，body 为原文，不渲染 Markdown）
	md = reMermaid.ReplaceAllStringFunc(md, func(match string) string {
		parts := reMermaid.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		body := strings.TrimSpace(parts[1])
		return fmt.Sprintf(`<div class="mermaid">%s</div>`, html.EscapeString(body))
	})

	// 2. folding（块级，body 需单独渲染 Markdown）
	md = reFolding.ReplaceAllStringFunc(md, func(match string) string {
		parts := reFolding.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		argsStr := parts[1]
		body := strings.TrimSpace(parts[2])

		color, open, title := parseFoldingArgs(argsStr)
		renderedBody := renderMD(body)

		var sb strings.Builder
		sb.WriteString(`<details class="tag-plugin colorful folding"`)
		if color != "" {
			fmt.Fprintf(&sb, ` color="%s"`, color)
		}
		if open {
			sb.WriteString(` open`)
		}
		sb.WriteString(`>`)
		if title != "" {
			fmt.Fprintf(&sb, `<summary>%s</summary>`, title)
		}
		fmt.Fprintf(&sb, `<div class="body">%s</div>`, renderedBody)
		sb.WriteString(`</details>`)
		return sb.String()
	})

	// 3. timeline（块级）
	md = reTimeline.ReplaceAllStringFunc(md, func(match string) string {
		parts := reTimeline.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		body := strings.TrimSpace(parts[2])
		return renderTimeline(body, renderMD)
	})

	// 4. gallery（块级）
	md = reGallery.ReplaceAllStringFunc(md, func(match string) string {
		parts := reGallery.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		argsStr := parts[1]
		body := strings.TrimSpace(parts[2])
		return renderGallery(argsStr, body)
	})

	// 5. tabs（块级）
	md = reTabs.ReplaceAllStringFunc(md, func(match string) string {
		parts := reTabs.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		argsStr := parts[1]
		body := strings.TrimSpace(parts[2])
		return renderTabs(argsStr, body, renderMD)
	})

	// 5.5 okr（块级，KR 用 <!-- okr krN --> 分隔，note 需渲染 Markdown）
	md = reOKR.ReplaceAllStringFunc(md, func(match string) string {
		parts := reOKR.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return renderOKR(parts[1], parts[2], renderMD)
	})

	// 5.6 folders（块级，<!-- folder title --> 分隔）
	md = reFolders.ReplaceAllStringFunc(md, func(match string) string {
		parts := reFolders.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return renderFolders(parts[1], parts[2], renderMD)
	})

	// 5.7 grid（块级，<!-- cell --> 分隔）
	md = reGrid.ReplaceAllStringFunc(md, func(match string) string {
		parts := reGrid.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return renderGrid(parts[1], parts[2], renderMD)
	})

	// 5.8 box（块级）
	md = reBox.ReplaceAllStringFunc(md, func(match string) string {
		parts := reBox.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return renderBox(parts[1], parts[2], renderMD)
	})

	// 6. button（单行）
	md = reButton.ReplaceAllStringFunc(md, func(match string) string {
		parts := reButton.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		el, ok := renderButton(parts[1])
		if !ok {
			return match
		}
		return el
	})

	// 7. checkbox（单行）
	md = reCheckbox.ReplaceAllStringFunc(md, func(match string) string {
		parts := reCheckbox.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		el, ok := renderCheckbox(parts[1])
		if !ok {
			return match
		}
		return el
	})

	// 8. note（单行）
	md = reNote.ReplaceAllStringFunc(md, func(match string) string {
		parts := reNote.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		argsStr := parts[1]
		color, title, content := parseNoteArgs(argsStr)

		var sb strings.Builder
		sb.WriteString(`<div class="tag-plugin colorful note"`)
		if color != "" {
			fmt.Fprintf(&sb, ` color="%s"`, color)
		}
		sb.WriteString(`>`)
		if title != "" {
			fmt.Fprintf(&sb, `<div class="title">%s</div>`, title)
		}
		fmt.Fprintf(&sb, `<div class="body">%s</div>`, content)
		sb.WriteString(`</div>`)
		return sb.String()
	})

	// 9. quot（单行，小标题，需 iconFunc 渲染前缀图标）
	md = reQuot.ReplaceAllStringFunc(md, func(match string) string {
		parts := reQuot.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return renderQuot(parts[1], iconFunc)
	})

	// 10. link（单行，链接卡片）
	md = reLink.ReplaceAllStringFunc(md, func(match string) string {
		parts := reLink.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		el, ok := renderLink(parts[1])
		if !ok {
			return match
		}
		return el
	})

	// 11. image（单行，增强图片）
	md = reImage.ReplaceAllStringFunc(md, func(match string) string {
		parts := reImage.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		el, ok := renderImage(parts[1])
		if !ok {
			return match
		}
		return el
	})

	// 12. mark（单行，荧光标记）
	md = reMark.ReplaceAllStringFunc(md, func(match string) string {
		parts := reMark.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		el, ok := renderMark(parts[1])
		if !ok {
			return match
		}
		return el
	})

	// 13. emoji（单行，表情图）
	md = reEmoji.ReplaceAllStringFunc(md, func(match string) string {
		parts := reEmoji.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		el, ok := renderEmoji(parts[1])
		if !ok {
			return match
		}
		return el
	})

	// 14. icon（单行，内联图标）
	md = reIcon.ReplaceAllStringFunc(md, func(match string) string {
		parts := reIcon.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		el, ok := renderIcon(parts[1], iconFunc)
		if !ok {
			return match
		}
		return el
	})

	// 15. hashtag（单行，话题标签）
	md = reHashtag.ReplaceAllStringFunc(md, func(match string) string {
		parts := reHashtag.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		el, ok := renderHashtag(parts[1])
		if !ok {
			return match
		}
		return el
	})

	// 16. ghrepo（单行，GitHub 仓库卡片）
	md = reGHRepo.ReplaceAllStringFunc(md, func(match string) string {
		parts := reGHRepo.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return renderGHRepo(parts[1], ghClient)
	})

	// 17. ghuser（单行，GitHub 用户卡片）
	md = reGHUser.ReplaceAllStringFunc(md, func(match string) string {
		parts := reGHUser.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return renderGHUser(parts[1], ghClient)
	})

	// 18. video（单行，视频播放器）
	md = reVideo.ReplaceAllStringFunc(md, func(match string) string {
		parts := reVideo.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return renderVideo(parts[1])
	})

	// 19. audio（单行，音频播放器）
	md = reAudio.ReplaceAllStringFunc(md, func(match string) string {
		parts := reAudio.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return renderAudio(parts[1])
	})

	// 20. copy（单行，复制按钮）
	md = reCopy.ReplaceAllStringFunc(md, func(match string) string {
		parts := reCopy.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return renderCopy(parts[1])
	})

	// 21. rating（单行，评分组件）
	md = reRating.ReplaceAllStringFunc(md, func(match string) string {
		parts := reRating.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return renderRating(parts[1], iconFunc)
	})

	// 22. ghcard（单行，GitHub 卡片；复用原生 ghrepo/ghuser 渲染，自托管无第三方依赖）
	md = reGHCard.ReplaceAllStringFunc(md, func(match string) string {
		parts := reGHCard.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return renderGHCard(parts[1], ghClient)
	})

	// 23. poetry（块级，诗词）
	md = rePoetry.ReplaceAllStringFunc(md, func(match string) string {
		parts := rePoetry.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return renderPoetry(parts[1], strings.TrimSpace(parts[2]), renderMD)
	})

	// 24. swiper（块级，图片轮播）
	md = reSwiper.ReplaceAllStringFunc(md, func(match string) string {
		parts := reSwiper.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return renderSwiper(parts[1], strings.TrimSpace(parts[2]), renderMD)
	})

	// 25. chat（块级，聊天消息）
	md = reChat.ReplaceAllStringFunc(md, func(match string) string {
		parts := reChat.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return renderChat(parts[1], parts[2], renderMD)
	})

	return md
}

func renderTabs(argsStr, body string, renderMD func(string) string) string {
	pairs := splitByMarker(reTabSplit, body)
	if len(pairs) == 0 {
		return ""
	}

	named, _ := parseNamedPositionalArgs(argsStr, []string{"active", "align"}, nil)
	active := 0
	if v, ok := named["active"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			active = n
		}
	}

	tabIdx := atomic.AddUint64(&tabIndexCounter, 1)
	tabName := fmt.Sprintf("tab_%d", tabIdx)
	containerID := strings.ReplaceAll(strings.ToLower(tabName), " ", "-")

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin tabs"`)
	if align := named["align"]; align != "" {
		fmt.Fprintf(&sb, ` align="%s"`, align)
	}
	fmt.Fprintf(&sb, ` id="%s">`, containerID)

	sb.WriteString(`<div class="nav-tabs">`)
	tabID := 0
	for _, pair := range pairs {
		tabID++
		abbr := fmt.Sprintf("%s %d", tabName, tabID)
		href := strings.ReplaceAll(strings.ToLower(abbr), " ", "-")
		isActive := (active > 0 && active == tabID) || (active == 0 && tabID == 1)
		header := pair.header
		if strings.TrimSpace(header) == "" {
			header = abbr
		}
		if isActive {
			sb.WriteString(`<div class="tab active">`)
		} else {
			sb.WriteString(`<div class="tab">`)
		}
		fmt.Fprintf(&sb, `<a href="#%s">%s</a></div>`, href, header)
	}
	sb.WriteString(`</div>`)

	sb.WriteString(`<div class="tab-content">`)
	tabID = 0
	for _, pair := range pairs {
		tabID++
		abbr := fmt.Sprintf("%s %d", tabName, tabID)
		href := strings.ReplaceAll(strings.ToLower(abbr), " ", "-")
		isActive := (active > 0 && active == tabID) || (active == 0 && tabID == 1)
		content := strings.TrimSpace(renderMD(pair.body))
		if isActive {
			fmt.Fprintf(&sb, `<div class="tab-pane active" id="%s">%s</div>`, href, content)
		} else {
			fmt.Fprintf(&sb, `<div class="tab-pane" id="%s">%s</div>`, href, content)
		}
	}
	sb.WriteString(`</div></div>`)
	return sb.String()
}

func renderTimeline(body string, renderMD func(string) string) string {
	pairs := splitByMarker(reNodeSplit, body)

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin timeline">`)
	for i, pair := range pairs {
		fmt.Fprintf(&sb, `<div class="timenode" index="%d">`, i)
		sb.WriteString(`<div class="header">`)
		if h := strings.TrimSpace(pair.header); h != "" {
			fmt.Fprintf(&sb, `<span>%s</span>`, h)
		}
		sb.WriteString(`</div>`)
		rendered := strings.TrimSpace(renderMD(pair.body))
		fmt.Fprintf(&sb, `<div class="body fs14">%s</div>`, rendered)
		sb.WriteString(`</div>`)
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

func renderGallery(argsStr, body string) string {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"layout", "size", "ratio"}, nil)

	layout := named["layout"]
	if layout == "" {
		layout = "grid"
	}
	layoutType := "grid"
	if layout == "flow" {
		layoutType = "flow"
	}

	size := named["size"]
	if size == "" {
		size = "mix"
	}
	ratio := named["ratio"]
	if ratio == "" {
		ratio = "origin"
	}

	idx := atomic.AddUint64(&galleryIndex, 1)

	var sb strings.Builder
	fmt.Fprintf(&sb, `<div class="tag-plugin gallery %s-box" size="%s" ratio="%s">`, layoutType, size, ratio)

	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := reGalleryMD.FindStringSubmatch(line)
		if len(m) < 3 {
			continue
		}
		alt, src := m[1], m[2]
		fmt.Fprintf(&sb, `<div class="%s-cell lazy-box">`, layoutType)
		fmt.Fprintf(&sb, `<img class="lazy" data-fancybox="gallery-%d" data-src="%s"`, idx, html.EscapeString(src))
		if alt != "" {
			fmt.Fprintf(&sb, ` alt="%s"`, html.EscapeString(alt))
		}
		sb.WriteString(`/>`)
		sb.WriteString(`<div class="lazy-icon"></div>`)
		sb.WriteString(`<div class="image-meta">`)
		if alt != "" {
			fmt.Fprintf(&sb, `<span class="image-caption">%s</span>`, html.EscapeString(alt))
		}
		sb.WriteString(`</div></div>`)
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

func renderButton(argsStr string) (string, bool) {
	named, positional := parseNamedPositionalArgs(argsStr, []string{"color", "icon", "size"}, []string{"text", "url"})
	text := named["text"]
	if text == "" && len(positional) > 0 {
		text = positional[0]
	}
	url := named["url"]
	if url == "" && len(positional) > 1 {
		url = positional[1]
	}
	if text == "" || url == "" {
		return "", false
	}

	// Stellar：url 含空格时，最后一段为 url，前面拼回 text
	if strings.Contains(url, " ") {
		parts := strings.Fields(url)
		url = parts[len(parts)-1]
		text = text + " " + strings.Join(parts[:len(parts)-1], " ")
	}

	var sb strings.Builder
	sb.WriteString(`<a class="tag-plugin colorful button"`)
	if color := named["color"]; color != "" {
		fmt.Fprintf(&sb, ` color="%s"`, color)
	}
	if size := named["size"]; size != "" {
		fmt.Fprintf(&sb, ` size="%s"`, size)
	}
	fmt.Fprintf(&sb, ` title="%s" href="%s">`, html.EscapeString(text), html.EscapeString(url))
	// icon 暂不渲染 SVG（Stellar 依赖 ctx.utils.icon）
	fmt.Fprintf(&sb, `<span>%s</span></a>`, text)
	return sb.String(), true
}

func renderCheckbox(argsStr string) (string, bool) {
	named, positional := parseNamedPositionalArgs(argsStr, []string{"color", "checked", "symbol"}, []string{"text"})
	text := named["text"]
	checked := named["checked"] == "true"
	var textParts []string
	for _, tok := range positional {
		if tok == "checked" {
			checked = true
			continue
		}
		textParts = append(textParts, tok)
	}
	if text == "" && len(textParts) > 0 {
		text = strings.Join(textParts, " ")
	}
	if text == "" {
		return "", false
	}

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin colorful checkbox"`)
	if color := named["color"]; color != "" {
		fmt.Fprintf(&sb, ` color="%s"`, color)
	}
	if symbol := named["symbol"]; symbol != "" {
		fmt.Fprintf(&sb, ` symbol="%s"`, symbol)
	}
	sb.WriteString(`>`)
	sb.WriteString(`<input type="checkbox"`)
	if checked {
		sb.WriteString(` checked="true"`)
	}
	fmt.Fprintf(&sb, `/><span>%s</span></div>`, text)
	return sb.String(), true
}

type markerPair struct {
	header string
	body   string
}

// splitByMarker 按 <!-- marker xxx --> 注释切分内容为 header/body 对（对齐 Stellar tabs.js / timeline.js）。
func splitByMarker(re *regexp.Regexp, content string) []markerPair {
	indices := re.FindAllStringSubmatchIndex(content, -1)
	if len(indices) == 0 {
		return nil
	}

	pairs := make([]markerPair, 0, len(indices))
	for i, loc := range indices {
		header := content[loc[2]:loc[3]]
		bodyStart := loc[1]
		bodyEnd := len(content)
		if i+1 < len(indices) {
			bodyEnd = indices[i+1][0]
		}
		pairs = append(pairs, markerPair{
			header: header,
			body:   content[bodyStart:bodyEnd],
		})
	}
	return pairs
}

// parseNamedPositionalArgs 解析 key:value 命名参数与位置参数（对齐 Stellar ctx.args.map）。
func parseNamedPositionalArgs(args string, namedKeys, positionalKeys []string) (map[string]string, []string) {
	namedSet := make(map[string]struct{}, len(namedKeys))
	for _, k := range namedKeys {
		namedSet[k] = struct{}{}
	}

	result := make(map[string]string)
	var positional []string

	for _, tok := range splitArgs(args) {
		isNamed := false
		for k := range namedSet {
			if strings.HasPrefix(tok, k+":") {
				result[k] = strings.TrimPrefix(tok, k+":")
				isNamed = true
				break
			}
		}
		if !isNamed {
			positional = append(positional, tok)
		}
	}

	for i, k := range positionalKeys {
		if i < len(positional) {
			result[k] = positional[i]
		}
	}
	return result, positional
}

// parseFoldingArgs 解析 folding 开标签参数：[color:xxx] [open:true/false] title
func parseFoldingArgs(args string) (color string, open bool, title string) {
	args = strings.TrimSpace(args)
	var remaining []string

	for _, tok := range splitArgs(args) {
		if strings.HasPrefix(tok, "color:") {
			color = strings.TrimPrefix(tok, "color:")
		} else if strings.HasPrefix(tok, "open:") {
			open = strings.TrimPrefix(tok, "open:") == "true"
		} else {
			remaining = append(remaining, tok)
		}
	}

	title = strings.Join(remaining, " ")
	return
}

// parseNoteArgs 解析 note 标签参数：[color:xxx] [title] content
func parseNoteArgs(args string) (color, title, content string) {
	args = strings.TrimSpace(args)
	var remaining []string

	for _, tok := range splitArgs(args) {
		if strings.HasPrefix(tok, "color:") {
			color = strings.TrimPrefix(tok, "color:")
		} else {
			remaining = append(remaining, tok)
		}
	}

	switch len(remaining) {
	case 0:
	case 1:
		content = remaining[0]
	default:
		title = remaining[0]
		content = strings.Join(remaining[1:], " ")
	}
	return
}

// splitArgs 按空白切分参数，但保留引号内的空格。
func splitArgs(s string) []string {
	var tokens []string
	var cur strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote {
			if c == quoteChar {
				inQuote = false
			} else {
				cur.WriteByte(c)
			}
		} else if c == '"' || c == '\'' {
			inQuote = true
			quoteChar = c
		} else if c == ' ' || c == '\t' {
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		} else {
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

// okrStatus okr 状态 → 配色 + 文案。
type okrStatus struct {
	color string
	label string
}

// okrStatusMap 内置状态映射，对齐 Stellar tag_plugins.okr.status 默认值。
var okrStatusMap = map[string]okrStatus{
	"in_track":   {"blue", "正常"},
	"at_risk":    {"yellow", "风险"},
	"off_track":  {"red", "滞后"},
	"finished":   {"green", "完成"},
	"unfinished": {"gray", "未完成"},
}

// parseOKRMeta 解析 "o1 percent:50 status:in_track color:blue"：首 token 为 index，其余为命名参数。
func parseOKRMeta(s string) (index string, percent int, hasPercent bool, status, color string) {
	for i, tok := range splitArgs(s) {
		if i == 0 {
			index = tok
			continue
		}
		switch {
		case strings.HasPrefix(tok, "percent:"):
			if n, err := strconv.Atoi(strings.TrimPrefix(tok, "percent:")); err == nil {
				percent = n
				hasPercent = true
			}
		case strings.HasPrefix(tok, "status:"):
			status = strings.TrimPrefix(tok, "status:")
		case strings.HasPrefix(tok, "color:"):
			color = strings.TrimPrefix(tok, "color:")
		}
	}
	return
}

// okrSplitTitleNote 首行非空为 title，其余为 note。
func okrSplitTitleNote(s string) (title, note string) {
	var lines []string
	for _, ln := range strings.Split(strings.TrimSpace(s), "\n") {
		if strings.TrimSpace(ln) != "" {
			lines = append(lines, strings.TrimSpace(ln))
		}
	}
	if len(lines) == 0 {
		return "", ""
	}
	return lines[0], strings.Join(lines[1:], "\n")
}

// okrResolveStatus 给定 status/percent 解析出配色与文案；status 缺省时按 percent 推断。
func okrResolveStatus(status string, percent int) okrStatus {
	if st, ok := okrStatusMap[status]; ok {
		return st
	}
	if percent >= 100 {
		return okrStatusMap["finished"]
	}
	return okrStatusMap["in_track"]
}

// okrLayoutItem 生成单个 okr-item（o 或 kr），对齐 Stellar okr.js 的 layoutItem。
func okrLayoutItem(typ, index, title, note string, st okrStatus, percent int, renderMD func(string) string) string {
	noteHTML := strings.TrimSpace(renderMD(note))
	var sb strings.Builder
	fmt.Fprintf(&sb, `<div class="okr-item %s" id="okr-%s">`, typ, strings.ToLower(index))
	fmt.Fprintf(&sb, `<div class="okr-left"><span class="title">%s</span></div>`, strings.ToUpper(index))
	fmt.Fprintf(&sb, `<div class="okr-center"><span class="title">%s</span><div class="note">%s</div></div>`, title, noteHTML)
	fmt.Fprintf(&sb, `<div class="okr-right colorful" color="%s"><div class="labels"><span class="status label">%s</span><span class="status percent">%d%%</span></div><div class="progress"><div class="fill" style="width:%d%%;"></div></div></div>`, st.color, st.label, percent, percent)
	sb.WriteString(`</div>`)
	return sb.String()
}

// renderOKR 渲染 {% okr %}...{% endokr %} 为 .tag-plugin.colorful.okr。
func renderOKR(oArgs, content string, renderMD func(string) string) string {
	oIndex, oPercent, oHasPercent, oStatus, oColor := parseOKRMeta(oArgs)

	type krItem struct{ meta, body string }
	var krs []krItem
	oBody := content
	if locs := reOKRItem.FindAllStringSubmatchIndex(content, -1); len(locs) > 0 {
		oBody = content[:locs[0][0]]
		for i, loc := range locs {
			meta := content[loc[2]:loc[3]]
			bodyStart := loc[1]
			bodyEnd := len(content)
			if i+1 < len(locs) {
				bodyEnd = locs[i+1][0]
			}
			krs = append(krs, krItem{meta: meta, body: content[bodyStart:bodyEnd]})
		}
	}

	var krHTML strings.Builder
	sum, cnt := 0, 0
	for _, kr := range krs {
		ki, kp, _, ks, _ := parseOKRMeta(kr.meta)
		kt, kn := okrSplitTitleNote(kr.body)
		sum += kp
		cnt++
		krHTML.WriteString(okrLayoutItem("kr", ki, kt, kn, okrResolveStatus(ks, kp), kp, renderMD))
	}
	if !oHasPercent && cnt > 0 {
		oPercent = sum / cnt
	}
	oTitle, oNote := okrSplitTitleNote(oBody)

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin colorful okr"`)
	if oColor != "" {
		fmt.Fprintf(&sb, ` color="%s"`, oColor)
	}
	sb.WriteString(`>`)
	sb.WriteString(okrLayoutItem("o", oIndex, oTitle, oNote, okrResolveStatus(oStatus, oPercent), oPercent, renderMD))
	sb.WriteString(krHTML.String())
	sb.WriteString(`</div>`)
	return sb.String()
}

// renderQuot 渲染 {% quot el:h2 icon:hashtag text:... %} 为 .tag-plugin.quot 小标题。
func renderQuot(argsStr string, iconFunc func(string) string) string {
	named, positional := parseNamedPositionalArgs(argsStr, []string{"el", "icon", "prefix", "suffix", "text"}, nil)
	el := named["el"]
	if el == "" {
		el = "p"
	}
	text := named["text"]
	if text == "" {
		text = strings.Join(positional, " ")
	}
	// 前缀图标：prefix > icon 预设 > 默认 hashtag
	prefixKey := named["prefix"]
	if prefixKey == "" {
		if ic := named["icon"]; ic == "" || ic == "default" || ic == "hashtag" {
			prefixKey = "solar:hashtag-square-bold"
		} else {
			prefixKey = ic
		}
	}
	suffixKey := named["suffix"]

	iconHTML := func(key string) string {
		if key == "" || iconFunc == nil {
			return `<span class="empty"></span>`
		}
		return iconFunc(key)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, `<div class="tag-plugin quot %s">`, el)
	if strings.Contains(el, "h") {
		fmt.Fprintf(&sb, `<%s class="content" id="%s">`, el, text)
	} else {
		fmt.Fprintf(&sb, `<%s class="content">`, el)
	}
	sb.WriteString(iconHTML(prefixKey))
	fmt.Fprintf(&sb, `<span class="text">%s</span>`, text)
	sb.WriteString(iconHTML(suffixKey))
	fmt.Fprintf(&sb, `</%s></div>`, el)
	return sb.String()
}

// ---------- Spec 16: 标签插件二期（link/image/folders/grid/box/mark/emoji/icon/hashtag） ----------

// renderLink 渲染 {% link url [title] [desc:...] [icon:...] %} 为 .tag-plugin.link 卡片。
func renderLink(argsStr string) (string, bool) {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"icon", "desc"}, []string{"url", "title"})
	url := named["url"]
	if url == "" {
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

	var style strings.Builder
	if w := named["width"]; w != "" {
		style.WriteString("width:" + w + ";")
	}
	if h := named["height"]; h != "" {
		style.WriteString("height:" + h + ";")
	}

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin image">`)
	// image-bg
	sb.WriteString(`<div class="image-bg"`)
	bgStyle := ""
	if bg := named["bg"]; bg != "" {
		bgStyle += "background:" + bg + ";"
	}
	if p := named["padding"]; p != "" {
		bgStyle += "padding:" + p + ";"
	}
	if r := named["ratio"]; r != "" {
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
		sb.WriteString(`<a class="image-download blur" style="opacity:0" target="_blank"`)
		if alt != "" {
			sb.WriteString(` download="` + html.EscapeString(alt) + `"`)
		}
		sb.WriteString(` href="` + html.EscapeString(href) + `"><svg class="icon" style="width: 1em; height: 1em;vertical-align: middle;fill: currentColor;overflow: hidden;" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg"><path d="M561.00682908 685.55838913a111.03077546 111.03077546 0 0 1-106.8895062 0L256.23182837 487.72885783a55.96309219 55.96309219 0 0 1 79.13181253-79.18777574L450.70357448 523.88101491V181.55477937a55.96309219 55.96309219 0 0 1 111.92618438 0v344.06109173l117.07478902-117.07478901a55.96309219 55.96309219 0 0 1 79.13181252 79.18777574zM282.81429711 797.1487951h447.70473912a55.96309219 55.96309219 0 0 1 0 111.92618438H282.81429711a55.96309219 55.96309219 0 0 1 0-111.92618438z"></path></svg></a>`)
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
	if c := named["color"]; c != "" {
		fmt.Fprintf(&sb, ` color="%s"`, c)
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
	w := named["w"]
	c := named["c"]
	if w == "" && c == "" {
		w = "240px"
	}

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin grid"`)
	if bg := named["bg"]; bg != "" {
		fmt.Fprintf(&sb, ` bg="%s"`, bg)
	}
	sb.WriteString(` style="`)
	if w != "" {
		fmt.Fprintf(&sb, `grid-template-columns: repeat(auto-fill, minmax(%s, 1fr));`, w)
	} else if c != "" {
		fmt.Fprintf(&sb, `grid-template-columns: repeat(%s, 1fr);`, c)
	}
	if gap := named["gap"]; gap != "" {
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
		if br := named["br"]; br != "" {
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
	if c := named["color"]; c != "" {
		fmt.Fprintf(&sb, ` color="%s"`, c)
	}
	if ch := named["child"]; ch != "" {
		fmt.Fprintf(&sb, ` child="%s"`, ch)
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
func renderMark(argsStr string) (string, bool) {
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
		color = "yellow"
	}
	return fmt.Sprintf(`<mark class="tag-plugin colorful mark" color="%s">%s</mark>`, color, text), true
}

// emojiSources 内置 emoji CDN 映射，对齐原版 _config.yml tag_plugins.emoji。
var emojiSources = map[string]string{
	"default": "https://gcore.jsdelivr.net/gh/cdn-x/emoticons@3.1/qq/{name}.gif",
	"twemoji": "https://gcore.jsdelivr.net/gh/twitter/twemoji/assets/svg/{name}.svg",
}

// renderEmoji 渲染 {% emoji [source] name [height:...] %}。
func renderEmoji(argsStr string) (string, bool) {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"height"}, []string{"source", "name"})
	source := named["source"]
	name := named["name"]
	height := named["height"]

	if source == "" {
		return "", false
	}
	// 省略 source 时，第一个位置参数实际是 name，用默认源
	if name == "" {
		name = source
		source = "default"
	}
	tpl, ok := emojiSources[source]
	if !ok {
		return "", false
	}
	url := strings.Replace(tpl, "{name}", name, 1)

	var sb strings.Builder
	sb.WriteString(`<span class="tag-plugin emoji">`)
	sb.WriteString(`<img no-lazy="" class="inline" src="` + html.EscapeString(url) + `"`)
	if height != "" {
		sb.WriteString(` style="height:` + height + `"`)
	}
	sb.WriteString(`/>`)
	sb.WriteString(`</span>`)
	return sb.String(), true
}

// renderIcon 渲染 {% icon key [color:...] [style:...] %}。
func renderIcon(argsStr string, iconFunc func(string) string) (string, bool) {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"color", "style"}, []string{"key", "text"})
	key := named["key"]
	if key == "" {
		return "", false
	}
	color := named["color"]
	if color == "" {
		color = "blue"
	}
	text := named["text"]

	var sb strings.Builder
	if text != "" {
		sb.WriteString(`<div class="tag-plugin icon-wrap">`)
	}
	fmt.Fprintf(&sb, `<span class="tag-plugin icon colorful" color="%s">`, color)
	if iconFunc != nil {
		sb.WriteString(iconFunc(key))
	}
	sb.WriteString(`</span>`)
	if text != "" {
		fmt.Fprintf(&sb, `<span class="text">%s</span></div>`, text)
	}
	return sb.String(), true
}

// hashtagColors 话题标签循环色板，对齐原版 hashtag.js。
var hashtagColors = []string{"red", "orange", "yellow", "green", "cyan", "blue", "purple"}

// hashtagSVG 话题标签图标，对齐原版 hashtag.js 内联 SVG。
const hashtagSVG = `<svg t="1701408144765" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" width="200" height="200"><path d="M426.6 64.8c34.8 5.8 58.4 38.8 52.6 73.6l-19.6 117.6h190.2l23-138.6c5.8-34.8 38.8-58.4 73.6-52.6s58.4 38.8 52.6 73.6l-19.4 117.6H896c35.4 0 64 28.6 64 64s-28.6 64-64 64h-137.8l-42.6 256H832c35.4 0 64 28.6 64 64s-28.6 64-64 64h-137.8l-23 138.6c-5.8 34.8-38.8 58.4-73.6 52.6s-58.4-38.8-52.6-73.6l19.6-117.4h-190.4l-23 138.6c-5.8 34.8-38.8 58.4-73.6 52.6s-58.4-38.8-52.6-73.6l19.4-117.8H128c-35.4 0-64-28.6-64-64s28.6-64 64-64h137.8l42.6-256H192c-35.4 0-64-28.6-64-64s28.6-64 64-64h137.8l23-138.6c5.8-34.8 38.8-58.4 73.6-52.6z m11.6 319.2l-42.6 256h190.2l42.6-256h-190.2z"></path></svg>`

// renderHashtag 渲染 {% hashtag text [href] [color:...] %}。
func renderHashtag(argsStr string) (string, bool) {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"color"}, []string{"text", "href"})
	text := named["text"]
	if text == "" {
		return "", false
	}
	href := named["href"]
	color := named["color"]
	if color == "" {
		idx := atomic.AddUint64(&hashtagColorIdx, 1) - 1
		color = hashtagColors[idx%uint64(len(hashtagColors))]
	}

	var sb strings.Builder
	sb.WriteString(`<a class="tag-plugin colorful hashtag"`)
	fmt.Fprintf(&sb, ` color="%s"`, color)
	if href != "" {
		fmt.Fprintf(&sb, ` href="%s"`, html.EscapeString(href))
	}
	sb.WriteString(`>`)
	sb.WriteString(hashtagSVG)
	fmt.Fprintf(&sb, `<span>%s</span>`, text)
	sb.WriteString(`</a>`)
	return sb.String(), true
}

// ---------- Spec 21: GitHub widgets（ghrepo/ghuser 卡片） ----------

// languageColors 常见编程语言色点颜色，对齐 GitHub linguist。
var languageColors = map[string]string{
	"Go": "#00ADD8", "Python": "#3572A5", "JavaScript": "#F1E05A", "TypeScript": "#3178C6",
	"Java": "#B07219", "C": "#555555", "C++": "#F34B7D", "C#": "#178600",
	"Rust": "#DEA584", "Ruby": "#701516", "PHP": "#4F5D95", "Swift": "#F05138",
	"Kotlin": "#A97BFF", "Dart": "#00B4AB", "Shell": "#89E051", "HTML": "#E34C26",
	"CSS": "#563D7C", "Vue": "#41B883", "Lua": "#000080", "Zig": "#EC915C",
}

// renderGHRepo 渲染 {% ghrepo user/repo %} 为静态仓库卡片。API 失败时降级为纯链接。
func renderGHRepo(argsStr string, ghClient GHClient) string {
	repoPath := strings.TrimSpace(argsStr)
	if repoPath == "" {
		return ""
	}
	// 支持 "user/repo" 或完整 URL
	repoPath = strings.TrimPrefix(repoPath, "https://github.com/")
	parts := strings.SplitN(repoPath, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Sprintf(`<a href="https://github.com/%s">%s</a>`, html.EscapeString(repoPath), html.EscapeString(repoPath))
	}
	owner, repo := parts[0], parts[1]
	fullURL := "https://github.com/" + owner + "/" + repo

	if ghClient == nil {
		return ghRepoFallback(fullURL, owner+"/"+repo)
	}
	info, err := ghClient.GetRepo(owner, repo)
	if err != nil {
		return ghRepoFallback(fullURL, owner+"/"+repo)
	}

	langDot := ""
	if info.Language != "" {
		color := languageColors[info.Language]
		if color == "" {
			color = "#8b8b8b"
		}
		langDot = fmt.Sprintf(`<span class="lang-dot" style="background:%s"></span><span>%s</span>`, color, html.EscapeString(info.Language))
	}

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin ghcard"><a class="ghcard" href="` + html.EscapeString(fullURL) + `" target="_blank" rel="external nofollow noopener noreferrer">`)
	sb.WriteString(`<div class="ghrepo-card">`)
	// 头部：头像 + 全名
	sb.WriteString(`<div class="repo-header">`)
	if info.OwnerAvatar != "" {
		sb.WriteString(`<img class="repo-avatar" src="` + html.EscapeString(info.OwnerAvatar) + `&s=40" alt=""/>`)
	}
	sb.WriteString(`<span class="repo-name">` + html.EscapeString(info.FullName) + `</span>`)
	sb.WriteString(`</div>`)
	// 描述
	if info.Description != "" {
		sb.WriteString(`<div class="repo-desc">` + html.EscapeString(info.Description) + `</div>`)
	}
	// 统计
	sb.WriteString(`<div class="repo-stats">`)
	sb.WriteString(fmt.Sprintf(`<span>⭐ %d</span>`, info.Stars))
	sb.WriteString(fmt.Sprintf(`<span>🍴 %d</span>`, info.Forks))
	if langDot != "" {
		sb.WriteString(`<span class="repo-lang">` + langDot + `</span>`)
	}
	if info.License != "" {
		sb.WriteString(`<span>📄 ` + html.EscapeString(info.License) + `</span>`)
	}
	sb.WriteString(`</div>`)
	sb.WriteString(`</div></a></div>`)
	return sb.String()
}

// ghRepoFallback API 失败时的纯链接降级。
func ghRepoFallback(url, name string) string {
	return fmt.Sprintf(`<div class="tag-plugin ghcard"><a class="ghcard" href="%s" target="_blank" rel="external nofollow noopener noreferrer"><div class="ghrepo-card"><div class="repo-header"><span class="repo-name">%s</span></div></div></a></div>`,
		html.EscapeString(url), html.EscapeString(name))
}

// renderGHUser 渲染 {% ghuser username %} 为静态用户卡片。API 失败时降级为纯链接。
func renderGHUser(argsStr string, ghClient GHClient) string {
	username := strings.TrimSpace(argsStr)
	if username == "" {
		return ""
	}
	username = strings.TrimPrefix(username, "https://github.com/")
	username = strings.TrimPrefix(username, "@")
	fullURL := "https://github.com/" + username

	if ghClient == nil {
		return ghUserFallback(fullURL, username)
	}
	info, err := ghClient.GetUser(username)
	if err != nil {
		return ghUserFallback(fullURL, username)
	}

	displayName := info.Name
	if displayName == "" {
		displayName = info.Login
	}

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin ghcard"><a class="ghcard" href="` + html.EscapeString(fullURL) + `" target="_blank" rel="external nofollow noopener noreferrer">`)
	sb.WriteString(`<div class="ghuser-card">`)
	// 头像 + 名称
	sb.WriteString(`<div class="user-header">`)
	if info.AvatarURL != "" {
		sb.WriteString(`<img class="user-avatar" src="` + html.EscapeString(info.AvatarURL) + `&s=64" alt=""/>`)
	}
	sb.WriteString(`<div class="user-info">`)
	sb.WriteString(`<span class="user-name">` + html.EscapeString(displayName) + `</span>`)
	sb.WriteString(`<span class="user-login">@` + html.EscapeString(info.Login) + `</span>`)
	sb.WriteString(`</div></div>`)
	// Bio
	if info.Bio != "" {
		sb.WriteString(`<div class="user-bio">` + html.EscapeString(info.Bio) + `</div>`)
	}
	// 统计
	sb.WriteString(`<div class="user-stats">`)
	sb.WriteString(fmt.Sprintf(`<span>%d followers</span>`, info.Followers))
	sb.WriteString(fmt.Sprintf(`<span>%d repos</span>`, info.Repos))
	sb.WriteString(`</div>`)
	sb.WriteString(`</div></a></div>`)
	return sb.String()
}

// ghUserFallback API 失败时的纯链接降级。
func ghUserFallback(url, username string) string {
	return fmt.Sprintf(`<div class="tag-plugin ghcard"><a class="ghcard" href="%s" target="_blank" rel="external nofollow noopener noreferrer"><div class="ghuser-card"><div class="user-header"><div class="user-info"><span class="user-name">@%s</span></div></div></div></a></div>`,
		html.EscapeString(url), html.EscapeString(username))
}

// ---------- Spec 20: 标签插件三期（video/audio/copy/poetry/rating/swiper/chat/ghcard） ----------

// renderVideo 渲染 {% video src %}，支持 bilibili/youtube 嵌入与本地视频。
func renderVideo(argsStr string) string {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"type", "bilibili", "youtube", "ratio", "width", "autoplay"}, []string{"src"})
	width := named["width"]
	if width == "" {
		width = "100%"
	}
	ratio := named["ratio"]
	if ratio == "" {
		ratio = "1.7777777777777777" // 16/9
	}
	if bv := named["bilibili"]; bv != "" {
		autoplay := named["autoplay"]
		if autoplay == "" {
			autoplay = "false"
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
	return fmt.Sprintf(`<div class="tag-plugin video-player" style="max-width:%s;"><video controls preload playsinline webkit-playsinline><source src="%s" type="%s">Your browser does not support the video tag.</video></div>`,
		width, html.EscapeString(named["src"]), typ)
}

// renderAudio 渲染 {% audio src %}，支持网易云外链与本地音频。
func renderAudio(argsStr string) string {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"type", "netease", "autoplay"}, []string{"src"})
	if netease := named["netease"]; netease != "" {
		typ := named["type"]
		if typ == "" {
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
	return fmt.Sprintf(`<div class="tag-plugin audio"><audio controls preload><source src="%s" type="%s">Your browser does not support the audio tag.</audio></div>`,
		html.EscapeString(named["src"]), typ)
}

// renderCopy 渲染 {% copy text %} / {% copy git user/repo %} 为复制按钮。
func renderCopy(argsStr string) string {
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
	const toast = "复制成功"
	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin copy">`)
	if prefix := named["prefix"]; prefix != "" {
		sb.WriteString(`<span>` + html.EscapeString(prefix) + `</span>`)
	}
	sb.WriteString(fmt.Sprintf(`<input class="copy-area" id="%s" value="%s">`, id, html.EscapeString(text)))
	sb.WriteString(fmt.Sprintf(`<button class="copy-btn" onclick="util.copy(&quot;%s&quot;,&quot;%s&quot;)">`, id, toast))
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
func renderGHCard(argsStr string, ghClient GHClient) string {
	params := []string{"show_owner", "theme", "title_color", "text_color", "icon_color", "bg_color", "hide_border", "cache_seconds", "locale"}
	named, _ := parseNamedPositionalArgs(argsStr, params, []string{"repo"})
	path := named["repo"]
	if path == "" {
		return ""
	}
	if strings.Contains(path, "/") {
		return renderGHRepo(path, ghClient)
	}
	return renderGHUser(path, ghClient)
}

// renderPoetry 渲染 {% poetry [title] [author:] [date:] [footer:] %} body {% endpoetry %}。
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
	style := named["style"]
	if style == "" {
		style = "qq"
	}
	scene := named["scene"]
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
		u := users[c.params["user"]]
		sb.WriteString(`<div class="chatcell`)
		if me != "" && me == c.params["user"] {
			sb.WriteString(` right`)
		}
		sb.WriteString(`">`)
		sb.WriteString(`<div class="user-avatar"><img lazy src="` + html.EscapeString(u.Avatar) + `"></div>`)
		sb.WriteString(`<div class="user-main"><div class="header">`)
		if u.LabelText != "" {
			textColor := u.LabelTextColor
			if textColor == "" {
				textColor = "white"
			}
			bgColor := u.LabelBgColor
			if bgColor == "" {
				bgColor = "#2196f3"
			}
			sb.WriteString(`<span class="label hand-color" style="color:` + textColor + `;background:` + bgColor + `;">` + html.EscapeString(u.LabelText) + `</span>`)
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
