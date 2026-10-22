// Package stellarmodels 将严格 Stellar 2.0 运行时配置投影为独立的集合、内容与渲染模型，不执行 IO。
package stellarmodels

import (
	"blog/internal/stellarv2"
	"encoding/json"
	"fmt"
	"golang.org/x/text/cases"
	textlanguage "golang.org/x/text/language"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type Object = map[string]any
type CollectionModel = Object
type PageModel struct {
	Collection CollectionModel `json:"collection"`
	Item       Object          `json:"item"`
	Render     Object          `json:"render"`
}
type Member struct {
	Page        Object `json:"page"`
	FrontMatter Object `json:"frontMatter"`
}
type Input struct {
	Theme              Object          `json:"stellarConfig"`
	SiteConfig         Object          `json:"siteConfig"`
	RuntimeData        Object          `json:"runtimeData"`
	Page               Object          `json:"page"`
	FrontMatter        Object          `json:"frontMatter"`
	CollectionConfig   Object          `json:"collectionConfig"`
	CollectionState    Object          `json:"collectionState"`
	CollectionModel    CollectionModel `json:"collectionModel"`
	CollectionID       string          `json:"collectionId"`
	Source             string          `json:"source"`
	ThemeSource        string          `json:"themeSource"`
	CollectionSource   string          `json:"collectionSource"`
	Members            []Member        `json:"members"`
	CollectionItems    []Object        `json:"collectionItems"`
	TagTree            []Object        `json:"tagTree"`
	RecentItems        []Object        `json:"recentItems"`
	RelatedItems       []Object        `json:"relatedItems"`
	RelatedCollections []Object        `json:"relatedCollections"`
	Related            []Object        `json:"related"`
	Listing            Object          `json:"listing"`
	CollectionListed   *bool           `json:"collectionListed"`
	IsBackup           bool            `json:"isBackup"`
}

type ConfigError struct{ Source, Path, Expected string }

func (e *ConfigError) Error() string {
	return fmt.Sprintf("Stellar v2 内容配置校验失败：%s: %s %s", e.Source, e.Path, e.Expected)
}
func obj(v any) Object {
	m, _ := v.(map[string]any)
	if m == nil {
		return Object{}
	}
	return m
}
func get(m Object, path string) any { v, _ := stellarv2.Get(m, path); return v }
func str(v any) string              { s, _ := v.(string); return s }
func truth(v any) bool {
	if v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t != ""
	case int:
		return t != 0
	case int64:
		return t != 0
	case float64:
		return t != 0 && !math.IsNaN(t)
	case json.Number:
		n, _ := numericValue(t)
		return n != 0 && !math.IsNaN(n)
	}
	return true
}
func text(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	if n, ok := v.(json.Number); ok {
		return jsNumberString(number(n))
	}
	return fmt.Sprint(v)
}
func first(v ...any) any {
	for _, x := range v {
		if truth(x) {
			return x
		}
	}
	return ""
}
func coalesce(v ...any) any {
	for _, x := range v {
		if x != nil {
			return x
		}
	}
	return nil
}

