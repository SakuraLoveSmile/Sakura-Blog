package stellarmigration

import (
	"fmt"
	"sort"
	"strings"

	"blog/internal/post"
	"blog/internal/stellarv2"
	"github.com/goccy/go-yaml"
)

type recordBuilder struct {
	candidate RecordCandidate
	raw, blog map[string]any
	row       Row
}

func body(row Row) string {
	if v, ok := row["content_md"]; ok {
		return stringValue(v)
	}
	return stringValue(row["content"])
}
func (b *recordBuilder) mapped(src, dst, msg string) {
	b.candidate.Mappings = append(b.candidate.Mappings, Mapping{b.candidate.Source + "." + src, dst, msg})
}
func (b *recordBuilder) move(src, dst string) {
	v, ok := b.row[src]
	if !ok {
		return
	}
	set(b.raw, dst, v)
	b.mapped(src, "front_matter."+dst, "公开展示字段原值保留")
}
func (b *recordBuilder) nonempty(src, dst string) {
	if v, ok := b.row[src]; ok && v != nil && v != "" {
		b.move(src, dst)
	}
}
func (b *recordBuilder) conflict(code, path, msg string, alternatives ...Alternative) {
	b.candidate.Diagnostics = append(b.candidate.Diagnostics, Diagnostic{Code: code, Source: b.candidate.Source, Path: path, Message: msg, Blocking: true, Alternatives: alternatives})
}
func (b *recordBuilder) finish() RecordCandidate {
	b.candidate.ConfigJSON = encode(b.raw)
	b.candidate.BlogJSON = encode(b.blog)
	return b.candidate
}
func newRecord(table string, row Row) *recordBuilder {
	id := stringValue(row["id"])
	profile := map[string]string{"posts": "post", "pages": "page", "wikidocs": "wiki", "wiki_projects": "wiki", "topics": "topic", "notebooks": "notebook", "notes": "notebook"}[table]
	p := RecordCandidate{Source: table + "/" + id, Table: table, ID: id, Profile: profile, SourceDigest: digest(row), StellarVersion: 2, Markdown: body(row), Mappings: []Mapping{}, Diagnostics: []Diagnostic{}}
	b := &recordBuilder{candidate: p, row: row, raw: map[string]any{}, blog: map[string]any{"native_comments": map[string]any{"enabled": nil}}}
	if stringValue(row["stellar_version"]) == "2" {
		b.candidate.ConfigJSON = stringValue(row["stellar_config"])
		b.candidate.BlogJSON = stringValue(row["blog_extensions"])
		b.candidate.Preserved = true
		return b
	}
	if v, exists := row["stellar_version"]; exists && v != nil && stringValue(v) != "0" && stringValue(v) != "" {
		b.conflict("unsupported_content_version", "stellar_version", "源版本不是 0 或 2；不得隐式覆盖未知或更高版本")
	}
	for _, key := range []string{"stellar_config", "blog_extensions"} {
		if value := strings.TrimSpace(stringValue(row[key])); value != "" {
			b.conflict("unversioned_metadata", key, "未 version 2 的记录已有原始元数据；可能是未完成迁移，源文完整保留，不能根据旧列覆盖它")
		}
	}
	b.move("title", "title")
	// 宿主权限/身份/发布时间/草稿/HTML 缓存不搬进上游 schema，也不更新。
	for _, k := range sortedKeys(row) {
		b.mapped(k, "host."+table+"."+k, "源列保持；候选只新增严格元数据与明确正文转换")
	}
	return b
}

// Preview produces all content candidates before resolving membership. Unknown
// fields stay in the private source snapshot and produce blocking diagnostics.
func Preview(s Snapshot) (Candidate, error) {
	return PreviewWithCapabilities(s, nil)
}

