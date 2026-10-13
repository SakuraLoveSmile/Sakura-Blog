package web

import (
	_ "embed"
	"fmt"
	"html/template"
	"net/url"
	"strings"

	"blog/internal/settings"
	"blog/internal/stellarhtml"
	"blog/internal/stellarmodels"
	"blog/internal/stellarv2"
	"blog/modules/stellar/appearance"
	"blog/modules/stellar/clientconfig"
	"github.com/goccy/go-yaml"
)

//go:embed stellar_widgets.yml
var stellarWidgetsYAML []byte

var stellarDefaultWidgets = func() map[string]any {
	var catalog map[string]any
	if err := yaml.Unmarshal(stellarWidgetsYAML, &catalog); err != nil {
		panic(err)
	}
	return catalog
}()

// PrepareStellar 只装配严格主题快照；内容归属与授权已在公开路由处理。
// 每次请求复制输入，不能修改 Manager 发布的主题、模型或原始保存值。
func PrepareStellar(data *settings.ViewData) (*StellarRenderData, error) {
	if data == nil || data.Site == nil || data.Site.StellarV2 == nil {
		return nil, fmt.Errorf("stellar v2 migration required: site")
	}
	if data.Page == nil {
		return nil, fmt.Errorf("stellar v2: missing page")
	}
	theme := prepCloneMap(data.Site.StellarV2.Theme)
	base, err := url.Parse(data.Site.URL)
	if err != nil || base.Host == "" || (base.Scheme != "http" && base.Scheme != "https") {
		return nil, fmt.Errorf("stellar v2: absolute HTTP(S) site URL required")
	}
	root := strings.TrimRight(base.EscapedPath(), "/") + "/"
	rootURL := *base
	rootURL.Path = strings.TrimRight(base.Path, "/") + "/"
	rootURL.RawPath, rootURL.RawQuery, rootURL.Fragment = "", "", ""
	cssURL := rootURL.ResolveReference(&url.URL{Path: "assets/css/main.css"}).String()
	if err := prepAppearanceURLs(theme, rootURL.String(), cssURL); err != nil {
		return nil, err
	}
	p := &StellarPresentation{
		Theme: theme, Root: root, Layout: data.Page.Layout, Language: data.Page.Lang,
		ColorScheme: prepString(prepGet(theme, "appearance.colorScheme")),
		Widgets:     map[string][]StellarWidget{}, RegionBodies: map[string]template.HTML{},
	}
	input := stellarmodels.Input{Theme: theme, SiteConfig: map[string]any{
		"title": data.Site.Title, "subtitle": data.Site.Subtitle, "url": rootURL.String(),
		"avatar": data.Site.Avatar, "language": p.Language,
	}, Page: map[string]any{}, FrontMatter: map[string]any{}, RuntimeData: map[string]any{}}
	if data.Stellar != nil {
		if data.Stellar.Version != 2 {
			return nil, fmt.Errorf("stellar v2 migration required: content")
		}
		input = data.Stellar.Input
		input.Theme = theme
		input.SiteConfig = prepCloneMap(input.SiteConfig)
		input.Page = prepCloneMap(input.Page)
		input.RuntimeData = prepCloneMap(input.RuntimeData)
		input.FrontMatter = prepCloneMap(input.FrontMatter)
		p.Profile = data.Stellar.Profile
		for _, item := range data.Stellar.ListItems {
			p.ListItems = append(p.ListItems, map[string]any{"collection": item.Collection, "item": item.Item, "render": item.Render})
		}
		p.Collections = data.Stellar.Collections
	} else if prepDetailLayout(p.Layout) {
		return nil, fmt.Errorf("stellar v2 migration required: %s", p.Layout)
	}
	if p.Profile == "" {
		p.Profile = prepProfile(p.Layout)
	}
	if p.Language == "" {
		p.Language = "zh-CN"
	}
	if input.SiteConfig["language"] == nil {
		input.SiteConfig["language"] = p.Language
	}
	if input.SiteConfig["url"] == nil {
		input.SiteConfig["url"] = rootURL.String()
	}
	if input.SiteConfig["title"] == nil {
		input.SiteConfig["title"] = data.Site.Title
	}
	publicData := prepCloneMap(prepMap(data.Site.StellarV2.Blog["data"]))
	for key, value := range input.RuntimeData {
		publicData[key] = prepClone(value)
	}
	catalog := prepCloneMap(stellarDefaultWidgets)
	for key, value := range prepMap(publicData["widgets"]) {
		catalog[key] = prepClone(value)
	}
	publicData["widgets"] = catalog
	publicData["page"] = input.FrontMatter
	publicData["headEnd"] = prepInject(theme, input.FrontMatter, "headEnd")
	if data.Stellar != nil {
		pinned := []any{}
		for _, item := range data.Stellar.PinnedItems {
			pinned = append(pinned, map[string]any{"collection": item.Collection, "item": item.Item, "render": item.Render})
		}
		publicData["pinnedItems"] = pinned
		publicData["index"] = data.Stellar.Index
	}
	input.RuntimeData = publicData
	p.Data = publicData
	profile := prepMap(prepGet(theme, "profiles."+p.Profile))
	for key, path := range map[string]string{"blogPath": "profiles.blogIndex.path", "categoriesPath": "profiles.blogIndex.categories.path", "tagsPath": "profiles.blogIndex.tags.path", "archivesPath": "profiles.blogIndex.archives.path"} {
		if _, exists := publicData[key]; !exists {
			publicData[key] = root + stellarv2.ProfilePath(prepGet(theme, path))
		}
	}
	p.Navigation = stellarv2.ToRenderNavigation(profile)
	p.SettingsPath = root + stellarv2.ProfilePath(prepGet(theme, "profiles.settings.path"))
	p.PageType = "index"
	if prepContentLayout(p.Layout) {
		p.PageType = "content"
	}
	p.ArticleStyle = prepString(prepGet(theme, "article.style"))
	if style := prepString(prepGet(input.FrontMatter, "article.style")); style != "" {
		p.ArticleStyle = style
	}
	if p.PageType == "index" {
		p.ArticleStyle = ""
	}
	indent := prepString(prepGet(input.FrontMatter, "article.paragraphIndent"))
	if indent == "" {
		indent = prepString(prepGet(theme, "article.paragraphIndent"))
	}
	p.Indent = indent == "always" || (indent != "never" && p.ArticleStyle == "story")
	var collection map[string]any
	if prepDetailLayout(p.Layout) && (p.Layout != "page" || prepGet(input.FrontMatter, "collection.profile") != nil) {
		model, err := stellarmodels.BuildPage(input)
		if err != nil {
			return nil, err
		}
		p.Model = &model
		collection = model.Collection
		p.Profile = prepString(collection["profile"])
		layout := prepMap(model.Render["layout"])
		p.Regions = map[string]any{"topbar": layout["topbar"], "leftbar": layout["leftbar"], "rightbar": layout["rightbar"]}
		p.PageType, p.ArticleStyle, p.Indent = prepString(layout["pageType"]), prepString(layout["articleStyle"]), layout["indent"] == true
		p.Layout = prepString(model.Item["layout"])
		p.Cover = prepMap(model.Render["cover"])
		p.Navigation = prepMap(model.Item["navigation"])
		document := prepMap(model.Render["document"])
		if language := prepString(document["language"]); language != "" {
			p.Language = language
		}
	} else {
		var collectionRegions map[string]any
		if data.Stellar != nil {
			collectionRegions = prepMap(data.Stellar.Index["layout"])
		}
		p.Regions = stellarv2.ResolveRegions(stellarv2.RegionOptions{
			Profile: p.Profile, DefaultState: prepString(prepGet(theme, "leftbar.defaultState")),
			Catalog: catalog, Layers: []map[string]any{theme, profile, collectionRegions, input.FrontMatter},
		})
	}
	prepBrandTokens(p.Regions, input.SiteConfig)
	p.MobileBrand = stellarv2.ShouldShowMobileBrand(p.Profile)
	p.MobileBrandModel = prepMap(prepGet(p.Regions, "topbar.brand"))
	brand := prepMap(prepGet(p.Regions, "leftbar.brand"))
	p.SearchEnabled = prepGet(p.Regions, "leftbar.enabled") != false && brand["search"] == true && (prepString(brand["name"]) != "" || prepString(prepGet(brand, "image.src")) != "") && prepString(prepGet(theme, "search.provider")) != ""
	prepPrepareWidgets(data, p)
	pageURL := prepString(input.Page["permalink"])
	if pageURL == "" {
		pageURL = data.Page.CanonicalURL
	}
	if pageURL == "" {
		pageURL = rootURL.String()
	}
	clientData := prepCloneMap(publicData)
	clientData["title"] = data.Page.Title
	clientData["isHomeFirstPage"] = p.Profile == "home" && (data.Paginator == nil || data.Paginator.Current == 1) && prepString(input.Page["path"]) == ""
	blog := data.Site.StellarV2.Blog
	p.BlogNativeComments = prepGet(blog, "native_comments.enabled") == true
	p.BlogContent = prepGet(blog, "content.enabled") == true
	publicData["blogReading"] = prepGet(blog, "reading.enabled") == true
	bundle, err := clientconfig.Build(clientconfig.Input{
		Root: root, PageURL: pageURL, SiteURL: rootURL.String(), SiteTitle: data.Site.Title, Language: p.Language,
		Profile: p.Profile, CollectionID: prepString(collection["id"]), Theme: theme, Page: input.FrontMatter,
		Collection: input.CollectionConfig, Regions: p.Regions, Data: clientData,
		Blog: clientconfig.BlogConfig{GoSearch: prepGet(blog, "search.provider") == "go", NativeComments: p.BlogNativeComments,
			AISummary: prepGet(blog, "ai.summary.enabled") == true, AIAsk: prepGet(blog, "ai.ask.enabled") == true,
			Reading: prepGet(blog, "reading.enabled") == true, Tagcloud: prepGet(blog, "tagcloud.enabled") == true,
			Checkbox: prepGet(theme, "tags.checkbox.interactive") == true, Content: p.BlogContent, Gist: true,
			SearchPath: root + "search.json", SearchTTL: 30},
	})
	if err != nil {
		return nil, err
	}
	p.Client = bundle
	p.CSS, err = appearance.BuildWithOptions(theme, appearance.Options{RootURL: rootURL.String(), CSSURL: cssURL, SkipHighlightStylesheet: true})
	if err != nil {
		return nil, err
	}
	p.HeadInject = prepInject(theme, input.FrontMatter, "headBegin")
	p.BodyBegin = prepInject(theme, input.FrontMatter, "bodyBegin")
	p.BodyEnd = prepInject(theme, input.FrontMatter, "bodyEnd")
	return &StellarRenderData{ViewData: data, Stellar: p}, nil
}

