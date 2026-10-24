package routes

import (
	"context"
	"net/http"
	"strings"
	"testing"

	stellarweb "blog/modules/stellar/web"
	"golang.org/x/net/html"
)

func TestStellarOfficialShareIconsThroughEmbedAndOffline(t *testing.T) {
	ws, st, _, router := stellarRouteFixture(t)
	createStellarPost(t, st, "one", "")
	icons, err := stellarweb.LoadIconsEmbed()
	if err != nil {
		t.Fatal(err)
	}
	ws.icons = icons
	engine, err := stellarweb.NewEngineFromEmbed(icons)
	if err != nil {
		t.Fatal(err)
	}
	ws.engine = engine
	response := stellarRequest(router, "/post/one")
	if response.Code != http.StatusOK {
		t.Fatal(response.Code, response.Body.String())
	}
	documents := map[string]string{"HTTP": response.Body.String()}
	snapshots, err := ws.renderSnapshots(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, snapshot := range snapshots {
		if snapshot.Path == "post/one/index.html" {
			documents["offline"] = snapshot.HTML
		}
	}
	if len(documents) != 2 {
		t.Fatal("missing offline post snapshot")
	}
	for flow, document := range documents {
		doc, err := html.Parse(strings.NewReader(document))
		if err != nil {
			t.Fatal(err)
		}
		buttons := map[string]*html.Node{}
		var walk func(*html.Node)
		walk = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "a" {
				for _, attr := range n.Attr {
					if attr.Key == "class" && strings.Contains(" "+attr.Val+" ", " share-item ") {
						for _, service := range []string{"qrcode", "weibo", "x", "telegram", "whatsapp", "email"} {
							if strings.Contains(" "+attr.Val+" ", " "+service+" ") {
								buttons[service] = n
							}
						}
					}
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
		walk(doc)
		for _, service := range []string{"qrcode", "weibo", "x", "telegram", "whatsapp", "email"} {
			button := buttons[service]
			if button == nil {
				t.Errorf("%s missing official %s button", flow, service)
				continue
			}
			var rendered strings.Builder
			if err := html.Render(&rendered, button); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(rendered.String(), "icon-missing") || !strings.Contains(rendered.String(), "<svg") {
				t.Errorf("%s %s button has no official SVG", flow, service)
			}
		}
	}
}
