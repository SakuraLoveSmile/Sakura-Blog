package routes

import (
	"blog/internal/settings"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"blog/internal/stellarmodels"
	"blog/internal/stellarv2"
	"blog/internal/store"
	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"

	"github.com/gin-gonic/gin"
)

type stellarHTTPError struct {
	Error  string            `json:"error"`
	Code   string            `json:"code"`
	Issues []stellarv2.Issue `json:"issues,omitempty"`
}

func stellarError(c *gin.Context, err error) {
	c.Header("Cache-Control", "no-store")
	var schema *stellarv2.SchemaError
	var model *stellarmodels.ConfigError
	if errors.Is(err, store.ErrStellarMigrationRequired) {
		c.JSON(http.StatusConflict, stellarHTTPError{Error: "内容尚未迁移到 Stellar 2.0，请先审阅并执行迁移预览", Code: "stellar_migration_required"})
		return
	}
	if errors.As(err, &schema) {
		c.JSON(http.StatusUnprocessableEntity, stellarHTTPError{Error: "Stellar 2.0 内容配置或归属无效", Code: "stellar_content_invalid", Issues: schema.Issues})
		return
	}
	if errors.As(err, &model) {
		c.JSON(http.StatusUnprocessableEntity, stellarHTTPError{Error: "Stellar 2.0 页面模型无效", Code: "stellar_model_invalid"})
		return
	}
	log.Printf("stellar public route: %v", err)
	c.JSON(http.StatusInternalServerError, stellarHTTPError{Error: "页面暂时不可用", Code: "stellar_render_failed"})
}
func (s *webServer) outputStellar(c *gin.Context, data *tmpl.ViewData, status int) {
	s.fillSEO(data, c)
	if data.Stellar != nil {
		data.Stellar.RequestURL = requestScheme(c) + "://" + c.Request.Host + c.Request.URL.EscapedPath()
		if c.Request.URL.RawQuery != "" {
			data.Stellar.RequestURL += "?" + c.Request.URL.RawQuery
		}
		page := stellarCopyObject(data.Stellar.Input.Page)
		page["path"] = strings.TrimLeft(c.Request.URL.Path, "/")
		canonical := s.canonicalBaseURL(data, c)
		base, _ := url.Parse(canonical)
		if base != nil {
			base.Path = c.Request.URL.Path
			base.RawPath = c.Request.URL.EscapedPath()
			base.RawQuery = ""
			base.Fragment = ""
			page["permalink"] = base.String()
		}
		data.Stellar.Input.Page = page
	}

	if data.Content != "" {
		data.TOC = stellarweb.ExtractTOCWith(string(data.Content), s.tocOptionsFrom(data.Site))
	}
	html, err := s.engine.Render("layout/base", data)
	if err != nil {
		stellarError(c, err)
		return
	}
	c.Data(status, "text/html; charset=utf-8", []byte(html))
}
func (s *webServer) stellarAggregate(snap *stellarSnapshot, profile, title, path, layout string) *tmpl.ViewData {
	in := snap.input
	in.Source = path
	in.Page = stellarmodels.Object{"path": strings.TrimPrefix(path, "/"), "title": title, "layout": layout, "permalink": tmpl.JoinSiteURL(snap.site.URL, path)}
	in.FrontMatter = stellarmodels.Object{}
	return &tmpl.ViewData{Site: snap.site, Page: &tmpl.Page{Title: title, Path: path, Layout: layout, Lang: stellarLanguage(in, nil)}, Stellar: &settings.StellarRenderContext{Version: 2, Profile: profile, Input: in, NavigationMembers: map[string]string{}}}
}
func (s *webServer) serveStellarPost(c *gin.Context, slug string, shared *store.Post) {
	row := shared
	if row == nil {
		var err error
		row, err = s.st.GetPublishedBySlug(slug)
		if err != nil {
			stellarError(c, err)
			return
		}
		if row == nil {
			s.render404(c)
			return
		}
		if row.IsPasswordProtected() {
			cookie, _ := c.Cookie(postPWCookieName(slug))
			if !validPostPWCookie(cookie, slug, row.Password) {
				s.renderPostPWPage(c, row, "")
				return
			}
		}
	}
	snap, err := s.stellarSnapshot(row)
	if err != nil {
		stellarError(c, err)
		return
	}
	for _, r := range snap.records {
		if r.post != nil && r.post.ID == row.ID && r.post.Slug == row.Slug {
			if shared == nil && r.post.IsPasswordProtected() {
				cookie, _ := c.Cookie(postPWCookieName(slug))
				if !validPostPWCookie(cookie, slug, r.post.Password) {
					s.renderPostPWPage(c, r.post, "")
					return
				}
			}
			data := s.stellarView(snap, r)
			if shared == nil {
				if views, e := s.st.IncrViewsAndGet(slug); e == nil {
					data.Page.Views = views
				}
			} else {
				stellarNotice(data, `<div class="blog-preview-notice">这是一篇通过分享链接预览的文章</div>`)
			}
			s.outputStellar(c, data, http.StatusOK)
			return
		}
	}
	s.render404(c)
}
func (s *webServer) serveStellarPage(c *gin.Context, p *store.Page) {
	if p == nil || p.Status != store.StatusPublished || p.DeletedAt != "" {
		s.render404(c)
		return
	}
	snap, err := s.stellarSnapshot(nil)
	if err != nil {
		stellarError(c, err)
		return
	}
	for _, r := range snap.records {
		if r.pageRow != nil && r.pageRow.ID == p.ID {
			data := s.stellarView(snap, r)
			_ = s.st.IncrementPageViews(p.Slug)
			s.outputStellar(c, data, http.StatusOK)
			return
		}
	}
	s.render404(c)
}
func (s *webServer) serveStellarWiki(c *gin.Context, project, slug string) {
	row, err := s.st.GetWikiProject(project)
	if err != nil {
		stellarError(c, err)
		return
	}
	if row == nil || !row.Visible {
		s.render404(c)
		return
	}
	if slug != "" {
		doc, err := s.st.GetWikidocBySlug(project, slug)
		if err != nil {
			stellarError(c, err)
			return
		}
		if doc == nil || doc.Status != store.StatusPublished {
			s.render404(c)
			return
		}
	}
	snap, err := s.stellarSnapshot(nil)
	if err != nil {
		stellarError(c, err)
		return
	}
	collection := snap.byCollection["wiki:"+project]
	if collection == nil {
		s.render404(c)
		return
	}
	if slug == "" {
		homepage := stellarString(stellarGet(collection.model, "route.homepage"))
		if homepage != "" {
			c.Redirect(http.StatusFound, stellarPath(homepage))
			return
		}
		data := s.stellarAggregate(snap, "wiki", stellarString(stellarGet(collection.model, "identity.name")), collection.path, "wiki")
		data.Stellar.Input.CollectionConfig = collection.config
		data.Stellar.Input.CollectionModel = collection.model
		data.Stellar.Input.CollectionID = project
		data.Stellar.Collections = []stellarmodels.Object{collection.index}
		s.outputStellar(c, data, http.StatusOK)
		return
	}
	for _, r := range collection.members {
		if (r.doc != nil && r.doc.Slug == slug) || (r.pageRow != nil && strings.HasSuffix(r.source, "/"+slug+".md")) {
			s.outputStellar(c, s.stellarView(snap, r), http.StatusOK)
			return
		}
	}
	s.render404(c)
}
func (s *webServer) serveStellarNote(c *gin.Context, notebook, slug string) {
	row, err := s.st.GetPublishedNote(notebook, slug)
	if err != nil {
		stellarError(c, err)
		return
	}
	if row == nil {
		s.render404(c)
		return
	}
	snap, err := s.stellarSnapshot(nil)
	if err != nil {
		stellarError(c, err)
		return
	}
	for _, r := range snap.records {
		if r.note != nil && r.note.ID == row.ID {
			s.outputStellar(c, s.stellarView(snap, r), http.StatusOK)
			return
		}
	}
	s.render404(c)
}

