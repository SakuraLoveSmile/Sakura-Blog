package post

import (
	"reflect"
	"testing"
)

func TestParseImportReportsFieldsAndKeepsBody(t *testing.T) {
	src := "---\nslug: hello-world\ntitle: 标题\ntags: [Go, Stellar]\ncustom: keep\n---\n\n{% tip text:提示 %}\n正文\n{% endtip %}\n"
	result, err := newParser().ParseImport([]byte(src))
	if err != nil {
		t.Fatalf("ParseImport error: %v", err)
	}
	if !result.HasFrontMatter {
		t.Fatal("expected front matter")
	}
	if result.Body != "{% tip text:提示 %}\n正文\n{% endtip %}\n" {
		t.Fatalf("body changed: %q", result.Body)
	}
	if result.Meta.Slug != "hello-world" || result.Meta.Title != "标题" {
		t.Fatalf("meta = %+v", result.Meta)
	}
	if !reflect.DeepEqual(result.Meta.Tags, []string{"Go", "Stellar"}) {
		t.Fatalf("tags = %#v", result.Meta.Tags)
	}
	if !reflect.DeepEqual(result.UnknownFields, []string{"custom"}) {
		t.Fatalf("unknown fields = %#v", result.UnknownFields)
	}
	if !reflect.DeepEqual(result.Fields, []string{"custom", "slug", "tags", "title"}) {
		t.Fatalf("fields = %#v", result.Fields)
	}
}

func TestParseImportRejectsMalformedFrontMatter(t *testing.T) {
	for _, src := range []string{"---\ntitle: no close\n正文", "---\ntitle: [bad\n---\n正文"} {
		if _, err := newParser().ParseImport([]byte(src)); err == nil {
			t.Fatalf("expected parse error for %q", src)
		}
	}
}

func TestParseImportWithoutFrontMatterIsVerbatim(t *testing.T) {
	src := "# 标题\n\n{% unknown foo %}\n"
	result, err := newParser().ParseImport([]byte(src))
	if err != nil {
		t.Fatalf("ParseImport error: %v", err)
	}
	if result.HasFrontMatter || result.Body != src || len(result.Fields) != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}
