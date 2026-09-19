package post

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"sync/atomic"

	"blog/internal/settings"
)

// note 标签正则：{% note [color:xxx] [title] content %}
var reNote = regexp.MustCompile(`{%\s*note\s+(.*?)\s*%}`)

// folding 标签正则（块级）：{% folding ... %} body {% endfolding %}
var reFolding = regexp.MustCompile(`(?s){%\s*folding\s+(.*?)\s*%}(.*?){%\s*endfolding\s*%}`)

// tabs 标签正则（块级）：{% tabs ... %} body {% endtabs %}
var reTabs = regexp.MustCompile(`(?s){%\s*tabs\s*(.*?)\s*%}(.*?){%\s*endtabs\s*%}`)

// timeline 标签正则（块级）：{% timeline ... %} body {% endtimeline %}
var reTimeline = regexp.MustCompile(`(?s){%\s*timeline\s*(.*?)\s*%}(.*?){%\s*endtimeline\s*%}`)
var reDropdown = regexp.MustCompile(`(?s){%\s*dropdown\s*(.*?)\s*%}(.*?){%\s*enddropdown\s*%}`)

// gallery 标签正则（块级）：{% gallery ... %} body {% endgallery %}
var reGallery = regexp.MustCompile(`(?s){%\s*gallery\s*(.*?)\s*%}(.*?){%\s*endgallery\s*%}`)

// mermaid 标签正则（块级）：{% mermaid %} body {% endmermaid %}
var reMermaid = regexp.MustCompile(`(?s){%\s*mermaid\s*%}(.*?){%\s*endmermaid\s*%}`)

// reEmojiHeight 校验 emoji height 参数格式，仅允许合法的 CSS 长度值，防 CSS 注入。
var reEmojiHeight = regexp.MustCompile(`^[0-9]*\.?[0-9]+(px|em|rem|%|vh|vw)?$`)

// button 标签正则：{% button ... %}
var reButton = regexp.MustCompile(`{%\s*button\s+(.*?)\s*%}`)

// checkbox/radio 标签正则：{% checkbox ... %} / {% radio ... %}（同一渲染入口，radio 为 input 类型变体）
var reCheckbox = regexp.MustCompile(`{%\s*(checkbox|radio)\s+(.*?)\s*%}`)

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

// table 标签正则（块级）：{% table [style:scroll|wrap|compact] %} body {% endtable %}
var reTable = regexp.MustCompile(`(?s){%\s*table\s*(.*?)\s*%}(.*?){%\s*endtable\s*%}`)

// tip 标签正则（块级）：{% tip text:... %} body {% endtip %}
var reTip = regexp.MustCompile(`(?s){%\s*tip\s*(.*?)\s*%}(.*?){%\s*endtip\s*%}`)

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

// mbti 标签正则（单行，1.38）：{% mbti INTJ-T 93-72-78-83-51 [url:...] [image:...] %}
var reMbti = regexp.MustCompile(`{%\s*mbti\s+(.*?)\s*%}`)

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

// vote 标签正则（单行）：{% vote [id:x] [yes:icon] [no:icon] [title] %}
var reVote = regexp.MustCompile(`{%\s*vote\s*(.*?)\s*%}`)

// friends 标签正则（单行）：{% friends [api:url] %}
var reFriends = regexp.MustCompile(`{%\s*friends\s*(.*?)\s*%}`)

// links 标签正则（单行）：{% links %}（分组友链卡片流，服务端直出站点 links 配置）
var reLinks = regexp.MustCompile(`{%\s*links\s*%}`)

// moments 标签正则（单行）：{% moments [limit:n] %}（公开瞬间时间流，服务端直出）
var reMoments = regexp.MustCompile(`{%\s*moments\s*(.*?)\s*%}`)

// sites 标签正则（单行）：{% sites api:url %}
var reSites = regexp.MustCompile(`{%\s*sites\s*(.*?)\s*%}`)

// 行内修饰标签（对齐原版 inline-labels.js）：{% u text %} 等简单包裹标签
var reInlineLabel = regexp.MustCompile(`{%\s*(u|emp|wavy|del|kbd|psw|blur)\s+(.*?)\s*%}`)