var errStellarNotFound = errors.New("stellar page not found")

func (s *webServer) stellarListing(c *gin.Context, snap *stellarSnapshot, profile, title, path string, records []*stellarRecord, page int, collection *stellarCollection, tag *string) {
	data, err := s.stellarListingView(snap, profile, title, path, records, page, collection, tag)
	if err != nil {
		if errors.Is(err, errStellarNotFound) {
			s.render404(c)
		} else {
			stellarError(c, err)
		}
		return
	}
	s.outputStellar(c, data, http.StatusOK)
}
func (s *webServer) stellarListingView(snap *stellarSnapshot, profile, title, path string, records []*stellarRecord, page int, collection *stellarCollection, tag *string) (*tmpl.ViewData, error) {
	data := s.stellarAggregate(snap, profile, title, path, "index")
	if profile == "notebook" {
		data.Page.Layout = "notes"
	}
	if profile == "author" {
		data.Page.Layout = "author"
	}
	candidates := []stellarmodels.Object{}
	byID := map[string]*stellarRecord{}
	for _, r := range records {
		item := stellarCopyObject(stellarObject(r.model.Render["listing"]))
		item["id"] = r.model.Item["id"]
		if r.profile == "wiki" {
			continue
		}
		candidates = append(candidates, item)
		byID[stellarString(item["id"])] = r
	}
	tags := []stellarmodels.Object{}
	sortConfig := stellarmodels.Object{"field": "date", "direction": "desc"}
	perPage := int(stellarNumber(snap.input.SiteConfig["per_page"]))
	if collection != nil {
		sortConfig = stellarObject(stellarGet(collection.model, "listing.sort"))
		if value := stellarGet(collection.model, "listing.perPage"); value != nil {
			perPage = int(stellarNumber(value))
		}
		for _, v := range stellarArray(collection.index["tags"]) {
			tags = append(tags, stellarObject(v))
		}
		data.Stellar.Input.CollectionConfig = collection.config
		data.Stellar.Input.CollectionModel = collection.model
		data.Stellar.Input.CollectionID = collection.id
		data.Stellar.Index = collection.index
		data.Stellar.Input.RuntimeData = stellarCopyObject(data.Stellar.Input.RuntimeData)
		data.Stellar.Input.RuntimeData["notebookId"] = collection.id
		data.Stellar.Input.RuntimeData["tagTree"] = collection.index["tags"]
		data.Stellar.Input.RuntimeData["recentItems"] = collection.index["recentItems"]
		if tag != nil {
			data.Stellar.Input.RuntimeData["activeTag"] = *tag
		}
	}
	selected := stellarmodels.SelectListingItems(candidates, tag, tags, func(a, b stellarmodels.Object) bool {
		if stellarNumber(a["priority"]) != stellarNumber(b["priority"]) {
			return stellarNumber(a["priority"]) > stellarNumber(b["priority"])
		}
		field := stellarString(sortConfig["field"])
		if field == "" {
			field = "date"
		}
		left, right := stellarString(a[field]), stellarString(b[field])
		if sortConfig["direction"] == "asc" {
			return left < right
		}
		return left > right
	})
	if (profile == "home" || profile == "blogIndex") && stellarGet(snap.input.Theme, "article.listing.pinnedLayout") == "carousel" {
		remaining := []stellarmodels.Object{}
		for _, item := range selected {
			if stellarNumber(item["priority"]) > 0 {
				if page == 1 {
					data.Stellar.PinnedItems = append(data.Stellar.PinnedItems, byID[stellarString(item["id"])].model)
				}
			} else {
				remaining = append(remaining, item)
			}
		}
		selected = remaining
	}
	pages := stellarmodels.PaginateItems(selected, perPage)
	if page < 1 || page > len(pages) {
		return nil, fmt.Errorf("%w: listing page", errStellarNotFound)
	}
	for _, item := range pages[page-1] {
		if r := byID[stellarString(item["id"])]; r != nil {
			data.Stellar.ListItems = append(data.Stellar.ListItems, r.model)
		}
	}
	numbers := make([]int, len(pages))
	for i := range numbers {
		numbers[i] = i + 1
	}
	data.Paginator = &tmpl.Paginator{Current: page, Total: len(pages), Pages: numbers}
	if profile == "home" && page == 1 {
		data.CommentPageKey = stellarPublicPath(snap.site, "/")
		s.fillStellarNativeComments(data, snap, nil)
	}
	return data, nil
}
func stellarCopyObject(source stellarmodels.Object) stellarmodels.Object {
	copy := stellarmodels.Object{}
	for key, value := range source {
		copy[key] = value
	}
	return copy
}
func (s *webServer) serveStellarHome(c *gin.Context, page int, profile string) {
	snap, err := s.stellarSnapshot(nil)
	if err != nil {
		stellarError(c, err)
		return
	}
	records := []*stellarRecord{}
	for _, r := range snap.records {
		if r.post != nil {
			records = append(records, r)
		}
	}
	path := stellarProfilePath(snap.input.Theme, "blogIndex")
	if profile == "home" {
		path = "/"
	}
	s.stellarListing(c, snap, profile, snap.site.Title, path, records, page, nil, nil)
}
func (s *webServer) serveStellarCollectionIndex(c *gin.Context, profile, tag string) {
	snap, err := s.stellarSnapshot(nil)
	if err != nil {
		stellarError(c, err)
		return
	}
	data, err := s.stellarCollectionIndexView(snap, profile, tag)
	if err != nil {
		s.render404(c)
		return
	}
	s.outputStellar(c, data, http.StatusOK)
}
func (s *webServer) stellarCollectionIndexView(snap *stellarSnapshot, profile, tag string) (*tmpl.ViewData, error) {
	routeProfile := map[string]string{"wiki": "wikiIndex", "topic": "topic", "notebook": "notebooks"}[profile]
	title := map[string]string{"wiki": "文库", "topic": "专栏", "notebook": "笔记本"}[profile]
	data := s.stellarAggregate(snap, routeProfile, title, stellarProfilePath(snap.input.Theme, routeProfile), map[string]string{"wiki": "wiki", "topic": "index_topic", "notebook": "notebooks"}[profile])
	all := []stellarmodels.Object{}
	for _, collection := range snap.collections {
		if collection.profile != profile || collection.index["listed"] == false {
			continue
		}
		all = append(all, collection.index)
		if tag != "" && !hasTag(stellarTerms(collection.index["tags"]), tag) {
			continue
		}
		data.Stellar.Collections = append(data.Stellar.Collections, collection.index)
	}
	sort.SliceStable(data.Stellar.Collections, func(i, j int) bool {
		a, b := data.Stellar.Collections[i], data.Stellar.Collections[j]
		if profile == "topic" || profile == "wiki" {
			blogs := stellarObject(data.Stellar.Input.RuntimeData["collectionBlogs"])
			left, right := stellarGet(stellarObject(blogs[profile+"/"+stellarString(a["id"])]), "content.collection_order"), stellarGet(stellarObject(blogs[profile+"/"+stellarString(b["id"])]), "content.collection_order")
			if left != nil || right != nil {
				if stellarNumber(left) != stellarNumber(right) {
					return stellarNumber(left) < stellarNumber(right)
				}
			}
		}
		if profile == "topic" {
			return stellarString(a["sortDate"]) > stellarString(b["sortDate"])
		}
		if stellarNumber(a["priority"]) != stellarNumber(b["priority"]) {
			return stellarNumber(a["priority"]) > stellarNumber(b["priority"])
		}
		if profile == "wiki" {
			return stellarNumber(a["order"]) > stellarNumber(b["order"])
		}
		return stellarNumber(a["order"]) < stellarNumber(b["order"])
	})
	data.Stellar.Input.RuntimeData = stellarCopyObject(data.Stellar.Input.RuntimeData)
	if profile == "wiki" {
		data.Stellar.Input.RuntimeData["allWikiItems"] = all
		data.Stellar.Input.RuntimeData["wikiFilter"] = tag
		data.Stellar.Input.RuntimeData["wikiTagName"] = tag
		tags := []any{}
		names := []string{}
		seen := map[string]bool{}
		for _, item := range all {
			for _, name := range stellarTerms(item["tags"]) {
				if !seen[name] {
					names = append(names, name)
					seen[name] = true
				}
			}
		}
		if tag != "" && !seen[tag] {
			return nil, errStellarNotFound
		}
		for _, name := range names {
			ids := []any{}
			for _, item := range all {
				if hasTag(stellarTerms(item["tags"]), name) {
					ids = append(ids, item["id"])
				}
			}
			tags = append(tags, stellarmodels.Object{"name": name, "path": strings.TrimPrefix(data.Page.Path, "/") + "/tags/" + name, "itemIds": ids})
		}
		data.Stellar.Input.RuntimeData["wikiTags"] = tags
	}
	return data, nil
}
func (s *webServer) serveStellarCollection(c *gin.Context, profile, id string, page int, tag *string) {
	snap, err := s.stellarSnapshot(nil)
	if err != nil {
		stellarError(c, err)
		return
	}
	collection := snap.byCollection[profile+":"+id]
	if collection == nil {
		s.render404(c)
		return
	}
	s.stellarListing(c, snap, map[string]string{"topic": "topic", "notebook": "notebook"}[profile], stellarString(stellarGet(collection.model, "identity.name")), collection.path, collection.members, page, collection, tag)
}

