package config

import (
	"encoding/json"
	"path/filepath"
	"sync/atomic"
	"testing"

	"blog/internal/settings"
	"blog/internal/stellarv2"
	"blog/internal/store"
)

func TestRebuildPreservesStrictStellarThemeAcrossLocalSettingChanges(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "site.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	base := DefaultSite()
	if err := Bootstrap(st, base); err != nil {
		t.Fatal(err)
	}
	// 夹具明确提供已审核的版本文档；这不是生产自动迁移。
	if err := st.PutSettingsSnapshot(map[string]string{
		SectionStellarV2:      `{"version":2,"theme":{"appearance":{"preset":"minimal","color_scheme":"light"}}}`,
		SectionBlogExtensions: `{}`,
	}); err != nil {
		t.Fatal(err)
	}
	pointer := &atomic.Pointer[settings.Site]{}
	pointer.Store(base)
	if err := RebuildSite(st, pointer); err != nil {
		t.Fatal(err)
	}
	if value, _ := stellarv2.Get(pointer.Load().StellarV2.Theme, "appearance.preset"); value != "minimal" {
		t.Fatal("theme not loaded from canonical document")
	}
	if err := SaveSection(st, pointer, SectionSite, []byte(`{"title":"Updated","url":"https://example.test"}`)); err != nil {
		t.Fatal(err)
	}
	if value, _ := stellarv2.Get(pointer.Load().StellarV2.Theme, "appearance.colorScheme"); value != "light" {
		t.Fatal("local setting update lost theme")
	}
	before := pointer.Load()
	broken := map[string]any{"version": 2, "theme": map[string]any{"style": map[string]any{}}}
	raw, err := json.Marshal(broken)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.PutSetting(SectionStellarV2, string(raw)); err != nil {
		t.Fatal(err)
	}
	if err := RebuildSite(st, pointer); err == nil {
		t.Fatal("legacy field accepted in versioned theme")
	}
	if pointer.Load() != before {
		t.Fatal("invalid theme replaced live snapshot")
	}
}
