package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"strings"

	"blog/internal/content"
	"blog/internal/settings"
	"blog/internal/stellarhtml"
	"blog/internal/stellarmodels"
	"blog/internal/stellarv2"
	"blog/internal/store"
	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"
	"golang.org/x/net/html"
)

func stellarDeepObject(m map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	err = decoder.Decode(&out)
	return out, err
}
func stellarPreviewSite(site *tmpl.Site, requestURL string) (*tmpl.Site, error) {
	if site == nil || site.StellarV2 == nil {
		return nil, store.ErrStellarMigrationRequired
	}
	original, err := url.Parse(site.URL)
	if err != nil {
		return nil, err
	}
	request, err := url.Parse(requestURL)
	if err != nil || request.Host == "" || (request.Scheme != "http" && request.Scheme != "https") {
		return nil, fmt.Errorf("preview requires actual HTTP(S) request URL")
	}
	copy := *site
	version := *site.StellarV2
	version.Theme, err = stellarDeepObject(version.Theme)
	if err != nil {
		return nil, err
	}
	version.Blog, err = stellarDeepObject(version.Blog)
	if err != nil {
		return nil, err
	}
	copy.StellarV2 = &version
	request.Path = strings.TrimRight(original.Path, "/")
	request.RawPath = ""
	request.RawQuery = ""
	request.Fragment = ""
	copy.URL = request.String()
	features := stellarObject(version.Theme["features"])
	features["partialNavigation"] = stellarmodels.Object{"enabled": false}
	version.Theme["features"] = features
	// 预览保留公开配置，但关闭需要身份、写入或服务端密钥的宿主扩展。
	version.Blog["native_comments"] = stellarmodels.Object{"enabled": false}
	version.Blog["ai"] = stellarmodels.Object{}
	for _, key := range []string{"reading", "checkbox", "search"} {
		version.Blog[key] = stellarmodels.Object{"enabled": false}
	}
	return &copy, nil
}
func (s *Server) RenderContentPreview(input content.StellarPreviewInput) (string, error) {
	if s.web == nil {
		return "", fmt.Errorf("stellar preview renderer unavailable")
	}
	return s.web.renderContentPreview(input)
}
func (s *Server) StellarWidgetCatalog() (map[string]any, error) {
	if s.deps.Site == nil {
		return nil, store.ErrStellarMigrationRequired
	}
	site := s.deps.Site.Load()
	if site == nil || site.StellarV2 == nil {
		return nil, store.ErrStellarMigrationRequired
	}
	return stellarweb.MergeStellarWidgetCatalog(stellarObject(stellarObject(site.StellarV2.Blog["data"])["widgets"])), nil
}
func (s *webServer) renderContentPreview(input content.StellarPreviewInput) (string, error) {
	site, err := stellarPreviewSite(s.site.Load(), input.RequestURL)
	if err != nil {
		return "", err
	}
	snap, err := s.stellarSnapshotAt(context.Background(), site, nil, true)
	if err != nil {
		return "", err
	}
	kind, profile := "posts", "post"
	if input.Layout != "" && input.Layout != "post" && input.Layout != "topic" {
		kind, profile = "pages", "page"
	}
	path := stellarPath(input.Path)
	root := strings.TrimRight(mustStellarURL(site.URL).Path, "/")
	if root != "" && (path == root || strings.HasPrefix(path, root+"/")) {
		path = stellarPath(strings.TrimPrefix(path, root))
	}
	source := strings.Trim(path, "/") + ".md"
	if kind == "posts" {
		source = "_posts/" + strings.TrimPrefix(path, "/post/") + ".md"
	}
	// 任何已有受保护文章都不能借组件预览的上下文读取。
	if strings.HasPrefix(path, "/post/") {
		row, e := s.st.GetBySlug(strings.TrimPrefix(path, "/post/"))
		if e != nil {
			return "", e
		}
		if row != nil && (row.IsPasswordProtected() || row.Status != store.StatusPublished || row.DeletedAt != "") {
			return "", errStellarNotFound
		}
	}
	row := &store.Post{StellarVersion: input.StellarVersion, StellarConfig: input.StellarConfig, BlogExtensions: input.BlogExtensions, Slug: strings.Trim(path, "/")}
	parsed, err := row.ValidateStellar()
	if err != nil {
		return "", err
	}
	membership := stellarv2.ResolveContentMembership(stellarv2.MembershipInput{Kind: kind, Source: source, PagePath: path, Config: parsed.Value, Registry: snap.registry})
	if membership.Config == nil || len(membership.Issues) > 0 {
		return "", &stellarv2.SchemaError{Issues: membership.Issues}
	}
	config := membership.Config
	if declared := stellarString(stellarGet(config, "collection.profile")); declared != "" {
		profile = declared
	}
	config = stellarv2.ValidatePageProfile(config, source, profile).Value
	in := snap.input
	in.Source = source
	in.FrontMatter = config
	in.Page = stellarmodels.Object{"_id": "__preview__", "title": input.Title, "layout": map[string]string{"posts": "post", "pages": "page"}[kind], "path": strings.TrimPrefix(path, "/"), "permalink": tmpl.JoinSiteURL(site.URL, path), "lang": input.Language}
	if existing := snap.byPath[stellarPathKey(path)]; existing != nil {
		in = existing.input
		in.Source = source
		in.FrontMatter = config
		in.Page = stellarCopyObject(existing.page)
		in.Page["title"] = input.Title
		in.Page["lang"] = input.Language
	}
	if profile != "post" && profile != "page" {
		id := stellarString(stellarGet(config, "collection.id"))
		collection := snap.byCollection[profile+":"+id]
		if collection == nil {
			return "", errStellarNotFound
		}
		in.CollectionID = id
		in.CollectionSource = collection.source
		in.CollectionConfig = collection.config
		in.CollectionModel = collection.model
		in.CollectionState = collection.state
		in.TagTree = nil
		for _, raw := range stellarArray(collection.state["tags"]) {
			in.TagTree = append(in.TagTree, stellarObject(raw))
		}
		in.RecentItems = nil
		for _, raw := range stellarArray(collection.state["recentItems"]) {
			in.RecentItems = append(in.RecentItems, stellarObject(raw))
		}
		in.Members = []stellarmodels.Member{}
		for _, r := range collection.members {
			in.Members = append(in.Members, stellarmodels.Member{Page: r.page, FrontMatter: r.config})
		}
	}
	language := stellarLanguage(in, config)
	body, err := stellarhtml.ProcessContent(input.HTML, stellarhtml.Context{Theme: in.Theme, SiteURL: tmpl.NormalizeSiteURL(site.URL), Icon: s.icons.Get, IconData: s.icons.Get, Language: language})
	if err != nil {
		return "", err
	}
	in.Page["content"] = body
	in.Page = stellarmodels.PageModelInput(in.Page, config)
	if _, err := stellarmodels.BuildPage(in); err != nil {
		return "", err
	}
	layout := map[string]string{"post": "post", "page": "page", "topic": "post", "wiki": "wiki-doc", "notebook": "note"}[profile]
	data := &tmpl.ViewData{Site: site, Page: &tmpl.Page{Title: input.Title, Path: path, Layout: layout, Lang: language}, Content: template.HTML(body), Stellar: &settings.StellarRenderContext{Version: 2, Profile: profile, Input: in, RequestURL: input.RequestURL, NavigationMembers: map[string]string{}}}
	data.Stellar.Input.RuntimeData = stellarCopyObject(data.Stellar.Input.RuntimeData)
	data.Stellar.Input.RuntimeData["nativeComments"] = stellarmodels.Object{"enabled": false}
	data.Stellar.Input.RuntimeData["preview"] = true
	data.Stellar.Input.RuntimeData["recordBlog"] = stellarPublishedBlog(input.BlogExtensions)
	if input.WidgetConfig != "" {
		page := stellarCopyObject(data.Stellar.Input.Page)
		page["content"] = ""
		data.Stellar.Input.Page = page
		data.Content = ""
		var widget any
		decoder := json.NewDecoder(strings.NewReader(input.WidgetConfig))
		decoder.UseNumber()
		if err := decoder.Decode(&widget); err != nil {
			return "", err
		}
		catalog := stellarCopyObject(stellarObject(data.Stellar.Input.RuntimeData["widgets"]))
		catalog["__preview__"] = widget
		data.Stellar.Input.RuntimeData["widgets"] = catalog
		fm := stellarCopyObject(config)
		fm["rightbar"] = stellarmodels.Object{"enabled": true, "widgets": []any{"__preview__"}}
		fm["leftbar"] = stellarmodels.Object{"enabled": false}
		fm["topbar"] = stellarmodels.Object{"enabled": false}
		data.Stellar.Input.FrontMatter = fm
	}
	data.Page.CanonicalURL = tmpl.JoinSiteURL(site.URL, path)
	data.TOC = stellarweb.ExtractTOCWith(body, s.tocOptionsFrom(site))
	document, err := s.engine.Render("layout/base", data)
	if err != nil {
		return "", err
	}
	return stellarPreviewDocument(document, site.URL, input.PreviewToken, input.WidgetConfig != "")
}
func mustStellarURL(raw string) *url.URL {
	u, _ := url.Parse(raw)
	if u == nil {
		return &url.URL{}
	}
	return u
}
func stellarPreviewDocument(document, siteURL, token string, widget bool) (string, error) {
	doc, err := html.Parse(strings.NewReader(document))
	if err != nil {
		return "", err
	}
	var body *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			attrs := n.Attr[:0]
			for _, attr := range n.Attr {
				if attr.Key != "data-stellar-navigation" {
					attrs = append(attrs, attr)
				}
			}
			n.Attr = attrs
		}
		if n.Type == html.ElementNode && n.Data == "html" {
			n.Attr = append(n.Attr, html.Attribute{Key: "data-stellar-preview-token", Val: token})
		}
		if n.Type == html.ElementNode && n.Data == "body" {
			body = n
		}
		for c := n.FirstChild; c != nil; {
			next := c.NextSibling
			remove := false
			for _, attr := range c.Attr {
				if c.Type == html.ElementNode && c.Data == "script" && attr.Key == "id" && attr.Val == "stellar-navigation-config" {
					remove = true
				}
			}
			if remove {
				n.RemoveChild(c)
			} else {
				walk(c)
			}
			c = next
		}

	}
	walk(doc)
	if body == nil {
		return "", fmt.Errorf("preview document missing body")
	}
	if widget {
		var instance, scripts *html.Node
		var find func(*html.Node)
		find = func(n *html.Node) {
			for _, attr := range n.Attr {
				if attr.Key == "data-widget-id" && strings.Contains(attr.Val, "__preview__") {
					instance = n
				}
				if attr.Key == "class" && attr.Val == "site-scripts" {
					scripts = n
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				find(c)
			}
		}
		find(body)
		if instance != nil && instance.Parent != nil {
			instance.Parent.RemoveChild(instance)
		}
		if scripts != nil && scripts.Parent != nil {
			scripts.Parent.RemoveChild(scripts)
		}
		for c := body.FirstChild; c != nil; {
			next := c.NextSibling
			body.RemoveChild(c)
			c = next
		}
		container := &html.Node{Type: html.ElementNode, Data: "main", Attr: []html.Attribute{{Key: "data-stellar-preview-content", Val: ""}, {Key: "class", Val: "ui-surface"}}}
		if instance != nil {
			container.AppendChild(instance)
		}
		body.AppendChild(container)
		if scripts != nil {
			body.AppendChild(scripts)
		}
	}
	body.AppendChild(&html.Node{Type: html.ElementNode, Data: "script", Attr: []html.Attribute{{Key: "src", Val: tmpl.JoinSiteURL(siteURL, "/assets/js/blog-preview.js")}, {Key: "defer", Val: ""}, {Key: "data-stellar-script", Val: "blog-preview"}}})
	var out bytes.Buffer
	err = html.Render(&out, doc)
	return out.String(), err
}
