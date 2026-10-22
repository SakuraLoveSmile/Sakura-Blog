package config

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"blog/internal/settings"
)

func TestStellarBlogRuntimePublishAndRebuildConsumers(t *testing.T) {
	st := newTestStore(t)
	initial, err := InitializeStellarV2(st, map[string]any{}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	// 历史记录必须保留，但不再是消费者真值。
	if err := st.PutSetting(SectionComments, `{"enabled":true,"notify":{"password":"obsolete"}}`); err != nil {
		t.Fatal(err)
	}
	if err := st.PutSetting(SectionFriends, `{"posts_per_link":99}`); err != nil {
		t.Fatal(err)
	}
	pointer := &atomic.Pointer[settings.Site]{}
	before := &settings.Site{StellarV2: initial, Comments: settings.CommentsConfig{Enabled: true}}
	pointer.Store(before)
	blog := map[string]any{
		"native_comments": map[string]any{"enabled": false, "moderation": true, "admin_email": "admin@example.test", "notify": map[string]any{"smtp_host": "smtp.example.test", "smtp_port": 2525, "password": "canonical-mail"}},
		"friends":         map[string]any{"posts_per_link": 3, "check_interval": 0, "snapshot_interval": 0},
		"data":            map[string]any{"authors": map[string]any{"public": map[string]any{"name": "Visible", "hidden": false, "custom": map[string]any{"count": json.Number("9007199254740993")}}}},
		"ai":              map[string]any{"api_key": "canonical-ai", "summary": map[string]any{"enabled": false, "allow_user_api": false}, "ask": map[string]any{"enabled": true, "allow_user_llm": false}},
	}
	saved, err := SaveAndPublishStellarV2(st, pointer, map[string]any{}, blog)
	if err != nil {
		t.Fatal(err)
	}
	current := pointer.Load()
	if current.Comments.Enabled || !current.Comments.Moderation || current.Comments.Notify.Password != "canonical-mail" || current.Comments.Notify.SMTPPort != 2525 || current.Friends.MaxPostsPerFeed != 3 || current.Friends.CheckInterval != 0 || current.Authors["public"].Name != "Visible" {
		t.Fatal("actual consumers did not receive canonical runtime")
	}
	if !before.Comments.Enabled || before.StellarV2 != initial {
		t.Fatal("old snapshot mutated")
	}
	cfg, err := LoadAI(st)
	if err != nil || cfg.APIKey != "canonical-ai" || cfg.SummaryOn() || cfg.UserAPIOn() || !cfg.QAOn() || cfg.QAUserLLMOn() {
		t.Fatal("AI policy not canonical")
	}
	if err := RebuildSite(st, pointer); err != nil {
		t.Fatal(err)
	}
	current = pointer.Load()
	if current.Comments.Notify.Password != "canonical-mail" || current.Friends.MaxPostsPerFeed != 3 || !reflect.DeepEqual(current.StellarV2, saved) {
		t.Fatal("rebuild restored obsolete configuration")
	}
	if _, err := SaveAndPublishStellarV2(st, pointer, map[string]any{}, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if pointer.Load().Comments.Enabled || len(pointer.Load().Authors) != 0 || pointer.Load().Comments.Notify.Password != "" {
		t.Fatal("omission revived old local features")
	}
	cfg, err = LoadAI(st)
	if err != nil || cfg.SummaryOn() || cfg.UserAPIOn() || cfg.QAOn() || cfg.QAUserLLMOn() {
		t.Fatal("omitted AI flags enabled features")
	}
}

func TestStellarBlogRuntimeInvalidCandidateIsAtomicAndRedacted(t *testing.T) {
	st := newTestStore(t)
	initial, err := InitializeStellarV2(st, map[string]any{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	pointer := &atomic.Pointer[settings.Site]{}
	pointer.Store(&settings.Site{StellarV2: initial})
	for _, raw := range []string{
		`{"native_comments":{"enabled":"private-value"}}`,
		`{"native_comments":{"notify":{"smtp_port":65536,"password":"private-value"}}}`,
		`{"native_comments":{"notify":{"smtp_host":"private-value\r\n"}}}`,
		`{"ai":{"summary":{"enabled":"private-value"},"api_key":"private-value"}}`,
		`{"ai":{"api_key":{"private-value":true}}}`,
		`{"data":{"authors":{"admin":{"hidden":"private-value"}}}}`,
		`{"friends":{"posts_per_link":1.5}}`,
	} {
		var blog map[string]any
		if err := decodeStellarV2JSON([]byte(raw), &blog, false); err != nil {
			t.Fatal(err)
		}
		before, _ := st.ReadSettingsSnapshot()
		old := pointer.Load()
		_, err := SaveAndPublishStellarV2(st, pointer, map[string]any{}, blog)
		if err == nil || strings.Contains(err.Error(), "private-value") {
			t.Fatalf("invalid or leaking diagnostic: %v", err)
		}
		after, _ := st.ReadSettingsSnapshot()
		if !reflect.DeepEqual(before, after) || pointer.Load() != old {
			t.Fatal("rejected candidate changed live state")
		}
	}
}

func TestStellarBlogAIStoreIsolationAndStrictGate(t *testing.T) {
	a, b := newTestStore(t), newTestStore(t)
	if _, err := LoadAI(a); err == nil {
		t.Fatal("unmigrated AI config accepted")
	} else {
		var migration *MigrationRequiredError
		if !errors.As(err, &migration) {
			t.Fatal(err)
		}
	}
	for _, item := range []struct {
		key    string
		target int
	}{{"A", 0}, {"B", 1}} {
		st := a
		if item.target == 1 {
			st = b
		}
		if _, err := InitializeStellarV2(st, map[string]any{}, map[string]any{"ai": map[string]any{"api_key": item.key}}); err != nil {
			t.Fatal(err)
		}
	}
	var workers sync.WaitGroup
	for i := 0; i < 6; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			for n := 0; n < 20; n++ {
				st, want := a, "A"
				if i%2 == 1 {
					st, want = b, "B"
				}
				got, err := LoadAI(st)
				if err != nil || got.APIKey != want {
					t.Error("cross-store AI configuration")
				}
			}
		}(i)
	}
	workers.Wait()
	if _, err := SaveStellarV2(a, map[string]any{}, map[string]any{"ai": map[string]any{"api_key": "updated"}}); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadAI(a)
	if err != nil || cfg.APIKey != "updated" {
		t.Fatal("read did not observe committed config")
	}
	if err := a.PutSetting(SectionAI, `{"api_key":"obsolete"}`); err != nil {
		t.Fatal(err)
	}
	if err := a.PutSetting(SectionStellarV2, `{"version":2,"theme":{"style":{}}}`); err != nil {
		t.Fatal(err)
	}
	if cfg, err := LoadAI(a); err == nil || cfg.APIKey != "" {
		t.Fatal("invalid canonical config fell back to obsolete secret")
	}
}

func TestStellarBlogNullableFlagsAndSummaryPermission(t *testing.T) {
	for _, raw := range []string{
		`{"native_comments":{"enabled":null},"ai":{"summary":{"enabled":null,"allow_user_api":true},"ask":{"enabled":null,"allow_user_llm":null}}}`,
		`{"ai":{"summary":{"allow_user_api":true}}}`,
		`{"ai":{"summary":{"enabled":false,"allow_user_api":true}}}`,
	} {
		var blog map[string]any
		if err := decodeStellarV2JSON([]byte(raw), &blog, false); err != nil {
			t.Fatal(err)
		}
		runtime, err := ParseStellarBlogRuntime(blog)
		if err != nil || runtime.Comments.Enabled || runtime.AI.SummaryOn() || runtime.AI.UserAPIOn() || runtime.AI.QAOn() {
			t.Fatal("omitted, null or disabled feature opened a public service")
		}
	}
	runtime, err := ParseStellarBlogRuntime(map[string]any{"ai": map[string]any{"summary": map[string]any{"enabled": true, "allow_user_api": true}}})
	if err != nil || !runtime.AI.SummaryOn() || !runtime.AI.UserAPIOn() {
		t.Fatal("explicitly enabled summary permissions were lost")
	}
}
