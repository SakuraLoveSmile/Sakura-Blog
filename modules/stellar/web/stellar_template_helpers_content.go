package web

import (
	"fmt"
	"html/template"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"blog/internal/settings"
	"blog/internal/stellarmodels"
)

func stellarListing(value map[string]any) map[string]any {
	if listing := stellarMap(stellarGet(value, "render.listing")); len(listing) > 0 {
		return listing
	}
	return value
}
func stellarDate(v any, layout string) string {
	if t, ok := v.(time.Time); ok {
		if t.IsZero() {
			return ""
		}
		return t.Format(layout)
	}
	s := stellarText(v)
	for _, format := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(format, s); err == nil {
			return t.Format(layout)
		}
	}
	return s
}
func stellarDateElement(d *StellarRenderData, v any) string {
	if !stellarTruth(v) {
		return ""
	}
	return `<time datetime="` + stellarEscape(stellarDate(v, time.RFC3339)) + `">` + stellarEscape(stellarDisplayDate(d, v)) + `</time>`
}

func (r stellarTemplateRenderer) content(d *StellarRenderData) (template.HTML, error) {
	if d == nil || d.Stellar == nil || d.ViewData == nil {
		return "", fmt.Errorf("stellar v2 template requires explicit presentation and host data")
	}
	s := d.Stellar
	layout := s.Layout
	for i, item := range s.ListItems {
		listing := stellarListing(item)
		if listing["href"] == nil {
			return "", fmt.Errorf("stellar v2 list item %d requires render.listing", i)
		}
	}
	switch layout {
	case "settings":
		return r.settingsPage(d), nil
	case "404", "error":
		return r.errorPage(d), nil
	case "categories":
		return r.categories(d), nil
	case "tags":
		return r.tags(d), nil
	case "moments":
		return r.blogMoments(d), nil
	case "archive", "archives", "author":
		return r.archives(d), nil
	case "index_topic", "topic-index", "topics":
		return r.topicIndex(d), nil
	case "topic":
		if s.Model == nil {
			return r.topicIndex(d), nil
		}
	case "index_wiki", "wiki":
		if s.Model == nil {
			return r.wikiIndex(d, false), nil
		}
	case "notebooks":
		return r.wikiIndex(d, true), nil
	case "notes":
		if s.Model == nil {
			return r.postIndex(d, true), nil
		}
	case "index", "home", "category", "tag":
		return r.postIndex(d, false), nil
	}
	if s.Model != nil {
		profile := stellarCollectionGet(d, "profile")
		if profile != "post" && profile != "wiki" && profile != "topic" && profile != "notebook" {
			return "", fmt.Errorf("stellar v2 content has unknown model profile %v", profile)
		}
		return r.article(d), nil
	}
	if s.Profile == "post" || s.Profile == "topic" || s.Profile == "notebook" || s.Profile == "note" || s.Profile == "wiki" {
		return "", fmt.Errorf("stellar v2 %s content requires explicit PageModel", s.Profile)
	}
	class := "md-text content" + r.reveal(d)
	if stellarGet(s.Theme, "features.heti.enabled") == true {
		class += " heti"
	}
	return template.HTML(string(r.banner(d)) + `<article class="` + class + `">` + string(r.blogSummary(d)) + string(d.Content) + `</article>`), nil
}

func (r stellarTemplateRenderer) article(d *StellarRenderData) template.HTML {
	article := stellarMap(stellarRenderGet(d, "article"))
	profile := stellarText(stellarCollectionGet(d, "profile"))
	class := "md-text content" + r.reveal(d)
	if article["heti"] == true {
		class += " heti"
	}
	content := stellarText(stellarItemGet(d, "content"))
	if profile == "wiki" && strings.TrimSpace(content) == "" {
		content = stellarText(article["readmeHtml"])
	}
	var b strings.Builder
	b.WriteString(string(r.banner(d)))
	fmt.Fprintf(&b, `<article class="%s">%s%s%s`, class, r.blogSummary(d), r.blogExpired(d), content)
	if profile == "notebook" {
		b.WriteString(string(r.articleTags(d, article["tags"])))
	}
	b.WriteString(`</article>`)
	if profile == "post" || profile == "topic" {
		b.WriteString(string(r.articleTags(d, article["tags"])))
	}
	footer := stellarMap(article["footer"])
	if profile != "notebook" || stellarBlogNotebookShareAllowed(d) {
		b.WriteString(string(r.share(d, stellarMap(footer["share"]))))
	}
	b.WriteString(string(r.articleFooter(d, footer)))
	if profile != "notebook" {
		if profile == "wiki" || stellarBlogGlobal(d, "read_next") != false {
			b.WriteString(string(r.readNext(d, article["previous"], article["next"], profile == "wiki")))
		}
		if profile != "wiki" {
			b.WriteString(string(r.relatedPosts(d, stellarMap(article["related"]))))
		}
	}
	return template.HTML(b.String())
}