func prepDetailLayout(layout string) bool {
	return layout == "post" || layout == "page" || layout == "wiki-doc" || layout == "note"
}
func prepContentLayout(layout string) bool {
	return prepDetailLayout(layout) || layout == "settings" || layout == "wiki" || layout == "post-password"
}
func prepProfile(layout string) string {
	switch layout {
	case "index":
		return "home"
	case "wiki":
		return "wikiIndex"
	case "topic":
		return "blogIndex"
	case "notes":
		return "notebook"
	case "notebooks":
		return "notebooks"
	case "archive", "categories", "tags", "category", "tag", "author":
		return "blogIndex"
	case "settings":
		return "settings"
	default:
		return "page"
	}
}
func prepMap(v any) map[string]any              { m, _ := v.(map[string]any); return m }
func prepString(v any) string                   { s, _ := v.(string); return s }
func prepGet(m map[string]any, path string) any { v, _ := stellarv2.Get(m, path); return v }
func prepCloneMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return prepClone(m).(map[string]any)
}
func prepClone(v any) any {
	switch x := v.(type) {
	case map[string]any:
		r := map[string]any{}
		for k, v := range x {
			r[k] = prepClone(v)
		}
		return r
	case []any:
		r := make([]any, len(x))
		for i, v := range x {
			r[i] = prepClone(v)
		}
		return r
	default:
		return v
	}
}
func prepInject(theme, page map[string]any, kind string) template.HTML {
	a, b := prepString(prepGet(theme, "inject."+kind)), prepString(prepGet(page, "inject."+kind))
	if a == "" {
		return template.HTML(b)
	}
	if b == "" {
		return template.HTML(a)
	}
	return template.HTML(a + "\n" + b)
}
func prepBrandTokens(regions, site map[string]any) {
	for _, region := range []string{"topbar", "leftbar"} {
		brand := prepMap(prepGet(regions, region+".brand"))
		for _, key := range []string{"name", "tagline", "href"} {
			if _, ok := brand[key]; ok {
				brand[key] = stellarv2.ReplaceConfigTokens(brand[key], site)
			}
		}
		image := prepMap(brand["image"])
		if _, ok := image["src"]; ok {
			image["src"] = stellarv2.ReplaceConfigTokens(image["src"], site)
		}
	}
}

