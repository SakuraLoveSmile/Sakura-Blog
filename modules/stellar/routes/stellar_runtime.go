package routes

import (
	"blog/internal/comments"
	"blog/internal/content"
	"blog/internal/settings"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"blog/internal/post"
	"blog/internal/stellarhtml"
	"blog/internal/stellarmodels"
	"blog/internal/stellarv2"
	"blog/internal/store"
	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"
)

type stellarRecord struct {
	kind, profile, collectionID, source, path, markdown string
	config                                              stellarmodels.Object
	page                                                stellarmodels.Object
	input                                               stellarmodels.Input
	model                                               stellarmodels.PageModel
	blogDefaults                                        stellarmodels.Object
	post                                                *store.Post
	pageRow                                             *store.Page
	doc                                                 *store.WikiDoc
	note                                                *store.Note
}
type stellarCollection struct {
	profile, id, source, path   string
	raw                         string
	config, state, model, index stellarmodels.Object
	blog                        stellarmodels.Object
	members                     []*stellarRecord
}
type stellarSnapshot struct {
	offline      bool
	site         *tmpl.Site
	inventory    *stellarInventory
	input        stellarmodels.Input
	registry     *stellarv2.CollectionRegistry
	collections  []*stellarCollection
	byCollection map[string]*stellarCollection
	records      []*stellarRecord
	byPath       map[string]*stellarRecord
}

