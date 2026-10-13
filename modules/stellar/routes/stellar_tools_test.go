package routes

import (
	"context"
	"encoding/json"
	"html/template"
	"strings"
	"testing"

	"blog/internal/content"
	"blog/internal/post"
	"blog/internal/store"
	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"
)

type stellarNoNetwork struct{ calls int }

func (c *stellarNoNetwork) GetRepo(string, string) (*post.GHRepoInfo, error) {
	c.calls++
	return nil, nil
}
func (c *stellarNoNetwork) GetUser(string) (*post.GHUserInfo, error) { c.calls++; return nil, nil }

type stellarOfflineSnapshotCapture struct {
	t        *testing.T
	renderer PageRenderer
	count    int
}

func (c *stellarOfflineSnapshotCapture) Render(name string, data *tmpl.ViewData) (string, error) {
	c.count++
	if data.Stellar.Input.RuntimeData["offline"] != true {
		c.t.Errorf("snapshot %s did not pass Offline to request-local Markdown consumers", data.Page.Path)
	}
	return c.renderer.Render(name, data)
}

func TestStellarToolSnapshotRenderContextsAreOffline(t *testing.T) {
	ws, st, _, _ := stellarRouteFixture(t)
	createStellarPost(t, st, "one", "")
	engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
	if err != nil {
		t.Fatal(err)
	}
	capture := &stellarOfflineSnapshotCapture{t: t, renderer: engine}
	ws.engine = capture
	pages, err := ws.renderSnapshots(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) == 0 || len(pages) != capture.count {
		t.Fatalf("snapshot render count=%d, pages=%d", capture.count, len(pages))
	}
}

func TestStellarToolSnapshotsAreReadOnlyAndComplete(t *testing.T) {
	ws, st, _, _ := stellarRouteFixture(t)
	if err := st.EnsureUsersTable(); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsurePostAuthorsTable(); err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"one", "two", "three"} {
		createStellarPost(t, st, slug, "")
	}
	network := &stellarNoNetwork{}
	ws.posts.SetGHClient(network)
	row := createStellarPost(t, st, "github", "")
	row.ContentMD = "{% blog_ghrepo xaoxuu/hexo-theme-stellar %}"
	if err := st.Update(row.Slug, row); err != nil {
		t.Fatal(err)
	}
	for _, row := range []*store.Post{{Slug: "draft", Title: "HIDDEN_DRAFT", Status: store.StatusDraft}, {Slug: "locked", Title: "HIDDEN_LOCKED", Password: "secret", Status: store.StatusPublished}} {
		if _, err := st.Create(row); err != nil {
			t.Fatal(err)
		}
	}
	engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
	if err != nil {
		t.Fatal(err)
	}
	st.DB().SetMaxOpenConns(1)
	if _, err := st.DB().Exec("PRAGMA query_only=ON"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = st.DB().Exec("PRAGMA query_only=OFF") })
	pages, err := RenderToolSnapshots(context.Background(), Dependencies{Site: ws.site, Store: st, Posts: ws.posts, Engine: engine, Icons: ws.icons})
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{}
	for _, page := range pages {
		paths[page.Path] = page.HTML
		if strings.Contains(page.HTML, "HIDDEN_") {
			t.Fatalf("private content leaked to %s", page.Path)
		}
	}
	for _, path := range []string{"index.html", "blog/index.html", "blog/page/2/index.html", "post/one/index.html", "blog/categories/index.html", "blog/categories/工程/index.html", "blog/categories/工程/服务/index.html", "author/alice/index.html", "settings/index.html"} {
		if _, ok := paths[path]; !ok {
			t.Errorf("snapshot missing %s", path)
		}
	}
	if network.calls != 0 {
		t.Fatalf("offline snapshot made %d network callbacks", network.calls)
	}
	if !strings.Contains(paths["post/github/index.html"], "ds-ghinfo") {
		t.Fatal("offline Github placeholder missing")
	}
	if _, ok := paths["post/locked/index.html"]; ok {
		t.Fatal("locked snapshot exposed")
	}
}
func TestStellarContentPreviewUsesActualOriginAndStrictMembership(t *testing.T) {
	ws, st, _, _ := stellarRouteFixture(t)
	createStellarPost(t, st, "one", "")
	site := *ws.site.Load()
	site.URL = "https://canonical.example/sub"
	ws.site.Store(&site)
	engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
	if err != nil {
		t.Fatal(err)
	}
	ws.engine = engine
	network := &stellarNoNetwork{}
	ws.posts.SetGHClient(network)
	st.DB().SetMaxOpenConns(1)
	if _, err := st.DB().Exec("PRAGMA query_only=ON"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = st.DB().Exec("PRAGMA query_only=OFF") })
	input := content.StellarPreviewInput{StellarVersion: 2, StellarConfig: `{"title":"Preview"}`, Title: "Preview", Layout: "post", Path: "/post/one", RequestURL: "http://127.0.0.1:8109/sub/console/posts", PreviewToken: "test-generation", HTML: "<h2>Preview body</h2><p>Current content</p>"}
	document, err := ws.renderContentPreview(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`data-stellar-preview-token="test-generation"`, `http://127.0.0.1:8109/sub/assets/js/blog-preview.js`, `data-stellar-script="defines"`, `data-page-layout="post"`, "Current content"} {
		if !strings.Contains(document, want) {
			t.Errorf("preview missing %q", want)
		}
	}
	if strings.Contains(document, `src="https://canonical.example/sub/assets`) {
		t.Fatal("preview uses canonical asset origin")
	}
	if strings.Contains(document, "stellar-navigation-config") {
		t.Fatal("preview includes navigation config")
	}
	if network.calls != 0 {
		t.Fatal("preview made server network callback")
	}
	if ws.site.Load().URL != "https://canonical.example/sub" {
		t.Fatal("preview mutated site")
	}
	input.StellarVersion = 0
	if _, err := ws.renderContentPreview(input); err == nil {
		t.Fatal("legacy preview silently accepted")
	}
	input.StellarVersion = 2
	input.Layout = "wiki"
	input.StellarConfig = `{"collection":{"profile":"wiki","id":"missing"}}`
	if _, err := ws.renderContentPreview(input); err == nil {
		t.Fatal("unknown collection preview accepted")
	}
}

