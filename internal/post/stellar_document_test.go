package post

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"blog/internal/stellarv2"
)

func TestStellarDocumentPreservesConfigurationBodyAndBlogExtension(t *testing.T) {
	src := "---\r\ntitle: 新文章\r\narticle:\r\n  style: story\r\n  ai_label: polished\r\nvisibility:\r\n  listed: false\r\ncomments:\r\n  enabled: false\r\n  options:\r\n    custom_limit: 9007199254740993\r\nblog:\r\n  authors: [alice, bob]\r\n  poster: {headline: 独立扩展}\r\n---\r\n# 标题\r\n{% box 标题 %}\r\n原始 Markdown  \r\n{% endbox %}\r\n"
	doc, err := ParseStellarDocument([]byte(src), StellarDocumentOptions{Source: "posts/example.md", Profile: "post"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(doc.Body, "# 标题\r\n") || !strings.HasSuffix(doc.Body, "{% endbox %}\r\n") {
		t.Fatalf("body changed: %q", doc.Body)
	}
	if doc.Page["visibility"].(map[string]any)["listed"] != false || doc.Page["comments"].(map[string]any)["enabled"] != false {
		t.Fatal("explicit false lost")
	}
	if _, exists := doc.Raw["blog"]; exists {
		t.Fatal("Blog extension leaked into upstream schema")
	}
	if doc.Page["article"].(map[string]any)["aiLabel"] != "polished" {
		t.Fatal("runtime normalization missing")
	}
	limit := doc.Page["comments"].(map[string]any)["options"].(map[string]any)["custom_limit"]
	if limit != json.Number("9007199254740993") {
		t.Fatalf("parameter bag number lost: %#v", limit)
	}
	exported, err := SerializeStellarDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(exported), "aiLabel") {
		t.Fatal("runtime names persisted")
	}
	reopened, err := ParseStellarDocument(exported, StellarDocumentOptions{Source: "posts/example.md", Profile: "post"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(doc.Raw, reopened.Raw) || !reflect.DeepEqual(doc.Blog, reopened.Blog) || doc.Body != reopened.Body {
		t.Fatalf("round trip lost document: %s", exported)
	}
}

func TestStellarDocumentStrictAndMalformedInputs(t *testing.T) {
	for _, source := range []string{
		"---\ntopic: old\n---\nbody", "---\npin: true\n---\nbody", "---\nbanner: /old.jpg\n---\nbody",
		"---\ntitle: one\ntitle: two\n---\nbody", "---\ntitle: unclosed", "---", "---\nblog: true\n---\nbody",
		";;;\nnull\n;;;\nbody", ";;;\n{} {}\n;;;\nbody", "---\n[one,two]\n---\nbody",
	} {
		if _, err := ParseStellarDocument([]byte(source), StellarDocumentOptions{Source: "invalid.md"}); err == nil {
			t.Errorf("accepted %q", source)
		}
	}
	for _, source := range []string{"body\r\n", "\xef\xbb\xbfbody\n", "---\n---\nbody", ";;;\n{\"title\":\"JSON\",\"comments\":{\"enabled\":false}}\n;;;\nbody"} {
		if _, err := ParseStellarDocument([]byte(source), StellarDocumentOptions{}); err != nil {
			t.Errorf("%q: %v", source, err)
		}
	}
}

func TestStellarDocumentCollectionMembershipUsesStrictRegistry(t *testing.T) {
	collection, err := stellarv2.ParseCollection(map[string]any{"name": "Docs"}, stellarv2.Options{})
	if err != nil {
		t.Fatal(err)
	}
	registry := stellarv2.CreateCollectionRegistry(map[string]map[string]any{"wiki/docs": collection.Value})
	opts := StellarDocumentOptions{Source: "wiki/docs/intro.md", Kind: "pages", PagePath: "wiki/docs/intro", Registry: registry, Profile: "wiki"}
	doc, err := ParseStellarDocument([]byte("---\ntitle: Intro\n---\nbody"), opts)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Page["collection"].(map[string]any)["id"] != "docs" {
		t.Fatal("membership not inferred")
	}
	if _, err := ParseStellarDocument([]byte("---\ncollection: {profile: wiki, id: absent}\n---\nbody"), opts); err == nil {
		t.Fatal("missing collection accepted")
	}
}
