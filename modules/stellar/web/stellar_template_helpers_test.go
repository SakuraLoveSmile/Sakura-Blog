package web

import (
	"bytes"
	"encoding/json"
	"html/template"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"blog/internal/settings"
	"blog/internal/stellarmodels"
	"blog/internal/stellarv2"
	"blog/modules/stellar/clientconfig"
	"golang.org/x/net/html"
)

func stellarTemplateFixture(t *testing.T) *StellarRenderData {
	t.Helper()
	theme := stellarv2.DefaultTheme()
	client, err := clientconfig.Build(clientconfig.Input{Theme: theme, Root: "/", PageURL: "https://example.test/post/test/", SiteURL: "https://example.test", Language: "zh-CN", Profile: "page"})
	if err != nil {
		t.Fatal(err)
	}
	return &StellarRenderData{ViewData: &settings.ViewData{Site: &settings.Site{Title: "Test Site", Description: "Test description", URL: "https://example.test"}, Page: &settings.Page{Title: "Title"}}, Stellar: &StellarPresentation{Theme: theme, Client: client, Regions: map[string]any{}, Data: map[string]any{}, Widgets: map[string][]StellarWidget{}, RegionBodies: map[string]template.HTML{}, Root: "/", Language: "zh-CN", ColorScheme: "auto", Profile: "page", PageType: "content", Layout: "page"}}
}

