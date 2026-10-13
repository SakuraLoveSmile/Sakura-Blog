package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"blog/internal/post"
	"blog/internal/settings"
	"blog/internal/stellarv2"
	"blog/internal/store"
	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
)

type stellarCapture struct{ last *tmpl.ViewData }

func (r *stellarCapture) Render(_ string, data *tmpl.ViewData) (string, error) {
	r.last = data
	raw, err := json.Marshal(struct {
		Page           *tmpl.Page
		Stellar        *settings.StellarRenderContext
		Content        string
		CommentPageKey string
		CommentCount   int
		Comments       any
	}{data.Page, data.Stellar, string(data.Content), data.CommentPageKey, data.CommentCount, data.Comments})
	return string(raw), err
}
func stellarRouteFixture(t *testing.T) (*webServer, *store.Store, *stellarCapture, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	if err := st.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	site := &tmpl.Site{Title: "严格站点", URL: "https://fixture.example", Authors: map[string]tmpl.AuthorDef{"alice": {Name: "Alice"}}, StellarV2: &settings.StellarV2Config{Theme: stellarv2.DefaultTheme(), Blog: map[string]any{"site": map[string]any{"per_page": 2}, "search": map[string]any{"provider": "go"}, "data": map[string]any{"wiki": []any{"docs"}, "authors": map[string]any{"alice": map[string]any{"name": "Alice", "password": "HIDDEN_AUTH_SECRET"}}}, "native_comments": map[string]any{"enabled": true, "title": "本地评论", "notify": map[string]any{"password": "HIDDEN_MAIL_SECRET"}}}}}
	ptr := &atomic.Pointer[tmpl.Site]{}
	ptr.Store(site)
	renderer := &stellarCapture{}
	ws := newWebServer(webDeps{Site: ptr, Store: st, Posts: post.NewService(""), Engine: renderer})
	ws.searchIdx = setupSearchIndex(st)
	r := gin.New()
	r.Use(ws.stellarDynamicRoutes)
	ws.registerHomeRoutes(r)
	ws.registerPostRoutes(r)
	ws.registerTaxonomyRoutes(r)
	ws.registerSearchRoutes(r)
	ws.registerFeedRoutes(r)
	r.NoRoute(ws.render404)
	return ws, st, renderer, r
}
func stellarRequest(r *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}
func createStellarPost(t *testing.T, st *store.Store, slug string, options string) *store.Post {
	t.Helper()
	raw := `{"title":"` + slug + `","tags":["Go"],"categories":["工程/服务"]` + options + `}`
	row := &store.Post{Slug: slug, Title: slug, Status: store.StatusPublished, ContentMD: "## 章节\n\n公开正文 " + slug, StellarVersion: 2, StellarConfig: raw}
	if _, err := st.Create(row); err != nil {
		t.Fatal(err)
	}
	return row
}
func TestStellarPublicVersionGateRunsAfterPermission(t *testing.T) {
	ws, st, _, r := stellarRouteFixture(t)
	if _, err := st.Create(&store.Post{Slug: "legacy", Title: "LEGACY_TITLE", Status: store.StatusPublished, ContentMD: "LEGACY_BODY"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Create(&store.Post{Slug: "draft", Title: "DRAFT_TITLE", Status: store.StatusDraft, ContentMD: "DRAFT_BODY"}); err != nil {
		t.Fatal(err)
	}
	locked := &store.Post{Slug: "locked", Title: "LOCKED_TITLE", Password: "pw", Status: store.StatusPublished, ContentMD: "LOCKED_BODY"}
	if _, err := st.Create(locked); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/post/legacy", "/", "/search.json", "/rss.xml", "/sitemap.xml"} {
		w := stellarRequest(r, path)
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "stellar_migration_required") {
			t.Fatalf("%s => %d %s", path, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "LEGACY_BODY") {
			t.Fatal("legacy body leaked")
		}
	}
	w := stellarRequest(r, "/post/draft")
	if w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), "DRAFT_") {
		t.Fatalf("draft=>%d %s", w.Code, w.Body.String())
	}
	w = stellarRequest(r, "/post/locked")
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "LOCKED_") {
		t.Fatalf("password gate leaked=>%d %s", w.Code, w.Body.String())
	}
	current := *ws.site.Load()
	current.StellarV2 = nil
	ws.site.Store(&current)
	w = stellarRequest(r, "/post/draft")
	if w.Code != http.StatusNotFound {
		t.Fatalf("unmigrated site revealed draft status: %d", w.Code)
	}
}
func TestStellarCanonicalRouteModelsVisibilityAndComments(t *testing.T) {
	ws, st, capture, r := stellarRouteFixture(t)
	createStellarPost(t, st, "one", `,"listing":{"priority":1}`)
	createStellarPost(t, st, "two", "")
	createStellarPost(t, st, "unlisted", `,"visibility":{"listed":false,"searchable":false}`)
	locked := createStellarPost(t, st, "locked", "")
	locked.Password = "pw"
	if err := st.Update(locked.Slug, locked); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateComment(&store.Comment{PageKey: "/post/one", Nick: "读者", Email: "PRIVATE_COMMENT_EMAIL", ContentMD: "既有评论", ContentHTML: "<p>既有评论</p>", Status: store.CommentApproved}); err != nil {
		t.Fatal(err)
	}
	w := stellarRequest(r, "/post/one")
	if w.Code != http.StatusOK {
		t.Fatalf("post=>%d %s", w.Code, w.Body.String())
	}
	data := capture.last
	if data.Stellar == nil || data.Stellar.Version != 2 || data.Stellar.Input.Page["title"] != "one" || data.Page.Slug != "one" || data.CommentPageKey != "/post/one" || data.CommentCount != 1 {
		t.Fatalf("invalid strict detail: %#v", data)
	}
	if data.Stellar.NavigationMembers["/post/two"] != `["post","post"]` {
		t.Fatalf("invalid navigation identity: %#v", data.Stellar.NavigationMembers)
	}
	for _, secret := range []string{"HIDDEN_AUTH_SECRET", "HIDDEN_MAIL_SECRET", "PRIVATE_COMMENT_EMAIL", "/post/locked"} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatalf("private value leaked: %s", secret)
		}
	}
	w = stellarRequest(r, "/blog")
	if w.Code != http.StatusOK {
		t.Fatalf("blog=>%d %s", w.Code, w.Body.String())
	}
	if len(capture.last.Stellar.ListItems)+len(capture.last.Stellar.PinnedItems) != 2 {
		t.Fatalf("listed count=%d", len(capture.last.Stellar.ListItems))
	}
	for _, item := range capture.last.Stellar.ListItems {
		if item.Item["title"] == "unlisted" || item.Item["title"] == "locked" {
			t.Fatal("private/unlisted model in listing")
		}
		if item.Collection == nil || item.Item == nil || item.Render == nil {
			t.Fatal("list is not complete page model")
		}
	}
	for _, path := range []string{"/search.json", "/rss.xml", "/feed.json", "/atom.xml", "/sitemap.xml"} {
		w = stellarRequest(r, path)
		if w.Code != http.StatusOK {
			t.Fatalf("%s=>%d %s", path, w.Code, w.Body.String())
		}
		for _, hidden := range []string{"unlisted", "locked", "HIDDEN_MAIL_SECRET"} {
			if strings.Contains(w.Body.String(), hidden) {
				t.Fatalf("%s leaked %s", path, hidden)
			}
		}
	}
	current := *ws.site.Load()
	v2 := *current.StellarV2
	v2.Blog = stellarCopyObject(v2.Blog)
	v2.Blog["native_comments"] = map[string]any{"enabled": false}
	current.StellarV2 = &v2
	ws.site.Store(&current)
	w = stellarRequest(r, "/post/one")
	if w.Code != http.StatusOK || capture.last.CommentCount != 0 {
		t.Fatal("native_comments false ignored")
	}
}
func TestStellarCollectionMembershipNotebookTagAndConfiguredPaths(t *testing.T) {
	ws, st, capture, r := stellarRouteFixture(t)
	wiki := &store.WikiProject{ID: "docs", Visible: true, StellarVersion: 2, StellarConfig: `{"name":"文档","route":{"path":"/docs/"},"navigation":{"tree":{"Z章":["intro"],"A章":["second"]}}}`}
	if err := st.CreateWikiProject(wiki); err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"intro", "second"} {
		if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "docs", Slug: slug, Title: slug, Status: store.StatusPublished, ContentMD: "文档正文", StellarVersion: 2, StellarConfig: `{"collection":{"profile":"wiki","id":"docs"}}`}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "docs", Slug: "draft", Title: "PRIVATE_DOC_TITLE", Status: store.StatusDraft}); err != nil {
		t.Fatal(err)
	}
	nb := &store.Notebook{ID: "journal", StellarVersion: 2, StellarConfig: `{"name":"笔记","route":{"path":"/journal/"},"listing":{"per_page":1,"sort":{"field":"title","direction":"asc"}}}`}
	if err := st.CreateNotebook(nb); err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"alpha", "beta"} {
		if _, err := st.CreateNote(&store.Note{NotebookID: nb.ID, Slug: slug, Title: slug, Status: store.StatusPublished, ContentMD: "笔记正文", Tags: []string{"技术/Go"}, CreatedAt: time.Now().UTC(), StellarVersion: 2, StellarConfig: `{"collection":{"profile":"notebook","id":"journal"},"tags":["技术/Go"]}`}); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"/docs/intro", "/docs/second", "/journal/alpha", "/journal", "/journal/tags/技术/go", "/journal/tags/技术/go/page/2", "/notebooks", "/wiki"} {
		w := stellarRequest(r, path)
		if w.Code != http.StatusOK {
			t.Fatalf("%s=>%d %s", path, w.Code, w.Body.String())
		}
	}
	w := stellarRequest(r, "/docs/intro")
	tree := stellarArray(stellarGet(capture.last.Stellar.Input.CollectionModel, "navigation.tree"))
	if len(tree) != 2 || stellarObject(tree[0])["title"] != "Z章" {
		t.Fatalf("raw section order lost: %#v", tree)
	}
	if strings.Contains(w.Body.String(), "PRIVATE_DOC_TITLE") {
		t.Fatal("draft title in model")
	}
	w = stellarRequest(r, "/journal/tags/技术/go/page/3")
	if w.Code != http.StatusNotFound {
		t.Fatalf("out of page bounds=%d", w.Code)
	}
	snapshot, err := ws.stellarSnapshot(nil)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.byCollection["wiki:docs"].model["profile"] != "wiki" {
		t.Fatal("wrong collection profile")
	}
	bad := &store.Page{Slug: "wrong", Status: store.StatusPublished, ContentMD: "bad", StellarVersion: 2, StellarConfig: `{"collection":{"profile":"wiki","id":"missing"}}`}
	if _, err := st.CreatePage(bad); err != nil {
		t.Fatal(err)
	}
	if w := stellarRequest(r, "/blog"); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("membership unresolved =>%d %s", w.Code, w.Body.String())
	}
}