// PreviewWithCapabilities only accepts explicitly supplied, verified consumer
// capabilities. It retains every diagnostic and never resolves policy choices.
func PreviewWithCapabilities(s Snapshot, capabilities ConsumerCapabilities) (Candidate, error) {
	if err := s.Verify(); err != nil {
		return Candidate{}, err
	}
	if err := validateCapabilities(capabilities); err != nil {
		return Candidate{}, err
	}
	c := Candidate{Version: 2, Source: clone(s), Theme: buildTheme(s), Records: []RecordCandidate{}, Diagnostics: []Diagnostic{}}
	c.ConsumerCapabilities = clone(capabilities)
	collections := map[string]map[string]any{}
	for _, table := range []string{"wiki_projects", "topics", "notebooks"} {
		for _, row := range s.Tables[table].Rows {
			b := buildCollection(s, table, row)
			p := b.candidate
			if !p.Preserved {
				p = b.finish()
			}
			c.Records = append(c.Records, p)
			v, e := decode(p.ConfigJSON)
			if e == nil {
				result, err := stellarv2.ParseCollection(v, stellarv2.Options{Source: p.Source})
				if err == nil {
					key := collectionKey(p)
					if _, exists := collections[key]; exists {
						c.Diagnostics = append(c.Diagnostics, Diagnostic{Code: "duplicate_collection_id", Source: p.Source, Path: "collection", Message: "多个源集合声明相同 profile/ID，禁止覆盖 Registry", Blocking: true})
					}
					collections[key] = result.Value
				}
			}
		}
	}
	registry := stellarv2.CreateCollectionRegistry(collections)
	for _, table := range []string{"posts", "pages", "wikidocs", "notes"} {
		for _, row := range s.Tables[table].Rows {
			b := buildPage(s, table, row)
			p := b.candidate
			if !p.Preserved {
				p = b.finish()
				p.Markdown, p.Diagnostics, p.Mappings = migrateMarkdown(p.Markdown, p.Source, p.Diagnostics, p.Mappings)
			} else {
				proposed, ds, _ := migrateMarkdown(p.Markdown, p.Source, nil, nil)
				p.Diagnostics = append(p.Diagnostics, ds...)
				if proposed != p.Markdown {
					p.Diagnostics = append(p.Diagnostics, Diagnostic{Code: "versioned_markdown_legacy", Source: p.Source, Path: "markdown", Message: "已 version 2 的正文仍含旧扩展名或旧缺省语义；原文逐字保留，须单独审核修复", Blocking: true})
				}
			}
			validateRecord(&p, registry)
			c.Records = append(c.Records, p)
		}
	}
	for _, path := range sortedKeys(s.Files) {
		p := buildFile(s.Files[path], path, s)
		validateRecord(&p, registry)
		c.Records = append(c.Records, p)
	}
	for i := range c.Records {
		if isCollection(c.Records[i].Table) {
			validateRecord(&c.Records[i], registry)
		}
	}
	if s.NotebookSchema.Required() {
		c.Diagnostics = append(c.Diagnostics, Diagnostic{Code: "legacy_notebook_schema", Source: "schema", Path: "notebooks/notes", Message: "旧 INTEGER ID、nb_order、content/tags 表与新 TEXT ID、slug/status/content_md/tags_json 不兼容；必须单独审核 schema 转换，不能先假设 EnsureTable 会升级", Blocking: true})
	}
	c.Diagnostics = classifyDiagnostics(uniqueDiagnostics(c.Diagnostics), capabilities)
	c.Theme.Diagnostics = classifyDiagnostics(uniqueDiagnostics(c.Theme.Diagnostics), capabilities)
	for i := range c.Records {
		c.Records[i].Diagnostics = classifyDiagnostics(uniqueDiagnostics(c.Records[i].Diagnostics), capabilities)
	}
	return c, nil
}