// Canonical DOM comparison ignores formatting and attribute order, while
// retaining structure, content, classes, and all runtime-facing attributes.
func stellarCanonicalDOM(source string) string {
	doc, _ := html.Parse(strings.NewReader(source))
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		sort.Slice(n.Attr, func(i, j int) bool { return n.Attr[i].Key < n.Attr[j].Key })
		for c := n.FirstChild; c != nil; {
			next := c.NextSibling
			if c.Type == html.TextNode && strings.TrimSpace(c.Data) == "" {
				n.RemoveChild(c)
			} else {
				walk(c)
			}
			c = next
		}
	}
	walk(doc)
	var b bytes.Buffer
	_ = html.Render(&b, doc)
	return b.String()
}
func TestStellarOfficialCollectionDOMOracle(t *testing.T) {
	var fixture struct {
		Commit string
		Cases  []struct {
			Options map[string]any
			HTML    string
		}
	}
	b, err := os.ReadFile("stellar_template_helpers_testdata/collection.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Commit != "9d11978a24fe6610e3a7dbab741cf62634539d05" {
		t.Fatal("unlocked oracle")
	}
	d := stellarTemplateFixture(t)
	r := stellarTemplateRenderer{}
	for i, c := range fixture.Cases {
		got := stellarCanonicalDOM(string(r.collection(d, c.Options)))
		want := stellarCanonicalDOM(c.HTML)
		if got != want {
			t.Fatalf("case %d\ngot %s\nwant %s", i, got, want)
		}
	}
}
func TestStellarShellUsesPreparedRegionsAndManagedScripts(t *testing.T) {
	d := stellarTemplateFixture(t)
	d.Stellar.RegionBodies["leftbar"] = `<header class="brand-header">Brand</header>`
	d.Stellar.RegionNames = "leftbar"
	d.Content = `<p>Body</p>`
	engine, err := NewEngineFromEmbed(nil)
	if err != nil {
		t.Fatal(err)
	}
	source, err := engine.RenderValue("layout/stellar", d)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for _, a := range n.Attr {
				if a.Key == "id" {
					order = append(order, a.Val)
				}
			}
			if n.Data == "script" {
				managed := false
				for _, a := range n.Attr {
					if a.Key == "data-stellar-script" || a.Key == "type" && a.Val == "application/ld+json" {
						managed = true
					}
				}
				if !managed {
					t.Fatalf("unmanaged script %v", n.Attr)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	joined := strings.Join(order, ",")
	if !strings.Contains(joined, "site-cover,start,leftbar-region,main") {
		t.Fatalf("cover/shell ordering: %s", joined)
	}
	if strings.Contains(source, `id="topbar-region"`) || strings.Contains(source, `id="rightbar-region"`) {
		t.Fatal("empty regions rendered")
	}
	if strings.Contains(source, `data-theme="auto"`) {
		t.Fatal("auto must follow CSS media query")
	}
	if strings.Index(source, "utils.js") > strings.Index(source, "main.js") || strings.Index(source, "main.js") > strings.Index(source, "navigation-init.js") {
		t.Fatal("stock core ordering")
	}
	if strings.Contains(source, "const utils") || strings.Contains(source, "blog-shell") {
		t.Fatal("legacy runtime shadowed stock")
	}
	for _, scheme := range []string{"light", "dark"} {
		d.Stellar.ColorScheme = scheme
		source, err := engine.RenderValue("layout/stellar", d)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(source, `data-theme="`+scheme+`"`) {
			t.Fatal("explicit scheme missing")
		}
	}
}
func TestStellarEmptyWidgetsDoNotCreateRegion(t *testing.T) {
	d := stellarTemplateFixture(t)
	d.Stellar.Regions = map[string]any{"topbar": map[string]any{"enabled": true, "brand": false}, "leftbar": map[string]any{"enabled": false}, "rightbar": map[string]any{"enabled": true}}
	d.Stellar.Widgets["rightbar"] = []StellarWidget{{State: map[string]any{"layout": "toc", "instanceId": "rightbar:toc", "item": map[string]any{}}}}
	r := stellarTemplateRenderer{}
	regions, err := r.regions(d)
	if err != nil {
		t.Fatal(err)
	}
	if regions["names"] != "" {
		t.Fatalf("empty region retained: %v", regions)
	}
	d.Content = `<h2 id="a">Heading</h2>`
	regions, err = r.regions(d)
	if err != nil {
		t.Fatal(err)
	}
	if regions["names"] != "rightbar" || !strings.Contains(string(regions["rightbar"].(template.HTML)), `data-widget-id="rightbar:toc"`) {
		t.Fatal("nonempty TOC missing")
	}
}
func TestStellarHeroEffectVideoAndTerminal(t *testing.T) {
	d := stellarTemplateFixture(t)
	r := stellarTemplateRenderer{}
	for _, effect := range []string{"strands", "galaxy", "light-rays", "ferrofluid"} {
		d.Stellar.Cover = map[string]any{"enabled": true, "title": "Hero<&>", "siteName": "Site", "background": map[string]any{"effect": map[string]any{"type": effect, "options": map[string]any{"value": 1}, "runtime": map[string]any{"respectReducedMotion": false}}}, "preview": map[string]any{"commands": []any{map[string]any{"label": "pnpm", "codes": "echo '<x>'\n\nsecond"}, map[string]any{"label": "npm", "codes": "npm install"}}}}
		source := string(r.hero(d))
		for _, fragment := range []string{`data-hero-effect="` + effect + `"`, `/assets/js/runtime/hero-effects/`, `data-effect-resource=`, `data-effect-runtime=`, `role="tablist"`, `aria-selected="true"`, `aria-selected="false"`, `<span>echo &#39;&lt;x&gt;&#39;</span><span>second</span>`} {
			if !strings.Contains(source, fragment) {
				t.Fatalf("%s missing %s", effect, fragment)
			}
		}
		if strings.Contains(source, "respect-reduced-motion") {
			t.Fatal("false motion preference lost")
		}
		d.Stellar.Cover["background"].(map[string]any)["video"] = "/movie.mp4"
		source = string(r.hero(d))
		if strings.Contains(source, "<canvas") || !strings.Contains(source, "<video") {
			t.Fatal("video must take precedence over effect")
		}
	}
}
func TestStellarOfficialCommentsAndNativeIdentityAreSeparate(t *testing.T) {
	d := stellarTemplateFixture(t)
	r := stellarTemplateRenderer{}
	for _, provider := range []string{"artalk", "waline", "twikoo", "giscus", "beaudar", "utterances"} {
		client := d.Stellar.Client.(clientconfig.Bundle)
		client.Comments = clientconfig.CommentsModel{Enabled: true, Title: "", ID: `page<&>`, Service: provider, Options: map[string]any{"data-repo": "owner/repo", "onclick": "bad()"}}
		d.Stellar.Client = client
		source := string(r.comments(d))
		if strings.Count(source, `id="comments"`) != 1 || !strings.Contains(source, provider) {
			t.Fatalf("provider %s wrong container", provider)
		}
		if strings.Contains(source, "onclick") || strings.Contains(source, "cmt-title") {
			t.Fatal("unsafe options or explicitly empty title lost")
		}
	}
	b, err := os.ReadFile("templates/partial/stellar/blog_comments.html")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `id="comments"`) || !strings.Contains(string(b), `id="blog-comments"`) || !strings.Contains(string(b), `id="cmt-body"`) || strings.Contains(string(b), "<script") {
		t.Fatal("native comments duplicate official identity or lifecycle")
	}
}
func TestStellarContentProfilesAndStrictModelRequirement(t *testing.T) {
	d := stellarTemplateFixture(t)
	r := stellarTemplateRenderer{}
	for _, profile := range []string{"post", "wiki", "topic", "notebook"} {
		d.Stellar.Profile = profile
		d.Stellar.Model = nil
		if _, err := r.content(d); err == nil {
			t.Fatalf("%s missing model accepted", profile)
		}
		d.Stellar.Model = &stellarmodels.PageModel{Collection: map[string]any{"profile": profile}, Item: map[string]any{"title": "Title", "content": "<p>Model body</p>", "presentation": map[string]any{}}, Render: map[string]any{"article": map[string]any{"tags": []any{map[string]any{"name": "Tag", "path": "/tags/tag/"}}, "footer": map[string]any{}}}}
		source, err := r.content(d)
		if err != nil || !strings.Contains(string(source), "Model body") {
			t.Fatalf("%s body missing: %v", profile, err)
		}
	}
	d.Stellar.Model = nil
	d.Stellar.Profile = "page"
	for _, layout := range []string{"index", "index_topic", "index_wiki", "notebooks", "notes", "archives", "categories", "tags", "author", "settings", "404", "page"} {
		d.Stellar.Layout = layout
		if _, err := r.content(d); err != nil {
			t.Fatalf("layout %s: %v", layout, err)
		}
	}
}
func TestStellarScriptJSONCannotCloseScript(t *testing.T) {
	value, err := stellarJSON(map[string]any{"title": "</script><script>alert(1)</script>\u2028"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(value), "<") || strings.Contains(string(value), "\u2028") {
		t.Fatal("unsafe script JSON")
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		t.Fatal(err)
	}
}

func TestStellarOfficialTagcloudOracle(t *testing.T) {
	var fixture struct {
		Cases []struct {
			Tags    []map[string]any
			Options map[string]any
			HTML    string
		}
	}
	b, err := os.ReadFile("stellar_template_helpers_testdata/tagcloud.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &fixture); err != nil {
		t.Fatal(err)
	}
	d := stellarTemplateFixture(t)
	r := stellarTemplateRenderer{}
	for i, c := range fixture.Cases {
		d.Stellar.Data["tags"] = c.Tags
		output, err := r.tagcloud(d, c.Options)
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		source := string(output)
		start := strings.Index(source, `<div class="widget-body">`)
		end := strings.LastIndex(source, `</div></widget>`)
		if start < 0 || end < 0 {
			t.Fatal("missing tagcloud body")
		}
		got := stellarCanonicalDOM(source[start+len(`<div class="widget-body">`) : end])
		want := stellarCanonicalDOM(c.HTML)
		if got != want {
			t.Fatalf("case %d\ngot %s\nwant %s", i, got, want)
		}
	}
	d.Stellar.Regions = map[string]any{"leftbar": map[string]any{"enabled": false}, "topbar": map[string]any{"enabled": false}, "rightbar": map[string]any{"enabled": true}}
	d.Stellar.Widgets["rightbar"] = []StellarWidget{{State: map[string]any{"layout": "tagcloud", "item": map[string]any{"color": true}}}}
	if _, err := r.regions(d); err == nil {
		t.Fatal("invalid tagcloud color must reject rendering")
	}
}

func TestStellarBlogCapabilitiesUseCanonicalFlagsAndUniqueContainers(t *testing.T) {
	d := stellarTemplateFixture(t)
	d.Stellar.SearchEnabled = true
	d.Stellar.BlogContent = true
	d.Content = `<h2 id="body">Body</h2>`
	d.Page.AISummary = "Saved summary"
	d.Page.AISummaryOn = false
	d.QAEnabled = false
	d.Stellar.Data["blogReading"] = true
	d.Stellar.Data["blog"] = map[string]any{"content": map[string]any{"share": map[string]any{"enabled": true, "platforms": []any{"wechat", "link", "email"}}, "reward": map[string]any{"enabled": true, "text": "Support", "wechat": "/pay.png"}}, "ai": map[string]any{"summary": map[string]any{"enabled": true, "allow_user_api": true, "api_key": "secret-must-not-leak"}, "ask": map[string]any{"enabled": true, "allow_user_llm": true}}}
	engine, err := NewEngineFromEmbed(nil)
	if err != nil {
		t.Fatal(err)
	}
	source, err := engine.RenderValue("layout/stellar", d)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"share", "reading-progress", "reward-toggle", "ai-summary", "rag-ai-ask"} {
		if strings.Count(source, `id="`+id+`"`) != 1 {
			t.Fatalf("missing or duplicate %s", id)
		}
	}
	if strings.Contains(source, "secret-must-not-leak") {
		t.Fatal("configured secret rendered")
	}
	for _, marker := range []string{`data-platform="link"`, `data-platform="wechat"`, `data-user-api="1"`, `data-rag-ai-key`, `Saved summary`, `/pay.png`} {
		if !strings.Contains(source, marker) {
			t.Fatalf("missing %s", marker)
		}
	}
	d.Stellar.Data["blog"] = map[string]any{}
	d.Stellar.Data["blogReading"] = false
	d.Page.AISummaryOn = true
	d.QAEnabled = true
	source, err = engine.RenderValue("layout/stellar", d)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"ai-summary", "rag-ai-ask", "share", "reward-toggle", "reading-progress"} {
		if strings.Contains(source, `id="`+id+`"`) {
			t.Fatalf("legacy state overrides canonical %s", id)
		}
	}
}