func stellarGet(m map[string]any, path string) any { v, _ := stellarv2.Get(m, path); return v }
func stellarObject(v any) stellarmodels.Object {
	m, _ := v.(map[string]any)
	if m == nil {
		return stellarmodels.Object{}
	}
	return m
}
func stellarString(v any) string { s, _ := v.(string); return s }
func stellarArray(v any) []any {
	switch a := v.(type) {
	case []any:
		return a
	case []string:
		r := make([]any, len(a))
		for i, v := range a {
			r[i] = v
		}
		return r
	case []stellarmodels.Object:
		r := make([]any, len(a))
		for i, v := range a {
			r[i] = v
		}
		return r
	}
	return []any{}
}
func stellarNumber(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return 0
}
func stellarPath(v string) string { return "/" + strings.Trim(v, "/") }
func stellarPathKey(v string) string {
	if decoded, err := url.PathUnescape(v); err == nil {
		v = decoded
	}
	return strings.TrimSuffix(stellarPath(v), "/")
}
func stellarProfilePath(theme map[string]any, profile string) string {
	return stellarPath(stellarv2.ProfilePath(stellarGet(theme, "profiles."+profile+".path")))
}
func stellarTerms(v any) []string {
	out := []string{}
	if s, ok := v.(string); ok {
		return []string{s}
	}
	for _, item := range stellarArray(v) {
		if s, ok := item.(string); ok {
			out = append(out, s)
		} else if s := stellarString(stellarObject(item)["name"]); s != "" {
			out = append(out, s)
		}
	}
	return out
}
func stellarData(site *tmpl.Site) stellarmodels.Object {
	authors := stellarmodels.Object{}
	for id, a := range site.Authors {
		authors[id] = stellarmodels.Object{"name": a.Name, "url": a.URL, "avatar": a.Avatar, "description": a.Description, "path": strings.Trim(stellarProfilePath(site.StellarV2.Theme, "author"), "/") + "/" + url.PathEscape(id)}
	}
	data := stellarObject(site.StellarV2.Blog["data"])
	for id, raw := range stellarObject(data["authors"]) {
		a := stellarObject(raw)
		pub := stellarmodels.Object{}
		for _, key := range []string{"name", "url", "avatar", "description", "path"} {
			if value, ok := a[key].(string); ok {
				pub[key] = value
			}
		}
		if _, exists := pub["path"]; !exists {
			pub["path"] = strings.Trim(stellarProfilePath(site.StellarV2.Theme, "author"), "/") + "/" + url.PathEscape(id)
		}
		authors[id] = pub
	}
	widgets := stellarweb.MergeStellarWidgetCatalog(stellarObject(data["widgets"]))
	return stellarmodels.Object{"authors": authors, "defaultAuthor": data["defaultAuthor"], "widgets": widgets}
}
func stellarSiteInput(site *tmpl.Site) (stellarmodels.Input, error) {
	if site == nil || site.StellarV2 == nil || site.StellarV2.Theme == nil {
		return stellarmodels.Input{}, fmt.Errorf("%w: stellar theme", store.ErrStellarMigrationRequired)
	}
	host := stellarObject(site.StellarV2.Blog["site"])
	perPage := any(10)
	if value, ok := host["per_page"]; ok {
		perPage = value
	}
	language := host["language"]
	if language == nil {
		language = "zh-CN"
	}
	config := stellarmodels.Object{"title": site.Title, "url": tmpl.NormalizeSiteURL(site.URL), "avatar": site.Avatar, "language": language, "date_format": "YYYY-MM-DD", "per_page": perPage, "index_generator": stellarmodels.Object{"path": strings.Trim(stellarProfilePath(site.StellarV2.Theme, "blogIndex"), "/")}}
	for _, key := range []string{"author", "email", "keywords", "date_format"} {
		if value, ok := host[key]; ok {
			config[key] = value
		}
	}
	return stellarmodels.Input{Theme: site.StellarV2.Theme, ThemeSource: "stellar_v2.theme", SiteConfig: config, RuntimeData: stellarData(site)}, nil
}
func (s *webServer) stellarSnapshot(include *store.Post) (*stellarSnapshot, error) {
	if s.site == nil {
		return nil, store.ErrStellarMigrationRequired
	}
	return s.stellarSnapshotAt(context.Background(), s.site.Load(), include, false)
}
func (s *webServer) stellarSnapshotAt(ctx context.Context, site *tmpl.Site, include *store.Post, offline bool) (*stellarSnapshot, error) {
	in, err := stellarSiteInput(site)
	if err != nil {
		return nil, err
	}
	inventory, err := s.readStellarInventory(ctx, include)
	if err != nil {
		return nil, err
	}
	snap := &stellarSnapshot{offline: offline, site: site, input: in, inventory: inventory, byCollection: map[string]*stellarCollection{}, byPath: map[string]*stellarRecord{}}
	configs := map[string]map[string]any{}
	addCollection := func(profile, id, raw string, validated stellarv2.Result, err error) error {
		if err != nil {
			return err
		}
		source := profile + "/" + id
		if profile == "notebook" {
			source = "notebooks/" + id
		}
		c := &stellarCollection{profile: profile, id: id, source: source, config: validated.Value, raw: raw, state: stellarmodels.Object{}}
		snap.collections = append(snap.collections, c)
		snap.byCollection[profile+":"+id] = c
		configs[source] = validated.Value
		return nil
	}
	projects := inventory.projects
	visibleProjects := map[string]bool{}
	for _, p := range projects {
		if !p.Visible {
			continue
		}
		visibleProjects[p.ID] = true
		r, e := p.ValidateStellar()
		if err := addCollection("wiki", p.ID, p.StellarConfig, r, e); err != nil {
			return nil, err
		}
		snap.byCollection["wiki:"+p.ID].blog = stellarPublishedBlog(p.BlogExtensions)
	}
	topics := inventory.topics
	for _, t := range topics {
		r, e := t.ValidateStellar()
		if err := addCollection("topic", t.Slug, t.StellarConfig, r, e); err != nil {
			return nil, err
		}
		snap.byCollection["topic:"+t.Slug].blog = stellarPublishedBlog(t.BlogExtensions)
	}
	notebooks := inventory.notebooks
	for _, nb := range notebooks {
		r, e := nb.ValidateStellar()
		if err := addCollection("notebook", nb.ID, nb.StellarConfig, r, e); err != nil {
			return nil, err
		}
		snap.byCollection["notebook:"+nb.ID].blog = stellarPublishedBlog(nb.BlogExtensions)
	}
	snap.registry = stellarv2.CreateCollectionRegistry(configs)
	addRecord := func(rec *stellarRecord, result stellarv2.Result, err error) error {
		if err != nil {
			return err
		}
		resolved := stellarv2.ResolveContentMembership(stellarv2.MembershipInput{Kind: rec.kind, Source: rec.source, PagePath: rec.path, Config: result.Value, Registry: snap.registry})
		if resolved.Config == nil || len(resolved.Issues) > 0 {
			return &stellarv2.SchemaError{Issues: resolved.Issues}
		}
		rec.profile = stellarString(stellarGet(resolved.Config, "collection.profile"))
		rec.collectionID = stellarString(stellarGet(resolved.Config, "collection.id"))
		if rec.profile == "" {
			rec.profile = "post"
			if rec.kind == "pages" {
				rec.profile = "page"
			}
		}
		rec.config = stellarv2.ValidatePageProfile(resolved.Config, rec.source, rec.profile).Value
		if rec.pageRow != nil && rec.pageRow.Slug == FriendsSlug && rec.profile == "page" {
			rec.config, rec.blogDefaults = stellarFriendsDefaults(site, rec.config, stellarRecordBlog(rec))
		}
		if rec.profile != "post" && rec.profile != "page" {
			collection := snap.byCollection[rec.profile+":"+rec.collectionID]
			if collection == nil {
				return fmt.Errorf("missing public collection: %s", rec.source)
			}
			collection.members = append(collection.members, rec)
		}
		snap.records = append(snap.records, rec)
		return nil
	}
	posts := inventory.posts
	included := false
	for _, p := range posts {
		if include != nil && include.ID == 0 && p.Slug == include.Slug {
			continue
		}
		if p.IsPasswordProtected() && (include == nil || p.ID != include.ID) {
			continue
		}
		if include != nil && p.ID == include.ID {
			included = true
		}
		rec := &stellarRecord{kind: "posts", source: "_posts/" + p.Slug + ".md", path: "/post/" + url.PathEscape(p.Slug), markdown: p.ContentMD, post: p}
		r, e := p.ValidateStellar()
		if err := addRecord(rec, r, e); err != nil {
			return nil, err
		}
	}
	if include != nil && include.ID == 0 && !included && include.Status != store.StatusPublished && include.DeletedAt == "" {
		rec := &stellarRecord{kind: "posts", source: "_posts/" + include.Slug + ".md", path: "/post/" + url.PathEscape(include.Slug), markdown: include.ContentMD, post: include}
		r, e := include.ValidateStellar()
		if err := addRecord(rec, r, e); err != nil {
			return nil, err
		}
	}
	pages := inventory.pages
	for _, p := range pages {
		rec := &stellarRecord{kind: "pages", source: p.Slug + ".md", path: "/" + stellarEscapePath(p.Slug), markdown: p.ContentMD, pageRow: p}
		r, e := p.ValidateStellar()
		if err := addRecord(rec, r, e); err != nil {
			return nil, err
		}
	}
	for _, project := range projects {
		if !visibleProjects[project.ID] {
			continue
		}
		docs := inventory.docs[project.ID]
		for _, d := range docs {
			if d.Status != store.StatusPublished {
				continue
			}
			rec := &stellarRecord{kind: "pages", source: "wiki/" + d.ProjectID + "/" + d.Slug + ".md", path: "/wiki/" + d.ProjectID + "/" + d.Slug, markdown: d.ContentMD, doc: d}
			r, e := d.ValidateStellar()
			if err := addRecord(rec, r, e); err != nil {
				return nil, err
			}
		}
	}
	for _, nb := range notebooks {
		notes := inventory.notes[nb.ID]
		for _, n := range notes {
			rec := &stellarRecord{kind: "pages", source: "notebooks/" + n.NotebookID + "/" + n.Slug + ".md", path: "/notebooks/" + n.NotebookID + "/" + n.Slug, markdown: n.ContentMD, note: n}
			r, e := n.ValidateStellar()
			if err := addRecord(rec, r, e); err != nil {
				return nil, err
			}
		}
	}
	// 归属完成后生成路径，所有聚合及详情共用同一快照。
	for _, c := range snap.collections {
		c.path = collectionBasePath(in.Theme, c)
	}
	for _, rec := range snap.records {
		slug := rec.path
		if rec.doc != nil {
			slug = snap.byCollection["wiki:"+rec.collectionID].path + "/" + url.PathEscape(rec.doc.Slug)
		}
		if rec.note != nil {
			slug = snap.byCollection["notebook:"+rec.collectionID].path + "/" + url.PathEscape(rec.note.Slug)
		}
		if rec.pageRow != nil && rec.profile != "page" && rec.profile != "post" {
			slug = snap.byCollection[rec.profile+":"+rec.collectionID].path + "/" + url.PathEscape(rec.pageRow.Slug[strings.LastIndex(rec.pageRow.Slug, "/")+1:])
		}
		if permalink := stellarString(rec.config["permalink"]); permalink != "" {
			parsed, err := url.Parse(permalink)
			if err != nil {
				return nil, fmt.Errorf("invalid permalink for %s: %w", rec.source, err)
			}
			slug = parsed.EscapedPath()
		}
		rec.path = stellarPath(slug)
		if snap.byPath[stellarPathKey(rec.path)] != nil {
			return nil, fmt.Errorf("duplicate public content path %s", rec.path)
		}
		snap.byPath[stellarPathKey(rec.path)] = rec
	}
	for _, rec := range snap.records {
		rec.page = stellarHostPage(snap, rec)
	}
	snap.input.RuntimeData = s.stellarBodyData(snap)
	in.RuntimeData = snap.input.RuntimeData
	for _, rec := range snap.records {
		body, err := s.stellarBody(snap, rec)
		if err != nil {
			return nil, err
		}
		rec.page["content"] = body
		rec.page = stellarmodels.PageModelInput(rec.page, rec.config)
	}

	for _, c := range snap.collections {
		ci := in
		ci.CollectionID = c.id
		ci.CollectionSource = c.source
		ci.CollectionConfig = c.config
		for _, member := range c.members {
			ci.Members = append(ci.Members, stellarmodels.Member{Page: member.page, FrontMatter: member.config})
		}
		if c.profile == "wiki" {
			listed := hasTag(stellarTerms(stellarObject(site.StellarV2.Blog["data"])["wiki"]), c.id)
			ci.CollectionListed = &listed
			c.state = stellarWikiState(c)
			ci.CollectionState = c.state
			c.model, err = stellarmodels.BuildWikiCollection(ci)
		}
		if c.profile == "topic" {
			c.model, err = stellarmodels.BuildTopicCollection(ci)
		}
		if c.profile == "notebook" {
			for _, member := range c.members {
				ci.CollectionItems = append(ci.CollectionItems, stellarmodels.Object{"collection": stellarmodels.Object{"profile": "notebook", "id": c.id}, "tags": member.page["tags"]})
			}
			c.model, err = stellarmodels.BuildNotebookCollection(ci)
		}
		if err != nil {
			return nil, err
		}
		if c.profile == "wiki" {
			c.index = stellarmodels.BuildWikiListingRender(ci, c.model)
		}
		if c.profile == "topic" {
			ci.CollectionModel = c.model
			c.index, err = stellarmodels.BuildTopicIndexRender(ci)
			if err != nil {
				return nil, err
			}
		}
	}
	for _, rec := range snap.records {
		rec.input = in
		rec.input.Source = rec.source
		rec.input.Page = rec.page
		rec.input.FrontMatter = rec.config
		if c := snap.byCollection[rec.profile+":"+rec.collectionID]; c != nil {
			rec.input.CollectionID = c.id
			rec.input.CollectionSource = c.source
			rec.input.CollectionConfig = c.config
			rec.input.CollectionState = c.state
			rec.input.CollectionModel = c.model
			for _, member := range c.members {
				rec.input.Members = append(rec.input.Members, stellarmodels.Member{Page: member.page, FrontMatter: member.config})
			}
		}
		if rec.profile == "wiki" {
			for _, tag := range stellarTerms(snap.byCollection["wiki:"+rec.collectionID].config["tags"]) {
				related := []any{}
				for _, c := range snap.collections {
					if c.profile == "wiki" && c.id != rec.collectionID && stellarGet(c.model, "visibility.listed") != false && hasTag(stellarTerms(c.config["tags"]), tag) {
						related = append(related, c.model)
					}
				}
				if len(related) > 0 {
					rec.input.RelatedCollections = append(rec.input.RelatedCollections, stellarmodels.Object{"name": tag, "items": related})
				}
			}
		}

		rec.model, err = stellarmodels.BuildPage(rec.input)
		if err != nil {
			return nil, err
		}
	}
	// 全部模型先完成第一阶段，再生成相关文章和Notebook标签聚合，避免请求顺序改变结果。
	for _, c := range snap.collections {
		if c.profile == "notebook" {
			c.index = stellarNotebookIndex(c)
			c.state = stellarmodels.Object{"tags": c.index["tags"], "recentItems": c.index["recentItems"]}
		}
	}
	for _, rec := range snap.records {
		if rec.profile == "notebook" {
			c := snap.byCollection["notebook:"+rec.collectionID]
			for _, v := range stellarArray(c.state["tags"]) {
				rec.input.TagTree = append(rec.input.TagTree, stellarObject(v))
			}
			for _, v := range stellarArray(c.state["recentItems"]) {
				rec.input.RecentItems = append(rec.input.RecentItems, stellarObject(v))
			}
		}
		if rec.profile == "post" || rec.profile == "topic" {
			rec.input = stellarPostNavigation(snap, rec)
		}
		rec.model, err = stellarmodels.BuildPage(rec.input)
		if err != nil {
			return nil, err
		}
	}
	stellarPopulatePublicData(snap)
	return snap, nil
}

