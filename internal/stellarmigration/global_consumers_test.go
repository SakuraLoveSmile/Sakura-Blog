package stellarmigration

import (
	"strings"
	"testing"
)

func globalSourceFixture() map[string]any {
	return map[string]any{
		"site":     map[string]any{"url": "https://fixture.example/", "avatar_bg": "/avatar-background.png"},
		"rightbar": map[string]any{"intro": "Fixture introduction", "welcome_linklist": []any{map[string]any{"title": "Fixture", "url": "/fixture/"}}},
		"nav_tabs": map[string]any{"enable_custom": true, "items": []any{}},
		"article":  map[string]any{"ai_label": map[string]any{"enabled": true}, "category_icon": map[string]any{"fixture": "default:folder"}, "tag_icon": map[string]any{}, "read_next": false, "expire_days": 30, "banner_ratio": 2, "edit_this_page": "https://edit.example/source/"},
		"notebook": map[string]any{"license": nil, "share": false},
		"pages":    map[string]any{"friends": map[string]any{"show_posts": false}},
		"seo":      map[string]any{"robots_extra": "PRIVATE_SEO_FIXTURE_NEVER_PUBLIC"},
	}
}
func allGlobalCapabilities() ConsumerCapabilities {
	capabilities := ConsumerCapabilities{}
	for _, capability := range globalConsumerTargets {
		capabilities[capability] = true
	}
	return capabilities
}
func TestStoredGlobalExtensionsNeedExplicitConsumerEvidence(t *testing.T) {
	s := settingsSnapshot(t, globalSourceFixture())
	c := candidate(t, s)
	if c.PublicReport().Blocking != 15 {
		t.Fatalf("stored global fields were silently claimed consumed: blockers=%d", c.PublicReport().Blocking)
	}
	seen := map[ConsumerCapability]bool{}
	for _, d := range c.Theme.Diagnostics {
		if d.Kind != "consumer" || !d.Blocking || d.Severity != "error" {
			t.Fatal("consumer evidence diagnostic misclassified")
		}
		seen[d.Consumer] = true
	}
	if len(seen) != 15 {
		t.Fatal("global consumers were grouped or omitted")
	}
	if strings.Contains(encode(c.PublicReport()), "PRIVATE_SEO_FIXTURE_NEVER_PUBLIC") {
		t.Fatal("public diagnostic leaked source value")
	}
	if encode(c.Source) != encode(s) {
		t.Fatal("adding evidence gates changed private source")
	}
	closed, err := PreviewWithCapabilities(s, allGlobalCapabilities())
	if err != nil {
		t.Fatal(err)
	}
	if closed.PublicReport().Blocking != 0 || closed.Theme.ThemeJSON != c.Theme.ThemeJSON || closed.Theme.BlogJSON != c.Theme.BlogJSON || encode(closed.Theme.Mappings) != encode(c.Theme.Mappings) {
		t.Fatal("consumer declaration rewrote or dropped candidate values")
	}
	if len(closed.Theme.Diagnostics) != len(c.Theme.Diagnostics) {
		t.Fatal("consumer declaration deleted diagnostics")
	}
	for _, d := range closed.Theme.Diagnostics {
		if d.Kind != "consumer" || d.Blocking || d.Severity != "info" {
			t.Fatal("explicit evidence declaration not reflected")
		}
	}
	if err = ValidateCandidate(closed, s); err != nil {
		t.Fatal(err)
	}
	caps := allGlobalCapabilities()
	caps[ConsumerReadNext] = false
	partial, err := PreviewWithCapabilities(s, caps)
	if err != nil || partial.PublicReport().Blocking != 1 {
		t.Fatal("false capability declaration closed the gate")
	}
}
func TestFalseNullAndEmptyOverridesAreNotEvidenceOfAConsumer(t *testing.T) {
	for name, value := range map[string]any{"false": false, "null": nil, "empty array": []any{}, "empty object": map[string]any{}, "empty string": ""} {
		t.Run(name, func(t *testing.T) {
			s := settingsSnapshot(t, map[string]any{"article": map[string]any{"read_next": value}})
			c := candidate(t, s)
			if c.PublicReport().Blocking != 1 || c.Theme.Diagnostics[0].Consumer != ConsumerReadNext {
				t.Fatal("explicit override treated as no source")
			}
			blog, err := decode(c.Theme.BlogJSON)
			if err != nil {
				t.Fatal(err)
			}
			kept, exists := get(object(blog), "content.read_next")
			if !exists || encode(kept) != encode(value) {
				t.Fatal("false/null/empty source override lost")
			}
		})
	}
}
func TestGlobalCapabilitiesDoNotResolvePolicyChoices(t *testing.T) {
	sections := globalSourceFixture()
	sections["data_cache"] = map[string]any{"enabled": true}
	s := settingsSnapshot(t, sections)
	c, err := PreviewWithCapabilities(s, allGlobalCapabilities())
	if err != nil {
		t.Fatal(err)
	}
	if c.PublicReport().Blocking != 1 || ValidateCandidate(c, s) == nil {
		t.Fatal("global consumer flags resolved unrelated cache policy")
	}
	found := false
	for _, d := range c.Theme.Diagnostics {
		if d.Code == "data_cache_removed" {
			found = true
			if !d.Blocking || d.Kind != "policy" {
				t.Fatal("policy classification changed")
			}
		}
	}
	if !found {
		t.Fatal("policy diagnosis removed")
	}
}