func TestStellarTaxonomyAndArchiveUsePublicProjections(t *testing.T) {
	d := stellarTemplateFixture(t)
	r := stellarTemplateRenderer{}
	d.Stellar.Data["index"] = map[string]any{"terms": []any{map[string]any{"name": "Parent", "path": "/custom/categories/parent/", "count": 3, "children": []any{map[string]any{"name": "Child", "path": "/custom/categories/parent/child/", "count": 2, "parentId": 1}}}}}
	source := string(r.categories(d))
	if !strings.Contains(source, "cat-children") || !strings.Contains(source, "/custom/categories/parent/child/") {
		t.Fatal("category hierarchy/path lost")
	}
	d.Stellar.Data["index"] = map[string]any{"terms": []any{map[string]any{"name": "Tag", "path": "/custom/tags/tag/", "count": 4}}}
	if !strings.Contains(string(r.tags(d)), "/custom/tags/tag/") {
		t.Fatal("tag public path lost")
	}
	d.Stellar.ListItems = []map[string]any{{"render": map[string]any{"listing": map[string]any{"title": "Archive Title", "href": "/custom/post/a/", "date": "2025-03-04T00:00:00Z", "listed": true}}}}
	source = string(r.archives(d))
	if !strings.Contains(source, "2025") || !strings.Contains(source, "03-04") || !strings.Contains(source, "/custom/post/a/") {
		t.Fatal("archive model projection lost")
	}
}