// sup/sub 行内标签：{% sup [color:x] text %} / {% sub [color:x] text %}
var reSupSub = regexp.MustCompile(`{%\s*(sup|sub)\s+(.*?)\s*%}`)

// [[text]] 快捷下划线语法（对齐原版 inline-labels.js）：渲染为 <u class="p tag-plugin">text</u>
// 需排除 [[alt](url)] 这类与 Markdown 链接嵌套冲突的场景
var reShortcutUnderline = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

// {color|text} 快捷颜色文字语法：渲染为 <span class="tag-plugin" style="color:color">text</span>
var reShortcutColor = regexp.MustCompile(`\{([a-zA-Z#][a-zA-Z0-9#]*)\|([^\}]+)\}`)

// --- Spec 31 展示类标签（对照原版 scripts/tags/lib）---

// about 标签（块级）：{% about [avatar:x] [height:x] [border:x] [back:x] %} body {% endabout %}
var reAbout = regexp.MustCompile(`(?s){%\s*about\s*(.*?)\s*%}(.*?){%\s*endabout\s*%}`)

// blockquote 标签（块级）：{% blockquote [indent:true/false] %} body {% endblockquote %}
var reBlockquoteTag = regexp.MustCompile(`(?s){%\s*blockquote\s*(.*?)\s*%}(.*?){%\s*endblockquote\s*%}`)

// banner 标签（块级，body 内可嵌 navbar 标签）：{% banner title [subtitle] [bg:src] [avatar:src] [link:url] %} ... {% endbanner %}
var reBannerTag = regexp.MustCompile(`(?s){%\s*banner\s+(.*?)\s*%}(.*?){%\s*endbanner\s*%}`)

// navbar 标签（单行）：{% navbar [active:url] [Home](/) [About](/about/) %}
var reNavbarTag = regexp.MustCompile(`{%\s*navbar\s+(.*?)\s*%}`)

// frame 标签（单行）：{% frame device [img:src] [video:url] [focus:top/bottom] [alt] %}
var reFrameTag = regexp.MustCompile(`{%\s*frame\s+(.*?)\s*%}`)

// --- Spec 49 展示容器 6 件套（块级语法，banner 改造 + navbar/frame 新增块级形式）---

// navbar 块级标签：{% navbar [active:文本] %} 链接列表（每行 [text](url)） {% endnavbar %}
var reNavbarBlock = regexp.MustCompile(`(?s){%\s*navbar\s*(.*?)\s*%}(.*?){%\s*endnavbar\s*%}`)

// frame 块级标签：{% frame [device:iphone/ipad/macbook] [width:宽度] %} 图片/视频 {% endframe %}
var reFrameBlock = regexp.MustCompile(`(?s){%\s*frame\s+(.*?)\s*%}(.*?){%\s*endframe\s*%}`)

// md 标签（单行，客户端 ds-mdrender 拉取渲染，对齐原版 md.js）：{% md src %}
var reMDTag = regexp.MustCompile(`{%\s*md\s+(.*?)\s*%}`)

// toc 标签（单行，wiki 文档树目录）：{% toc [wiki:xxx] [title] [open:true] [display:mobile] %}
var reTOCTag = regexp.MustCompile(`{%\s*toc\s*(.*?)\s*%}`)

// albums / posters 标签（单行）：{% albums [group] [repo:owner/repo] [api:http] [size:s/m/l/xl/mix] %}
var reAlbumsTag = regexp.MustCompile(`{%\s*albums\s*(.*?)\s*%}`)
var rePostersTag = regexp.MustCompile(`{%\s*posters\s*(.*?)\s*%}`)

// reel 标签（块级）：{% reel [title] [author:x] [date:x] [footer:x] %} body {% endreel %}
var reReelTag = regexp.MustCompile(`(?s){%\s*reel\s*(.*?)\s*%}(.*?){%\s*endreel\s*%}`)

