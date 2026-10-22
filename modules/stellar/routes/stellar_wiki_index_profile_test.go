package routes

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"blog/internal/content"
	"blog/internal/store"
	stellarweb "blog/modules/stellar/web"
)

// 固定上游 scripts/generators/wiki.js 将总索引和标签索引投影为 index_wiki。
func TestStellarWikiIndexProfileThroughEmbedOfflineAndPreviewDocument(t *testing.T) {
	for _, root := range []string{"", "/sub"} {
		t.Run("root="+root, func(t *testing.T) {
			ws, st, _, router := stellarRouteFixture(t)
			site := *ws.site.Load()
			site.URL = "https://fixture.example" + root
			ws.site.Store(&site)
			if err := st.CreateWikiProject(&store.WikiProject{ID: "docs", Visible: true, StellarVersion: 2, StellarConfig: `{"name":"文档","tags":["Go"],"route":{"path":"/docs/"},"navigation":{"tree":["intro"]}}`}); err != nil {
				t.Fatal(err)
			}
			if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: "docs", Slug: "intro", Title: "intro", Status: store.StatusPublished, ContentMD: "文档正文", StellarVersion: 2, StellarConfig: `{"collection":{"profile":"wiki","id":"docs"}}`}); err != nil {
				t.Fatal(err)
			}
			engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
			if err != nil {
				t.Fatal(err)
			}
			ws.engine = engine
			snap, err := ws.stellarSnapshot(nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, tag := range []string{"", "Go"} {
				data, err := ws.stellarCollectionIndexView(snap, "wiki", tag)
				if err != nil {
					t.Fatal(err)
				}
				if data.Page.Layout != "index_wiki" || data.Stellar.Input.Page["layout"] != "index_wiki" || data.Stellar.Profile != "wikiIndex" || data.Stellar.Input.RuntimeData["wikiFilter"] != tag {
					t.Fatalf("tag=%q: layout=%q input=%v profile=%q filter=%v", tag, data.Page.Layout, data.Stellar.Input.Page["layout"], data.Stellar.Profile, data.Stellar.Input.RuntimeData["wikiFilter"])
				}
			}
			snapshots, err := ws.renderSnapshots(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			offline := map[string]string{}
			for _, snapshot := range snapshots {
				offline[snapshot.Path] = snapshot.HTML
			}
			for _, path := range []string{"/wiki", "/wiki/tags/Go", "/docs/intro"} {
				response := stellarRequest(router, root+path)
				if response.Code != http.StatusOK {
					t.Fatal(root+path, response.Code, response.Body.String())
				}
				key := strings.Trim(path, "/") + "/index.html"
				if offline[key] == "" {
					t.Fatalf("missing offline Wiki snapshot %s", key)
				}
				// 这里只验证完整文档的 opaque 包装；正式 Console API 仍为正文/组件预览。
				preview, err := stellarPreviewDocument(response.Body.String(), site.URL, "wiki-index-preview", content.StellarPreviewModeDocument)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(preview, `data-stellar-preview-token="wiki-index-preview"`) || strings.Contains(preview, `data-stellar-navigation=`) {
					t.Fatal("preview document lost isolation metadata or retained navigation")
				}
				layout, pageType := "index_wiki", "index"
				if path == "/docs/intro" {
					// 文档路由的 wiki-doc 由既有模型归一化为官方 wiki 布局。
					layout, pageType = "wiki", "content"
				}
				for flow, document := range map[string]string{"HTTP": response.Body.String(), "offline": offline[key], "preview document": preview} {
					for _, want := range []string{`data-page-layout="` + layout + `"`, `data-page-type="` + pageType + `"`} {
						if !strings.Contains(document, want) {
							t.Errorf("%s %s missing %s", flow, root+path, want)
						}
					}
				}
			}
		})
	}
}