func TestStellarOfficialDateFormatOracle(t *testing.T) {
	var fixture struct {
		Cases []struct{ Value, Locale, Format, Want string }
	}
	b, err := os.ReadFile("stellar_template_helpers_testdata/dates.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &fixture); err != nil {
		t.Fatal(err)
	}
	for i, c := range fixture.Cases {
		date, err := time.Parse(time.RFC3339Nano, c.Value)
		if err != nil {
			t.Fatal(err)
		}
		if got := stellarMomentFormat(date, c.Format, c.Locale); got != c.Want {
			t.Errorf("case %d %s %s\ngot  %s\nwant %s", i, c.Locale, c.Format, got, c.Want)
		}
	}
}

func TestStellarSubdirectoryAndPaginationSEOUseDisplayCanonical(t *testing.T) {
	d := stellarTemplateFixture(t)
	d.Site.URL = "https://example.test/blog/"
	d.Stellar.Root = "/blog/"
	d.Stellar.Profile = "home"
	d.Stellar.Layout = "index"
	d.Page.Path = "/blog/page/2/"
	d.Page.CanonicalURL = "https://canonical.test/blog/page/2/"
	d.Paginator = &settings.Paginator{Current: 2, Total: 3}
	metadata, err := stellarHeadMeta(d)
	if err != nil {
		t.Fatal(err)
	}
	source := string(metadata)
	if !strings.Contains(source, `rel="canonical" href="https://canonical.test/blog/page/2/"`) || !strings.Contains(source, `property="og:url" content="https:&#x2F;&#x2F;canonical.test&#x2F;blog&#x2F;page&#x2F;2&#x2F;"`) || !strings.Contains(source, `"url":"https://canonical.test/blog/page/2/"`) || strings.Contains(source, "/blog/blog/") {
		t.Fatal(source)
	}
	d.Page.CanonicalURL = ""
	if got := stellarPublicPageURL(d); got != "https://example.test/blog/page/2/" {
		t.Fatal(got)
	}
	d.Stellar.Profile = "page"
	d.Stellar.Layout = "page"
	d.Page.Path = "/blog/about/"
	if got := stellarPublicPageURL(d); got != "https://example.test/blog/about/" {
		t.Fatal(got)
	}
	d.Page.CanonicalURL = "https://canonical.test/blog/post/a/"
	original := map[string]any{"@context": "https://schema.org", "@type": "BlogPosting", "url": "https://example.test/blog/blog/post/a/", "mainEntityOfPage": map[string]any{"@id": "https://example.test/blog/blog/post/a/"}}
	d.Stellar.Model = &stellarmodels.PageModel{Render: map[string]any{"seo": map[string]any{"jsonLd": original}}, Item: map[string]any{"route": map[string]any{"path": "/blog/post/a/"}}}
	metadata, err = stellarHeadMeta(d)
	if err != nil {
		t.Fatal(err)
	}
	source = string(metadata)
	if strings.Contains(source, "/blog/blog/") {
		t.Fatal(source)
	}
	if original["url"] != "https://example.test/blog/blog/post/a/" {
		t.Fatal("shared model mutated")
	}
}