func uniqueDiagnostics(ds []Diagnostic) []Diagnostic {
	out := []Diagnostic{}
	seen := map[string]bool{}
	for _, d := range ds {
		key := encode(d)
		if !seen[key] {
			seen[key] = true
			out = append(out, d)
		}
	}
	return out
}
func contains(a []string, v string) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}
func isCollection(t string) bool { return t == "wiki_projects" || t == "topics" || t == "notebooks" }
func collectionKey(p RecordCandidate) string {
	id := p.CollectionID
	prefix := p.Profile
	if p.Profile == "notebook" {
		prefix = "notebooks"
	}
	return prefix + "/" + id
}
func buildCollection(s Snapshot, table string, row Row) *recordBuilder {
	b := newRecord(table, row)
	b.candidate.CollectionID = b.candidate.ID
	if table == "topics" {
		b.candidate.CollectionID = stringValue(row["slug"])
		b.candidate.Path = "/topic/" + stringValue(row["slug"])
		b.mapped("slug", "collection.id", stringValue(row["slug"]))
	} else if table == "notebooks" {
		b.candidate.Path = "/notebooks/" + b.candidate.ID
	} else {
		b.candidate.Path = "/wiki/" + b.candidate.ID
	}
	if b.candidate.Preserved {
		return b
	}
	delete(b.raw, "title")
	for _, p := range [][2]string{{"name", "name"}, {"description", "description"}, {"icon", "icon"}, {"cover", "cover"}, {"headline", "headline"}, {"subtitle", "tagline"}, {"repo", "source.repository"}, {"branch", "source.branch"}} {
		b.nonempty(p[0], p[1])
	}
	set(b.raw, "route.path", strings.TrimPrefix(b.candidate.Path, "/"))
	b.mapped("id/slug", "front_matter.route.path", "保持旧公开集合路径")
	if table == "topics" {
		// Topic sort 是旧 Topic 卡片之间的排序，而官方 topic listing.sort
		// 排的是成员文章；不可把二者静默混用。
		set(b.blog, "content.collection_order", row["sort"])
		b.mapped("sort", "blog.content.collection_order", "旧集合列表排序独立保留，不能错用上游成员排序")
		b.conflict("topic_order_consumer", "sort", "旧专栏集合排序保留于 Blog 扩展；宿主集合列表必须消费后才能执行迁移")
	}
	if table == "notebooks" {
		if v, ok := row["sort"]; ok {
			set(b.raw, "listing.order", v)
		} else if v, ok := row["nb_order"]; ok {
			set(b.raw, "listing.order", v)
			b.mapped("nb_order", "front_matter.listing.order", "排序数值与方向保持")
		}
	}
	if table == "wiki_projects" {
		b.move("sort", "listing.order")
		set(b.raw, "listing.priority", numberBool(row["pin"]))
		set(b.raw, "visibility.listed", boolSQL(row["visible"]))
		name, title := stringValue(row["name"]), stringValue(row["title"])
		if name == "" {
			b.raw["name"] = title
			b.mapped("title", "front_matter.name", "旧 name 空，显示标题保留为官方 name")
		} else if title != "" && title != name {
			shown := clone(b.raw)
			shown["name"] = title
			source := clone(b.raw)
			b.conflict("wiki_name_title", "name/title", "旧 name 与 title 同时存在且不同；官方 Collection 只有 name/headline，须审核显示标题与技术名的归属", Alternative{Name: "保留旧 name 为 name", ConfigJSON: encode(source)}, Alternative{Name: "保留旧 title 为显示 name", ConfigJSON: encode(shown)})
		}
		docs := []Row{}
		for _, d := range s.Tables["wikidocs"].Rows {
			if stringValue(d["project_id"]) == b.candidate.ID {
				docs = append(docs, d)
			}
		}
		sort.SliceStable(docs, func(i, j int) bool {
			a, _ := strconvNumber(docs[i]["doc_order"])
			z, _ := strconvNumber(docs[j]["doc_order"])
			if a == z {
				return stringValue(docs[i]["id"]) < stringValue(docs[j]["id"])
			}
			return a < z
		})
		tree := []any{}
		first := ""
		for _, d := range docs {
			tree = append(tree, stringValue(d["slug"]))
			if first == "" && d["status"] == "published" {
				first = stringValue(d["slug"])
			}
		}
		set(b.raw, "navigation.tree", tree)
		b.mapped("wikidocs.project_id/doc_order/slug", "front_matter.navigation.tree", "完整成员与顺序保留，权限仍由宿主状态控制")
		if first != "" {
			legacy := clone(b.blog)
			set(legacy, "content.wiki_homepage_redirect", b.candidate.Path+"/"+first)
			b.conflict("wiki_homepage", "route.path/navigation.tree", "旧项目入口重定向至第一篇已发布文档；上游会生成首页/Collection 模型。两个候选都保留成员身份，需审核入口行为", Alternative{Name: "独立 Blog 扩展保留旧重定向", ConfigJSON: encode(b.raw), BlogJSON: encode(legacy)}, Alternative{Name: "采用官方 Collection 首页", ConfigJSON: encode(b.raw), BlogJSON: encode(b.blog)})
		}
		if u := stringValue(row["url"]); u != "" {
			set(b.blog, "content.wiki_external_url", u)
			b.mapped("url", "blog.content.wiki_external_url", "旧外链作为已授权 Blog 扩展完整保留；canonical route/path/homepage 仍为本地路径")
			b.conflict("wiki_external_url_consumer", "url", "旧外链已保留为独立 Blog 入口；模板消费未显式验证前继续阻断，不能将 HTTP URL 写入 route.path")
		}
		if available := stringValue(row["available"]); available != "" {
			set(b.blog, "content.available", available)
			b.conflict("wiki_available", "available", "旧可用性标识没有一对一官方 audience 语义，保留扩展后需明确消费")
		}
		if hero := stringValue(row["hero_json"]); hero != "" {
			parsed, e := decode(hero)
			if e != nil {
				b.conflict("invalid_hero", "hero_json", "旧 Hero JSON 无法解析，原文保留")
			} else {
				set(b.blog, "content.legacy_hero", parsed)
				b.conflict("wiki_hero", "hero_json/cover/title", "Hero title/background/start 与 cover/标题有独立来源，禁止猜测；完整旧 Hero 保留，需显式选择上游 Hero 转换", Alternative{Name: "保留独立旧 Hero", ConfigJSON: encode(b.raw), BlogJSON: encode(b.blog)}, Alternative{Name: "关闭 Hero 保留 Collection 卡片", ConfigJSON: encode(b.raw), BlogJSON: encode(map[string]any{"native_comments": map[string]any{"enabled": nil}})})
			}
		}
	}
	return b
}
func numberBool(v any) any {
	if v == nil {
		return 0
	}
	if value, ok := v.(bool); ok {
		if value {
			return 1
		}
		return 0
	}
	return v
}
func boolSQL(v any) bool { return v == true || stringValue(v) == "1" }
func strconvNumber(v any) (float64, error) {
	switch n := v.(type) {
	case interface{ Float64() (float64, error) }:
		return n.Float64()
	}
	var n float64
	_, err := fmt.Sscan(stringValue(v), &n)
	return n, err
}

