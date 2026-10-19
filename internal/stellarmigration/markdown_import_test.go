package stellarmigration

import (
	"encoding/json"
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