func collectionBasePath(theme map[string]any, c *stellarCollection) string {
	if p := stellarString(stellarGet(c.config, "route.path")); p != "" {
		return stellarPath(p)
	}
	profile := map[string]string{"wiki": "wikiIndex", "topic": "topic", "notebook": "notebooks"}[c.profile]
	return strings.TrimSuffix(stellarProfilePath(theme, profile), "/") + "/" + url.PathEscape(c.id)
}
func stellarHostPage(snap *stellarSnapshot, r *stellarRecord) stellarmodels.Object {
	p := stellarmodels.Object{"_id": r.source, "source": r.source, "path": strings.TrimLeft(r.path, "/"), "permalink": tmpl.JoinSiteURL(snap.site.URL, r.path), "layout": "post", "tags": []any{}, "categories": []any{}}
	if r.post != nil {
		row := r.post
		p["title"] = row.Title
		p["date"] = row.CreatedAt
		p["updated"] = row.UpdatedAt
		p["excerpt"] = row.Excerpt
		p["tags"] = tmpl.ParseTags(row.Tags)
		p["categories"] = row.Categories
	}
	if r.pageRow != nil {
		row := r.pageRow
		p["title"] = row.Title
		p["date"] = row.CreatedAt
		p["updated"] = row.UpdatedAt
		p["layout"] = "page"
	}
	if r.doc != nil {
		row := r.doc
		p["title"] = row.Title
		p["date"] = row.CreatedAt
		p["updated"] = row.UpdatedAt
		p["layout"] = "wiki"
	}
	if r.note != nil {
		row := r.note
		p["title"] = row.Title
		p["date"] = row.CreatedAt
		p["updated"] = row.UpdatedAt
		p["layout"] = "page"
		p["tags"] = row.Tags
	}
	for _, key := range []string{"title", "date", "updated", "excerpt", "layout", "link", "lang", "language", "tags", "categories"} {
		if value, exists := r.config[key]; exists {
			p[key] = value
		}
	}
	for _, field := range []string{"tags", "categories"} {
		links := []any{}
		for _, name := range stellarTerms(p[field]) {
			links = append(links, stellarmodels.Object{"name": name, "path": strings.Trim(stellarTaxonomyPath(snap.input.Theme, field), "/") + "/" + stellarEscapePath(name)})
		}
		p[field] = links
	}
	return p
}