func TestStellarActualEngineNavigationAndRequestURL(t *testing.T) {
	ws, st, _, r := stellarRouteFixture(t)
	for _, slug := range []string{"one", "two", "three"} {
		createStellarPost(t, st, slug, "")
	}
	site := *ws.site.Load()
	version := *site.StellarV2
	theme := stellarCopyObject(version.Theme)
	features := stellarCopyObject(stellarObject(theme["features"]))
	partial := stellarCopyObject(stellarObject(features["partialNavigation"]))
	partial["enabled"] = true
	features["partialNavigation"] = partial
	theme["features"] = features
	version.Theme = theme
	site.StellarV2 = &version
	site.URL = "https://fixture.example/sub"
	ws.site.Store(&site)
	engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
	if err != nil {
		t.Fatal(err)
	}
	ws.engine = engine
	w := stellarRequest(r, "http://127.0.0.1:8109/sub/post/one")
	if w.Code != http.StatusOK {
		t.Fatalf("detail=>%d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "data-stellar-navigation") || !strings.Contains(w.Body.String(), "stellar-navigation-config") {
		t.Fatal("actual HTML has no partial-navigation markers")
	}
	if strings.Contains(w.Body.String(), "const utils") {
		t.Fatal("legacy global shadows stock utils")
	}
	if !strings.Contains(w.Body.String(), `rel="canonical" href="https://fixture.example/sub/post/one"`) {
		t.Fatal("detail canonical did not preserve actual subpath")
	}

	w = stellarRequest(r, "http://127.0.0.1:8109/sub/blog/page/2")
	if w.Code != http.StatusOK {
		t.Fatalf("page2=>%d %s", w.Code, w.Body.String())
	}
	doc, err := html.Parse(strings.NewReader(w.Body.String()))
	if err != nil {
		t.Fatal(err)
	}
	var config string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "script" {
			for _, attr := range n.Attr {
				if attr.Key == "data-stellar-script" && attr.Val == "defines" && n.FirstChild != nil {
					config = n.FirstChild.Data
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if !strings.Contains(w.Body.String(), `rel="canonical" href="https://fixture.example/sub/blog/page/2"`) {
		t.Fatal("aggregate canonical did not preserve actual subpath")
	}
	config = strings.TrimPrefix(config, "window.stellarClientData=")
	config = strings.TrimSuffix(config, ";")
	var client map[string]any
	if err := json.NewDecoder(strings.NewReader(config)).Decode(&client); err != nil {
		t.Fatalf("defines JSON: %v %s", err, config)
	}
	got := stellarString(stellarGet(client, "canonical.param.permalink"))
	if got != "http://127.0.0.1:8109/sub/blog/page/2" {
		t.Fatalf("actual page2 client URL=%q config=%s", got, config)
	}
}

func TestStellarCustomPrivatePathsPrecedeMigrationErrors(t *testing.T) {
	_, st, _, r := stellarRouteFixture(t)
	if _, err := st.Create(&store.Post{Slug: "legacy", Title: "LEGACY", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	for _, project := range []*store.WikiProject{{ID: "private", Visible: false, StellarVersion: 2, StellarConfig: `{"route":{"path":"/internal"}}`}, {ID: "docs", Visible: true, StellarVersion: 2, StellarConfig: `{"route":{"path":"/manual"}}`}} {
		if err := st.CreateWikiProject(project); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "docs", Slug: "draft", Title: "PRIVATE_DRAFT", Status: store.StatusDraft}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/internal", "/internal/secret", "/manual/draft"} {
		w := stellarRequest(r, path)
		if w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), "PRIVATE_") {
			t.Fatalf("private %s => %d %s", path, w.Code, w.Body.String())
		}
	}
}
func TestStellarDeletedShareSnapshotDoesNotRestoreStaleContent(t *testing.T) {
	ws, st, _, _ := stellarRouteFixture(t)
	row := &store.Post{Slug: "draft", Title: "STALE_DRAFT", Status: store.StatusDraft, StellarVersion: 2, StellarConfig: `{}`, ContentMD: "STALE_BODY"}
	if _, err := st.Create(row); err != nil {
		t.Fatal(err)
	}
	stale := *row
	if err := st.Delete(row.Slug); err != nil {
		t.Fatal(err)
	}
	snap, err := ws.stellarSnapshot(&stale)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range snap.records {
		if r.post != nil && r.post.ID == row.ID {
			t.Fatal("deleted draft was restored from stale share input")
		}
	}
}
func TestStellarBlogProjectionIsExplicitAndPublic(t *testing.T) {
	_, st, capture, r := stellarRouteFixture(t)
	row := createStellarPost(t, st, "extensions", "")
	row.AISummary = "OLD_AI_DECOY"
	row.Poster = `{"headline":"OLD_POSTER_DECOY"}`
	row.BlogExtensions = `{"content":{"poster":{"headline":"公开海报","caption":"说明","api_key":"PRIVATE_POSTER_SECRET"},"authors":["alice"],"ai_summary":"当前摘要","banner_image":"/current.jpg","collection_order":2,"wiki_external_url":"https://example.test/docs","api_key":"PRIVATE_CONTENT_SECRET"},"notify":{"password":"PRIVATE_NOTIFY_SECRET"},"api_keys":{"secret":"PRIVATE_API_SECRET"}}`
	if err := st.Update(row.Slug, row); err != nil {
		t.Fatal(err)
	}
	w := stellarRequest(r, "/post/extensions")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	record := stellarObject(capture.last.Stellar.Input.RuntimeData["recordBlog"])
	if stellarGet(record, "content.poster.headline") != "公开海报" || capture.last.Page.AISummary != "当前摘要" {
		t.Fatal("published extension projection missing")
	}
	if stellarGet(record, "content.ai_summary") != nil {
		t.Fatal("recordBlog included a field outside the public extension contract")
	}
	for _, secret := range []string{"PRIVATE_POSTER_SECRET", "PRIVATE_CONTENT_SECRET", "PRIVATE_NOTIFY_SECRET", "PRIVATE_API_SECRET", "OLD_AI_DECOY", "OLD_POSTER_DECOY"} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatalf("projection leaked %s", secret)
		}
	}
	if len(capture.last.Page.AuthorLinks) != 1 || capture.last.Page.AuthorLinks[0].ID != "alice" {
		t.Fatal("canonical extension authors were not resolved")
	}
	w = stellarRequest(r, "/")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "itemBlogs") || !strings.Contains(w.Body.String(), "公开海报") {
		t.Fatal("list extension projection missing")
	}
}