// 仅计算时转为 JS number 对应的 double；clone/merge 保留原始 json.Number。
func numericValue(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		n, err := x.Float64()
		if err != nil && !math.IsInf(n, 0) {
			return math.NaN(), true
		}
		return n, true
	}
	return 0, false
}
func number(v any) float64 { n, _ := numericValue(v); return n }
func jsNumberString(n float64) string {
	if n == 0 {
		return "0"
	}
	if math.IsNaN(n) {
		return "NaN"
	}
	if math.IsInf(n, 1) {
		return "Infinity"
	}
	if math.IsInf(n, -1) {
		return "-Infinity"
	}
	if math.Abs(n) >= 1e-6 && math.Abs(n) < 1e21 {
		return strconv.FormatFloat(n, 'f', -1, 64)
	}
	s := strconv.FormatFloat(n, 'e', -1, 64)
	pos := strings.IndexByte(s, 'e')
	exponent, _ := strconv.Atoi(s[pos+1:])
	return s[:pos] + fmt.Sprintf("e%+d", exponent)
}
func array(v any) []any {
	if v == nil {
		return []any{}
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return []any{}
	}
	out := make([]any, rv.Len())
	for i := range out {
		out[i] = rv.Index(i).Interface()
	}
	return out
}
func clone(v any) any {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case map[string]any:
		r := Object{}
		for k, z := range x {
			r[k] = clone(z)
		}
		return r
	case time.Time:
		return x
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice {
		r := make([]any, rv.Len())
		for i := range r {
			r[i] = clone(rv.Index(i).Interface())
		}
		return r
	}
	return v
}
func cp(m Object) Object {
	if m == nil {
		return Object{}
	}
	return clone(m).(Object)
}
func pick(m Object, fields ...string) Object {
	r := Object{}
	for _, k := range fields {
		if m[k] != nil {
			r[k] = clone(m[k])
		}
	}
	return r
}
func merge(base, override Object) Object {
	r := cp(base)
	for k, v := range override {
		if v == nil {
			continue
		}
		if _, ok := v.(map[string]any); ok {
			if b, ok := r[k].(map[string]any); ok {
				r[k] = merge(b, obj(v))
				continue
			}
		}
		r[k] = clone(v)
	}
	return r
}
func assign(ms ...Object) Object {
	r := Object{}
	for _, m := range ms {
		for k, v := range m {
			r[k] = clone(v)
		}
	}
	return r
}
func normalizePath(v any) string {
	s := str(v)
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return s
	}
	if strings.HasSuffix(s, "/index.html") {
		s = strings.TrimSuffix(s, "index.html")
	}
	if strings.HasSuffix(s, "/index") {
		s = strings.TrimSuffix(s, "index")
	}
	s = strings.TrimSuffix(s, ".html")
	if len(s) > 1 {
		s = strings.TrimSuffix(s, "/")
	}
	return strings.TrimLeft(s, "/")
}
func normalizeDate(v any) any {
	if v == nil {
		return nil
	}
	var t time.Time
	var err error
	switch x := v.(type) {
	case time.Time:
		t = x
	case float64, int, int64, json.Number:
		n, _ := numericValue(x)
		if math.IsNaN(n) || math.IsInf(n, 0) || math.Abs(n) > 8.64e15 {
			return nil
		}
		t = time.UnixMilli(int64(n))
	case string:
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02", "2006-01", "2006"} {
			t, err = time.Parse(layout, x)
			if err == nil {
				break
			}
		}
		if err != nil {
			return nil
		}
	default:
		return nil
	}
	t = t.UTC()
	if year := t.Year(); year < 0 || year > 9999 {
		return fmt.Sprintf("%+07d", year) + t.Format("-01-02T15:04:05.000Z")
	}
	return t.Format("2006-01-02T15:04:05.000Z")
}
func terms(v any) []any {
	r := []any{}
	for _, x := range array(v) {
		if s, ok := x.(string); ok {
			r = append(r, s)
		} else if s, ok := obj(x)["name"].(string); ok {
			r = append(r, s)
		}
	}
	return r
}
func stringList(v any) []any {
	r := []any{}
	if s, ok := v.(string); ok {
		for _, p := range strings.Split(s, ",") {
			if p = strings.TrimSpace(p); p != "" {
				r = append(r, p)
			}
		}
	} else {
		for _, x := range array(v) {
			s := jsString(x)
			if s != "" {
				r = append(r, s)
			}
		}
	}
	return r
}
func language(v ...any) string {
	for _, x := range v {
		if s := str(x); s != "" {
			return s
		}
		for _, z := range array(x) {
			if s := str(z); s != "" {
				return s
			}
		}
	}
	return ""
}
func links(v any) []any {
	r := []any{}
	for _, x := range array(v) {
		m := obj(x)
		name, path := str(m["name"]), normalizePath(m["path"])
		if name != "" && path != "" {
			r = append(r, Object{"name": name, "path": path})
		}
	}
	return r
}
func nav(m Object) Object {
	r := Object{}
	if m["activeMenu"] != nil {
		r["menu"] = clone(m["activeMenu"])
	}
	if m["breadcrumb"] != nil {
		r["breadcrumb"] = clone(m["breadcrumb"])
	}
	return r
}
func requireTheme(theme Object, source string) error {
	if source == "" {
		source = "<theme>"
	}
	for _, p := range []string{"profiles", "article", "profiles.notebook", "canonical"} {
		if _, ok := get(theme, p).(map[string]any); !ok {
			return &ConfigError{source, "stellarConfig." + p, "必须是 normalized runtime object"}
		}
	}
	return nil
}
func defaults(theme Object, collection bool) (Object, Object) {
	a := obj(theme["article"])
	footer := obj(a["footer"])
	f := Object{"references": []any{}, "license": clone(footer["license"]), "share": clone(footer["share"]), "showTags": clone(footer["showTags"])}
	if collection {
		f["share"] = []any{}
	}
	return pick(a, "style", "paragraphIndent"), f
}
func normalizeComments(v any) Object {
	m := obj(v)
	p := any(nil)
	if s, ok := m["provider"].(string); ok {
		p = s
	}
	return Object{"enabled": m["enabled"] != false, "title": str(m["title"]), "id": str(m["id"]), "provider": p, "options": Object{}}
}
func identity(c Object) Object {
	return Object{"name": text(first(c["name"])), "headline": text(coalesce(c["headline"], c["name"], "")), "tagline": text(first(c["tagline"])), "description": text(first(c["description"])), "audience": text(first(c["audience"])), "icon": str(c["icon"])}
}
func brand(id Object, path, index, label string) Object {
	back := "/"
	if p := stellarv2.ProfilePath(index); p != "" {
		back = "/" + p + "/"
	}
	href := "/"
	if p := normalizePath(path); p != "" {
		href = "/" + p + "/"
	}
	var icon any
	if truth(id["icon"]) {
		icon = id["icon"]
	}
	return Object{"source": "collection", "style": "regular", "backButton": true, "search": true, "backHref": back, "backLabel": label, "image": Object{"src": icon, "variant": "icon"}, "name": clone(id["name"]), "tagline": clone(id["tagline"]), "href": href}
}
func assertBrand(in Input, source string) error {
	b := obj(get(in.CollectionConfig, "leftbar.brand"))
	if text(first(b["source"], source)) != "collection" && b["backButton"] != nil {
		return &ConfigError{text(first(in.CollectionSource, "<collection>")), "leftbar.brand.back_button", "仅支持 source: collection"}
	}
	return nil
}