// paper 标签（块级）：{% paper [style:underline] [title:x] [author:x] [date:x] [footer:x] %} body {% endpaper %}
var rePaperTag = regexp.MustCompile(`(?s){%\s*paper\s*(.*?)\s*%}(.*?){%\s*endpaper\s*%}`)

// paper 内部分隔标记：<!-- paragraph --> / <!-- section 标题 --> / <!-- line right -->
var rePaperMarker = regexp.MustCompile(`<!--\s*(paragraph|section|line)\s*(.*?)\s*-->`)

// 原版 about.js / banner.js 共用的返回箭头 SVG（逐字对齐）
const backArrowSVG = `<svg aria-hidden="true" viewBox="0 0 16 16" fill="currentColor"><path fill-rule="evenodd" d="M7.78 12.53a.75.75 0 01-1.06 0L2.47 8.28a.75.75 0 010-1.06l4.25-4.25a.75.75 0 011.06 1.06L4.81 7h7.44a.75.75 0 010 1.5H4.81l2.97 2.97a.75.75 0 010 1.06z"></path></svg>`

// 代码区段（围栏代码块 + 行内代码）：行内标签替换时跳过，避免误伤示例代码
var reCodeSegment = regexp.MustCompile("(?s)```.*?```|~~~.*?~~~|`[^`\n]+`")

var (
	reTabSplit    = regexp.MustCompile(`<!--\s*tab (.*?)\s*-->`)
	reNodeSplit   = regexp.MustCompile(`<!--\s*node (.*?)\s*-->`)
	reFolderSplit = regexp.MustCompile(`<!--\s*folder (.*?)\s*-->`)
	reCellSplit   = regexp.MustCompile(`<!--\s*cell(.*?)-->`)
	reGalleryMD   = regexp.MustCompile(`!\[(.*?)\]\((.*?)\)`)

	// swiper 从渲染后的 Markdown 中提取 img 标签
	reSwiperImg = regexp.MustCompile(`(?i)<img(.*?)src="(.*?)"(.*?)>`)
	// navbar 块级内部链接：行内 [text](url) 格式
	reNavbarLink = regexp.MustCompile(`\[(.*?)\]\((.*?)\)`)
	// frame 块级从渲染后的 Markdown 中提取媒体标签（img/video/iframe）
	reFrameMedia = regexp.MustCompile(`(?is)<(?:img|video|iframe)\b[^>]*>`)
	// chat 注释单元格：<!-- chattip ... --> / <!-- chatcell ... -->
	reChatComment = regexp.MustCompile(`(?s)<!--\s*(.*?)\s*-->`)
	// chat 单元格参数：user:/md:/tag:/quote:
	reChatParam = regexp.MustCompile(`(?:^|\s)(user|md|tag|quote):`)

	tabIndexCounter uint64
	galleryIndex    uint64
	hashtagColorIdx uint64
	copyIndex       uint64
	mdRenderIndex   uint64 // md 标签自增 id（对齐原版 md_index）
)

// renderInlineMD 仅渲染行内 Markdown 语法（粗体、斜体、链接、行内代码、图片），
// 排除块级语法（表格、围栏代码块、标题、列表等），供标签插件 body 安全使用。
func (s *Service) renderInlineMD(md string) string {
	md = protectMathDelimiters(md)
	var sb strings.Builder
	if err := s.inlineMD.Convert([]byte(md), &sb); err != nil {
		return md
	}
	return sb.String()
}