// 热配置路径通过每次请求的主题解析，避免注册时固化旧路径。
func (s *webServer) stellarDynamicRoutes(c *gin.Context) {
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		return
	}
	if s.serveStellarPath(c) {
		c.Abort()
	}
}
func (s *webServer) serveStellarPath(c *gin.Context) bool {
	if s.serveGeneratedStellarAsset(c) || s.serveSiteRootAsset(c) {
		return true
	}
	path := stellarPathKey(c.Request.URL.Path)
	root := s.stellarSiteRoot()
	if root != "" && (path == root || strings.HasPrefix(path, root+"/")) {
		path = stellarPathKey(strings.TrimPrefix(path, root))
		if strings.HasPrefix(path, "/uploads/") {
			s.serveUpload(c, strings.TrimPrefix(path, "/uploads"))
			return true
		}
		switch path {
		case "/search.json":
			s.serveStellarSearchJSON(c)
			return true
		case "/sitemap.xml":
			s.serveStellarSitemap(c)
			return true
		case "/rss.xml":
			s.serveStellarFeed(c, "rss", "", "")
			return true
		case "/atom.xml":
			s.serveStellarFeed(c, "atom", "", "")
			return true
		case "/feed.json":
			s.serveStellarFeed(c, "json", "", "")
			return true
		case "/robots.txt":
			NewFeedHandler(s.st, s.site).robots(c)
			return true
		case "/sw.js":
			s.serveSW()(c)
			return true
		case "/manifest.webmanifest":
			s.serveManifest()(c)
			return true
		case "/favicon.ico":
			s.serveAsset("img/favicon.ico", "image/x-icon", nil)(c)
			return true
		case "/apple-touch-icon.png":
			s.serveAsset("img/apple-touch-icon.png", "image/png", nil)(c)
			return true
		}
	}
	if strings.HasPrefix(path, "/assets/") {
		c.Header("Access-Control-Allow-Origin", "*")
		return false
	}
	if path == "/stellar/embed/gist" {
		s.serveGistEmbed(c)
		return true
	}

	for _, prefix := range []string{"/assets/", "/uploads/", "/api/", "/console", "/stellar/", "/preview/", "/og/"} {
		if strings.HasPrefix(path, prefix) {
			return false
		}
	}
	for _, asset := range []string{"/search.json", "/sitemap.xml", "/rss.xml", "/atom.xml", "/feed.json", "/robots.txt", "/sw.js", "/manifest.json", "/favicon.ico"} {
		if path == asset {
			return false
		}
	}
	if path == "/moments" {
		s.serveStellarMoments(c, 1)
		return true
	}
	if strings.HasPrefix(path, "/moments/page/") {
		page, err := stellarPageNumber(strings.TrimPrefix(path, "/moments/page/"))
		if err != nil {
			s.render404(c)
		} else {
			s.serveStellarMoments(c, page)
		}
		return true
	}
	if strings.HasPrefix(path, "/post/") && !strings.HasSuffix(path, "/unlock") {
		s.serveStellarPost(c, strings.TrimPrefix(path, "/post/"), nil)
		return true
	}
	// 先检查稳定身份路由的公开状态；草稿/隐藏集合永远先返回404。
	for _, prefix := range []string{"/wiki/", "/notebooks/"} {
		if strings.HasPrefix(path, prefix) {
			parts := strings.SplitN(strings.TrimPrefix(path, prefix), "/", 2)
			if parts[0] == "tags" {
				continue
			}
			if prefix == "/wiki/" {
				project, e := s.st.GetWikiProject(parts[0])
				if e != nil {
					stellarError(c, e)
					return true
				}
				if project == nil || !project.Visible {
					s.render404(c)
					return true
				}
				if len(parts) == 2 && !strings.HasPrefix(parts[1], "page/") {
					doc, e := s.st.GetWikidocBySlug(parts[0], parts[1])
					if e != nil {
						stellarError(c, e)
						return true
					}
					if doc == nil || doc.Status != "published" {
						s.render404(c)
						return true
					}
				}
			} else if len(parts) == 2 && !strings.HasPrefix(parts[1], "tags/") && !strings.HasPrefix(parts[1], "page/") {
				note, e := s.st.GetPublishedNote(parts[0], parts[1])
				if e != nil {
					stellarError(c, e)
					return true
				}
				if note == nil {
					s.render404(c)
					return true
				}
			}
		}
	}
	if s.preflightStellarCustomPath(c, path) {
		return true
	}
	if s.site == nil || s.site.Load() == nil {
		return false
	}
	site := s.site.Load()
	if site.StellarV2 == nil {
		for _, prefix := range []string{"/", "/blog", "/wiki", "/topic", "/notebooks", "/settings"} {
			if path == prefix {
				stellarError(c, fmtMigration())
				return true
			}
		}
		return false
	}
	theme := site.StellarV2.Theme
	if path == "" || path == "/" {
		s.serveStellarHome(c, 1, "home")
		return true
	}
	for _, profile := range []string{"blogIndex", "wikiIndex", "topic", "notebooks", "settings", "author", "error"} {
		base := stellarPathKey(stellarProfilePath(theme, profile))
		if base == "" {
			continue
		}
		if path == base {
			switch profile {
			case "blogIndex":
				s.serveStellarHome(c, 1, profile)
			case "wikiIndex":
				s.serveStellarCollectionIndex(c, "wiki", "")
			case "topic":
				s.serveStellarCollectionIndex(c, "topic", "")
			case "notebooks":
				s.serveStellarCollectionIndex(c, "notebook", "")
			case "settings", "error":
				in, err := stellarSiteInput(site)
				if err != nil {
					stellarError(c, err)
					break
				}
				snap := &stellarSnapshot{site: site, input: in}
				layout := profile
				status := http.StatusOK
				if profile == "error" {
					layout = "404"
					status = http.StatusNotFound
				}
				data := s.stellarAggregate(snap, profile, map[string]string{"settings": "设置", "error": "页面不存在"}[profile], path, layout)
				s.outputStellar(c, data, status)
			default:
				s.render404(c)
			}
			return true
		}
		if profile == "blogIndex" && strings.HasPrefix(path, base+"/page/") {
			page, err := stellarPageNumber(strings.TrimPrefix(path, base+"/page/"))
			if err != nil {
				s.render404(c)
			} else {
				s.serveStellarHome(c, page, profile)
			}
			return true
		}
		if profile == "wikiIndex" && strings.HasPrefix(path, base+"/tags/") {
			tag, _ := url.PathUnescape(strings.TrimPrefix(path, base+"/tags/"))
			s.serveStellarCollectionIndex(c, "wiki", tag)
			return true
		}
		if profile == "author" && strings.HasPrefix(path, base+"/") {
			s.serveStellarAuthor(c, strings.TrimPrefix(path, base+"/"))
			return true
		}
	}
	// 稳定身份集合及自定义 collection.route.path 同时使用当前配置。
	snap, err := s.stellarSnapshot(nil)
	if err != nil {
		if strings.HasPrefix(path, stellarPathKey(stellarProfilePath(theme, "wikiIndex"))+"/") || strings.HasPrefix(path, stellarPathKey(stellarProfilePath(theme, "notebooks"))+"/") || strings.HasPrefix(path, stellarPathKey(stellarProfilePath(theme, "topic"))+"/") {
			stellarError(c, err)
			return true
		}
		return false
	}
	if rec := snap.byPath[path]; rec != nil {
		s.outputStellar(c, s.stellarView(snap, rec), http.StatusOK)
		return true
	}
	for _, collection := range snap.collections {
		base := stellarPathKey(collection.path)
		if path == base {
			if collection.profile == "wiki" {
				home := stellarString(stellarGet(collection.model, "route.homepage"))
				if home != "" {
					c.Redirect(http.StatusFound, stellarPath(home))
					return true
				}
			}
			s.serveStellarCollection(c, collection.profile, collection.id, 1, nil)
			return true
		}
		if strings.HasPrefix(path, base+"/page/") {
			page, err := stellarPageNumber(strings.TrimPrefix(path, base+"/page/"))
			if err != nil {
				s.render404(c)
			} else {
				s.serveStellarCollection(c, collection.profile, collection.id, page, nil)
			}
			return true
		}
		if collection.profile == "notebook" && strings.HasPrefix(path, base+"/tags/") {
			tag := strings.TrimPrefix(path, base+"/tags/")
			page := 1
			if index := strings.LastIndex(tag, "/page/"); index >= 0 {
				n, e := stellarPageNumber(tag[index+6:])
				if e != nil {
					s.render404(c)
					return true
				}
				page = n
				tag = tag[:index]
			}
			s.serveStellarCollection(c, "notebook", collection.id, page, &tag)
			return true
		}
	}
	for _, name := range []string{"archives", "categories", "tags"} {
		base := stellarPathKey(stellarTaxonomyPath(theme, name))
		if path == base || strings.HasPrefix(path, base+"/") {
			term := strings.TrimPrefix(strings.TrimPrefix(path, base), "/")
			if strings.HasSuffix(term, "/rss.xml") && name != "archives" {
				s.serveStellarFeed(c, "rss", name, strings.TrimSuffix(term, "/rss.xml"))
				return true
			}
			s.serveStellarTaxonomy(c, name, term)
			return true
		}
	}
	return false
}
func fmtMigration() error { return store.ErrStellarMigrationRequired }
func stellarPagedName(raw string) (string, int, error) {
	page := 1
	if index := strings.LastIndex(raw, "/page/"); index >= 0 {
		n, e := stellarPageNumber(raw[index+6:])
		if e != nil {
			return "", 0, e
		}
		page = n
		raw = raw[:index]
	}
	return strings.Trim(raw, "/"), page, nil
}
func (s *webServer) serveStellarAuthor(c *gin.Context, id string) {
	id, page, err := stellarPagedName(id)
	if err != nil {
		s.render404(c)
		return
	}
	if raw := c.Query("page"); raw != "" {
		page, err = stellarPageNumber(raw)
		if err != nil {
			s.render404(c)
			return
		}
	}
	snap, err := s.stellarSnapshot(nil)
	if err != nil {
		stellarError(c, err)
		return
	}
	data, err := s.stellarAuthorView(snap, id, page)
	if err != nil {
		s.render404(c)
		return
	}
	s.outputStellar(c, data, http.StatusOK)
}
func (s *webServer) stellarAuthorView(snap *stellarSnapshot, id string, page int) (*tmpl.ViewData, error) {
	author := stellarObject(stellarObject(snap.input.RuntimeData["authors"])[id])
	if len(author) == 0 {
		return nil, errStellarNotFound
	}
	records := []*stellarRecord{}
	for _, r := range snap.records {
		if r.post != nil && (stellarGet(r.config, "article.author") == id || hasTag(stellarTerms(stellarGet(stellarRecordBlog(r), "content.authors")), id)) {
			records = append(records, r)
		}
	}
	data, err := s.stellarListingView(snap, "author", stellarString(author["name"]), stellarProfilePath(snap.input.Theme, "author")+"/"+id, records, page, nil, nil)
	if err != nil {
		return nil, err
	}
	data.Page.Author = id
	data.AuthorProfile = &tmpl.AuthorDef{Name: stellarString(author["name"]), URL: stellarString(author["url"]), Avatar: stellarString(author["avatar"]), Description: stellarString(author["description"])}
	data.Stellar.Input.RuntimeData = stellarCopyObject(data.Stellar.Input.RuntimeData)
	data.Stellar.Input.RuntimeData["authorCover"] = author
	return data, nil
}
func stellarTaxonomyPath(theme map[string]any, kind string) string {
	if path := stellarGet(theme, "profiles.blogIndex."+kind+".path"); path != nil {
		return stellarPath(stellarv2.ProfilePath(path))
	}
	return stellarProfilePath(theme, "blogIndex") + "/" + kind
}
func (s *webServer) serveStellarTaxonomy(c *gin.Context, kind, name string) {
	name, page, err := stellarPagedName(name)
	if err != nil {
		s.render404(c)
		return
	}
	snap, err := s.stellarSnapshot(nil)
	if err != nil {
		stellarError(c, err)
		return
	}
	data, err := s.stellarTaxonomyView(snap, kind, name, page)
	if err != nil {
		s.render404(c)
		return
	}
	s.outputStellar(c, data, http.StatusOK)
}
func stellarTaxonomyTerms(snap *stellarSnapshot, kind string) map[string]int {
	terms := map[string]int{}
	for _, r := range snap.records {
		if r.post == nil || stellarGet(r.model.Item, "visibility.listed") == false {
			continue
		}
		seen := map[string]bool{}
		for _, term := range stellarTerms(r.model.Item[kind]) {
			seen[term] = true
			if kind == "categories" {
				parts := strings.Split(term, "/")
				for i := 1; i < len(parts); i++ {
					seen[strings.Join(parts[:i], "/")] = true
				}
			}
		}
		for term := range seen {
			terms[term]++
		}
	}
	return terms
}
func (s *webServer) stellarTaxonomyView(snap *stellarSnapshot, kind, name string, page int) (*tmpl.ViewData, error) {
	records := []*stellarRecord{}
	terms := stellarTaxonomyTerms(snap, kind)
	for _, r := range snap.records {
		if r.post == nil || stellarGet(r.model.Item, "visibility.listed") == false {
			continue
		}
		matched := name == ""
		for _, term := range stellarTerms(r.model.Item[kind]) {
			if term == name || (kind == "categories" && strings.HasPrefix(term, name+"/")) {
				matched = true
			}
		}
		if matched {
			records = append(records, r)
		}
	}
	if name != "" && terms[name] == 0 {
		return nil, errStellarNotFound
	}
	path := stellarTaxonomyPath(snap.input.Theme, kind)
	if name != "" {
		return s.stellarListingView(snap, "blogIndex", name, path+"/"+name, records, page, nil, nil)
	}
	data := s.stellarAggregate(snap, "blogIndex", map[string]string{"archives": "归档", "categories": "分类", "tags": "标签"}[kind], path, kind)
	for _, r := range records {
		data.Stellar.ListItems = append(data.Stellar.ListItems, r.model)
	}
	names := []string{}
	for name := range terms {
		names = append(names, name)
	}
	sort.Strings(names)
	termItems := []any{}
	nodes := map[string]stellarmodels.Object{}
	for _, key := range names {
		title := key
		parent := ""
		if kind == "categories" {
			if index := strings.LastIndex(key, "/"); index >= 0 {
				title = key[index+1:]
				parent = key[:index]
			}
		}
		item := stellarmodels.Object{"id": key, "name": title, "path": strings.TrimPrefix(path, "/") + "/" + key, "parentId": parent, "count": terms[key], "children": []any{}}
		nodes[key] = item
		if parent == "" {
			termItems = append(termItems, item)
		} else if p := nodes[parent]; p != nil {
			p["children"] = append(stellarArray(p["children"]), item)
		}
	}
	data.Stellar.Index = stellarmodels.Object{"terms": termItems, "type": kind}
	return data, nil
}

func stellarNotice(data *tmpl.ViewData, notice string) {
	data.Content = template.HTML(notice) + data.Content
	if data.Stellar != nil {
		page := stellarCopyObject(data.Stellar.Input.Page)
		page["content"] = string(data.Content)
		data.Stellar.Input.Page = page
	}
}
