package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tmpl "blog/internal/settings"
)

func TestBootstrapBackfillsRuntimeDefaultsWithoutOverwritingExplicitValues(t *testing.T) {
	st := newTestStore(t)
	seed := map[string]string{
		SectionSearch:     `{"service":"local_search","local_search":{"content":false,"lazy_load":false,"cache_ttl":0,"custom":"keep"}}`,
		SectionDataCache:  `{"enable":false,"default_ttl":0,"max_entries":0,"ttl":{"friends":0,"custom":17}}`,
		SectionTagPlugins: `{"note":{"border":false},"checkbox":{"interactive":false},"quot":{"prefix":"legacy-left","suffix":"legacy-right"},"emoji":{"qq":""},"okr":{"status":{"custom":{"color":"pink","label":"自定义"}}},"custom":{"enabled":false}}`,
		SectionServices:   `{"siteinfo":{"api":"https://api.example.test?url={href}"},"custom":{"enabled":false}}`,
	}
	for section, raw := range seed {
		if err := st.PutSetting(section, raw); err != nil {
			t.Fatalf("seed %s: %v", section, err)
		}
	}

	site := newSitePtr()
	if err := Bootstrap(st, site.Load()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if err := RebuildSite(st, site); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	search := site.Load().Search.LocalSearch
	if search.Content == nil || *search.Content || search.LazyLoad == nil || *search.LazyLoad || search.CacheTTL == nil || *search.CacheTTL != 0 {
		t.Fatalf("explicit search values changed: %+v", search)
	}
	cache := site.Load().DataCache
	if cache.Enable == nil || *cache.Enable || cache.DefaultTTL == nil || *cache.DefaultTTL != 0 || cache.MaxEntries == nil || *cache.MaxEntries != 0 {
		t.Fatalf("explicit cache values changed: %+v", cache)
	}
	for _, key := range []string{"giscus", "waline", "artalk", "memos", "memos-user", "sites", "friends", "friends_and_posts", "siteinfo"} {
		if _, ok := cache.TTL[key]; !ok {
			t.Fatalf("missing service ttl %q: %#v", key, cache.TTL)
		}
	}
	if cache.TTL["friends"] != 0 || cache.TTL["custom"] != 17 {
		t.Fatalf("custom/explicit ttl changed: %#v", cache.TTL)
	}

	tag := site.Load().TagPlugins
	if tag.Note.Border == nil || *tag.Note.Border || tag.Checkbox.Interactive {
		t.Fatalf("explicit tag plugin switches changed: %+v", tag)
	}
	if got := tag.Quot.Default; got == nil || got.Prefix != "legacy-left" || got.Suffix != "legacy-right" {
		t.Fatalf("legacy quot was not migrated: %+v", got)
	}
	if tag.Emoji["qq"] != "" || tag.OKR.Status["custom"].Label != "自定义" {
		t.Fatalf("custom tag plugin values changed: %+v", tag)
	}
	if site.Load().Services.SiteInfo.API == "" {
		t.Fatal("services.siteinfo.api was not restored")
	}

	storedTag, err := st.GetSetting(SectionTagPlugins)
	if err != nil {
		t.Fatalf("read normalized tag plugins: %v", err)
	}
	var tagJSON map[string]any
	if err := json.Unmarshal([]byte(storedTag), &tagJSON); err != nil {
		t.Fatalf("normalized tag json: %v", err)
	}
	quotJSON, _ := tagJSON["quot"].(map[string]any)
	if _, ok := quotJSON["prefix"]; ok {
		t.Fatalf("legacy quot prefix should be removed after migration: %s", storedTag)
	}
	if _, ok := quotJSON["suffix"]; ok {
		t.Fatalf("legacy quot suffix should be removed after migration: %s", storedTag)
	}
	if _, ok := tagJSON["custom"]; !ok {
		t.Fatal("unknown tag plugin key should survive migration")
	}
	var searchJSON map[string]any
	searchRaw, _ := st.GetSetting(SectionSearch)
	_ = json.Unmarshal([]byte(searchRaw), &searchJSON)
	localSearch, _ := searchJSON["local_search"].(map[string]any)
	if localSearch["custom"] != "keep" {
		t.Fatalf("unknown search key should survive migration: %s", searchRaw)
	}
}

func TestRuntimeDefaultsRemainAvailableForZeroValueConfigs(t *testing.T) {
	search := tmpl.SearchConfig{}
	search.FillDefaults()
	if search.LocalSearch.LazyLoad == nil || !*search.LocalSearch.LazyLoad || search.LocalSearch.CacheTTL == nil || *search.LocalSearch.CacheTTL != 86400 {
		t.Fatalf("search defaults = %+v", search.LocalSearch)
	}
	cache := tmpl.DataCacheConfig{}
	cache.FillDefaults()
	if cache.DefaultTTL == nil || *cache.DefaultTTL != tmpl.DefaultDataCacheTTL || cache.MaxEntries == nil || *cache.MaxEntries != tmpl.DefaultDataCacheMaxEntries {
		t.Fatalf("cache defaults = %+v", cache)
	}
	if len(cache.TTL) != 9 {
		t.Fatalf("service ttl defaults = %#v", cache.TTL)
	}
}

func TestRuntimeDefaultsPreserveExplicitEmptySearchStrings(t *testing.T) {
	raw := `{"service":"","local_search":{"field":"","path":"","content":false}}`
	normalized, err := mergeRuntimeDefaults([]byte(raw), []byte(raw), SectionSearch)
	if err != nil {
		t.Fatalf("merge runtime defaults: %v", err)
	}
	parsed := parseSections(map[string]string{SectionSearch: string(normalized)}).search
	if parsed.Service != "" || parsed.LocalSearch.Field != "" || parsed.LocalSearch.Path != "" {
		t.Fatalf("explicit empty search strings changed: %+v", parsed)
	}
	if parsed.LocalSearch.Content == nil || *parsed.LocalSearch.Content {
		t.Fatalf("explicit false content changed: %+v", parsed.LocalSearch)
	}
}

func TestLoadSiteNormalizesStellarRuntimeDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "site.yml")
	if err := os.WriteFile(path, []byte("search:\n  local_search:\n    lazy_load: false\n    cache_ttl: 0\ndata_cache:\n  enable: false\n  default_ttl: 0\ntag_plugins:\n  note:\n    border: false\n  quot:\n    default:\n      prefix: custom-prefix\n  okr:\n    status:\n      custom:\n        color: pink\n        label: ''\nservices:\n  siteinfo:\n    api: https://api.example/{href}\n"), 0o600); err != nil {
		t.Fatalf("write site config: %v", err)
	}
	site, err := LoadSite(path)
	if err != nil {
		t.Fatalf("load site config: %v", err)
	}
	if site.Search.LocalSearch.LazyLoad == nil || *site.Search.LocalSearch.LazyLoad || site.Search.LocalSearch.CacheTTL == nil || *site.Search.LocalSearch.CacheTTL != 0 {
		t.Fatalf("yaml search values changed: %+v", site.Search.LocalSearch)
	}
	if site.DataCache.Enable == nil || *site.DataCache.Enable || site.DataCache.DefaultTTL == nil || *site.DataCache.DefaultTTL != 0 {
		t.Fatalf("yaml cache values changed: %+v", site.DataCache)
	}
	if site.TagPlugins.Note.Border == nil || *site.TagPlugins.Note.Border || site.Services.SiteInfo.API == "" {
		t.Fatalf("yaml tag/services values changed: %+v %+v", site.TagPlugins, site.Services)
	}
	if site.TagPlugins.Quot.Default == nil || site.TagPlugins.Quot.Default.Prefix != "custom-prefix" || site.TagPlugins.Quot.Default.Suffix != "quot:quote-right" {
		t.Fatalf("yaml quot nested defaults not merged: %+v", site.TagPlugins.Quot.Default)
	}
	if custom := site.TagPlugins.OKR.Status["custom"]; custom.Color != "pink" || custom.Label != "" {
		t.Fatalf("yaml explicit empty status label changed: %+v", custom)
	}
}

