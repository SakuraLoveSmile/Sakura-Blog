package routes

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"blog/internal/store"
	stellarweb "blog/modules/stellar/web"
	"golang.org/x/net/html"
)

func TestStellarMenuActiveThroughActualEmbed(t *testing.T) {
	for _, root := range []string{"", "/sub"} {
		for columns := 1; columns <= 5; columns++ {
			t.Run(fmt.Sprintf("root=%s/columns=%d", root, columns), func(t *testing.T) {
				ws, st, _, router := stellarRouteFixture(t)
				site := *ws.site.Load()
				version := *site.StellarV2
				version.Theme = stellarCopyObject(version.Theme)
				profiles := stellarObject(version.Theme["profiles"])
				for _, raw := range profiles {
					stellarObject(raw)["activeMenu"] = nil
					stellarObject(stellarObject(raw)["leftbar"])["menuColumns"] = columns
				}
				leftbar := stellarObject(version.Theme["leftbar"])
				leftbar["menuColumns"] = columns
				leftbar["menu"] = []any{
					map[string]any{"id": "home", "title": "Home", "url": "/"},
					map[string]any{"id": "absolute", "title": "External", "url": "https://external.test/about/"},
					map[string]any{"id": "protocol", "title": "Protocol external", "url": "//external.test/about/"},
					map[string]any{"id": "about", "title": "About", "url": "/about/?source=menu#top"},
					map[string]any{"id": "post", "title": "Posts", "url": "/post/"},
					map[string]any{"id": "one", "title": "One", "url": "/post/one/"},
					map[string]any{"id": "custom", "title": "Custom", "url": "/custom/"},
				}
				site.URL = "https://fixture.example" + root
				site.StellarV2 = &version
				ws.site.Store(&site)
				engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
				if err != nil {
					t.Fatal(err)
				}
				ws.engine = engine
				for _, slug := range []string{"about", "unrelated"} {
					if _, err := st.CreatePage(&store.Page{Slug: slug, Title: slug, ContentMD: "Public page", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{}`}); err != nil {
						t.Fatal(err)
					}
				}
				createStellarPost(t, st, "one", "")
				createStellarPost(t, st, "two", `,"active_menu":"custom"`)
				createStellarPost(t, st, "three", "")
				assertMenu := func(path string, want []string) string {
					t.Helper()
					response := stellarRequest(router, root+path)
					if response.Code != http.StatusOK {
						t.Fatal(response.Code, response.Body.String())
					}
					got := stellarActiveMenuLinks(t, response.Body.String())
					if !reflect.DeepEqual(got, want) {
						t.Errorf("%s active menu = %q, want %q", root+path, got, want)
					}
					return response.Body.String()
				}
				assertMenu("/", []string{root + "/"})
				assertMenu("/about", []string{root + "/about/?source=menu#top"})
				assertMenu("/about/", []string{root + "/about/?source=menu#top"})
				assertMenu("/unrelated", nil)
				assertMenu("/post/one", []string{root + "/post/one/"})
				assertMenu("/post/two", []string{root + "/custom/"})
				assertMenu("/post/three", []string{root + "/post/"})
				stellarObject(profiles["page"])["activeMenu"] = "custom"
				assertMenu("/about/", []string{root + "/custom/"})
				stellarObject(profiles["page"])["activeMenu"] = "absolute"
				body := assertMenu("/about", []string{"https://external.test/about/"})
				if !strings.Contains(body, `href="//external.test/about/"`) {
					t.Fatal("protocol-relative menu URL changed")
				}
				stellarObject(profiles["page"])["activeMenu"] = nil
				stellarObject(stellarArray(leftbar["menu"])[3])["url"] = "/different/"
				assertMenu("/about", nil)
			})
		}
	}
}

func stellarActiveMenuLinks(t *testing.T, document string) []string {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(document))
	if err != nil {
		t.Fatal(err)
	}
	var links []string
	var walk func(*html.Node, bool)
	walk = func(node *html.Node, inMenu bool) {
		attrs := map[string]string{}
		for _, attr := range node.Attr {
			attrs[attr.Key] = attr.Val
		}
		inMenu = inMenu || attrs["data-widget-id"] == "leftbar:fixed:menu"
		if inMenu && node.Type == html.ElementNode && node.Data == "a" && strings.Contains(" "+attrs["class"]+" ", " is-active ") {
			if attrs["aria-current"] != "page" {
				t.Error("active menu lacks aria-current=page")
			}
			links = append(links, attrs["href"])
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child, inMenu)
		}
	}
	walk(doc, false)
	return links
}
