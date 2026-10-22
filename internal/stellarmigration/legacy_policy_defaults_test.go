package stellarmigration

import (
	"os"
	"strings"
	"testing"
)

// 固定官方 runtime policy 的完整旧形状，不读取生产配置。
const officialCacheFixture = `{"enable":true,"default_ttl":3600,"max_entries":200,"ttl":{"giscus":600,"waline":600,"artalk":600,"memos":600,"memos-user":86400,"sites":86400,"friends":86400,"friends_and_posts":86400,"siteinfo":86400}}`

func TestExactLegacyCacheMatchesLockedOfficialRuntime(t *testing.T) {
	bytes, err := os.ReadFile("../../modules/stellar/clientconfig/resources.json")
	if err != nil {
		t.Fatal(err)
	}
	resources, err := decode(string(bytes))
	if err != nil {
		t.Fatal(err)
	}
	if object(resources)["commit"] != "9d11978a24fe6610e3a7dbab741cf62634539d05" {
		t.Fatal("runtime constants are not pinned to reviewed upstream")
	}
	raw, _ := get(object(resources), "constants.runtime.cache")
	cache := object(raw)
	expected := map[string]any{"enable": cache["enabled"], "default_ttl": cache["defaultTtl"], "max_entries": cache["maxEntries"], "ttl": cache["ttl"]}
	fixture, err := decode(officialCacheFixture)
	if err != nil {
		t.Fatal(err)
	}
	if encode(expected) != encode(fixture) || !isOfficialLegacyDataCache(expected) {
		t.Fatal("legacy default is not identical to complete official runtime cache tree")
	}
	s := settingsSnapshot(t, map[string]any{"data_cache": expected})
	c := candidate(t, s)
	assertNoPolicyLoss(t, c, s, "data_cache_removed", "settings.data_cache")
	theme, _ := decode(c.Theme.ThemeJSON)
	if _, exists := get(object(theme), "features.data_cache"); exists {
		t.Fatal("removed configuration injected into strict schema")
	}
}

func TestIncompleteOrCustomCacheRemainsBlocking(t *testing.T) {
	cases := map[string]func(map[string]any){
		"disabled":           func(m map[string]any) { m["enable"] = false },
		"wrong enable type":  func(m map[string]any) { m["enable"] = "true" },
		"null enable":        func(m map[string]any) { m["enable"] = nil },
		"different default":  func(m map[string]any) { m["default_ttl"] = 3601 },
		"different capacity": func(m map[string]any) { m["max_entries"] = 201 },
		"null ttl":           func(m map[string]any) { m["ttl"] = nil },
		"extra root":         func(m map[string]any) { m["unknown"] = false },
		"missing enable":     func(m map[string]any) { delete(m, "enable") },
		"missing default":    func(m map[string]any) { delete(m, "default_ttl") },
		"missing capacity":   func(m map[string]any) { delete(m, "max_entries") },
		"missing ttl":        func(m map[string]any) { delete(m, "ttl") },
		"extra ttl":          func(m map[string]any) { object(m["ttl"])["unknown"] = 600 },
		"wrong ttl type":     func(m map[string]any) { object(m["ttl"])["giscus"] = "600" },
	}
	// 每个服务单独差分，防止只比较一部分服务树。
	for _, service := range []string{"giscus", "waline", "artalk", "memos", "memos-user", "sites", "friends", "friends_and_posts", "siteinfo"} {
		cases["custom "+service] = func(m map[string]any) { object(m["ttl"])[service] = 0 }
		cases["missing "+service] = func(m map[string]any) { delete(object(m["ttl"]), service) }
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			value, _ := decode(officialCacheFixture)
			change(object(value))
			assertPolicyBlocked(t, "data_cache", value, "data_cache_removed")
		})
	}
	for name, value := range map[string]any{"null": nil, "empty": map[string]any{}, "string": ""} {
		t.Run(name, func(t *testing.T) { assertPolicyBlocked(t, "data_cache", value, "data_cache_removed") })
	}
}

func TestInactiveHomeCoverKeepsSourceWithoutInventingHero(t *testing.T) {
	for name, value := range map[string]any{
		"description":   map[string]any{"title": "", "description": "PRIVATE_UNUSED_DESCRIPTION"},
		"empty fields":  map[string]any{"title": "", "description": ""},
		"missing title": map[string]any{"description": "PRIVATE_UNUSED_DESCRIPTION"},
		"empty object":  map[string]any{},
	} {
		t.Run(name, func(t *testing.T) {
			s := settingsSnapshot(t, map[string]any{"cover": value})
			c := candidate(t, s)
			assertNoPolicyLoss(t, c, s, "home_cover_removed", "settings.cover")
			if strings.Contains(c.Theme.ThemeJSON, "PRIVATE_UNUSED_DESCRIPTION") || strings.Contains(encode(c.PublicReport()), "PRIVATE_UNUSED_DESCRIPTION") {
				t.Fatal("inactive cover invented visible Hero or leaked private source")
			}
		})
	}
}

func TestActiveUnknownOrInvalidHomeCoverRemainsBlocking(t *testing.T) {
	for name, value := range map[string]any{
		"active":            map[string]any{"title": "Visible", "description": "text"},
		"unknown field":     map[string]any{"title": "", "description": "text", "features": []any{}},
		"null title":        map[string]any{"title": nil},
		"wrong title":       map[string]any{"title": false},
		"null description":  map[string]any{"title": "", "description": nil},
		"wrong description": map[string]any{"title": "", "description": []any{}},
		"null":              nil, "string": "", "array": []any{},
	} {
		t.Run(name, func(t *testing.T) { assertPolicyBlocked(t, "cover", value, "home_cover_removed") })
	}
}

func assertNoPolicyLoss(t *testing.T, c Candidate, s Snapshot, code, source string) {
	t.Helper()
	if hasCode(c.Theme.Diagnostics, code) || c.PublicReport().Blocking != 0 {
		t.Fatal("exact equivalent source still requires policy choice")
	}
	if encode(c.Source) != encode(s) {
		t.Fatal("equivalent source was discarded")
	}
	found := false
	for _, mapping := range c.Theme.Mappings {
		if mapping.Source == source && mapping.Target == "source.snapshot" {
			found = true
		}
	}
	if !found {
		t.Fatal("preview omitted source inheritance explanation")
	}
	if err := ValidateCandidate(c, s); err != nil {
		t.Fatal(err)
	}
}

func assertPolicyBlocked(t *testing.T, section string, value any, code string) {
	t.Helper()
	s := settingsSnapshot(t, map[string]any{section: value})
	c := candidate(t, s)
	if !hasCode(c.Theme.Diagnostics, code) || c.PublicReport().Blocking == 0 || ValidateCandidate(c, s) == nil {
		t.Fatal("custom or malformed policy silently inherited")
	}
	if encode(c.Source) != encode(s) {
		t.Fatal("blocked source changed")
	}
}