func TestMergeRuntimeDefaultsMigratesLegacyQuotValuesAndFillsNestedKeys(t *testing.T) {
	raw := []byte(`{"quot":{"prefix":"legacy-left","suffix":"legacy-right"},"okr":{"status":{"custom":{"color":"pink","label":""}}}}`)
	normalized, err := mergeRuntimeDefaults(raw, raw, SectionTagPlugins)
	if err != nil {
		t.Fatalf("merge runtime defaults: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(normalized, &got); err != nil {
		t.Fatalf("decode normalized tag plugins: %v", err)
	}
	quot := got["quot"].(map[string]any)
	if quot["prefix"] != nil || quot["suffix"] != nil {
		t.Fatalf("legacy quot keys remain: %#v", quot)
	}
	def := quot["default"].(map[string]any)
	if def["prefix"] != "legacy-left" || def["suffix"] != "legacy-right" {
		t.Fatalf("legacy quot values changed: %#v", def)
	}
	status := got["okr"].(map[string]any)["status"].(map[string]any)["custom"].(map[string]any)
	if status["color"] != "pink" || status["label"] != "" {
		t.Fatalf("explicit empty status label changed: %#v", status)
	}
}

func TestSaveRuntimeSectionsValidatesTTLAndPreservesServices(t *testing.T) {
	st := newTestStore(t)
	site := newSitePtr()
	if err := Bootstrap(st, site.Load()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if err := SaveSection(st, site, SectionDataCache, []byte(`{"default_ttl":-1}`)); err == nil {
		t.Fatal("negative default ttl should be rejected")
	}
	if err := SaveSection(st, site, SectionDataCache, []byte(`{"max_entries":0}`)); err == nil {
		t.Fatal("non-positive max entries should be rejected")
	}
	if err := SaveSection(st, site, SectionServices, []byte(`{"siteinfo":{"api":"https://api.example/{href}"},"custom":{"enabled":false}}`)); err != nil {
		t.Fatalf("save services: %v", err)
	}
	if site.Load().Services.SiteInfo.API != "https://api.example/{href}" {
		t.Fatalf("services hot update failed: %+v", site.Load().Services)
	}
	stored, err := st.GetSetting(SectionServices)
	if err != nil || !strings.Contains(stored, `"custom":{"enabled":false}`) {
		t.Fatalf("services custom key lost: %s (%v)", stored, err)
	}
}