func TestStellarCollectionDetailsKeepPublicRecordBlog(t *testing.T) {
	_, st, capture, r := stellarRouteFixture(t)
	if err := st.CreateWikiProject(&store.WikiProject{ID: "docs", Visible: true, StellarVersion: 2, StellarConfig: `{"name":"文档","route":{"path":"/docs/"}}`}); err != nil {
		t.Fatal(err)
	}
	extension := `{"content":{"poster":{"headline":"详情海报","topic":"专题","caption":"说明","color":"red","password":"PRIVATE_POSTER_SECRET"},"banner_image":"/detail.jpg","authors":["alice"],"collection_order":2,"wiki_external_url":"https://example.test/docs","password":"PRIVATE_CONTENT_SECRET"},"notify":{"password":"PRIVATE_NOTIFY_SECRET"}}`
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "docs", Slug: "intro", Status: store.StatusPublished, Title: "文档", StellarVersion: 2, StellarConfig: `{"collection":{"profile":"wiki","id":"docs"}}`, BlogExtensions: extension}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateNotebook(&store.Notebook{ID: "journal", StellarVersion: 2, StellarConfig: `{"name":"笔记","route":{"path":"/journal/"}}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateNote(&store.Note{NotebookID: "journal", Slug: "entry", Status: store.StatusPublished, Title: "笔记", StellarVersion: 2, StellarConfig: `{"collection":{"profile":"notebook","id":"journal"}}`, BlogExtensions: extension}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/docs/intro", "/journal/entry"} {
		w := stellarRequest(r, path)
		if w.Code != http.StatusOK {
			t.Fatalf("%s => %d %s", path, w.Code, w.Body.String())
		}
		record := stellarObject(capture.last.Stellar.Input.RuntimeData["recordBlog"])
		if stellarGet(record, "content.poster.headline") != "详情海报" || stellarGet(record, "content.banner_image") != "/detail.jpg" {
			t.Fatalf("%s lost recordBlog: %#v", path, record)
		}
		for _, secret := range []string{"PRIVATE_POSTER_SECRET", "PRIVATE_CONTENT_SECRET", "PRIVATE_NOTIFY_SECRET"} {
			if strings.Contains(w.Body.String(), secret) {
				t.Fatalf("%s leaked %s", path, secret)
			}
		}
	}
}

func TestStellarCollectionOrderAndAuthorConsumers(t *testing.T) {
	ws, st, capture, r := stellarRouteFixture(t)
	for _, item := range []struct{ id, order string }{{"docs", "2"}, {"first", "1"}} {
		if err := st.CreateWikiProject(&store.WikiProject{ID: item.id, Visible: true, StellarVersion: 2, StellarConfig: `{"name":"` + item.id + `"}`, BlogExtensions: `{"content":{"collection_order":` + item.order + `,"wiki_external_url":"https://example.test/` + item.id + `","secret":"PRIVATE_COLLECTION_SECRET"}}`}); err != nil {
			t.Fatal(err)
		}
	}
	row := createStellarPost(t, st, "coauthored", "")
	row.BlogExtensions = `{"content":{"authors":["alice","alice"]}}`
	if err := st.Update(row.Slug, row); err != nil {
		t.Fatal(err)
	}
	site := *ws.site.Load()
	version := *site.StellarV2
	version.Blog = stellarCopyObject(version.Blog)
	data := stellarCopyObject(stellarObject(version.Blog["data"]))
	data["wiki"] = []any{"docs", "first"}
	version.Blog["data"] = data
	site.StellarV2 = &version
	ws.site.Store(&site)
	w := stellarRequest(r, "/wiki")
	if w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	if len(capture.last.Stellar.Collections) != 2 || capture.last.Stellar.Collections[0]["id"] != "first" {
		t.Fatalf("collection_order not consumed: %#v", capture.last.Stellar.Collections)
	}
	blogs := stellarObject(capture.last.Stellar.Input.RuntimeData["collectionBlogs"])
	if stellarGet(stellarObject(blogs["wiki/docs"]), "content.wiki_external_url") != "https://example.test/docs" || strings.Contains(w.Body.String(), "PRIVATE_COLLECTION_SECRET") {
		t.Fatal("collectionBlogs contract or secret filtering failed")
	}
	w = stellarRequest(r, "/author/alice")
	if w.Code != http.StatusOK || len(capture.last.Stellar.ListItems) != 1 || capture.last.Stellar.ListItems[0].Item["title"] != "coauthored" {
		t.Fatalf("extension author aggregation failed: %d %s", w.Code, w.Body.String())
	}
	w = stellarRequest(r, "/post/coauthored")
	if w.Code != http.StatusOK || len(capture.last.Page.AuthorLinks) != 1 {
		t.Fatal("extension author links not deduplicated")
	}
}
