package stellarmodels

import (
	"encoding/json"
	"sort"
)

// PageModelInput 对应官方集合流水线的快照边界。Page 只能包含公开宿主数据；
// config 必须先经过严格 Schema、profile 校验与归属解析。保留日期供模型归一化，
// 不将 ORM 或服务对象带入模板模型。
func PageModelInput(page, config Object) Object {
	date := coalesce(page["date"], config["date"])
	updated := coalesce(page["updated"], config["updated"], date)
	layout := str(page["layout"])
	if _, ok := page["layout"].(string); !ok {
		layout = text(first(config["layout"], "page"))
	}
	title := str(page["title"])
	if _, ok := page["title"].(string); !ok {
		title = text(first(config["title"]))
	}
	link := str(page["link"])
	if _, ok := page["link"].(string); !ok {
		link = str(config["link"])
	}
	var membership any
	if config["collection"] != nil {
		membership = pick(obj(config["collection"]), "profile", "id")
	}
	return Object{"_id": text(first(page["_id"], page["source"], page["path"])), "source": str(page["source"]), "path": str(page["path"]), "permalink": str(page["permalink"]), "link": link, "title": title, "layout": layout, "content": str(page["content"]), "excerpt": str(page["excerpt"]), "date": plainDate(date), "updated": plainDate(updated), "tags": plainTerms(config["tags"]), "categories": plainTerms(config["categories"]), "categoryLinks": plainLinks(page["categories"]), "tagLinks": plainLinks(page["tags"]), "previous": plainPostLink(page["prev"]), "next": plainPostLink(page["next"]), "lang": str(page["lang"]), "language": str(page["language"]), "collection": membership}
}
func plainTerms(v any) []any {
	items := array(v)
	if _, ok := v.(string); ok {
		items = []any{v}
	}
	r := []any{}
	for _, x := range items {
		if s, ok := x.(string); ok && s != "" {
			r = append(r, s)
		} else if s := str(obj(x)["name"]); s != "" {
			r = append(r, s)
		}
	}
	return r
}
func plainLinks(v any) []any {
	r := []any{}
	for _, x := range array(v) {
		m := obj(x)
		if str(m["name"]) != "" && str(m["path"]) != "" {
			r = append(r, Object{"name": str(m["name"]), "path": str(m["path"])})
		}
	}
	return r
}
func plainDate(v any) any {
	if v == nil {
		return nil
	}
	switch v.(type) {
	case string, float64, int, int64, json.Number:
		return v
	}
	return normalizeDate(v)
}
func plainPostLink(v any) any {
	if v == nil {
		return nil
	}
	m := obj(v)
	return Object{"title": str(m["title"]), "path": str(m["path"]), "date": plainDate(m["date"])}
}

type ContentRecord struct {
	Index        int    `json:"index"`
	Kind         string `json:"kind"`
	Profile      string `json:"profile"`
	CollectionID string `json:"collectionId"`
	Config       Object `json:"config"`
	Page         Object `json:"page"`
	Snapshot     Object `json:"snapshot"`
}
type Discovery struct {
	Records      []ContentRecord            `json:"records"`
	ByProfile    map[string][]ContentRecord `json:"byProfile"`
	ByCollection map[string][]ContentRecord `json:"byCollection"`
	Visits       int                        `json:"visits"`
}

// DiscoverContent 接受配对的页面与配置，不回调数据库。nil 配置表示宿主已记录
// 的严格校验失败；发现阶段不修复或读取旧配置。
func DiscoverContent(posts, pages []Member) Discovery {
	r := Discovery{Records: []ContentRecord{}, ByProfile: map[string][]ContentRecord{}, ByCollection: map[string][]ContentRecord{}}
	visit := func(kind string, m Member) {
		r.Visits++
		if m.FrontMatter == nil {
			return
		}
		profile := str(get(m.FrontMatter, "collection.profile"))
		if kind == "posts" && profile == "" {
			profile = "post"
		}
		if kind == "posts" && profile != "post" && profile != "topic" {
			return
		}
		if kind == "pages" && profile != "wiki" && profile != "notebook" {
			return
		}
		id := str(get(m.FrontMatter, "collection.id"))
		if profile == "post" {
			id = "post"
		}
		record := ContentRecord{Index: len(r.Records), Kind: kind, Profile: profile, CollectionID: id, Config: cp(m.FrontMatter), Page: cp(m.Page), Snapshot: PageModelInput(m.Page, m.FrontMatter)}
		r.Records = append(r.Records, record)
		r.ByProfile[profile] = append(r.ByProfile[profile], record)
		key := profile + ":" + id
		r.ByCollection[key] = append(r.ByCollection[key], record)
	}
	for _, m := range posts {
		visit("posts", m)
	}
	for _, m := range pages {
		visit("pages", m)
	}
	return r
}

// SelectListingItems 应用可见性与官方 tag.itemIds 筛选。显式标签未匹配时返回空列表；
// 调用方提供 adapter 的比较器，保留各 profile 的稳定排序。
func SelectListingItems(items []Object, tagID *string, tags []Object, less func(a, b Object) bool) []Object {
	var ids map[string]bool
	if tagID != nil {
		ids = map[string]bool{}
		for _, tag := range tags {
			if text(coalesce(tag["id"], tag["name"])) == *tagID {
				for _, id := range array(tag["itemIds"]) {
					ids[text(id)] = true
				}
				break
			}
		}
	}
	r := []Object{}
	for _, item := range items {
		if item["listed"] == false || (ids != nil && !ids[text(item["id"])]) {
			continue
		}
		r = append(r, cp(item))
	}
	if less != nil {
		sort.SliceStable(r, func(i, j int) bool { return less(r[i], r[j]) })
	}
	return r
}
func PaginateItems(items []Object, perPage int) [][]Object {
	size := perPage
	if size <= 0 {
		size = len(items)
		if size == 0 {
			size = 1
		}
	}
	r := [][]Object{}
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		page := []Object{}
		for _, item := range items[i:end] {
			page = append(page, cp(item))
		}
		r = append(r, page)
	}
	if len(r) == 0 {
		r = append(r, []Object{})
	}
	return r
}
