package web

import (
	"encoding/xml"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"blog/internal/settings"
	"golang.org/x/net/html"
)

func stellarHeadIconCSS(t *testing.T, registry *settings.IconRegistry) (string, string) {
	t.Helper()
	engine, err := NewEngineFromEmbed(registry)
	if err != nil {
		t.Fatal(err)
	}
	head, err := engine.RenderValue("partial/stellar/head", stellarTemplateFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(strings.NewReader(head))
	if err != nil {
		t.Fatal(err)
	}
	var css strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "style" {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.TextNode {
					css.WriteString(c.Data)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return head, css.String()
}

func TestStellarHeadEmbedsIconsConsumedByOfficialCSS(t *testing.T) {
	registry, err := settings.LoadIconsEmbed()
	if err != nil {
		t.Fatal(err)
	}
	_, css := stellarHeadIconCSS(t, registry)
	mainCSS, err := os.ReadFile("assets/css/main-card.css")
	if err != nil {
		t.Fatal(err)
	}
	for property, key := range map[string]string{
		"--icon-h3-left": "default:arrow-left", "--icon-h3-right": "default:arrow-right",
		"--icon-quote-left": "quot:quote-left", "--icon-quote-right": "quot:quote-right",
		"--icon-loading": "default:loading",
	} {
		t.Run(property, func(t *testing.T) {
			if !strings.Contains(string(mainCSS), "var("+property) {
				t.Fatalf("locked stylesheet no longer consumes %s", property)
			}
			match := regexp.MustCompile(regexp.QuoteMeta(property) + `:url\("data:image/svg\+xml,([^"\n]+)"\);`).FindStringSubmatch(css)
			if len(match) != 2 {
				t.Fatalf("rendered head is missing the %s SVG resource", property)
			}
			svg, err := url.PathUnescape(match[1])
			if err != nil || svg != registry.Get(key) {
				t.Fatalf("CSS image must decode to the registered %s SVG: %v", key, err)
			}
			decoder := xml.NewDecoder(strings.NewReader(svg))
			for {
				if _, err := decoder.Token(); err == io.EOF {
					break
				} else if err != nil {
					t.Fatalf("CSS mask resource is not valid SVG XML: %v", err)
				}
			}
		})
	}
}

func TestStellarHeadLoadingURLKeepsDocumentRelativeBaseAndCannotCloseStyle(t *testing.T) {
	for _, raw := range []string{
		"../images/loading.svg?size=24&color=blue",
		"/images/loading.svg",
		"https://cdn.example.test/loading.svg",
		`../loading image"</style><script>injected()</script>.svg?x=1\2`,
	} {
		t.Run(raw, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "icons.yml")
			if err := os.WriteFile(file, []byte("default:loading: "+raw+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			registry, err := settings.LoadIcons(file)
			if err != nil {
				t.Fatal(err)
			}
			head, css := stellarHeadIconCSS(t, registry)
			match := regexp.MustCompile(`--icon-loading:url\("([^"\n]*)"\);`).FindStringSubmatch(css)
			if len(match) != 2 {
				t.Fatal("loading image must be defined in the document's inline CSS")
			}
			value := regexp.MustCompile(`\\([0-9a-fA-F]{1,6}) `).ReplaceAllStringFunc(match[1], func(value string) string {
				n, err := strconv.ParseInt(value[1:len(value)-1], 16, 32)
				if err != nil {
					t.Fatal(err)
				}
				return string(rune(n))
			})
			if value != raw {
				t.Fatalf("loading URL changed from %q to %q", raw, value)
			}
			if strings.Contains(head, "<script>injected()") || strings.Contains(head, "ZgotmplZ") {
				t.Fatal("icon URL escaped its CSS/HTML context")
			}
			if strings.Contains(css, "icon-missing") || strings.Contains(css, "--icon-h3-left") {
				t.Fatal("missing registry icons must not become invalid image masks")
			}
		})
	}
}

func TestStellarHeadWithoutIconRegistryPreservesLoadingText(t *testing.T) {
	_, css := stellarHeadIconCSS(t, nil)
	if strings.Contains(css, "--icon-") || !strings.Contains(css, "--text-loading:") || !strings.Contains(css, "--text-loading-error:") {
		t.Fatal("nil registry must keep existing loading text and omit icon declarations")
	}
}
