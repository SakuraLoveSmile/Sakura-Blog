package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"blog/internal/post"
	"blog/internal/settings"
	stellari18n "blog/modules/stellar/i18n"
)

// StellarTemplateFunctions adds only the strict Stellar 2 template vocabulary.
// All locale and configuration state comes from the current render request.
func StellarTemplateFunctions(reg *settings.IconRegistry) template.FuncMap {
	r := stellarTemplateRenderer{icons: reg}
	return template.FuncMap{
		"sf_get": stellarGet, "sf_text": stellarText, "sf_truth": stellarTruth,
		"sf_first": stellarFirst, "sf_list": stellarList, "sf_dict": stellarDict,
		"sf_json": stellarJSON, "sf_html": stellarRenderedHTML,
		"sf_t": stellari18n.Text, "sf_url": stellarURL, "sf_asset": stellarAsset,
		"sf_icon": r.icon, "sf_icon_css": r.iconCSS, "sf_collection": r.collection,
		"sf_regions": r.regions, "sf_brand": r.brand,
		"sf_region_body": func(d *StellarRenderData, region string) (template.HTML, error) {
			regions, err := r.regions(d)
			if err != nil {
				return "", err
			}
			body, _ := regions[region].(template.HTML)
			return body, nil
		},
		"sf_hero": r.hero, "sf_head_meta": stellarHeadMeta,
		"sf_page_title": stellarPageTitle, "sf_loading_css": stellarLoadingCSS,
		"sf_footer": r.footer, "sf_content": r.content,
		"sf_comments":   r.comments,
		"sf_blog_share": r.blogShare, "sf_blog_detail": stellarBlogDetail,
		"sf_reading_content": func(d *StellarRenderData) bool {
			return stellarBlogDetail(d) && strings.TrimSpace(stellarText(stellarFirst(stellarItemGet(d, "content"), stellarRenderGet(d, "article.readmeHtml"), d.Content))) != ""
		},
	}
}

// stellarGet accepts both canonical runtime maps and typed host projections.
// The JSON field tag is checked before case-insensitive Go field names.
func stellarGet(value any, path string) any {
	for _, part := range strings.Split(path, ".") {
		v := reflect.ValueOf(value)
		for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
			if v.IsNil() {
				return nil
			}
			v = v.Elem()
		}
		if !v.IsValid() {
			return nil
		}
		switch v.Kind() {
		case reflect.Map:
			if v.Type().Key().Kind() != reflect.String {
				return nil
			}
			item := v.MapIndex(reflect.ValueOf(part).Convert(v.Type().Key()))
			if !item.IsValid() {
				return nil
			}
			value = item.Interface()
		case reflect.Struct:
			found := false
			for i := 0; i < v.NumField(); i++ {
				field := v.Type().Field(i)
				if !field.IsExported() {
					continue
				}
				if strings.Split(field.Tag.Get("json"), ",")[0] == part || strings.EqualFold(field.Name, part) {
					value = v.Field(i).Interface()
					found = true
					break
				}
			}
			if !found {
				return nil
			}
		default:
			return nil
		}
	}
	return value
}
func stellarText(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case template.HTML:
		return string(t)
	case template.CSS:
		return string(t)
	case json.Number:
		return t.String()
	}
	return fmt.Sprint(v)
}
func stellarTruth(v any) bool {
	if v == nil {
		return false
	}
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Bool:
		return r.Bool()
	case reflect.String:
		return r.Len() > 0
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return r.Int() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return r.Uint() != 0
	case reflect.Float32, reflect.Float64:
		return r.Float() != 0
	case reflect.Pointer, reflect.Interface:
		return !r.IsNil()
	}
	return true
}
func stellarFirst(values ...any) any {
	for _, v := range values {
		if stellarTruth(v) {
			return v
		}
	}
	return nil
}
func stellarList(value any) []any {
	r := reflect.ValueOf(value)
	if !r.IsValid() || (r.Kind() != reflect.Slice && r.Kind() != reflect.Array) {
		return nil
	}
	out := make([]any, r.Len())
	for i := range out {
		out[i] = r.Index(i).Interface()
	}
	return out
}
func stellarDict(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("stellar template dict requires key/value pairs")
	}
	out := map[string]any{}
	for i := 0; i < len(values); i += 2 {
		k, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("stellar template dict key must be a string")
		}
		out[k] = values[i+1]
	}
	return out, nil
}
func stellarJSON(value any) (template.JS, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	// encoding/json escapes HTML and U+2028/U+2029; never splice arbitrary JS.
	return template.JS(b), nil
}

