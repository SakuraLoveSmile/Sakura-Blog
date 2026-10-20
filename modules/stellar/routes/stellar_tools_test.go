package routes

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
	"testing"
	"time"

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
	site := *ws.site.Load()
	version := *site.StellarV2
	version.Theme = stellarCopyObject(version.Theme)
	version.Theme["footer"] = map[string]any{"content": "Footer {% blog_ghrepo xaoxuu/hexo-theme-stellar %}"}
	version.Theme["rightbar"] = map[string]any{"enabled": true, "widgets": []any{"offline-widget"}}
	profiles := stellarCopyObject(stellarObject(version.Theme["profiles"]))
	profile := stellarCopyObject(stellarObject(profiles["post"]))
	profile["rightbar"] = map[string]any{"enabled": true, "widgets": []any{"offline-widget"}}
	profiles["post"] = profile
	version.Theme["profiles"] = profiles
	version.Blog = stellarCopyObject(version.Blog)
	version.Blog["content"] = map[string]any{"enabled": true}
	data := stellarCopyObject(stellarObject(version.Blog["data"]))
	data["widgets"] = map[string]any{"offline-widget": map[string]any{"layout": "markdown", "content": "Widget {% blog_ghuser xaoxuu %}"}}
	version.Blog["data"] = data
	site.StellarV2 = &version
	ws.site.Store(&site)
	network := &stellarNoNetwork{}
	ws.posts.SetGHClient(network)
	engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
	if err != nil {
		t.Fatal(err)
	}
	footer, widget := false, false
	engine.SetStellarMarkdownRender(func(body string, ctx post.StellarRenderContext) (string, error) {
		if !ctx.Offline {
			t.Error("template Markdown was not Offline")
		}
		footer = footer || strings.HasPrefix(body, "Footer ")
		widget = widget || strings.HasPrefix(body, "Widget ")
		return ws.posts.RenderStellarV2WithContext(body, ctx)
	})
	capture := &stellarOfflineSnapshotCapture{t: t, renderer: engine}
	ws.engine = capture
	pages, err := ws.renderSnapshots(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) == 0 || len(pages) != capture.count {
		t.Fatalf("snapshot render count=%d, pages=%d", capture.count, len(pages))
	}
	if !footer || !widget || network.calls != 0 {
		t.Fatalf("offline footer=%v widget=%v network callbacks=%d", footer, widget, network.calls)
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

func TestStellarToolSnapshotsUseActualPageBaseURL(t *testing.T) {
	ws, st, _, _ := stellarRouteFixture(t)
	for _, slug := range []string{"one", "two", "three"} {
		createStellarPost(t, st, slug, "")
	}
	site := *ws.site.Load()
	site.URL = "https://images.example/sub"
	version := *site.StellarV2
	version.Theme = stellarCopyObject(version.Theme)
	version.Theme["canonical"] = map[string]any{"host": "mirror.example"}
	site.StellarV2 = &version
	ws.site.Store(&site)
	engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
	if err != nil {
		t.Fatal(err)
	}
	pages, err := RenderToolSnapshots(context.Background(), Dependencies{Site: ws.site, Store: st, Posts: ws.posts, Engine: engine, Icons: ws.icons})
	if err != nil {
		t.Fatal(err)
	}
	bases := map[string]string{}
	documents := map[string]string{}
	for _, page := range pages {
		bases[page.Path] = page.BaseURL
		documents[page.Path] = page.HTML
		if strings.Contains(page.BaseURL, "mirror.example") || strings.Contains(page.BaseURL, "/sub/sub/") {
			t.Fatalf("snapshot base uses a mirror or duplicated root: %s", page.BaseURL)
		}
	}
	for path, want := range map[string]string{
		"index.html":             "https://images.example/sub/",
		"post/one/index.html":    "https://images.example/sub/post/one",
		"blog/page/2/index.html": "https://images.example/sub/blog/page/2",
	} {
		if bases[path] != want {
			t.Errorf("%s BaseURL=%q, want %q", path, bases[path], want)
		}
		canonical := strings.Replace(want, "https://images.example", "https://mirror.example", 1)
		if !strings.Contains(documents[path], `rel="canonical" href="`+canonical+`"`) {
			t.Errorf("snapshot %s canonical did not honor canonical.host: %s", path, canonical)
		}
	}
}

func TestStellarContentPreviewUsesActualOriginAndStrictMembership(t *testing.T) {
	ws, st, _, _ := stellarRouteFixture(t)
	createStellarPost(t, st, "one", "")
	site := *ws.site.Load()
	site.URL = "https://canonical.example/sub"
	version := *site.StellarV2
	version.Theme = stellarCopyObject(version.Theme)
	version.Theme["canonical"] = map[string]any{"host": "different.example"}
	site.StellarV2 = &version
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
	for _, want := range []string{`data-stellar-preview-token="test-generation"`, `http://127.0.0.1:8109/sub/assets/js/blog-preview.js`, `data-stellar-script="defines"`, `data-page-layout="post"`, `rel="canonical" href="http://127.0.0.1:8109/sub/post/one"`, "Current content"} {
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
	input.Mode = content.StellarPreviewModeContent
	contentPreview, err := ws.renderContentPreviewSnapshot(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Current content", `class="md-text content`, `data-stellar-preview-content`} {
		if !strings.Contains(contentPreview.Document, want) {
			t.Errorf("content preview missing %q", want)
		}
	}
	for _, omit := range []string{"data-stellar-preview-body", "article-tags", "data-blog-expired", "ai-summary"} {
		if strings.Contains(contentPreview.Document, omit) {
			t.Errorf("content preview leaked host content %q", omit)
		}
	}
	if contentPreview.HTML != input.HTML {
		t.Fatalf("content response body changed: %q", contentPreview.HTML)
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

func TestStellarContentPreviewRendersCandidateAtHiddenPostPaths(t *testing.T) {
	ws, st, _, router := stellarRouteFixture(t)
	for _, slug := range []string{"published", "draft", "locked", "deleted", "future"} {
		row := createStellarPost(t, st, slug, "")
		if slug != "published" {
			row.Title = "PRIVATE_RECORD_" + slug
			row.ContentMD = "PRIVATE_RECORD_BODY_" + slug
			row.StellarConfig = `{"title":"PRIVATE_RECORD_` + slug + `","tags":["PRIVATE_RECORD_TAG_` + slug + `"]}`
		}
		switch slug {
		case "draft":
			row.Status = store.StatusDraft
		case "locked":
			row.Password = "secret"
		case "future":
			row.PublishAt = time.Date(2999, time.January, 1, 0, 0, 0, 0, time.UTC)
		}
		if err := st.Update(slug, row); err != nil {
			t.Fatal(err)
		}
		if slug == "deleted" {
			if err := st.Delete(slug); err != nil {
				t.Fatal(err)
			}
		}
	}
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
	markdown := "## Candidate heading\n\n{% copy CANDIDATE_TAG_CONTENT %}"
	for _, slug := range []string{"new", "published", "draft", "locked", "deleted", "future"} {
		for _, mode := range []content.StellarPreviewMode{content.StellarPreviewModeContent, content.StellarPreviewModeDocument} {
			t.Run(slug+"/"+string(mode), func(t *testing.T) {
				input := content.StellarPreviewInput{
					Mode: mode, StellarVersion: 2, StellarConfig: `{"title":"Candidate title"}`,
					Markdown: &markdown, Title: "Candidate title", Layout: "post", Path: "/sub/post/" + slug,
					RequestURL: "http://127.0.0.1:8109/sub/console/posts", PreviewToken: "candidate-preview",
				}
				out, err := ws.renderContentPreviewSnapshot(input)
				if err != nil {
					t.Fatal(err)
				}
				for _, rendered := range []string{out.HTML, out.Document} {
					if !strings.Contains(rendered, "CANDIDATE_TAG_CONTENT") || !strings.Contains(rendered, "Candidate heading") || strings.Contains(rendered, "{% copy") {
						t.Fatal("preview did not render the submitted candidate Markdown")
					}
					for _, secret := range []string{"PRIVATE_RECORD_", "HIDDEN_AUTH_SECRET", "HIDDEN_MAIL_SECRET"} {
						if strings.Contains(rendered, secret) {
							t.Fatalf("preview leaked %s", secret)
						}
					}
				}
			})
		}
	}
	for _, slug := range []string{"draft", "locked", "deleted", "future"} {
		response := stellarRequest(router, "/post/"+slug)
		want := http.StatusNotFound
		if slug == "locked" {
			want = http.StatusOK
		}
		if response.Code != want || strings.Contains(response.Body.String(), "PRIVATE_RECORD_") {
			t.Errorf("public access to %s: status=%d, want=%d, hidden content must remain private", slug, response.Code, want)
		}
	}
	if network.calls != 0 {
		t.Fatalf("preview made %d network callbacks", network.calls)
	}
}

func TestStellarPreviewDocumentModes(t *testing.T) {
	document := `<!doctype html><html><head><link rel="stylesheet" href="/assets/css/main.css"><link rel="manifest" href="/manifest.json"></head><body><div class="site-shell"><header>SHELL_ONLY</header><main id="main"><article class="md-text content"><div>HOST_SUMMARY_ONLY</div><div data-stellar-preview-body="preview-token">BODY_ONLY<article class="md-text content">NESTED_ARTICLE_ONLY</article></div><div>ARTICLE_TAGS_ONLY</div></article><footer>FOOTER_ONLY<article class="md-text content">FOOTER_ARTICLE_ONLY</article></footer></main><aside id="rightbar-region"><article class="md-text content">RIGHTBAR_ARTICLE_ONLY</article></aside><section class="banner">BANNER_ONLY</section><div id="comments">COMMENTS_ONLY</div></div><div class="site-scripts"><script src="/assets/js/runtime.js"></script></div></body></html>`
	for _, tc := range []struct {
		mode content.StellarPreviewMode
		want []string
		omit []string
	}{
		{mode: content.StellarPreviewModeDocument, want: []string{"SHELL_ONLY", "BODY_ONLY", "NESTED_ARTICLE_ONLY", "HOST_SUMMARY_ONLY", "ARTICLE_TAGS_ONLY", "FOOTER_ARTICLE_ONLY", "RIGHTBAR_ARTICLE_ONLY", "BANNER_ONLY", "FOOTER_ONLY", "COMMENTS_ONLY"}},
		{mode: content.StellarPreviewModeContent, want: []string{"BODY_ONLY", "NESTED_ARTICLE_ONLY", "main.css", "manifest.json", "runtime.js", "blog-preview.js", "data-stellar-preview-content"}, omit: []string{"HOST_SUMMARY_ONLY", "ARTICLE_TAGS_ONLY", "SHELL_ONLY", "BANNER_ONLY", "FOOTER_ONLY", "FOOTER_ARTICLE_ONLY", "RIGHTBAR_ARTICLE_ONLY", "COMMENTS_ONLY", "data-stellar-preview-body"}},
	} {
		out, err := stellarPreviewDocument(document, "https://example.test", "preview-token", tc.mode)
		if err != nil {
			t.Fatalf("mode %s: %v", tc.mode, err)
		}
		for _, want := range tc.want {
			if !strings.Contains(out, want) {
				t.Errorf("mode %s missing %q: %s", tc.mode, want, out)
			}
		}
		for _, omit := range tc.omit {
			if strings.Contains(out, omit) {
				t.Errorf("mode %s leaked %q: %s", tc.mode, omit, out)
			}
		}
	}
	defaultDocument, err := stellarPreviewDocument(document, "https://example.test", "preview-token", "")
	if err != nil || !strings.Contains(defaultDocument, "SHELL_ONLY") || !strings.Contains(defaultDocument, "BODY_ONLY") {
		t.Fatalf("empty mode must retain complete document preview: err=%v document=%s", err, defaultDocument)
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
	input := content.StellarPreviewInput{StellarVersion: 2, StellarConfig: `{}`, Layout: "post", Title: "Widget context", Path: "/post/__preview__", RequestURL: "http://127.0.0.1:8110/console/settings", PreviewToken: "widget-generation", HTML: "<p>BODY_SHOULD_NOT_RENDER</p>", Mode: content.StellarPreviewModeWidget, WidgetConfig: `{"layout":"markdown","title":"Preview widget","content":"**Actual widget body**"}`}
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
