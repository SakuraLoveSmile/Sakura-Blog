package web

import (
	"fmt"
	"html/template"
	"net/url"
	"sort"
	"strings"

	"blog/internal/settings"
	stellari18n "blog/modules/stellar/i18n"
)

func stellarTranslate(d *StellarRenderData, key string, args ...any) string {
	return stellari18n.Text(d.Stellar.Language, key, args...)
}
func stellarRenderGet(d *StellarRenderData, path string) any {
	if d.Stellar.Model == nil {
		return nil
	}
	return stellarGet(d.Stellar.Model.Render, path)
}
func stellarItemGet(d *StellarRenderData, path string) any {
	if d.Stellar.Model == nil {
		return nil
	}
	return stellarGet(d.Stellar.Model.Item, path)
}
func stellarCollectionGet(d *StellarRenderData, path string) any {
	if d.Stellar.Model == nil {
		return nil
	}
	return stellarGet(d.Stellar.Model.Collection, path)
}
func stellarPagePath(d *StellarRenderData) string {
	return stellarText(stellarFirst(stellarItemGet(d, "route.path"), stellarGet(d.Page, "Path")))
}
func (r stellarTemplateRenderer) reveal(d *StellarRenderData) string {
	if stellarGet(d.Stellar.Theme, "features.reveal.enabled") == true {
		return " slide-up"
	}
	return ""
}
func (r stellarTemplateRenderer) frame(d *StellarRenderData, class, title string, body, footer, action template.HTML, attrs map[string]any, reveal bool) template.HTML {
	body = template.HTML(strings.TrimSpace(string(body)))
	footer = template.HTML(strings.TrimSpace(string(footer)))
	if strings.TrimSpace(string(body)) == "" && strings.TrimSpace(string(footer)) == "" {
		return ""
	}
	classes := "widget-wrapper"
	if reveal {
		classes += r.reveal(d)
	}
	if class != "" {
		classes += " " + class
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<widget class="%s"`, stellarEscape(classes))
	for _, key := range []string{"id", "collapse", "data-wiki", "data-notebook", "data-expand-active"} {
		if value := attrs[key]; value != nil && value != false {
			fmt.Fprintf(&b, ` %s="%s"`, key, stellarEscape(value))
		}
	}
	b.WriteString(`>`)
	if title != "" || action != "" {
		b.WriteString(`<div class="widget-header dis-select">`)
		if title != "" {
			fmt.Fprintf(&b, `<span class="name">%s</span>`, stellarEscape(title))
		}
		fmt.Fprintf(&b, `%s</div>`, action)
	}
	if body != "" {
		bodyClass := "widget-body"
		if class == "markdown" || class == "timeline" {
			bodyClass += " fs14"
		}
		fmt.Fprintf(&b, `<div class="%s">%s</div>`, bodyClass, body)
	}
	b.WriteString(string(footer))
	b.WriteString(`</widget>`)
	return template.HTML(b.String())
}
func (r stellarTemplateRenderer) search(d *StellarRenderData, placement string, hover bool) template.HTML {
	if !d.Stellar.SearchEnabled {
		return ""
	}
	label := stellarTranslate(d, "btn.search")
	iconOnly := placement == "topbar" || placement == "wiki-toolbar"
	class := "search-trigger"
	if hover {
		class += " has-hover-surface"
	}
	attrs := map[string]any{"title": label, "aria-label": label, "data-shell-action": "open-search", "data-search-instance": "leftbar-brand-search"}
	filter := stellarText(stellarRenderGet(d, "layout.algoliaFilterPath"))
	if filter != "" && stellarGet(d.Stellar.Theme, "search.provider") == "algolia" {
		attrs["data-algolia-filter-path"] = "/" + strings.TrimLeft(filter, "/")
	}
	if stellarGet(d.Stellar.Client, "ClientData.ctx.search.service") == "local_search" {
		profile := stellarText(stellarCollectionGet(d, "profile"))
		id := stellarText(stellarCollectionGet(d, "id"))
		label := stellarText(stellarCollectionGet(d, "identity.name"))
		if profile == "" && d.Stellar.Layout == "notes" {
			profile = "notebook"
			id = stellarText(d.Stellar.Data["notebookId"])
			label = stellarText(d.Stellar.Data["notebookName"])
		}
		scope := stellarText(stellarFirst(stellarGet(d.Stellar.Theme, "search.local.scope"), "all"))
		current := ""
		hasBlog := false
		if (profile == "wiki" || profile == "notebook" || profile == "topic") && id != "" {
			kind := "page"
			if profile == "topic" {
				kind = "post"
			}
			if scope == "all" || scope == kind {
				current = profile + ":" + id
				hasBlog = profile == "topic"
				if label == "" {
					label = id
				}
			}
		} else if profile == "post" || d.Stellar.Profile == "home" || d.Stellar.Profile == "blogIndex" {
			if scope == "all" || scope == "post" {
				current = "blog"
				label = stellarTranslate(d, "search.scope_blog")
				hasBlog = true
			}
		}
		if current != "" {
			attrs["data-search-domain"] = current
			attrs["data-search-domain-label"] = label
			attrs["data-search-domain-blog"] = fmt.Sprint(hasBlog)
		}
	}

	after := template.HTML("")
	if !iconOnly {
		after = `<span class="search-shortcut" aria-hidden="true"><span class="search-shortcut__modifier"></span><span class="search-shortcut__key">K</span></span>`
	}
	return r.collection(d, map[string]any{"items": []any{map[string]any{"type": "button", "title": label, "icon": "default:search", "iconOnly": iconOnly, "className": class, "attrs": attrs, "after": after}}})
}
func (r stellarTemplateRenderer) brand(d *StellarRenderData, placement string, brand map[string]any) template.HTML {
	if !stellarTruth(brand["name"]) && !stellarTruth(stellarGet(brand, "image.src")) {
		return ""
	}
	style := "regular"
	if placement == "topbar" {
		style = "topbar"
	} else if placement == "main" || brand["style"] == "compact" {
		style = "compact"
	}
	class := "brand-header brand-header--" + style
	if brand["source"] != "collection" {
		class += " brand-header--site"
	}
	if placement == "main" {
		class += " mobile-only"
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<header class="%s">`, class)
	if placement == "leftbar" && brand["source"] == "collection" && brand["backButton"] == true && stellarTruth(brand["backHref"]) {
		label := stellarTranslate(d, stellarText(brand["backLabel"]))
		fmt.Fprintf(&b, `<div class="brand-navigation"><a class="brand-navigation__back cap" href="%s"><span class="ui-icon" aria-hidden="true">%s</span><span>%s</span></a></div>`, stellarEscape(stellarURL(d.Stellar.Root, brand["backHref"])), r.icon("default:chevron-left"), stellarEscape(label))
		if stellarCollectionGet(d, "profile") == "wiki" {
			fmt.Fprintf(&b, `<div class="brand-navigation-rail">%s</div>`, r.navigation(d, "leftbar", 1, []any{map[string]any{"href": brand["backHref"], "title": label, "icon": "default:chevron-left", "iconOnly": true, "attrs": map[string]any{"aria-label": label}}}))
		}
	}
	b.WriteString(`<div class="brand-wrap">`)
	href := stellarText(brand["href"])
	tag := "div"
	attrs := ""
	if href != "" {
		tag = "a"
		attrs = ` href="` + stellarEscape(stellarURL(d.Stellar.Root, href)) + `"`
	}
	if src := stellarText(stellarGet(brand, "image.src")); src != "" {
		variant := stellarText(stellarFirst(stellarGet(brand, "image.variant"), "icon"))
		fmt.Fprintf(&b, `<%s class="brand-image brand-image--%s"%s><img no-lazy src="%s" alt=""></%s>`, tag, stellarEscape(variant), attrs, stellarEscape(stellarURL(d.Stellar.Root, src)), tag)
	}
	if stellarTruth(brand["name"]) {
		fmt.Fprintf(&b, `<%s class="brand-title"%s><div class="brand-name">%s</div>`, tag, attrs, stellarEscape(brand["name"]))
		if style != "topbar" && stellarTruth(brand["tagline"]) {
			fmt.Fprintf(&b, `<div class="brand-tagline cap">%s</div>`, stellarEscape(brand["tagline"]))
		}
		fmt.Fprintf(&b, `</%s>`, tag)
	}
	if placement == "leftbar" && style == "regular" {
		b.WriteString(string(r.brandGithub(d, brand)))
	}
	b.WriteString(`</div>`)
	if placement == "leftbar" && brand["search"] == true {
		search := r.search(d, placement, true)
		if search != "" {
			fmt.Fprintf(&b, `<div class="brand-search">%s</div>`, search)
		}
	}
	b.WriteString(`</header>`)
	return template.HTML(b.String())
}
func (r stellarTemplateRenderer) brandGithub(d *StellarRenderData, brand map[string]any) template.HTML {
	base := strings.TrimRight(stellarText(stellarGet(d.Stellar.Theme, "services.github.apiUrl")), "/")
	if base == "" {
		return ""
	}
	repo := strings.TrimSpace(stellarText(brand["ghrepo"]))
	user := strings.TrimSpace(stellarText(brand["ghuser"]))
	var api, href string
	var entries []map[string]string
	if repo != "" {
		parts := strings.Split(repo, "/")
		for i := range parts {
			parts[i] = url.PathEscape(parts[i])
		}
		repo = strings.Join(parts, "/")
		api = base + "/repos/" + repo
		href = "https://github.com/" + repo
		entries = []map[string]string{{"key": "stargazers_count", "label": "stars", "href": href + "/stargazers"}, {"key": "forks_count", "label": "forks", "href": href + "/forks"}, {"key": "latest-tag-name", "label": "tag", "href": href + "/tags", "api": api + "/tags"}}
	} else if user != "" {
		user = url.PathEscape(user)
		api = base + "/users/" + user
		href = "https://github.com/" + user
		entries = []map[string]string{{"key": "followers", "label": "followers", "href": href + "?tab=followers"}, {"key": "following", "label": "following", "href": href + "?tab=following"}, {"key": "public_repos", "label": "repos", "href": href + "?tab=repositories"}}
	} else {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<div class="brand-stats data-service ds-ghinfo" data-api="%s">`, stellarEscape(api))
	for _, e := range entries {
		class := r.ui(d, "brand-stat")
		attrs := ""
		if e["api"] != "" {
			class += " data-service ds-ghinfo"
			attrs = ` data-api="` + stellarEscape(e["api"]) + `" index="0"`
		}
		fmt.Fprintf(&b, `<a class="%s"%s href="%s"><span class="brand-stat__value" type="text" id="%s"></span><span class="brand-stat__label">%s</span></a>`, stellarEscape(class), attrs, stellarEscape(e["href"]), e["key"], e["label"])
	}
	b.WriteString(`</div>`)
	return template.HTML(b.String())
}
func (r stellarTemplateRenderer) navigation(d *StellarRenderData, placement string, columns int, entries []any) template.HTML {
	layout, variant := "list", "nav"
	if placement == "leftbar" && columns > 1 {
		layout = "grid"
	}
	if placement == "leftbar" && columns > 2 {
		variant = "icon"
	}
	body := r.collection(d, map[string]any{"layout": layout, "variant": variant, "columns": columns, "items": entries})
	if body == "" {
		return ""
	}
	class := "menu dis-select"
	if placement == "main" {
		class += " mobile-hidden"
	}
	return template.HTML(`<nav class="` + class + `">` + string(body) + `</nav>`)
}
func (r stellarTemplateRenderer) menu(d *StellarRenderData, region string, state map[string]any) template.HTML {
	columns := stellarInt(state["menuColumns"], 1)
	if columns < 1 || columns > 5 {
		columns = 1
	}
	active := stellarText(stellarItemGet(d, "navigation.menu"))
	pagePath := stellarURL(d.Stellar.Root, stellarPagePath(d))
	entries := []any{}
	// Longest path match wins; the normalized model already respects activeMenu.
	if d.Stellar.Model == nil {
		best := 0
		for _, raw := range stellarList(state["menu"]) {
			item := stellarMap(raw)
			path := stellarURL(d.Stellar.Root, item["url"])
			if (pagePath == path || path != "/" && strings.HasPrefix(pagePath, strings.TrimRight(path, "/")+"/")) && len(path) > best {
				best = len(path)
				active = stellarText(item["id"])
			}
		}
	}
	for _, raw := range stellarList(state["menu"]) {
		item := stellarMap(raw)
		if item["id"] == nil || item["url"] == nil || item["title"] == nil {
			continue
		}
		title := stellarTranslate(d, stellarText(item["title"]))
		attrs := map[string]any{}
		if region == "leftbar" && columns > 2 {
			attrs["aria-label"] = title
		}
		class := ""
		if stellarTruth(item["accent"]) {
			class = "is-themed"
		}
		entries = append(entries, map[string]any{"href": item["url"], "title": title, "icon": item["icon"], "attrs": attrs, "theme": item["accent"], "active": stellarText(item["id"]) == active, "className": class})
	}
	return r.navigation(d, region, columns, entries)
}
func (r stellarTemplateRenderer) settings(d *StellarRenderData, placement string) template.HTML {
	provider := stellarText(stellarGet(d.Stellar.Theme, "comments.provider"))
	identity := provider == "artalk" || provider == "waline" || provider == "twikoo"
	label := stellarTranslate(d, "settings.label")
	path := stellarText(stellarFirst(d.Stellar.SettingsPath, stellarGet(d.Stellar.Theme, "profiles.settings.path"), "/settings/"))
	fallback := stellarGet(d.Stellar.Theme, "fallbacks.avatar")
	fallbackURL := ""
	if stellarTruth(fallback) {
		fallbackURL = stellarURL(d.Stellar.Root, fallback)
	}
	return template.HTML(fmt.Sprintf(`<a class="%s" href="%s" data-profile-provider="%s" data-profile-identity-enabled="%t" data-profile-fallback-avatar="%s" data-settings-placement="%s" data-settings-label="%s" aria-label="%s" title="%s"><span class="settings-widget__avatar" aria-hidden="true"><span class="ui-icon">%s</span></span><span class="settings-widget__name ui-collection__title">%s</span></a>`, stellarEscape(r.ui(d, "settings-widget", "collectionItem")), stellarEscape(stellarURL(d.Stellar.Root, path)), stellarEscape(provider), identity, stellarEscape(fallbackURL), stellarEscape(placement), stellarEscape(label), stellarEscape(label), stellarEscape(label), r.icon("default:settings"), stellarEscape(label)))
}
func (r stellarTemplateRenderer) action(d *StellarRenderData, item map[string]any, dropdown bool) template.HTML {
	typeName := stellarText(item["type"])
	class := r.ui(d, "social", "collectionItem", "is-icon-only")
	if dropdown {
		class = r.ui(d, "dropdown-item", "collectionItem")
	}
	title := stellarText(item["title"])
	body := `<span class="ui-icon">` + string(r.icon(item["icon"])) + `</span>`
	if dropdown {
		body += `<span class="ui-collection__content"><span class="ui-collection__title">` + stellarEscape(title) + `</span></span>`
	}
	if typeName == "button" {
		return template.HTML(`<button type="button" class="` + stellarEscape(class) + `" title="` + stellarEscape(title) + `" aria-label="` + stellarEscape(title) + `" onclick="` + stellarEscape(item["onclick"]) + `">` + body + `</button>`)
	}
	if typeName == "link" {
		attrs := ` rel="noopener noreferrer"`
		if strings.Contains(stellarText(item["url"]), "://") {
			attrs = ` target="_blank" rel="external nofollow noopener noreferrer"`
		}
		return template.HTML(`<a class="` + stellarEscape(class) + `" title="` + stellarEscape(title) + `" href="` + stellarEscape(stellarURL(d.Stellar.Root, item["url"])) + `"` + attrs + `>` + body + `</a>`)
	}
	return ""
}
func (r stellarTemplateRenderer) actions(d *StellarRenderData, items []any) template.HTML {
	var b strings.Builder
	for _, raw := range items {
		item := stellarMap(raw)
		switch item["type"] {
		case "spacer":
			b.WriteString(`<span class="social-spacer" aria-hidden="true"></span>`)
		case "dropdown":
			var children strings.Builder
			for _, child := range stellarList(item["items"]) {
				c := stellarMap(child)
				if !stellarTruth(c["title"]) || c["type"] == "link" && !stellarTruth(c["url"]) || c["type"] == "button" && !stellarTruth(c["onclick"]) {
					continue
				}
				children.WriteString(string(r.action(d, c, true)))
			}
			if stellarTruth(item["icon"]) && children.Len() > 0 {
				fmt.Fprintf(&b, `<details class="dropdown social-dropdown"><summary class="%s" title="%s" aria-label="%s"><span class="ui-icon">%s</span></summary><div class="dropdown-menu ui-collection" data-layout="list" data-variant="nav" data-density="compact">%s</div></details>`, stellarEscape(r.ui(d, "dropdown-trigger", "interactive", "social", "collectionItem", "is-icon-only")), stellarEscape(item["title"]), stellarEscape(item["title"]), r.icon(item["icon"]), children.String())
			}
		default:
			b.WriteString(string(r.action(d, item, false)))
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return template.HTML(`<div class="region-actions social-wrap ui-collection" data-layout="inline" data-variant="nav" data-density="compact">` + b.String() + `</div>`)
}

// regions renders each region first, so the shell's data-regions reflects
// genuinely non-empty bodies, including unavailable data-driven widgets.
func (r stellarTemplateRenderer) regions(d *StellarRenderData) (map[string]any, error) {
	out := map[string]any{}
	names := []string{}
	for _, name := range []string{"topbar", "leftbar", "rightbar"} {
		state := stellarMap(d.Stellar.Regions[name])
		if state["enabled"] == false {
			continue
		}
		var body strings.Builder
		brand := r.brand(d, name, stellarMap(state["brand"]))
		stack, err := r.widgetStack(d, name, d.Stellar.Widgets[name])
		if err != nil {
			return nil, err
		}
		if name == "topbar" {
			if brand != "" {
				fmt.Fprintf(&body, `<div class="widget-instance widget-instance--brand" data-widget-id="topbar:fixed:brand" data-presentation="topbar">%s</div>`, brand)
			}
			body.WriteString(string(stack))
		} else if name == "leftbar" {
			// Match the official fixed leftbar zones, which always include settings.
			body.WriteString(`<header class="site-region__header site-region__zone site-region__zone--top">`)
			if brand != "" {
				fmt.Fprintf(&body, `<div class="widget-instance widget-instance--brand" data-widget-id="leftbar:fixed:brand" data-presentation="leftbar" data-leftbar-rail>%s</div>`, brand)
			}
			body.WriteString(`</header><div class="site-region__body site-region__zone site-region__zone--body">`)
			if menu := r.menu(d, name, state); menu != "" {
				fmt.Fprintf(&body, `<div class="widget-instance widget-instance--menu" data-widget-id="leftbar:fixed:menu" data-presentation="leftbar" data-leftbar-rail>%s</div>`, menu)
			}
			body.WriteString(string(stack))
			body.WriteString(`</div><footer class="site-region__footer">`)
			if actions := r.actions(d, stellarList(stellarGet(state, "footer.actions"))); actions != "" {
				fmt.Fprintf(&body, `<div class="site-region__footer-actions"><div class="widget-instance widget-instance--actions" data-widget-id="leftbar:fixed:actions" data-presentation="leftbar">%s</div></div>`, actions)
			}
			fmt.Fprintf(&body, `<div class="site-region__system ui-collection" data-layout="inline" data-variant="nav" data-density="compact"><div class="site-region__settings"><div class="widget-instance widget-instance--settings" data-widget-id="leftbar:fixed:settings" data-presentation="leftbar" data-leftbar-rail>%s</div></div><button type="button" class="%s" aria-label="Collapse leftbar" aria-expanded="true" data-shell-action="toggle-leftbar"><span class="ui-icon">%s</span></button></div></footer>`, r.settings(d, name), stellarEscape(r.ui(d, "leftbar-state-toggle", "collectionItem", "is-icon-only")), r.icon("default:leftbar-toggle"))
		} else {
			body.WriteString(string(stack))
		}
		if strings.TrimSpace(body.String()) != "" {
			out[name] = template.HTML(body.String())
			names = append(names, name)
		}
	}
	out["names"] = strings.Join(names, " ")
	return out, nil
}
func (r stellarTemplateRenderer) widgetStack(d *StellarRenderData, region string, widgets []StellarWidget) (template.HTML, error) {
	if len(widgets) == 0 {
		return "", nil
	}
	var b strings.Builder
	for _, w := range widgets {
		state := w.State
		layout := stellarText(state["layout"])
		var body template.HTML
		if layout == "tagcloud" {
			var err error
			body, err = r.tagcloud(d, stellarMap(state["item"]))
			if err != nil {
				return "", err
			}
		} else {
			body = r.widget(d, region, layout, stellarMap(state["item"]), w.Widget)
		}
		if body == "" {
			continue
		}
		rail := ""
		for _, p := range stellarList(state["presentations"]) {
			if p == "leftbarRail" {
				rail = " data-leftbar-rail"
			}
		}
		fmt.Fprintf(&b, `<div class="widget-instance widget-instance--%s" data-widget-id="%s" data-presentation="%s"%s>%s</div>`, stellarEscape(layout), stellarEscape(state["instanceId"]), stellarEscape(state["presentation"]), rail, body)
	}
	if b.Len() == 0 {
		return "", nil
	}
	zone := region
	if region == "leftbar" {
		zone = "body"
	}
	return template.HTML(`<div class="widgets widget-stack widget-stack--` + region + ` widget-stack--` + zone + `">` + b.String() + `</div>`), nil
}

func (r stellarTemplateRenderer) widget(d *StellarRenderData, region, layout string, item map[string]any, host *settings.WidgetInstance) template.HTML {
	state := stellarMap(d.Stellar.Regions[region])
	switch layout {
	case "menu":
		return r.menu(d, region, state)
	case "settings":
		return r.settings(d, region)
	case "spacer":
		return `<span class="topbar-spacer" aria-hidden="true"></span>`
	case "actions":
		return r.actions(d, stellarList(item["actions"]))
	case "markdown":
		var b strings.Builder
		if stellarTruth(item["content"]) {
			b.WriteString(string(stellarMarkdown(d, item["content"])))
		}
		if stellarTruth(item["src"]) {
			b.WriteString(string(stellarRemoteMarkdown(d, stellarText(item["src"]))))
		}
		if stellarTruth(item["linklist"]) {
			b.WriteString(string(r.linklist(d, stellarMap(item["linklist"]))))
		}
		return r.frame(d, "markdown", stellarText(item["title"]), template.HTML(b.String()), "", "", nil, true)
	case "linklist":
		return r.frame(d, "link-list-widget", stellarText(item["title"]), r.linklist(d, item), "", "", nil, false)
	case "toc":
		content := stellarText(stellarFirst(stellarItemGet(d, "content"), d.Content))
		body := ExtractTOCWith(content, TOCOptions{ListNumber: stellarTruth(item["list_number"]), MinDepth: stellarInt(item["min_depth"], 1), MaxDepth: stellarInt(item["max_depth"], 6)})
		if body != "" {
			body = template.HTML(strings.ReplaceAll(strings.Replace(string(body), `class="toc"`, `class="toc ui-collection-adapter"`, 1), `class="toc-link`, `class="toc-link ui-interactive`))
		}
		if region == "topbar" {
			if body == "" {
				return ""
			}
			return template.HTML(`<details class="topbar-toc"><summary class="` + stellarEscape(r.ui(d, "region-compact-action")) + `" aria-label="` + stellarEscape(stellarTranslate(d, "btn.toc")) + `"><span class="ui-icon">` + string(r.icon("default:rightbar")) + `</span></summary><div class="topbar-toc-popover">` + string(body) + `</div></details>`)
		}
		remote := stellarTruth(stellarRenderGet(d, "article.readmeHtml"))
		comments := stellarTruth(stellarGet(d.Stellar.Client, "Comments.Enabled")) || d.Stellar.BlogNativeComments
		if body == "" && !remote && !comments {
			return ""
		}
		title := ""
		action := template.HTML("")
		if body != "" || remote {
			title = stellarTranslate(d, "meta.toc")
			action = template.HTML(`<button type="button" class="` + stellarEscape(r.ui(d, "cap-action")) + `" data-shell-action="toggle-toc" aria-label="Toggle table of contents" aria-pressed="false"><span class="ui-icon">` + string(r.icon("default:rightbar")) + `</span></button>`)
			if body == "" {
				body = `<nav class="toc ui-collection-adapter"></nav>`
			}
		}
		footer := `<div class="widget-footer"><a class="top ui-interactive" onclick="util.scrollTop()"><span class="ui-icon">` + string(r.icon("default:to-top")) + `</span><span>` + stellarEscape(stellarTranslate(d, "btn.top")) + `</span></a>`
		if stellarTruth(stellarGet(d.Stellar.Client, "Comments.Enabled")) {
			footer += `<a class="buttom ui-interactive" onclick="util.scrollComment()"><span class="ui-icon">` + string(r.icon("default:to-comment")) + `</span><span>` + stellarEscape(stellarTranslate(d, "btn.comments")) + `</span></a>`
		}
		if d.Stellar.BlogNativeComments {
			footer += `<a class="buttom ui-interactive" data-blog-action="scroll-comments" data-comments-target="blog-comments" href="#blog-comments"><span class="ui-icon">` + string(r.icon("default:to-comment")) + `</span><span>` + stellarEscape(stellarTranslate(d, "btn.comments")) + `</span></a>`
		}
		footer += `</div>`
		collapse := stellarText(item["collapse"])
		if collapse != "auto" {
			collapse = fmt.Sprint(item["collapse"] == true)
		}
		return r.frame(d, "toc", title, body, template.HTML(footer), action, map[string]any{"id": "data-toc", "collapse": collapse}, true)
	case "recent":
		entries := []any{}
		source := stellarFirst(stellarRenderGet(d, "layout.recentItems"), d.Stellar.Data["recentItems"])
		for _, raw := range stellarList(source) {
			v := stellarMap(raw)
			entries = append(entries, map[string]any{"href": stellarFirst(v["href"], v["link"], v["path"]), "title": v["title"], "prefix": v["collectionName"], "indicator": false})
		}
		if source == nil {
			recent := d.Recent
			if len(d.WikiRecentDocs) > 0 {
				recent = d.WikiRecentDocs
			}
			for _, p := range recent {
				entries = append(entries, map[string]any{"href": p.URL, "title": p.Title, "indicator": false})
			}
		}
		limit := stellarInt(item["limit"], 5)
		if limit >= 0 && len(entries) > limit {
			entries = entries[:limit]
		}
		action := template.HTML("")
		if stellarTruth(item["rss"]) {
			action = template.HTML(`<a class="cap-action" id="rss" title="Subscribe" href="` + stellarEscape(stellarURL(d.Stellar.Root, item["rss"])) + `"><span class="ui-icon">` + string(r.icon("default:rss")) + `</span></a>`)
		}
		return r.frame(d, "recent-widget", stellarTranslate(d, "meta.recent_update"), r.collection(d, map[string]any{"density": "compact", "items": entries}), "", action, nil, true)
	case "tree":
		return r.docTree(d)
	case "tagtree":
		return r.tagTree(d, item)
	case "related":
		return r.related(d)

	case "author":
		return r.authorWidget(d, item, host)
	case "ghrepo", "ghuser", "ghissues", "timeline":
		return r.dynamicWidget(d, layout, item)
	}
	return ""
}
func (r stellarTemplateRenderer) linklist(d *StellarRenderData, item map[string]any) template.HTML {
	entries := []any{}
	for _, raw := range stellarList(item["items"]) {
		v := stellarMap(raw)
		if !stellarTruth(v["title"]) || !stellarTruth(v["url"]) {
			continue
		}
		entries = append(entries, map[string]any{"href": v["url"], "title": v["title"], "icon": v["icon"], "active": stellarURL(d.Stellar.Root, v["url"]) == stellarURL(d.Stellar.Root, stellarPagePath(d)), "showTitle": item["show_title"] != false})
	}
	layout := "list"
	columns := 1
	if item["view"] == "grid" {
		layout = "grid"
		columns = stellarInt(item["columns"], 2)
	}
	variant := "nav"
	if item["show_title"] == false {
		variant = "icon"
	}
	return r.collection(d, map[string]any{"items": entries, "layout": layout, "columns": columns, "variant": variant})
}
func (r stellarTemplateRenderer) docTree(d *StellarRenderData) template.HTML {
	sections := stellarList(stellarCollectionGet(d, "navigation.tree"))
	total := 0
	for _, raw := range sections {
		total += len(stellarList(stellarGet(raw, "items")))
	}
	if total <= 1 {
		return ""
	}
	var b strings.Builder
	for _, raw := range sections {
		section := stellarMap(raw)
		entries := []any{}
		for _, v := range stellarList(section["items"]) {
			m := stellarMap(v)
			if !stellarTruth(m["title"]) {
				continue
			}
			entries = append(entries, map[string]any{"href": m["path"], "title": m["title"], "active": m["path"] == stellarItemGet(d, "route.path")})
		}
		body := r.collection(d, map[string]any{"density": "compact", "items": entries})
		if body != "" {
			b.WriteString(`<section class="widget-section">`)
			if stellarTruth(section["title"]) {
				fmt.Fprintf(&b, `<div class="widget-header dis-select"><span class="name">%s</span></div>`, stellarEscape(section["title"]))
			}
			b.WriteString(string(body))
			b.WriteString(`</section>`)
		}
	}
	return r.frame(d, "doc-tree-widget", "", template.HTML(b.String()), "", "", map[string]any{"data-wiki": stellarCollectionGet(d, "id")}, false)
}
func (r stellarTemplateRenderer) tagTree(d *StellarRenderData, item map[string]any) template.HTML {
	items := stellarList(stellarFirst(stellarRenderGet(d, "layout.tagTree"), d.Stellar.Data["tagTree"]))
	tree := map[string]map[string]any{}
	for _, raw := range items {
		tag := stellarMap(raw)
		tree[stellarText(tag["id"])] = tag
	}
	if tree[""] == nil {
		return ""
	}
	active := stellarText(d.Stellar.Data["activeTag"])
	seen := map[string]bool{}
	var visit func(string, int) string
	visit = func(id string, depth int) string {
		tag := tree[id]
		if tag == nil || seen[id] {
			return ""
		}
		seen[id] = true
		children := stellarList(tag["children"])
		leaf := id == "" || len(children) == 0
		class := " is-parent"
		if leaf {
			class = " is-leaf"
		}
		if stellarTruth(item["expand_all"]) || stellarTruth(item["expand_active"]) && active == id || strings.HasPrefix(active, id+"/") {
			class += " is-expanded"
		}
		icon := ""
		if id != "" && stellarTruth(item["show_tagcon"]) {
			icons := stellarMap(stellarGet(d.Stellar.Theme, "profiles.notebook.tagIcons"))
			icon = stellarText(stellarFirst(icons[stellarText(tag["name"])], icons[id], icons[stellarText(tag["label"])], icons[strings.ToLower(stellarText(tag["label"]))], "quot:hashtag"))
		}
		label := stellarText(tag["label"])
		if id == "" {
			label = stellarTranslate(d, "meta.all_notes")
		}
		after := template.HTML("")
		if !leaf {
			after = `<span class="tag-switcher-wrapper"><span class="tag-switcher"></span></span>`
		}
		entry := r.collection(d, map[string]any{"items": []any{map[string]any{"href": tag["path"], "title": label, "icon": icon, "active": id == active, "indicator": false, "depth": depth, "className": "tag-tree__link", "after": after}}})
		content := string(entry)
		start := strings.Index(content, ">")
		end := strings.LastIndex(content, "</div>")
		if start >= 0 && end > start {
			content = content[start+1 : end]
		}
		out := `<div class="tag-subtree` + class + `" data-tag="` + stellarEscape(id) + `">` + content
		if !leaf {
			for _, child := range children {
				out += visit(stellarText(child), depth+1)
			}
		}
		return out + `</div>`
	}
	body := `<div class="ui-collection tag-tree" data-layout="list" data-variant="nav" data-density="compact">` + visit("", 0)
	for _, child := range stellarList(tree[""]["children"]) {
		body += visit(stellarText(child), 0)
	}
	body += `</div>`
	return r.frame(d, "tag-tree-widget", stellarTranslate(d, "meta.tag_tree"), template.HTML(body), "", "", map[string]any{"data-notebook": stellarFirst(stellarCollectionGet(d, "id"), d.Stellar.Data["notebookId"]), "data-expand-active": fmt.Sprint(stellarTruth(item["expand_active"]))}, true)
}
func (r stellarTemplateRenderer) related(d *StellarRenderData) template.HTML {
	if stellarCollectionGet(d, "profile") == "topic" {
		entries := []any{}
		for _, raw := range stellarList(stellarCollectionGet(d, "navigation.series")) {
			v := stellarMap(raw)
			entries = append(entries, map[string]any{"href": v["path"], "title": v["title"], "active": v["id"] == stellarItemGet(d, "id")})
		}
		title := stellarTranslate(d, "btn.topic") + stellarTranslate(d, "symbol.colon") + stellarText(stellarFirst(stellarCollectionGet(d, "identity.headline"), stellarCollectionGet(d, "identity.name")))
		return r.frame(d, "related-widget", title, r.collection(d, map[string]any{"density": "compact", "items": entries}), "", "", nil, false)
	}
	var b strings.Builder
	for _, raw := range stellarList(stellarRenderGet(d, "article.related")) {
		group := stellarMap(raw)
		entries := []any{}
		for _, p := range stellarList(group["items"]) {
			v := stellarMap(p)
			entries = append(entries, map[string]any{"href": v["href"], "title": v["title"], "description": v["description"], "indicator": false})
		}
		b.WriteString(string(r.frame(d, "related-summary-widget", stellarTranslate(d, "meta.more")+stellarTranslate(d, "symbol.colon")+stellarText(group["name"]), r.collection(d, map[string]any{"density": "compact", "variant": "summary", "items": entries}), "", "", nil, true)))
	}
	return template.HTML(b.String())
}
func (r stellarTemplateRenderer) authorWidget(d *StellarRenderData, item map[string]any, host *settings.WidgetInstance) template.HTML {
	author := stellarMap(d.Stellar.Data["defaultAuthor"])
	id := stellarText(stellarRenderGet(d, "listing.authorId"))
	if id != "" {
		if a := stellarMap(stellarGet(d.Stellar.Data, "authors."+id)); len(a) > 0 {
			author = a
		}
	}
	if len(author) == 0 && host != nil && host.AuthorData() != nil {
		a := host.AuthorData()
		author = map[string]any{"name": a.Author.Name, "avatar": a.Author.Avatar, "description": a.Author.Description, "path": a.Path}
	}
	if len(author) == 0 {
		return ""
	}
	body := ""
	if stellarTruth(item["avatar"]) {
		body = `<div class="avatar"><img type="img" id="avatar_url" src="` + stellarEscape(stellarURL(d.Stellar.Root, author["avatar"])) + `"></div>`
	}
	body += `<p class="username" type="text" id="name">` + stellarEscape(author["name"]) + `</p><p class="bio" type="text" id="bio">` + stellarEscape(stellarFirst(author["desc"], author["description"])) + `</p><a class="follow" href="` + stellarEscape(stellarURL(d.Stellar.Root, author["path"])) + `"><span class="ui-icon">` + string(r.icon("default:documents")) + `</span>` + stellarEscape(stellarTranslate(d, "btn.all_posts")) + `</a>`
	return r.frame(d, "user-card author", "", template.HTML(body), "", "", nil, true)
}
func (r stellarTemplateRenderer) dynamicWidget(d *StellarRenderData, layout string, item map[string]any) template.HTML {
	base := strings.TrimRight(stellarText(stellarGet(d.Stellar.Theme, "services.github.apiUrl")), "/")
	repo := stellarText(stellarRenderGet(d, "listing.repository"))
	if repo == "" {
		repo = stellarText(stellarGet(d.Stellar.Data, "page.source.repository"))
	}
	switch layout {
	case "timeline", "ghissues":
		api := stellarText(item["api"])
		if layout == "ghissues" {
			if repo == "" {
				return ""
			}
			api = base + "/repos/" + repo + "/issues?per_page=" + fmt.Sprint(stellarInt(item["limit"], 3))
			if stellarTruth(item["labels"]) {
				api += "&labels=" + url.QueryEscape(stellarText(item["labels"]))
			}
		}
		if api == "" {
			return ""
		}
		kind := stellarText(stellarFirst(item["type"], "timeline"))
		if layout == "ghissues" {
			kind = "timeline"
		}
		attrs := ` data-api="` + stellarEscape(api) + `"`
		keys := make([]string, 0, len(item))
		for key := range item {
			if key != "title" && key != "type" && key != "api" && stellarCommentAttr(key) {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		for _, key := range keys {
			if stellarTruth(item[key]) {
				attrs += ` ` + key + `="` + stellarEscape(item[key]) + `"`
			}
		}
		return r.frame(d, "timeline", stellarText(item["title"]), template.HTML(`<div class="tag-plugin timeline data-service ds-`+stellarEscape(kind)+`"`+attrs+`></div>`), "", "", nil, true)
	case "ghuser":
		user := stellarText(item["username"])
		if user == "" {
			return ""
		}
		href := "https://github.com/" + url.PathEscape(user)
		var b strings.Builder
		fmt.Fprintf(&b, `<widget class="widget-wrapper%s user-card ghuser">`, r.reveal(d))
		if stellarTruth(item["header"]) {
			fmt.Fprintf(&b, `<div class="widget-header"><a class="gh-url" href="%s"><span class="ui-icon">%s</span><span>github/%s</span></a></div>`, stellarEscape(href), r.icon("github:logo-alt"), stellarEscape(user))
		}
		fmt.Fprintf(&b, `<div class="widget-body data-service ds-ghinfo" data-api="%s/users/%s">`, stellarEscape(base), stellarEscape(url.PathEscape(user)))
		if stellarTruth(item["avatar"]) {
			fmt.Fprintf(&b, `<div class="avatar"><img no-lazy type="img" id="avatar_url" src="%s"></div>`, stellarEscape(d.Site.Avatar))
		}
		b.WriteString(`<p class="username" type="text" id="name">&nbsp;</p><p class="bio" type="text" id="bio">&nbsp;</p><div class="buttons">`)
		for _, v := range []map[string]string{{"key": "followers", "tab": "followers", "label": "followers"}, {"key": "following", "tab": "following", "label": "following"}, {"key": "public_repos", "tab": "repositories", "label": "repos"}} {
			fmt.Fprintf(&b, `<a class="btn" href="%s?tab=%s"><span class="title" type="text" id="%s">0</span><span class="desc">%s</span></a>`, stellarEscape(href), v["tab"], v["key"], v["label"])
		}
		b.WriteString(`</div>`)
		if item["follow"] != false {
			fmt.Fprintf(&b, `<a class="follow" href="%s"><span class="ui-icon">%s</span>Follow</a>`, stellarEscape(href), r.icon("github:logo"))
		}
		b.WriteString(`</div>`)
		b.WriteString(`</widget>`)
		return template.HTML(b.String())
	case "ghrepo":
		if repo == "" {
			return ""
		}
		api := stellarText(stellarFirst(stellarRenderGet(d, "listing.repositoryApi"), base+"/repos/"+repo))
		body := `<div class="items data-service ds-ghinfo" data-api="` + stellarEscape(api) + `"><a class="repo" href="https://github.com/` + stellarEscape(repo) + `" target="_blank" rel="external nofollow noopener noreferrer"><div class="repo-name flex-row"><span class="ui-icon">` + string(r.icon("github:repo")) + `</span>` + stellarEscape(repo) + `</div><div class="repo-desc"><span type="text" id="description">&nbsp;</span></div><div class="grid">`
		for _, v := range []map[string]string{{"id": "stargazers_count", "icon": "github:star"}, {"id": "forks_count", "icon": "github:fork"}, {"id": "latest-tag-name", "icon": "github:tag"}} {
			attrs := ""
			if v["id"] == "latest-tag-name" {
				attrs = ` data-api="` + stellarEscape(api+"/tags") + `" index="0"`
			}
			class := "flex-row"
			if attrs != "" {
				class += " data-service ds-ghinfo"
			}
			body += `<div class="` + class + `"` + attrs + `><span class="ui-icon">` + string(r.icon(v["icon"])) + `</span><span type="text" id="` + v["id"] + `"></span></div>`
		}
		body += `</div></a></div>`
		return r.frame(d, "ghrepo", "", template.HTML(body), "", "", nil, true)
	}
	return ""
}