func (r stellarTemplateRenderer) banner(d *StellarRenderData) template.HTML {
	banner := stellarMap(stellarFirst(stellarRenderGet(d, "article.banner"), stellarItemGet(d, "presentation.banner"), stellarGet(d.Stellar.Data, "page.banner")))
	title := stellarFirst(banner["headline"], stellarItemGet(d, "title"), stellarGet(d.Page, "Title"))
	if _, exists := banner["headline"]; exists {
		title = banner["headline"]
	}
	tagline := banner["tagline"]
	avatar := banner["avatar"]
	image := stellarText(stellarFirst(stellarItemGet(d, "cover"), stellarGet(d.Stellar.Data, "page.cover")))
	image = stellarBlogBannerImage(d, image)
	if d.AuthorProfile != nil {
		title = d.AuthorProfile.Name
		avatar = d.AuthorProfile.Avatar
		tagline = d.AuthorProfile.Description
		image = stellarText(d.Stellar.Data["authorCover"])
	} else if banner["background"] == false || banner["background"] == nil && stellarGet(d.Stellar.Theme, "article.banner.background") == false {
		image = ""
	}
	if banner["enabled"] == false {
		return ""
	}
	var top strings.Builder
	breadcrumb := stellarItemGet(d, "navigation.breadcrumb")
	if breadcrumb == nil {
		breadcrumb = stellarGet(d.Stellar.Data, "page.breadcrumb")
	}
	if breadcrumb != false {
		fmt.Fprintf(&top, `<div class="top bread-nav footnote"><div class="top-row"><div class="left" id="breadcrumb"><a class="cap breadcrumb" href="%s">%s</a>`, stellarEscape(d.Stellar.Root), stellarEscape(stellarTranslate(d, "btn.home")))
		top.WriteString(r.breadcrumbs(d))
		top.WriteString(`</div>`)
		var right strings.Builder
		if d.Stellar.Layout == "post" && stellarGet(d.Stellar.Theme, "article.showReadingTime") == true && d.WordCount > 0 {
			fmt.Fprintf(&right, `<span class="cap reading">%s · %s</span>`, stellarEscape(stellarTranslate(d, "meta.word_count", d.WordCount)), stellarEscape(stellarTranslate(d, "meta.reading_time", d.ReadingMinutes)))
		}
		labelValue := stellarText(stellarFirst(stellarItemGet(d, "presentation.article.aiLabel"), stellarGet(d.Stellar.Data, "page.article.aiLabel")))
		right.WriteString(string(r.blogAILabel(d, labelValue, image != "")))
		if right.Len() > 0 {
			fmt.Fprintf(&top, `<div class="right">%s</div>`, right.String())
		}
		top.WriteString(`</div><div class="meta-row">`)
		top.WriteString(string(r.dateInfo(d)))
		top.WriteString(`</div></div>`)
	}
	var bottom strings.Builder
	if stellarTruth(title) {
		class := "bottom"
		if !stellarTruth(tagline) {
			class += " only-title"
		}
		fmt.Fprintf(&bottom, `<div class="%s">`, class)
		if stellarTruth(avatar) {
			fmt.Fprintf(&bottom, `<img class="lazy avatar" data-src="%s">`, stellarEscape(stellarURL(d.Stellar.Root, avatar)))
		}
		fmt.Fprintf(&bottom, `<div class="text-area"><h1 class="text title"><span>%s</span></h1>`, stellarEscape(title))
		if stellarTruth(tagline) {
			fmt.Fprintf(&bottom, `<div class="text subtitle">%s</div>`, stellarEscape(tagline))
		}
		bottom.WriteString(`</div></div>`)
	}
	if top.Len() == 0 && bottom.Len() == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<div class="article-banner-wrap%s"><div class="article banner top"`, r.reveal(d))
	if image != "" {
		ratio := stellarText(stellarBlogGlobal(d, "banner_ratio"))
		ratioStyle := ""
		if n, err := strconv.ParseFloat(ratio, 64); err == nil && n > 0 && !math.IsInf(n, 0) && !math.IsNaN(n) {
			ratioStyle = ";--banner-ratio:" + strconv.FormatFloat(n, 'g', -1, 64)
		}
		fmt.Fprintf(&b, ` style="--bg-url:url('%s')%s"`, stellarEscape(stellarCSSURL(image)), ratioStyle)
	}
	b.WriteString(`>`)
	if image != "" {
		fmt.Fprintf(&b, `<img class="lazy bg" data-src="%s">`, stellarEscape(stellarURL(d.Stellar.Root, image)))
	}
	b.WriteString(`<div class="content"`)
	if image != "" {
		b.WriteString(` data-text-adaptive="split"`)
	}
	b.WriteString(`>`)
	if image != "" {
		b.WriteString(`<div class="banner-mask banner-mask-top" aria-hidden="true"></div><div class="banner-mask banner-mask-bottom" aria-hidden="true"></div>`)
	}
	b.WriteString(top.String())
	b.WriteString(bottom.String())
	b.WriteString(`</div></div></div>`)
	return template.HTML(b.String())
}
func (r stellarTemplateRenderer) dateInfo(d *StellarRenderData) template.HTML {
	var b strings.Builder
	b.WriteString(`<div class="flex-row" id="post-meta">`)
	profile := stellarText(stellarCollectionGet(d, "profile"))
	created := stellarFirst(stellarItemGet(d, "date"), stellarGet(d.Stellar.Data, "page.date"), stellarGet(d.Stellar.Data, "hostPage.date"), stellarGet(d.Page, "Date"))
	updated := stellarFirst(stellarItemGet(d, "updated"), stellarGet(d.Stellar.Data, "page.updated"), stellarGet(d.Stellar.Data, "hostPage.updated"), stellarGet(d.Page, "Updated"))
	if profile == "wiki" {
		updated = stellarRenderGet(d, "article.updated")
		if stellarTruth(updated) {
			fmt.Fprintf(&b, `<span class="text created">%s%s%s</span>`, stellarEscape(stellarTranslate(d, "meta.updated")), stellarEscape(stellarTranslate(d, "symbol.colon")), stellarDateElement(d, updated))
		}
	} else {
		label := "meta.created"
		if profile == "notebook" {
			label = "meta.updated"
			created = stellarRenderGet(d, "article.updated")
			updated = stellarRenderGet(d, "article.created")
		}
		author := stellarMap(d.Stellar.Data["defaultAuthor"])
		id := stellarText(stellarFirst(stellarRenderGet(d, "listing.authorId"), stellarGet(d.Stellar.Data, "page.article.author")))
		if id != "" {
			if a := stellarMap(stellarGet(d.Stellar.Data, "authors."+id)); len(a) > 0 {
				author = a
			}
		}
		if len(author) > 0 && profile != "notebook" {
			if names, extended := r.blogAuthors(d, author, id); extended {
				b.WriteString(string(names))
			} else {
				fmt.Fprintf(&b, `<a class="author" href="%s">%s</a><span class="sep"></span>`, stellarEscape(stellarURL(d.Stellar.Root, author["path"])), stellarEscape(author["name"]))
			}
		}
		prefix := ""
		if len(author) == 0 || profile == "notebook" {
			prefix = stellarEscape(stellarTranslate(d, label) + stellarTranslate(d, "symbol.colon"))
		}
		fmt.Fprintf(&b, `<span class="text created">%s%s</span><span class="sep updated"></span><span class="text updated">%s%s</span>`, prefix, stellarDateElement(d, created), stellarEscape(stellarTranslate(d, stellarText(stellarFirst(map[bool]string{true: "meta.created", false: "meta.updated"}[profile == "notebook"], "meta.updated")))+stellarTranslate(d, "symbol.colon")), stellarDateElement(d, updated))
	}
	b.WriteString(`</div>`)
	return template.HTML(b.String())
}

func (r stellarTemplateRenderer) breadcrumbs(d *StellarRenderData) string {
	profile := stellarText(stellarCollectionGet(d, "profile"))
	var b strings.Builder
	link := func(path, title, class, id string) {
		b.WriteString(`<span class="sep"></span>`)
		fmt.Fprintf(&b, `<a class="cap %s"`, class)
		if id != "" {
			fmt.Fprintf(&b, ` id="%s"`, id)
		}
		fmt.Fprintf(&b, ` href="%s">%s</a>`, stellarEscape(stellarURL(d.Stellar.Root, path)), stellarEscape(title))
	}
	base := ""
	switch profile {
	case "wiki":
		base = stellarText(stellarRenderGet(d, "layout.wikiIndexPath"))
		if base != "" {
			link(base, stellarTranslate(d, "btn.wiki"), "breadcrumb", "menu")
		}
	case "notebook":
		base = stellarText(stellarRenderGet(d, "layout.notebookIndexPath"))
		if base != "" {
			link(base, stellarTranslate(d, "btn.notebook"), "breadcrumb", "")
		}
	case "post", "topic":
		base = stellarText(stellarRenderGet(d, "layout.blogPath"))
		key, id := "btn.blog", ""
		if profile == "topic" {
			key, id = "btn.topic", "menu"
		}
		link(base, stellarTranslate(d, key), "breadcrumb", id)
	default:
		link(stellarPagePath(d), d.Page.Title, "breadcrumb", "")
		return b.String()
	}
	for _, raw := range stellarList(stellarRenderGet(d, "layout.breadcrumbs")) {
		item := stellarMap(raw)
		path := stellarText(item["path"])
		if (profile == "topic" || profile == "wiki") && stellarURL(d.Stellar.Root, path) == stellarURL(d.Stellar.Root, base) {
			continue
		}
		class, id := "breadcrumb", ""
		if profile == "post" {
			class = "breadcrumb-link"
		}
		if profile == "wiki" || profile == "topic" {
			id = "proj"
		}
		link(path, stellarText(item["name"]), class, id)
	}
	return b.String()
}
func (r stellarTemplateRenderer) articleTags(d *StellarRenderData, tags any) template.HTML {
	var b strings.Builder
	for _, raw := range stellarList(tags) {
		v := stellarMap(raw)
		fmt.Fprintf(&b, `<a class="tag" href="%s">%s%s</a>`, stellarEscape(stellarURL(d.Stellar.Root, v["path"])), r.blogTaxonomyIcon(d, "tag", stellarText(v["name"]), "default:hashtag"), stellarEscape(v["name"]))
	}
	if b.Len() == 0 {
		return ""
	}
	return template.HTML(`<div class="article-tags` + r.reveal(d) + `">` + b.String() + `</div>`)
}
func (r stellarTemplateRenderer) articleFooter(d *StellarRenderData, footer map[string]any) template.HTML {
	var b strings.Builder
	if refs := stellarList(footer["references"]); len(refs) > 0 {
		fmt.Fprintf(&b, `<section id="references"><div class="header"><span>%s</span></div><div class="body"><ul>`, stellarEscape(stellarTranslate(d, "meta.references")))
		for _, ref := range refs {
			fmt.Fprintf(&b, `<li class="post-title">%s</li>`, stellarMarkdown(d, ref))
		}
		b.WriteString(`</ul></div></section>`)
	}
	if stellarTruth(footer["license"]) {
		fmt.Fprintf(&b, `<section id="license"><div class="header"><span>%s</span></div><div class="body">%s</div></section>`, stellarEscape(stellarTranslate(d, "meta.license")), stellarMarkdown(d, footer["license"]))
	}
	if c := stellarMap(footer["contributor"]); len(c) > 0 {
		fmt.Fprintf(&b, `<section id="contributors"><div class="header"><span>%s</span><span class="spacer"></span><a class="edit" href="%s">%s<span>%s</span></a></div><div class="body"><div class="ds-contributors users-wrap" api-type="commits" data-api="%s"><div class="grid-box"></div></div></div></section>`, stellarEscape(stellarTranslate(d, "meta.contributors")), stellarEscape(stellarURL(d.Stellar.Root, c["editUrl"])), r.icon("default:edit"), stellarEscape(stellarTranslate(d, "btn.edit")), stellarEscape(c["commitsUrl"]))
	}
	if len(stellarMap(footer["contributor"])) == 0 {
		b.WriteString(string(r.blogEditFooter(d)))
	}
	if b.Len() == 0 {
		return ""
	}
	return template.HTML(`<div class="article-footer` + r.reveal(d) + `">` + b.String() + `</div>`)
}
func (r stellarTemplateRenderer) share(d *StellarRenderData, share map[string]any) template.HTML {
	var b strings.Builder
	qrcode := false
	official := map[string]bool{}
	for _, service := range stellarList(share["services"]) {
		name := stellarText(service)
		official[name] = true
		action := stellarmodels.BuildShareAction(name, share)
		if len(action) == 0 {
			continue
		}
		label := stellarTranslate(d, "meta.share")
		if stellarTruth(action["label"]) {
			label += ": " + stellarText(action["label"])
		}
		class := r.ui(d, "social share-item "+name, "interactive", "is-icon-only")
		attrs := ` href="` + stellarEscape(stellarURL(d.Stellar.Root, action["href"])) + `"`
		if action["kind"] == "external" {
			attrs += ` target="_blank" rel="external nofollow noopener noreferrer"`
		}
		if action["kind"] == "toggle" {
			attrs = ` onclick="util.toggle(&quot;` + stellarEscape(action["target"]) + `&quot;)"`
			qrcode = true
		}
		fmt.Fprintf(&b, `<a class="%s"%s aria-label="%s" title="%s"><span class="share-item__icon ui-icon">%s</span></a>`, stellarEscape(class), attrs, stellarEscape(label), stellarEscape(label), r.icon("share:"+name))
	}
	local, localQR := r.blogShareButtons(d, share, official)
	b.WriteString(string(local))
	qrcode = qrcode || localQR
	shareURL := stellarText(stellarFirst(share["permalink"], stellarBlogURL(d)))
	shareTitle := stellarText(stellarFirst(share["title"], d.Page.Title+" - "+d.Site.Title))
	if b.Len() == 0 {
		return ""
	}
	qr := ""
	if qrcode {
		qr = `<div class="share-qrcode" id="share-qrcode" style="opacity:0;height:0"><img src="https://api.qrserver.com/v1/create-qr-code/?size=256x256&amp;data=` + stellarEscape(url.QueryEscape(shareURL)) + `"></div>`
	}
	return template.HTML(`<div class="article-share` + r.reveal(d) + `" id="share" data-url="` + stellarEscape(shareURL) + `" data-title="` + stellarEscape(shareTitle) + `"><div class="body"><div class="social-wrap dis-select">` + b.String() + `</div>` + qr + `</div></div>`)
}
func (r stellarTemplateRenderer) readNext(d *StellarRenderData, previous, next any, wiki bool) template.HTML {
	if !stellarTruth(previous) && !stellarTruth(next) {
		return ""
	}
	var b strings.Builder
	for i, raw := range []any{previous, next} {
		id, label, icon := "prev", "meta.newer", "default:arrow-left"
		if i == 1 {
			id, label, icon = "next", "meta.older", "default:arrow-right"
		}
		if wiki {
			if i == 0 {
				label = "meta.prev"
			} else {
				label = "meta.next"
			}
		}
		fmt.Fprintf(&b, `<div class="item" id="%s">`, id)
		v := stellarMap(raw)
		if len(v) > 0 {
			note := stellarEscape(stellarTranslate(d, label))
			if i == 0 {
				note = string(r.icon(icon)) + " " + note
			} else {
				note += " " + string(r.icon(icon))
			}
			fmt.Fprintf(&b, `<div class="note">%s</div><a href="%s">%s</a>`, note, stellarEscape(stellarURL(d.Stellar.Root, v["path"])), stellarEscape(stellarFirst(v["title"], stellarDisplayDate(d, v["date"]))))
		}
		b.WriteString(`</div>`)
	}
	return template.HTML(`<div class="related-wrap` + r.reveal(d) + `" id="read-next"><section class="body">` + b.String() + `</section></div>`)
}
func (r stellarTemplateRenderer) relatedPosts(d *StellarRenderData, related map[string]any) template.HTML {
	if !stellarTruth(related["enabled"]) {
		return ""
	}
	var b strings.Builder
	for _, raw := range stellarList(related["items"]) {
		v := stellarMap(raw)
		fmt.Fprintf(&b, `<a class="item" href="%s" title="%s"><span class="title">%s</span>`, stellarEscape(stellarURL(d.Stellar.Root, v["path"])), stellarEscape(v["title"]), stellarEscape(v["title"]))
		if stellarTruth(v["excerpt"]) {
			fmt.Fprintf(&b, `<span class="excerpt">%s</span>`, stellarEscape(stellarTruncate(stellarText(v["excerpt"]), 120)))
		}
		b.WriteString(`</a>`)
	}
	if b.Len() == 0 {
		return ""
	}
	return template.HTML(`<div class="related-wrap` + r.reveal(d) + `" id="related-posts"><section class="header"><div class="title cap theme">` + stellarEscape(stellarFirst(related["title"], stellarTranslate(d, "meta.related_posts"))) + `</div></section><section class="body"><div class="related-posts">` + b.String() + `</div></section></div>`)
}
func stellarTruncate(s string, n int) string {
	s = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(s, "")
	runes := []rune(s)
	if len(runes) > n {
		return string(runes[:n]) + "…"
	}
	return s
}

func (r stellarTemplateRenderer) listingNav(d *StellarRenderData, wiki bool) template.HTML {
	profile := "blogIndex"
	kind := "post"
	if wiki {
		profile = "wikiIndex"
		kind = "wiki"
	}
	config := stellarMap(stellarGet(d.Stellar.Theme, "profiles."+profile+".listingNav"))
	var b strings.Builder
	if config["enabled"] != true {
		return ""
	}
	link := func(href, title string, active, pretty bool) {
		href = stellarURL(d.Stellar.Root, href)
		if parsed, err := url.Parse(href); pretty && err == nil && !parsed.IsAbs() && parsed.Host == "" {
			// 官方内置导航使用 pretty_url；自定义 tabs 使用原始 url_for。
			if strings.HasSuffix(href, "/index.html") {
				href = strings.TrimSuffix(href, "index.html")
			} else if strings.HasSuffix(href, "/index") {
				href = strings.TrimSuffix(href, "index")
			} else {
				href = strings.TrimSuffix(href, ".html")
			}
			href = strings.TrimRight(href, "/") + "/"
		}
		class := ""
		if active {
			class = ` class="active"`
		}
		fmt.Fprintf(&b, `<a%s href="%s">%s</a>`, class, stellarEscape(href), stellarEscape(title))
	}
	custom := !wiki && stellarBlogGlobal(d, "nav_tabs.enable_custom") == true
	if custom {
		b.WriteString(string(r.blogNavTabs(d)))
	} else if wiki {
		link(stellarText(stellarGet(d.Stellar.Theme, "profiles.wikiIndex.path")), stellarTranslate(d, "btn.all_wiki"), !stellarTruth(d.Stellar.Data["wikiFilter"]), true)
		for _, raw := range stellarList(d.Stellar.Data["wikiTags"]) {
			v := stellarMap(raw)
			link(stellarText(v["path"]), stellarText(v["name"]), v["name"] == d.Stellar.Data["wikiTagName"], true)
		}
	} else {
		link(stellarText(stellarFirst(d.Stellar.Data["blogPath"], "/")), stellarTranslate(d, "btn.recent_publish"), d.Stellar.Layout == "index" || d.Stellar.Layout == "home", true)
		if len(d.Cats) > 0 || stellarTruth(d.Stellar.Data["hasCategories"]) {
			title := stellarTranslate(d, "btn.categories")
			if d.Page.Category != "" {
				title = stellarTranslate(d, "btn.category") + stellarTranslate(d, "symbol.colon") + d.Page.Category
			}
			link(stellarText(stellarFirst(d.Stellar.Data["categoriesPath"], "/categories/")), title, d.Page.Category != "" || d.Stellar.Layout == "categories", true)
		}
		if len(d.TagItems) > 0 || len(d.TagCloud) > 0 || stellarTruth(d.Stellar.Data["hasTags"]) {
			title := stellarTranslate(d, "btn.tags")
			if d.Page.Tag != "" {
				title = stellarTranslate(d, "btn.tag") + stellarTranslate(d, "symbol.colon") + d.Page.Tag
			}
			link(stellarText(stellarFirst(d.Stellar.Data["tagsPath"], "/tags/")), title, d.Page.Tag != "" || d.Stellar.Layout == "tags", true)
		}
		if d.HasTopics || stellarTruth(d.Stellar.Data["hasTopics"]) {
			link(stellarText(stellarGet(d.Stellar.Theme, "profiles.topic.path")), stellarTranslate(d, "btn.topic"), (d.Stellar.Layout == "index_topic" || d.Stellar.Layout == "topic"), true)
		}
		if len(d.Stellar.ListItems) > 0 || stellarTruth(d.Stellar.Data["hasPosts"]) {
			link(stellarText(stellarFirst(d.Stellar.Data["archivesPath"], "/archives/")), stellarTranslate(d, "btn.archives"), d.Stellar.Layout == "archive" || d.Stellar.Layout == "archives", true)
		}
	}
	if !custom {
		for _, raw := range stellarList(config["tabs"]) {
			v := stellarMap(raw)
			href := stellarText(v["url"])
			link(href, stellarText(v["title"]), strings.HasPrefix(stellarURL(d.Stellar.Root, stellarPagePath(d)), stellarURL(d.Stellar.Root, href)), false)
		}
		if !wiki {
			b.WriteString(string(r.blogNavTabs(d)))
		}
	}
	return template.HTML(`<div class="listing-nav"><div class="listing-nav__surface"><div class="listing-nav__viewport"><nav class="` + kind + `">` + b.String() + `</nav></div></div></div>`)
}
func (r stellarTemplateRenderer) postIndex(d *StellarRenderData, notebook bool) template.HTML {
	items := d.Stellar.ListItems
	firstHome := !notebook && (d.Stellar.Layout == "index" || d.Stellar.Layout == "home") && (d.Paginator == nil || d.Paginator.Current <= 1)
	flat := stellarGet(d.Stellar.Theme, "article.listing.pinnedLayout") == "flat"
	if firstHome && flat {
		pins := []map[string]any{}
		seen := map[string]bool{}
		for _, raw := range stellarList(d.Stellar.Data["pinnedItems"]) {
			item := stellarMap(raw)
			listing := stellarListing(item)
			if listing["listed"] != false && stellarInt(listing["priority"], 0) > 0 {
				pins = append(pins, item)
			}
		}
		sort.SliceStable(pins, func(i, j int) bool {
			return stellarInt(stellarListing(pins[i])["priority"], 0) > stellarInt(stellarListing(pins[j])["priority"], 0)
		})
		all := append([]map[string]any{}, pins...)
		for _, item := range pins {
			seen[stellarText(stellarListing(item)["href"])] = true
		}
		for _, item := range items {
			if !seen[stellarText(stellarListing(item)["href"])] {
				all = append(all, item)
			}
		}
		items = all
	}
	var b strings.Builder
	if !notebook {
		if stellarGet(d.Stellar.Theme, "article.listing.pinnedLayout") != "flat" {
			b.WriteString(string(r.pinSlider(d, false)))
		}
		b.WriteString(string(r.listingNav(d, false)))
	}
	b.WriteString(`<div class="post-list post">`)
	for _, raw := range items {
		listing := stellarListing(raw)
		if listing["listed"] == false {
			continue
		}
		if firstHome && !flat && stellarInt(listing["priority"], 0) > 0 {
			continue
		}
		if poster := r.blogPosterCard(d, raw, listing, notebook); poster != "" {
			b.WriteString(string(poster))
			continue
		}
		hero := listing["cardStyle"] == "hero" && stellarTruth(listing["cover"]) && !notebook
		class := "post-card post"
		style := ""
		if hero {
			class += " photo"
			style = ` style="--cover-url:url('` + stellarEscape(stellarCSSURL(stellarText(listing["cover"]))) + `')"`
		}
		href := stellarURL(d.Stellar.Root, listing["href"])
		if notebook && stellarTruth(d.Stellar.Data["activeTag"]) {
			href += "?tag=" + stellarQuery(d.Stellar.Data["activeTag"])
		}
		fmt.Fprintf(&b, `<div class="post-card-wrap%s"><a class="%s"%s href="%s">%s</a></div>`, r.reveal(d), stellarEscape(r.ui(d, class, "hoverCard")), style, stellarEscape(href), r.postCard(d, listing, notebook, hero))
	}
	b.WriteString(`</div>`)
	b.WriteString(string(r.paginator(d)))
	return template.HTML(b.String())
}
func (r stellarTemplateRenderer) postCard(d *StellarRenderData, v map[string]any, notebook, hero bool) template.HTML {
	title := stellarFirst(v["title"], stellarDisplayDate(d, v["date"]))
	var b strings.Builder
	if hero {
		fmt.Fprintf(&b, `<div class="cover" position="bottom" style="--cover-url:url('%s')"><img src="%s"><div class="cover-info" position="bottom" data-text-adaptive="split"><div class="text headline">%s</div>`, stellarEscape(stellarCSSURL(stellarText(v["cover"]))), stellarEscape(stellarURL(d.Stellar.Root, v["cover"])), stellarEscape(title))
		if stellarTruth(v["caption"]) {
			fmt.Fprintf(&b, `<div class="text caption">%s</div>`, stellarEscape(v["caption"]))
		}
		b.WriteString(`</div></div>`)
		return template.HTML(b.String())
	}
	b.WriteString(`<article class="md-text">`)
	if cover := stellarText(v["cover"]); cover != "" && (notebook || strings.Contains(cover, "/")) {
		fmt.Fprintf(&b, `<div class="post-cover"><img src="%s" alt="cover"></div>`, stellarEscape(stellarURL(d.Stellar.Root, cover)))
	}
	fmt.Fprintf(&b, `<h2 class="post-title">%s</h2>`, stellarEscape(title))
	if stellarTruth(v["excerpt"]) || notebook {
		class := "excerpt"
		if stellarGet(d.Stellar.Theme, "features.heti.enabled") == true {
			class += " heti"
		}
		fmt.Fprintf(&b, `<div class="%s"><p>%s</p></div>`, class, stellarEscape(v["excerpt"]))
	}
	date := v["date"]
	if notebook {
		date = stellarFirst(v["updated"], date)
	}
	fmt.Fprintf(&b, `<div class="meta cap"><span class="cap" id="post-meta">%s%s</span>`, r.icon("default:calendar"), stellarDateElement(d, date))
	if cats := stellarList(v["categories"]); len(cats) > 0 && !notebook {
		names := []string{}
		for _, cat := range cats {
			names = append(names, stellarText(cat))
		}
		style := ""
		if stellarTruth(v["categoryStyle"]) {
			style = ` style="` + stellarEscape(v["categoryStyle"]) + `"`
		}
		fmt.Fprintf(&b, `<span class="cap breadcrumb"%s>%s<span>%s</span></span>`, style, r.blogTaxonomyIcon(d, "category", names[len(names)-1], "default:category"), stellarEscape(strings.Join(names, " / ")))
	}
	tags := stellarList(v["tags"])
	if len(tags) > 0 {
		if !notebook {
			b.WriteString(`<span class="card-tags">`)
		}
		for _, tag := range tags {
			if notebook {
				fmt.Fprintf(&b, `<span class="cap breadcrumb"><span class="tag">%s</span></span>`, stellarEscape(tag))
			} else {
				fmt.Fprintf(&b, `<span class="cap tag-chip">%s%s</span>`, r.blogTaxonomyIcon(d, "tag", stellarText(tag), "default:hashtag"), stellarEscape(tag))
			}
		}
		if !notebook {
			b.WriteString(`</span>`)
		}
	}
	if stellarInt(v["priority"], 0) > 0 {
		fmt.Fprintf(&b, `<span class="pin">%s</span>`, r.icon("default:pin"))
	}
	b.WriteString(`</div></article>`)
	return template.HTML(b.String())
}
func (r stellarTemplateRenderer) wikiIndex(d *StellarRenderData, notebook bool) template.HTML {
	var b strings.Builder
	if !notebook {
		b.WriteString(string(r.pinSlider(d, true)))
		b.WriteString(string(r.listingNav(d, true)))
	}
	b.WriteString(`<div class="post-list wiki">`)
	for _, raw := range d.Stellar.Collections {
		v := stellarListing(raw)
		if v["listed"] == false {
			continue
		}
		cover := stellarText(v["cover"])
		class := "wiki-card"
		style := ""
		if cover == "" {
			class += " no-cover"
		} else {
			style = ` style="--cover-url:url('` + stellarEscape(stellarCSSURL(cover)) + `')"`
		}
		wrap := "post-card-wrap" + r.reveal(d)
		if stellarTruth(d.Stellar.Data["wikiFilter"]) {
			wrap += " filter"
		}
		fmt.Fprintf(&b, `<div class="%s"><a class="%s"%s href="%s">%s</a></div>`, wrap, stellarEscape(r.ui(d, class, "hoverCard")), style, stellarEscape(stellarBlogWikiHref(d, v, notebook)), r.wikiCard(d, v, notebook))
	}
	b.WriteString(`</div>`)
	return template.HTML(b.String())
}
func (r stellarTemplateRenderer) wikiCard(d *StellarRenderData, v map[string]any, notebook bool) template.HTML {
	var b strings.Builder
	cover := stellarText(v["cover"])
	class := "wiki-card-cover"
	if cover == "" {
		class += " no-cover"
	}
	fmt.Fprintf(&b, `<article class="wiki-card-content"><div class="%s" position="bottom">`, class)
	if cover != "" {
		fmt.Fprintf(&b, `<img src="%s" alt="">`, stellarEscape(stellarURL(d.Stellar.Root, cover)))
	}
	b.WriteString(`<div class="wiki-card-info" position="bottom"`)
	if cover != "" {
		b.WriteString(` data-text-adaptive="split"`)
	}
	b.WriteString(`>`)
	headline := v["headline"]
	if !notebook {
		headline = stellarFirst(headline, v["name"])
	}
	if stellarTruth(headline) || !notebook {
		b.WriteString(`<div class="wiki-card-summary">`)
		if !notebook {
			if tags := stellarList(v["tags"]); len(tags) > 0 {
				b.WriteString(`<div class="wiki-tags wiki-meta">`)
				for _, tag := range tags {
					fmt.Fprintf(&b, `<span>%s</span>`, stellarEscape(tag))
				}
				b.WriteString(`</div>`)
			}
		}
		fmt.Fprintf(&b, `<h2 class="headline">%s</h2>`, stellarEscape(headline))
		if !notebook && (stellarTruth(v["audience"]) || stellarTruth(v["repository"])) {
			b.WriteString(`<div class="wiki-meta">`)
			if stellarTruth(v["audience"]) {
				fmt.Fprintf(&b, `<span class="wiki-available">%s<span class="label">%s</span><span class="value">%s</span></span>`, r.icon("default:platforms"), stellarEscape(stellarTranslate(d, "meta.available")), stellarEscape(v["audience"]))
			}
			if stellarTruth(v["repository"]) {
				fmt.Fprintf(&b, `<span class="wiki-stars data-service ds-ghinfo" data-api="%s">%s<span type="text" id="stargazers_count"></span></span>`, stellarEscape(v["repositoryApi"]), r.icon("default:fire"))
			}
			b.WriteString(`</div>`)
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`<div class="wiki-project"><div class="project-icon">`)
	if stellarTruth(v["icon"]) {
		fmt.Fprintf(&b, `<img src="%s" alt="">`, stellarEscape(stellarURL(d.Stellar.Root, v["icon"])))
	} else {
		b.WriteString(string(r.icon("default:documents")))
	}
	fmt.Fprintf(&b, `</div><div class="project-text"><div class="project-name">%s</div>`, stellarEscape(v["name"]))
	if stellarTruth(v["caption"]) {
		fmt.Fprintf(&b, `<div class="project-subtitle">%s</div>`, stellarEscape(v["caption"]))
	}
	b.WriteString(`</div></div></div></div></article>`)
	return template.HTML(b.String())
}
func (r stellarTemplateRenderer) archiveItem(d *StellarRenderData, v map[string]any) string {
	href := stellarFirst(v["href"], v["link"], v["path"])
	date := stellarDate(v["date"], "01-02")
	timeHTML := ""
	if date != "" {
		timeHTML = `<time>` + stellarEscape(date) + `</time>`
	}
	return `<div class="archive-list"><a class="post fs14" href="` + stellarEscape(stellarURL(d.Stellar.Root, href)) + `">` + timeHTML + `<span>` + stellarEscape(stellarFirst(v["title"], stellarDisplayDate(d, v["date"]))) + `</span></a></div>`
}
func (r stellarTemplateRenderer) topicIndex(d *StellarRenderData) template.HTML {
	var b strings.Builder
	b.WriteString(string(r.listingNav(d, false)))
	b.WriteString(`<div class="post-list topic">`)
	for _, raw := range d.Stellar.Collections {
		v := stellarListing(raw)
		fmt.Fprintf(&b, `<div class="post-card-wrap%s"><div class="post-card topic"><article class="md-text"><h2 class="topic-title">%s</h2>`, r.reveal(d), stellarEscape(stellarFirst(v["headline"], v["name"])))
		if stellarTruth(v["description"]) {
			fmt.Fprintf(&b, `<p class="topic-desc">%s</p>`, stellarEscape(v["description"]))
		}
		latest := stellarMap(v["latest"])
		cover := stellarText(v["cover"])
		class := "cover"
		attrs := ""
		if cover != "" {
			attrs = ` style="--cover-url:url('` + stellarEscape(stellarCSSURL(cover)) + `')"`
		} else {
			class += " no-cover"
		}
		fmt.Fprintf(&b, `<a class="%s" position="bottom"%s href="%s">`, stellarEscape(r.ui(d, class, "hoverCard")), attrs, stellarEscape(stellarURL(d.Stellar.Root, latest["path"])))
		if cover != "" {
			fmt.Fprintf(&b, `<img src="%s" alt="">`, stellarEscape(stellarURL(d.Stellar.Root, cover)))
		}
		b.WriteString(`<div class="cover-info" position="bottom"`)
		if cover != "" {
			b.WriteString(` data-text-adaptive="split"`)
		}
		b.WriteString(`>`)
		if len(latest) > 0 {
			fmt.Fprintf(&b, `<div class="text headline">%s</div><div class="text caption">%s</div>`, stellarEscape(latest["title"]), stellarEscape(stellarDisplayDate(d, latest["date"])))
		}
		b.WriteString(`</div></a>`)
		items := stellarList(v["items"])
		if len(items) > 0 {
			b.WriteString(`<div class="topic-posts">`)
			for i, raw := range items {
				if i == 3 {
					fmt.Fprintf(&b, `<details class="topic-posts-more"><summary class="topic-posts-toggle"><span class="when-closed">%s</span><span class="when-open">%s</span>%s</summary><div class="topic-posts-extra">`, stellarEscape(stellarTranslate(d, "btn.expand_posts", len(items)-3)), stellarEscape(stellarTranslate(d, "btn.collapse_posts")), r.icon("default:arrow-right"))
				}
				b.WriteString(r.archiveItem(d, stellarMap(raw)))
			}
			if len(items) > 3 {
				b.WriteString(`</div></details>`)
			}
			b.WriteString(`</div>`)
		}
		b.WriteString(`</article></div></div>`)
	}
	b.WriteString(`</div>`)
	return template.HTML(b.String())
}
func (r stellarTemplateRenderer) archives(d *StellarRenderData) template.HTML {
	var b strings.Builder
	class := "post-list archives"
	if d.AuthorProfile != nil {
		class = "post-list author archives"
		b.WriteString(string(r.banner(d)))
	} else {
		b.WriteString(string(r.listingNav(d, false)))
	}
	fmt.Fprintf(&b, `<div class="%s">`, class)
	if len(d.Stellar.ListItems) > 0 {
		grouped := map[string][]map[string]any{}
		years := []string{}
		for _, raw := range d.Stellar.ListItems {
			item := stellarListing(raw)
			if item["listed"] == false {
				continue
			}
			year := stellarDate(item["date"], "2006")
			if year == "" || !stellarTruth(stellarFirst(item["title"], item["date"])) {
				continue
			}
			if _, ok := grouped[year]; !ok {
				years = append(years, year)
			}
			grouped[year] = append(grouped[year], item)
		}
		for _, year := range years {
			fmt.Fprintf(&b, `<article class="%s" id="archive"><div class="archive-header h4">%s</div>`, strings.TrimSpace(r.reveal(d)), stellarEscape(year))
			for _, item := range grouped[year] {
				b.WriteString(r.archiveItem(d, item))
			}
			b.WriteString(`</article>`)
		}
		b.WriteString(`</div>`)
		return template.HTML(b.String())
	}
	for _, year := range d.Archives {
		fmt.Fprintf(&b, `<article class="%s" id="archive"><div class="archive-header h4">%s</div>`, strings.TrimSpace(r.reveal(d)), stellarEscape(year.Year))
		for _, post := range year.Posts {
			b.WriteString(r.archiveItem(d, map[string]any{"title": post.Title, "path": "/post/" + post.Slug, "date": post.Date}))
		}
		b.WriteString(`</article>`)
	}
	b.WriteString(`</div>`)
	return template.HTML(b.String())
}
func (r stellarTemplateRenderer) categories(d *StellarRenderData) template.HTML {
	if terms := stellarList(stellarGet(d.Stellar.Data, "index.terms")); len(terms) > 0 {
		var visit func([]any) string
		visit = func(nodes []any) string {
			var b strings.Builder
			for _, raw := range nodes {
				node := stellarMap(raw)
				children := stellarList(node["children"])
				class, icon := "cat", "default:category"
				if stellarTruth(node["parentId"]) {
					class += " child"
				}
				if len(children) > 0 {
					icon = "default:category-open"
				}
				fmt.Fprintf(&b, `<div class="cat-item"><a class="%s" href="%s"><span class="name">%s%s</span><span class="badge">(%d)</span></a>`, class, stellarEscape(stellarURL(d.Stellar.Root, node["path"])), r.blogTaxonomyIcon(d, "category", stellarText(node["name"]), icon), stellarEscape(node["name"]), stellarInt(node["count"], 0))
				if len(children) > 0 {
					fmt.Fprintf(&b, `<div class="cat-children">%s</div>`, visit(children))
				}
				b.WriteString(`</div>`)
			}
			return b.String()
		}
		return template.HTML(string(r.listingNav(d, false)) + `<div class="post-list"><article class="` + strings.TrimSpace(r.reveal(d)) + `" id="cats">` + visit(terms) + `</article></div>`)
	}
	if len(d.Cats) == 0 {
		return ""
	}
	var visit func([]*settings.CatItem) string
	visit = func(nodes []*settings.CatItem) string {
		var b strings.Builder
		for _, node := range nodes {
			class := "cat"
			if node.ParentID > 0 {
				class += " child"
			}
			icon := "default:category"
			if len(node.Children) > 0 {
				icon = "default:category-open"
			}
			fmt.Fprintf(&b, `<div class="cat-item"><a class="%s" href="%s"><span class="name">%s%s</span><span class="badge">(%d)</span></a>`, class, stellarEscape(stellarURL(d.Stellar.Root, node.Path)), r.blogTaxonomyIcon(d, "category", node.Name, icon), stellarEscape(node.Name), node.Count)
			if len(node.Children) > 0 {
				fmt.Fprintf(&b, `<div class="cat-children">%s</div>`, visit(node.Children))
			}
			b.WriteString(`</div>`)
		}
		return b.String()
	}
	return template.HTML(string(r.listingNav(d, false)) + `<div class="post-list"><article class="` + strings.TrimSpace(r.reveal(d)) + `" id="cats">` + visit(d.Cats) + `</article></div>`)
}
func (r stellarTemplateRenderer) tags(d *StellarRenderData) template.HTML {
	if terms := stellarList(stellarGet(d.Stellar.Data, "index.terms")); len(terms) > 0 {
		sort.SliceStable(terms, func(i, j int) bool {
			return stellarInt(stellarGet(terms[i], "count"), 0) > stellarInt(stellarGet(terms[j], "count"), 0)
		})
		var b strings.Builder
		for _, raw := range terms {
			tag := stellarMap(raw)
			fmt.Fprintf(&b, `<a class="tag" href="%s">%s<span class="name">%s</span></a>`, stellarEscape(stellarURL(d.Stellar.Root, tag["path"])), r.blogTaxonomyIcon(d, "tag", stellarText(tag["name"]), "default:hashtag"), stellarEscape(tag["name"]))
		}
		return template.HTML(string(r.listingNav(d, false)) + `<div class="post-list"><article class="` + strings.TrimSpace(r.reveal(d)) + `" id="tags">` + b.String() + `</article></div>`)
	}
	if len(d.TagItems) == 0 {
		return ""
	}
	var b strings.Builder
	items := append([]*settings.TagItem(nil), d.TagItems...)
	sort.SliceStable(items, func(i, j int) bool { return items[i].Count > items[j].Count })
	for _, tag := range items {
		fmt.Fprintf(&b, `<a class="tag" href="%s">%s<span class="name">%s</span></a>`, stellarEscape(stellarURL(d.Stellar.Root, "/tags/"+url.PathEscape(tag.Name)+"/")), r.blogTaxonomyIcon(d, "tag", tag.Name, "default:hashtag"), stellarEscape(tag.Name))
	}
	return template.HTML(string(r.listingNav(d, false)) + `<div class="post-list"><article class="` + strings.TrimSpace(r.reveal(d)) + `" id="tags">` + b.String() + `</article></div>`)
}
func (r stellarTemplateRenderer) errorPage(d *StellarRenderData) template.HTML {
	image := stellarGet(d.Stellar.Theme, "profiles.error.image")
	body := `<article class="md-text error-page">`
	if stellarTruth(image) {
		body += `<h1><img id="error" src="` + stellarEscape(stellarURL(d.Stellar.Root, image)) + `" alt="404"></h1>`
	}
	body += `<p class="what"><strong>` + stellarEscape(stellarTranslate(d, "page.error.what")) + `</strong></p><p class="why">` + stellarEscape(stellarTranslate(d, "page.error.why")) + `</p><br><br><a class="button" id="back" href="` + stellarEscape(d.Stellar.Root) + `">` + stellarEscape(stellarTranslate(d, "page.error.action")) + `</a></article>`
	return template.HTML(body)
}
func (r stellarTemplateRenderer) paginator(d *StellarRenderData) template.HTML {
	p := d.Paginator
	if p == nil || p.Total <= 1 {
		return ""
	}
	base := stellarText(stellarFirst(d.Stellar.Data["paginationPath"], stellarPagePath(d), "/"))
	activeTag := stellarText(d.Stellar.Data["activeTag"])
	href := func(page int) string {
		path := strings.TrimRight(base, "/") + "/"
		if page > 1 {
			path += fmt.Sprintf("page/%d/", page)
		}
		if activeTag != "" {
			path += "?tag=" + stellarQuery(activeTag)
		}
		return stellarURL(d.Stellar.Root, path)
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<div class="paginator-wrap dis-select%s">`, r.reveal(d))
	if p.Current > 1 {
		fmt.Fprintf(&b, `<a class="extend prev" rel="prev" href="%s">%s</a>`, stellarEscape(href(p.Current-1)), r.icon("default:arrow-left"))
	} else {
		fmt.Fprintf(&b, `<span class="extend prev">%s</span>`, r.icon("default:arrow-left"))
	}
	for _, page := range p.Pages {
		if page == p.Current {
			fmt.Fprintf(&b, `<span class="page-number current">%d</span>`, page)
		} else {
			fmt.Fprintf(&b, `<a class="page-number" href="%s">%d</a>`, stellarEscape(href(page)), page)
		}
	}
	if p.Current < p.Total {
		fmt.Fprintf(&b, `<a class="extend next" rel="next" href="%s">%s</a>`, stellarEscape(href(p.Current+1)), r.icon("default:arrow-right"))
	} else {
		fmt.Fprintf(&b, `<span class="extend next">%s</span>`, r.icon("default:arrow-right"))
	}
	b.WriteString(`</div>`)
	return template.HTML(b.String())
}
func (r stellarTemplateRenderer) pinSlider(d *StellarRenderData, wiki bool) template.HTML {
	source := stellarList(d.Stellar.Data["pinnedItems"])
	if wiki {
		source = stellarList(d.Stellar.Data["allWikiItems"])
	}
	items := []map[string]any{}
	for _, raw := range source {
		v := stellarListing(stellarMap(raw))
		if v["listed"] != false && stellarInt(v["priority"], 0) > 0 {
			items = append(items, v)
		}
	}
	if len(items) == 0 {
		return ""
	}
	sort.SliceStable(items, func(i, j int) bool { return stellarInt(items[i]["priority"], 0) > stellarInt(items[j]["priority"], 0) })
	kind := "post"
	if wiki {
		kind = "wiki"
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<div class="%s" id="pin-slider" data-interval="5000" data-key="%s" data-group="%s"><div class="pin-slider-track">`, stellarEscape(r.ui(d, "pin-slider", "hoverCard")), kind, kind)
	for _, item := range items {
		cover := stellarText(item["cover"])
		class := "pin-slide"
		if !wiki {
			class += " post-slide"
		}
		if cover == "" {
			class += " no-cover"
		}
		href := stellarURL(d.Stellar.Root, item["href"])
		if wiki {
			href = stellarBlogWikiHref(d, item, false)
		}
		fmt.Fprintf(&b, `<a class="%s" href="%s"`, class, stellarEscape(href))
		if cover != "" {
			fmt.Fprintf(&b, ` style="--pin-cover-url:url('%s')"`, stellarEscape(stellarCSSURL(cover)))
		}
		b.WriteString(`>`)
		if cover != "" {
			fmt.Fprintf(&b, `<img class="pin-slide-bg" src="%s" alt="">`, stellarEscape(stellarURL(d.Stellar.Root, cover)))
		}
		info := "pin-slide-text"
		if wiki {
			info = "pin-slide-info"
		}
		fmt.Fprintf(&b, `<div class="%s"`, info)
		if cover != "" {
			b.WriteString(` data-text-adaptive="split"`)
		}
		b.WriteString(`>`)
		if wiki {
			b.WriteString(`<div class="pin-slide-caps">`)
			for _, tag := range stellarList(item["tags"]) {
				fmt.Fprintf(&b, `<span class="pin-slide-chip">%s</span>`, stellarEscape(tag))
			}
			fmt.Fprintf(&b, `</div><div class="pin-slide-title">%s</div>`, stellarEscape(stellarFirst(item["headline"], item["name"], item["id"])))
			if stellarTruth(item["description"]) {
				fmt.Fprintf(&b, `<div class="pin-slide-excerpt">%s</div>`, stellarEscape(item["description"]))
			}
		} else {
			fmt.Fprintf(&b, `<div class="pin-slide-headline">%s</div>`, stellarEscape(item["title"]))
			if stellarTruth(item["caption"]) {
				fmt.Fprintf(&b, `<div class="pin-slide-caption">%s</div>`, stellarEscape(item["caption"]))
			}
		}
		b.WriteString(`</div></a>`)
	}
	b.WriteString(`</div>`)
	if len(items) > 1 {
		b.WriteString(`<div class="pin-slider-dots">`)
		for i := range items {
			class := "pin-slider-dot"
			if i == 0 {
				class += " active"
			}
			fmt.Fprintf(&b, `<button type="button" class="%s" data-index="%d"></button>`, class, i)
		}
		fmt.Fprintf(&b, `</div><button type="button" class="pin-slider-nav prev">%s</button><button type="button" class="pin-slider-nav next">%s</button>`, r.icon("default:arrow-left"), r.icon("default:arrow-right"))
	}
	b.WriteString(`</div>`)
	return template.HTML(b.String())
}
