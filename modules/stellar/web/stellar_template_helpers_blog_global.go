package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"regexp"
	"strings"
	"time"

	"blog/internal/settings"
	"blog/internal/stellarmodels"
)

// 主题扩展只向渲染器投影确实有公开消费者的字段；开放袋中其余值不可传播。
func prepBlogContentProjection(raw any) map[string]any {
	source := prepMap(raw)
	out := map[string]any{}
	scalar := func(dst, src map[string]any, names ...string) {
		for _, name := range names {
			if value, exists := src[name]; exists {
				switch value.(type) {
				case string, bool, int, int64, float64, json.Number, nil:
					dst[name] = value
				}
			}
		}
	}
	scalar(out, source, "enabled", "avatar_background", "rightbar_intro", "read_next", "expire_days", "banner_ratio")
	for _, key := range []string{"category_icons", "tag_icons", "edit_this_page"} {
		if values, exists := source[key]; exists {
			selected := map[string]any{}
			for name, value := range prepMap(values) {
				if s, ok := value.(string); ok {
					selected[name] = s
				}
			}
			out[key] = selected
		}
	}
	for key, fields := range map[string][]string{"reward": {"enabled", "text", "wechat", "alipay"}, "notebook": {"license", "share"}, "ai_label": {"default"}, "share": {"enabled"}, "nav_tabs": {"enable_custom"}, "welcome_linklist": {"columns"}} {
		if raw, exists := source[key]; exists {
			selected := map[string]any{}
			scalar(selected, prepMap(raw), fields...)
			out[key] = selected
		}
	}
	if notebook := prepMap(source["notebook"]); notebook["share"] != nil {
		if _, ok := notebook["share"].([]any); ok {
			values := []any{}
			for _, value := range stellarList(notebook["share"]) {
				if s, ok := value.(string); ok {
					values = append(values, s)
				}
			}
			prepMap(out["notebook"])["share"] = values
		}
	}
	if share := prepMap(source["share"]); share["platforms"] != nil {
		values := []any{}
		for _, value := range stellarList(share["platforms"]) {
			if s, ok := value.(string); ok {
				values = append(values, s)
			}
		}
		prepMap(out["share"])["platforms"] = values
	}
	labels := prepMap(source["ai_label"])
	for _, name := range []string{"manual", "reviewed", "polished", "generated"} {
		if raw, exists := labels[name]; exists {
			if raw == nil {
				prepMap(out["ai_label"])[name] = nil
			} else {
				selected := map[string]any{}
				scalar(selected, prepMap(raw), "color", "icon")
				prepMap(out["ai_label"])[name] = selected
			}
		}
	}
	for key, fields := range map[string][]string{"nav_tabs": {"title", "url", "icon", "external", "active_rules"}, "welcome_linklist": {"title", "url", "icon"}} {
		if raw, exists := source[key]; exists {
			values := []any{}
			for _, value := range stellarList(prepMap(raw)["items"]) {
				selected := map[string]any{}
				scalar(selected, prepMap(value), fields...)
				values = append(values, selected)
			}
			prepMap(out[key])["items"] = values
		}
	}
	return out
}

func stellarBlogGlobal(d *StellarRenderData, key string) any {
	return stellarGet(d.Stellar.Data, "blog.content."+key)
}

// Note 的扩展默认值仅补入当前渲染副本；显式文章/集合 footer 优先；null 沿官方规则继承，不由扩展替代。
func prepBlogNotebookFooter(input *stellarmodels.Input, profile, layout string, blog map[string]any) {
	if profile != "note" && layout != "note" && prepGet(input.FrontMatter, "collection.profile") != "notebook" {
		return
	}
	defaults := prepMap(prepGet(prepBlogContentProjection(blog["content"]), "notebook"))
	if len(defaults) == 0 {
		return
	}
	front := prepCloneMap(input.FrontMatter)
	footer := prepCloneMap(prepMap(front["footer"]))
	collection := prepMap(input.CollectionConfig["footer"])
	for _, key := range []string{"license", "share"} {
		if value, exists := defaults[key]; exists {
			if _, explicit := footer[key]; explicit {
				continue
			}
			if _, explicit := collection[key]; explicit {
				continue
			}
			footer[key] = prepClone(value)
		}
	}
	front["footer"] = footer
	input.FrontMatter = front
}

