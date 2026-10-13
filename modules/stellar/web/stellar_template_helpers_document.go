package web

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"regexp"
	"strings"
)

//go:embed stellar_template_helpers_hero.json
var stellarHeroResourcesJSON []byte

var stellarHeroResources = func() map[string]any {
	var value map[string]any
	if err := json.Unmarshal(stellarHeroResourcesJSON, &value); err != nil {
		panic(err)
	}
	return value
}()

func stellarPageTitle(d *StellarRenderData) string {
	if title := stellarText(stellarRenderGet(d, "seo.title")); title != "" {
		return title
	}
	if d.Page.Title != "" {
		return d.Page.Title + " - " + d.Site.Title
	}
	if d.Stellar.Profile == "home" && d.Paginator != nil && d.Paginator.Current > 1 {
		return d.Site.Title + " - " + stellarTranslate(d, "symbol.page", d.Paginator.Current)
	}
	return d.Site.Title
}
func stellarLoadingCSS(d *StellarRenderData) template.CSS {
	quote := func(value string) string {
		var b strings.Builder
		b.WriteByte('"')
		for _, c := range value {
			if c == '"' || c == '\\' || c == '<' || c < 32 {
				fmt.Fprintf(&b, "\\%x ", c)
			} else {
				b.WriteRune(c)
			}
		}
		b.WriteByte('"')
		return b.String()
	}
	return template.CSS(":root{--text-loading:" + quote(stellarTranslate(d, "message.loading")) + ";--text-loading-error:" + quote(stellarTranslate(d, "message.loading_failed")) + "}")
}

func stellarHeadMeta(d *StellarRenderData) template.HTML {
	seo := stellarMap(stellarRenderGet(d, "seo"))
	var b strings.Builder
	meta := func(kind, name string, value any) {
		if stellarTruth(value) {
			fmt.Fprintf(&b, `<meta data-stellar-page-meta %s="%s" content="%s">`, kind, stellarEscape(name), stellarEscape(value))
		}
	}
	description := stellarFirst(seo["description"], d.Site.Description)
	meta("name", "description", description)
	keywords := []string{}
	for _, v := range stellarList(seo["keywords"]) {
		keywords = append(keywords, stellarText(v))
	}
	if len(keywords) > 0 {
		meta("name", "keywords", strings.Join(keywords, ", "))
	}
	robots := seo["robots"]
	if robots == nil {
		switch d.Stellar.Profile {
		case "error":
			robots = "none"
		case "blogIndex":
			robots = "noindex,follow"
		case "settings":
			robots = "noindex,nofollow"
		}
	}
	meta("name", "robots", robots)
	canonical := stellarText(stellarFirst(d.Page.CanonicalURL, seo["canonical"]))
	if d.Stellar.Profile == "error" || d.Stellar.Layout == "404" {
		canonical = ""
	}
	if canonical == "" && d.Stellar.Profile != "error" && d.Stellar.Layout != "404" {
		host := stellarText(stellarGet(d.Stellar.Theme, "canonical.host"))
		if host != "" {
			canonical = "https://" + host + stellarURL(d.Stellar.Root, stellarPagePath(d))
		}
	}
	if canonical != "" {
		fmt.Fprintf(&b, `<link data-stellar-page-meta rel="canonical" href="%s">`, stellarEscape(canonical))
	}
	og := stellarMap(seo["openGraph"])
	if len(og) == 0 && stellarGet(d.Stellar.Theme, "openGraph.enabled") == true {
		title := stellarFirst(stellarGet(d.Page, "Title"), d.Site.Title)
		og = map[string]any{"title": title, "siteName": d.Site.Title, "twitterTitle": title, "args": map[string]any{"type": "website", "description": description, "url": stellarPublicPageURL(d), "language": d.Stellar.Language, "image": d.Site.Avatar}}
	}
	if len(og) > 0 {
		args := stellarMap(og["args"])
		meta("property", "og:type", args["type"])
		meta("property", "og:title", og["title"])
		meta("property", "og:site_name", og["siteName"])
		meta("property", "og:url", stellarFirst(d.Page.CanonicalURL, args["url"], stellarPublicPageURL(d)))
		meta("property", "og:description", args["description"])
		meta("property", "og:locale", strings.ReplaceAll(stellarText(args["language"]), "-", "_"))
		for _, image := range stellarList(args["image"]) {
			meta("property", "og:image", image)
		}
		if _, ok := args["image"].(string); ok {
			meta("property", "og:image", args["image"])
		}
		meta("property", "article:published_time", og["publishedTime"])
		meta("property", "article:modified_time", og["modifiedTime"])
		for _, tag := range stellarList(og["tags"]) {
			meta("property", "article:tag", tag)
		}
		meta("name", "twitter:card", stellarFirst(args["twitter_card"], "summary"))
		meta("name", "twitter:title", og["twitterTitle"])
		meta("name", "twitter:description", args["description"])
		meta("name", "twitter:creator", args["twitter_id"])
		meta("name", "twitter:site", args["twitter_site"])
		if image, ok := args["image"].(string); ok {
			meta("name", "twitter:image", image)
		}
	}
	ld := seo["jsonLd"]
	if ld == nil {
		pageURL := stellarPublicPageURL(d)
		name, desc := d.Site.Title, d.Site.Description
		if d.Stellar.Layout == "page" {
			name = stellarText(stellarFirst(d.Page.Title, name))
			desc = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(stellarText(stellarFirst(stellarGet(d.Stellar.Data, "page.description"), d.Content)), "")
		}
		author := map[string]any{"@type": "Person", "name": stellarFirst(d.Stellar.Data["authorName"], ""), "sameAs": stellarGet(d.Stellar.Theme, "structuredData.sameAs")}
		if d.Site.Avatar != "" {
			image := d.Site.Avatar
			if strings.HasPrefix(image, "/") {
				image = strings.TrimRight(d.Site.URL, "/") + image
			}
			author["image"] = image
		}
		ld = map[string]any{"@context": "https://schema.org", "@type": "Website", "@id": pageURL, "author": author, "name": name, "description": desc, "url": pageURL}
	}
	if model, ok := ld.(map[string]any); ok && d.Page.CanonicalURL != "" {
		model = copyStellarMap(model)
		model["url"] = d.Page.CanonicalURL
		if model["@id"] != nil {
			model["@id"] = d.Page.CanonicalURL
		}
		if entity, ok := model["mainEntityOfPage"].(map[string]any); ok {
			entity = copyStellarMap(entity)
			entity["@id"] = d.Page.CanonicalURL
			model["mainEntityOfPage"] = entity
		}
		ld = model
	}
	if ld != nil {
		if raw, err := json.Marshal(ld); err == nil {
			fmt.Fprintf(&b, `<script data-stellar-page-meta type="application/ld+json">%s</script>`, raw)
		}
	}
	return template.HTML(b.String())
}

