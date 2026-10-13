package routes

import (
	"context"
	"net/http"
	"strings"
	"testing"

	stellarweb "blog/modules/stellar/web"
	"golang.org/x/net/html"
)

// 固定上游 listing_nav/blog.ejs 使用 page.category/page.tag；详情链接仍指向总索引。
func TestStellarTaxonomyNavigationThroughEmbedAndOffline(t *testing.T) {
	for _, root := range []string{"", "/sub"} {
		t.Run("root="+root, func(t *testing.T) {
			ws, st, _, router := stellarRouteFixture(t)
			site := *ws.site.Load()
			site.URL = "https://fixture.example" + root
			ws.site.Store(&site)
			for _, slug := range []string{"one", "two", "three"} {
				createStellarPost(t, st, slug, "")
			}
			engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
			if err != nil {
				t.Fatal(err)
			}
			ws.engine = engine
			snapshots, err := ws.renderSnapshots(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			offline := map[string]string{}
			for _, snapshot := range snapshots {
				offline[snapshot.Path] = snapshot.HTML
			}
			for _, c := range []struct{ path, label, href string }{
				{"/blog/categories", "分类", "/blog/categories/"},
				{"/blog/categories/工程", "分类：工程", "/blog/categories/"},
				{"/blog/categories/工程/page/2", "分类：工程", "/blog/categories/"},
				{"/blog/tags", "标签", "/blog/tags/"},
				{"/blog/tags/Go", "标签：Go", "/blog/tags/"},
				{"/blog/tags/Go/page/2", "标签：Go", "/blog/tags/"},
				{"/blog", "近期发布", "/blog/"},
			} {
				response := stellarRequest(router, root+c.path)
				if response.Code != http.StatusOK {
					t.Fatal(root+c.path, response.Code, response.Body.String())
				}
				key := strings.Trim(c.path, "/") + "/index.html"
				if offline[key] == "" {
					t.Fatalf("missing offline taxonomy snapshot %s", key)
				}
				for flow, document := range map[string]string{"HTTP": response.Body.String(), "offline": offline[key]} {
					links := stellarActiveListingLinks(t, document)
					if len(links) != 1 || links[0].label != c.label || links[0].href != root+c.href {
						t.Errorf("%s %s active listing links = %#v, want %q -> %q", flow, root+c.path, links, c.label, root+c.href)
					}
				}
			}
		})
	}
}

type stellarListingNavLink struct{ label, href string }

func stellarActiveListingLinks(t *testing.T, document string) []stellarListingNavLink {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(document))
	if err != nil {
		t.Fatal(err)
	}
	var links []stellarListingNavLink
	var text func(*html.Node) string
	text = func(n *html.Node) string {
		if n.Type == html.TextNode {
			return n.Data
		}
		value := ""
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			value += text(c)
		}
		return value
	}
	var walk func(*html.Node, bool)
	walk = func(n *html.Node, inListing bool) {
		attrs := map[string]string{}
		for _, attr := range n.Attr {
			attrs[attr.Key] = attr.Val
		}
		inListing = inListing || strings.Contains(" "+attrs["class"]+" ", " listing-nav ")
		if inListing && n.Type == html.ElementNode && n.Data == "a" && strings.Contains(" "+attrs["class"]+" ", " active ") {
			links = append(links, stellarListingNavLink{text(n), attrs["href"]})
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, inListing)
		}
	}
	walk(doc, false)
	return links
}
