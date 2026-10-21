package stellarmigration

import (
	"errors"
	"testing"
)

func TestLegacyAuthorConsentOnlyAppliesToUnresolvedUnconflictedAliases(t *testing.T) {
	for _, test := range []struct {
		name, header string
		catalog      map[string]bool
		prompt, bad  bool
	}{
		{"missing", "author: old", map[string]bool{}, true, false},
		{"valid", "author: sakura", map[string]bool{"sakura": true}, false, false},
		{"invalid-name", "author: old", map[string]bool{"old": false}, true, false},
		{"exact-id", "author: Sakura", map[string]bool{"sakura": true}, true, false},
		{"no-id-trimming", "author: ' sakura '", map[string]bool{"sakura": true}, true, false},
		{"no-context", "author: old", nil, false, false},
		{"null", "author:", map[string]bool{}, false, false},
		{"empty", "author: ''", map[string]bool{}, false, false},
		{"number", "author: 1", map[string]bool{}, false, true},
		{"boolean", "author: false", map[string]bool{}, false, true},
		{"object", "author: {name: old}", map[string]bool{}, false, true},
		{"array", "author: [old]", map[string]bool{}, false, true},
		{"target-conflict", "author: old\narticle: {author: null}", map[string]bool{}, false, true},
		{"parent-conflict", "author: old\narticle: false", map[string]bool{}, false, true},
		{"canonical", "article: {author: old}", map[string]bool{}, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := []byte("---\n" + test.header + "\n---\nbody")
			options := LegacyMarkdownImportOptions{PublicAuthorIDs: test.catalog}
			parameters, err := InspectLegacyMarkdownParametersWithOptions(source, "old.md", options)
			if err != nil || (len(parameters) == 1) != test.prompt || len(parameters) > 1 {
				t.Fatalf("wrong optional parameter classification: %+v %v", parameters, err)
			}
			_, err = ConvertLegacyMarkdownWithOptions(source, "old.md", "post", options)
			var unsupported *UnsupportedMarkdownParametersError
			if test.prompt {
				if !errors.As(err, &unsupported) || unsupported.Parameters[0].Path != "/author" {
					t.Fatalf("unresolved author not prompted: %v", err)
				}
			} else if (err != nil) != test.bad {
				t.Fatalf("unexpected conversion: %v", err)
			}
			options.IgnoredParameters = []string{"/author"}
			result, err := ConvertLegacyMarkdownWithOptions(source, "old.md", "post", options)
			if test.prompt {
				if err != nil || result.Document.Body != "body" || result.Document.Blog["status"] != "draft" {
					t.Fatalf("confirmed author still blocked: %+v %v", result, err)
				}
			} else if err == nil {
				t.Fatal("compatible or invalid author could be erased by forged consent")
			}
		})
	}
}