func (r stellarTemplateRenderer) hero(d *StellarRenderData) template.HTML {
	cover := d.Stellar.Cover
	if len(cover) == 0 {
		cover = stellarMap(stellarRenderGet(d, "cover"))
	}
	if !stellarTruth(cover["enabled"]) {
		return ""
	}
	background := stellarMap(cover["background"])
	image := stellarText(background["image"])
	video := strings.TrimSpace(stellarText(background["video"]))
	effect := stellarMap(background["effect"])
	preview := stellarMap(cover["preview"])
	commands := []any{}
	for _, raw := range stellarList(preview["commands"]) {
		if stellarTruth(stellarGet(raw, "codes")) {
			commands = append(commands, raw)
		}
	}
	hasPreview := preview["type"] == "image" && stellarTruth(preview["src"]) || len(commands) > 0
	hasEffect := video == "" && stellarTruth(effect["type"])
	respects := stellarGet(effect, "runtime.respectReducedMotion") != false
	effectBackground := stellarText(stellarFirst(stellarGet(effect, "presentation.background"), "#000000"))
	var b strings.Builder
	class := "cover-content wiki wiki-hero"
	if !hasPreview {
		class += " without-preview"
	}
	fmt.Fprintf(&b, `<div class="%s">`, class)
	class = "wiki-cover-background"
	if hasEffect {
		class += " has-effect"
	}
	if respects {
		class += " respect-reduced-motion"
	}
	if image != "" {
		class += " image"
	}
	if video != "" {
		class += " has-video"
	}
	fmt.Fprintf(&b, `<div class="%s" data-parallax="%s"`, class, stellarEscape(background["parallax"]))
	if hasEffect {
		fmt.Fprintf(&b, ` style="--hero-effect-background:%s"`, stellarEscape(stellarCSSValue(effectBackground)))
	}
	b.WriteString(` aria-hidden="true">`)
	if image != "" {
		for _, class := range []string{"wiki-cover-background-image", "wiki-cover-background-blur"} {
			fmt.Fprintf(&b, `<img class="%s" src="%s" alt="">`, class, stellarEscape(stellarURL(d.Stellar.Root, image)))
		}
	}
	if video != "" {
		fmt.Fprintf(&b, `<video class="wiki-cover-background-video" src="%s"`, stellarEscape(stellarURL(d.Stellar.Root, video)))
		if image != "" {
			fmt.Fprintf(&b, ` poster="%s"`, stellarEscape(stellarURL(d.Stellar.Root, image)))
		}
		b.WriteString(` autoplay muted loop playsinline preload="metadata" disablepictureinpicture tabindex="-1"></video>`)
	}
	if hasEffect {
		resource := copyStellarMap(stellarMap(stellarHeroResources[stellarText(effect["type"])]))
		if module := stellarText(resource["module"]); module != "" {
			resource["module"] = stellarAsset(d.Stellar.Root, strings.Replace(module, "/js/", "/assets/js/", 1))
		}
		for _, key := range []string{"options", "runtime"} {
			if effect[key] == nil {
				effect = copyStellarMap(effect)
				effect[key] = map[string]any{}
			}
		}
		options, _ := json.Marshal(effect["options"])
		runtime, _ := json.Marshal(effect["runtime"])
		assets, _ := json.Marshal(resource)
		fmt.Fprintf(&b, `<canvas data-hero-effect="%s" data-effect-resource="%s" data-effect-options="%s" data-effect-runtime="%s"></canvas>`, stellarEscape(effect["type"]), stellarEscape(string(assets)), stellarEscape(string(options)), stellarEscape(string(runtime)))
	}
	b.WriteString(`<div class="wiki-cover-background-overlay"></div></div><div class="wiki-cover-layout"`)
	if video != "" {
		b.WriteString(` data-text-adaptive="split" style="--adaptive-background:#000000"`)
	} else if image != "" {
		fmt.Fprintf(&b, ` data-text-adaptive="split" style="--cover-url:url('%s')"`, stellarEscape(stellarCSSURL(image)))
	} else if hasEffect {
		fmt.Fprintf(&b, ` data-text-adaptive="split" style="--adaptive-background:%s"`, stellarEscape(stellarCSSValue(effectBackground)))
	}
	fmt.Fprintf(&b, `><nav class="wiki-navbar" aria-label="%s"><a class="wiki-navbar-home" href="%s">%s</a></nav><article class="cover-wrap wiki-cover-content"><section class="wiki-cover-intro">`, stellarEscape(cover["siteName"]), stellarEscape(d.Stellar.Root), stellarEscape(cover["siteName"]))
	if stellarTruth(cover["repository"]) {
		fmt.Fprintf(&b, `<a class="wiki-cover-release ds-ghinfo is-loading" data-blog-service="wiki-release" data-project-name="%s" data-repo="%s" data-api="%s" index="0" target="_blank" rel="external nofollow noopener noreferrer"><span class="wiki-cover-release-value"><span type="text" id="latest-tag-name"></span></span></a>`, stellarEscape(cover["projectName"]), stellarEscape(cover["repository"]), stellarEscape(cover["releaseApi"]))
	}
	if stellarTruth(cover["title"]) {
		fmt.Fprintf(&b, `<h1 class="cover-title" data-text-adaptive="contrast">%s</h1>`, stellarEscape(cover["title"]))
	}
	if stellarTruth(cover["description"]) {
		fmt.Fprintf(&b, `<div class="description">%s</div>`, stellarMarkdown(cover["description"]))
	}
	b.WriteString(`<div class="wiki-cover-actions">`)
	button := func(href, title string, icon any, external bool) {
		attrs := ` rel="noopener noreferrer"`
		if external {
			attrs = ` target="_blank" rel="external nofollow noopener noreferrer"`
		}
		fmt.Fprintf(&b, `<a class="%s" href="%s"%s>%s<span>%s</span></a>`, stellarEscape(r.ui(d, "button", "spotlight")), stellarEscape(stellarURL(d.Stellar.Root, href)), attrs, r.icon(icon), stellarEscape(title))
	}
	if stellarTruth(cover["repository"]) {
		button(stellarText(cover["sourceUrl"]), stellarTranslate(d, "btn.source"), "default:github", true)
	}
	button("#start", stellarTranslate(d, "btn.docs"), "default:documents", false)
	for _, raw := range stellarList(cover["actions"]) {
		item := stellarMap(raw)
		if stellarTruth(item["title"]) && stellarTruth(item["url"]) {
			button(stellarText(item["url"]), stellarText(item["title"]), item["icon"], strings.Contains(stellarText(item["url"]), "://"))
		}
	}
	b.WriteString(`</div></section>`)
	if hasPreview {
		fmt.Fprintf(&b, `<section class="wiki-cover-preview" aria-label="%s">`, stellarEscape(stellarTranslate(d, "meta.project_preview")))
		if preview["type"] == "image" && stellarTruth(preview["src"]) {
			fmt.Fprintf(&b, `<img src="%s" alt="%s">`, stellarEscape(stellarURL(d.Stellar.Root, preview["src"])), stellarEscape(preview["alt"]))
		} else {
			fmt.Fprintf(&b, `<div class="wiki-cover-terminal"><div class="wiki-cover-terminal-toolbar"><div class="wiki-cover-terminal-tabs" role="tablist" aria-label="%s">`, stellarEscape(stellarTranslate(d, "meta.install_method")))
			for i, raw := range commands {
				item := stellarMap(raw)
				class := ""
				if i == 0 {
					class = "active"
				}
				label := stellarFirst(item["label"], stellarTranslate(d, "meta.command", i+1))
				fmt.Fprintf(&b, `<button type="button" role="tab" aria-selected="%t" class="%s" data-codes="%s">%s</button>`, i == 0, class, stellarEscape(item["codes"]), stellarEscape(label))
			}
			fmt.Fprintf(&b, `</div><button type="button" class="wiki-cover-copy" data-copy-message="%s" aria-label="%s">%s<span>%s</span></button></div><pre><code>`, stellarEscape(stellarTranslate(d, "message.copied")), stellarEscape(stellarTranslate(d, "btn.copy")), r.icon("copy:copy"), stellarEscape(stellarTranslate(d, "btn.copy")))
			for _, line := range strings.Split(strings.ReplaceAll(stellarText(stellarGet(commands[0], "codes")), "\r\n", "\n"), "\n") {
				if strings.TrimSpace(line) != "" {
					fmt.Fprintf(&b, `<span>%s</span>`, stellarEscape(line))
				}
			}
			b.WriteString(`</code></pre></div>`)
		}
		b.WriteString(`</section>`)
	}
	b.WriteString(`</article></div></div>`)
	return template.HTML(b.String())
}
func copyStellarMap(m map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	return out
}
func stellarCSSURL(s string) string {
	s = stellarURL("/", s)
	return strings.NewReplacer("'", "%27", "\"", "%22", "\\", "%5C", "\r", "%0D", "\n", "%0A", "<", "%3C", ">", "%3E").Replace(s)
}

