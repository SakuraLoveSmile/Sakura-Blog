package stellarmigration

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"blog/internal/config"
	"blog/internal/stellarv2"
)

type themeBuilder struct {
	raw         map[string]any
	theme       map[string]any
	blog        map[string]any
	used        map[string]bool
	mappings    []Mapping
	diagnostics []Diagnostic
}

func get(m map[string]any, path string) (any, bool) { return stellarv2.Get(m, path) }
func set(m map[string]any, path string, v any) {
	parts := strings.Split(path, ".")
	for _, p := range parts[:len(parts)-1] {
		next, ok := m[p].(map[string]any)
		if !ok {
			next = map[string]any{}
			m[p] = next
		}
		m = next
	}
	m[parts[len(parts)-1]] = clone(v)
}
func (b *themeBuilder) mark(src, dst, behavior string) {
	b.used[src] = true
	b.mappings = append(b.mappings, Mapping{"settings." + src, dst, behavior})
}
func (b *themeBuilder) move(src, dst string) {
	v, ok := get(b.raw, src)
	if !ok {
		return
	}
	m := b.theme
	target := "theme." + dst
	if strings.HasPrefix(dst, "blog.") {
		m = b.blog
		dst = strings.TrimPrefix(dst, "blog.")
		target = "blog." + dst
	}
	set(m, dst, v)
	b.mark(src, target, "原值保留（包含 false/null/空列表）")
	if capability, needsEvidence := globalConsumerTargets[target]; needsEvidence {
		b.conflict(globalConsumerCode(capability), src, "已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力")
	}
}
func (b *themeBuilder) nonempty(src, dst string) {
	v, ok := get(b.raw, src)
	if !ok {
		return
	}
	if v == nil || v == "" {
		b.mark(src, "theme."+dst, "旧缺省占位保留在源快照；候选继承上游默认")
		return
	}
	b.move(src, dst)
}
func (b *themeBuilder) conflict(code, src, msg string) {
	b.diagnostics = append(b.diagnostics, Diagnostic{Code: code, Source: "settings", Path: src, Message: msg, Blocking: true})
}
func (b *themeBuilder) converted(src, dst string, v any, behavior string) {
	set(b.theme, dst, v)
	b.mark(src, "theme."+dst, behavior)
}

