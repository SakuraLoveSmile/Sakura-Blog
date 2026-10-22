package routes

import (
	"encoding/xml"
	"net/http"
	"strings"
	"testing"
)

func TestStrictSitemapPreservesBlogIndexesAndRootPriority(t *testing.T) {
	ws, _, _, router := stellarRouteFixture(t)
	site := *ws.site.Load()
	site.URL = "https://fixture.example/sub"
	version := *site.StellarV2
	version.Theme = stellarCopyObject(version.Theme)
	version.Theme["profiles"].(map[string]any)["blogIndex"].(map[string]any)["tags"] = map[string]any{"path": "/labels"}
	site.StellarV2 = &version
	ws.site.Store(&site)
	response := stellarRequest(router, "/sitemap.xml")
	if response.Code != http.StatusOK {
		t.Fatalf("sitemap: %d %s", response.Code, response.Body.String())
	}
	var document struct {
		URLs []struct {
			Loc      string `xml:"loc"`
			Priority string `xml:"priority"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(response.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, item := range document.URLs {
		if _, duplicate := seen[item.Loc]; duplicate || strings.Contains(item.Loc, "/sub/sub/") {
			t.Fatalf("invalid canonical sitemap URL: %s", item.Loc)
		}
		seen[item.Loc] = item.Priority
	}
	for path, priority := range map[string]string{"/": "1.0", "/blog/archives": "0.6", "/blog/categories": "0.6", "/labels": "0.6", "/links": "0.6", "/moments": "0.6"} {
		url := "https://fixture.example/sub" + path
		if seen[url] != priority {
			t.Errorf("%s priority=%q, want %q", url, seen[url], priority)
		}
	}
	if _, stale := seen["https://fixture.example/sub/blog/tags"]; stale {
		t.Fatal("sitemap retained the old taxonomy path")
	}
}