func TestStellarWidgetPreviewUsesCatalogWithoutPersistence(t *testing.T) {
	stellarweb.SetMarkdownRender(func(raw string) template.HTML {
		body, err := post.NewService("").RenderStellarV2WithContext(raw, post.StellarRenderContext{Offline: true})
		if err != nil {
			t.Fatal(err)
		}
		return template.HTML(body)
	})
	t.Cleanup(func() { stellarweb.SetMarkdownRender(nil) })
	ws, _, _, _ := stellarRouteFixture(t)
	engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
	if err != nil {
		t.Fatal(err)
	}
	ws.engine = engine
	before, _ := json.Marshal(ws.site.Load().StellarV2)
	input := content.StellarPreviewInput{StellarVersion: 2, StellarConfig: `{}`, Layout: "post", Title: "Widget context", Path: "/post/__preview__", RequestURL: "http://127.0.0.1:8110/console/settings", PreviewToken: "widget-generation", HTML: "<p>BODY_SHOULD_NOT_RENDER</p>", WidgetConfig: `{"layout":"markdown","title":"Preview widget","content":"**Actual widget body**"}`}
	document, err := ws.renderContentPreview(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Preview widget", "<strong>Actual widget body</strong>", "data-stellar-preview-content", "blog-preview.js", `data-stellar-preview-token="widget-generation"`} {
		if !strings.Contains(document, want) {
			t.Errorf("widget document missing %q document=%s", want, document)
		}
	}
	if strings.Contains(document, "BODY_SHOULD_NOT_RENDER") {
		t.Fatal("widget preview leaked context body")
	}
	after, _ := json.Marshal(ws.site.Load().StellarV2)
	if string(after) != string(before) {
		t.Fatal("widget preview persisted candidate")
	}
	input.WidgetConfig = `{"layout":"future-upstream-layout"}`
	if _, err := ws.renderContentPreview(input); err != nil {
		t.Fatalf("unsupported layout must follow warning/skip: %v", err)
	}
}
func TestStellarCanonicalSettingsPreviewAndLegacyRejection(t *testing.T) {
	ws, st, _, _ := stellarRouteFixture(t)
	createStellarPost(t, st, "first", "")
	site := *ws.site.Load()
	version := *site.StellarV2
	version.RawTheme = map[string]any{}
	site.StellarV2 = &version
	ws.site.Store(&site)
	engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
	if err != nil {
		t.Fatal(err)
	}
	ws.engine = engine
	document, err := ws.renderSitePreview(map[string]json.RawMessage{"site": json.RawMessage(`{"title":"Draft title"}`), "stellar_v2": json.RawMessage(`{"version":2,"theme":{"appearance":{"color_scheme":"light"}}}`)}, "index")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(document, "Draft title") || !strings.Contains(document, `data-theme="light"`) {
		t.Fatal("canonical draft settings missing")
	}
	if ws.site.Load().Title == "Draft title" {
		t.Fatal("draft settings changed live site")
	}
	if _, err := ws.renderSitePreview(map[string]json.RawMessage{"widgets": json.RawMessage(`{}`)}, "index"); err == nil {
		t.Fatal("legacy preview section accepted")
	}
}
func TestStellarSubpathAssetsCORSAndFeeds(t *testing.T) {
	ws, _, _, r := stellarRouteFixture(t)
	site := *ws.site.Load()
	site.URL = "https://fixture.example/sub"
	ws.site.Store(&site)
	if err := ws.registerStaticRoutes(r); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/sub/assets/js/blog-preview.js", "/sub/assets/css/main.css"} {
		response := stellarRequest(r, path)
		if response.Code != 200 || response.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Fatalf("%s asset response=%d cors=%s", path, response.Code, response.Header().Get("Access-Control-Allow-Origin"))
		}
	}
	for _, path := range []string{"/sub/search.json", "/sub/rss.xml", "/sub/sitemap.xml", "/sub/manifest.webmanifest"} {
		response := stellarRequest(r, path)
		if response.Code != 200 {
			t.Fatalf("%s response=%d %s", path, response.Code, response.Body.String())
		}
		if response.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatalf("identity/content route got static CORS: %s", path)
		}
	}
}
