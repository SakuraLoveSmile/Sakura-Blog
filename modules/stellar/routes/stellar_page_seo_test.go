package routes

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"blog/internal/content"
	"blog/internal/store"
	stellarweb "blog/modules/stellar/web"
	"golang.org/x/net/html"
)

func TestStellarStandalonePageOpenGraphThroughActualEmbed(t *testing.T) {
	for _, test := range []struct {
		name, config string
		want         map[string][]string
	}{
		{"explicit", `{"description":"Page SEO","seo":{"open_graph":{"title":"Ignored upstream title","site_name":"Ignored upstream site","image":["/explicit.png","/second.png"],"description":"Independent OG","type":"profile","language":"zh-TW","twitter_card":"summary_large_image","twitter_id":"@page-author","twitter_site":"@page-site"}}}`, map[string][]string{
			"description": {"Page SEO"}, "og:description": {"Independent OG"}, "twitter:description": {"Independent OG"}, "og:image": {"/explicit.png", "/second.png"}, "og:type": {"profile"}, "og:locale": {"zh_TW"}, "twitter:card": {"summary_large_image"}, "twitter:creator": {"@page-author"}, "twitter:site": {"@page-site"},
		}},
		{"null", `{"seo":{"open_graph":{"image":null,"description":null,"type":null,"language":null,"twitter_id":null,"twitter_site":null}}}`, map[string][]string{"og:image": nil, "twitter:image": nil, "og:description": nil, "og:type": nil, "og:locale": nil, "twitter:creator": nil, "twitter:site": nil}},
		{"empty", `{"seo":{"open_graph":{"image":"","description":""}}}`, map[string][]string{"og:image": nil, "twitter:image": nil, "og:description": nil}},
		{"cover", `{"cover":"/cover.png"}`, map[string][]string{"og:image": {"/cover.png"}}},
		{"content", `{}`, map[string][]string{"og:image": {"/body.png"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ws, st, _, router := stellarRouteFixture(t)
			site := *ws.site.Load()
			site.Avatar = "/avatar.png"
			version := *site.StellarV2
			version.Theme = stellarCopyObject(version.Theme)
			version.Theme["openGraph"] = map[string]any{"enabled": true, "twitterId": "@theme-author"}
			site.StellarV2 = &version
			ws.site.Store(&site)
			engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
			if err != nil {
				t.Fatal(err)
			}
			ws.engine = engine
			row := &store.Page{Slug: "about", Title: "Page title", ContentMD: "![Body](/body.png)", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: test.config}
			if _, err := st.CreatePage(row); err != nil {
				t.Fatal(err)
			}
			response := stellarRequest(router, "/about")
			if response.Code != http.StatusOK {
				t.Fatal(response.Code, response.Body.String())
			}
			documents := map[string]string{"SSR": response.Body.String()}
			snapshots, err := ws.renderSnapshots(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			for _, snapshot := range snapshots {
				if snapshot.Path == "about/index.html" {
					documents["offline"] = snapshot.HTML
				}
			}
			preview, err := ws.renderContentPreview(content.StellarPreviewInput{StellarVersion: 2, StellarConfig: test.config, Title: row.Title, Layout: "page", Path: "/about", RequestURL: "https://preview.test/api/v1/preview", HTML: `<p><img src="/body.png" alt="Body"></p>`})
			if err != nil {
				t.Fatal(err)
			}
			documents["preview"] = preview
			if len(documents) != 3 {
				t.Fatal("standalone page missing from offline snapshots")
			}
			for flow, document := range documents {
				metadata := stellarPageSEOMeta(t, document)
				for key, want := range test.want {
					if !reflect.DeepEqual(metadata[key], want) {
						t.Errorf("%s %s = %q, want %q", flow, key, metadata[key], want)
					}
				}
				// 固定上游 head.ejs 最后重写这三项；显式 args.title/site_name 不改变它们。
				for key, want := range map[string]string{"og:title": row.Title, "twitter:title": row.Title, "og:site_name": site.Title} {
					if !reflect.DeepEqual(metadata[key], []string{want}) {
						t.Errorf("%s %s = %q, want upstream title %q", flow, key, metadata[key], want)
					}
				}
			}
			persisted, err := st.GetPageBySlug(row.Slug)
			if err != nil || persisted.StellarConfig != test.config {
				t.Fatal("rendering changed persisted strict page config", err)
			}
		})
	}
}

func stellarPageSEOMeta(t *testing.T, document string) map[string][]string {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(document))
	if err != nil {
		t.Fatal(err)
	}
	metadata := map[string][]string{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "meta" {
			key, value := "", ""
			for _, attr := range n.Attr {
				switch attr.Key {
				case "name", "property":
					key = attr.Val
				case "content":
					value = attr.Val
				}
			}
			metadata[key] = append(metadata[key], value)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return metadata
}