// processShortcodes 在 goldmark 渲染前预处理 Stellar 标签插件，返回替换后的 Markdown。
// 块级 body 内的 Markdown 由调用者传入 renderMD 函数单独渲染。
func processShortcodes(
	md string,
	renderMD func(string) string,
	renderInlineMD func(string) string,
	iconFunc func(string) string,
	ghClient GHClient,
	linksRender func() string,
	momentsRender func(string) string,
	albumsRender func(kind, argsStr string) string,
	tocRender func(argsStr string) string,
	tagPlugins settings.TagPluginsConfig,
) string {
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

		color, child, open, title := parseFoldingArgs(argsStr)
		renderedBody := renderMD(body)

		var sb strings.Builder
		sb.WriteString(`<details class="tag-plugin colorful folding"`)
		// 安全审计 2026-08-16：color/title 用户可控，白名单/转义后再拼入
		if c := safeColor(color); c != "" {
			fmt.Fprintf(&sb, ` color="%s"`, html.EscapeString(c))
		}
		// 安全审计 2026-08-30：child 只放行 codeblock/iframe（main.css 仅定义这两类子元素排版）
		if ch := safeFoldingChild(child); ch != "" {
			fmt.Fprintf(&sb, ` child="%s"`, html.EscapeString(ch))
		}
		if open {
			sb.WriteString(` open`)
		}
		sb.WriteString(`>`)
		if title != "" {
			fmt.Fprintf(&sb, `<summary>%s</summary>`, html.EscapeString(title))
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
		argsStr := parts[1]
		body := strings.TrimSpace(parts[2])
		return renderTimeline(argsStr, body, renderMD, tagPlugins)
	})

	// 3.1 dropdown（块级）
	md = reDropdown.ReplaceAllStringFunc(md, func(match string) string {
		parts := reDropdown.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		argsStr := parts[1]
		body := strings.TrimSpace(parts[2])
		return renderDropdown(argsStr, body, iconFunc)
	})

	// 4. gallery（块级）
	md = reGallery.ReplaceAllStringFunc(md, func(match string) string {
		parts := reGallery.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		argsStr := parts[1]
		body := strings.TrimSpace(parts[2])
		return renderGallery(argsStr, body, tagPlugins)
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
		return renderOKR(parts[1], parts[2], renderMD, tagPlugins)
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

	// 5.9 table（块级，表格 body 单独渲染 Markdown）
	md = reTable.ReplaceAllStringFunc(md, func(match string) string {
		parts := reTable.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return renderTable(parts[1], parts[2], renderMD)
	})

	// 5.10 tip（块级标签，内部 Markdown 收敛为行内内容）
	md = reTip.ReplaceAllStringFunc(md, func(match string) string {
		parts := reTip.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return renderTip(parts[1], parts[2], renderMD)
	})

	// 6. button（单行）
	md = reButton.ReplaceAllStringFunc(md, func(match string) string {
		parts := reButton.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		el, ok := renderButton(parts[1], tagPlugins)
		if !ok {
			return match
		}
		return el
	})

	// 7. checkbox / radio（单行）
	md = reCheckbox.ReplaceAllStringFunc(md, func(match string) string {
		parts := reCheckbox.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		el, ok := renderCheckbox(parts[1], parts[2])
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
		// tag_plugins.note.default_color：未指定 color 时的默认配色（空=不输出 color 属性，现状回归）
		if color == "" {
			color = tagPlugins.Note.DefaultColor
		}

		var sb strings.Builder
		sb.WriteString(`<div class="tag-plugin colorful note"`)
		// 安全审计 2026-08-16：color/title 用户可控，白名单/转义后再拼入
		if c := safeColor(color); c != "" {
			fmt.Fprintf(&sb, ` color="%s"`, html.EscapeString(c))
		}
		sb.WriteString(`>`)
		if title != "" {
			fmt.Fprintf(&sb, `<div class="title">%s</div>`, html.EscapeString(title))
		}
		fmt.Fprintf(&sb, `<div class="body">%s</div>`, renderInlineMD(content))
		sb.WriteString(`</div>`)
		return sb.String()
	})

	// 9. quot（单行，小标题，需 iconFunc 渲染前缀图标）
	md = reQuot.ReplaceAllStringFunc(md, func(match string) string {
		parts := reQuot.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return renderQuot(parts[1], iconFunc, tagPlugins)
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
		el, ok := renderMark(parts[1], tagPlugins)
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
		el, ok := renderEmoji(parts[1], tagPlugins)
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
		el, ok := renderIcon(parts[1], iconFunc, tagPlugins)
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
		el, ok := renderHashtag(parts[1], tagPlugins)
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
		return renderCopy(parts[1], tagPlugins)
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

	// 24.5 banner（块级，Spec 49 横幅容器；注册于 swiper 之后、chat 之前）
	md = reBannerTag.ReplaceAllStringFunc(md, func(match string) string {
		parts := reBannerTag.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return renderBanner(parts[1], parts[2], renderMD)
	})

	// 25. chat（块级，聊天消息）
	md = reChat.ReplaceAllStringFunc(md, func(match string) string {
		parts := reChat.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return renderChat(parts[1], parts[2], renderMD)
	})

	// 26. vote（单行，投票组件，对接本地 /api/vote 数据服务）
	md = reVote.ReplaceAllStringFunc(md, func(match string) string {
		parts := reVote.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return renderVote(parts[1], iconFunc)
	})

	// 27. friends（单行，友链卡片流，缺省指向本地 /api/friends 聚合端点）
	md = reFriends.ReplaceAllStringFunc(md, func(match string) string {
		parts := reFriends.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return renderFriends(parts[1])
	})

	// 27.5 links（单行，分组友链卡片流服务端直出，数据来自站点 links 配置注入）
	md = reLinks.ReplaceAllStringFunc(md, func(match string) string {
		if linksRender == nil {
			return match
		}
		return linksRender()
	})

	// 27.6 moments（单行，最新公开瞬间时间流，数据由当前站点注入）
	md = reMoments.ReplaceAllStringFunc(md, func(match string) string {
		parts := reMoments.FindStringSubmatch(match)
		if len(parts) < 2 || momentsRender == nil {
			return match
		}
		return momentsRender(parts[1])
	})

	// 28. sites（单行，站点卡片流，api 必填透传，参数缺失原样输出）
	md = reSites.ReplaceAllStringFunc(md, func(match string) string {
		parts := reSites.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		el, ok := renderSites(parts[1])
		if !ok {
			return match
		}
		return el
	})

	// 29. 行内修饰标签（u/emp/wavy/del/kbd/psw/blur/sup/sub），跳过代码块与行内代码
	md = replaceOutsideCode(md, renderInlineLabels)

	// 29.5 [[text]] 快捷下划线语法，跳过代码块与行内代码
	md = replaceOutsideCode(md, func(s string) string {
		return reShortcutUnderline.ReplaceAllStringFunc(s, func(match string) string {
			parts := reShortcutUnderline.FindStringSubmatch(match)
			if len(parts) < 2 {
				return match
			}
			text := strings.TrimSpace(parts[1])
			if text == "" {
				return match
			}
			// 排除与 Markdown 链接/图片嵌套冲突：若原文以 "](" 结尾则不替换
			if strings.HasSuffix(match, "](") {
				return match
			}
			// 安全审计 2026-08-16：text 用户可控，转义后再输出
			return fmt.Sprintf(`<u class="p tag-plugin">%s</u>`, html.EscapeString(text))
		})
	})

	// 29.6 {color|text} 快捷颜色文字语法，跳过代码块与行内代码
	md = replaceOutsideCode(md, func(s string) string {
		return reShortcutColor.ReplaceAllStringFunc(s, func(match string) string {
			parts := reShortcutColor.FindStringSubmatch(match)
			if len(parts) < 3 {
				return match
			}
			color := strings.TrimSpace(parts[1])
			text := strings.TrimSpace(parts[2])
			if color == "" || text == "" {
				return match
			}
			// 安全审计 2026-08-16：color 走颜色白名单（非法丢 style），text 用户可控须转义
			style := ""
			if c := safeColor(color); c != "" {
				style = fmt.Sprintf(` style="color:%s"`, html.EscapeString(c))
			}
			return fmt.Sprintf(`<span class="tag-plugin"%s>%s</span>`, style, html.EscapeString(text))
		})
	})

	// --- Spec 31 展示类 11 件（块级先、行内后；navbar 必须先于 banner，banner 从 body 提取已展开的 navbar 行）---

	// 30. about（块级）
	md = reAbout.ReplaceAllStringFunc(md, func(match string) string {
		parts := reAbout.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return renderAbout(parts[1], strings.TrimSpace(parts[2]), renderMD)
	})

	// 31. blockquote（块级）
	md = reBlockquoteTag.ReplaceAllStringFunc(md, func(match string) string {
		parts := reBlockquoteTag.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return renderBlockquote(parts[1], strings.TrimSpace(parts[2]), renderMD)
	})

	// 31.5 mbti（单行，1.38 人格卡片）
	md = reMbti.ReplaceAllStringFunc(md, func(match string) string {
		parts := reMbti.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		return renderMbti(parts[1])
	})

	// 32. reel（块级）
	md = reReelTag.ReplaceAllStringFunc(md, func(match string) string {
		parts := reReelTag.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return renderReel(parts[1], strings.TrimSpace(parts[2]), renderMD)
	})

	// 33. paper（块级，内部按 <!-- paragraph/section/line --> 分段）
	md = rePaperTag.ReplaceAllStringFunc(md, func(match string) string {
		parts := rePaperTag.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return renderPaper(parts[1], parts[2], renderMD)
	})

	// 33.5 navbar 块级（Spec 49 文章内导航；必须先于单行 navbar 处理，避免开放标签被单行正则吞掉）
	md = reNavbarBlock.ReplaceAllStringFunc(md, func(match string) string {
		parts := reNavbarBlock.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return renderNavbarBlock(parts[1], parts[2])
	})

	// 34. navbar 单行（Spec 31 兼容语法，行内 [text](url) 列表）
	md = reNavbarTag.ReplaceAllStringFunc(md, func(match string) string {
		parts := reNavbarTag.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		el, ok := renderNavbar(parts[1])
		if !ok {
			return match
		}
		return el
	})

	// 35. albums / posters（单行，数据走 ds-friends 服务或 group 友链卡片直出）
	md = reAlbumsTag.ReplaceAllStringFunc(md, func(match string) string {
		parts := reAlbumsTag.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		if albumsRender != nil {
			return albumsRender("albums", parts[1])
		}
		return renderAlbumsPosters("albums", parts[1])
	})
	md = rePostersTag.ReplaceAllStringFunc(md, func(match string) string {
		parts := rePostersTag.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		if albumsRender != nil {
			return albumsRender("posters", parts[1])
		}
		return renderAlbumsPosters("posters", parts[1])
	})

	// 37. frame 块级（Spec 49 设备框架；必须先于单行 frame 处理，避免开放标签被单行正则吞掉）
	md = reFrameBlock.ReplaceAllStringFunc(md, func(match string) string {
		parts := reFrameBlock.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		el, ok := renderFrameBlock(parts[1], parts[2], renderInlineMD)
		if !ok {
			return match
		}
		return el
	})

	// 37.5 frame 单行（Spec 31 兼容语法，设备边框；参数不全时原样输出容错）
	md = reFrameTag.ReplaceAllStringFunc(md, func(match string) string {
		parts := reFrameTag.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		el, ok := renderFrame(parts[1])
		if !ok {
			return match
		}
		return el
	})

	// 38. md（单行，客户端 ds-mdrender 拉取——对齐原版，服务端不发请求，无 SSRF 面）
	md = reMDTag.ReplaceAllStringFunc(md, func(match string) string {
		parts := reMDTag.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		named, _ := parseNamedPositionalArgs(parts[1], []string{"wrap"}, []string{"src"})
		src := strings.TrimSpace(named["src"])
		if src == "" {
			return match
		}
		id := atomic.AddUint64(&mdRenderIndex, 1)
		replace := named["wrap"] == "false"
		attr := ""
		if replace {
			attr = ` data-replace="true"`
		}
		return fmt.Sprintf(`<div class="data-service ds-mdrender" src="%s"%s data-heading="true" id="md_%d"></div>`, html.EscapeString(src), attr, id)
	})

	// 39. toc（单行，文章目录外壳或 wiki 文档树目录；对齐原版 toc.js）
	md = reTOCTag.ReplaceAllStringFunc(md, func(match string) string {
		parts := reTOCTag.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		if tocRender != nil {
			return tocRender(parts[1])
		}
		return renderTOCTag(parts[1], nil, false)
	})

	return md
}

// okrStatus okr 状态 → 配色 + 文案。
