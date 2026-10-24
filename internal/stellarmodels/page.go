package stellarmodels

import (
	"blog/internal/stellarv2"
	"regexp"
	"strings"
)

// BuildPage 消费已归一化且经过 profile 校验的配置。调用方先解析归属；
// 此处也拒绝与页面显式归属不匹配的集合模型。
func BuildPage(in Input) (PageModel, error) {
	if err := requireTheme(in.Theme, in.ThemeSource); err != nil {
		return PageModel{}, err
	}
	profile := str(get(in.FrontMatter, "collection.profile"))
	if profile == "" {
		profile = "post"
	}
	id := in.CollectionID
	if profile != "post" {
		if id == "" && profile != "notebook" {
			id = str(get(in.FrontMatter, "collection.id"))
		}
		if in.CollectionConfig == nil {
			return PageModel{}, &ConfigError{text(first(in.Source, "<page>")), "collection.id", "未找到 " + profile + " collection " + id}
		}
		if id == "" || get(in.FrontMatter, "collection.id") != id {
			return PageModel{}, &ConfigError{text(first(in.Source, "<page>")), "collection.id", "与 collection " + id + " 不匹配"}
		}
	}
	in.CollectionID = id
	c := in.CollectionModel
	var err error
	if c == nil {
		switch profile {
		case "post":
			c, err = BuildPostCollection(in.Theme)
		case "wiki":
			c, err = BuildWikiCollection(in)
		case "topic":
			c, err = BuildTopicCollection(in)
		case "notebook":
			c, err = BuildNotebookCollection(in)
		default:
			err = &ConfigError{text(first(in.Source, "<page>")), "collection.profile", "不支持 " + profile}
		}
		if err != nil {
			return PageModel{}, err
		}
	} else {
		if c["profile"] != profile || (profile != "post" && c["id"] != id) {
			return PageModel{}, &ConfigError{text(first(in.Source, "<page>")), "collectionModel", "与页面显式 collection 归属不匹配"}
		}
		c = cp(c)
	}
	in.CollectionModel = c
	item := itemModel(in, c)
	var render Object
	switch profile {
	case "wiki":
		render = wikiRender(in, c, item)
	case "notebook":
		render = notebookRender(in, c, item)
	default:
		render = postRender(in, c, item)
		if profile == "topic" {
			layout := obj(render["layout"])
			layout["blogPath"] = get(c, "route.baseDir")
			path := get(c, "route.path")
			if series := array(get(c, "navigation.series")); len(series) > 0 {
				path = first(obj(series[0])["path"], path)
			}
			layout["breadcrumbs"] = []any{Object{"name": first(get(c, "identity.headline"), get(c, "identity.name"), c["id"]), "path": path}}
			obj(render["article"])["banner"] = cp(obj(get(item, "presentation.banner")))
		}
	}
	return PageModel{Collection: cp(c), Item: cp(item), Render: cp(render)}, nil
}
func postRender(in Input, c, item Object) Object {
	layout := assign(Object{"pageType": "content", "articleStyle": coalesce(get(item, "presentation.article.style")), "indent": indent(item), "siteBackground": truth(get(in.Theme, "appearance.backgrounds.page.image")), "blogPath": normalizePath(get(in.SiteConfig, "index_generator.path")), "breadcrumbs": links(in.Page["categoryLinks"])}, finalLayout(in, c, item))
	related := []any{}
	for _, r := range in.RelatedItems {
		if str(r["title"]) != "" && str(r["path"]) != "" {
			related = append(related, Object{"title": str(r["title"]), "path": str(r["path"]), "excerpt": str(r["excerpt"])})
		}
	}
	tags := []any{}
	if get(item, "presentation.footer.showTags") == true {
		tags = links(in.Page["tagLinks"])
	}
	summary := text(first(in.FrontMatter["description"], item["excerpt"], item["content"]))
	return Object{"document": document(in), "layout": layout, "seo": pageSEO(in, c, item, false), "article": Object{"heti": get(in.Theme, "features.heti.enabled") == true, "tags": tags, "footer": footer(in, item, summary, false), "previous": postLink(in.Page["previous"]), "next": postLink(in.Page["next"]), "related": Object{"enabled": number(get(in.Theme, "article.relatedPostsLimit")) > 0, "title": "", "maxCount": get(in.Theme, "article.relatedPostsLimit"), "items": related}, "comments": ResolveComments(in.Theme, obj(get(item, "presentation.comments")), str(item["title"]))}, "listing": listing(in, c, item)}
}
func notebookRender(in Input, c, item Object) Object {
	core := postRender(in, c, item)
	layoutCore := obj(core["layout"])
	layout := pick(layoutCore, "pageType", "articleStyle", "indent", "siteBackground", "topbar", "leftbar", "rightbar", "regionWarnings")
	layout["notebookIndexPath"] = stellarv2.ProfilePath(get(in.Theme, "profiles.notebooks.path"))
	layout["notebookPath"] = get(c, "route.baseDir")
	layout["algoliaFilterPath"] = get(c, "route.baseDir")
	layout["breadcrumbs"] = []any{Object{"name": first(get(c, "identity.headline"), get(c, "identity.name"), c["id"]), "path": get(c, "route.baseDir")}}
	layout["tagTree"] = notebookTagTree(c, in.TagTree)
	layout["recentItems"] = clone(array(in.RecentItems))
	seo := obj(core["seo"])
	if og := obj(seo["openGraph"]); len(og) > 0 {
		obj(og["args"])["type"] = "website"
	}
	seo["jsonLd"] = webPageLD(in, item, str(seo["description"]), false)
	articleCore := obj(core["article"])
	tags := []any{}
	if get(item, "presentation.footer.showTags") == true {
		for _, name := range terms(item["tags"]) {
			id := jsLower(text(name))
			path := normalizePath(str(get(c, "route.baseDir")) + "/tags/" + id)
			for _, t := range array(get(c, "navigation.tags")) {
				if obj(t)["id"] == id {
					path = text(first(obj(t)["path"], path))
					break
				}
			}
			tags = append(tags, Object{"name": text(name), "path": path})
		}
	}
	list := obj(core["listing"])
	return Object{"document": core["document"], "layout": layout, "seo": seo, "article": Object{"heti": articleCore["heti"], "banner": cp(obj(get(item, "presentation.banner"))), "created": item["date"], "updated": item["updated"], "tags": tags, "footer": clone(articleCore["footer"]), "comments": clone(articleCore["comments"])}, "listing": Object{"id": item["id"], "collectionId": c["id"], "collectionName": get(c, "identity.name"), "href": list["href"], "title": item["title"], "cover": str(item["cover"]), "excerpt": list["excerpt"], "tags": clone(terms(item["tags"])), "date": item["date"], "updated": item["updated"], "priority": get(item, "listing.priority"), "listed": get(item, "visibility.listed") != false}}
}
func WikiTitle(title, wiki, site, lang string) string {
	stripped := title
	if jsLower(title) == jsLower(wiki) {
		stripped = ""
	} else if wiki != "" {
		flex := regexp.QuoteMeta(wiki)
		flex = regexp.MustCompile(jsWhitespaceClass[:len(jsWhitespaceClass)-1]+`-]+`).ReplaceAllLiteralString(flex, jsWhitespaceClass[:len(jsWhitespaceClass)-1]+`-]*`)
		if re, err := regexp.Compile(`(?i)^` + flex + jsWhitespaceClass[:len(jsWhitespaceClass)-1] + `-]*[：:\-](.*)`); err == nil {
			if m := re.FindStringSubmatch(title); len(m) > 1 && m[1] != "" {
				stripped = jsTrim(m[1])
			}
		}
	}
	colon := ": "
	if strings.HasPrefix(jsLower(lang), "zh") {
		colon = "："
	}
	subject := wiki
	if stripped != "" {
		subject += colon + stripped
	}
	if subject != "" {
		return subject + " - " + site
	}
	return site
}
func wikiRender(in Input, c, item Object) Object {
	path := str(get(item, "route.path"))
	parts := strings.Split(path+"/", "/")
	filter := ""
	if len(parts) >= 3 {
		filter = parts[0] + "/" + parts[1] + "/"
	}
	layout := assign(Object{"pageType": "content", "articleStyle": coalesce(get(item, "presentation.article.style")), "indent": indent(item), "siteBackground": truth(get(in.Theme, "appearance.backgrounds.page.image")), "wikiIndexPath": stellarv2.ProfilePath(get(in.Theme, "profiles.wikiIndex.path")), "algoliaFilterPath": filter, "breadcrumbs": []any{Object{"name": get(c, "identity.name"), "path": get(c, "route.homepage")}}}, finalLayout(in, c, item))
	summary := text(first(in.FrontMatter["description"], get(c, "identity.description"), item["excerpt"], item["content"]))
	previous, next := wikiNeighbors(c, item)
	banner := cp(obj(get(item, "presentation.banner")))
	if banner["headline"] == nil {
		banner["headline"] = item["title"]
	}
	related := clone(array(in.Related))
	if in.Related == nil {
		related = BuildWikiRelated(in)
	}
	list := cp(in.Listing)
	if in.Listing == nil {
		list = BuildWikiListingRender(in, c)
	}
	return Object{"document": document(in), "layout": layout, "seo": pageSEO(in, c, item, true), "cover": BuildHero(HeroInput{Theme: in.Theme, Collection: c, Item: item, SiteConfig: in.SiteConfig, FrontMatter: in.FrontMatter}), "article": Object{"heti": get(in.Theme, "features.heti.enabled") == true, "banner": banner, "updated": item["updated"], "readmeHtml": ReadmeHTML(in, c, item), "footer": footer(in, item, summary, true), "previous": previous, "next": next, "comments": ResolveComments(in.Theme, obj(get(item, "presentation.comments")), str(item["title"])), "related": related}, "listing": list}
}
