package stellarmigration

import (
	"blog/internal/stellarv2"
	"strings"
	"testing"
)

func TestSEOMigratesOfficialFieldsAndKeepsTypedDefaultsSeparate(t *testing.T) {
	source := map[string]any{
		"preconnect":          []any{"https://assets.example"},
		"canonical":           map[string]any{"originalHost": "https://public.example/", "officialHosts": []any{"public.example", "mirror.example"}},
		"open_graph":          map[string]any{"enable": false, "twitter_id": "@fixture"},
		"structured_data":     map[string]any{"links": []any{"https://social.example/profile"}},
		"default_description": "SEO fallback", "default_keywords": "seo,fixture", "robots_extra": "Disallow: /hidden/", "no_index": true, "google_verification": "PRIVATE_VERIFY_FIXTURE", "bing_verification": "", "baidu_verification": "",
	}
	s := settingsSnapshot(t, map[string]any{"site": map[string]any{"description": "Explicit site text", "keywords": "site,fixture"}, "seo": source})
	c := candidate(t, s)
	if c.PublicReport().Blocking != 1 || c.Theme.Diagnostics[0].Consumer != ConsumerSEO {
		t.Fatal("official fields or pending typed SEO misclassified")
	}
	raw, err := decode(c.Theme.ThemeJSON)
	if err != nil {
		t.Fatal(err)
	}
	theme := object(raw)
	for path, want := range map[string]any{"preconnect": source["preconnect"], "canonical.host": "https://public.example/", "canonical.allowed_hosts": object(source["canonical"])["officialHosts"], "open_graph.enabled": false, "open_graph.twitter_id": "@fixture", "structured_data.same_as": object(source["structured_data"])["links"]} {
		got, exists := get(theme, path)
		if !exists || encode(got) != encode(want) {
			t.Fatalf("canonical SEO target %s not preserved", path)
		}
	}
	if _, exists := get(theme, "open_graph.options"); exists {
		t.Fatal("invented OpenGraph options schema")
	}
	blog, err := decode(c.Theme.BlogJSON)
	if err != nil {
		t.Fatal(err)
	}
	data := object(blog)
	for path, want := range map[string]any{"site.description": "Explicit site text", "site.keywords": "site,fixture", "seo.default_description": "SEO fallback", "seo.default_keywords": "seo,fixture", "seo.no_index": true, "seo.google_verification": "PRIVATE_VERIFY_FIXTURE"} {
		got, exists := get(data, path)
		if !exists || got != want {
			t.Fatalf("independent SEO source %s overridden", path)
		}
	}
	for _, key := range []string{"preconnect", "canonical", "open_graph", "structured_data"} {
		if _, exists := object(data["seo"])[key]; exists {
			t.Fatal("canonical source duplicated into raw SEO consumer bag")
		}
	}
	if strings.Contains(encode(c.PublicReport()), "PRIVATE_VERIFY_FIXTURE") {
		t.Fatal("verification source value leaked")
	}
	if _, err = stellarv2.ParseTheme(theme, stellarv2.Options{}); err != nil {
		t.Fatal(err)
	}
	closed, err := PreviewWithCapabilities(s, ConsumerCapabilities{ConsumerSEO: true})
	if err != nil || ValidateCandidate(closed, s) != nil {
		t.Fatal("official canonical candidate cannot validate after explicit typed consumer evidence")
	}
	if encode(c.Source) != encode(s) {
		t.Fatal("migrating fields changed private source")
	}
}
func TestSEONullAndEmptyRulesAreExplicit(t *testing.T) {
	for _, enabled := range []any{nil, false, true} {
		s := settingsSnapshot(t, map[string]any{"seo": map[string]any{"preconnect": nil, "canonical": map[string]any{"originalHost": "", "officialHosts": nil}, "open_graph": map[string]any{"enable": enabled, "twitter_id": ""}, "structured_data": map[string]any{"links": nil}}})
		c := candidate(t, s)
		if c.PublicReport().Blocking != 0 {
			t.Fatal("only official equivalences need no typed consumer")
		}
		raw, _ := decode(c.Theme.ThemeJSON)
		theme := object(raw)
		for _, path := range []string{"preconnect", "canonical.allowed_hosts", "structured_data.same_as"} {
			got, exists := get(theme, path)
			if !exists || encode(got) != "[]" {
				t.Fatalf("nil slice %s not explicitly empty", path)
			}
		}
		if host, exists := get(theme, "canonical.host"); !exists || host != nil {
			t.Fatal("empty canonical host not null")
		}
		want := enabled
		if want == nil {
			want = true
		}
		got, _ := get(theme, "open_graph.enabled")
		if got != want {
			t.Fatal("old nil Enabled=true or explicit false not preserved")
		}
		if err := ValidateCandidate(c, s); err != nil {
			t.Fatal(err)
		}
	}
}
func TestSEOUnknownAndBadSourceCannotBeClosedByConsumerEvidence(t *testing.T) {
	for name, value := range map[string]any{
		"unknown root":   map[string]any{"custom": "PRIVATE_UNKNOWN_SEO"},
		"unknown nested": map[string]any{"canonical": map[string]any{"custom": "PRIVATE_UNKNOWN_SEO"}},
		"wrong enabled":  map[string]any{"open_graph": map[string]any{"enable": "false"}},
		"invalid list":   map[string]any{"preconnect": []any{"https://assets.example", 4}},
		"wrong host":     map[string]any{"canonical": map[string]any{"originalHost": true}},
		"wrong no index": map[string]any{"no_index": "false"},
	} {
		t.Run(name, func(t *testing.T) {
			s := settingsSnapshot(t, map[string]any{"seo": value})
			c, err := PreviewWithCapabilities(s, ConsumerCapabilities{ConsumerSEO: true})
			if err != nil {
				t.Fatal(err)
			}
			if c.PublicReport().Blocking == 0 || ValidateCandidate(c, s) == nil {
				t.Fatal("consumer capability erased invalid/unknown schema source")
			}
			if strings.Contains(encode(c.PublicReport()), "PRIVATE_UNKNOWN_SEO") {
				t.Fatal("unknown source value leaked")
			}
			if encode(c.Source) != encode(s) {
				t.Fatal("invalid source changed")
			}
		})
	}
}
func TestSEOConflictingCanonicalTargetIsNotOverwritten(t *testing.T) {
	b := themeBuilder{raw: map[string]any{"seo": map[string]any{"canonical": map[string]any{"originalHost": "source.example"}}}, theme: map[string]any{"canonical": map[string]any{"host": "existing.example"}}, blog: map[string]any{}, used: map[string]bool{}}
	b.migrateSEO()
	if host, _ := get(b.theme, "canonical.host"); host != "existing.example" || !hasCode(b.diagnostics, "seo_source_conflict") {
		t.Fatal("different source silently overwrote canonical target")
	}
	b.diagnostics = classifyDiagnostics(b.diagnostics, ConsumerCapabilities{ConsumerSEO: true})
	found := false
	for _, d := range b.diagnostics {
		if d.Code == "seo_source_conflict" {
			found = true
			if !d.Blocking || d.Kind != "policy" {
				t.Fatal("consumer resolved source choice")
			}
		}
	}
	if !found {
		t.Fatal("conflict removed")
	}
}
func TestPagesUnsupportedHeroCannotHideBehindPagesCapability(t *testing.T) {
	for _, field := range []string{"animation", "start", "preview", "actions"} {
		t.Run(field, func(t *testing.T) {
			s := settingsSnapshot(t, map[string]any{"pages": map[string]any{"friends": map[string]any{"hero": map[string]any{field: map[string]any{"custom": "PRIVATE_HERO_SOURCE"}}}}})
			c, err := PreviewWithCapabilities(s, ConsumerCapabilities{ConsumerPages: true})
			if err != nil {
				t.Fatal(err)
			}
			if !hasCode(c.Theme.Diagnostics, "unmapped_setting") || c.PublicReport().Blocking == 0 || ValidateCandidate(c, s) == nil {
				t.Fatal("unsupported pages Hero claimed consumed")
			}
			if encode(c.Source) != encode(s) || strings.Contains(encode(c.PublicReport()), "PRIVATE_HERO_SOURCE") {
				t.Fatal("source dropped or leaked")
			}
		})
	}
}
