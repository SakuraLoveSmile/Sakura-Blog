package web

import (
	"strings"
	"testing"

	"blog/internal/settings"
)

func TestStellarPaginatorUsesUpstreamWindowAndKeepsNavigation(t *testing.T) {
	d := stellarTemplateFixture(t)
	d.Stellar.Root = "/sub/"
	d.Stellar.Data["paginationPath"] = "/notes/"
	d.Stellar.Data["activeTag"] = "Go 工程"
	pages := make([]int, 20)
	for i := range pages {
		pages[i] = i + 1
	}
	d.Paginator = &settings.Paginator{Current: 10, Total: 20, Pages: pages}
	body := string((stellarTemplateRenderer{}).paginator(d))
	if strings.Count(body, `class="page-number`) != 7 || strings.Count(body, `class="space"`) != 2 {
		t.Fatalf("20 pages must show both edges, two neighbors, current, and two ellipses: %s", body)
	}
	for _, want := range []string{`<span class="page-number current">10</span>`, `/sub/notes/page/8/?tag=Go+%E5%B7%A5%E7%A8%8B`, `/sub/notes/page/12/?tag=Go+%E5%B7%A5%E7%A8%8B`, `/sub/notes/page/20/?tag=Go+%E5%B7%A5%E7%A8%8B`, `rel="prev" href="/sub/notes/page/9/`, `rel="next" href="/sub/notes/page/11/`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s: %s", want, body)
		}
	}
	for _, current := range []int{1, 3} {
		d.Paginator = &settings.Paginator{Current: current, Total: 3, Pages: []int{1, 2, 3}}
		body = string((stellarTemplateRenderer{}).paginator(d))
		if strings.Count(body, `class="page-number`) != 3 || strings.Contains(body, `class="space"`) {
			t.Fatal("short pagination must show every page without ellipses")
		}
	}
}
