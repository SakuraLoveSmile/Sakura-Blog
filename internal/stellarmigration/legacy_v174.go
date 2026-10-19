package stellarmigration

import "html"

// 这些字段以旧 1.7.4 的消费者及固定 Stellar 2.0.0 schema 为依据。
// 只消费已核实的叶子；未知兄弟字段仍由 buildTheme 发出 unmapped_setting。
func (b *themeBuilder) migrateLegacyV174() {
	b.migrateLegacyFooter()
	if v, ok := get(b.raw, "style.error_page"); ok {
		if v == "" {
			b.converted("style.error_page", "profiles.error.image", nil, "旧空字符串不显示错误页图片；转显式 null，禁止继承上游示例图片")
		} else {
			b.move("style.error_page", "profiles.error.image")
		}
	}
	if v, ok := get(b.raw, "style.leftbar.ui_style"); ok {
		switch v {
		case "card", "glass":
			b.converted("style.leftbar.ui_style", "appearance.preset", v, "旧 card/glass 选择对应的 2.0 整体外观预设；采用该预设的官方布局与视觉")
		case "":
			b.converted("style.leftbar.ui_style", "appearance.preset", "card", "旧空值由宿主补为 card；选择对应的 2.0 card 预设")
		default:
			b.mark("style.leftbar.ui_style", "source.snapshot", "未知旧外观原文保留")
			b.conflict("appearance_no_equivalent", "style.leftbar.ui_style", "仅已核实旧 card/glass 与 2.0 对应预设；未知类型或枚举不能猜测")
		}
	}
	for _, path := range []string{"plugins.adaptive_text.enable", "plugins.copycode.enable", "plugins.swiper.enable"} {
		if v, exists := get(b.raw, path); exists {
			if _, valid := v.(bool); !valid {
				b.requireBool(v, path)
				b.mark(path, "source.snapshot", "无效开关原文保留")
			} else if v == true {
				b.mark(path, "source.snapshot", "旧显式开启与固定 2.0 按匹配 DOM 加载功能的官方策略一致；不伪造独立 enable 字段")
			} else {
				b.officialPolicy(path, "旧显式关闭没有 2.0 独立开关；官方会按匹配 DOM 加载对应功能")
			}
		}
	}
	for _, path := range []string{"style.gradient.avatar", "style.loading.loading", "style.loading.error", "plugins.copycode.default_text", "plugins.copycode.success_text", "plugins.copycode.toast"} {
		if v, exists := get(b.raw, path); exists {
			if text, valid := v.(string); valid && text == "" {
				b.mark(path, "source.snapshot", "旧空覆盖没有可见自定义效果；原值保留，候选使用官方默认")
			} else if valid {
				b.officialPolicy(path, "旧自定义外观或文案没有已核实的 2.0 等效字段")
			} else {
				b.mark(path, "source.snapshot", "无效旧文本原文保留")
				b.conflict("invalid_legacy_string", path, "旧配置必须是字符串，不能用官方策略批准无效类型")
			}
		}
	}
	if v, exists := get(b.raw, "style.page_transition.enable"); exists {
		// 旧 null 明确表示开启。2.0 无公开对应字段，不能把 null 当无效果。
		if _, valid := v.(bool); valid || v == nil {
			b.officialPolicy("style.page_transition.enable", "旧页面过渡（null 也代表开启）没有已核实的 2.0 等效配置")
		} else {
			b.mark("style.page_transition.enable", "source.snapshot", "无效旧过渡开关原文保留")
			b.requireBool(v, "style.page_transition.enable")
		}
	}
	if v, exists := get(b.raw, "plugins.mermaid.style_optimization"); exists {
		if v == false {
			b.mark("plugins.mermaid.style_optimization", "source.snapshot", "旧明确关闭额外 Stellar Mermaid CSS，与 2.0 官方 Mermaid 样式一致")
		} else if v == true {
			b.officialPolicy("plugins.mermaid.style_optimization", "旧额外 Stellar Mermaid CSS 没有 2.0 等效开关")
		} else {
			b.mark("plugins.mermaid.style_optimization", "source.snapshot", "无效开关原文保留")
			b.requireBool(v, "plugins.mermaid.style_optimization")
		}
	}
	if v, exists := get(b.raw, "plugins.mermaid.theme"); exists {
		switch v {
		case "default", "dark", "forest", "neutral":
			b.move("plugins.mermaid.theme", "features.diagrams.mermaid.theme")
		case "":
			b.mark("plugins.mermaid.theme", "source.snapshot", "旧空值继承 Mermaid 默认；额外样式的差异单独审核")
		default:
			b.mark("plugins.mermaid.theme", "source.snapshot", "不支持的旧 Mermaid 主题原文保留")
			b.conflict("mermaid_theme_unsupported", "plugins.mermaid.theme", "仅 default/dark/forest/neutral 有已核实的 2.0 枚举；其它主题不可静默替换")
		}
	}
	if v, exists := get(b.raw, "plugins.mathjax.v3"); exists {
		b.mark("plugins.mathjax.v3", "source.snapshot", "旧 MathJax 版本选择原文保留；2.0 仅支持 MathJax v3")
		if _, valid := v.(bool); !valid {
			b.requireBool(v, "plugins.mathjax.v3")
		} else if enabled, _ := get(b.raw, "plugins.mathjax.enable"); enabled != false && !(enabled == true && v == true) {
			b.conflict("mathjax_version_unsupported", "plugins.mathjax.v3", "只有旧 MathJax 明确关闭或明确开启 v3 可迁移；v2/缺省版本需先明确兼容处理，不能假称保留")
		}
	} else if enabled, _ := get(b.raw, "plugins.mathjax.enable"); enabled == true {
		b.conflict("mathjax_version_unsupported", "plugins.mathjax.v3", "旧 MathJax 已开启但没有明确 v3 版本；不能推断为 2.0 支持的 MathJax v3")
	}
}