func (r stellarTemplateRenderer) blogAILabel(d *StellarRenderData, value string, image bool) template.HTML {
	cfg := stellarMap(stellarBlogGlobal(d, "ai_label"))
	if value == "" {
		value = stellarText(cfg["default"])
	}
	def := stellarmodels.ResolveAILabel(value)
	if len(def) == 0 {
		return ""
	}
	color, icon := stellarText(def["color"]), stellarText(def["icon"])
	if raw, exists := cfg[value]; exists {
		if raw == nil {
			return ""
		}
		custom := stellarMap(raw)
		color = stellarText(custom["color"])
		icon = stellarText(custom["icon"])
	}
	if color != "" && !strings.HasPrefix(color, "#") {
		color = "#" + color
	}
	color = stellarBlogPosterColor(color)
	style := ""
	if !image && color != "" {
		style = ` style="color:` + stellarEscape(color) + `"`
	}
	return template.HTML(`<span class="cap ai-label"` + style + `>` + string(r.icon(icon)) + stellarEscape(stellarTranslate(d, "meta.ai_label."+value)) + `</span>`)
}

func (r stellarTemplateRenderer) blogTaxonomyIcon(d *StellarRenderData, kind, name, fallback string) template.HTML {
	icon := stellarText(stellarMap(stellarBlogGlobal(d, kind+"_icons"))[name])
	if icon == "" {
		icon = fallback
	}
	return r.icon(icon)
}

func (r stellarTemplateRenderer) blogExpired(d *StellarRenderData) template.HTML {
	profile := stellarText(stellarCollectionGet(d, "profile"))
	if profile != "post" && profile != "topic" {
		return ""
	}
	value := stellarBlogGlobal(d, "expire_days")
	if value == nil {
		return ""
	}
	threshold := stellarInt(value, 180)
	if threshold <= 0 {
		threshold = 180
	}
	created := stellarDate(stellarItemGet(d, "date"), "2006-01-02 15:04:05")
	updated := stellarDate(stellarItemGet(d, "updated"), "2006-01-02 15:04:05")
	days := settings.ExpiredDays(created, updated, threshold, time.Now())
	if days == 0 {
		return ""
	}
	label := fmt.Sprintf("本文最后更新于 %d 天前，其中的信息可能已经过时，请谨慎参考。", days)
	locale := strings.ToLower(d.Stellar.Language)
	if strings.HasPrefix(locale, "en") {
		label = fmt.Sprintf("This article was last updated %d days ago. Its information may be outdated; please use it with care.", days)
	} else if locale == "zh-tw" || locale == "zh-hk" || locale == "zh-hant" {
		label = fmt.Sprintf("本文最後更新於 %d 天前，其中的資訊可能已經過時，請謹慎參考。", days)
	}
	return template.HTML(`<div class="tag-plugin colorful note" data-blog-expired color="warning"><div class="body">` + stellarEscape(label) + `</div></div>`)
}

