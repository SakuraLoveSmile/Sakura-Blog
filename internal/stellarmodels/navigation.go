package stellarmodels

import (
	"golang.org/x/text/collate"
	textlanguage "golang.org/x/text/language"
	"math"
	"sort"
	"strings"
)

func wikiTree(v any) []any {
	out := []any{}
	for _, s := range array(v) {
		section := obj(s)
		pages := []any{}
		for _, p := range array(section["pages"]) {
			page := obj(p)
			var n any
			if value, ok := numericValue(page["page_number"]); ok && !math.IsNaN(value) && !math.IsInf(value, 0) {
				n = page["page_number"]
			}
			pages = append(pages, Object{"id": text(first(page["_id"], page["id"], page["path"])), "title": text(first(page["title"])), "path": normalizePath(page["path"]), "pageNumber": n, "isHomepage": page["is_homepage"] == true})
		}
		out = append(out, Object{"title": str(section["title"]), "items": pages})
	}
	return out
}
func topicSeries(id string, members []Member, sortConfig Object) []any {
	r := []any{}
	for _, member := range members {
		c, p := member.FrontMatter, member.Page
		if get(c, "collection.profile") != "topic" || get(c, "collection.id") != id || get(c, "visibility.listed") == false {
			continue
		}
		r = append(r, Object{"id": text(first(p["_id"], p["source"], p["path"])), "title": text(first(p["title"], c["title"])), "path": normalizePath(p["path"]), "date": normalizeDate(coalesce(p["date"], c["date"]))})
	}
	if text(first(sortConfig["field"], "date")) == "date" {
		asc := sortConfig["direction"] == "asc"
		sort.SliceStable(r, func(i, j int) bool {
			left, right := text(obj(r[i])["date"]), text(obj(r[j])["date"])
			if asc {
				return left < right
			}
			return left > right
		})
	}
	return r
}
func notebookTags(id, base string, items []Object) []any {
	byID := map[string]Object{}
	for _, item := range items {
		if get(item, "collection.profile") != "notebook" || get(item, "collection.id") != id {
			continue
		}
		for _, term := range terms(item["tags"]) {
			parts := []string{}
			for _, p := range strings.Split(text(term), "/") {
				if p != "" {
					parts = append(parts, p)
				}
			}
			for i := range parts {
				name := strings.Join(parts[:i+1], "/")
				key := jsLower(name)
				if _, exists := byID[key]; exists {
					continue
				}
				var parent any
				if i > 0 {
					parent = jsLower(strings.Join(parts[:i], "/"))
				}
				byID[key] = Object{"id": key, "name": name, "label": parts[i], "path": normalizePath(base + "/tags/" + key), "parentId": parent}
			}
		}
	}
	keys := []string{}
	for k := range byID {
		keys = append(keys, k)
	}
	collator := collate.New(textlanguage.English)
	sort.SliceStable(keys, func(i, j int) bool { return collator.CompareString(keys[i], keys[j]) < 0 })
	r := []any{}
	for _, k := range keys {
		r = append(r, byID[k])
	}
	return r
}
func notebookTagTree(c Object, input []Object) []any {
	tags := array(input)
	if len(tags) == 0 {
		tags = append([]any{Object{"id": "", "name": "", "label": "", "path": get(c, "route.baseDir"), "parentId": nil}}, array(get(c, "navigation.tags"))...)
	}
	children := map[string][]any{}
	for _, t := range tags {
		tag := obj(t)
		parent := str(tag["parentId"])
		if id := str(tag["id"]); id != "" {
			children[parent] = append(children[parent], id)
		}
	}
	r := []any{}
	for _, t := range tags {
		tag := obj(t)
		label := str(tag["label"])
		if _, ok := tag["label"].(string); !ok {
			label = str(tag["part"])
		}
		var parent any
		if p, ok := tag["parentId"].(string); ok {
			parent = p
		} else if p := str(tag["parent"]); p != "" {
			parent = p
		}
		path := str(get(c, "route.baseDir"))
		if p, ok := tag["path"].(string); ok {
			path = normalizePath(p)
		}
		child := children[str(tag["id"])]
		if child == nil {
			child = []any{}
		}
		if _, ok := tag["children"]; ok {
			child = array(tag["children"])
		}
		r = append(r, Object{"id": str(tag["id"]), "name": str(tag["name"]), "label": label, "path": path, "parentId": parent, "children": clone(child)})
	}
	return r
}
func wikiNeighbors(c, item Object) (any, any) {
	pages := []Object{}
	for _, s := range array(get(c, "navigation.tree")) {
		for _, p := range array(obj(s)["items"]) {
			m := obj(p)
			if m["pageNumber"] != nil {
				pages = append(pages, m)
			}
		}
	}
	sort.SliceStable(pages, func(i, j int) bool { return number(pages[i]["pageNumber"]) < number(pages[j]["pageNumber"]) })
	byNumber := map[float64]Object{}
	for _, p := range pages {
		byNumber[number(p["pageNumber"])] = p
	}
	for i := len(pages) - 1; i >= 0; i-- {
		p := pages[i]
		if p["path"] == get(item, "route.path") {
			n := number(p["pageNumber"])
			return wikiPageLink(byNumber[n-1]), wikiPageLink(byNumber[n+1])
		}
	}
	return nil, nil
}
func wikiPageLink(v Object) any {
	if str(v["path"]) == "" {
		return nil
	}
	return Object{"title": text(first(v["title"])), "path": normalizePath(v["path"]), "date": nil}
}
func postLink(v any) any {
	p := obj(v)
	if path := normalizePath(p["path"]); path != "" {
		return Object{"title": str(p["title"]), "path": path, "date": normalizeDate(p["date"])}
	}
	return nil
}
func BuildTopicIndexRender(in Input) (Object, error) {
	c := in.CollectionModel
	var err error
	if c == nil {
		c, err = BuildTopicCollection(in)
		if err != nil {
			return nil, err
		}
	}
	pages := []any{}
	for _, x := range array(get(c, "navigation.series")) {
		pages = append(pages, pick(obj(x), "title", "path", "date"))
	}
	var latest any
	href := "/"
	var date any
	rest := []any{}
	if len(pages) > 0 {
		latest = pages[0]
		href = text(first(obj(latest)["path"], "/"))
		date = coalesce(obj(latest)["date"])
		rest = pages[1:]
	}
	return Object{"id": c["id"], "name": get(c, "identity.name"), "headline": get(c, "identity.headline"), "description": get(c, "identity.description"), "cover": str(c["cover"]), "href": href, "latest": latest, "items": rest, "sortDate": date, "listed": get(c, "visibility.listed") != false}, nil
}
func BuildWikiListingRender(in Input, c CollectionModel) Object {
	repo := str(get(c, "source.repository"))
	api := ""
	if repo != "" {
		api = str(get(in.Theme, "services.github.apiUrl")) + "/repos/" + repo
	}
	return Object{"id": c["id"], "href": get(c, "route.homepage"), "name": get(c, "identity.name"), "headline": get(c, "identity.headline"), "caption": first(get(c, "identity.tagline"), get(c, "identity.description")), "description": get(c, "identity.description"), "tags": terms(in.CollectionConfig["tags"]), "audience": get(c, "identity.audience"), "icon": get(c, "identity.icon"), "cover": text(first(c["cover"])), "repository": repo, "repositoryApi": api, "priority": get(c, "listing.priority"), "order": coalesce(get(c, "listing.order"), 0), "listed": get(c, "visibility.listed") != false}
}
func BuildWikiRelated(in Input) []any {
	r := []any{}
	for _, group := range in.RelatedCollections {
		name := text(first(group["name"]))
		items := []any{}
		for _, p := range array(group["items"]) {
			project := obj(p)
			href := normalizePath(first(get(project, "route.homepage"), get(project, "route.path")))
			title := text(first(get(project, "identity.name"), project["id"]))
			if href != "" && title != "" {
				items = append(items, Object{"href": href, "title": title, "description": text(first(get(project, "identity.description")))})
			}
		}
		if name != "" && len(items) > 0 {
			r = append(r, Object{"name": name, "items": items})
		}
	}
	return r
}