// 正文、组件及页脚共用同一公开 inventory 快照；不能传整个 Blog 配置袋。
func (s *webServer) stellarBodyData(snap *stellarSnapshot) stellarmodels.Object {
	data := stellarCopyObject(snap.input.RuntimeData)
	data["siteURL"] = tmpl.NormalizeSiteURL(snap.site.URL)
	data["links"] = stellarObject(snap.site.StellarV2.Blog["data"])["links"]
	icons := map[string]any{}
	if s.icons != nil {
		for _, key := range s.icons.Keys() {
			icons[key] = s.icons.Get(key)
		}
	}
	data["icons"] = icons
	tree := stellarmodels.Object{}
	for _, c := range snap.collections {
		if c.profile == "wiki" {
			state := stellarWikiState(c)
			pages := []any{}
			for _, section := range stellarArray(state["sections"]) {
				pages = append(pages, stellarArray(stellarObject(section)["pages"])...)
			}
			state["pages"] = pages
			tree[c.id] = state
		}
	}
	data["wiki"] = stellarmodels.Object{"tree": tree}
	return data
}
func (s *webServer) stellarBody(snap *stellarSnapshot, r *stellarRecord) (string, error) {
	if s.posts == nil {
		return "", fmt.Errorf("stellar renderer unavailable")
	}
	data := stellarCopyObject(snap.input.RuntimeData)
	if r.pageRow != nil && r.pageRow.Slug == FriendsSlug && r.profile == "page" {
		data["friendsShowPosts"] = snap.site.Pages.Friends.ShowPostsOn()
	}
	body, err := s.posts.RenderStellarV2WithContext(r.markdown, post.StellarRenderContext{Theme: snap.input.Theme, Data: data, Page: r.config, Language: stellarLanguage(snap.input, r.config), BlogExtensions: true, Offline: snap.offline})
	if err != nil {
		return "", err
	}
	icon := func(key string) string {
		if s.icons == nil {
			return ""
		}
		return s.icons.Get(key)
	}
	return stellarhtml.ProcessContent(body, stellarhtml.Context{Theme: snap.input.Theme, SiteURL: tmpl.NormalizeSiteURL(snap.site.URL), Icon: icon, IconData: icon, Language: stellarLanguage(snap.input, r.config)})
}
func stellarLanguage(in stellarmodels.Input, fm map[string]any) string {
	return content.ResolveStellarLanguage(fm, map[string]any{"site": map[string]any{"language": in.SiteConfig["language"]}})
}

