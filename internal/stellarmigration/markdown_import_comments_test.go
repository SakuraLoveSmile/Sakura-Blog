package stellarmigration

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"blog/internal/post"
)

func TestLegacyMarkdownBooleanCommentsPreserveSwitchAndBody(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		for _, kind := range []string{"post", "page"} {
			t.Run(fmt.Sprint(enabled)+"/"+kind, func(t *testing.T) {
				body := "正文\r\n```yaml\ncomments: untouched\n```\n"
				source := []byte(fmt.Sprintf("---\ntitle: Keep\ncomments: %t\n---\n%s", enabled, body))
				if _, err := post.ParseStellarDocument(source, post.StellarDocumentOptions{}); err == nil {
					t.Fatal("strict import accepted legacy boolean comments")
				}
				result, err := ConvertLegacyMarkdown(source, "Perfdog.md", kind, nil)
				if err != nil {
					t.Fatal(err)
				}
				comments, _ := get(result.Document.Raw, "comments.enabled")
				if comments != enabled || result.Document.Body != body || !result.LegacyConverted {
					t.Fatalf("comment switch or body lost: %+v", result)
				}
				native, exists := get(result.Document.Blog, "native_comments.enabled")
				if enabled && exists || !enabled && (!exists || native != false) {
					t.Fatalf("legacy switch changed native enablement: %+v", result.Document.Blog)
				}
			})
		}
	}
}

func TestLegacyMarkdownCommentsObjectUnknownFieldsStillRequireConsent(t *testing.T) {
	source := []byte("---\ncomments: {enabled: false, extra/key: true}\n---\nbody")
	_, err := ConvertLegacyMarkdown(source, "comments.md", "post", nil)
	var problem *UnsupportedMarkdownParametersError
	if !errors.As(err, &problem) || len(problem.Parameters) != 1 || problem.Parameters[0].Path != "/comments/extra~1key" {
		t.Fatalf("comments object lost precise unsupported diagnostics: %+v %v", problem, err)
	}
	result, err := ConvertLegacyMarkdownWithIgnoredParameters(source, "comments.md", "post", nil, []string{problem.Parameters[0].Path})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Document.Raw["comments"], map[string]any{"enabled": false}) {
		t.Fatal("ignoring unknown option removed supported comments setting")
	}
	if _, exists := get(result.Document.Blog, "native_comments.enabled"); exists {
		t.Fatal("canonical object was treated as a legacy boolean switch")
	}
}

func TestLegacyMarkdownCommentsKeepCanonicalObjectsAndExplicitNativeSettings(t *testing.T) {
	for _, header := range []string{
		"comments: {enabled: false, options: {custom_limit: 9007199254740993}}\nmermaid: false",
		"comments: true\nblog: {native_comments: {enabled: false, title: Keep}}",
	} {
		source := []byte("---\n" + header + "\n---\nbody")
		result, err := ConvertLegacyMarkdown(source, "comments.md", "post", nil)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(header, "custom_limit") {
			value, _ := get(result.Document.Raw, "comments.options.custom_limit")
			if fmt.Sprint(value) != "9007199254740993" {
				t.Fatal("canonical comments options changed")
			}
		} else if !reflect.DeepEqual(result.Document.Blog["native_comments"], map[string]any{"enabled": false, "title": "Keep"}) {
			t.Fatal("explicit native settings changed")
		}
	}
	for _, native := range []string{"{native_comments: {enabled: true}}", "{native_comments: {enabled: false}}", "{native_comments: null}", "false"} {
		source := []byte("---\ncomments: false\nblog: " + native + "\n---\nbody")
		if _, err := ConvertLegacyMarkdown(source, "conflict.md", "post", nil); err == nil {
			t.Fatalf("silently overwrote explicit native target: %s", native)
		}
	}
	for _, value := range []string{"null", "7", "'true'", "[]"} {
		source := []byte("---\ncomments: " + value + "\n---\nbody")
		if _, err := ConvertLegacyMarkdown(source, "invalid.md", "post", nil); err == nil {
			t.Fatalf("guessed invalid comment switch: %s", value)
		}
	}
}