// Only already-rendered host/model HTML reaches this helper. Raw author text
// must go through the existing Markdown renderer before model construction.
func stellarRenderedHTML(value any) template.HTML { return template.HTML(stellarText(value)) }
func stellarEscape(value any) string              { return template.HTMLEscapeString(stellarText(value)) }
func stellarURL(root string, value any) string {
	s := stellarText(value)
	if s == "" {
		return root
	}
	u, err := url.Parse(s)
	if err != nil {
		return "#ZgotmplZ"
	}
	if u.Scheme != "" && u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "mailto" && u.Scheme != "tel" {
		return "#ZgotmplZ"
	}
	if u.IsAbs() || strings.HasPrefix(s, "//") || strings.HasPrefix(s, "#") {
		return s
	}
	base := strings.TrimRight(root, "/")
	if base == "" {
		return "/" + strings.TrimLeft(s, "/")
	}
	if strings.HasPrefix(s, base+"/") {
		return s
	}
	return base + "/" + strings.TrimLeft(s, "/")
}
func stellarAsset(root, path string) string {
	href := stellarURL(root, path)
	version := AssetVersion(path)
	if version != "" {
		if strings.Contains(href, "?") {
			href += "&"
		} else {
			href += "?"
		}
		href += "v=" + url.QueryEscape(version)
	}
	return href
}
func stellarInt(v any, fallback int) int {
	n, err := strconv.ParseFloat(stellarText(v), 64)
	if err != nil {
		return fallback
	}
	return int(n)
}
func stellarMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	if m == nil {
		return map[string]any{}
	}
	return m
}
func stellarMarkdown(d *StellarRenderData, v any) template.HTML {
	s := stellarText(v)
	if s == "" {
		return ""
	}
	if d.markdownError != nil {
		return ""
	}
	if d.markdownRender == nil {
		d.markdownError = fmt.Errorf("stellar v2: request Markdown renderer required")
		return ""
	}
	body, err := d.markdownRender(s, post.StellarRenderContext{
		Theme: d.Stellar.Theme, Data: d.Stellar.Data,
		Page: stellarMap(d.Stellar.Data["page"]), Language: d.Stellar.Language,
		BlogExtensions: d.Stellar.BlogContent, Offline: d.Stellar.Data["offline"] == true,
	})
	if err != nil {
		d.markdownError = err
		return ""
	}
	return template.HTML(body)
}

type stellarTemplateRenderer struct{ icons *settings.IconRegistry }

func (r stellarTemplateRenderer) icon(value any) template.HTML {
	key := stellarText(value)
	if key == "" {
		return ""
	}
	var content string
	if r.icons != nil {
		content = r.icons.Get(key)
	}
	if strings.HasPrefix(key, "/") || strings.HasPrefix(key, "https://") || strings.HasPrefix(key, "http://") {
		content = key
	}
	if strings.HasPrefix(content, "/") || strings.HasPrefix(content, "https://") || strings.HasPrefix(content, "http://") {
		return template.HTML(`<img no-lazy src="` + stellarEscape(stellarURL("/", content)) + `" alt="">`)
	}
	return template.HTML(content)
}
func (r stellarTemplateRenderer) ui(data *StellarRenderData, names ...string) string {
	classes := stellarMap(stellarGet(data.Stellar.Client, "ClientData.ctx.ui.classes"))
	defaults := map[string]string{"interactive": "ui-interactive", "interactiveSpotlight": "ui-interactive card-hover card-hover--spotlight", "collectionItem": "ui-collection__item ui-interactive card-hover card-hover--spotlight", "spotlight": "card-hover card-hover--spotlight", "hoverCard": "card-hover card-hover--spotlight card-hover--tilt"}
	out := []string{}
	seen := map[string]bool{}
	add := func(s string) {
		for _, token := range strings.Fields(s) {
			if !seen[token] {
				seen[token] = true
				out = append(out, token)
			}
		}
	}
	capability := false
	for _, name := range names {
		if fallback, ok := defaults[name]; ok {
			capability = true
			add(stellarText(stellarFirst(classes[name], fallback)))
		} else {
			add(name)
		}
	}
	if !capability {
		add(stellarText(stellarFirst(classes["interactiveSpotlight"], defaults["interactiveSpotlight"])))
	}
	return strings.Join(out, " ")
}

var stellarSafeAttribute = regexp.MustCompile(`(?i)^(?:(?:aria|data)-[a-z0-9_-]+|title)$`)

