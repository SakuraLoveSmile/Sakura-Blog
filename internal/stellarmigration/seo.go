package stellarmigration

import "strings"

// 已冻结的旧 SEO 六项有上游等价；余下七项由 typed Blog.seo 消费。
// default_description/keywords 不覆盖独立 Site 来源，runtime 优先级由宿主消费。
func (b *themeBuilder) migrateSEO() {
	value, exists := get(b.raw, "seo")
	if !exists {
		return
	}
	source, valid := value.(map[string]any)
	if !valid {
		b.move("seo", "blog.seo")
		b.requireObject(value, "seo")
		return
	}
	remaining := clone(source)
	b.checkKeys(value, "seo", "preconnect", "canonical", "open_graph", "structured_data", "default_description", "default_keywords", "robots_extra", "no_index", "google_verification", "bing_verification", "baidu_verification")
	for _, bag := range []struct {
		name string
		keys []string
	}{
		{"canonical", []string{"originalHost", "officialHosts"}},
		{"open_graph", []string{"enable", "twitter_id"}},
		{"structured_data", []string{"links"}},
	} {
		if v, ok := source[bag.name]; ok {
			b.checkKeys(v, "seo."+bag.name, bag.keys...)
		}
	}
	for _, spec := range []struct{ source, target, kind string }{
		{"preconnect", "preconnect", "array"},
		{"canonical.originalHost", "canonical.host", "host"},
		{"canonical.officialHosts", "canonical.allowed_hosts", "array"},
		{"open_graph.enable", "open_graph.enabled", "enabled"},
		{"open_graph.twitter_id", "open_graph.twitter_id", "string"},
		{"structured_data.links", "structured_data.same_as", "array"},
	} {
		v, ok := get(source, spec.source)
		if !ok {
			continue
		}
		switch spec.kind {
		case "array":
			if v == nil {
				v = []any{}
			} else {
				items, ok := v.([]any)
				if !ok {
					b.requireArray(v, "seo."+spec.source)
					continue
				}
				validItems := true
				for _, item := range items {
					if _, ok := item.(string); !ok {
						validItems = false
					}
				}
				if !validItems {
					b.conflict("invalid_legacy_string_array", "seo."+spec.source, "SEO 列表含非字符串，不丢弃坏项或猜 URL")
					continue
				}
			}
		case "enabled":
			if v == nil {
				v = true
			} else {
				if _, ok := v.(bool); !ok {
					b.requireBool(v, "seo."+spec.source)
					continue
				}
			}
		case "host", "string":
			if v != nil {
				if _, ok := v.(string); !ok {
					b.conflict("invalid_legacy_string", "seo."+spec.source, "SEO 字段必须为字符串或 null；不格式化未知原值")
					continue
				}
			}
			if spec.kind == "host" && v == "" {
				v = nil
			}
		}
		if current, present := get(b.theme, spec.target); present && encode(current) != encode(v) {
			b.conflict("seo_source_conflict", "seo."+spec.source, "同一上游 SEO 目标已有不同来源，不覆盖原值或静默选取")
			continue
		}
		b.converted("seo."+spec.source, spec.target, v, "采用已核实的官方 SEO 字段；保持显式关闭/空数组，旧 nil slice 转空数组，空 canonical host 转 null")
		deleteSEOPath(remaining, spec.source)
	}
	for _, field := range []string{"default_description", "default_keywords", "robots_extra", "google_verification", "bing_verification", "baidu_verification"} {
		if v, ok := remaining[field]; ok && v != nil {
			if _, valid := v.(string); !valid {
				b.conflict("invalid_legacy_string", "seo."+field, "宿主 SEO 扩展须保留字符串或 null，不猜默认内容")
			}
		}
	}
	if v, ok := remaining["no_index"]; ok {
		b.requireBool(v, "seo.no_index")
	}
	if len(remaining) > 0 {
		set(b.blog, "seo", remaining)
		b.mark("seo", "blog.seo", "仅保留宿主 SEO 默认文案/robots/验证及未映射原字段；不覆盖 site，未知字段继续阻断")
		b.conflict(globalConsumerCode(ConsumerSEO), "seo", "已批准的宿主 SEO 扩展仅存储不代表已消费；须实测 typed 默认文案、head 与 robots 后显式声明能力")
	} else {
		b.mark("seo", "source.snapshot", "所有有值 SEO 项已逐字段映射官方契约；完整旧对象仍留私密源快照")
	}
}
func deleteSEOPath(m map[string]any, path string) {
	parts := strings.Split(path, ".")
	if len(parts) == 1 {
		delete(m, path)
		return
	}
	child, ok := m[parts[0]].(map[string]any)
	if !ok {
		return
	}
	delete(child, parts[1])
	if len(child) == 0 {
		delete(m, parts[0])
	}
}
