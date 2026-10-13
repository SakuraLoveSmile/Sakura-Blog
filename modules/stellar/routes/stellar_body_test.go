package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"blog/internal/content"
	"blog/internal/post"
	"blog/internal/settings"
	"blog/internal/stellarv2"
	"blog/internal/store"
	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"
	"golang.org/x/net/html"
)

func TestStellarBodyAndDocumentAdaptersKeepStrictRawPageSchema(t *testing.T) {
	ws, st, _, _ := stellarRouteFixture(t)
	engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
	if err != nil {
		t.Fatal(err)
	}
	engine.SetStellarMarkdownRender(ws.posts.RenderStellarV2WithContext)
	ws.engine = engine
	server := &Server{web: ws}
	if err := st.CreateWikiProject(&store.WikiProject{ID: "raw", Visible: true, StellarVersion: 2, StellarConfig: `{"name":"Raw wiki"}`}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateNotebook(&store.Notebook{ID: "raw", StellarVersion: 2, StellarConfig: `{"name":"Raw notebook"}`}); err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{"post", "page", "wiki", "note"} {
		t.Run(profile, func(t *testing.T) {
			page := map[string]any{
				"article": map[string]any{"ai_label": "manual", "paragraph_indent": "always"},
				"footer":  map[string]any{"show_tags": true},
				"inject":  map[string]any{"head_begin": "<!-- RAW_HEAD_BEGIN -->", "head_end": "<!-- RAW_HEAD_END -->", "body_begin": "<!-- RAW_BODY_BEGIN -->", "body_end": "<!-- RAW_BODY_END -->"},
			}
			if profile == "wiki" || profile == "note" {
				collection := "wiki"
				if profile == "note" {
					collection = "notebook"
				}
				page["collection"] = map[string]any{"profile": collection, "id": "raw"}
			}
			markdown := "Raw adapter body"
			body, err := server.RenderStellarMarkdown(content.StellarBodyInput{Markdown: markdown, Page: page})
			if err != nil || !strings.Contains(body, markdown) {
				t.Fatalf("raw body rejected: %v %s", err, body)
			}
			raw, _ := json.Marshal(page)
			input := content.StellarPreviewInput{StellarVersion: 2, StellarConfig: string(raw), BlogExtensions: `{}`, Markdown: &markdown, Title: "Raw preview", Layout: profile, Path: "/raw-preview", RequestURL: "http://127.0.0.1:8109/api/v1/preview/markdown", PreviewToken: "raw-schema-preview"}
			output, err := server.RenderContentPreviewSnapshot(input)
			if err != nil || output.HTML != body {
				t.Fatalf("raw document rejected: %v", err)
			}
			for _, marker := range []string{"RAW_HEAD_BEGIN", "RAW_HEAD_END", "RAW_BODY_BEGIN", "RAW_BODY_END", markdown} {
				if !strings.Contains(output.Document, marker) {
					t.Errorf("document omitted raw %s", marker)
				}
			}
			parsed, err := stellarv2.ParsePage(page, stellarv2.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := server.RenderStellarMarkdown(content.StellarBodyInput{Markdown: markdown, Page: parsed.Value}); err == nil {
				t.Fatal("body adapter accepted normalized camelCase as raw schema")
			}
			invalid, _ := json.Marshal(parsed.Value)
			input.StellarConfig = string(invalid)
			if _, err := server.RenderContentPreviewSnapshot(input); err == nil {
				t.Fatal("document adapter accepted normalized camelCase as raw schema")
			}
		})
	}
}

func TestStellarBodyCallbackSharesPublicWikiAndAbsoluteLinkContext(t *testing.T) {
	ws, st, _, _ := stellarRouteFixture(t)
	for _, project := range []*store.WikiProject{
		{ID: "docs", Visible: true, StellarVersion: 2, StellarConfig: `{"name":"Manual","route":{"path":"/docs"},"navigation":{"tree":["intro"]}}`},
		{ID: "private", Visible: false, StellarVersion: 2, StellarConfig: `{"name":"HIDDEN_PROJECT"}`},
	} {
		if err := st.CreateWikiProject(project); err != nil {
			t.Fatal(err)
		}
	}
	for _, doc := range []*store.WikiDoc{
		{ProjectID: "docs", Slug: "intro", Title: "Public introduction", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{"collection":{"profile":"wiki","id":"docs"}}`},
		{ProjectID: "docs", Slug: "draft", Title: "HIDDEN_DRAFT", Status: store.StatusDraft},
		{ProjectID: "private", Slug: "secret", Title: "HIDDEN_PRIVATE", Status: store.StatusPublished},
	} {
		if err := st.CreateWikidoc(doc); err != nil {
			t.Fatal(err)
		}
	}
	locked := createStellarPost(t, st, "locked", "")
	locked.Password = "pw"
	if err := st.Update(locked.Slug, locked); err != nil {
		t.Fatal(err)
	}
	input := content.StellarBodyInput{Markdown: "{% toc wiki:docs %}\n\n{% link /destination title:Destination %}", Page: map[string]any{}}
	body, err := ws.renderStellarMarkdown(input)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := ws.stellarSnapshotAt(context.Background(), ws.site.Load(), nil, true)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := ws.posts.RenderStellarV2WithContext(input.Markdown, post.StellarRenderContext{Theme: snap.input.Theme, Data: snap.input.RuntimeData, Page: input.Page, Language: stellarLanguage(snap.input, input.Page), BlogExtensions: true})
	if err != nil || body != canonical || !strings.Contains(body, "Public introduction") || !strings.Contains(body, `href="/docs/intro"`) || !strings.Contains(body, "https://fixture.example/destination") {
		t.Fatalf("context mismatch: %v %s", err, body)
	}
	raw, _ := json.Marshal(snap.input.RuntimeData)
	if strings.Contains(string(raw), "HIDDEN_") || strings.Contains(string(raw), "locked") {
		t.Fatalf("private content leaked: %s", raw)
	}
	// 同一个只读快照同时进入 footer 和 Markdown widget，不再从配置袋补树。
	site := *ws.site.Load()
	version := *site.StellarV2
	version.Theme = stellarCopyObject(version.Theme)
	version.Theme["footer"] = map[string]any{"content": input.Markdown}
	version.Theme["rightbar"] = map[string]any{"enabled": true, "widgets": []any{"context"}}
	version.Blog = stellarCopyObject(version.Blog)
	version.Blog["data"] = map[string]any{"wiki": []any{"docs"}, "secret": "HIDDEN_DATA_SECRET", "widgets": map[string]any{"context": map[string]any{"layout": "markdown", "content": input.Markdown}}}
	site.StellarV2 = &version
	ws.site.Store(&site)
	engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	engine.SetStellarMarkdownRender(func(markdown string, ctx post.StellarRenderContext) (string, error) {
		if !strings.Contains(markdown, "toc wiki:docs") {
			return ws.posts.RenderStellarV2WithContext(markdown, ctx)
		}
		calls++
		rendered, err := ws.posts.RenderStellarV2WithContext(markdown, ctx)
		if err != nil || !strings.Contains(rendered, "Public introduction") || !strings.Contains(rendered, "https://fixture.example/destination") {
			t.Fatalf("template context absent: %v %s", err, rendered)
		}
		raw, _ := json.Marshal(ctx.Data)
		if strings.Contains(string(raw), "HIDDEN_") {
			t.Fatalf("template private data leaked: %s", raw)
		}
		return rendered, err
	})
	ws.engine = engine
	snapshots, err := ws.renderSnapshots(context.Background())
	if err != nil || len(snapshots) == 0 || calls < 2 {
		t.Fatalf("template snapshots: %v calls=%d", err, calls)
	}
}

type stellarPreviewMutationRenderer struct {
	renderer PageRenderer
	mutate   func()
}

func (r stellarPreviewMutationRenderer) Render(name string, data *tmpl.ViewData) (string, error) {
	r.mutate()
	return r.renderer.Render(name, data)
}

func TestStellarOpaquePreviewKeepsBodyAndDocumentInOneSnapshot(t *testing.T) {
	ws, st, _, _ := stellarRouteFixture(t)
	if err := st.CreateWikiProject(&store.WikiProject{ID: "docs", Visible: true, StellarVersion: 2, StellarConfig: `{"name":"Manual","route":{"path":"/docs"}}`}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "docs", Slug: "intro", Title: "Public introduction", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{"collection":{"profile":"wiki","id":"docs"}}`}); err != nil {
		t.Fatal(err)
	}
	site := *ws.site.Load()
	version := *site.StellarV2
	version.Theme = stellarCopyObject(version.Theme)
	version.Theme["footer"] = map[string]any{"content": "OLD_FOOTER {% toc wiki:docs %} {% link /destination title:Footer %}"}
	site.StellarV2 = &version
	ws.site.Store(&site)
	engine, err := stellarweb.NewEngineFromEmbed(nil)
	if err != nil {
		t.Fatal(err)
	}
	engine.SetStellarMarkdownRender(ws.posts.RenderStellarV2WithContext)
	ws.engine = stellarPreviewMutationRenderer{renderer: engine, mutate: func() {
		next := site
		next.URL = "https://new-generation.test/"
		newer := version
		newer.Theme = stellarCopyObject(version.Theme)
		newer.Theme["footer"] = map[string]any{"content": "NEW_FOOTER"}
		next.StellarV2 = &newer
		ws.site.Store(&next)
	}}
	markdown := "{% toc wiki:docs %}\n\n{% link /destination title:Body %}\n\n{% mbti INTJ-T 93-72-78-83-51 %}"
	output, err := ws.renderContentPreviewSnapshot(content.StellarPreviewInput{StellarVersion: 2, StellarConfig: `{"lang":"zh-CN"}`, BlogExtensions: `{}`, Markdown: &markdown, Title: "Preview", Layout: "page", Language: "en", Path: "/preview", RequestURL: "http://127.0.0.1:8109/api/v1/preview/markdown", PreviewToken: "single-snapshot"})
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{output.HTML, output.Document} {
		if !strings.Contains(part, "Public introduction") || !strings.Contains(part, "http://127.0.0.1:8109/destination") || !strings.Contains(part, "Architect") || strings.Contains(part, "new-generation") || strings.Contains(part, "NEW_FOOTER") {
			t.Fatalf("preview crossed generations or locales: %s", part)
		}
	}
	if !strings.Contains(output.Document, "OLD_FOOTER") || !strings.Contains(output.Document, `lang="en"`) {
		t.Fatal("document ignored its captured theme/language")
	}
}

func TestStellarEditorBodyCanRepairPublishedInvalidMarkdown(t *testing.T) {
	ws, st, _, _ := stellarRouteFixture(t)
	broken := createStellarPost(t, st, "broken", "")
	broken.ContentMD = "{% tabs %}"
	if err := st.Update(broken.Slug, broken); err != nil {
		t.Fatal(err)
	}
	if _, err := ws.stellarSnapshotAt(context.Background(), ws.site.Load(), nil, true); err == nil || !strings.Contains(err.Error(), "missing endtabs") {
		t.Fatalf("broken public body was accepted: %v", err)
	}
	replacement := "## Repaired\n\nValid content"
	rendered, err := ws.renderStellarMarkdown(content.StellarBodyInput{Markdown: replacement, Page: map[string]any{}})
	if err != nil {
		t.Fatalf("valid repair blocked by prior body: %v", err)
	}
	broken.ContentMD = replacement
	broken.ContentHTML = rendered
	if err := st.Update(broken.Slug, broken); err != nil {
		t.Fatal(err)
	}
	if _, err := ws.stellarSnapshotAt(context.Background(), ws.site.Load(), nil, true); err != nil {
		t.Fatalf("repaired published content remains broken: %v", err)
	}
	other := createStellarPost(t, st, "other", "")
	other.ContentMD = "{% tabs %}"
	if err := st.Update(other.Slug, other); err != nil {
		t.Fatal(err)
	}
	if _, err := ws.renderStellarMarkdown(content.StellarBodyInput{Markdown: "Updated again", Page: map[string]any{}}); err != nil {
		t.Fatalf("unrelated invalid body blocked update: %v", err)
	}
	registry, err := settings.LoadIconsEmbed()
	if err != nil {
		t.Fatal(err)
	}
	ws.icons = registry
	engine, err := stellarweb.NewEngineFromEmbed(registry)
	if err != nil {
		t.Fatal(err)
	}
	engine.SetStellarMarkdownRender(ws.posts.RenderStellarV2WithContext)
	ws.engine = engine
	markdown := "Preview valid candidate"
	output, err := ws.renderContentPreviewSnapshot(content.StellarPreviewInput{StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`, Markdown: &markdown, Title: "Repair", Layout: "post", Path: "/post/broken", RequestURL: "http://127.0.0.1:8109/api/v1/preview/markdown", PreviewToken: "repair-preview"})
	if err != nil || !strings.Contains(output.HTML, markdown) || !strings.Contains(output.Document, markdown) {
		t.Fatalf("unrelated invalid body blocked opaque preview: %v", err)
	}
	if _, err := ws.stellarSnapshotAt(context.Background(), ws.site.Load(), nil, true); err == nil || !strings.Contains(err.Error(), "missing endtabs") {
		t.Fatalf("front end silently hid unrelated broken body: %v", err)
	}
}

func TestStellarPublicContextStillValidatesPublishedSchema(t *testing.T) {
	ws, st, _, _ := stellarRouteFixture(t)
	row := createStellarPost(t, st, "schema", "")
	// 隔离库直接注入损坏元数据；正常 Store.Update 本身已拒绝这种 schema。
	if _, err := st.DB().Exec(`UPDATE posts SET stellar_config=? WHERE slug=?`, `{"style":{}}`, row.Slug); err != nil {
		t.Fatal(err)
	}
	if _, err := ws.renderStellarMarkdown(content.StellarBodyInput{Markdown: "Valid candidate", Page: map[string]any{}}); err == nil {
		t.Fatal("body-free context bypassed published metadata schema")
	}
}

func TestStellarBodyWikiTOCPathsUseSiteRootInNestedAndOpaqueContexts(t *testing.T) {
	for _, siteURL := range []string{"https://fixture.example/sub", "https://fixture.example/sub/", "https://fixture.example"} {
		t.Run(siteURL, func(t *testing.T) {
			ws, st, _, router := stellarRouteFixture(t)
			for _, project := range []*store.WikiProject{
				{ID: "docs", Visible: true, StellarVersion: 2, StellarConfig: `{"name":"Manual","route":{"path":"/wiki/galaxy/"},"navigation":{"tree":{"First":["intro"],"Second":["advanced"]}}}`},
				{ID: "private", Visible: false, StellarVersion: 2, StellarConfig: `{"name":"HIDDEN_PROJECT"}`},
			} {
				if err := st.CreateWikiProject(project); err != nil {
					t.Fatal(err)
				}
			}
			for _, doc := range []*store.WikiDoc{
				{ProjectID: "docs", Slug: "intro", Title: "Introduction", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{"collection":{"profile":"wiki","id":"docs"}}`},
				{ProjectID: "docs", Slug: "advanced", Title: "Advanced", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{"collection":{"profile":"wiki","id":"docs"}}`},
				{ProjectID: "docs", Slug: "draft", Title: "HIDDEN_DRAFT", Status: store.StatusDraft},
				{ProjectID: "private", Slug: "secret", Title: "HIDDEN_PRIVATE", Status: store.StatusPublished},
			} {
				if err := st.CreateWikidoc(doc); err != nil {
					t.Fatal(err)
				}
			}
			markdown := "Body {% toc wiki:docs %}"
			row := createStellarPost(t, st, "nested/entry", "")
			row.ContentMD = markdown
			if err := st.Update(row.Slug, row); err != nil {
				t.Fatal(err)
			}
			if err := st.CreateNotebook(&store.Notebook{ID: "journal", StellarVersion: 2, StellarConfig: `{"name":"Journal","route":{"path":"/notebooks/journal/"}}`}); err != nil {
				t.Fatal(err)
			}
			if _, err := st.CreateNote(&store.Note{NotebookID: "journal", Slug: "entry", Title: "Entry", Status: store.StatusPublished, ContentMD: markdown, StellarVersion: 2, StellarConfig: `{"collection":{"profile":"notebook","id":"journal"},"rightbar":{"enabled":true,"widgets":["context"]}}`}); err != nil {
				t.Fatal(err)
			}
			site := *ws.site.Load()
			site.URL = siteURL
			version := *site.StellarV2
			version.Theme = stellarCopyObject(version.Theme)
			version.Theme["footer"] = map[string]any{"content": "Footer {% toc wiki:docs %}"}
			profiles := stellarCopyObject(stellarObject(version.Theme["profiles"]))
			for _, profile := range []string{"post", "notebook"} {
				cfg := stellarCopyObject(stellarObject(profiles[profile]))
				cfg["rightbar"] = map[string]any{"enabled": true, "widgets": []any{"context"}}
				profiles[profile] = cfg
			}
			version.Theme["profiles"] = profiles
			version.Blog = stellarCopyObject(version.Blog)
			data := stellarCopyObject(stellarObject(version.Blog["data"]))
			data["widgets"] = map[string]any{"context": map[string]any{"layout": "markdown", "content": "Widget {% toc wiki:docs %}"}}
			version.Blog["data"] = data
			site.StellarV2 = &version
			ws.site.Store(&site)
			root := strings.TrimRight(mustStellarURL(siteURL).EscapedPath(), "/")
			want := []string{root + "/wiki/galaxy/intro", root + "/wiki/galaxy/advanced"}
			checkTOC := func(fragment, base string, wantCount int) {
				t.Helper()
				doc, err := html.Parse(strings.NewReader(fragment))
				if err != nil {
					t.Fatal(err)
				}
				count := 0
				var walk func(*html.Node)
				walk = func(n *html.Node) {
					if n.Type == html.ElementNode && n.Data == "a" {
						class, href := "", ""
						for _, attr := range n.Attr {
							if attr.Key == "class" {
								class = attr.Val
							}
							if attr.Key == "href" {
								href = attr.Val
							}
						}
						if class == "list-link" {
							expected := want[count%len(want)]
							if href != expected {
								t.Errorf("TOC href=%q want=%q at %s", href, expected, base)
							}
							origin, _ := url.Parse(base)
							ref, err := url.Parse(href)
							if err != nil || origin.ResolveReference(ref).EscapedPath() != expected {
								t.Errorf("TOC does not resolve to public path at %s: %q", base, href)
							}
							count++
						}
					}
					for c := n.FirstChild; c != nil; c = c.NextSibling {
						walk(c)
					}
				}
				walk(doc)
				if count != wantCount*len(want) {
					t.Errorf("TOC links=%d want=%d at %s", count, wantCount*len(want), base)
				}
				if strings.Contains(fragment, "HIDDEN_") {
					t.Fatal("TOC leaked filtered Wiki content")
				}
			}
			engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
			if err != nil {
				t.Fatal(err)
			}
			footerCalls, widgetCalls := 0, 0
			engine.SetStellarMarkdownRender(func(body string, ctx post.StellarRenderContext) (string, error) {
				rendered, err := ws.posts.RenderStellarV2WithContext(body, ctx)
				if strings.Contains(body, "toc wiki:docs") {
					checkTOC(rendered, "http://127.0.0.1:8109"+root+"/console/pages/edit", 1)
				}
				if strings.HasPrefix(body, "Footer ") {
					footerCalls++
				}
				if strings.HasPrefix(body, "Widget ") {
					widgetCalls++
				}
				return rendered, err
			})
			ws.engine = engine
			snap, err := ws.stellarSnapshotAt(context.Background(), &site, nil, true)
			if err != nil {
				t.Fatal(err)
			}
			project := stellarObject(stellarGet(snap.input.RuntimeData, "wiki.tree.docs"))
			if stellarString(stellarGet(project, "homepage.path")) != want[0] {
				t.Errorf("runtime homepage=%v want=%s", stellarGet(project, "homepage.path"), want[0])
			}
			flat := stellarArray(project["pages"])
			if len(flat) != len(want) {
				t.Fatalf("flattened public pages=%d want=%d", len(flat), len(want))
			}
			for i, page := range flat {
				if stellarString(stellarObject(page)["path"]) != want[i] {
					t.Errorf("flat page path=%v want=%s", stellarObject(page)["path"], want[i])
				}
			}
			raw, _ := json.Marshal(snap.input.RuntimeData)
			if strings.Contains(string(raw), "HIDDEN_") {
				t.Fatal("body tree retained private/draft metadata")
			}
			formal := snap.byCollection["wiki:docs"].state
			if stellarString(stellarGet(formal, "homepage.path")) != "wiki/galaxy/intro" {
				t.Fatal("body projection changed formal Wiki state")
			}
			for _, path := range []string{"/post/nested/entry", "/notebooks/journal/entry"} {
				response := stellarRequest(router, root+path)
				if response.Code != http.StatusOK {
					t.Fatalf("%s => %d %s", path, response.Code, response.Body.String())
				}
				checkTOC(response.Body.String(), siteURL+path, 3)
				config, layout := `{}`, "post"
				if strings.HasPrefix(path, "/notebooks/") {
					config, layout = `{"collection":{"profile":"notebook","id":"journal"},"rightbar":{"enabled":true,"widgets":["context"]}}`, "page"
				}
				output, err := ws.renderContentPreviewSnapshot(content.StellarPreviewInput{StellarVersion: 2, StellarConfig: config, BlogExtensions: `{}`, Markdown: &markdown, Title: "Preview", Layout: layout, Path: root + path, RequestURL: "http://127.0.0.1:8109" + root + "/api/v1/preview/markdown", PreviewToken: "root-path-preview"})
				if err != nil {
					t.Fatal(err)
				}
				checkTOC(output.HTML, "http://127.0.0.1:8109"+root+"/console/pages/edit", 1)
				checkTOC(output.Document, "http://127.0.0.1:8109"+root+"/console/pages/edit", 3)
			}
			if footerCalls < 4 || widgetCalls < 4 {
				t.Errorf("footer/widget contexts not exercised: %d/%d", footerCalls, widgetCalls)
			}
		})
	}
}
