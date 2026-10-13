package stellarmodels

import (
	"blog/internal/stellarv2"
	"crypto/md5"
	"embed"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"
)

//go:embed comments-defaults.json model-fields.json
var generated embed.FS
var commentDefaults Object

func init() {
	b, err := generated.ReadFile("comments-defaults.json")
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(b, &commentDefaults); err != nil {
		panic(err)
	}
	b, err = generated.ReadFile("model-fields.json")
	if err != nil {
		panic(err)
	}
	var fields map[string][]string
	if err = json.Unmarshal(b, &fields); err != nil {
		panic(err)
	}
	articleFields = fields["article"]
	footerFields = fields["footer"]
	sourceFields = fields["source"]
	bannerFields = fields["banner"]
	commentsFields = fields["comments"]
}
func stripHTML(s string) string {
	state, depth := 0, 0
	quote := byte(0)
	start, plain := -1, 0
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch state {
		case 0:
			if c == '<' {
				out.WriteString(s[plain:i])
				state = 1
				start = i
			}
		case 1:
			switch c {
			case '<':
				if quote == 0 {
					depth++
				}
			case '>':
				if quote == 0 {
					if depth > 0 {
						depth--
					} else {
						quote = 0
						state = 0
						start = -1
						plain = i + 1
					}
				}
			case '\'', '"':
				if c == quote {
					quote = 0
				} else if quote == 0 {
					quote = c
				}
			case '-':
				if i-start == 3 && s[start+1] == '!' && s[start+2] == '-' {
					state = 2
				}
			case ' ', '\n':
				if i-start == 1 {
					state = 0
					out.WriteString("< ")
					start = -1
					plain = i + 1
				}
			}
		case 2:
			if c == '>' {
				if i >= 2 && s[i-1] == '-' && s[i-2] == '-' {
					state = 0
					plain = i + 1
				}
				start = -1
			}
		}
	}
	if state == 0 && plain < len(s) {
		out.WriteString(s[plain:])
	}
	return out.String()
}
func substring(s string, n int) string {
	units := utf16.Encode([]rune(s))
	if n < 0 {
		n = 0
	}
	if n > len(units) {
		n = len(units)
	}
	return string(utf16.Decode(units[:n]))
}
func truncate(s string, n int) string {
	if len(utf16.Encode([]rune(s))) < n {
		return s
	}
	return substring(s, n-3) + "..."
}

var whitespace = regexp.MustCompile(jsWhitespaceClass + `+`)

func caption(item Object) string {
	tag := text(item["tagline"])
	if tag != "" {
		if i := strings.Index(tag, " | "); i > 0 {
			return tag[:i]
		}
		return tag
	}
	if s := text(item["description"]); s != "" {
		return s
	}
	s := jsTrim(whitespace.ReplaceAllString(stripHTML(text(first(item["excerpt"], item["content"]))), " "))
	return substring(s, 50)
}

var dataImage = regexp.MustCompile(`(?i)<img[^>]*data-src=["']([^"']+)["']`)
var srcImage = regexp.MustCompile(`(?i)<img[^>]*src=["']([^"']+)["']`)

func firstContentImage(s string) string {
	for _, re := range []*regexp.Regexp{dataImage, srcImage} {
		if m := re.FindStringSubmatch(s); len(m) > 0 {
			return m[1]
		}
	}
	return ""
}
func canonical(host, path string) any {
	if host == "" {
		return nil
	}
	p := normalizePath(path)
	if p == "404" || strings.HasPrefix(p, "404/") {
		return nil
	}
	if p != "" {
		p += "/"
	}
	return "https://" + host + "/" + p
}
func FilterShareServices(v any) []any {
	r := []any{}
	for _, x := range array(v) {
		switch x {
		case "qrcode", "weibo", "x", "telegram", "whatsapp", "email":
			r = append(r, x)
		}
	}
	return r
}
func encoded(s string) string {
	r := strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
	for _, pair := range [][2]string{{"%21", "!"}, {"%2A", "*"}, {"%28", "("}, {"%29", ")"}, {"%27", "'"}} {
		r = strings.ReplaceAll(r, pair[0], pair[1])
	}
	return r
}
func BuildShareAction(service string, share Object) Object {
	if len(FilterShareServices([]any{service})) == 0 {
		return nil
	}
	labels := map[string]string{"qrcode": "QR Code", "weibo": "Weibo", "email": "Email", "x": "X", "telegram": "Telegram", "whatsapp": "WhatsApp"}
	r := Object{"service": service, "label": labels[service]}
	title, link, image, summary := text(share["title"]), text(share["permalink"]), text(share["image"]), text(share["summary"])
	switch service {
	case "qrcode":
		r["kind"] = "toggle"
		r["target"] = "share-qrcode"
	case "weibo":
		r["kind"] = "external"
		href := "https://service.weibo.com/share/share.php?url=" + encoded(link) + "&title=" + encoded(title)
		if image != "" {
			href += "&pics=" + encoded(image)
		}
		r["href"] = href + "&summary=" + encoded(summary)
	case "email":
		r["kind"] = "link"
		r["href"] = "mailto:?subject=" + encoded(title) + "&body=" + encoded(link)
	case "x":
		r["kind"] = "external"
		r["href"] = "https://x.com/intent/tweet?text=" + encoded(title) + "&url=" + encoded(link)
	case "telegram":
		r["kind"] = "external"
		r["href"] = "https://t.me/share/url?url=" + encoded(link) + "&text=" + encoded(title)
	case "whatsapp":
		r["kind"] = "external"
		r["href"] = "https://wa.me/?text=" + encoded(title+"\n"+link)
	}
	return r
}