func TestStellarGoSearchUsesHostProviderWithoutChangingStrictTheme(t *testing.T) {
	d := stellarTemplateFixture(t)
	d.Stellar.Theme["search"].(map[string]any)["provider"] = nil
	client, err := clientconfig.Build(clientconfig.Input{Theme: d.Stellar.Theme, Root: "/", PageURL: "https://example.test/wiki/docs/", SiteURL: "https://example.test", Language: "zh-CN", Profile: "wiki", Blog: clientconfig.BlogConfig{GoSearch: true}})
	if err != nil {
		t.Fatal(err)
	}
	d.Stellar.Client = client
	d.Stellar.SearchEnabled = true
	d.Stellar.Model = &stellarmodels.PageModel{Collection: map[string]any{"profile": "wiki", "id": "docs", "identity": map[string]any{"name": "Docs"}}}
	r := stellarTemplateRenderer{}
	source := string(r.search(d, "leftbar", true))
	if !strings.Contains(source, `data-shell-action="open-search"`) || !strings.Contains(source, `data-search-domain="wiki:docs"`) || !strings.Contains(source, `data-search-domain-blog="false"`) {
		t.Fatal(source)
	}
	if stellarGet(d.Stellar.Theme, "search.provider") != nil {
		t.Fatal("strict theme was mutated")
	}
}

func TestStellarNativeCommentTextUsesPublicExtensionProjection(t *testing.T) {
	d := stellarTemplateFixture(t)
	d.Site.Comments.Title = "Legacy title"
	d.Site.Comments.AdminBadge = "Legacy badge"
	d.CommentPageKey = "/custom/page/"
	d.Stellar.BlogNativeComments = true
	d.Stellar.Data["nativeComments"] = map[string]any{"title": "Canonical title", "admin_badge": "Canonical badge"}
	engine, err := NewEngineFromEmbed(nil)
	if err != nil {
		t.Fatal(err)
	}
	source, err := engine.RenderValue("layout/stellar", d)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(source, "Legacy title") || strings.Contains(source, "Legacy badge") || !strings.Contains(source, "Canonical title") || !strings.Contains(source, `data-admin-badge="Canonical badge"`) || !strings.Contains(source, `data-page-key="/custom/page/"`) {
		t.Fatal("native comments leaked legacy configuration")
	}
	// 区域运行时只能查询组件根内的节点；登录弹窗及表单必须由同一根拥有。
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	owned := map[string]bool{}
	var walk func(*html.Node, bool)
	walk = func(n *html.Node, inside bool) {
		for _, attr := range n.Attr {
			if attr.Key == "id" {
				inside = inside || attr.Val == "blog-comments"
				owned[attr.Val] = inside
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child, inside)
		}
	}
	walk(doc, false)
	for _, id := range []string{"cmt-modal", "cmt-modal-close", "cmt-modal-mask", "cmt-choose-email", "cmt-choose-skip", "cmt-form-login", "cmt-form-register", "cmt-login-username", "cmt-login-password", "cmt-do-login", "cmt-do-register", "cmt-reg-question", "cmt-reg-captcha", "cmt-reg-refresh", "cmt-reg-status"} {
		if !owned[id] {
			t.Errorf("native comment control %s is outside its runtime root", id)
		}
	}
	if !strings.Contains(source, `name="captcha_answer"`) || !strings.Contains(source, "验证码") || !strings.Contains(source, "换一题") {
		t.Fatal("native registration omitted the captcha form")
	}
}