func stellarWikiState(c *stellarCollection) stellarmodels.Object {
	byPath := map[string]*stellarRecord{}
	for _, r := range c.members {
		byPath[stellarv2.NormalizeMembershipPath(r.path)] = r
	}
	tree := stellarGet(c.config, "navigation.tree")
	groups, grouped := tree.(map[string]any)
	keys := stellarWikiSectionKeys(c.raw)
	if !grouped {
		groups = map[string]any{"": tree}
		keys = []string{""}
	}
	if grouped && len(keys) == 0 {
		for key := range groups {
			keys = append(keys, key)
		}
		sort.Strings(keys)
	}
	base := stellarString(stellarGet(c.config, "route.path"))
	base = strings.TrimPrefix(base, "/")
	if len(base) > 1 && !strings.HasSuffix(base, "/") {
		base += "/"
	}
	homepage := ""
	for _, title := range keys {
		for _, path := range stellarTerms(groups[title]) {
			if r := byPath[stellarv2.NormalizeMembershipPath(base+path)]; r != nil {
				homepage = strings.TrimPrefix(r.path, "/")
				break
			}
		}
		if homepage != "" {
			break
		}
	}
	if homepage == "" && len(c.members) > 0 {
		homepage = strings.TrimPrefix(c.members[0].path, "/")
	}
	sections := []any{}
	assigned := map[string]bool{}
	number := 0
	page := func(r *stellarRecord) any {
		value := stellarmodels.Object{"_id": r.source, "id": r.source, "title": r.page["title"], "path": strings.TrimPrefix(r.path, "/"), "page_number": number, "is_homepage": strings.TrimPrefix(r.path, "/") == homepage}
		number++
		return value
	}
	if tree != nil {
		for _, title := range keys {
			pages := []any{}
			for _, path := range stellarTerms(groups[title]) {
				if r := byPath[stellarv2.NormalizeMembershipPath(base+path)]; r != nil && stellarGet(r.config, "visibility.listed") != false {
					pages = append(pages, page(r))
					assigned[r.source] = true
				}
			}
			sections = append(sections, stellarmodels.Object{"title": title, "pages": pages})
		}
	}
	others := []any{}
	for _, r := range c.members {
		if !assigned[r.source] && stellarGet(r.config, "visibility.listed") != false {
			others = append(others, page(r))
		}
	}
	if tree == nil {
		sections = append(sections, stellarmodels.Object{"pages": others})
	} else if len(others) > 0 {
		sections = append(sections, stellarmodels.Object{"title": "...", "pages": others})
	}
	return stellarmodels.Object{"homepage": stellarmodels.Object{"path": homepage}, "sections": sections}
}
func stellarWikiSectionKeys(raw string) []string {
	root := map[string]json.RawMessage{}
	if json.Unmarshal([]byte(raw), &root) != nil {
		return nil
	}
	navigation := map[string]json.RawMessage{}
	if json.Unmarshal(root["navigation"], &navigation) != nil {
		return nil
	}
	decoder := json.NewDecoder(strings.NewReader(string(navigation["tree"])))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil
	}
	keys := []string{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil
		}
		name, ok := key.(string)
		if !ok {
			return nil
		}
		keys = append(keys, name)
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return nil
		}
	}
	return keys
}
func stellarPostNavigation(snap *stellarSnapshot, r *stellarRecord) stellarmodels.Input {
	in := r.input
	members := []*stellarRecord{}
	for _, other := range snap.records {
		if other.post == nil || other.post.IsPasswordProtected() || stellarGet(other.model.Item, "visibility.listed") == false {
			continue
		}
		if r.profile == "topic" && (other.profile != "topic" || other.collectionID != r.collectionID) {
			continue
		}
		members = append(members, other)
	}
	sort.SliceStable(members, func(i, j int) bool {
		return stellarString(members[i].model.Item["date"]) > stellarString(members[j].model.Item["date"])
	})
	page := stellarmodels.Object{}
	for key, value := range in.Page {
		page[key] = value
	}
	in.Page = page
	for i, m := range members {
		if m == r {
			if i > 0 {
				in.Page["previous"] = stellarNavLink(members[i-1])
			}
			if i+1 < len(members) {
				in.Page["next"] = stellarNavLink(members[i+1])
			}
			break
		}
	}
	limit := int(stellarNumber(stellarGet(in.Theme, "article.relatedPostsLimit")))
	related := []*stellarRecord{}
	tags := map[string]bool{}
	for _, tag := range stellarTerms(r.model.Item["tags"]) {
		tags[tag] = true
	}
	score := func(other *stellarRecord) int {
		n := 0
		for _, tag := range stellarTerms(other.model.Item["tags"]) {
			if tags[tag] {
				n++
			}
		}
		return n
	}
	for _, other := range members {
		if other != r && score(other) > 0 {
			related = append(related, other)
		}
	}
	sort.SliceStable(related, func(i, j int) bool { return score(related[i]) > score(related[j]) })
	for _, other := range related {
		if len(in.RelatedItems) >= limit {
			break
		}
		in.RelatedItems = append(in.RelatedItems, stellarmodels.Object{"title": other.model.Item["title"], "path": strings.TrimPrefix(other.path, "/"), "excerpt": other.model.Item["excerpt"]})
	}
	return in
}
func stellarNavLink(r *stellarRecord) stellarmodels.Object {
	return stellarmodels.Object{"title": r.model.Item["title"], "path": strings.TrimPrefix(r.path, "/"), "date": r.model.Item["date"]}
}
func stellarNotebookIndex(c *stellarCollection) stellarmodels.Object {
	tags := []any{stellarmodels.Object{"id": "", "name": "", "label": "", "path": strings.TrimPrefix(c.path, "/"), "parentId": nil, "children": []any{}, "itemIds": []any{}}}
	for _, v := range stellarArray(stellarGet(c.model, "navigation.tags")) {
		copy := stellarmodels.Object{}
		for key, value := range stellarObject(v) {
			copy[key] = value
		}
		copy["children"] = []any{}
		copy["itemIds"] = []any{}
		tags = append(tags, copy)
	}
	items := []any{}
	recent := []any{}
	for _, r := range c.members {
		item := stellarObject(r.model.Render["listing"])
		items = append(items, item)
		if item["listed"] != false {
			recent = append(recent, item)
		}
		names := stellarTerms(r.page["tags"])
		for _, v := range tags {
			tag := stellarObject(v)
			id := stellarString(tag["id"])
			match := id == ""
			for _, name := range names {
				parts := []string{}
				for _, part := range strings.Split(name, "/") {
					if part != "" {
						parts = append(parts, part)
					}
				}
				name = strings.ToLower(strings.Join(parts, "/"))
				if name == id || strings.HasPrefix(name, id+"/") {
					match = true
				}
			}
			if match {
				tag["itemIds"] = append(stellarArray(tag["itemIds"]), r.source)
			}
		}
	}
	for _, v := range tags {
		tag := stellarObject(v)
		for _, child := range tags {
			ch := stellarObject(child)
			parent := stellarString(ch["parentId"])
			if stellarString(ch["id"]) != "" && parent == tag["id"] {
				tag["children"] = append(stellarArray(tag["children"]), ch["id"])
			}
		}
	}
	sort.SliceStable(recent, func(i, j int) bool {
		return stellarString(stellarObject(recent[i])["updated"]) > stellarString(stellarObject(recent[j])["updated"])
	})
	return stellarmodels.Object{"id": c.id, "href": strings.TrimPrefix(c.path, "/"), "name": stellarGet(c.model, "identity.name"), "headline": c.config["headline"], "caption": stellarGet(c.model, "identity.tagline"), "description": stellarGet(c.model, "identity.description"), "icon": stellarGet(c.model, "identity.icon"), "cover": c.model["cover"], "order": stellarGet(c.model, "listing.order"), "listed": stellarGet(c.model, "visibility.listed") != false, "navigation": stellarmodels.Object{"menu": c.config["activeMenu"]}, "layout": stellarmodels.Object{"topbar": c.config["topbar"], "leftbar": c.config["leftbar"], "rightbar": c.config["rightbar"], "algoliaFilterPath": strings.TrimPrefix(c.path, "/")}, "tags": tags, "items": items, "recentItems": recent, "perPage": stellarGet(c.model, "listing.perPage")}
}
func (s *webServer) stellarView(snap *stellarSnapshot, r *stellarRecord) *tmpl.ViewData {
	layout := map[string]string{"post": "post", "page": "page", "topic": "post", "wiki": "wiki-doc", "notebook": "note"}[r.profile]
	data := &tmpl.ViewData{Site: snap.site, Page: &tmpl.Page{Title: stellarString(r.model.Item["title"]), Slug: stellarRecordSlug(r), Path: r.path, Layout: layout, Lang: stellarLanguage(r.input, r.config)}, Content: template.HTML(stellarString(r.model.Item["content"])), Stellar: &settings.StellarRenderContext{Version: 2, Profile: r.profile, Input: r.input, NavigationMembers: map[string]string{}}}
	for _, other := range snap.records {
		if other.profile == r.profile && other.collectionID == r.collectionID && !((other.post != nil) && other.post.IsPasswordProtected()) {
			membership, _ := json.Marshal([]string{stellarString(other.model.Collection["profile"]), stellarString(other.model.Collection["id"])})
			data.Stellar.NavigationMembers[stellarPublicPath(snap.site, other.path)] = string(membership)
		}
	}
	data.Page.Author = stellarString(stellarGet(r.config, "article.author"))
	data.Page.Date = stellarString(r.model.Item["date"])
	data.Page.Updated = stellarString(r.model.Item["updated"])
	data.CommentPageKey = stellarPublicPath(snap.site, r.path)
	s.fillStellarNativeComments(data, snap, r)
	data.WordCount, data.ReadingMinutes = tmpl.ReadingStats(string(data.Content))
	if r.post != nil {
		data.Page.Views = r.post.Views
		data.Page.AuthorLinks = stellarRecordAuthors(snap, r)
		data.Page.AISummary = stellarString(stellarBlogContent(stellarRecordBlogRaw(r))["ai_summary"])
	}
	return data
}
func stellarPageNumber(raw string) (int, error) {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid page number")
	}
	return n, nil
}

