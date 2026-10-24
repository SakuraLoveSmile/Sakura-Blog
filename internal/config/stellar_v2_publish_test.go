package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"blog/internal/settings"
)

func TestStellarV2MaskAndPublishSecrets(t *testing.T) {
	st := newTestStore(t)
	blog := map[string]any{
		"native_comments": map[string]any{"notify": map[string]any{"password": "mail-secret", "enabled": false}},
		"comments":        map[string]any{"notify": map[string]any{"password": "legacy-mail"}},
		"notify":          map[string]any{"password": "notify-secret"},
		"ai":              map[string]any{"api_key": "ai-secret", "model": "initial"},
		"custom":          map[string]any{"password": "public-value", "api_key": "public-key"},
	}
	initial, err := InitializeStellarV2(st, map[string]any{}, blog)
	if err != nil {
		t.Fatal(err)
	}
	masked, err := MaskStellarV2Blog(initial.Blog)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(masked)
	for _, secret := range []string{"mail-secret", "legacy-mail", "notify-secret", "ai-secret"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatal("known secret leaked")
		}
	}
	if custom := masked["custom"].(map[string]any); custom["password"] != "public-value" || custom["api_key"] != "public-key" {
		t.Fatal("public parameter bag was masked")
	}
	masked["ai"].(map[string]any)["model"] = "updated"
	before := &settings.Site{Title: "retained", StellarV2: initial}
	pointer := &atomic.Pointer[settings.Site]{}
	pointer.Store(before)
	saved, err := SaveAndPublishStellarV2(st, pointer, map[string]any{"appearance": map[string]any{"color_scheme": "light"}}, masked)
	if err != nil {
		t.Fatal(err)
	}
	if pointer.Load() == before || pointer.Load().StellarV2 != saved || pointer.Load().Title != "retained" {
		t.Fatal("site copy/publish contract failed")
	}
	if initial.Blog["ai"].(map[string]any)["model"] != "initial" || before.StellarV2 != initial {
		t.Fatal("old published snapshot changed")
	}
	for _, path := range stellarV2SecretPaths {
		got, _ := stellarV2SecretParent(saved.Blog, path)
		want, _ := stellarV2SecretParent(initial.Blog, path)
		if !reflect.DeepEqual(got[path[len(path)-1]], want[path[len(path)-1]]) {
			t.Fatal("masked save discarded stored secret")
		}
	}
	masked["ai"].(map[string]any)["api_key"] = nil
	masked["native_comments"].(map[string]any)["notify"].(map[string]any)["password"] = ""
	cleared, err := SaveAndPublishStellarV2(st, pointer, saved.RawTheme, masked)
	if err != nil || cleared.Blog["ai"].(map[string]any)["api_key"] != nil || cleared.Blog["native_comments"].(map[string]any)["notify"].(map[string]any)["password"] != "" {
		t.Fatalf("explicit secret clearing failed: %v", err)
	}
}

func TestStellarV2PublishFailureDoesNotSaveOrReplace(t *testing.T) {
	st := newTestStore(t)
	initial, err := InitializeStellarV2(st, map[string]any{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := st.ReadSettingsSnapshot()
	pointer := &atomic.Pointer[settings.Site]{}
	if _, err := SaveAndPublishStellarV2(st, pointer, map[string]any{}, nil); err == nil {
		t.Fatal("nil site accepted")
	}
	current := &settings.Site{StellarV2: initial}
	pointer.Store(current)
	if _, err := SaveAndPublishStellarV2(st, pointer, map[string]any{"style": map[string]any{}}, nil); err == nil {
		t.Fatal("invalid schema accepted")
	}
	if _, err := SaveAndPublishStellarV2(st, pointer, map[string]any{}, map[string]any{"ai": map[string]any{"api_key": StellarV2SecretMask}}); err == nil {
		t.Fatal("masked key without stored secret accepted")
	}
	after, _ := st.ReadSettingsSnapshot()
	if !reflect.DeepEqual(before, after) || pointer.Load() != current {
		t.Fatal("failure changed database or published site")
	}
}

func TestStellarV2ConcurrentPublishAndLegacySave(t *testing.T) {
	st := newTestStore(t)
	initial, err := InitializeStellarV2(st, map[string]any{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	pointer := &atomic.Pointer[settings.Site]{}
	pointer.Store(&settings.Site{Title: "retained", StellarV2: initial})
	var workers sync.WaitGroup
	for id := 0; id < 5; id++ {
		workers.Add(1)
		go func(id int) {
			defer workers.Done()
			for n := 0; n < 8; n++ {
				if id == 0 {
					if err := SaveSection(st, pointer, "feedback", []byte(`{"enabled":false,"app_name":"legacy"}`)); err != nil {
						t.Error(err)
					}
					continue
				}
				generation := fmt.Sprintf("%d/%d", id, n)
				if _, err := SaveAndPublishStellarV2(st, pointer, map[string]any{"comments": map[string]any{"giscus": map[string]any{"generation": generation}}}, map[string]any{"generation": generation}); err != nil {
					t.Error(err)
				}
			}
		}(id)
	}
	workers.Wait()
	loaded, err := LoadStellarV2(st)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, pointer.Load().StellarV2) || pointer.Load().Feedback.AppName != "legacy" {
		t.Fatal("published site differs from latest committed snapshot")
	}
}