func (b *themeBuilder) officialPolicy(path, message string) {
	b.mark(path, "source.snapshot", "旧值完整保留在源快照；待具名审核是否接受 2.0 官方行为")
	b.diagnostics = append(b.diagnostics, Diagnostic{Kind: "policy", Severity: "error", Code: "legacy_official_policy", Source: "settings", Path: path, Message: message + "；需要显式接受 2.0 官方行为", Blocking: true})
}

func (b *themeBuilder) migrateLegacyFooter() {
	content, hasContent := get(b.raw, "footer.content")
	beian, hasBeian := get(b.raw, "footer.beian")
	if !hasContent && !hasBeian {
		return
	}
	text, validContent := content.(string)
	registration, validBeian := beian.(string)
	if hasContent && !validContent {
		b.mark("footer.content", "source.snapshot", "无效页脚原文保留")
		b.conflict("invalid_legacy_string", "footer.content", "旧页脚正文必须是字符串")
	}
	if hasBeian && !validBeian {
		b.mark("footer.beian", "source.snapshot", "无效备案号原文保留")
		b.conflict("invalid_legacy_string", "footer.beian", "旧备案号必须是字符串")
	}
	if (hasContent && !validContent) || (hasBeian && !validBeian) {
		return
	}
	if text != "" && registration != "" {
		// 旧模板分别渲染 Markdown 正文与备案 HTML；拼接后统一渲染不等效，
		// 未闭合代码块或 HTML 块尤其可能吞掉备案链接，不能尝试自动闭合。
		set(b.theme, "footer.content", text)
		b.mark("footer.content", "theme.footer.content", "旧非空 Markdown 正文逐字保留；与备案号的独立渲染尚无等效迁移")
		b.mark("footer.beian", "source.snapshot", "旧备案号完整保留在源快照；禁止拼入任意 Markdown 后假称显示等效")
		b.conflict("footer_beian_content_conflict", "footer.beian", "旧非空页脚正文与备案号原本分别渲染；合并可能使 Markdown 代码块或 HTML 吞掉备案链接，须实现等效独立渲染后再迁移，不能接受官方策略略过")
		return
	}
	if text == "" {
		title, _ := get(b.raw, "site.title")
		text = `<p>&copy; ` + html.EscapeString(stringValue(title)) + ` · Powered by <a href="https://github.com/xaoxuu/hexo-theme-stellar" target="_blank" rel="noopener">Stellar</a></p>`
	}
	if registration != "" {
		text += "\n\n" + `<p><a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener">` + html.EscapeString(registration) + `</a></p>`
	}
	set(b.theme, "footer.content", text)
	if hasContent {
		b.mark("footer.content", "theme.footer.content", "保留旧 Markdown；空正文显式重建旧站点署名 fallback，禁止改成上游示例许可文字")
	}
	if hasBeian {
		b.mark("footer.beian", "theme.footer.content", "非空备案号按旧模板追加工信部链接并转义文本；空值不添加备案段，原文保留")
	}
}