var articleFields, footerFields, sourceFields, bannerFields, commentsFields []string

func regions(in Input, profile string, def, source Object) Object {
	return stellarv2.ToRenderRegions(in.Theme, obj(get(in.Theme, "profiles."+profile)), def, stellarv2.RegionOptions{Layers: []Object{pick(in.CollectionConfig, "topbar", "leftbar", "rightbar")}, BrandSources: source})
}
func BuildPostCollection(theme Object) (CollectionModel, error) {
	if err := requireTheme(theme, ""); err != nil {
		return nil, err
	}
	a, f := defaults(theme, false)
	return Object{"id": "post", "profile": "post", "identity": stellarv2.NormalizeBrand(get(theme, "leftbar.brand")), "source": Object{}, "route": Object{"baseDir": stellarv2.ProfilePath(get(theme, "profiles.blogIndex.path"))}, "navigation": stellarv2.ToRenderNavigation(obj(get(theme, "profiles.post"))), "listing": Object{"pinStyle": clone(get(theme, "article.listing.pinnedLayout")), "cardStyle": clone(get(theme, "article.listing.cardLayout")), "excerptLength": clone(get(theme, "article.listing.excerptLength"))}, "presentation": assign(stellarv2.ToRenderRegions(theme, obj(get(theme, "profiles.post")), nil, stellarv2.RegionOptions{}), Object{"article": a, "footer": f, "comments": normalizeComments(theme["comments"])}), "visibility": Object{"listed": true, "searchable": true}}, nil
}
func buildCollection(in Input, profile string) (CollectionModel, error) {
	if err := requireTheme(in.Theme, in.ThemeSource); err != nil {
		return nil, err
	}
	if in.CollectionConfig == nil {
		return nil, &ConfigError{text(first(in.Source, "<page>")), "collection.id", "未找到 " + profile + " collection " + in.CollectionID}
	}
	if err := assertBrand(in, map[string]string{"topic": "site", "wiki": "collection", "notebook": "collection"}[profile]); err != nil {
		return nil, err
	}
	c := in.CollectionConfig
	id := identity(c)
	route, listing := obj(c["route"]), obj(c["listing"])
	base := ""
	navigation := merge(stellarv2.ToRenderNavigation(obj(get(in.Theme, "profiles."+profile))), nav(c))
	modelRoute := Object{}
	modelListing := Object{}
	var reg Object
	switch profile {
	case "wiki":
		base = text(first(route["path"], text(first(stellarv2.ProfilePath(get(in.Theme, "profiles.wikiIndex.path")), "wiki"))+"/"+in.CollectionID))
		modelRoute = Object{"baseDir": normalizePath(base), "homepage": normalizePath(get(in.CollectionState, "homepage.path"))}
		navigation["tree"] = wikiTree(in.CollectionState["sections"])
		modelListing = Object{"priority": coalesce(listing["priority"], 0), "order": coalesce(listing["order"], 0), "excerptLength": coalesce(listing["excerptLength"]), "perPage": coalesce(listing["perPage"])}
		b := brand(id, base, str(get(in.Theme, "profiles.wikiIndex.path")), "btn.all_wiki")
		reg = regions(in, "wiki", Object{"leftbar": Object{"brand": b}}, Object{"collection": b})
	case "topic":
		base = text(first(stellarv2.ProfilePath(get(in.Theme, "profiles.topic.path")), "topic"))
		path := text(first(route["path"], base+"/"+in.CollectionID))
		sortConfig := coalesce(listing["sort"], Object{"field": "date", "direction": "desc"})
		modelRoute = Object{"baseDir": normalizePath(base), "path": normalizePath(path), "start": normalizePath(route["start"])}
		navigation["series"] = topicSeries(in.CollectionID, in.Members, obj(sortConfig))
		modelListing = Object{"priority": coalesce(listing["priority"], 0), "order": coalesce(listing["order"]), "cardStyle": get(in.Theme, "article.listing.cardLayout"), "excerptLength": coalesce(listing["excerptLength"]), "perPage": coalesce(listing["perPage"]), "sort": clone(sortConfig)}
		b := brand(id, path, str(get(in.Theme, "profiles.topic.path")), "btn.all_topic")
		reg = regions(in, "topic", nil, Object{"collection": b})
	case "notebook":
		base = str(route["path"])
		if base == "" {
			base = strings.Trim(stellarv2.ProfilePath(get(in.Theme, "profiles.notebooks.path"))+"/"+in.CollectionID, "/")
		}
		base = normalizePath(base)
		modelRoute = Object{"baseDir": base}
		navigation["tags"] = notebookTags(in.CollectionID, base, in.CollectionItems)
		modelListing = Object{"priority": coalesce(listing["priority"], 0), "order": coalesce(listing["order"], 0), "excerptLength": coalesce(listing["excerptLength"], get(in.Theme, "profiles.notebook.listing.excerptLength")), "perPage": coalesce(listing["perPage"], get(in.Theme, "profiles.notebook.listing.perPage"), in.SiteConfig["per_page"], 10), "sort": clone(coalesce(listing["sort"], get(in.Theme, "profiles.notebook.listing.sort")))}
		b := brand(id, base, str(get(in.Theme, "profiles.notebooks.path")), "btn.all_notebook")
		reg = regions(in, "note", Object{"leftbar": Object{"brand": b}}, Object{"collection": b})
	}
	a, f := defaults(in.Theme, profile != "topic")
	presentation := assign(reg, Object{"article": merge(a, pick(obj(c["article"]), articleFields...)), "footer": merge(f, pick(obj(c["footer"]), footerFields...)), "comments": merge(normalizeComments(in.Theme["comments"]), pick(obj(c["comments"]), commentsFields...))})
	if profile == "wiki" {
		presentation["hero"] = clone(coalesce(c["hero"], Object{}))
	}
	listed := get(c, "visibility.listed") != false
	if profile != "notebook" && in.CollectionListed != nil && !*in.CollectionListed {
		listed = false
	}
	return Object{"id": in.CollectionID, "profile": profile, "identity": id, "cover": str(c["cover"]), "source": pick(obj(c["source"]), sourceFields...), "route": modelRoute, "navigation": navigation, "listing": modelListing, "presentation": presentation, "visibility": Object{"listed": listed, "searchable": get(c, "visibility.searchable") != false}}, nil
}
func BuildWikiCollection(in Input) (CollectionModel, error)  { return buildCollection(in, "wiki") }
func BuildTopicCollection(in Input) (CollectionModel, error) { return buildCollection(in, "topic") }
func BuildNotebookCollection(in Input) (CollectionModel, error) {
	return buildCollection(in, "notebook")
}

func jsString(v any) string {
	if v == nil {
		return "null"
	}
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return jsNumberString(number(x))
	case map[string]any:
		return "[object Object]"
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice {
		parts := []string{}
		for _, x := range array(v) {
			s := ""
			if x != nil {
				s = jsString(x)
			}
			parts = append(parts, s)
		}
		return strings.Join(parts, ",")
	}
	return fmt.Sprint(v)
}

func jsLower(s string) string { return cases.Lower(textlanguage.Und).String(s) }

const jsWhitespaceClass = `[\t\n\r\f\x{000b} \x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]`

func jsTrim(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v' || r == 0xa0 || r == 0x1680 || (r >= 0x2000 && r <= 0x200a) || r == 0x2028 || r == 0x2029 || r == 0x202f || r == 0x205f || r == 0x3000 || r == 0xfeff
	})
}