var resourceScheme = regexp.MustCompile(`(?i)^(?:[a-z][a-z0-9+.-]*:|//)`)

func resource(v any) any {
	s, ok := v.(string)
	if !ok || resourceScheme.MatchString(s) {
		return v
	}
	for strings.HasPrefix(s, "./") || strings.HasPrefix(s, "/") {
		s = strings.TrimPrefix(strings.TrimPrefix(s, "./"), "/")
	}
	return "/" + s
}
func ResolveComments(theme Object, overrides Object, title string) Object {
	c := assign(normalizeComments(theme["comments"]))
	for k, v := range overrides {
		if v != nil {
			c[k] = clone(v)
		}
	}
	service := str(c["provider"])
	options := cp(obj(get(theme, "comments."+service)))
	for k, v := range obj(c["options"]) {
		if v != nil {
			options[k] = clone(v)
		}
	}
	fields := []string{"js"}
	if service == "artalk" || service == "waline" {
		fields = append(fields, "css")
	}
	if service == "waline" {
		fields = append(fields, "meta_css")
	}
	for _, f := range fields {
		if v, ok := obj(c["options"])[f]; ok {
			options[f] = clone(v)
		}
	}
	assets := Object{}
	switch service {
	case "beaudar", "utterances", "twikoo", "waline", "artalk":
		assets["localCss"] = "/css/comments/" + service + ".css"
	}
	for _, f := range fields {
		key := f
		if f == "meta_css" {
			key = "metaCss"
		}
		assets[key] = resource(coalesce(options[f], get(commentDefaults, service+"."+f)))
		delete(options, f)
	}
	if service == "artalk" {
		server := strings.TrimRight(jsTrim(str(options["server"])), "/")
		for _, f := range []string{"js", "css"} {
			if assets[f] == nil && server != "" {
				assets[f] = server + "/dist/Artalk." + f
			}
		}
	}
	if service == "giscus" && get(theme, "appearance.colorScheme") != "auto" && options["data-theme"] == "preferred_color_scheme" {
		options["data-theme"] = get(theme, "appearance.colorScheme")
	}
	return Object{"enabled": c["enabled"] != false && service != "", "title": str(c["title"]), "id": str(c["id"]), "service": service, "options": options, "assets": assets, "pageTitle": title}
}
func contributor(in Input, item Object) any {
	service := obj(get(in.Theme, "services.contributors"))
	repos := array(get(service, str(service["provider"])+".repositories"))
	source := str(get(item, "source.file"))
	var matched Object
	for _, r := range repos {
		m := obj(r)
		prefix, ok := m["sourcePrefix"].(string)
		if ok && strings.HasPrefix(source, prefix) && (matched == nil || len(prefix) > len(str(matched["sourcePrefix"]))) {
			matched = m
		}
	}
	if matched == nil {
		return nil
	}
	relative := strings.TrimLeft(strings.TrimPrefix(source, str(matched["sourcePrefix"])), "/")
	repo := text(matched["repository"])
	branch := text(first(matched["branch"], "main"))
	api := strings.TrimRight(str(get(in.Theme, "services.github.apiUrl")), "/")
	return Object{"editUrl": "https://github.com/" + repo + "/blob/" + branch + "/" + relative, "commitsUrl": api + "/repos/" + repo + "/commits?path=" + encoded(relative)}
}
func resolveLicense(v any, item, runtime Object) string {
	s := str(v)
	if s == "" {
		return ""
	}
	authors, ok := runtime["authors"].(map[string]any)
	if !ok {
		return s
	}
	author := obj(authors[str(get(item, "presentation.article.author"))])
	if len(author) == 0 {
		author = obj(runtime["defaultAuthor"])
	}
	if len(author) == 0 {
		return s
	}
	s = strings.Replace(s, "{author.name}", text(first(author["name"])), 1)
	return strings.Replace(s, "{author.url}", text(first(author["url"])), 1)
}
func itemModel(in Input, c Object) Object {
	p, f := in.Page, in.FrontMatter
	pres := obj(c["presentation"])
	source := assign(Object{"file": text(first(p["source"], in.Source, "<page>"))}, obj(c["source"]), pick(obj(f["source"]), sourceFields...))
	visibility := obj(c["visibility"])
	if c["profile"] != "post" {
		visibility = merge(Object{"listed": true, "searchable": true}, obj(in.CollectionConfig["visibility"]))
	}
	return Object{"id": text(first(p["_id"], p["source"], p["path"])), "title": text(first(p["title"], f["title"])), "layout": text(first(p["layout"], f["layout"], "post")), "content": str(p["content"]), "excerpt": str(p["excerpt"]), "date": normalizeDate(coalesce(p["date"], f["date"])), "updated": normalizeDate(coalesce(p["updated"], f["updated"], p["date"], f["date"])), "tags": terms(coalesce(p["tags"], f["tags"])), "categories": terms(coalesce(p["categories"], f["categories"])), "cover": str(f["cover"]), "tagline": str(f["tagline"]), "source": source, "route": Object{"path": normalizePath(p["path"]), "permalink": str(p["permalink"])}, "navigation": merge(pick(obj(c["navigation"]), "menu", "breadcrumb"), nav(f)), "listing": Object{"priority": coalesce(get(f, "listing.priority"), 0)}, "presentation": assign(Object{"banner": pick(obj(f["banner"]), bannerFields...)}, stellarv2.CascadeRegions([]Object{pres, pick(f, "topbar", "leftbar", "rightbar")}, nil), Object{"article": merge(obj(pres["article"]), pick(obj(f["article"]), articleFields...)), "footer": merge(obj(pres["footer"]), pick(obj(f["footer"]), footerFields...)), "comments": merge(obj(pres["comments"]), pick(obj(f["comments"]), commentsFields...))}), "visibility": merge(visibility, pick(obj(f["visibility"]), "listed", "searchable"))}
}
func finalLayout(in Input, c, item Object) Object {
	r := stellarv2.ResolveRegions(stellarv2.RegionOptions{Profile: str(c["profile"]), DefaultState: str(get(in.Theme, "leftbar.defaultState")), Catalog: obj(in.RuntimeData["widgets"]), Layers: []Object{obj(item["presentation"])}})
	return Object{"topbar": r["topbar"], "leftbar": r["leftbar"], "rightbar": r["rightbar"], "regionWarnings": r["warnings"]}
}
func document(in Input) Object {
	r := Object{"language": language(in.Page["lang"], in.Page["language"], in.FrontMatter["lang"], in.FrontMatter["language"], in.SiteConfig["language"]), "preferredTheme": str(get(in.Theme, "appearance.colorScheme"))}
	for _, k := range []string{"headBegin", "headEnd", "bodyBegin", "bodyEnd"} {
		r[k+"Inject"] = str(get(in.FrontMatter, "inject."+k))
	}
	return r
}
func indent(item Object) bool {
	style, mode := get(item, "presentation.article.style"), first(get(item, "presentation.article.paragraphIndent"), "auto")
	return mode == "always" || (mode == "auto" && style == "story")
}
func pageSEO(in Input, c, item Object, wiki bool) Object {
	site, f := in.SiteConfig, in.FrontMatter
	title := str(item["title"])
	lang := str(document(in)["language"])
	descriptionSource := text(first(f["description"], item["excerpt"], item["content"]))
	if wiki {
		descriptionSource = text(first(f["description"], get(c, "identity.description"), item["excerpt"], item["content"]))
	}
	description := truncate(stripHTML(descriptionSource), 150)
	keys := stringList(f["keywords"])
	if len(keys) == 0 {
		keys = terms(item["tags"])
		if len(keys) == 0 {
			keys = stringList(site["keywords"])
		}
	}
	var robots any
	if in.IsBackup {
		robots = "noindex, nofollow"
	} else if s := str(f["robots"]); s != "" {
		robots = s
	}
	var og any
	cover := str(item["cover"])
	avatar := text(first(site["avatar"]))
	if avatar == "" && truth(site["email"]) {
		avatar = fmt.Sprintf("https://www.gravatar.com/avatar/%x", md5.Sum([]byte(jsLower(text(site["email"])))))
	}
	if get(in.Theme, "openGraph.enabled") == true {
		pageOG := obj(get(f, "seo.openGraph"))
		ogTitle := title
		if wiki {
			ogTitle = text(first(title, get(c, "identity.headline")))
		}
		image := first(cover, firstContentImage(str(item["content"])), avatar)
		if wiki {
			image = first(cover, firstContentImage(str(item["content"])), site["avatar"])
		}
		if !truth(image) {
			image = nil
		}
		kind := "article"
		if wiki {
			kind = "website"
		}
		args := Object{"type": kind, "title": ogTitle, "url": get(item, "route.permalink"), "site_name": text(first(site["title"])), "description": strings.ReplaceAll(jsTrim(substring(stripHTML(descriptionSource), 200)), "\n", " "), "language": lang, "author": text(first(site["author"])), "date": false, "updated": false, "image": image}
		if truth(get(in.Theme, "openGraph.twitterId")) {
			args["twitter_id"] = clone(get(in.Theme, "openGraph.twitterId"))
		}
		if cover != "" {
			args["twitter_card"] = "summary_large_image"
		}
		args = assign(args, pageOG)
		var pub, mod any
		tags := []any{}
		if !wiki {
			if pageOG["date"] != false && truth(item["date"]) {
				pub = item["date"]
			}
			if pageOG["updated"] != false && truth(item["updated"]) {
				mod = item["updated"]
			}
			tags = terms(item["tags"])
			sort.Slice(tags, func(i, j int) bool { return utf16Less(text(tags[i]), text(tags[j])) })
		}
		og = Object{"args": args, "title": ogTitle, "siteName": text(first(site["title"])), "twitterTitle": ogTitle, "publishedTime": pub, "modifiedTime": mod, "tags": tags}
	}
	seoTitle := text(first(site["title"]))
	if title != "" {
		seoTitle = title + " - " + seoTitle
	}
	if wiki {
		seoTitle = WikiTitle(title, str(get(c, "identity.name")), text(first(site["title"])), lang)
	}
	var ld Object
	if wiki {
		ld = webPageLD(in, item, description, true)
	} else {
		author := Object{"@type": "Person", "name": text(first(site["author"])), "sameAs": clone(get(in.Theme, "structuredData.sameAs"))}
		publisher := assign(author, Object{"@type": "Organization"})
		if avatar != "" {
			if !strings.HasPrefix(avatar, "http://") && !strings.HasPrefix(avatar, "https://") {
				if root := strings.TrimRight(text(site["url"]), "/"); root != "" {
					avatar = root + "/" + strings.TrimLeft(avatar, "/")
				}
			}
			author["image"] = avatar
			publisher["image"] = avatar
			publisher["logo"] = Object{"@type": "ImageObject", "url": avatar}
		}
		images := array(f["photos"])
		if cover != "" {
			images = append([]any{cover}, images...)
		}
		if len(images) == 0 {
			if img := firstContentImage(str(item["content"])); img != "" {
				images = append(images, img)
			}
		}
		if len(images) == 0 && truth(get(in.Theme, "fallbacks.cover")) {
			images = append(images, get(in.Theme, "fallbacks.cover"))
		}
		plainRE := regexp.MustCompile(`<[^>]+>`)
		desc := plainRE.ReplaceAllString(str(item["excerpt"]), "")
		if desc == "" {
			desc = truncate(plainRE.ReplaceAllString(str(item["content"]), ""), 200)
		}
		ld = Object{"@context": "https://schema.org", "@type": "BlogPosting", "author": author, "dateCreated": item["date"], "dateModified": item["updated"], "datePublished": item["date"], "description": desc, "headline": title, "mainEntityOfPage": Object{"@type": "WebPage", "@id": get(item, "route.permalink")}, "publisher": publisher, "url": get(item, "route.permalink"), "image": clone(images)}
		if tags := terms(item["tags"]); len(tags) > 0 {
			ts := []string{}
			for _, t := range tags {
				ts = append(ts, text(t))
			}
			ld["keywords"] = strings.Join(ts, ", ")
		}
		if len(images) > 0 {
			ld["thumbnailUrl"] = clone(images[0])
		}
	}
	return Object{"title": seoTitle, "description": description, "keywords": keys, "robots": robots, "canonical": canonical(str(get(in.Theme, "canonical.host")), str(get(item, "route.path"))), "openGraph": og, "jsonLd": ld}
}
func webPageLD(in Input, item Object, description string, wiki bool) Object {
	title := item["title"]
	if wiki {
		title = first(title, get(in.CollectionModel, "identity.headline"))
	}
	return Object{"@context": "https://schema.org", "@type": "WebPage", "@id": get(item, "route.permalink"), "name": title, "description": description, "url": get(item, "route.permalink"), "isPartOf": Object{"@type": "WebSite", "name": text(first(in.SiteConfig["title"])), "url": text(first(in.SiteConfig["url"]))}}
}
func footer(in Input, item Object, description string, wiki bool) Object {
	f := obj(get(item, "presentation.footer"))
	share := f["share"]
	if share == true {
		share = get(in.Theme, "article.footer.share")
	}
	services := FilterShareServices(share)
	license := f["license"]
	if license == true {
		license = get(in.Theme, "article.footer.license")
	}
	if wiki && (license == false || license == nil) {
		license = ""
	}
	var shareModel any
	if len(services) > 0 {
		shareModel = Object{"services": services, "permalink": get(item, "route.permalink"), "title": str(item["title"]) + " - " + text(first(in.SiteConfig["title"])), "image": str(item["cover"]), "summary": truncate(stripHTML(description), 120)}
	}
	return Object{"references": clone(array(f["references"])), "license": resolveLicense(license, item, in.RuntimeData), "share": shareModel, "contributor": contributor(in, item)}
}
func listing(in Input, c, item Object) Object {
	a := obj(in.Theme["article"])
	categories := links(in.Page["categoryLinks"])
	tagLinks := links(in.Page["tagLinks"])
	desc := str(in.FrontMatter["description"])
	excerpt := stripHTML(str(item["excerpt"]))
	if !truth(item["excerpt"]) {
		excerpt = desc
		if excerpt == "" && number(get(c, "listing.excerptLength")) > 0 {
			excerpt = truncate(stripHTML(str(item["content"])), int(number(get(c, "listing.excerptLength"))))
		}
	}
	names := []any{}
	last := ""
	for _, x := range categories {
		last = str(obj(x)["name"])
		names = append(names, last)
	}
	style := ""
	if raw := str(obj(a["categoryColors"])[last]); raw != "" {
		color := raw
		if !strings.HasPrefix(color, "#") {
			color = "#" + color
		}
		bg := color
		if len(color) == 4 {
			bg += "2"
		} else if len(color) == 7 {
			bg += "20"
		}
		style = "--text-p2:" + color + ";--theme-block:" + bg
	}
	tags := []any{}
	if get(a, "listing.showTags") == true {
		for i, x := range tagLinks {
			if i == 5 {
				break
			}
			tags = append(tags, obj(x)["name"])
		}
	}
	author := str(get(item, "presentation.article.author"))
	if author == "" {
		author = text(first(get(in.RuntimeData, "defaultAuthor.id")))
	}
	return Object{"href": first(in.Page["link"], get(item, "route.path")), "title": item["title"], "layout": item["layout"], "date": item["date"], "cover": str(item["cover"]), "caption": caption(Object{"tagline": item["tagline"], "description": desc, "excerpt": item["excerpt"], "content": item["content"]}), "excerpt": excerpt, "categories": names, "categoryStyle": style, "tags": tags, "authorId": author, "priority": get(item, "listing.priority"), "listed": get(item, "visibility.listed") != false, "cardStyle": first(get(c, "listing.cardStyle"), "classic")}
}

func utf16Less(a, b string) bool {
	aa, bb := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(aa) && i < len(bb); i++ {
		if aa[i] != bb[i] {
			return aa[i] < bb[i]
		}
	}
	return len(aa) < len(bb)
}