func (r stellarTemplateRenderer) blogEditFooter(d *StellarRenderData) template.HTML {
	mapping := map[string]string{}
	for key, value := range stellarMap(stellarBlogGlobal(d, "edit_this_page")) {
		if s, ok := value.(string); ok {
			mapping[key] = s
		}
	}
	link := settings.ResolveEditPage(stellarText(stellarItemGet(d, "source.file")), mapping)
	if link == nil {
		return ""
	}
	href := stellarBlogContentURL(d.Stellar.Root, link.EditURL)
	if href == "" {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<section id="contributors" data-blog-edit><div class="header"><span>%s</span><span class="spacer"></span><a class="edit" href="%s">%s<span>%s</span></a></div>`, stellarEscape(stellarTranslate(d, "meta.contributors")), stellarEscape(href), r.icon("default:edit"), stellarEscape(stellarTranslate(d, "btn.edit")))
	if link.APIURL != "" {
		fmt.Fprintf(&b, `<div class="body"><div class="ds-contributors users-wrap" api-type="commits" data-api="%s"><div class="grid-box"></div></div></div>`, stellarEscape(link.APIURL))
	}
	b.WriteString(`</section>`)
	return template.HTML(b.String())
}

func (r stellarTemplateRenderer) blogWelcome(d *StellarRenderData, item map[string]any, host *settings.WidgetInstance) template.HTML {
	var b strings.Builder
	// 官方 Widget 描述按 Object.assign 覆盖，字段级 null 不会被丢弃。
	// 显式空值清除正文；只有省略 content 才采用独立 Blog 欢迎语默认值。
	if content, exists := item["content"]; exists {
		if stellarTruth(content) {
			b.WriteString(string(stellarMarkdown(d, content)))
		}
	} else if host != nil && host.Name == "welcome" {
		if text, ok := stellarBlogGlobal(d, "rightbar_intro").(string); ok && text != "" {
			fmt.Fprintf(&b, `<div data-blog-intro style="white-space:pre-wrap">%s</div>`, stellarEscape(text))
		}
	}
	if stellarTruth(item["src"]) {
		b.WriteString(string(stellarRemoteMarkdown(d, stellarText(item["src"]))))
	}
	if _, exists := item["linklist"]; exists {
		if stellarTruth(item["linklist"]) {
			b.WriteString(string(r.linklist(d, stellarMap(item["linklist"]))))
		}
	} else if host != nil && host.Name == "welcome" {
		links := stellarMap(stellarBlogGlobal(d, "welcome_linklist"))
		if len(stellarList(links["items"])) > 0 {
			selected := prepCloneMap(links)
			columns := stellarInt(selected["columns"], 0)
			if columns <= 0 {
				columns = len(stellarList(selected["items"]))
			}
			selected["view"] = "grid"
			selected["columns"] = columns
			selected["show_title"] = columns <= 2
			b.WriteString(string(r.linklist(d, selected)))
		}
	}
	return r.frame(d, "markdown", stellarText(item["title"]), template.HTML(b.String()), "", "", nil, true)
}

// 导航允许官方链接中的片段、查询及 mailto/tel，仍拒绝执行协议和反斜杠。
func stellarBlogNavURL(root string, value any) string {
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" || strings.Contains(text, `\`) || strings.ContainsAny(text, "\x00\r\n\t") {
		return ""
	}
	text = strings.TrimSpace(text)
	parsed, err := url.Parse(text)
	if err != nil {
		return ""
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	switch parsed.Scheme {
	case "http", "https":
		if parsed.Host == "" {
			return ""
		}
	case "mailto", "tel", "":
	default:
		return ""
	}
	if strings.HasPrefix(text, "?") {
		return text
	}
	return stellarURL(root, parsed.String())
}

func (r stellarTemplateRenderer) blogNavTabs(d *StellarRenderData) template.HTML {
	var b strings.Builder
	pageURL := stellarPublicPageURL(d)
	pagePath := stellarURL(d.Stellar.Root, stellarPagePath(d))
	for _, raw := range stellarList(stellarBlogGlobal(d, "nav_tabs.items")) {
		tab := stellarMap(raw)
		href := stellarBlogNavURL(d.Stellar.Root, tab["url"])
		if href == "" {
			continue
		}
		active := href == pageURL || href == pagePath
		if rule := stellarText(tab["active_rules"]); rule != "" {
			compiled, err := regexp.Compile(rule)
			active = err == nil && compiled.MatchString(pageURL)
		}
		class := ""
		if active {
			class = ` class="active"`
		}
		attrs := ""
		if tab["external"] == true {
			attrs = ` target="_blank" rel="noopener noreferrer"`
		}
		fmt.Fprintf(&b, `<a data-blog-nav%s href="%s"%s>%s%s</a>`, class, stellarEscape(href), attrs, r.icon(tab["icon"]), stellarEscape(tab["title"]))
	}
	return template.HTML(b.String())
}

// 笔记明确关闭分享时也关闭独立 Blog 分享按钮；省略仍保留官方及扩展既有行为。
func stellarBlogNotebookShareAllowed(d *StellarRenderData) bool {
	if footer := stellarMap(stellarGet(d.Stellar.Data, "page.footer")); len(footer) > 0 {
		if value, exists := footer["share"]; exists {
			return value != false
		}
	}
	if d.ViewData.Stellar != nil {
		if footer := stellarMap(d.ViewData.Stellar.Input.CollectionConfig["footer"]); len(footer) > 0 {
			if value, exists := footer["share"]; exists {
				return value != false
			}
		}
	}
	return true
}
