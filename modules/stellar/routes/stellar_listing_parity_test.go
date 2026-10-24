package routes

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"blog/internal/stellarmodels"
	stellarweb "blog/modules/stellar/web"
	"golang.org/x/net/html"
)

func TestStellarGlobalPinsRemainOnFilteredAndPaginatedListings(t *testing.T) {
	ws, st, capture, router := stellarRouteFixture(t)
	for i := 1; i <= 3; i++ {
		row := createStellarPost(t, st, fmt.Sprintf("pin-%d", i), fmt.Sprintf(`,"listing":{"priority":%d}`, 4-i))
		row.CreatedAt = time.Date(2026, 1, i, 0, 0, 0, 0, time.UTC)
		if i == 1 {
			row.StellarConfig = `{"title":"pin-1","tags":["Other"],"categories":["其他"],"listing":{"priority":3}}`
		}
		if err := st.Update(row.Slug, row); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= 6; i++ {
		row := createStellarPost(t, st, fmt.Sprintf("ordinary-%d", i), "")
		row.CreatedAt = time.Date(2026, 2, i, 0, 0, 0, 0, time.UTC)
		if err := st.Update(row.Slug, row); err != nil {
			t.Fatal(err)
		}
	}
	createStellarPost(t, st, "unlisted-pin", `,"visibility":{"listed":false},"listing":{"priority":99}`)
	locked := createStellarPost(t, st, "locked-pin", `,"listing":{"priority":98}`)
	locked.Password = "private"
	if err := st.Update(locked.Slug, locked); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/page/2", "/blog/categories", "/blog/tags", "/blog/archives", "/topic", "/blog/tags/Go", "/blog/tags/Go/page/2"} {
		response := stellarRequest(router, path)
		if response.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, response.Code, response.Body.String())
		}
		pins := capture.last.Stellar.PinnedItems
		if len(pins) != 3 {
			t.Fatalf("%s: expected all three public pins, got %d", path, len(pins))
		}
		for _, item := range pins {
			if !strings.HasPrefix(stellarString(item.Item["title"]), "pin-") {
				t.Fatalf("%s: non-public pin %v", path, item.Item["title"])
			}
		}
	}
	response := stellarRequest(router, "/")
	if response.Code != http.StatusOK || capture.last.Paginator.Total != 5 || len(capture.last.Stellar.ListItems) != 2 || !strings.HasPrefix(stellarString(capture.last.Stellar.ListItems[0].Item["title"]), "ordinary-") {
		t.Fatal("home pagination must use the dated public list, with pins projected separately")
	}
	response = stellarRequest(router, "/page/4")
	if response.Code != http.StatusOK || !strings.HasPrefix(stellarString(capture.last.Stellar.ListItems[0].Item["title"]), "pin-") {
		t.Fatal("pins must remain in subsequent dated listing pages")
	}

	engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
	if err != nil {
		t.Fatal(err)
	}
	ws.engine = engine
	for _, path := range []string{"/", "/page/2", "/blog/categories", "/blog/tags", "/blog/archives", "/topic", "/blog/tags/Go"} {
		response := stellarRequest(router, path)
		body := response.Body.String()
		if response.Code != http.StatusOK || strings.Count(body, `id="pin-slider"`) != 1 || !strings.Contains(body, `pin-slide-headline">pin-1`) {
			t.Fatalf("%s: global carousel must render exactly once", path)
		}
	}
	current := *ws.site.Load()
	v2 := *current.StellarV2
	v2.Theme = stellarCopyObject(v2.Theme)
	profiles := stellarCopyObject(stellarObject(v2.Theme["profiles"]))
	index := stellarCopyObject(stellarObject(profiles["blogIndex"]))
	index["listingNav"] = stellarmodels.Object{"enabled": false}
	profiles["blogIndex"] = index
	v2.Theme["profiles"] = profiles
	current.StellarV2 = &v2
	ws.site.Store(&current)
	response = stellarRequest(router, "/blog/tags")
	if strings.Count(response.Body.String(), `id="pin-slider"`) != 1 || strings.Contains(response.Body.String(), `<div class="listing-nav">`) {
		t.Fatal("disabling listing navigation must preserve the independent carousel")
	}
}

func TestStellarFlatPinsExceedPageSizeWithoutDuplicatingCards(t *testing.T) {
	ws, st, _, router := stellarRouteFixture(t)
	for i := 1; i <= 3; i++ {
		createStellarPost(t, st, fmt.Sprintf("pin-%d", i), fmt.Sprintf(`,"listing":{"priority":%d}`, 4-i))
	}
	createStellarPost(t, st, "ordinary", "")
	current := *ws.site.Load()
	v2 := *current.StellarV2
	v2.Theme = stellarCopyObject(v2.Theme)
	article := stellarCopyObject(stellarObject(v2.Theme["article"]))
	listing := stellarCopyObject(stellarObject(article["listing"]))
	listing["pinnedLayout"] = "flat"
	article["listing"] = listing
	v2.Theme["article"] = article
	current.StellarV2 = &v2
	ws.site.Store(&current)
	engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
	if err != nil {
		t.Fatal(err)
	}
	ws.engine = engine
	response := stellarRequest(router, "/")
	body := response.Body.String()
	if response.Code != http.StatusOK || strings.Contains(body, `id="pin-slider"`) {
		t.Fatal("flat layout must render cards instead of a carousel")
	}
	for i := 1; i <= 3; i++ {
		if stellarPostCardLinks(t, body)[fmt.Sprintf("/post/pin-%d", i)] != 1 {
			t.Fatalf("flat homepage must show pin-%d once even above per_page=2", i)
		}
	}
}

func stellarPostCardLinks(t *testing.T, body string) map[string]int {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	links := map[string]int{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		class, href := "", ""
		for _, a := range n.Attr {
			if a.Key == "class" {
				class = " " + a.Val + " "
			}
			if a.Key == "href" {
				href = a.Val
			}
		}
		if n.Data == "a" && strings.Contains(class, " post-card ") {
			links[href]++
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return links
}
