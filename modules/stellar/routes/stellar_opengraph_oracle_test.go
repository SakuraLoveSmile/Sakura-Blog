package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"blog/internal/content"
	"blog/internal/store"
	stellarweb "blog/modules/stellar/web"
)

func TestStellarOpenGraphMatchesFixedHeadAndHexo8ThroughEmbed(t *testing.T) {
	var oracle struct {
		Commit         string            `json:"commit"`
		Hexo           string            `json:"hexo"`
		HexoUtil       string            `json:"hexoUtil"`
		HexoHelperUtil string            `json:"hexoHelperUtil"`
		Fingerprints   map[string]string `json:"fingerprints"`
		Entries        []struct {
			Name, Profile, Path, Markdown string
			RawConfig                     map[string]any `json:"rawConfig"`
			Outputs                       map[string]string
			Errors                        map[string]struct{ Name, Message string }
		}
	}
	raw, err := os.ReadFile("../web/stellar_template_helpers_testdata/opengraph.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &oracle); err != nil {
		t.Fatal(err)
	}
	if oracle.Commit != "9d11978a24fe6610e3a7dbab741cf62634539d05" || oracle.Hexo != "8.0.0" || oracle.HexoUtil != "4.0.0" || oracle.HexoHelperUtil != "3.3.0" || oracle.Fingerprints["head"] != "24fb9a1db1e2b1b13c9148a9b3466a07c67c783fdc88f1279560fad90b374b48" || oracle.Fingerprints["helper"] != "b18e925f72ed0863779b102e83e904f5c07ea573fde60d5a4cbd415e3fd3f0df" {
		t.Fatal("oracle source changed")
	}
	for _, entry := range oracle.Entries {
		t.Run(entry.Name, func(t *testing.T) {
			ws, st, _, router := stellarRouteFixture(t)
			site := *ws.site.Load()
			site.Title = "OG Site"
			site.Description = "Site description"
			site.Avatar = "/avatar.png"
			site.URL = "https://fixture.example/sub/"
			version := *site.StellarV2
			version.Theme = stellarCopyObject(version.Theme)
			version.Theme["openGraph"] = map[string]any{"enabled": true, "twitterId": "theme-user"}
			version.Theme["canonical"] = map[string]any{"host": "canonical.fixture.test", "allowedHosts": []any{}}
			version.Blog = stellarCopyObject(version.Blog)
			version.Blog["site"] = map[string]any{"language": "en", "author": "Default Author", "keywords": []any{"site-keyword"}}
			site.StellarV2 = &version
			ws.site.Store(&site)
			engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
			if err != nil {
				t.Fatal(err)
			}
			engine.SetStellarMarkdownRender(ws.posts.RenderStellarV2WithContext)
			ws.engine = engine
			config, _ := json.Marshal(entry.RawConfig)
			title := entry.RawConfig["title"].(string)
			switch entry.Profile {
			case "post":
				if _, err = st.Create(&store.Post{Slug: "og-fixture", Title: title, ContentMD: entry.Markdown, Status: store.StatusPublished, StellarVersion: 2, StellarConfig: string(config), BlogExtensions: "{}"}); err != nil {
					t.Fatal(err)
				}
			case "page":
				if _, err = st.CreatePage(&store.Page{Slug: "og-page", Title: title, ContentMD: entry.Markdown, Status: store.StatusPublished, StellarVersion: 2, StellarConfig: string(config), BlogExtensions: "{}"}); err != nil {
					t.Fatal(err)
				}
			case "wiki":
				if err = st.CreateWikiProject(&store.WikiProject{ID: "og", Name: "OG Collection", Visible: true, StellarVersion: 2, StellarConfig: `{"name":"OG Collection"}`, BlogExtensions: "{}"}); err != nil {
					t.Fatal(err)
				}
				if err = st.CreateWikidoc(&store.WikiDoc{ProjectID: "og", Slug: "entry", Title: title, ContentMD: entry.Markdown, Status: store.StatusPublished, StellarVersion: 2, StellarConfig: string(config), BlogExtensions: "{}"}); err != nil {
					t.Fatal(err)
				}
			case "notebook":
				if err = st.CreateNotebook(&store.Notebook{ID: "og", Name: "OG Collection", StellarVersion: 2, StellarConfig: `{"name":"OG Collection"}`, BlogExtensions: "{}"}); err != nil {
					t.Fatal(err)
				}
				if _, err = st.CreateNote(&store.Note{NotebookID: "og", Slug: "entry", Title: title, ContentMD: entry.Markdown, Status: store.StatusPublished, StellarVersion: 2, StellarConfig: string(config), BlogExtensions: "{}"}); err != nil {
					t.Fatal(err)
				}
			}
			response := stellarRequest(router, "/sub"+entry.Path)
			invalid := len(entry.Errors) > 0
			if invalid {
				if entry.Errors["SSR"].Name != "TypeError" || response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "stellar_render_failed") {
					t.Fatalf("invalid OG must fail HTTP SSR: %d %s", response.Code, response.Body.String())
				}
			} else if response.Code != http.StatusOK {
				t.Fatal(response.Code, response.Body.String())
			}
			docs := map[string]string{"SSR": response.Body.String()}
			snapshots, err := ws.renderSnapshots(context.Background())
			if invalid {
				if entry.Errors["offline"].Name != "TypeError" || err == nil || !strings.Contains(err.Error(), "stellar open_graph.") {
					t.Fatalf("invalid OG must fail offline render: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			for _, snapshot := range snapshots {
				if snapshot.Path == strings.Trim(entry.Path, "/")+"/index.html" {
					docs["offline"] = snapshot.HTML
				}
			}
			layout := entry.Profile
			if layout == "notebook" {
				layout = "note"
			}
			output, err := ws.renderContentPreviewSnapshot(content.StellarPreviewInput{StellarVersion: 2, StellarConfig: string(config), BlogExtensions: "{}", Title: title, Layout: layout, Path: "/sub" + entry.Path, RequestURL: "https://preview.test/sub/api/v1/preview/markdown", Markdown: &entry.Markdown, PreviewToken: "og-oracle-preview"})
			if invalid {
				if entry.Errors["preview"].Name != "TypeError" || err == nil || !strings.Contains(err.Error(), "stellar open_graph.") {
					t.Fatalf("invalid OG must fail opaque preview: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			docs["preview"] = output.Document
			if len(docs) != 3 {
				t.Fatalf("missing offline document for %s", entry.Path)
			}
			for flow, document := range docs {
				got := stellarOGOracleMeta(t, document)
				want := stellarOGOracleMeta(t, entry.Outputs[flow])
				if !reflect.DeepEqual(got, want) {
					t.Errorf("%s OG differs\ngot  %#v\nwant %#v", flow, got, want)
				}
				// 显式社交url不会改变本地canonical／JSONLD规范地址。
				if strings.Contains(entry.Name, "all-options") {
					canonical := "https://canonical.fixture.test/sub" + entry.Path
					if flow == "preview" {
						canonical = "https://preview.test/sub" + entry.Path
					}
					if !strings.Contains(document, `rel="canonical" href="`+canonical+`"`) || !strings.Contains(document, `"url":"`+canonical+`"`) {
						t.Errorf("%s changed canonical/JSONLD", flow)
					}
				}
			}
		})
	}
}
func stellarOGOracleMeta(t *testing.T, document string) map[string][]string {
	all := stellarPageSEOMeta(t, document)
	out := map[string][]string{}
	for key, value := range all {
		if key == "description" || strings.HasPrefix(key, "og:") || strings.HasPrefix(key, "twitter:") || strings.HasPrefix(key, "article:") || strings.HasPrefix(key, "fb:") {
			out[key] = value
		}
	}
	return out
}