func prepPrepareWidgets(data *settings.ViewData, p *StellarPresentation) {
	for _, region := range []string{"topbar", "leftbar", "rightbar"} {
		instances, _ := prepGet(p.Regions, region+".widgets").([]any)
		for _, value := range instances {
			state := prepMap(value)
			item := prepMap(state["item"])
			w := &settings.WidgetInstance{Name: prepString(state["id"]), Layout: prepString(item["layout"]), Params: settings.WidgetDef(item), V: data}
			switch w.Layout {
			case "recent":
				if p.Profile == "wiki" {
					w.Data = data.WikiRecentDocs
				} else {
					w.Data = data.Recent
				}
			case "tagcloud":
				w.Data = data.TagCloud
			case "author":
				if len(data.Page.AuthorLinks) > 0 {
					a := data.Page.AuthorLinks[0]
					w.Data = &settings.AuthorWidgetData{ID: a.ID, Author: settings.AuthorDef{Name: a.Name, Avatar: a.Avatar, Description: a.Description, URL: a.URL}, Authors: data.Page.AuthorLinks, Path: p.Root + "author/" + url.PathEscape(a.ID) + "/"}
				}
			}
			p.Widgets[region] = append(p.Widgets[region], StellarWidget{State: state, Widget: w})
		}
	}
}

func prepAppearanceURLs(theme map[string]any, rootURL, cssURL string) error {
	for _, entry := range []struct{ path, base string }{{"appearance.codeBlock.highlightStylesheet", rootURL}, {"appearance.backgrounds.page.image", cssURL}, {"appearance.backgrounds.leftbar.image", cssURL}} {
		value := prepString(prepGet(theme, entry.path))
		if value == "" {
			continue
		}
		if strings.HasPrefix(value, "data:") || strings.HasPrefix(value, "blob:") {
			continue
		}
		resolved, err := stellarhtml.ResolveHTTPResource(value, entry.base)
		if err != nil {
			return fmt.Errorf("%s: %w", entry.path, err)
		}
		keys := strings.Split(entry.path, ".")
		object := theme
		for _, key := range keys[:len(keys)-1] {
			object = prepMap(object[key])
			if object == nil {
				break
			}
		}
		if object != nil {
			object[keys[len(keys)-1]] = resolved
		}
	}
	return nil
}
