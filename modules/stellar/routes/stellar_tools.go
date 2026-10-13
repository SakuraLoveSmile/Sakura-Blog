package routes

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"blog/internal/stellarimages"
	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"
)

// RenderToolSnapshots 只读取一次公开内容快照；不挂载路由、索引钩子或后台任务。
func RenderToolSnapshots(ctx context.Context, d Dependencies) ([]stellarimages.PageSnapshot, error) {
	if d.Site == nil || d.Store == nil || d.Engine == nil || d.Posts == nil {
		return nil, fmt.Errorf("stellar snapshot dependencies unavailable")
	}
	ws := newWebServer(webDeps{Site: d.Site, Store: d.Store, Posts: d.Posts, Engine: d.Engine, Icons: d.Icons, Root: d.Root, UploadDir: d.UploadDir, NotesPath: d.NotesPath})
	return ws.renderSnapshots(ctx)
}
func (s *Server) RenderSnapshots(ctx context.Context) ([]stellarimages.PageSnapshot, error) {
	return RenderToolSnapshots(ctx, s.deps)
}
func (s *webServer) renderSnapshots(ctx context.Context) ([]stellarimages.PageSnapshot, error) {
	snap, err := s.stellarSnapshotAt(ctx, s.site.Load(), nil, true)
	if err != nil {
		return nil, err
	}
	output := []stellarimages.PageSnapshot{}
	seen := map[string]bool{}
	appendView := func(path string, data *tmpl.ViewData) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		key := strings.Trim(path, "/")
		if seen[key] {
			return nil
		}
		seen[key] = true
		absolute := tmpl.JoinSiteURL(snap.site.URL, path)
		data.Stellar.RequestURL = absolute
		page := stellarCopyObject(data.Stellar.Input.Page)
		public, _ := url.Parse(absolute)
		page["path"] = strings.TrimLeft(public.Path, "/")
		canonical := *public
		if host := canonicalHost(stellarString(stellarGet(snap.input.Theme, "canonical.host"))); host != "" {
			canonical.Scheme = "https"
			canonical.Host = host
		}
		page["permalink"] = canonical.String()
		data.Stellar.Input.Page = page
		data.Stellar.Input.RuntimeData = stellarCopyObject(data.Stellar.Input.RuntimeData)
		data.Stellar.Input.RuntimeData["offline"] = true
		data.Page.CanonicalURL = canonical.String()
		if data.Page.Description == "" {
			data.Page.Description = snap.site.Description
		}
		data.TOC = stellarweb.ExtractTOCWith(string(data.Content), s.tocOptionsFrom(data.Site))
		html, err := s.engine.Render("layout/base", data)
		if err != nil {
			return fmt.Errorf("render %s: %w", path, err)
		}
		if key == "" {
			key = "index.html"
		} else if !strings.HasSuffix(key, ".html") {
			key += "/index.html"
		}
		output = append(output, stellarimages.PageSnapshot{Path: key, HTML: html, BaseURL: absolute})
		return nil
	}
	appendPages := func(path string, build func(int) (*tmpl.ViewData, error)) error {
		for page := 1; ; page++ {
			data, err := build(page)
			if errors.Is(err, errStellarNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			current := path
			if page > 1 {
				current = strings.TrimRight(path, "/") + fmt.Sprintf("/page/%d", page)
			}
			if err := appendView(current, data); err != nil {
				return err
			}
			if data.Paginator == nil || page >= data.Paginator.Total {
				return nil
			}
		}
	}
	for _, r := range snap.records {
		if err := appendView(r.path, s.stellarView(snap, r)); err != nil {
			return nil, err
		}
	}
	posts := []*stellarRecord{}
	for _, r := range snap.records {
		if r.post != nil {
			posts = append(posts, r)
		}
	}
	for _, profile := range []string{"home", "blogIndex"} {
		path := stellarProfilePath(snap.input.Theme, "blogIndex")
		if profile == "home" {
			path = "/"
		}
		if err := appendPages(path, func(page int) (*tmpl.ViewData, error) {
			return s.stellarListingView(snap, profile, snap.site.Title, path, posts, page, nil, nil)
		}); err != nil {
			return nil, err
		}
	}
	for _, profile := range []string{"wiki", "topic", "notebook"} {
		data, err := s.stellarCollectionIndexView(snap, profile, "")
		if err != nil {
			return nil, err
		}
		if err := appendView(data.Page.Path, data); err != nil {
			return nil, err
		}
		if profile == "wiki" {
			for _, raw := range stellarArray(data.Stellar.Input.RuntimeData["wikiTags"]) {
				tag := stellarString(stellarObject(raw)["name"])
				filtered, err := s.stellarCollectionIndexView(snap, profile, tag)
				if err != nil {
					return nil, err
				}
				if err := appendView(data.Page.Path+"/tags/"+tag, filtered); err != nil {
					return nil, err
				}
			}
		}
	}
	for _, collection := range snap.collections {
		if collection.profile == "wiki" {
			if stellarGet(collection.model, "route.homepage") != "" {
				continue
			}
			data := s.stellarAggregate(snap, "wiki", stellarString(stellarGet(collection.model, "identity.name")), collection.path, "wiki")
			data.Stellar.Input.CollectionConfig = collection.config
			data.Stellar.Input.CollectionModel = collection.model
			data.Stellar.Input.CollectionID = collection.id
			data.Stellar.Collections = append(data.Stellar.Collections, collection.index)
			if err := appendView(collection.path, data); err != nil {
				return nil, err
			}
			continue
		}
		if err := appendPages(collection.path, func(page int) (*tmpl.ViewData, error) {
			return s.stellarListingView(snap, collection.profile, stellarString(stellarGet(collection.model, "identity.name")), collection.path, collection.members, page, collection, nil)
		}); err != nil {
			return nil, err
		}
		if collection.profile == "notebook" {
			for _, raw := range stellarArray(collection.index["tags"]) {
				tag := stellarString(stellarObject(raw)["id"])
				if tag == "" {
					tag = stellarString(stellarObject(raw)["name"])
				}
				path := collection.path + "/tags/" + tag
				if err := appendPages(path, func(page int) (*tmpl.ViewData, error) {
					return s.stellarListingView(snap, "notebook", stellarString(stellarGet(collection.model, "identity.name")), path, collection.members, page, collection, &tag)
				}); err != nil {
					return nil, err
				}
			}
		}
	}
	for _, kind := range []string{"archives", "categories", "tags"} {
		path := stellarTaxonomyPath(snap.input.Theme, kind)
		data, err := s.stellarTaxonomyView(snap, kind, "", 1)
		if err != nil {
			return nil, err
		}
		if err := appendView(path, data); err != nil {
			return nil, err
		}
		terms := stellarTaxonomyTerms(snap, kind)
		names := []string{}
		for name := range terms {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if err := appendPages(path+"/"+name, func(page int) (*tmpl.ViewData, error) { return s.stellarTaxonomyView(snap, kind, name, page) }); err != nil {
				return nil, err
			}
		}
	}
	authors := stellarObject(snap.input.RuntimeData["authors"])
	ids := []string{}
	for id := range authors {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := appendPages(stellarProfilePath(snap.input.Theme, "author")+"/"+id, func(page int) (*tmpl.ViewData, error) { return s.stellarAuthorView(snap, id, page) }); err != nil {
			return nil, err
		}
	}
	if err := appendPages("/moments", func(page int) (*tmpl.ViewData, error) { return s.stellarMomentsView(snap, page) }); err != nil {
		return nil, err
	}
	for _, profile := range []string{"settings", "error"} {
		layout := profile
		if profile == "error" {
			layout = "404"
		}
		data := s.stellarAggregate(snap, profile, map[string]string{"settings": "设置", "error": "页面不存在"}[profile], stellarProfilePath(snap.input.Theme, profile), layout)
		if err := appendView(data.Page.Path, data); err != nil {
			return nil, err
		}
	}
	return output, nil
}
