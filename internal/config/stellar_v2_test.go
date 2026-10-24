package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"blog/internal/stellarv2"
	"blog/internal/store"
)

func TestStellarV2LegacyGateDoesNotWrite(t *testing.T) {
	st := newTestStore(t)
	if err := st.PutSetting("style", `{"color_scheme":"dark"}`); err != nil {
		t.Fatal(err)
	}
	before, _ := st.AllSettings()
	for _, attempt := range []func() error{
		func() error { _, err := LoadStellarV2(st); return err },
		func() error { _, err := SaveStellarV2(st, map[string]any{}, nil); return err },
		func() error { _, err := InitializeStellarV2(st, map[string]any{}, nil); return err },
	} {
		var migration *MigrationRequiredError
		if err := attempt(); !errors.As(err, &migration) || migration.Source != "settings" || !reflect.DeepEqual(migration.Sections, []string{"style"}) {
			t.Fatalf("expected localized migration diagnostic, got %v", err)
		}
		after, _ := st.AllSettings()
		if !reflect.DeepEqual(before, after) {
			t.Fatal("migration gate changed legacy data")
		}
	}
}

func TestStellarV2PersistRawAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v2.db")
	st, err := store.New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	if err := st.EnsureSettingsTable(); err != nil {
		t.Fatal(err)
	}
	bag := map[string]any{"flag": false, "zero": 0, "nothing": nil, "list": []any{}, "nested": map[string]any{"snake_case": "kept"}, "large": json.Number("9007199254740993")}
	theme := map[string]any{"appearance": map[string]any{"color_scheme": "auto"}, "comments": map[string]any{"giscus": bag}}
	blog := map[string]any{"native_comments": bag, "poster": nil, "share_link": false}
	wantTheme, _ := json.Marshal(theme)
	wantBlog, _ := json.Marshal(blog)
	config, err := InitializeStellarV2(st, theme, blog)
	if err != nil {
		t.Fatal(err)
	}
	// 候选、公开输入和运行时参数袋各有独立所有权。
	bag["flag"] = true
	config.Theme["comments"].(map[string]any)["giscus"].(map[string]any)["flag"] = true
	config.Blog["share_link"] = true
	if err := st.PutSetting("unknown_business_section", `{"retained":true}`); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = store.New(path)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadStellarV2(st)
	if err != nil {
		t.Fatal(err)
	}
	gotTheme, _ := json.Marshal(loaded.RawTheme)
	gotBlog, _ := json.Marshal(loaded.Blog)
	if string(wantTheme) != string(gotTheme) || string(wantBlog) != string(gotBlog) {
		t.Fatal("raw parameters changed after save/reopen")
	}
	if got, _ := stellarv2.Get(loaded.Theme, "appearance.colorScheme"); got != "auto" {
		t.Fatalf("normalized theme not available: %v", got)
	}
	if _, exists := loaded.RawTheme["leftbar"]; exists {
		t.Fatal("runtime defaults were written back into raw input")
	}
	loaded.RawTheme["appearance"].(map[string]any)["color_scheme"] = "light"
	if _, err := SaveStellarV2(st, loaded.RawTheme, loaded.Blog); err != nil {
		t.Fatal(err)
	}
	retained, _ := st.GetSetting("unknown_business_section")
	if retained != `{"retained":true}` {
		t.Fatal("save removed unrelated settings")
	}
}

func TestStellarV2InvalidCandidateLeavesSnapshot(t *testing.T) {
	st := newTestStore(t)
	if _, err := InitializeStellarV2(st, map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	before, _ := st.AllSettings()
	_, err := SaveStellarV2(st, map[string]any{"style": map[string]any{"color_scheme": "dark"}}, map[string]any{"changed": true})
	var schema *stellarv2.SchemaError
	if !errors.As(err, &schema) {
		t.Fatalf("legacy field was not strictly rejected: %v", err)
	}
	if len(schema.Issues) == 0 || schema.Issues[0].Source != "settings.stellar_v2.theme" {
		t.Fatalf("schema diagnostic lost location: %v", schema)
	}
	for _, blog := range []map[string]any{{"secret": math.NaN()}, {"secret": make(chan int)}} {
		if _, err := SaveStellarV2(st, map[string]any{}, blog); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("invalid Blog JSON must fail without disclosing parameter names: %v", err)
		}
	}
	after, _ := st.AllSettings()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("invalid candidate wrote data")
	}
}