func stellarRecordSlug(r *stellarRecord) string {
	if r.post != nil {
		return r.post.Slug
	}
	if r.pageRow != nil {
		return r.pageRow.Slug
	}
	if r.doc != nil {
		return r.doc.Slug
	}
	if r.note != nil {
		return r.note.Slug
	}
	return strings.Trim(r.path, "/")
}
func stellarPublicPath(site *tmpl.Site, path string) string {
	parsed, err := url.Parse(tmpl.JoinSiteURL(site.URL, path))
	if err != nil {
		return path
	}
	return parsed.Path
}
func (s *webServer) fillStellarNativeComments(data *tmpl.ViewData, snap *stellarSnapshot, r *stellarRecord) {
	raw := stellarObject(snap.site.StellarV2.Blog["native_comments"])
	pub := stellarmodels.Object{"enabled": raw["enabled"] == true, "title": raw["title"], "admin_badge": raw["admin_badge"], "gravatar_mirror": raw["gravatar_mirror"]}
	data.Stellar.Input.RuntimeData = stellarCopyObject(data.Stellar.Input.RuntimeData)
	data.Stellar.Input.RuntimeData["nativeComments"] = pub
	enabled := raw["enabled"] == true
	if r != nil {
		extensions := stellarmodels.Object{}
		var text string
		if r.post != nil {
			text = r.post.BlogExtensions
		}
		if r.pageRow != nil {
			text = r.pageRow.BlogExtensions
		}
		if r.doc != nil {
			text = r.doc.BlogExtensions
		}
		if r.note != nil {
			text = r.note.BlogExtensions
		}
		decoder := json.NewDecoder(strings.NewReader(text))
		decoder.UseNumber()
		_ = decoder.Decode(&extensions)
		if value, ok := stellarGet(extensions, "native_comments.enabled").(bool); ok {
			enabled = value
		}
	}
	if r == nil {
		enabled = stellarGet(raw, "home.enabled") == true
	}
	pub["enabled"] = enabled
	if !enabled {
		return
	}
	cfg := tmpl.CommentsConfig{Enabled: true, Title: stellarString(raw["title"]), AdminBadge: stellarString(raw["admin_badge"]), GravatarMirror: stellarString(raw["gravatar_mirror"])}
	list := snap.inventory.comments[data.CommentPageKey]
	boosts := snap.inventory.boosts[data.CommentPageKey]
	data.Comments = comments.ToViewItemsWithOptions(list, &cfg, comments.BoostViewOptions{Boosts: boosts})
	data.CommentCount = len(list)
}

