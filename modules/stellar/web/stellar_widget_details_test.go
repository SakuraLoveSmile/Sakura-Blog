package web

import (
	"strings"
	"testing"

	"blog/internal/stellarmodels"
	"golang.org/x/net/html"
)

func TestStellarLinklistSelectsEquivalentLocalPaths(t *testing.T) {
	for _, root := range []string{"/", "/blog/"} {
		for _, pagePath := range []string{"/about", "/about/", "/about/index", "/about/index.html", "/about.html", "about/", root + "about/", root + "about/index.html"} {
			t.Run(root+pagePath, func(t *testing.T) {
				d := stellarTemplateFixture(t)
				d.Stellar.Root = root
				d.Page.Path = pagePath
				paths := []struct {
					path   string
					active bool
				}{
					{"/about", true}, {"/about/", true}, {"/about/index", true},
					{"/about/index.html", true}, {"/about.html", true}, {"about/", true},
					{root + "about/", true}, {"/about/child/", false}, {"/about-other/", false},
					{"https://example.test/about/", false}, {"//other.test/about/", false},
				}
				var items []any
				for _, path := range paths {
					items = append(items, map[string]any{"title": path.path, "url": path.path})
				}
				source := string((stellarTemplateRenderer{}).linklist(d, map[string]any{"items": items}))
				doc, err := html.Parse(strings.NewReader(source))
				if err != nil {
					t.Fatal(err)
				}
				index := 0
				var walk func(*html.Node)
				walk = func(n *html.Node) {
					if n.Type == html.ElementNode && n.Data == "a" {
						if index >= len(paths) {
							t.Fatalf("unexpected link: %s", source)
						}
						want := paths[index]
						class, current, href := "", "", ""
						for _, attr := range n.Attr {
							switch attr.Key {
							case "class":
								class = attr.Val
							case "aria-current":
								current = attr.Val
							case "href":
								href = attr.Val
							}
						}
						if active := strings.Contains(" "+class+" ", " is-active "); active != want.active || (current == "page") != want.active {
							t.Errorf("link %q selected=%v aria-current=%q, want selected=%v", want.path, active, current, want.active)
						}
						if strings.Contains(href, "/blog/blog/") {
							t.Errorf("root applied twice: %q", href)
						}
						index++
					}
					for child := n.FirstChild; child != nil; child = child.NextSibling {
						walk(child)
					}
				}
				walk(doc)
				if index != len(paths) {
					t.Fatalf("got %d links, want %d", index, len(paths))
				}
			})
		}
	}
}

func TestStellarAuthorWidgetUsesCollectionAuthorSelection(t *testing.T) {
	for _, tc := range []struct {
		name, profile, pageAuthor, listingAuthor, wantName, wantPath string
		withoutModel                                                 bool
	}{
		{name: "wiki explicit", profile: "wiki", pageAuthor: "bob", listingAuthor: "alice", wantName: "Bob", wantPath: "/authors/bob/"},
		{name: "notebook explicit", profile: "notebook", pageAuthor: "bob", wantName: "Bob", wantPath: "/authors/bob/"},
		{name: "page explicit", profile: "page", pageAuthor: "bob", wantName: "Bob", wantPath: "/authors/bob/"},
		{name: "page without model", pageAuthor: "bob", withoutModel: true, wantName: "Bob", wantPath: "/authors/bob/"},
		{name: "dotted page author", profile: "wiki", pageAuthor: "alice.dev", wantName: "Alice Dev", wantPath: "/authors/alice.dev/"},
		{name: "post listing wins", profile: "post", pageAuthor: "alice", listingAuthor: "bob", wantName: "Bob", wantPath: "/authors/bob/"},
		{name: "topic listing wins", profile: "topic", pageAuthor: "alice", listingAuthor: "bob", wantName: "Bob", wantPath: "/authors/bob/"},
		{name: "dotted listing author", profile: "post", listingAuthor: "alice.dev", wantName: "Alice Dev", wantPath: "/authors/alice.dev/"},
		{name: "missing author", profile: "wiki", wantName: "Alice", wantPath: "/authors/alice/"},
		{name: "unknown author", profile: "page", pageAuthor: "unknown", wantName: "Alice", wantPath: "/authors/alice/"},
		{name: "post missing listing uses default", profile: "post", pageAuthor: "bob", wantName: "Alice", wantPath: "/authors/alice/"},
		{name: "topic unknown listing uses default", profile: "topic", pageAuthor: "bob", listingAuthor: "unknown", wantName: "Alice", wantPath: "/authors/alice/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := stellarTemplateFixture(t)
			d.Stellar.Data = map[string]any{
				"defaultAuthor": map[string]any{"id": "alice", "name": "Alice", "path": "/authors/alice/"},
				"authors": map[string]any{
					"alice":     map[string]any{"name": "Alice", "path": "/authors/alice/"},
					"bob":       map[string]any{"name": "Bob", "path": "/authors/bob/"},
					"alice.dev": map[string]any{"name": "Alice Dev", "path": "/authors/alice.dev/"},
				},
				"page": map[string]any{"article": map[string]any{"author": tc.pageAuthor}},
			}
			if !tc.withoutModel {
				d.Stellar.Model = &stellarmodels.PageModel{
					Collection: map[string]any{"profile": tc.profile},
					Render:     map[string]any{"listing": map[string]any{"authorId": tc.listingAuthor}},
				}
			}
			source := string((stellarTemplateRenderer{}).authorWidget(d, map[string]any{}, nil))
			if !strings.Contains(source, `<p class="username" type="text" id="name">`+tc.wantName+`</p>`) || !strings.Contains(source, `<a class="follow" href="`+tc.wantPath+`">`) {
				t.Fatalf("want author %q linking to %q, got %s", tc.wantName, tc.wantPath, source)
			}
		})
	}
}
