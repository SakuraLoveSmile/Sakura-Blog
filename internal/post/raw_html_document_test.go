package post

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const fullHTMLDocument = `<!doctype html>
<html data-saved=""><head><meta charset="utf-8"><style>body{padding:16px;background:#151515}</style></head>
<body><div id="app"><h1>hello &amp; world</h1><button>go</button></div></body></html>`

func frameSrcDoc(t *testing.T, out string) string {
	t.Helper()
	nodes, err := html.ParseFragment(strings.NewReader(out), &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		t.Fatalf("parse output: %v", err)
	}
	var walk func(*html.Node) string
	walk = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "iframe" {
			for _, attr := range n.Attr {
				if attr.Key == "srcdoc" {
					return attr.Val
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if v := walk(c); v != "" {
				return v
			}
		}
		return ""
	}
	for _, n := range nodes {
		if v := walk(n); v != "" {
			return v
		}
	}
	t.Fatalf("no iframe[data-raw-html-frame] in output: %.300s", out)
	return ""
}

func TestRenderIsolatesFullHTMLDocument(t *testing.T) {
	svc := NewService("")
	out, err := svc.Render(fullHTMLDocument)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(out, "<p>&lt;!doctype") {
		t.Fatalf("doctype must not be escaped into a paragraph: %.300s", out)
	}
	if !strings.Contains(out, `data-raw-html-frame="1"`) || !strings.Contains(out, `sandbox="allow-scripts"`) {
		t.Fatalf("expected isolated sandbox iframe: %.300s", out)
	}
	if !strings.Contains(out, "data-stellar-raw-html-report") {
		t.Fatal("expected height report script inside srcdoc")
	}
	src := frameSrcDoc(t, out)
	if !strings.Contains(src, "<h1>hello &amp; world</h1>") || !strings.Contains(src, "<button>go</button>") {
		t.Fatalf("srcdoc lost document content: %.400s", src)
	}
	if !strings.Contains(src, "<!doctype html>") {
		t.Fatalf("srcdoc must keep the original document: %.200s", src)
	}
	if !strings.Contains(src, "stellar-raw-html-height") {
		t.Fatal("srcdoc must report its height to the host page")
	}
}

func TestRenderStellarV2IsolatesFullHTMLDocument(t *testing.T) {
	svc := NewService("")
	out, err := svc.RenderStellarV2WithContext(fullHTMLDocument, StellarRenderContext{})
	if err != nil {
		t.Fatalf("render v2: %v", err)
	}
	if strings.Contains(out, "<p>&lt;!doctype") {
		t.Fatalf("doctype must not be escaped into a paragraph: %.300s", out)
	}
	if !strings.Contains(out, `data-raw-html-frame="1"`) {
		t.Fatalf("expected isolated iframe: %.300s", out)
	}
}

func TestRenderKeepsFragmentInline(t *testing.T) {
	svc := NewService("")
	out, err := svc.Render(`<div class="card"><p>片段</p></div>`)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(out, "data-raw-html-frame") {
		t.Fatalf("fragment must stay inline: %.300s", out)
	}
	if !strings.Contains(out, "<p>片段</p>") {
		t.Fatalf("fragment content lost: %.300s", out)
	}
}

func TestRenderKeepsDocumentInsideCodeFence(t *testing.T) {
	svc := NewService("")
	body := "示例：\n\n" + "```html\n" + fullHTMLDocument + "\n```\n"
	out, err := svc.Render(body)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(out, "data-raw-html-frame") {
		t.Fatalf("document inside a code fence must not be isolated: %.400s", out)
	}
	if !strings.Contains(out, "&lt;!doctype html&gt;") {
		t.Fatalf("code block should keep the escaped document: %.400s", out)
	}
}

func TestRenderIsolatesDocumentAmongMarkdown(t *testing.T) {
	svc := NewService("")
	body := "# 标题\n\n" + fullHTMLDocument + "\n\n结尾段落。\n"
	out, err := svc.Render(body)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "data-raw-html-frame") {
		t.Fatalf("expected isolated iframe: %.400s", out)
	}
	if strings.Contains(out, "<p>&lt;!doctype") {
		t.Fatalf("doctype must not be escaped: %.400s", out)
	}
	if !strings.Contains(out, "<h1") || !strings.Contains(out, "结尾段落") {
		t.Fatalf("surrounding markdown lost: %.400s", out)
	}
	if strings.Contains(out, "<style>body{padding:16px") {
		t.Fatal("document styles must not leak into the host page")
	}
}

func TestIsolateRawHTMLDocumentsKeepsLinesOutsideDocument(t *testing.T) {
	in := "开头\n\n" + fullHTMLDocument + "\n\n结尾"
	out := isolateRawHTMLDocuments(in)
	if !strings.HasPrefix(out, "开头") || !strings.HasSuffix(out, "结尾") {
		t.Fatalf("outer text changed: %.120s", out)
	}
	if strings.Count(out, `data-raw-html-frame="1"`) != 1 {
		t.Fatalf("expected exactly one frame: %d", strings.Count(out, `data-raw-html-frame="1"`))
	}
}

func TestRenderAddsDoctypeWhenEditorDroppedIt(t *testing.T) {
	svc := NewService("")
	out, err := svc.Render("<html><body><p>no doctype</p></body></html>")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	src := frameSrcDoc(t, out)
	if !strings.HasPrefix(src, "<!doctype html>") {
		t.Fatalf("srcdoc must start with a doctype to stay in standards mode: %.120s", src)
	}
	if !strings.Contains(src, "<p>no doctype</p>") {
		t.Fatalf("content lost: %.200s", src)
	}
}