func stellarPopulatePublicData(snap *stellarSnapshot) {
	data := stellarCopyObject(snap.input.RuntimeData)
	itemBlogs, collectionBlogs := stellarmodels.Object{}, stellarmodels.Object{}
	for _, r := range snap.records {
		itemBlogs[stellarString(r.model.Item["id"])] = stellarRecordBlog(r)
	}
	for _, c := range snap.collections {
		collectionBlogs[c.profile+"/"+c.id] = c.blog
	}
	data["itemBlogs"], data["collectionBlogs"] = itemBlogs, collectionBlogs
	counts := map[string]int{}
	seenCategories := map[string]bool{}
	recent := []any{}
	hasTopics := false
	hasPosts := false
	for _, r := range snap.records {
		if r.post == nil || stellarGet(r.model.Item, "visibility.listed") == false {
			continue
		}
		hasPosts = true
		if r.profile == "topic" {
			hasTopics = true
		}
		tagSet := map[string]bool{}
		for _, tag := range stellarTerms(r.model.Item["tags"]) {
			if !tagSet[tag] {
				counts[tag]++
				tagSet[tag] = true
			}
		}
		for _, cat := range stellarTerms(r.model.Item["categories"]) {
			seenCategories[cat] = true
		}
		recent = append(recent, r.model.Render["listing"])
	}
	sort.SliceStable(recent, func(i, j int) bool {
		return stellarString(stellarObject(recent[i])["date"]) > stellarString(stellarObject(recent[j])["date"])
	})
	names := []string{}
	for tag := range counts {
		names = append(names, tag)
	}
	sort.Strings(names)
	tags := []any{}
	base := stellarProfilePath(snap.input.Theme, "blogIndex")
	for _, tag := range names {
		tags = append(tags, stellarmodels.Object{"name": tag, "count": counts[tag], "path": strings.TrimPrefix(stellarTaxonomyPath(snap.input.Theme, "tags"), "/") + "/" + url.PathEscape(tag)})
	}
	data["tags"] = tags
	data["recentItems"] = recent
	data["hasTags"] = len(tags) > 0
	data["hasCategories"] = len(seenCategories) > 0
	data["hasTopics"] = hasTopics
	data["hasPosts"] = hasPosts
	data["blogPath"] = strings.TrimPrefix(base, "/")
	data["categoriesPath"] = strings.TrimPrefix(stellarTaxonomyPath(snap.input.Theme, "categories"), "/")
	data["tagsPath"] = strings.TrimPrefix(stellarTaxonomyPath(snap.input.Theme, "tags"), "/")
	data["archivesPath"] = strings.TrimPrefix(stellarTaxonomyPath(snap.input.Theme, "archives"), "/")
	snap.input.RuntimeData = data
	for _, r := range snap.records {
		r.input.RuntimeData = stellarCopyObject(data)
		if r.profile == "notebook" {
			r.input.RuntimeData = stellarCopyObject(data)
			c := snap.byCollection["notebook:"+r.collectionID]
			r.input.RuntimeData["tagTree"] = c.index["tags"]
			r.input.RuntimeData["recentItems"] = c.index["recentItems"]
			r.input.RuntimeData["notebookId"] = c.id
			r.input.RuntimeData["notebookName"] = stellarGet(c.model, "identity.name")
		} else if r.profile == "wiki" {
			r.input.RuntimeData = stellarCopyObject(data)
			c := snap.byCollection["wiki:"+r.collectionID]
			items := []any{}
			for _, m := range c.members {
				if stellarGet(m.model.Item, "visibility.listed") != false {
					items = append(items, stellarmodels.Object{"title": m.model.Item["title"], "href": strings.TrimPrefix(m.path, "/"), "date": m.model.Item["updated"]})
				}
			}
			sort.SliceStable(items, func(i, j int) bool {
				return stellarString(stellarObject(items[i])["date"]) > stellarString(stellarObject(items[j])["date"])
			})
			r.input.RuntimeData["recentItems"] = items
		}
		r.input.RuntimeData["recordBlog"] = stellarRecordBlog(r)
	}
}

