package stellarmigration

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"blog/internal/post"
	"blog/internal/store"
)

func TestExplicitLegacyMarkdownFieldsAndCodePreservation(t *testing.T) {
	source := "---\ntitle: Old\nslug: original\nstatus: draft\npinned: true\ncategory: Tech/Go\nauthor: Alice\nai_label: generated\nposter: {headline: Hello, topic: Series, caption: Caption, color: red}\nbanner: /banner.jpg\ndate: 2019-02-03 04:05:06\ntopic: Series\n---\n```md\n{% unknown a %}\n{% tip code %}\n```\n{% tip 'keep spacing' %}\n"
	if _, err := post.ParseStellarDocument([]byte(source), post.StellarDocumentOptions{}); err == nil {
		t.Fatal("strict import silently converted legacy fields")
	}
	result, err := ConvertLegacyMarkdown([]byte(source), "old.md", "post", []*store.Topic{{Slug: "series", Name: "Series"}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.LegacyConverted || result.SlugSuggestion != "original" || len(result.Warnings) == 0 {
		t.Fatalf("missing conversion metadata: %+v", result)
	}
	for path, want := range map[string]string{"listing.priority": "1", "article.author": `"Alice"`, "article.ai_label": `"generated"`, "collection.id": `"series"`, "date": `"2019-02-03 04:05:06"`, "categories": `["Tech/Go"]`} {
		value, _ := get(result.Document.Raw, path)
		if encode(value) != want {
			t.Errorf("%s=%s, want %s", path, encode(value), want)
		}
	}
	if status, _ := get(result.Document.Blog, "status"); status != "draft" {
		t.Fatal("lost draft status")
	}
	if banner, _ := get(result.Document.Blog, "content.banner_image"); banner != "/banner.jpg" {
		t.Fatal("lost independent banner")
	}
	if poster, _ := get(result.Document.Blog, "content.poster.headline"); poster != "Hello" {
		t.Fatal("lost poster")
	}
	if !strings.Contains(result.Document.Body, "```md\n{% unknown a %}\n{% tip code %}\n```") || !strings.Contains(result.Document.Body, "{% blog_tip 'keep spacing' %}") {
		t.Fatal("code or shortcode parameters changed")
	}
}

func TestLegacyMarkdownRejectsUnsupportedUnknownAndConflictingFields(t *testing.T) {
	for _, header := range []string{
		"password: secret", "authors: [alice]", "publish_at: 2030-01-01", "draft_json: '{}'", "mystery: true", "status: private", "pin: true\npinned: false", "category: One\ncategories: [Two]", "author: Alice\narticle: {author: Bob}", "pinned: true\nlisting: false", "status: draft\nblog: {status: published}", "poster: {unknown: value}", "poster: wrong", "poster: {headline: One}\nposter_headline: Two", "banner: /old.jpg\nblog: {content: {banner_image: /new.jpg}}", "slug: ../escape", "blog: {host: {password: secret}}",
	} {
		t.Run(header, func(t *testing.T) {
			if _, err := ConvertLegacyMarkdown([]byte("---\n"+header+"\n---\nbody"), "old.md", "post", nil); err == nil {
				t.Fatalf("accepted %s", header)
			}
		})
	}
	for _, body := range []string{"{% mystery a %}", "{% links\narg %}", "{% tip 'unclosed %}"} {
		if _, err := ConvertLegacyMarkdown([]byte("---\ntitle: Same\n---\n"+body), "old.md", "post", nil); err == nil {
			t.Fatalf("accepted unsafe shortcode %q", body)
		}
	}
}

func TestLegacyMarkdownTopicMustMatchExactlyOneExistingTopic(t *testing.T) {
	for _, topics := range [][]*store.Topic{nil, {{Slug: "one", Name: "Shared"}, {Slug: "Shared", Name: "Two"}}} {
		if _, err := ConvertLegacyMarkdown([]byte("---\ntopic: Shared\n---\nbody"), "old.md", "post", topics); err == nil {
			t.Fatal("guessed missing or ambiguous topic")
		}
	}
}

func TestLegacyMarkdownEmptyTopicIsUnassigned(t *testing.T) {
	for _, header := range []string{"topic:", "topic: null", "topic: ~", `topic: ""`, `topic: "   "`} {
		for _, kind := range []string{"post", "page"} {
			for _, topics := range [][]*store.Topic{nil, {{Slug: "series", Name: "Series"}}} {
				t.Run(header+"/"+kind+fmt.Sprint(len(topics)), func(t *testing.T) {
					body := "TeamSpeak 正文。\r\n\n```md\n{% tip 示例 %}\n```\n"
					source := []byte("---\ntitle: TeamSpeak使用指南\n" + header + "\nmathjax: true\n---\n" + body)
					result, err := ConvertLegacyMarkdown(source, "TeamSpeak使用指南.md", kind, topics)
					if err != nil {
						t.Fatal(err)
					}
					if result.Document.Body != body || result.Document.Raw["title"] != "TeamSpeak使用指南" {
						t.Fatal("empty topic changed article content")
					}
					if _, exists := result.Document.Raw["topic"]; exists {
						t.Fatal("empty legacy topic was not removed")
					}
					if _, exists := result.Document.Raw["collection"]; exists {
						t.Fatal("empty topic guessed a collection")
					}
					if !result.LegacyConverted || !strings.Contains(strings.Join(result.Warnings, "\n"), "topic 为空") {
						t.Fatal("empty topic conversion was not disclosed")
					}
				})
			}
		}
	}
}

func TestLegacyMarkdownTopicErrorsExplainResolution(t *testing.T) {
	for _, test := range []struct {
		header, message string
		topics          []*store.Topic
	}{
		{"topic: Missing", "未找到专栏", nil},
		{"topic: Shared", "匹配到多个专栏", []*store.Topic{{Slug: "one", Name: "Shared"}, {Slug: "Shared", Name: "Two"}}},
		{"topic: ' Series '", "未找到专栏", []*store.Topic{{Slug: "series", Name: "Series"}}},
		{"topic: false", "topic 必须是专栏名称或 Slug 字符串", nil},
		{"topic: 42", "topic 必须是专栏名称或 Slug 字符串", nil},
		{"topic: []", "topic 必须是专栏名称或 Slug 字符串", nil},
		{"topic: {}", "topic 必须是专栏名称或 Slug 字符串", nil},
	} {
		t.Run(test.header, func(t *testing.T) {
			_, err := ConvertLegacyMarkdown([]byte("---\n"+test.header+"\n---\n正文"), "TeamSpeak使用指南.md", "post", test.topics)
			if err == nil || !strings.Contains(err.Error(), test.message) || !strings.Contains(err.Error(), "TeamSpeak使用指南.md") {
				t.Fatalf("expected actionable topic error %q, got %v", test.message, err)
			}
		})
	}
}

func TestLegacyMarkdownEmptyTopicPreservesExplicitCollection(t *testing.T) {
	for _, header := range []string{
		"topic:\ncollection: {profile: topic, id: series}",
		"topic: ''\nwiki: docs",
	} {
		result, err := ConvertLegacyMarkdown([]byte("---\n"+header+"\n---\n正文"), "empty-topic.md", "post", nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, exists := result.Document.Raw["collection"]; !exists {
			t.Fatal("empty topic removed an explicit collection")
		}
	}
}

func TestLegacyMarkdownCanonicalAndBodyOnlyMigration(t *testing.T) {
	for _, kind := range []string{"post", "page"} {
		source := ";;;\n{\"title\":\"V2\",\"blog\":{\"status\":\"published\"}}\n;;;\n{% image /same.jpg fancybox:true %}\n```\n{% tip code %}\n```\n"
		result, err := ConvertLegacyMarkdown([]byte(source), "v2.md", kind, nil)
		if err != nil || result.LegacyConverted || result.Document.Body != "{% image /same.jpg fancybox:true %}\n```\n{% tip code %}\n```\n" {
			t.Fatalf("canonical document changed: %+v %v", result, err)
		}
		legacy, err := ConvertLegacyMarkdown([]byte("---\ntitle: Old\n---\n{% tip old %}\n{% image /same.jpg %}"), "v1.md", kind, nil)
		if err != nil || !legacy.LegacyConverted || !strings.Contains(legacy.Document.Body, "{% blog_tip old %}") || !strings.Contains(legacy.Document.Body, "fancybox:false") {
			t.Fatalf("old body was not converted: %+v %v", legacy, err)
		}
		encoded, err := json.Marshal(legacy.Document.Raw)
		if err != nil || string(encoded) != `{"title":"Old"}` {
			t.Fatal("body migration changed canonical fields")
		}
	}
}

func TestLegacyMarkdownImageDefaultDoesNotDependOnOtherTags(t *testing.T) {
	for _, suffix := range []string{"", "\n{% tip old %}"} {
		source := []byte("---\ntitle: Old\n---\n{% image /same.jpg %}" + suffix)
		strict, err := post.ParseStellarDocument(source, post.StellarDocumentOptions{})
		if err != nil || !strings.Contains(strict.Body, "{% image /same.jpg %}") {
			t.Fatalf("strict import changed image: %+v %v", strict, err)
		}
		legacy, err := ConvertLegacyMarkdown(source, "old.md", "post", nil)
		if err != nil || !legacy.LegacyConverted || !strings.Contains(legacy.Document.Body, "fancybox:false") {
			t.Fatalf("legacy image lost default with suffix %q: %+v %v", suffix, legacy, err)
		}
	}
}

func TestLegacyMarkdownRenderAndStyleFieldsPreserveExistingContent(t *testing.T) {
	for _, test := range []struct {
		name, flags, style string
		math, diagrams     any
	}{
		{"enabled-tech", "mathjax: true\nmermaid: true", "tech", "mathjax", "mermaid"},
		{"disabled-story", "mathjax: false\nmermaid: false", "story", false, false},
		{"math-only", "mathjax: true\nmermaid: false", "tech", "mathjax", false},
		{"diagrams-only", "mathjax: false\nmermaid: true", "story", false, "mermaid"},
		{"existing-diagrams", "mathjax: true\nrender: {diagrams: mermaid}", "tech", "mathjax", "mermaid"},
		{"existing-math", "mermaid: true\nrender: {math: katex}", "story", "katex", "mermaid"},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := "正文\r\n\n```mermaid\ngraph LR\n A --> B\n```\n\n$a+b$\n"
			source := "---\ntitle: Old\n" + test.flags + "\ntype: " + test.style + "\nreferences: [https://example.com/ref, '42']\ntopic: Series\nbanner: /old.jpg\narticle: {author: Alice}\n---\n" + body
			if _, err := post.ParseStellarDocument([]byte(source), post.StellarDocumentOptions{}); err == nil {
				t.Fatal("strict parsing accepted legacy fields")
			}
			result, err := ConvertLegacyMarkdown([]byte(source), "old.md", "post", []*store.Topic{{Slug: "series", Name: "Series"}})
			if err != nil {
				t.Fatal(err)
			}
			for path, want := range map[string]any{
				"render.math": test.math, "render.diagrams": test.diagrams,
				"article.style": test.style, "article.author": "Alice", "collection.id": "series",
				"footer.references": []any{"https://example.com/ref", "42"},
			} {
				got, exists := get(result.Document.Raw, path)
				if !exists || !reflect.DeepEqual(got, want) {
					t.Errorf("%s=%#v, want %#v", path, got, want)
				}
			}
			if got, _ := get(result.Document.Blog, "content.banner_image"); got != "/old.jpg" {
				t.Fatalf("banner changed: %#v", got)
			}
			if !result.LegacyConverted || result.Document.Body != body {
				t.Fatalf("conversion metadata or Markdown changed: %+v", result)
			}
			for _, old := range []string{"mathjax", "mermaid", "type", "references", "topic", "banner"} {
				if _, exists := result.Document.Raw[old]; exists {
					t.Errorf("legacy field %s was not removed", old)
				}
			}
		})
	}
}

func TestLegacyMarkdownRenderAndStyleRejectInvalidValuesAndConflicts(t *testing.T) {
	for _, field := range []string{"mathjax", "mermaid"} {
		for _, value := range []string{`"true"`, `"false"`, "1", "null", "[]", "{}"} {
			header := field + ": " + value
			t.Run(header, func(t *testing.T) {
				if _, err := ConvertLegacyMarkdown([]byte("---\n"+header+"\n---\nbody"), "old.md", "post", nil); err == nil {
					t.Fatalf("accepted non-boolean %s", header)
				}
			})
		}
	}
	for _, value := range []string{"unknown", `""`, "true", "1", "null", "[]", "{}"} {
		if _, err := ConvertLegacyMarkdown([]byte("---\ntype: "+value+"\n---\nbody"), "old.md", "post", nil); err == nil {
			t.Errorf("accepted invalid article style %s", value)
		}
	}
	for _, header := range []string{
		"mathjax: true\nrender: {math: mathjax}", "mathjax: false\nrender: {math: false}", "mathjax: true\nrender: false",
		"mermaid: true\nrender: {diagrams: mermaid}", "mermaid: false\nrender: {diagrams: false}", "mermaid: true\nrender: false",
		"type: tech\narticle: {style: tech}", "type: story\narticle: false",
		"references: []\nfooter: {references: []}", "references: []\nfooter: false",
		"topic: Series\ncollection: {profile: topic, id: series}", "topic: Series\ncollection: false",
		"banner: /old.jpg\nblog: false", "banner: /old.jpg\nblog: {content: false}",
	} {
		t.Run(header, func(t *testing.T) {
			_, err := ConvertLegacyMarkdown([]byte("---\n"+header+"\n---\nbody"), "old.md", "post", []*store.Topic{{Slug: "series", Name: "Series"}})
			if err == nil || !strings.Contains(err.Error(), "conflicts with") {
				t.Fatalf("expected explicit conflict for %s, got %v", header, err)
			}
		})
	}
}