func buildTheme(s Snapshot) ThemeCandidate {
	stored := map[string]string{}
	for _, r := range s.Tables["settings"].Rows {
		stored[stringValue(r["section"])] = stringValue(r["value"])
	}
	if raw, exists := stored[config.SectionStellarV2]; exists {
		t := ThemeCandidate{Version: 2, DocumentJSON: raw, BlogJSON: stored[config.SectionBlogExtensions], Preserved: true}
		v, e := decode(raw)
		if e == nil {
			t.ThemeJSON = encode(object(v)["theme"])
		}
		if _, err := config.LoadStellarV2Snapshot(stored); err != nil {
			t.Diagnostics = append(t.Diagnostics, Diagnostic{Code: "invalid_versioned_theme", Source: "settings", Path: "stellar_v2", Message: "已迁移配置严格校验失败；原文保留，不重建", Blocking: true})
		}
		for _, name := range sortedKeys(stored) {
			t.Mappings = append(t.Mappings, Mapping{"settings." + name, "settings." + name, "已迁移设置与宿主设置原文保留"})
		}
		return t
	}
	b := themeBuilder{raw: map[string]any{}, theme: map[string]any{}, blog: map[string]any{}, used: map[string]bool{}}
	// 应用专用设置继续由原宿主读取，候选不复制其密钥进入上游主题。
	host := map[string]bool{"admin": true, "dashboard": true, "seo_tools": true, "wiki_projects_migrated": true, "page_banner_friends_legacy_migrated": true}
	for _, name := range sortedKeys(stored) {
		if host[name] {
			b.mark(name, "settings."+name, "宿主设置原文保留，无主题转换")
			continue
		}
		v, err := decode(stored[name])
		if err != nil {
			b.conflict("invalid_settings_json", name, "源设置不是有效 JSON；保留原文，需人工处理")
			continue
		}
		b.raw[name] = v
	}
	for _, p := range [][2]string{
		{"site.title", "blog.site.title"}, {"site.subtitle", "blog.site.subtitle"}, {"site.description", "blog.site.description"}, {"site.url", "blog.site.url"},
		{"site.author", "blog.site.author"}, {"site.email", "blog.site.email"}, {"site.language", "blog.site.language"}, {"site.keywords", "blog.site.keywords"}, {"site.per_page", "blog.site.per_page"}, {"site.date_format", "blog.site.date_format"},
		{"site.logo_title", "leftbar.brand.name"}, {"site.logo_sub", "leftbar.brand.tagline"}, {"site.avatar", "leftbar.brand.image.src"},
		{"site.avatar_bg", "blog.content.avatar_background"}, {"site.favicon_url", "blog.site.favicon_url"}, {"site.apple_touch_icon", "blog.site.apple_touch_icon"}, {"site.sw_cache_version", "blog.site.sw_cache_version"},
		{"menubar.columns", "leftbar.menu_columns"}, {"rightbar.intro", "blog.content.rightbar_intro"}, {"rightbar.welcome_linklist", "blog.content.welcome_linklist"},
		{"comments", "blog.native_comments"}, {"authors", "blog.data.authors"}, {"links", "blog.data.links"}, {"friends", "blog.friends"}, {"pages", "blog.pages"}, {"nav_tabs", "blog.content.nav_tabs"},
		{"article.reward", "blog.content.reward"}, {"article.ai_label", "blog.content.ai_label"}, {"article.category_icon", "blog.content.category_icons"}, {"article.tag_icon", "blog.content.tag_icons"},
		{"article.read_next", "blog.content.read_next"}, {"article.expire_days", "blog.content.expire_days"}, {"article.banner_ratio", "blog.content.banner_ratio"}, {"article.edit_this_page", "blog.content.edit_this_page"},
		{"article.reading_time", "article.show_reading_time"}, {"article.card_tags", "article.listing.show_tags"}, {"article.related_posts", "article.related_posts_limit"}, {"article.category_color", "article.category_colors"},
		{"plugins.preload.enable", "features.link_prefetch.enabled"}, {"plugins.heti.enable", "features.heti.enabled"}, {"plugins.fancybox.enable", "features.lightbox.enabled"}, {"plugins.fancybox.selector", "features.lightbox.selector"},
		{"plugins.scrollreveal.enable", "features.reveal.enabled"}, {"plugins.scrollreveal.duration", "features.reveal.duration"}, {"plugins.scrollreveal.interval", "features.reveal.interval"},
		{"plugins.swiper", "features.swiper"}, {"plugins.katex.css", "features.math.katex.css"}, {"plugins.mathjax.js", "features.math.mathjax.js"}, {"plugins.mermaid.js", "features.diagrams.mermaid.js"},
		{"inject.inject_head", "inject.head_end"}, {"inject.inject_footer", "inject.body_end"},
		{"ai.base_url", "blog.ai.base_url"}, {"ai.api_key", "blog.ai.api_key"}, {"ai.model", "blog.ai.model"}, {"ai.embedding_model", "blog.ai.embedding_model"},
		{"ai.summary_enabled", "blog.ai.summary.enabled"}, {"ai.allow_user_api", "blog.ai.summary.allow_user_api"}, {"ai.qa_enabled", "blog.ai.ask.enabled"}, {"ai.qa_allow_user_llm", "blog.ai.ask.allow_user_llm"},
		{"notebook.auto_excerpt", "profiles.notebook.listing.excerpt_length"}, {"notebook.license", "blog.content.notebook.license"}, {"notebook.share", "blog.content.notebook.share"},
	} {
		b.move(p[0], p[1])
	}
	for _, p := range [][2]string{
		{"article.type", "article.style"}, {"article.pin_style", "article.listing.pinned_layout"},
		{"style.prefers_theme", "appearance.color_scheme"}, {"style.text_align", "appearance.typography.content_align"},
		{"style.color.theme", "appearance.colors.primary"}, {"style.color.accent", "appearance.colors.accent"}, {"style.color.link", "appearance.colors.link"},
		{"style.font_size.root", "appearance.typography.font_size.root"}, {"style.font_size.code", "appearance.typography.font_size.inline_code"}, {"style.font_size.codeblock", "appearance.typography.font_size.code_block"},
		{"style.font_family.body", "appearance.typography.font_family.body"}, {"style.font_family.code", "appearance.typography.font_family.code"},
		{"style.corner_shape", "appearance.shape.corner"}, {"style.border_radius.card-l", "appearance.shape.radius.card_large"}, {"style.border_radius.card", "appearance.shape.radius.card"}, {"style.border_radius.card-s", "appearance.shape.radius.card_small"},
		{"style.border_radius.bar", "appearance.shape.radius.bar"}, {"style.border_radius.image-l", "appearance.shape.radius.image_large"}, {"style.border_radius.image", "appearance.shape.radius.image"}, {"style.border_radius.image-s", "appearance.shape.radius.image_small"},
		{"style.gradient.start", "appearance.gradients.primary_action"}, {"style.gradient.searchbar", "appearance.gradients.search_bar"},
		{"style.leftbar.background_image", "appearance.backgrounds.leftbar.image"}, {"style.leftbar.blur_px", "appearance.backgrounds.leftbar.backdrop.radius"},
		{"style.site.background_image", "appearance.backgrounds.page.image"}, {"style.site.blur_px", "appearance.backgrounds.page.backdrop.radius"}, {"style.site.blur_sat", "appearance.backgrounds.page.backdrop.saturation"}, {"style.site.blur_bg", "appearance.backgrounds.page.backdrop.overlay"},
		{"style.codeblock.scrollbar", "appearance.code_block.scrollbar_width"}, {"style.codeblock.highlightjs_theme", "appearance.code_block.highlight_stylesheet"},
		{"tag_plugins.button.default_color", "tags.button.default_color"}, {"tag_plugins.checkbox.interactive", "tags.checkbox.interactive"}, {"tag_plugins.note.border", "tags.note.border"}, {"tag_plugins.note.default_color", "tags.note.default_color"},
		{"tag_plugins.icon.default_color", "tags.icon.default_color"}, {"tag_plugins.mark.default_color", "tags.mark.default_color"}, {"tag_plugins.hashtag.default_color", "tags.hashtag.default_color"},
		{"tag_plugins.gallery.size", "tags.gallery.size"}, {"tag_plugins.gallery.ratio", "tags.gallery.aspect_ratio"},
		{"tag_plugins.quot.default", "tags.quot.default"}, {"tag_plugins.quot.hashtag.prefix", "tags.quot.hashtag.prefix"}, {"tag_plugins.quot.question.prefix", "tags.quot.question.prefix"},
	} {
		b.nonempty(p[0], p[1])
	}
	b.migrateSEO()
	b.validatePages()
	b.specials()
	b.wikiShelf(s)
	for _, path := range []string{"article.share.enabled", "article.reward.enabled"} {
		if v, ok := get(b.raw, path); ok && v == true {
			set(b.blog, "content.enabled", true)
			b.mappings = append(b.mappings, Mapping{"settings." + path, "blog.content.enabled", "旧本地分享或打赏显式启用，开启 Blog content 消费；不控制 blog_* 标签注册"})
		}
	}
	for _, leaf := range leaves(b.raw, "") {
		consumed := false
		for path := range b.used {
			if leaf == path || strings.HasPrefix(leaf, path+".") {
				consumed = true
				break
			}
		}
		if !consumed {
			b.conflict("unmapped_setting", leaf, "没有已核实的等效目标；原值保留在源快照，禁止静默丢弃")
		}
	}
	if _, err := stellarv2.ParseTheme(b.theme, stellarv2.Options{Source: "migration.theme"}); err != nil {
		b.diagnostics = append(b.diagnostics, schemaDiagnostics(err)...)
	}
	return ThemeCandidate{Version: 2, ThemeJSON: encode(b.theme), DocumentJSON: encode(map[string]any{"version": 2, "theme": b.theme}), BlogJSON: encode(b.blog), Mappings: b.mappings, Diagnostics: b.diagnostics}
}

