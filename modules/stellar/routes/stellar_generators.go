package routes

import (
	"encoding/xml"
	"html"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"blog/internal/post"
	"blog/internal/store"
	"blog/internal/textutil"
	tmpl "blog/modules/stellar/viewmodel"

	"github.com/gin-gonic/gin"
)

type stellarSearchItem struct {
	Title      string            `json:"title,omitempty"`
	Path       string            `json:"path"`
	Content    *string           `json:"content,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	Categories []string          `json:"categories,omitempty"`
	Domains    []string          `json:"domains,omitempty"`
	Anchors    []textutil.Anchor `json:"anchors,omitempty"`
}

func stellarSearchItems(snap *stellarSnapshot) []stellarSearchItem {
	out := []stellarSearchItem{}
	scope := stellarString(stellarGet(snap.input.Theme, "search.local.scope"))
	include := stellarGet(snap.input.Theme, "search.local.includeContent") != false
	records := append([]*stellarRecord(nil), snap.records...)
	sort.SliceStable(records, func(i, j int) bool {
		a, b := records[i], records[j]
		if a.post != nil && b.post == nil {
			return true
		}
		if a.post == nil && b.post != nil {
			return false
		}
		if a.post != nil {
			return stellarString(a.model.Item["date"]) > stellarString(b.model.Item["date"])
		}
		return false
	})
	for _, r := range records {
		if stellarGet(r.model.Item, "visibility.searchable") == false {
			continue
		}
		kind := "page"
		if r.post != nil {
			kind = "post"
		}
		if scope != "all" && scope != kind {
			continue
		}
		layout := stellarString(r.page["layout"])
		if (kind == "post" && layout != "post") || (kind == "page" && layout != "page" && layout != "wiki") {
			continue
		}
		body := stellarString(r.model.Item["content"])
		if body == "" {
			continue
		}
		item := stellarSearchItem{Title: strings.TrimSpace(stellarString(r.model.Item["title"])), Path: stellarPublicPath(snap.site, r.path) + "/", Tags: stellarTerms(r.model.Item["tags"]), Categories: stellarTerms(r.model.Item["categories"])}
		if include {
			content, anchors := textutil.StripHTMLWithAnchors(body)
			item.Content = &content
			item.Anchors = anchors
		}
		if layout == "post" {
			item.Domains = append(item.Domains, "blog")
		}
		if r.profile == "wiki" || r.profile == "topic" || r.profile == "notebook" {
			item.Domains = append(item.Domains, r.profile+":"+r.collectionID)
		}
		out = append(out, item)
	}
	return out
}
func (s *webServer) serveStellarSearchJSON(c *gin.Context) {
	snap, err := s.stellarSnapshot(nil)
	if err != nil {
		stellarError(c, err)
		return
	}
	if stellarGet(snap.input.Theme, "search.provider") != "local" {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, stellarSearchItems(snap))
}

type stellarSearchAPIItem struct {
	Type       string   `json:"type"`
	Title      string   `json:"title"`
	Path       string   `json:"path"`
	Snippet    string   `json:"snippet"`
	AnchorID   string   `json:"anchor_id,omitempty"`
	AnchorText string   `json:"anchor_text,omitempty"`
	Domains    []string `json:"domains,omitempty"`
}
type stellarSearchAPIResponse struct {
	Items []stellarSearchAPIItem `json:"items"`
}

func (s *webServer) serveStellarSearchAPI(c *gin.Context) {
	snap, err := s.stellarSnapshot(nil)
	if err != nil {
		stellarError(c, err)
		return
	}
	if stellarGet(snap.site.StellarV2.Blog, "search.provider") != "go" {
		c.Status(http.StatusNotFound)
		return
	}
	if s.searchIdx == nil {
		c.JSON(http.StatusServiceUnavailable, stellarHTTPError{Error: "server search disabled", Code: "stellar_search_unavailable"})
		return
	}
	query := strings.TrimSpace(c.Query("q"))
	kind := c.Query("type")
	scope := c.DefaultQuery("scope", "all")
	if scope != "all" && scope != "post" && scope != "page" {
		c.JSON(http.StatusBadRequest, stellarHTTPError{Error: "invalid search scope", Code: "stellar_search_scope_invalid"})
		return
	}
	configuredScope := stellarString(stellarGet(snap.input.Theme, "search.local.scope"))
	includeContent := stellarGet(snap.input.Theme, "search.local.includeContent") != false && c.Query("include_content") != "false"
	domain := c.Query("domain")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	response := stellarSearchAPIResponse{Items: []stellarSearchAPIItem{}}
	if query == "" {
		c.JSON(http.StatusOK, response)
		return
	}
	matches, err := s.searchIdx.Search(query, kind, 100)
	if err != nil {
		stellarError(c, err)
		return
	}
	authorized := map[string]bool{}
	for _, m := range matches {
		authorized[m.Type+":"+m.Slug] = true
	}
	terms := strings.Fields(strings.ToLower(query))
	for _, r := range snap.records {
		if stellarGet(r.model.Item, "visibility.searchable") == false {
			continue
		}
		recordKind := "page"
		identity := ""
		if r.post != nil {
			recordKind = "post"
			identity = r.post.Slug
		}
		if r.pageRow != nil {
			identity = r.pageRow.Slug
		}
		if r.doc != nil {
			recordKind = "wiki"
		}
		if r.note != nil {
			recordKind = "note"
		}
		if kind != "" && recordKind != kind {
			continue
		}
		scopeKind := "page"
		if r.post != nil {
			scopeKind = "post"
		}
		if (scope != "all" && scope != scopeKind) || (configuredScope != "" && configuredScope != "all" && configuredScope != scopeKind) {
			continue
		}
		domains := []string{}
		if r.post != nil {
			domains = append(domains, "blog")
		}
		if r.profile == "wiki" || r.profile == "topic" || r.profile == "notebook" {
			domains = append(domains, r.profile+":"+r.collectionID)
		}
		if domain != "" && !hasTag(domains, domain) {
			continue
		}
		title := stellarString(r.model.Item["title"])
		content, anchors := textutil.StripHTMLWithAnchors(stellarString(r.model.Item["content"]))
		metadata := strings.ToLower(title + " " + strings.Join(stellarTerms(r.model.Item["tags"]), " ") + " " + strings.Join(stellarTerms(r.model.Item["categories"]), " "))
		text := metadata
		if includeContent {
			text += " " + strings.ToLower(content)
		} else {
			content, anchors = "", nil
		}
		match := true
		metadataMatch := true
		for _, term := range terms {
			if !strings.Contains(metadata, term) {
				metadataMatch = false
			}
			if !strings.Contains(text, term) {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		if includeContent && !metadataMatch && identity != "" && !authorized[recordKind+":"+identity] {
			continue
		}
		item := stellarSearchAPIItem{Type: recordKind, Title: title, Path: stellarPublicPath(snap.site, r.path), Snippet: html.EscapeString(textutil.Truncate(content, 200)), Domains: domains}
		first := -1
		for _, term := range terms {
			if i := strings.Index(strings.ToLower(content), term); i >= 0 && (first < 0 || i < first) {
				first = i
			}
		}
		if first >= 0 {
			offset := textutil.Utf16Len(content[:first])
			for _, anchor := range anchors {
				if anchor.Offset <= offset {
					item.AnchorID = anchor.ID
					item.AnchorText = anchor.Text
				}
			}
		}
		response.Items = append(response.Items, item)
		if len(response.Items) >= limit {
			break
		}
	}
	if (kind == "" || kind == "moment") && len(response.Items) < limit && scope == "all" && (configuredScope == "" || configuredScope == "all") && domain == "" && includeContent {
		for _, moment := range snap.inventory.moments {
			if !authorized["moment:"+store.MomentSlugKey(moment.ID)] {
				continue
			}
			body, err := s.posts.RenderStellarV2WithContext(moment.ContentMD, post.StellarRenderContext{Theme: snap.input.Theme, Data: snap.input.RuntimeData, BlogExtensions: true})
			if err != nil {
				stellarError(c, err)
				return
			}
			content := textutil.StripHTML(body)
			matched := true
			for _, term := range terms {
				if !strings.Contains(strings.ToLower(content), term) {
					matched = false
					break
				}
			}
			if !matched {
				continue
			}
			response.Items = append(response.Items, stellarSearchAPIItem{Type: "moment", Title: textutil.Truncate(content, 40), Path: stellarPublicPath(snap.site, "/moments"), Snippet: html.EscapeString(textutil.Truncate(content, 200))})
			if len(response.Items) >= limit {
				break
			}
		}
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, response)
}
func (s *webServer) serveStellarSitemap(c *gin.Context) {
	snap, err := s.stellarSnapshot(nil)
	if err != nil {
		stellarError(c, err)
		return
	}
	type sitemapURL struct {
		Loc        string `xml:"loc"`
		LastMod    string `xml:"lastmod,omitempty"`
		ChangeFreq string `xml:"changefreq,omitempty"`
		Priority   string `xml:"priority,omitempty"`
	}
	type sitemapDocument struct {
		XMLName xml.Name     `xml:"urlset"`
		XMLNS   string       `xml:"xmlns,attr"`
		URLs    []sitemapURL `xml:"url"`
	}
	base := s.canonicalBaseURL(&tmpl.ViewData{Site: snap.site}, c)
	doc := sitemapDocument{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	seen := map[string]bool{}
	add := func(path, last string) {
		loc := tmpl.JoinSiteURL(base, path)
		if !seen[loc] {
			doc.URLs = append(doc.URLs, sitemapURL{Loc: loc, LastMod: last, ChangeFreq: "weekly", Priority: "0.6"})
			seen[loc] = true
		}
	}
	add("/", "")
	for _, profile := range []string{"blogIndex", "wikiIndex", "topic", "notebooks"} {
		add(stellarProfilePath(snap.input.Theme, profile), "")
	}
	for _, collection := range snap.collections {
		if stellarGet(collection.model, "visibility.listed") != false {
			add(collection.path, "")
		}
	}
	for _, r := range snap.records {
		if stellarGet(r.model.Item, "visibility.listed") == false || r.config["sitemap"] == false {
			continue
		}
		date := stellarString(r.model.Item["updated"])
		if len(date) > 10 {
			date = date[:10]
		}
		add(r.path, date)
	}
	output, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		stellarError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Cache-Control", "public, max-age=300")
	c.Data(http.StatusOK, "application/xml; charset=utf-8", append([]byte(xml.Header), output...))
}
func (s *webServer) stellarFeedRecords() (*stellarSnapshot, []*stellarRecord, error) {
	snap, err := s.stellarSnapshot(nil)
	if err != nil {
		return nil, nil, err
	}
	out := []*stellarRecord{}
	for _, r := range snap.records {
		if r.post != nil && stellarGet(r.model.Item, "visibility.listed") != false {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return stellarString(out[i].model.Item["date"]) > stellarString(out[j].model.Item["date"])
	})
	return snap, out, nil
}
func (s *webServer) serveStellarFeed(c *gin.Context, format, taxonomy, name string) {
	snap, records, err := s.stellarFeedRecords()
	if err != nil {
		stellarError(c, err)
		return
	}
	filtered := []*stellarRecord{}
	for _, r := range records {
		if taxonomy != "" && !stellarTaxonomyContains(r.model.Item[taxonomy], taxonomy, name) {
			continue
		}
		filtered = append(filtered, r)
		if len(filtered) >= feedPageSize {
			break
		}
	}
	base := s.canonicalBaseURL(&tmpl.ViewData{Site: snap.site}, c)
	title := snap.site.Title
	if taxonomy != "" {
		title += " — " + map[string]string{"categories": "分类", "tags": "标签"}[taxonomy] + "：" + name
	}
	now := time.Now().UTC().Format(time.RFC3339)
	feedURL := tmpl.JoinSiteURL(base, c.Request.URL.EscapedPath())
	c.Header("Cache-Control", "max-age="+strconv.Itoa(feedCacheAge))
	if format == "json" {
		items := []jsonFeedItem{}
		for _, r := range filtered {
			link := tmpl.JoinSiteURL(base, r.path)
			items = append(items, jsonFeedItem{ID: link, URL: link, Title: stellarString(r.model.Item["title"]), DatePublished: stellarString(r.model.Item["date"]), ContentHTML: stellarString(r.model.Item["content"]), Author: s.resolveAuthor(stellarString(stellarGet(r.config, "article.author")))})
		}
		c.JSON(http.StatusOK, jsonFeed{Version: "https://jsonfeed.org/version/1.1", Title: title, HomePageURL: tmpl.JoinSiteURL(base, "/"), FeedURL: feedURL, Description: snap.site.Description, Items: items})
		return
	}
	var document any
	contentType := "application/rss+xml; charset=utf-8"
	if format == "atom" {
		entries := []atomEntry{}
		for _, r := range filtered {
			link := tmpl.JoinSiteURL(base, r.path)
			entries = append(entries, atomEntry{Title: stellarString(r.model.Item["title"]), Link: atomLink{Href: link}, ID: link, Updated: stellarString(r.model.Item["updated"]), Published: stellarString(r.model.Item["date"]), Content: atomContent{Type: "html", Body: stellarString(r.model.Item["content"])}, Author: &atomAuthor{Name: s.resolveAuthor(stellarString(stellarGet(r.config, "article.author")))}})
		}
		if len(filtered) > 0 {
			now = stellarString(filtered[0].model.Item["updated"])
		}
		document = atomFeed{XMLNS: "http://www.w3.org/2005/Atom", Title: title, ID: tmpl.JoinSiteURL(base, "/"), Updated: now, Link: []atomLink{{Href: feedURL, Rel: "self"}, {Href: tmpl.JoinSiteURL(base, "/")}}, Entries: entries}
		contentType = "application/atom+xml; charset=utf-8"
	} else {
		items := []rssItem{}
		for _, r := range filtered {
			link := tmpl.JoinSiteURL(base, r.path)
			date, _ := time.Parse(time.RFC3339Nano, stellarString(r.model.Item["date"]))
			items = append(items, rssItem{Title: stellarString(r.model.Item["title"]), Link: link, Guid: link, Description: stellarString(r.model.Item["content"]), PubDate: date.Format(time.RFC1123Z), Author: s.resolveAuthor(stellarString(stellarGet(r.config, "article.author")))})
		}
		document = rssFeed{Version: "2.0", Channel: rssChannel{Title: title, Link: tmpl.JoinSiteURL(base, "/"), Description: snap.site.Description, Items: items}}
	}
	output, err := xml.MarshalIndent(document, "", "  ")
	if err != nil {
		stellarError(c, err)
		return
	}
	c.Data(http.StatusOK, contentType, append([]byte(xml.Header), output...))
}

func stellarTaxonomyContains(raw any, kind, name string) bool {
	for _, term := range stellarTerms(raw) {
		if term == name || (kind == "categories" && strings.HasPrefix(term, name+"/")) {
			return true
		}
	}
	return false
}
