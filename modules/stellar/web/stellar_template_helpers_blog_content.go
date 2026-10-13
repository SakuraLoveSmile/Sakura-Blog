package web

import (
	"fmt"
	"html/template"
	"net/url"
	"regexp"
	"strings"
)

// Blog 内容扩展只读取宿主筛选后的独立记录投影，不回读旧 Poster、
// Banner 或 AuthorLinks 字段，也不把它们补进官方 PageModel。
func stellarBlogRecordContent(d *StellarRenderData) map[string]any {
	return stellarMap(stellarGet(d.Stellar.Data, "recordBlog.content"))
}

func stellarBlogItemContent(d *StellarRenderData, model map[string]any) map[string]any {
	id := stellarText(stellarGet(model, "item.id"))
	if id == "" {
		return nil
	}
	return stellarMap(stellarGet(stellarMap(d.Stellar.Data["itemBlogs"])[id], "content"))
}

func stellarBlogCollectionContent(d *StellarRenderData, profile string, listing map[string]any) map[string]any {
	id := stellarText(listing["id"])
	if id == "" {
		return nil
	}
	return stellarMap(stellarGet(stellarMap(d.Stellar.Data["collectionBlogs"])[profile+"/"+id], "content"))
}

// 外链扩展只改变卡片的目标，不改变 canonical route 或集合首页。
// 浏览器把 //、/\ 和反斜杠识别为跨站地址，因此根路径必须排除这些写法。
func stellarBlogContentURL(root string, value any) string {
	s, ok := value.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return ""
	}
	if strings.Contains(s, `\`) || strings.ContainsAny(s, "\x00\r\n\t") {
		return ""
	}
	s = strings.TrimSpace(s)
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	if u.IsAbs() {
		u.Scheme = strings.ToLower(u.Scheme)
		if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return ""
		}
		return u.String()
	}
	if !strings.HasPrefix(s, "/") || strings.HasPrefix(s, "//") || u.Host != "" {
		return ""
	}
	return stellarURL(root, u.String())
}

func stellarBlogBannerImage(d *StellarRenderData, canonical string) string {
	if !stellarBlogDetail(d) {
		return canonical
	}
	if image := stellarBlogContentURL(d.Stellar.Root, stellarBlogRecordContent(d)["banner_image"]); image != "" {
		return image
	}
	return canonical
}

func stellarBlogWikiHref(d *StellarRenderData, listing map[string]any, notebook bool) string {
	if !notebook {
		if href := stellarBlogContentURL(d.Stellar.Root, stellarBlogCollectionContent(d, "wiki", listing)["wiki_external_url"]); href != "" {
			return href
		}
	}
	return stellarURL(d.Stellar.Root, listing["href"])
}

var stellarBlogHexColor = regexp.MustCompile(`(?i)^#(?:[a-f0-9]{3}|[a-f0-9]{4}|[a-f0-9]{6}|[a-f0-9]{8})$`)
var stellarBlogColorVariable = regexp.MustCompile(`^var\(--[a-zA-Z0-9_-]+\)$`)

func stellarBlogPosterColor(value any) string {
	s, ok := value.(string)
	if !ok {
		return ""
	}
	s = strings.TrimSpace(s)
	if stellarBlogHexColor.MatchString(s) || stellarBlogColorVariable.MatchString(s) {
		return s
	}
	switch strings.ToLower(s) {
	case "transparent", "currentcolor", "inherit", "initial", "unset", "revert", "revert-layer":
		return s
	}
	if _, err := stellarParseColor(s); err == nil {
		return s
	}
	return ""
}

func (r stellarTemplateRenderer) blogPosterCard(d *StellarRenderData, model, listing map[string]any, notebook bool) template.HTML {
	if notebook {
		return ""
	}
	poster := stellarMap(stellarBlogItemContent(d, model)["poster"])
	headline, _ := poster["headline"].(string)
	cover := stellarText(listing["cover"])
	if strings.TrimSpace(headline) == "" || cover == "" {
		return ""
	}
	position, color := "bottom", stellarBlogPosterColor(poster["color"])
	if stellarTruth(poster["topic"]) {
		position = "top"
	}
	infoStyle := ""
	if position == "top" {
		// 2.0 的普通 hero 固定底部；只恢复独立旧 poster 的顶部文字位置。
		infoStyle = "top:0;bottom:auto;"
	}
	if color != "" {
		infoStyle += "color:" + color + ";"
	}
	styleAttr := ""
	if infoStyle != "" {
		styleAttr = ` style="` + stellarEscape(infoStyle) + `"`
	}
	coverURL := stellarURL(d.Stellar.Root, cover)
	var b strings.Builder
	fmt.Fprintf(&b, `<div class="post-card-wrap%s"><a class="%s" data-blog-poster href="%s"><div class="cover" position="%s" style="--cover-url:url('%s')"><img src="%s" alt="%s"><div class="cover-info" position="%s"%s>`, r.reveal(d), stellarEscape(r.ui(d, "post-card post photo", "hoverCard")), stellarEscape(stellarURL(d.Stellar.Root, listing["href"])), position, stellarEscape(stellarCSSURL(coverURL)), stellarEscape(coverURL), stellarEscape(listing["title"]), position, styleAttr)
	if stellarTruth(poster["topic"]) {
		fmt.Fprintf(&b, `<div class="text topic">%s</div>`, stellarEscape(poster["topic"]))
	}
	fmt.Fprintf(&b, `<div class="text headline">%s</div>`, stellarEscape(headline))
	if stellarTruth(poster["caption"]) {
		fmt.Fprintf(&b, `<div class="text caption">%s</div>`, stellarEscape(poster["caption"]))
	}
	b.WriteString(`</div></div></a></div>`)
	return template.HTML(b.String())
}

// 只有公开作者目录中命中的额外 ID 才显示；不猜角色或旧 AuthorLinks。
// 官方 primary 保持首位，扩展作者按原列表顺序追加并去重。
func (r stellarTemplateRenderer) blogAuthors(d *StellarRenderData, primary map[string]any, primaryID string) (template.HTML, bool) {
	ids := stellarList(stellarBlogRecordContent(d)["authors"])
	if len(ids) == 0 {
		return "", false
	}
	seen := map[string]bool{}
	if primaryID != "" {
		seen[primaryID] = true
	}
	if id := stellarText(primary["id"]); id != "" {
		seen[id] = true
	}
	authors := []map[string]any{}
	if len(primary) > 0 {
		authors = append(authors, primary)
	}
	extra := 0
	directory := stellarMap(d.Stellar.Data["authors"])
	for _, raw := range ids {
		id, ok := raw.(string)
		if !ok || id == "" || seen[id] {
			continue
		}
		seen[id] = true
		author := stellarMap(directory[id])
		if len(author) == 0 || !stellarTruth(author["name"]) || !stellarTruth(author["path"]) {
			continue
		}
		authors = append(authors, author)
		extra++
	}
	if extra == 0 {
		return "", false
	}
	var b strings.Builder
	for i, author := range authors {
		if i > 0 {
			b.WriteString(`<span class="sep"></span>`)
		}
		fmt.Fprintf(&b, `<a class="author" href="%s">%s</a>`, stellarEscape(stellarURL(d.Stellar.Root, author["path"])), stellarEscape(author["name"]))
	}
	b.WriteString(`<span class="sep"></span>`)
	return template.HTML(b.String()), true
}

func stellarBlogMomentsLabel(language, key string) string {
	locale := strings.ToLower(strings.ReplaceAll(language, "_", "-"))
	if strings.HasPrefix(locale, "en") {
		if key == "empty" {
			return "No moments yet. Stay tuned."
		}
		return "Moments"
	}
	if locale == "zh-tw" || locale == "zh-hk" || locale == "zh-hant" {
		if key == "empty" {
			return "還沒有瞬間，敬請期待。"
		}
		return "瞬間"
	}
	if key == "empty" {
		return "还没有瞬间，敬请期待。"
	}
	return "瞬间"
}

func (r stellarTemplateRenderer) blogMoments(d *StellarRenderData) template.HTML {
	var b strings.Builder
	fmt.Fprintf(&b, `<div class="article banner top"><div class="content"><div class="top bread-nav footnote"><div class="left"><div class="flex-row" id="breadcrumb"><a class="cap breadcrumb" href="%s">%s</a><span class="sep"></span><a class="cap breadcrumb" href="%s">%s</a></div></div></div><div class="bottom only-title"><div class="text-area"><h1 class="text title"><span>%s</span></h1></div></div></div></div><article class="md-text content">`, stellarEscape(d.Stellar.Root), stellarEscape(stellarTranslate(d, "btn.home")), stellarEscape(stellarURL(d.Stellar.Root, "/moments/")), stellarEscape(stellarBlogMomentsLabel(d.Stellar.Language, "label")), stellarEscape(d.Page.Title))
	if len(d.Moments) == 0 {
		fmt.Fprintf(&b, `<p class="cap" style="text-align:center;padding:2rem 0;color:var(--text-p3)">%s</p></article>`, stellarEscape(stellarBlogMomentsLabel(d.Stellar.Language, "empty")))
		return template.HTML(b.String())
	}
	b.WriteString(`<div class="tag-plugin timeline">`)
	for _, moment := range d.Moments {
		if moment == nil {
			continue
		}
		fmt.Fprintf(&b, `<div class="timenode"><div class="header"><div class="user-info"><img src="%s" alt=""><span>%s</span></div><span title="%s">%s</span></div><div class="body">%s`, stellarEscape(stellarURL(d.Stellar.Root, d.Site.Avatar)), stellarEscape(stellarFirst(d.Site.LogoTitle, d.Site.Title)), stellarEscape(moment.Date), stellarEscape(moment.TimeAgo), moment.ContentHTML)
		if len(moment.Images) > 0 {
			fmt.Fprintf(&b, `<div class="tag-plugin gallery grid-box" size="%s" ratio="%s">`, moment.GridSize(), moment.GridRatio())
			for _, image := range moment.Images {
				fmt.Fprintf(&b, `<div class="grid-cell lazy-box"><img class="lazy" data-fancybox="moment-%d" data-src="%s" alt=""><div class="lazy-icon"></div><div class="image-meta"></div></div>`, moment.ID, stellarEscape(stellarURL(d.Stellar.Root, image)))
			}
			b.WriteString(`</div>`)
		}
		b.WriteString(`</div></div>`)
	}
	b.WriteString(`</div>`)
	// 只为分页基路径创建当前请求副本，禁止写回宿主公开快照。
	local, presentation := *d, *d.Stellar
	presentation.Data = copyStellarMap(d.Stellar.Data)
	if !stellarTruth(presentation.Data["paginationPath"]) {
		presentation.Data["paginationPath"] = "/moments/"
	}
	local.Stellar = &presentation
	b.WriteString(string(r.paginator(&local)))
	b.WriteString(`</article>`)
	return template.HTML(b.String())
}