func (b *themeBuilder) wikiShelf(s Snapshot) {
	v, exists := b.raw["wiki"]
	if !exists {
		return
	}
	b.used["wiki"] = true
	b.requireArray(v, "wiki")
	ids := []any{}
	for i, item := range array(v) {
		if id, ok := item.(string); ok {
			ids = append(ids, id)
			continue
		}
		b.requireObject(item, fmt.Sprintf("wiki[%d]", i))
		m := object(item)
		id := stringValue(m["id"])
		if id == "" {
			b.conflict("wiki_shelf_missing_id", fmt.Sprintf("wiki[%d].id", i), "旧 shelf 对象缺少项目 ID，不可静默略过")
			continue
		}
		ids = append(ids, id)
		var project Row
		for _, row := range s.Tables["wiki_projects"].Rows {
			if stringValue(row["id"]) == id {
				project = row
				break
			}
		}
		if project == nil {
			b.conflict("wiki_shelf_project_missing", fmt.Sprintf("wiki[%d].id", i), "旧 shelf 对应项目定义不存在；候选保持 ID 与源对象，须明确集合定义")
			continue
		}
		for _, key := range sortedKeys(m) {
			if key == "id" {
				continue
			}
			value := m[key]
			source := fmt.Sprintf("wiki[%d].%s", i, key)
			if key == "tags" {
				tags := []any{}
				for _, link := range s.Tables["wiki_project_tags"].Rows {
					if stringValue(link["project_id"]) == id {
						tags = append(tags, link["tag"])
					}
				}
				if encode(tags) == encode(value) {
					b.mark(source, "host.wiki_project_tags", "已存在项目标签一致；原关联与 shelf 顺序不变")
					continue
				}
			}
			if raw, ok := project[key]; ok && encode(raw) == encode(value) {
				b.mark(source, "host.wiki_projects."+key, "旧 shelf 与现有集合列一致，值保留在集合候选")
				continue
			}
			base := buildCollection(s, "wiki_projects", project).raw
			usingShelf := clone(base)
			target := map[string]string{"name": "name", "title": "name", "description": "description", "icon": "icon", "tags": "tags", "cover": "cover", "url": "route.path", "sort": "listing.order", "repo": "source.repository"}[key]
			if target != "" && key != "url" {
				set(usingShelf, target, value)
			}
			currentBlog := buildCollection(s, "wiki_projects", project).blog
			shelfBlog := clone(currentBlog)
			if key == "url" {
				set(shelfBlog, "content.wiki_external_url", value)
			}
			b.diagnostics = append(b.diagnostics, Diagnostic{Code: "wiki_shelf_definition_conflict", Source: "settings", Path: source, Message: "旧 shelf 冗余定义与当前项目表不同或无目标；官方 shelf 仅 ID 数组，需明确集合值来源", Blocking: true, Alternatives: []Alternative{{TargetSource: "wiki_projects/" + id, Name: "保留当前项目表集合定义", ConfigJSON: encode(base), BlogJSON: encode(currentBlog)}, {TargetSource: "wiki_projects/" + id, Name: "审核旧 shelf 字段覆盖（无等效字段需单独适配）", ConfigJSON: encode(usingShelf), BlogJSON: encode(shelfBlog)}}})
		}
	}
	set(b.blog, "data.wiki", ids)
	b.mark("wiki", "blog.data.wiki", "旧 shelf 顺序转官方项目 ID 字符串数组；冗余定义逐字段核对，冲突不能静默丢弃")
}
func leaves(m map[string]any, prefix string) []string {
	out := []string{}
	for _, k := range sortedKeys(m) {
		p := k
		if prefix != "" {
			p = prefix + "." + k
		}
		v := m[k]
		if n, ok := v.(map[string]any); ok && len(n) > 0 {
			out = append(out, leaves(n, p)...)
		} else {
			out = append(out, p)
		}
	}
	return out
}
func (b *themeBuilder) specials() {
	b.nonempty("services.siteinfo.api", "services.site_info.site_info_api.endpoint")
	if v, ok := get(b.raw, "menubar.items"); ok {
		b.requireArray(v, "menubar.items")
		items := array(v)
		out := []any{}
		for i, item := range items {
			b.checkKeys(item, fmt.Sprintf("menubar.items[%d]", i), "id", "title", "url", "icon", "theme")
			m := clone(object(item))
			if theme, ok := m["theme"]; ok {
				m["accent"] = theme
				delete(m, "theme")
			}
			out = append(out, m)
			b.mappings = append(b.mappings, Mapping{fmt.Sprintf("settings.menubar.items[%d].theme", i), fmt.Sprintf("theme.leftbar.menu[%d].accent", i), "旧菜单 CSS 颜色改名；严格 CSS color 校验"})
		}
		b.converted("menubar.items", "leftbar.menu", out, "菜单顺序/ID/标题/图标/URL 保留；theme 改名 accent")
	}
	if v, ok := get(b.raw, "footer.social"); ok {
		b.requireArray(v, "footer.social")
		out := []any{}
		for i, item := range array(v) {
			b.checkKeys(item, fmt.Sprintf("footer.social[%d]", i), "title", "url", "icon", "icon_key")
			m := clone(object(item))
			m["type"] = "link"
			if k := stringValue(m["icon_key"]); k != "" {
				m["icon"] = k
			}
			delete(m, "icon_key")
			out = append(out, m)
		}
		b.converted("footer.social", "leftbar.footer.actions", out, "社会链接顺序保留；icon_key 优先，否则原 HTML 图标严格验证")
	}
	if v, ok := get(b.raw, "footer.sitemap"); ok {
		b.requireArray(v, "footer.sitemap")
		groups := []any{}
		for i, item := range array(v) {
			b.checkKeys(item, fmt.Sprintf("footer.sitemap[%d]", i), "title", "items")
			m := object(item)
			b.requireArray(m["items"], fmt.Sprintf("footer.sitemap[%d].items", i))
			links := []any{}
			for j, link := range array(m["items"]) {
				b.checkKeys(link, fmt.Sprintf("footer.sitemap[%d].items[%d]", i, j), "title", "url")
				lm := object(link)
				links = append(links, "["+stringValue(lm["title"])+"]("+stringValue(lm["url"])+")")
			}
			groups = append(groups, map[string]any{"title": m["title"], "items": links})
		}
		b.converted("footer.sitemap", "footer.sitemap", groups, "链接对象转官方 Markdown 字符串，保留组及顺序")
	}
	for _, src := range []string{"article.auto_excerpt", "article.cover_ratio"} {
		if v, ok := get(b.raw, src); ok {
			target := "article.listing.excerpt_length"
			if strings.HasSuffix(src, "cover_ratio") {
				target = "article.listing.cover_ratio"
			}
			if stringValue(v) == "0" {
				b.mark(src, "theme."+target, "旧 0 代表默认；候选省略，继承上游默认，源原值保留")
			} else {
				b.converted(src, target, v, "数值保持")
			}
		}
	}
	if v, ok := get(b.raw, "article.card_style"); ok {
		if v == nil || v == "" {
			b.mark("article.card_style", "theme.article.listing.card_layout", "旧未声明覆盖继承上游默认")
		} else {
			b.converted("article.card_style", "article.listing.card_layout", v, "严格候选校验，不猜未知枚举")
		}
	}
	if v, ok := get(b.raw, "article.indent"); ok {
		b.requireBool(v, "article.indent")
		target := "never"
		if v == true {
			target = "always"
		}
		b.converted("article.indent", "article.paragraph_indent", target, "布尔缩进转 always/never")
	}
	if v, ok := get(b.raw, "article.tags"); ok {
		if v == nil {
			b.mark("article.tags", "theme.article.footer.show_tags", "null 为旧继承；候选省略，原值仍在源快照")
		} else {
			b.move("article.tags", "article.footer.show_tags")
		}
	}
	if v, ok := get(b.raw, "article.copyright"); ok {
		b.checkKeys(v, "article.copyright", "enabled", "license", "license_url")
		m := object(v)
		if enabled, exists := m["enabled"]; exists {
			b.requireBool(enabled, "article.copyright.enabled")
		}
		var license any = false
		if m["enabled"] == true {
			license = "本文采用 [" + stringValue(m["license"]) + "](" + stringValue(m["license_url"]) + ") 许可协议，转载请注明出处。"
		}
		b.converted("article.copyright", "article.footer.license", license, "按旧 enabled 保留关闭/Markdown 许可文字")
	}
	if v, ok := get(b.raw, "article.share"); ok {
		b.checkKeys(v, "article.share", "enabled", "platforms")
		m := object(v)
		if enabled, exists := m["enabled"]; exists {
			b.requireBool(enabled, "article.share.enabled")
		}
		if platforms, exists := m["platforms"]; exists {
			b.requireArray(platforms, "article.share.platforms")
		}
		b.move("article.share", "blog.content.share")
		out := []any{}
		if m["enabled"] == true {
			for _, platform := range array(m["platforms"]) {
				p := stringValue(platform)
				if p == "wechat" {
					p = "qrcode"
				}
				if p == "link" {
					continue
				}
				out = append(out, p)
			}
		}
		set(b.theme, "article.footer.share", out)
		b.mappings = append(b.mappings, Mapping{"settings.article.share.platforms", "theme.article.footer.share + blog.content.share", "wechat 候选转 qrcode；复制链接完整保留为 Blog 扩展"})
	}
	for _, src := range []string{"style.font_size.body", "style.font_family.codeblock", "style.leftbar.background_color_dark", "style.border_radius.block", "style.border_radius.navbar", "style.gradient.angle", "style.paginator", "style.header_prefix", "style.animated_avatar"} {
		if v, ok := get(b.raw, src); ok {
			if emptyLegacy(v) {
				b.mark(src, "source.snapshot", "旧空默认占位原值保留；上游没有独立覆盖")
			} else {
				b.conflict("appearance_no_equivalent", src, "没有一对一上游覆盖，需审核外观迁移；源值完整保留")
				b.used[src] = true
			}
		}
	}
	if v, ok := get(b.raw, "style.leftbar.background_opacity"); ok {
		if v == "" || v == nil {
			b.mark("style.leftbar.background_opacity", "theme.appearance.backgrounds.leftbar.opacity", "旧空值继承默认")
		} else if n, e := strconv.ParseFloat(stringValue(v), 64); e == nil {
			b.converted("style.leftbar.background_opacity", "appearance.backgrounds.leftbar.opacity", n, "数字字符串转数值")
		} else {
			b.conflict("invalid_opacity", "style.leftbar.background_opacity", "必须提供可校验的数值")
		}
	}
	if v, ok := get(b.raw, "plugins.scrollreveal.distance"); ok {
		n, e := strconv.ParseFloat(strings.TrimSuffix(stringValue(v), "px"), 64)
		if e == nil {
			b.converted("plugins.scrollreveal.distance", "features.reveal.distance", n, "旧 px 长度转官方数字 px")
		} else {
			b.conflict("reveal_distance", "plugins.scrollreveal.distance", "旧距离不是可无损转换的 px 数值")
		}
	}
	if v, ok := get(b.raw, "plugins.preload.service"); ok {
		b.mark("plugins.preload.service", "theme.features.link_prefetch", "flying_pages 使用官方实现")
		if v != "flying_pages" && v != "" {
			b.conflict("prefetch_provider", "plugins.preload.service", "非官方 flying_pages 服务不可静默换实现")
		}
	}
	if v, ok := get(b.raw, "plugins.pjax"); ok {
		b.checkKeys(v, "plugins.pjax", "enable", "selectors", "timeout")
		m := object(v)
		set(b.theme, "features.partial_navigation.enabled", m["enable"])
		b.mark("plugins.pjax", "theme.features.partial_navigation.enabled", "开关保留；官方 manifest 导航替代旧 PJAX")
		b.conflict("pjax_policy_removed", "plugins.pjax", "旧 selectors/timeout 与官方 manifest 导航没有直接等效字段；审核后明确接受官方策略")
	}
	for _, name := range []string{"katex", "mathjax", "mermaid"} {
		src := "plugins." + name + ".enable"
		if v, ok := get(b.raw, src); ok {
			target := "features.math.provider"
			if name == "mermaid" {
				target = "features.diagrams.provider"
			}
			if v == true {
				b.converted(src, target, name, "显式选择官方 provider")
			} else {
				b.mark(src, "theme."+target, "旧关闭；不选择 provider，默认 null")
			}
		}
	}
	if v, ok := get(b.raw, "data_cache"); ok {
		b.used["data_cache"] = true
		b.mark("data_cache", "source.snapshot", "旧缓存策略完整保留在源快照")
		b.conflict("data_cache_removed", "data_cache", "2.0 runtime policy 没有公开同等配置；禁止伪造 features.data_cache，需确认具体策略处理")
		_ = v
	}
	if v, ok := get(b.raw, "widgets.library"); ok {
		b.move("widgets.library", "blog.data.widgets")
		for id, item := range object(v) {
			if layout := stringValue(object(item)["layout"]); layout == "author" {
				b.conflict("widget_author_removed", "widgets.library."+id, "author widget 没有同等上游布局，完整定义保留在 Blog.data.widgets；需决定独立扩展消费")
			}
		}
	}
	if v, ok := get(b.raw, "widgets.pages"); ok {
		b.requireObject(v, "widgets.pages")
		b.used["widgets.pages"] = true
		if len(object(v)) > 0 {
			for profile, item := range object(v) {
				b.requireObject(item, "widgets.pages."+profile)
				for side, cfg := range object(item) {
					if side == "leftbar" || side == "rightbar" {
						set(b.theme, "profiles."+legacyProfile(profile)+"."+side+".widgets", cfg)
					} else {
						b.conflict("widget_region_unknown", "widgets.pages."+profile+"."+side, "未知旧区域，保持源值")
					}
				}
			}
			b.mark("widgets.pages", "theme.profiles", "旧 profile 侧栏逐条映射")
		} else {
			b.mark("widgets.pages", "theme.profiles", "旧空覆盖；继承官方默认区域")
		}
	}
	if v, ok := get(b.raw, "site_tree"); ok {
		b.requireObject(v, "site_tree")
		b.used["site_tree"] = true
		for profile, item := range object(v) {
			b.requireObject(item, "site_tree."+profile)
			for k, x := range object(item) {
				target := "profiles." + legacyProfile(profile) + "." + k
				switch k {
				case "leftbar", "rightbar":
					set(b.theme, target+".widgets", x)
				case "menu_id":
					set(b.theme, "profiles."+legacyProfile(profile)+".active_menu", x)
				case "path":
					set(b.theme, target, x)
				default:
					b.conflict("site_tree_unmapped", "site_tree."+profile+"."+k, "旧页面规则无已核实的上游映射，保留并要求审核")
				}
			}
		}
		b.mark("site_tree", "theme.profiles", "逐 profile 映射；空列表明确清空，缺省继承")
	}
	if v, ok := get(b.raw, "search.service"); ok {
		p := stringValue(v)
		switch p {
		case "", "local_search":
			set(b.theme, "search.provider", "local")
		case "algolia":
			set(b.theme, "search.provider", "algolia")
		case "go":
			set(b.blog, "search.provider", "go")
		default:
			b.conflict("search_provider_unmapped", "search.service", "服务不是官方 local/algolia 或显式 Go FTS，禁止猜测")
		}
		b.mark("search.service", "theme.search.provider / blog.search.provider", "空旧默认转 local；只有 go 显式选择 Go FTS")
	}
	for _, p := range [][2]string{{"cache_ttl", "cache_ttl_seconds"}, {"content", "include_content"}, {"field", "scope"}} {
		src := "search.local_search." + p[0]
		if v, ok := get(b.raw, src); ok {
			if v == nil || v == "" {
				b.mark(src, "theme.search.local."+p[1], "旧空占位继承官方默认")
			} else {
				b.converted(src, "search.local."+p[1], v, "原值保持")
			}
		}
	}
	for _, src := range []string{"search.local_search.path", "search.local_search.lazy_load", "search.local_search.skip_search"} {
		if v, ok := get(b.raw, src); ok {
			b.used[src] = true
			b.mark(src, "source.snapshot", "官方索引和加载策略由 runtime policy 管理")
			if !emptyLegacy(v) && v != true {
				b.conflict("search_policy_removed", src, "旧自定义策略没有直接上游字段，需明确确认")
			}
		}
	}
	b.move("search.algolia", "search.algolia")
	if v, ok := get(b.raw, "notebook.order_by"); ok {
		value := stringValue(v)
		direction := "asc"
		if strings.HasPrefix(value, "-") {
			direction = "desc"
		}
		field := strings.TrimPrefix(value, "-")
		b.converted("notebook.order_by", "profiles.notebook.listing.sort", map[string]any{"field": field, "direction": direction}, "旧排序符号转 direction，字段由严格 schema 验证")
	}
	if v, ok := get(b.raw, "tag_plugins.emoji"); ok {
		b.requireObject(v, "tag_plugins.emoji")
		sources := clone(object(v))
		def := stringValue(sources["default"])
		delete(sources, "default")
		name := ""
		for _, k := range sortedKeys(sources) {
			if sources[k] == def {
				name = k
				break
			}
		}
		if name == "" && def != "" {
			b.conflict("emoji_default_unmapped", "tag_plugins.emoji.default", "默认 URL 与任何具名 source 不同，需选择或新定义 source")
		} else if name != "" {
			set(b.theme, "tags.emoji.default_source", name)
		}
		set(b.theme, "tags.emoji.sources", sources)
		b.mark("tag_plugins.emoji", "theme.tags.emoji", "所有 source 原值保留；默认 URL 转匹配的具名 source")
	}
	for _, src := range []string{"tag_plugins.quot.hashtag.suffix", "tag_plugins.quot.question.suffix"} {
		if v, ok := get(b.raw, src); ok {
			if v == "" || v == nil {
				b.converted(src, strings.Replace(src, "tag_plugins.", "tags.", 1), nil, "空后缀转官方 null")
			} else {
				b.conflict("quot_suffix_removed", src, "官方特殊 quot suffix 仅接受 null，不能丢弃自定义值")
			}
		}
	}
	for _, src := range []string{"tag_plugins.copy", "tag_plugins.okr", "tag_plugins.timeline", "tag_plugins.gallery.layout"} {
		if v, ok := get(b.raw, src); ok {
			if src == "tag_plugins.okr" && isOfficialLegacyOKR(v) {
				b.mark(src, "source.snapshot", "旧 OKR 对象与固定 Stellar 2.0.0 边框、五状态颜色和 zh-CN 文案完全一致；原值留源快照，候选继承官方等效及请求 locale 文案")
				continue
			}
			b.used[src] = true
			b.mark(src, "source.snapshot", "旧参数在源快照完整保留")
			if !emptyLegacy(v) {
				b.conflict("tag_config_removed", src, "2.0 tags schema 不接收旧参数；需明确消费路径，不能静默丢弃")
			}
		}
	}
	if v, ok := get(b.raw, "cover"); ok {
		b.used["cover"] = true
		if !emptyLegacy(v) {
			b.conflict("home_cover_removed", "cover", "旧主页 cover 独立配置不在上游主题结构；需确认独立 Blog 展示或首页内容迁移")
		} else {
			b.mark("cover", "source.snapshot", "旧空配置无表现覆盖")
		}
	}
	// 已声明的 Blog 公开 data 外，不把 raw settings 作为模板运行时输入。
	if v, ok := get(b.raw, "site.url"); ok && stringValue(v) != "" {
		if _, err := url.ParseRequestURI(stringValue(v)); err != nil {
			b.conflict("site_url", "site.url", "站点 URL 必须可解析，源值不在公开报告中显示")
		}
	}
}
func (b *themeBuilder) checkKeys(v any, path string, keys ...string) {
	m, ok := v.(map[string]any)
	if !ok {
		b.conflict("invalid_legacy_object", path, "旧转换字段必须是对象；源原文保留，禁止修复为默认值")
		return
	}
	for _, key := range sortedKeys(m) {
		if !contains(keys, key) {
			b.conflict("unmapped_setting", path+"."+key, "转换对象含未知字段；源原文保留，禁止静默丢弃")
		}
	}
}
func (b *themeBuilder) requireArray(v any, path string) {
	if _, ok := v.([]any); !ok {
		b.conflict("invalid_legacy_array", path, "旧转换字段必须是数组；保留源原文，不能静默清空")
	}
}
func (b *themeBuilder) requireObject(v any, path string) {
	if _, ok := v.(map[string]any); !ok {
		b.conflict("invalid_legacy_object", path, "旧转换字段必须是对象；保留源原文，不能静默清空")
	}
}
func (b *themeBuilder) requireBool(v any, path string) {
	if _, ok := v.(bool); !ok {
		b.conflict("invalid_legacy_boolean", path, "旧转换字段必须是布尔值；保留源原文，不能猜默认开关")
	}
}
func array(v any) []any { a, _ := v.([]any); return a }
func emptyLegacy(v any) bool {
	if v == nil {
		return true
	}
	switch x := v.(type) {
	case string:
		return x == ""
	case map[string]any:
		for _, child := range x {
			if !emptyLegacy(child) {
				return false
			}
		}
		return true
	case []any:
		return len(x) == 0
	}
	return false
}
func legacyProfile(s string) string {
	aliases := map[string]string{"index": "blog_index", "wiki": "wiki", "notes": "notebooks", "error_page": "error"}
	if a := aliases[s]; a != "" {
		return a
	}
	return s
}
func schemaDiagnostics(err error) []Diagnostic {
	e, ok := err.(*stellarv2.SchemaError)
	if !ok {
		return []Diagnostic{{Code: "strict_validation", Source: "migration", Path: "root", Message: "候选严格校验失败（不输出原值）", Blocking: true}}
	}
	out := []Diagnostic{}
	for _, issue := range e.Issues {
		out = append(out, Diagnostic{Code: issue.Code, Source: issue.Source, Path: issue.Path, Message: issue.Expected, Blocking: issue.Severity != "warning"})
	}
	return out
}
