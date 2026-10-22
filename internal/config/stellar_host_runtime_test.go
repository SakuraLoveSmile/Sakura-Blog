package config

import (
	"strings"
	"sync/atomic"
	"testing"

	"blog/internal/settings"
)

func TestStellarHostPagesSEOCanonicalPublishReopenAndClear(t *testing.T) {
	st := newTestStore(t)
	initial, err := InitializeStellarV2(st, map[string]any{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.PutSetting(SectionSEO, `{"robots_extra":"OLD","google_verification":"OLD"}`); err != nil {
		t.Fatal(err)
	}
	if err := st.PutSetting(SectionPages, `{"friends":{"title":"OLD","posts_per_link":99}}`); err != nil {
		t.Fatal(err)
	}
	ptr := &atomic.Pointer[settings.Site]{}
	ptr.Store(&settings.Site{StellarV2: initial})
	blog := map[string]any{
		"friends": map[string]any{"posts_per_link": 5, "refresh_interval": 12},
		"pages":   map[string]any{"friends": map[string]any{"title": "New friends", "show_posts": false, "posts_per_link": 2, "refresh_interval": 3, "hero": map[string]any{"title": "Hero", "description": "Tagline", "background": "/cover.png"}}},
		"seo":     map[string]any{"default_description": "Default description", "default_keywords": "alpha,beta", "robots_extra": "Disallow: /private", "no_index": true, "google_verification": "google", "bing_verification": "bing", "baidu_verification": "baidu"},
	}
	if _, err := SaveAndPublishStellarV2(st, ptr, map[string]any{}, blog); err != nil {
		t.Fatal(err)
	}
	assert := func() {
		t.Helper()
		site := ptr.Load()
		if site.Pages.Friends.Title != "New friends" || site.Pages.Friends.ShowPostsOn() || site.Pages.Friends.Hero.Background != "/cover.png" || site.FriendsPostsPerLink() != 2 || site.FriendsRefreshInterval() != 3 || site.SEO.DefaultDescription != "Default description" || site.SEO.DefaultKeywords != "alpha,beta" || !site.SEO.NoIndex || site.SEO.RobotsExtra != "Disallow: /private" || site.SEO.GoogleVerification != "google" || site.SEO.BingVerification != "bing" || site.SEO.BaiduVerification != "baidu" {
			t.Fatalf("canonical consumers not published: %+v %+v", site.Pages, site.SEO)
		}
	}
	assert()
	if err := RebuildSite(st, ptr); err != nil {
		t.Fatal(err)
	}
	assert()
	blog["pages"] = map[string]any{"friends": map[string]any{"posts_per_link": 0, "refresh_interval": 0}}
	if _, err := SaveAndPublishStellarV2(st, ptr, map[string]any{}, blog); err != nil {
		t.Fatal(err)
	}
	if site := ptr.Load(); site.FriendsPostsPerLink() != 5 || site.FriendsRefreshInterval() != 12 {
		t.Fatal("zero thresholds must inherit canonical Blog.friends")
	}
	if _, err := SaveAndPublishStellarV2(st, ptr, map[string]any{}, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if site := ptr.Load(); site.Pages.Friends.Title != "" || site.SEO.GoogleVerification != "" || site.SEO.NoIndex || site.SEO.RobotsExtra != "" {
		t.Fatal("omission revived historical host fields")
	}
}

func TestStellarHostTypedErrorsDoNotEchoValues(t *testing.T) {
	for _, blog := range []map[string]any{
		{"pages": map[string]any{"friends": map[string]any{"posts_per_link": 1.5}}},
		{"pages": map[string]any{"friends": map[string]any{"show_posts": "private-marker"}}},
		{"seo": map[string]any{"no_index": "private-marker"}},
		{"seo": map[string]any{"robots_extra": []any{"private-marker"}}},
	} {
		_, err := ValidateStellarV2(map[string]any{}, blog)
		if err == nil || strings.Contains(err.Error(), "private-marker") {
			t.Fatalf("invalid host config was accepted or echoed: %v", err)
		}
	}
}

func TestStellarFriendsNullablePointersPublishAndReopen(t *testing.T) {
	st := newTestStore(t)
	initial, err := InitializeStellarV2(st, map[string]any{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ptr := &atomic.Pointer[settings.Site]{}
	ptr.Store(&settings.Site{StellarV2: initial})
	// false → null 必须清掉旧的关闭值，保存原文 null，并恢复 nil 的默认语义。
	for _, value := range []any{false, nil, true, false} {
		blog := map[string]any{"pages": map[string]any{"friends": map[string]any{"show_posts": value}}}
		saved, err := SaveAndPublishStellarV2(st, ptr, map[string]any{}, blog)
		if err != nil {
			t.Fatalf("show_posts=%v: %v", value, err)
		}
		friends := saved.Blog["pages"].(map[string]any)["friends"].(map[string]any)
		if raw, exists := friends["show_posts"]; !exists || raw != value {
			t.Fatal("nullable raw value was removed or rewritten")
		}
		if err := RebuildSite(st, ptr); err != nil {
			t.Fatal(err)
		}
		current := ptr.Load().Pages.Friends
		if current.ShowPostsOn() != (value != false) || (current.ShowPosts == nil) != (value == nil) || current.Hero != nil {
			t.Fatalf("nullable pointer consumer changed: %+v", current)
		}
	}
	if _, err := ParseStellarBlogRuntime(map[string]any{"pages": map[string]any{"friends": map[string]any{"hero": nil}}}); err != nil {
		t.Fatalf("optional hero null: %v", err)
	}
}
