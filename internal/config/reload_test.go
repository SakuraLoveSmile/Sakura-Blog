package config

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	tmpl "blog/internal/settings"
	"blog/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := s.EnsureSettingsTable(); err != nil {
		t.Fatalf("ensure settings: %v", err)
	}
	if err := s.EnsureLinksTable(); err != nil {
		t.Fatalf("ensure links: %v", err)
	}
	if err := s.EnsureUsersTable(); err != nil {
		t.Fatalf("ensure users: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// newSitePtr 构造原子站点指针测试助手（Spec 61 阶段 3：site 热更新改复制-交换发布）。
func newSitePtr() *atomic.Pointer[tmpl.Site] {
	p := &atomic.Pointer[tmpl.Site]{}
	p.Store(&tmpl.Site{})
	return p
}

func TestBootstrapMigratesAdminSectionToUsersTable(t *testing.T) {
	s := newTestStore(t)
	if err := s.PutSetting(SectionAdmin, `{"password":"secret"}`); err != nil {
		t.Fatalf("seed admin section: %v", err)
	}
	site := newSitePtr()

	if err := Bootstrap(s, site.Load()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	// users 表 admin 存在且 bcrypt 匹配；admin 段已清空
	admin, err := s.GetUserByUsername("admin")
	if err != nil || admin == nil {
		t.Fatalf("admin user should be created: %v", err)
	}
	if err := store.CheckPassword(admin.PasswordHash, "secret"); err != nil {
		t.Fatalf("admin hash should match section password: %v", err)
	}
	v, _ := s.GetSetting(SectionAdmin)
	if v != "" {
		t.Fatalf("admin section should be cleared after migration, got %q", v)
	}

	// 幂等：二次 Bootstrap 不重复创建/覆盖
	if err := Bootstrap(s, site.Load()); err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	admin2, _ := s.GetUserByUsername("admin")
	if admin2 == nil || admin2.ID != admin.ID {
		t.Fatal("second bootstrap should not recreate admin user")
	}
	if err := store.CheckPassword(admin2.PasswordHash, "secret"); err != nil {
		t.Fatalf("admin hash should be unchanged: %v", err)
	}
}

func TestBootstrapMigratesSiteYmlPasswordWhenUsersEmpty(t *testing.T) {
	s := newTestStore(t)
	site := newSitePtr()
	site.Load().Admin.Password = "boot-pass"

	if err := Bootstrap(s, site.Load()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	admin, err := s.GetUserByUsername("admin")
	if err != nil || admin == nil {
		t.Fatalf("admin user should be created from site.yml password: %v", err)
	}
	if err := store.CheckPassword(admin.PasswordHash, "boot-pass"); err != nil {
		t.Fatalf("admin hash should match yml password: %v", err)
	}
	v, _ := s.GetSetting(SectionAdmin)
	if v != "" {
		t.Fatalf("admin section should be cleared after migration, got %q", v)
	}
	// 幂等：二次 Bootstrap 不重导 site.yml 明文（admin 段保持空）
	if err := Bootstrap(s, site.Load()); err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	v, _ = s.GetSetting(SectionAdmin)
	if v != "" {
		t.Fatal("second bootstrap should not re-import site.yml password into admin section")
	}
	admin2, _ := s.GetUserByUsername("admin")
	if err := store.CheckPassword(admin2.PasswordHash, "boot-pass"); err != nil {
		t.Fatalf("admin hash should be stable: %v", err)
	}
}

func TestBootstrapSkipsMigrationWhenAdminSectionEmpty(t *testing.T) {
	s := newTestStore(t)
	site := newSitePtr() // 无密码配置（site.yml 无 admin.password）
	site.Load().Admin.Password = ""

	if err := Bootstrap(s, site.Load()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	admin, _ := s.GetUserByUsername("admin")
	if admin != nil && admin.PasswordHash != "" {
		t.Fatal("no-password site should not get an admin user with hash")
	}
	v, _ := s.GetSetting(SectionAdmin)
	if v != "" {
		t.Fatalf("admin section should stay empty, got %q", v)
	}
}

func TestRebuildSiteDerivesAuthEnabledFromUsersTable(t *testing.T) {
	s := newTestStore(t)
	if err := s.EnsureUsersTable(); err != nil {
		t.Fatalf("users table: %v", err)
	}
	site := newSitePtr()

	// 无 admin 用户 → AuthEnabled=false
	if err := RebuildSite(s, site); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if site.Load().Admin.AuthEnabled {
		t.Fatal("no admin user should mean AuthEnabled=false")
	}

	// 建 admin 用户（bcrypt）→ AuthEnabled=true
	hash, _ := store.HashPassword("secret")
	if err := s.CreateUser(&store.User{
		Username: "admin", PasswordHash: hash, DisplayName: "管理员",
		AuthSecret: store.GenerateAuthSecret(), Role: store.RoleAdmin,
	}); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if err := RebuildSite(s, site); err != nil {
		t.Fatalf("rebuild 2: %v", err)
	}
	if !site.Load().Admin.AuthEnabled {
		t.Fatal("admin user with hash should mean AuthEnabled=true")
	}

	// 清空哈希 → AuthEnabled=false（无密码站点语义）
	admin, _ := s.GetUserByUsername("admin")
	if err := s.UpdatePassword(admin.ID, "", admin.AuthSecret); err != nil {
		t.Fatalf("clear hash: %v", err)
	}
	if err := RebuildSite(s, site); err != nil {
		t.Fatalf("rebuild 3: %v", err)
	}
	if site.Load().Admin.AuthEnabled {
		t.Fatal("empty hash should mean AuthEnabled=false")
	}
}

// TestBootstrapImportsEditableSections 新段（cover/nav_tabs/authors/plugins）首次导入 + 幂等。
func TestBootstrapImportsEditableSections(t *testing.T) {
	s := newTestStore(t)
	site := newSitePtr()
	site.Load().Cover = tmpl.CoverConfig{Title: "Welcome", Features: []tmpl.CoverFeature{{Title: "开始阅读", URL: "/archives/"}}}
	site.Load().NavTabs = tmpl.NavTabsConfig{EnableCustom: true, Items: []tmpl.NavTabItem{{Title: "友链", URL: "/friends/"}}}
	site.Load().Authors = map[string]tmpl.AuthorDef{"sakura": {Name: "Sakura", Description: "作者"}}
	site.Load().Plugins = tmpl.PluginsConfig{Heti: &tmpl.HetiConfig{Enable: true}}
	site.Load().Feedback = tmpl.FeedbackConfig{Enabled: true, APIBase: "http://127.0.0.1:8787", AppID: "com.test.blog"}
	en := false
	defaultTTL := 600
	site.Load().DataCache = tmpl.DataCacheConfig{Enable: &en, DefaultTTL: &defaultTTL}

	if err := Bootstrap(s, site.Load()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	for _, sec := range []string{SectionCover, SectionNavTabs, SectionAuthors, SectionPlugins, SectionDataCache, SectionFeedback} {
		v, err := s.GetSetting(sec)
		if err != nil || v == "" {
			t.Fatalf("section %s not imported: %q, %v", sec, v, err)
		}
	}
	// 二次 Bootstrap 幂等：DB 值不被 site.yml 初始值覆盖。
	if err := SaveSection(s, site, SectionCover, []byte(`{"title":"DB 版"}`)); err != nil {
		t.Fatalf("save cover: %v", err)
	}
	if err := Bootstrap(s, site.Load()); err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	if site.Load().Cover.Title != "DB 版" {
		t.Fatalf("second bootstrap must keep DB cover, got %q", site.Load().Cover.Title)
	}
}

// TestSaveSectionNewSectionsRoundtrip 新段 SaveSection → RebuildSite 热更新生效。
func TestSaveSectionNewSectionsRoundtrip(t *testing.T) {
	s := newTestStore(t)
	site := newSitePtr()
	if err := Bootstrap(s, site.Load()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	cases := []struct {
		section string
		payload string
		check   func() bool
	}{
		{SectionCover, `{"title":"Hello","description":"desc","features":[{"title":"Go","url":"/archives/"}]}`, func() bool { return site.Load().Cover.Title == "Hello" && len(site.Load().Cover.Features) == 1 }},
		{SectionNavTabs, `{"enable_custom":true,"items":[{"title":"GitHub","url":"https://github.com","external":true}]}`, func() bool {
			return site.Load().NavTabs.EnableCustom && len(site.Load().NavTabs.Items) == 1 && site.Load().NavTabs.Items[0].External
		}},
		{SectionAuthors, `{"sakura":{"name":"Sakura","description":"作者"}}`, func() bool { return site.Load().Authors["sakura"].Name == "Sakura" }},
		{SectionPlugins, `{"heti":{"enable":true},"fancybox":{"selector":"div.lazy-box>img.lazy"}}`, func() bool {
			return site.Load().Plugins.Heti != nil && site.Load().Plugins.Heti.Enable && site.Load().Plugins.Fancybox != nil && site.Load().Plugins.Fancybox.Selector != ""
		}},
		{SectionSearch, `{"service":"local_search","local_search":{"field":"page","path":"/custom-search.json","content":false,"skip_search":["/wiki/private"]}}`, func() bool {
			cfg := site.Load().Search
			return cfg.LocalSearch.Field == "page" && cfg.LocalSearch.Path == "/custom-search.json" && cfg.LocalSearch.Content != nil && !*cfg.LocalSearch.Content && len(cfg.LocalSearch.SkipSearch) == 1
		}},
		{SectionSiteTree, `{"home":{"comments":true}}`, func() bool { return site.Load().SiteTree.Home.Comments }},
		{SectionDataCache, `{"enable":false,"default_ttl":120,"ttl":{"friends":86400}}`, func() bool {
			cfg := site.Load().DataCache
			return cfg.Enable != nil && !*cfg.Enable && cfg.DefaultTTL != nil && *cfg.DefaultTTL == 120 && cfg.TTL["friends"] == 86400
		}},
		{SectionSEO, `{"canonical":{"originalHost":"origin.example","officialHosts":["mirror.example"]},"open_graph":{"enable":false,"twitter_id":"blog"},"preconnect":["https://cdn.example"],"structured_data":{"links":["https://example.com"]}}`, func() bool {
			cfg := site.Load().SEO
			return cfg.Canonical.OriginalHost == "origin.example" && len(cfg.Canonical.OfficialHosts) == 1 && cfg.OpenGraph.Enable != nil && !*cfg.OpenGraph.Enable && len(cfg.Preconnect) == 1 && len(cfg.StructuredData.Links) == 1
		}},
		{SectionFeedback, `{"enabled":true,"api_base":"https://feedback.example.com","app_id":"com.example.blog","launcher_mode":"tab","capture_mode":"off","side":"left"}`, func() bool {
			cfg := site.Load().Feedback
			return cfg.Active() && cfg.EffectiveLauncherMode() == "tab" && cfg.EffectiveCaptureMode() == "off" && cfg.EffectiveSide() == "left"
		}},
	}
	for _, tc := range cases {
		if err := SaveSection(s, site, tc.section, []byte(tc.payload)); err != nil {
			t.Fatalf("save %s: %v", tc.section, err)
		}
		if !tc.check() {
			t.Fatalf("section %s hot update failed", tc.section)
		}
	}
}

// TestSaveSectionFeedbackValidation feedback 段 api_base 必须为 http(s)，缺省值按组件语义回落。
func TestSaveSectionFeedbackValidation(t *testing.T) {
	s := newTestStore(t)
	site := newSitePtr()
	if err := Bootstrap(s, site.Load()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if err := SaveSection(s, site, SectionFeedback, []byte(`{"enabled":true,"api_base":"ftp://x","app_id":"a"}`)); err == nil {
		t.Fatal("non-http(s) api_base should be rejected")
	}
	if site.Load().Feedback.Enabled {
		t.Fatal("rejected payload must not mutate site")
	}
	// 未填 launcher/side/capture 字段时按组件缺省回落（orb / viewport / right / 25%）。
	if err := SaveSection(s, site, SectionFeedback, []byte(`{"enabled":true,"api_base":"http://127.0.0.1:8787","app_id":"com.example.blog"}`)); err != nil {
		t.Fatalf("save feedback: %v", err)
	}
	cfg := site.Load().Feedback
	if !cfg.Active() || cfg.EffectiveLauncherMode() != "orb" || cfg.EffectiveCaptureMode() != "viewport" || cfg.EffectiveSide() != "right" || cfg.EffectiveLauncherBottom() != "25%" {
		t.Fatalf("feedback defaults mismatch: %+v", cfg)
	}
}

func TestApplyDraftSectionsOnlyOverwritesProvidedSections(t *testing.T) {
	site := &tmpl.Site{
		Title:    "线上标题",
		Comments: tmpl.CommentsConfig{Enabled: true, Title: "评论"},
		Authors:  map[string]tmpl.AuthorDef{"owner": {Name: "站长", Description: "简介"}},
		Style:    tmpl.StyleConfig{PrefersTheme: "light"},
		Widgets: tmpl.WidgetsConfig{Library: map[string]tmpl.WidgetDef{
			"recent": {"layout": "recent", "limit": 5},
		}},
	}

	ApplyDraftSections(site, map[string]json.RawMessage{
		SectionWidgets: json.RawMessage(`{"library":{"tagcloud":{"layout":"tagcloud","amount":10}}}`),
	})

	if site.Title != "线上标题" {
		t.Errorf("title was cleared by widgets-only draft: %q", site.Title)
	}
	if !site.Comments.Enabled || site.Comments.Title != "评论" {
		t.Errorf("comments changed by widgets-only draft: %+v", site.Comments)
	}
	if got := site.Authors["owner"]; got.Name != "站长" || got.Description != "简介" {
		t.Errorf("authors changed by widgets-only draft: %+v", site.Authors)
	}
	if site.Style.PrefersTheme != "light" {
		t.Errorf("style changed by widgets-only draft: %+v", site.Style)
	}
	if site.Widgets.Library["tagcloud"].Layout() != "tagcloud" || site.Widgets.Library["recent"] != nil {
		t.Errorf("widgets section was not replaced: %+v", site.Widgets.Library)
	}

	ApplyDraftSections(site, map[string]json.RawMessage{
		SectionSite:     json.RawMessage(`{"title":"草稿标题"}`),
		SectionComments: json.RawMessage(`{"enabled":false}`),
		SectionAuthors:  json.RawMessage(`{"editor":{"name":"编辑"}}`),
		SectionStyle:    json.RawMessage(`{"prefers_theme":"dark"}`),
	})
	if site.Title != "草稿标题" || site.Comments.Enabled || site.Authors["editor"].Name != "编辑" || site.Style.PrefersTheme != "dark" {
		t.Errorf("provided sections were not applied: %+v", site)
	}
}

// TestBootstrapMergesFavicon 旧库 site 段缺 favicon 键时从 site.yml 初值合并补写。
func TestBootstrapMergesFavicon(t *testing.T) {
	s := newTestStore(t)
	// 预置旧格式 site 段（无 favicon_url/apple_touch_icon 键）
	if err := s.PutSetting(SectionSite, `{"title":"旧库","subtitle":"","description":"","url":"","avatar":"","avatar_bg":"","logo_title":"","logo_sub":""}`); err != nil {
		t.Fatalf("seed site: %v", err)
	}
	site := &atomic.Pointer[tmpl.Site]{}
	site.Store(&tmpl.Site{Title: "旧库", FaviconURL: "https://example.com/favicon.svg", AppleIcon: "https://example.com/apple.png"})
	if err := Bootstrap(s, site.Load()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	v, _ := s.GetSetting(SectionSite)
	var sec map[string]json.RawMessage
	if err := json.Unmarshal([]byte(v), &sec); err != nil {
		t.Fatalf("parse site: %v", err)
	}
	var favicon string
	if err := json.Unmarshal(sec["favicon_url"], &favicon); err != nil || favicon != "https://example.com/favicon.svg" {
		t.Fatalf("favicon_url not merged: %q, %v", favicon, err)
	}
	// RebuildSite 后 site 字段保持 yml 初值不被清零
	if err := RebuildSite(s, site); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if site.Load().FaviconURL != "https://example.com/favicon.svg" {
		t.Fatalf("rebuild cleared favicon: %q", site.Load().FaviconURL)
	}
}

func TestSaveSectionSiteURLValidationAndNormalization(t *testing.T) {
	s := newTestStore(t)
	if err := s.EnsureUsersTable(); err != nil {
		t.Fatalf("ensure users table: %v", err)
	}
	site := newSitePtr()

	// 1. Invalid URL rejected
	badPayload := []byte(`{"title":"Site","url":"invalid-url"}`)
	if err := SaveSection(s, site, SectionSite, badPayload); err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}

	// 2. Trailing slash normalized
	goodPayload := []byte(`{"title":"Site","url":"https://example.com/blog/"}`)
	if err := SaveSection(s, site, SectionSite, goodPayload); err != nil {
		t.Fatalf("unexpected error saving valid URL: %v", err)
	}
	if got, want := site.Load().URL, "https://example.com/blog"; got != want {
		t.Fatalf("site.URL = %q, want %q", got, want)
	}
}

func TestSaveSectionPreservesMaskedSecretsAndDoesNotWriteOnSnapshotFailure(t *testing.T) {
	s := newTestStore(t)
	if err := s.PutSetting(SectionAI, `{"api_key":"stored-key","base_url":"https://example.test"}`); err != nil {
		t.Fatalf("seed ai setting: %v", err)
	}
	if err := SaveSection(s, newSitePtr(), SectionAI, []byte(`{"api_key":"****","base_url":"https://changed.test"}`)); err != nil {
		t.Fatalf("save masked ai setting: %v", err)
	}
	stored, err := s.GetSetting(SectionAI)
	if err != nil || !strings.Contains(stored, `"api_key":"stored-key"`) {
		t.Fatalf("masked key should be preserved, stored=%q err=%v", stored, err)
	}

	if err := s.PutSetting(SectionSite, `{"title":"before"}`); err != nil {
		t.Fatalf("seed site setting: %v", err)
	}
	if err := SaveSection(s, &atomic.Pointer[tmpl.Site]{}, SectionSite, []byte(`{"title":"after"}`)); err == nil {
		t.Fatal("nil snapshot should reject before writing")
	}
	stored, err = s.GetSetting(SectionSite)
	if err != nil || !strings.Contains(stored, `"title":"before"`) {
		t.Fatalf("failed candidate must not write setting, stored=%q err=%v", stored, err)
	}
}

func TestSaveSectionRejectsMalformedUnrelatedStoredSection(t *testing.T) {
	s := newTestStore(t)
	if err := s.PutSetting(SectionStyle, `{malformed`); err != nil {
		t.Fatalf("seed malformed setting: %v", err)
	}
	if err := s.PutSetting(SectionSite, `{"title":"before"}`); err != nil {
		t.Fatalf("seed site setting: %v", err)
	}
	if err := SaveSection(s, newSitePtr(), SectionSite, []byte(`{"title":"after"}`)); err == nil {
		t.Fatal("malformed unrelated section should reject the candidate")
	}
	stored, err := s.GetSetting(SectionSite)
	if err != nil || !strings.Contains(stored, `"title":"before"`) {
		t.Fatalf("rejected candidate must not write site setting, stored=%q err=%v", stored, err)
	}
}

func TestSaveSectionConcurrentDifferentSectionsPublishesDatabaseState(t *testing.T) {
	s := newTestStore(t)
	site := newSitePtr()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, item := range []struct {
		section string
		raw     string
	}{
		{SectionSite, `{"title":"concurrent-title"}`},
		{SectionComments, `{"enabled":false}`},
	} {
		item := item
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- SaveSection(s, site, item.section, []byte(item.raw))
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent save: %v", err)
		}
	}
	settings, err := s.AllSettings()
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	current := site.Load()
	if current == nil || current.Title != "concurrent-title" || current.Comments.Enabled {
		t.Fatalf("published site does not match DB writes: %+v settings=%v", current, settings)
	}
}