func stellarAttrs(values map[string]any) string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out strings.Builder
	for _, k := range keys {
		v := values[k]
		if !stellarSafeAttribute.MatchString(k) || v == nil || v == false {
			continue
		}
		if v == true {
			v = k
		}
		fmt.Fprintf(&out, ` %s="%s"`, k, stellarEscape(v))
	}
	return out.String()
}
func (r stellarTemplateRenderer) collection(data *StellarRenderData, options map[string]any) template.HTML {
	entries := stellarList(options["items"])
	var body strings.Builder
	for _, raw := range entries {
		e := stellarMap(raw)
		if e["href"] == nil && e["type"] != "button" {
			continue
		}
		title := stellarText(e["title"])
		iconOnly := e["iconOnly"] == true
		showTitle := e["showTitle"] != false && !iconOnly
		classes := r.ui(data, "collectionItem")
		if iconOnly {
			classes += " is-icon-only"
		}
		if stellarTruth(e["active"]) {
			classes += " is-active"
		}
		if c := stellarText(e["className"]); c != "" {
			classes += " " + c
		}
		tag := "a"
		primary := ` href="` + stellarEscape(stellarURL(data.Stellar.Root, e["href"])) + `"`
		if e["type"] == "button" {
			tag = "button"
			primary = ` type="button"`
		}
		attrs := stellarMap(e["attrs"])
		if attrs["title"] == nil {
			primary += ` title="` + stellarEscape(title) + `"`
		}
		if stellarTruth(e["active"]) {
			primary += ` aria-current="page"`
		}
		style := ""
		if t := stellarText(e["theme"]); t != "" {
			style += "--item-theme:" + stellarCSSValue(t) + ";"
		}
		if depth := stellarInt(e["depth"], 0); depth > 0 {
			style += fmt.Sprintf("--ui-item-depth:%d;", depth)
		}
		if style != "" {
			primary += ` style="` + stellarEscape(style) + `"`
		}
		fmt.Fprintf(&body, `<%s class="%s"%s%s>`, tag, stellarEscape(classes), primary, stellarAttrs(attrs))
		// before/after are internal trusted template fragments, not public config.
		if before, ok := e["before"].(template.HTML); ok {
			body.WriteString(string(before))
		}
		if stellarTruth(e["icon"]) {
			fmt.Fprintf(&body, `<span class="ui-icon">%s</span>`, r.icon(e["icon"]))
		} else if showTitle {
			initial := []rune(strings.TrimSpace(title))
			if len(initial) > 0 {
				fmt.Fprintf(&body, `<span class="ui-collection__initial" aria-hidden="true">%s</span>`, stellarEscape(string(unicode.ToUpper(initial[0]))))
			}
		}
		if !iconOnly && (showTitle || stellarTruth(e["prefix"]) || stellarTruth(e["description"]) || stellarTruth(e["meta"])) {
			body.WriteString(`<span class="ui-collection__content">`)
			for _, field := range []string{"prefix", "title", "description", "meta"} {
				if field == "title" && !showTitle {
					continue
				}
				if field == "title" || stellarTruth(e[field]) {
					fmt.Fprintf(&body, `<span class="ui-collection__%s">%s</span>`, field, stellarEscape(e[field]))
				}
			}
			body.WriteString(`</span>`)
		}
		if after, ok := e["after"].(template.HTML); ok {
			body.WriteString(string(after))
		}
		if stellarTruth(e["active"]) && e["indicator"] != false {
			body.WriteString(`<span class="ui-collection__indicator"></span>`)
		}
		fmt.Fprintf(&body, `</%s>`, tag)
	}
	if body.Len() == 0 {
		return ""
	}
	layout := stellarText(options["layout"])
	if layout != "grid" && layout != "inline" {
		layout = "list"
	}
	variant := stellarText(options["variant"])
	if variant != "summary" && variant != "icon" {
		variant = "nav"
	}
	density := "auto"
	if options["density"] == "compact" {
		density = "compact"
	}
	columns := stellarInt(options["columns"], 1)
	if columns < 1 {
		columns = 1
	}
	style := ""
	if layout == "grid" {
		style = fmt.Sprintf(` style="--ui-columns:%d"`, columns)
	}
	class := "ui-collection"
	if stellarTruth(options["className"]) {
		class += " " + stellarText(options["className"])
	}
	return template.HTML(fmt.Sprintf(`<div class="%s" data-layout="%s" data-variant="%s" data-density="%s"%s>%s</div>`, stellarEscape(class), layout, variant, density, style, body.String()))
}

// CSS values interpolated by trusted builders cannot terminate a declaration.
func stellarCSSValue(s string) string {
	return strings.Map(func(c rune) rune {
		if c == '\n' || c == '\r' || c == ';' || c == '{' || c == '}' || c == '<' || c == '>' {
			return -1
		}
		return c
	}, s)
}