func stellarEscapePath(raw string) string {
	parts := strings.Split(raw, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func stellarBlogContent(raw string) stellarmodels.Object {
	var extension map[string]any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&extension); err != nil {
		return stellarmodels.Object{}
	}
	return stellarObject(extension["content"])
}
func stellarPublishedBlog(raw string) stellarmodels.Object {
	content := stellarBlogContent(raw)
	pub := stellarmodels.Object{}
	for _, key := range []string{"banner_image", "wiki_external_url"} {
		if value, ok := content[key].(string); ok {
			pub[key] = value
		}
	}
	if value, exists := content["collection_order"]; exists {
		switch value.(type) {
		case json.Number, float64, int, int64:
			pub["collection_order"] = value
		}
	}
	if value, exists := content["authors"]; exists {
		pub["authors"] = stellarTerms(value)
	}
	if value, exists := content["poster"]; exists {
		poster := stellarObject(value)
		clean := stellarmodels.Object{}
		for _, key := range []string{"headline", "topic", "caption", "color"} {
			if value, ok := poster[key].(string); ok {
				clean[key] = value
			}
		}
		pub["poster"] = clean
	}
	return stellarmodels.Object{"content": pub}
}
func stellarRecordBlogRaw(r *stellarRecord) string {
	raw := ""
	switch {
	case r.post != nil:
		raw = r.post.BlogExtensions
	case r.pageRow != nil:
		raw = r.pageRow.BlogExtensions
	case r.doc != nil:
		raw = r.doc.BlogExtensions
	case r.note != nil:
		raw = r.note.BlogExtensions
	}
	return raw
}
func stellarRecordBlog(r *stellarRecord) stellarmodels.Object {
	blog := stellarPublishedBlog(stellarRecordBlogRaw(r))
	content := stellarObject(blog["content"])
	for key, value := range r.blogDefaults {
		if _, exists := content[key]; !exists {
			content[key] = value
		}
	}
	return blog
}
func stellarRecordAuthors(snap *stellarSnapshot, r *stellarRecord) []tmpl.AuthorLink {
	ids := stellarTerms(stellarGet(stellarRecordBlog(r), "content.authors"))
	if len(ids) == 0 {
		id := stellarString(stellarGet(r.config, "article.author"))
		if id == "" {
			id = stellarString(snap.input.RuntimeData["defaultAuthor"])
		}
		if id != "" {
			ids = []string{id}
		}
	}
	authors := stellarObject(snap.input.RuntimeData["authors"])
	out := []tmpl.AuthorLink{}
	seen := map[string]bool{}
	for _, id := range ids {
		author := stellarObject(authors[id])
		if seen[id] || stellarString(author["name"]) == "" {
			continue
		}
		seen[id] = true
		out = append(out, tmpl.AuthorLink{ID: id, Name: stellarString(author["name"]), Avatar: stellarString(author["avatar"]), Description: stellarString(author["description"]), URL: stellarString(author["url"])})
	}
	return out
}