func TestStellarV2LoadRejectsVersionsAndMalformedDocuments(t *testing.T) {
	for _, tc := range []struct {
		name, document, blog string
		migration            bool
	}{
		{"empty", "", "{}", true},
		{"absent version", `{"theme":{}}`, "{}", true},
		{"old version", `{"version":1,"theme":{}}`, "{}", true},
		{"future version", `{"version":3,"theme":{}}`, "{}", true},
		{"string version", `{"version":"2","theme":{}}`, "{}", true},
		{"fractional version", `{"version":2.1,"theme":{}}`, "{}", true},
		{"object version", `{"version":{},"theme":{}}`, "{}", true},
		{"null document", "null", "{}", true},
		{"missing theme", `{"version":2}`, "{}", true},
		{"missing blog", `{"version":2,"theme":{}}`, "", true},
		{"malformed", `{`, "{}", false},
		{"extra envelope key", `{"version":2,"theme":{},"style":{}}`, "{}", false},
		{"multiple documents", `{"version":2,"theme":{}} {}`, "{}", false},
		{"invalid theme", `{"version":2,"theme":{"appearance":{"color_scheme":"sepia"}}}`, "{}", false},
		{"null blog", `{"version":2,"theme":{}}`, "null", false},
		{"array blog", `{"version":2,"theme":{}}`, "[]", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := map[string]string{SectionStellarV2: tc.document, SectionBlogExtensions: tc.blog}
			_, err := LoadStellarV2Snapshot(snapshot)
			var migration *MigrationRequiredError
			if err == nil || errors.As(err, &migration) != tc.migration {
				t.Fatalf("migration=%v, got %v", tc.migration, err)
			}
		})
	}
}

func TestStellarV2ConcurrentSaveReadCompletePairs(t *testing.T) {
	st := newTestStore(t)
	if _, err := InitializeStellarV2(st, map[string]any{"comments": map[string]any{"giscus": map[string]any{"generation": "initial"}}}, map[string]any{"generation": "initial"}); err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for id := 0; id < 8; id++ {
		workers.Add(1)
		go func(id int) {
			defer workers.Done()
			for n := 0; n < 8; n++ {
				generation := fmt.Sprintf("%d/%d", id, n)
				theme := map[string]any{"comments": map[string]any{"giscus": map[string]any{"generation": generation}}}
				if _, err := SaveStellarV2(st, theme, map[string]any{"generation": generation}); err != nil {
					t.Error(err)
					return
				}
				loaded, err := LoadStellarV2(st)
				if err != nil {
					t.Error(err)
					return
				}
				got, _ := stellarv2.Get(loaded.Theme, "comments.giscus.generation")
				if got != loaded.Blog["generation"] {
					t.Errorf("torn snapshot: theme=%v blog=%v", got, loaded.Blog["generation"])
				}
			}
		}(id)
	}
	workers.Wait()
}

func TestStellarV2ConcurrentInitializationOnlyOneWins(t *testing.T) {
	st := newTestStore(t)
	var workers sync.WaitGroup
	results := make(chan error, 8)
	for id := 0; id < cap(results); id++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, err := InitializeStellarV2(st, map[string]any{}, nil)
			results <- err
		}()
	}
	workers.Wait()
	close(results)
	success := 0
	for err := range results {
		var migration *MigrationRequiredError
		if err == nil {
			success++
		} else if !errors.As(err, &migration) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("initializations succeeded: %d, want 1", success)
	}
}