func buildPage(s Snapshot, table string, row Row) *recordBuilder {
	b := newRecord(table, row)
	slug := stringValue(row["slug"])
	switch table {
	case "posts":
		b.candidate.Path = "/post/" + slug
	case "pages":
		b.candidate.Path = "/" + slug
	case "wikidocs":
		b.candidate.Path = "/wiki/" + stringValue(row["project_id"]) + "/" + slug
	case "notes":
		b.candidate.Path = ""
	}
	if b.candidate.Preserved {
		return b
	}
	for _, p := range [][2]string{{"excerpt", "excerpt"}, {"cover", "cover"}, {"meta_description", "description"}, {"meta_keywords", "keywords"}, {"ai_label", "article.ai_label"}, {"created_at", "date"}, {"updated_at", "updated"}} {
		b.nonempty(p[0], p[1])
	}
	if author := stringValue(row["author"]); author != "" {
		set(b.raw, "article.author", author)
		b.mapped("author", "front_matter.article.author", "单作者 ID 保留；所有关联作者不修改")
	}
	if table == "posts" {
		set(b.raw, "listing.priority", numberBool(row["pinned"]))
		b.mapped("pinned", "front_matter.listing.priority", "旧布尔 false/true 转 0/1")
		if refs := stringValue(row["references"]); refs != "" {
			v, e := decode(refs)
			if e != nil {
				b.conflict("invalid_references", "references", "旧参考文献不是有效 JSON，原文保留")
			} else {
				set(b.raw, "footer.references", v)
				b.mapped("references", "front_matter.footer.references", "原始 JSON 数值/字符串保真")
			}
		}
		if poster := stringValue(row["poster"]); poster != "" {
			v, e := decode(poster)
			if e != nil {
				b.conflict("invalid_poster", "poster", "旧海报 JSON 无法解析；原文保留")
			} else {
				set(b.blog, "content.poster", v)
				b.conflict("poster_consumer", "poster", "旧 poster headline/topic/caption/color 完整保留为 Blog 扩展；尚需接通模板消费，不假称视觉保留")
			}
		}
		if banner := stringValue(row["banner"]); banner != "" {
			keep := clone(b.blog)
			set(keep, "content.banner_image", banner)
			cover := clone(b.raw)
			cover["cover"] = banner
			set(cover, "banner.background", true)
			b.conflict("banner_cover", "banner/cover", "旧 banner 与 cover 是独立图源，官方 banner.background 是布尔且图源来自 cover；禁止覆盖旧 cover", Alternative{Name: "保留独立 Blog banner 图源", ConfigJSON: encode(b.raw), BlogJSON: encode(keep)}, Alternative{Name: "用旧 banner 作为官方 cover", ConfigJSON: encode(cover), BlogJSON: encode(b.blog)})
			set(b.blog, "content.banner_image", banner)
		}
		topicID, topicText := stringValue(row["topic_id"]), stringValue(row["topic"])
		found := ""
		legacyName := ""
		for _, t := range s.Tables["topics"].Rows {
			if topicID != "" && topicID != "0" && stringValue(t["id"]) == topicID {
				found = stringValue(t["slug"])
				legacyName = stringValue(t["name"])
			}
		}
		if found == "" && topicText != "" {
			for _, t := range s.Tables["topics"].Rows {
				if stringValue(t["name"]) == topicText {
					if found != "" {
						b.conflict("topic_ambiguous", "topic", "多个定义匹配旧专栏名称")
					}
					found = stringValue(t["slug"])
					legacyName = topicText
				}
			}
		}
		if found != "" {
			set(b.raw, "collection", map[string]any{"profile": "topic", "id": found})
			b.candidate.Profile = "topic"
			b.mapped("topic_id/topic", "front_matter.collection", "迁移预览使用确切旧归属；strict runtime 不猜旧列")
			if topicText != "" && legacyName != topicText {
				b.conflict("topic_column_conflict", "topic_id/topic", "旧 ID 与名称指向不同归属，须明确选择")
			}
		} else if topicText != "" || (topicID != "" && topicID != "0") {
			b.conflict("topic_not_found", "topic_id/topic", "旧专栏没有有效定义，不能删除归属")
		}
		for _, key := range []string{"tags", "category"} {
			if text := stringValue(row[key]); text != "" {
				values := []any{}
				if key == "tags" {
					for _, v := range strings.Split(text, ",") {
						values = append(values, v)
					}
				} else {
					values = append(values, text)
				}
				target := key
				if key == "category" {
					target = "categories"
				}
				b.raw[target] = values
			}
		}
		links := []Row{}
		for _, link := range s.Tables["post_categories"].Rows {
			if stringValue(link["post_id"]) == b.candidate.ID {
				links = append(links, link)
			}
		}
		if len(links) > 0 {
			sort.SliceStable(links, func(i, j int) bool {
				a, _ := strconvNumber(links[i]["position"])
				z, _ := strconvNumber(links[j]["position"])
				return a < z
			})
			paths := []any{}
			for _, link := range links {
				path, e := categoryPath(s, stringValue(link["category_id"]), map[string]bool{})
				if e != nil {
					b.conflict("category_relationship", "post_categories", "分类关联缺失或层级循环，不能截断完整分类")
				} else {
					paths = append(paths, path)
				}
			}
			b.raw["categories"] = paths
			b.mapped("post_categories/categories", "front_matter.categories", "所有完整层级路径与关联 position 顺序保留")
		}
		authors := []Row{}
		for _, a := range s.Tables["post_authors"].Rows {
			if stringValue(a["post_id"]) == b.candidate.ID {
				authors = append(authors, a)
			}
		}
		if len(authors) > 0 {
			sort.SliceStable(authors, func(i, j int) bool {
				a, _ := strconvNumber(authors[i]["sort_order"])
				z, _ := strconvNumber(authors[j]["sort_order"])
				return a < z
			})
			set(b.blog, "content.authors", authors)
			if len(authors) > 1 {
				b.conflict("multiple_authors", "post_authors", "全部关联作者与顺序原样保留，官方 article.author 单字符串不能截断多作者；Blog 消费须接通")
			}
		}
		if summary := stringValue(row["ai_summary"]); summary != "" {
			set(b.blog, "content.ai_summary", summary)
		}
	}
	if table == "pages" {
		for _, side := range []string{"leftbar", "rightbar"} {
			key := side + "_widgets"
			if raw := stringValue(row[key]); raw != "" {
				v, e := decode(raw)
				if e != nil {
					b.conflict("invalid_widgets", key, "旧侧栏 JSON 无法解析，原文保留")
				} else {
					widgets := []any{}
					for _, w := range array(v) {
						if _, ok := w.(string); ok {
							widgets = append(widgets, w)
						} else {
							m := object(w)
							entry := clone(object(m["params"]))
							entry["id"] = m["name"]
							widgets = append(widgets, entry)
						}
					}
					set(b.raw, side+".widgets", widgets)
					b.mapped(key, "front_matter."+side+".widgets", "空列表明确清空；widget 覆盖按官方实例结构校验")
				}
			}
		}
		if info := stringValue(row["banner_info"]); info != "" {
			v, e := decode(info)
			if e != nil {
				b.conflict("invalid_banner_info", "banner_info", "旧横幅参数无法解析，原文保留")
			} else {
				for _, pair := range [][2]string{{"title", "headline"}, {"subtitle", "tagline"}, {"avatar", "avatar"}} {
					if x := object(v)[pair[0]]; x != nil && x != "" {
						set(b.raw, "banner."+pair[1], x)
					}
				}
				if x := object(v)["icon"]; x != nil && x != "" {
					set(b.blog, "content.banner_icon", x)
				}
			}
		}
		if banner := stringValue(row["banner"]); banner != "" {
			set(b.raw, "cover", banner)
			set(b.raw, "banner.background", true)
			b.mapped("banner", "front_matter.cover + banner.background", "页面无独立 cover 列；保留旧图源")
		}
		if v, ok := row["breadcrumb"]; ok {
			set(b.raw, "breadcrumb", boolSQL(v))
		}
	}
	if table == "wikidocs" {
		set(b.raw, "collection", map[string]any{"profile": "wiki", "id": stringValue(row["project_id"])})
		b.mapped("project_id", "front_matter.collection", "原 Wiki 归属保持，不移动文档")
	}
	if table == "notes" {
		set(b.raw, "collection", map[string]any{"profile": "notebook", "id": stringValue(row["notebook_id"])})
		b.mapped("notebook_id", "front_matter.collection", "旧数字 ID 的路径文本保持；不换父本")
		if v, ok := row["tags_json"]; ok {
			parsed, e := decode(stringValue(v))
			if e == nil {
				b.raw["tags"] = parsed
			} else {
				b.conflict("invalid_tags", "tags_json", "标签 JSON 无效，保留原值")
			}
		} else {
			tags := []any{}
			if value := stringValue(row["tags"]); value != "" {
				for _, x := range strings.Split(value, ",") {
					tags = append(tags, x)
				}
			}
			b.raw["tags"] = tags
		}
		if !contains(s.Tables["notes"].Columns, "slug") {
			b.conflict("note_missing_slug_status", "slug/status", "旧笔记无 slug/status/owner，当前没有逐笔记公开路由；迁移须显式决定新公开路径与发布状态，不可假定作者", Alternative{Name: "保留旧 ID 与当前未公开状态", ConfigJSON: encode(b.raw), BlogJSON: encode(b.blog)}, Alternative{Name: "另行确认 slug/状态/ownership 迁移", ConfigJSON: encode(b.raw), BlogJSON: encode(b.blog)})
		}
	}
	return b
}
func categoryPath(s Snapshot, id string, visited map[string]bool) (string, error) {
	if visited[id] {
		return "", fmt.Errorf("category cycle")
	}
	visited[id] = true
	for _, row := range s.Tables["categories"].Rows {
		if stringValue(row["id"]) != id {
			continue
		}
		name := stringValue(row["name"])
		parent := stringValue(row["parent_id"])
		if parent != "" && parent != "0" {
			path, err := categoryPath(s, parent, visited)
			if err != nil {
				return "", err
			}
			return path + "/" + name, nil
		}
		return name, nil
	}
	return "", fmt.Errorf("category missing")
}
func validateRecord(p *RecordCandidate, registry *stellarv2.CollectionRegistry) {
	v, e := decode(p.ConfigJSON)
	if e != nil {
		p.Diagnostics = append(p.Diagnostics, Diagnostic{Code: "invalid_config_json", Source: p.Source, Path: "stellar_config", Message: "候选必须是单一 JSON 对象，保留原文", Blocking: true})
		return
	}
	if b, e := decode(p.BlogJSON); e != nil || b == nil {
		p.Diagnostics = append(p.Diagnostics, Diagnostic{Code: "invalid_blog_json", Source: p.Source, Path: "blog_extensions", Message: "扩展必须是 JSON 对象，保留原文", Blocking: true})
	} else if _, ok := b.(map[string]any); !ok {
		p.Diagnostics = append(p.Diagnostics, Diagnostic{Code: "invalid_blog_json", Source: p.Source, Path: "blog_extensions", Message: "扩展必须是 JSON 对象", Blocking: true})
	}
	var result stellarv2.Result
	var err error
	if isCollection(p.Table) {
		result, err = stellarv2.ParseCollection(v, stellarv2.Options{Source: p.Source})
		if err == nil {
			profile, profileErr := stellarv2.ValidateCollectionProfile(result.Value, p.Source, p.Profile)
			if profileErr != nil {
				err = profileErr
			} else {
				for _, issue := range profile.Issues {
					p.Diagnostics = append(p.Diagnostics, Diagnostic{Code: issue.Code, Source: issue.Source, Path: issue.Path, Message: issue.Expected, Blocking: true})
				}
			}
		}
	} else {
		result, err = stellarv2.ParsePage(v, stellarv2.Options{Source: p.Source})
		if err == nil {
			kind := "pages"
			if p.Table == "posts" || strings.HasPrefix(p.Source, "content/posts/") {
				kind = "posts"
			}
			m := stellarv2.ResolveContentMembership(stellarv2.MembershipInput{Kind: kind, Source: p.Source, PagePath: p.Path, Config: result.Value, Registry: registry})
			for _, issue := range m.Issues {
				p.Diagnostics = append(p.Diagnostics, Diagnostic{Code: issue.Code, Source: issue.Source, Path: issue.Path, Message: issue.Expected, Blocking: true})
			}
			profile := stellarv2.ValidatePageProfile(result.Value, p.Source, p.Profile)
			for _, issue := range profile.Issues {
				p.Diagnostics = append(p.Diagnostics, Diagnostic{Code: issue.Code, Source: issue.Source, Path: issue.Path, Message: issue.Expected, Blocking: true})
			}
		}
	}
	if err != nil {
		p.Diagnostics = append(p.Diagnostics, schemaDiagnostics(err)...)
	}
	for _, issue := range result.Issues {
		if issue.Severity == "warning" {
			p.Diagnostics = append(p.Diagnostics, Diagnostic{Code: issue.Code, Source: issue.Source, Path: issue.Path, Message: issue.Expected, Blocking: false})
		}
	}
}
func buildFile(src, path string, s Snapshot) RecordCandidate {
	raw, markdown, err := readLegacyDocument(src)
	b := newRecord("", Row{"id": path})
	b.candidate.Source = path
	b.candidate.ID = path
	b.candidate.SourceDigest = digest(src)
	b.candidate.Path = path
	b.candidate.Markdown = markdown
	b.candidate.Profile = "page"
	if strings.HasPrefix(path, "content/posts/") {
		b.candidate.Profile = "post"
		b.candidate.Path = "/post/" + strings.TrimSuffix(strings.TrimPrefix(path, "content/posts/"), ".md")
	} else {
		b.candidate.Path = "/" + strings.TrimSuffix(strings.TrimPrefix(path, "content/"), ".md")
	}
	b.raw = map[string]any{}
	b.blog = map[string]any{}
	if err != nil {
		b.conflict("invalid_front_matter", "front_matter", "源 front matter 无法解析或未关闭；保留完整原文")
		b.candidate.Markdown = src
		return b.finish()
	}
	for _, key := range sortedKeys(raw) {
		v := raw[key]
		switch key {
		case "blog":
			if m, ok := v.(map[string]any); ok {
				b.blog = m
			} else {
				b.conflict("invalid_blog", "blog", "扩展必须是对象")
			}
		case "topic":
			found := ""
			for _, t := range s.Tables["topics"].Rows {
				if stringValue(t["name"]) == stringValue(v) || stringValue(t["slug"]) == stringValue(v) {
					found = stringValue(t["slug"])
				}
			}
			if found == "" {
				b.conflict("topic_not_found", key, "文件旧专栏无定义，保持原值等待审核")
			} else {
				set(b.raw, "collection", map[string]any{"profile": "topic", "id": found})
				b.candidate.Profile = "topic"
			}
		case "wiki":
			set(b.raw, "collection", map[string]any{"profile": "wiki", "id": v})
			b.candidate.Profile = "wiki"
		case "pin", "pinned":
			set(b.raw, "listing.priority", numberBool(v))
		case "category":
			if _, exists := raw["categories"]; exists {
				b.conflict("category_columns_conflict", key, "同时声明旧 category 与 categories，不能静默选其中一项")
			} else {
				b.raw["categories"] = []any{v}
			}
		case "author":
			set(b.raw, "article.author", v)
		case "ai_label":
			set(b.raw, "article.ai_label", v)
		case "references", "license", "share":
			set(b.raw, "footer."+key, v)
		case "poster":
			set(b.blog, "content.poster", v)
			b.conflict("poster_consumer", key, "完整海报作为 Blog 扩展保留，尚需接通消费")
		case "poster_headline", "poster_topic", "poster_caption", "poster_color":
			set(b.blog, "content.poster."+strings.TrimPrefix(key, "poster_"), v)
			b.conflict("poster_consumer", key, "旧独立 poster 字段完整保留于 Blog 扩展，尚需接通消费")
		case "status":
			set(b.blog, "status", v)
			b.conflict("file_host_fields", key, "发布状态保留于 blog.status；须验证宿主导入保存该状态，不得把草稿隐式发布")
			b.mapped(key, "blog.status", "保留原发布状态，使用已定义的宿主 Markdown 导入契约；不进入上游 schema")
			continue
		case "password", "authors", "publish_at", "draft_json":
			set(b.blog, "host."+key, v)
			b.conflict("file_host_fields", key, "宿主权限/生命周期字段不能塞入上游；文件导入须另行接通显式宿主契约")
		default:
			b.raw[key] = v
		}
		b.mapped(key, "candidate.front_matter/blog", "每字段候选保留；unknown 原值仍保留并由严格 schema 报错")
	}
	b.candidate.Markdown, b.candidate.Diagnostics, b.candidate.Mappings = migrateMarkdown(markdown, path, b.candidate.Diagnostics, b.candidate.Mappings)
	p := b.finish()
	document, e := post.SerializeStellarDocument(post.StellarDocument{Raw: b.raw, Blog: b.blog, Body: p.Markdown})
	if e == nil {
		p.Document = string(document)
	} else {
		p.Diagnostics = append(p.Diagnostics, schemaDiagnostics(e)...)
	}
	return p
}
func readLegacyDocument(src string) (map[string]any, string, error) {
	raw := map[string]any{}
	first, rest, ok := strings.Cut(strings.TrimPrefix(src, "\ufeff"), "\n")
	delimiter := strings.TrimSuffix(first, "\r")
	if delimiter != "---" && delimiter != ";;;" {
		return raw, src, nil
	}
	if !ok {
		return raw, src, fmt.Errorf("unclosed front matter")
	}
	header := strings.Builder{}
	closed := false
	body := src
	for rest != "" {
		line, next, newline := strings.Cut(rest, "\n")
		if strings.TrimSuffix(line, "\r") == delimiter {
			closed = true
			body = next
			break
		}
		header.WriteString(line)
		if newline {
			header.WriteByte('\n')
		}
		rest = next
	}
	if !closed {
		return raw, src, fmt.Errorf("unclosed front matter")
	}
	if delimiter == ";;;" {
		v, err := decode(header.String())
		if err != nil {
			return raw, src, err
		}
		m, ok := v.(map[string]any)
		if !ok {
			return raw, src, fmt.Errorf("object required")
		}
		raw = m
	} else if err := yaml.Unmarshal([]byte(header.String()), &raw); err != nil {
		return raw, src, err
	}
	return clone(raw), body, nil
}
func validateAll(c Candidate) error {
	t, err := decode(c.Theme.ThemeJSON)
	if err != nil {
		return err
	}
	b, err := decode(c.Theme.BlogJSON)
	if err != nil {
		return err
	}
	if _, err = stellarv2.ParseTheme(t, stellarv2.Options{Source: "migration.theme"}); err != nil {
		return err
	}
	if _, ok := b.(map[string]any); !ok {
		return fmt.Errorf("blog config must be object")
	}
	collections := map[string]map[string]any{}
	seen := map[string]bool{}
	for _, p := range c.Records {
		if seen[p.Source] {
			return fmt.Errorf("duplicate candidate record")
		}
		seen[p.Source] = true
		if p.StellarVersion != 2 {
			return fmt.Errorf("candidate version must be 2")
		}
		found := false
		if p.Table != "" {
			for _, row := range c.Source.Tables[p.Table].Rows {
				if stringValue(row["id"]) == p.ID && digest(row) == p.SourceDigest {
					if isCollection(p.Table) {
						id := stringValue(row["id"])
						if p.Table == "topics" {
							id = stringValue(row["slug"])
						}
						if p.CollectionID != id {
							return fmt.Errorf("candidate collection identity mismatch")
						}
					}
					found = true
					break
				}
			}
		} else {
			source, ok := c.Source.Files[p.Source]
			found = ok && digest(source) == p.SourceDigest
		}
		if !found {
			return fmt.Errorf("candidate source record mismatch")
		}
		if isCollection(p.Table) {
			raw, e := decode(p.ConfigJSON)
			if e != nil {
				return e
			}
			parsed, e := stellarv2.ParseCollection(raw, stellarv2.Options{Source: p.Source})
			if e != nil {
				return e
			}
			collections[collectionKey(p)] = parsed.Value
		}
	}
	expected := len(c.Source.Files)
	for _, name := range []string{"posts", "pages", "wikidocs", "wiki_projects", "topics", "notebooks", "notes"} {
		expected += len(c.Source.Tables[name].Rows)
	}
	if expected != len(c.Records) {
		return fmt.Errorf("candidate record set incomplete")
	}
	registry := stellarv2.CreateCollectionRegistry(collections)
	for _, p := range c.Records {
		p.Diagnostics = nil
		validateRecord(&p, registry)
		for _, d := range p.Diagnostics {
			if d.Blocking {
				return fmt.Errorf("candidate strict validation failed: %s.%s", d.Source, d.Path)
			}
		}
	}
	return nil
}