func (r stellarTemplateRenderer) footer(d *StellarRenderData) template.HTML {
	foot := stellarMap(d.Stellar.Theme["footer"])
	var b strings.Builder
	fmt.Fprintf(&b, `<footer class="page-footer%s footnote"><hr>`, r.reveal(d))
	groups := stellarList(foot["sitemap"])
	if len(groups) > 0 {
		count := len(groups)
		switch count {
		case 6:
			count = 3
		case 7, 8:
			count = 4
		default:
			if count > 5 {
				count = 5
			}
		}
		fmt.Fprintf(&b, `<div class="sitemap" style="column-count:%d">`, count)
		for _, raw := range groups {
			group := stellarMap(raw)
			items := stellarList(group["items"])
			if len(items) == 0 {
				continue
			}
			fmt.Fprintf(&b, `<div class="sitemap-group"><span class="fs15">%s</span>`, stellarEscape(group["title"]))
			for _, item := range items {
				title, href := stellarMarkdownLink(stellarText(item))
				fmt.Fprintf(&b, `<a href="%s">%s</a>`, stellarEscape(stellarURL(d.Stellar.Root, href)), stellarEscape(stellarTranslate(d, title)))
			}
			b.WriteString(`</div>`)
		}
		b.WriteString(`</div>`)
	}
	content := stellarText(foot["content"])
	author := stellarFirst(d.Stellar.Data["authorName"], stellarGet(d.Site, "Author"), "Anonymity")
	content = strings.NewReplacer("{author.name}", stellarText(author), "{theme.name}", "Stellar", "{theme.version}", "2.0.0", "{theme.tree}", "https://github.com/xaoxuu/hexo-theme-stellar/tree/2.0.0").Replace(content)
	fmt.Fprintf(&b, `<div class="text">%s</div></footer>`, stellarMarkdown(content))
	return template.HTML(b.String())
}
func stellarMarkdownLink(s string) (string, string) {
	if strings.HasPrefix(s, "[") {
		if i := strings.Index(s, "]("); i >= 0 && strings.HasSuffix(s, ")") {
			return s[1:i], s[i+2 : len(s)-1]
		}
	}
	return s, s
}
func (r stellarTemplateRenderer) comments(d *StellarRenderData) template.HTML {
	model := stellarGet(d.Stellar.Client, "Comments")
	if !stellarTruth(stellarGet(model, "enabled")) {
		return ""
	}
	service := stellarText(stellarGet(model, "service"))
	if service != "artalk" && service != "waline" && service != "twikoo" && service != "giscus" && service != "beaudar" && service != "utterances" {
		return ""
	}
	title := stellarGet(model, "title")
	if title == nil {
		title = stellarTranslate(d, "btn.comments")
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<div class="related-wrap md-text%s" id="comments">`, r.reveal(d))
	if stellarTruth(title) {
		fmt.Fprintf(&b, `<section class="header cmt-title cap theme">%s</section>`, stellarMarkdown(title))
	}
	fmt.Fprintf(&b, `<section class="body cmt-body %s">`, service)
	id := stellarText(stellarGet(model, "id"))
	if service == "artalk" || service == "waline" || service == "twikoo" {
		class := map[string]string{"artalk": "artalk with-fancybox", "waline": "waline_thread with-fancybox", "twikoo": "with-fancybox"}[service]
		fmt.Fprintf(&b, `<div id="%s_container" class="%s"`, service, class)
		if id != "" {
			fmt.Fprintf(&b, ` comment_id="%s"`, stellarEscape(id))
		}
		b.WriteString(`><div class="lazy-icon"></div></div>`)
	} else {
		fmt.Fprintf(&b, `<div class="lazy-icon"></div><div id="%s"`, service)
		options := stellarMap(stellarGet(model, "options"))
		for key, value := range options {
			if value != nil && stellarCommentAttr(key) {
				fmt.Fprintf(&b, ` %s="%s"`, key, stellarEscape(value))
			}
		}
		b.WriteString(`></div>`)
	}
	b.WriteString(`</section></div>`)
	return template.HTML(b.String())
}
func stellarCommentAttr(name string) bool {
	if strings.HasPrefix(strings.ToLower(name), "on") || name == "style" || name == "id" || name == "class" {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return name != ""
}

// Keep query encoding separate from URL joining for Notebook tag links.
func stellarQuery(value any) string { return url.QueryEscape(stellarText(value)) }

func stellarPublicPageURL(d *StellarRenderData) string {
	if d.Page.CanonicalURL != "" {
		return d.Page.CanonicalURL
	}
	base, err := url.Parse(d.Site.URL)
	if err != nil || base.Host == "" {
		return ""
	}
	path, err := url.Parse(stellarURL(d.Stellar.Root, stellarPagePath(d)))
	if err != nil {
		return ""
	}
	return base.ResolveReference(path).String()
}

func stellarRemoteMarkdown(d *StellarRenderData, src string) template.HTML {
	base := ""
	u, err := url.Parse(src)
	configured, configErr := url.Parse(stellarText(stellarGet(d.Stellar.Theme, "services.github.rawUrl")))
	configuredOrigin := ""
	if configErr == nil && configured.IsAbs() && configured.Host != "" {
		configuredOrigin = configured.Scheme + "://" + configured.Host
	}
	if err == nil && u.IsAbs() && u.Host != "" && (u.Hostname() == "raw.githubusercontent.com" || u.Scheme+"://"+u.Host == configuredOrigin) {
		segments := []string{}
		for _, part := range strings.Split(u.EscapedPath(), "/") {
			if part != "" {
				segments = append(segments, part)
			}
		}
		if len(segments) >= 3 {
			origin := configuredOrigin
			if origin == "" {
				origin = "https://" + u.Hostname()
			}
			base = origin + "/" + strings.Join(segments[:3], "/") + "/"
			src = base + strings.Join(segments[3:], "/")
		}
	}
	attrs := ` src="` + stellarEscape(src) + `"`
	if base != "" {
		attrs += ` data-base="` + stellarEscape(base) + `"`
	}
	return template.HTML(`<div class="data-service ds-mdrender"` + attrs + ` data-heading="true"></div>`)
}
