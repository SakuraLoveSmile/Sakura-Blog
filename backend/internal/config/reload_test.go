package config

import (
	"sync/atomic"
	"encoding/json"
	"path/filepath"
	"testing"

	"blog/internal/store"
	tmpl "blog/internal/template"
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
	t.Cleanup(func() { s.Close() })
	return s
}

// newSitePtr 构造原子站点指针测试助手（Spec 61 阶段 3：site 热更新改复制-交换发布）。
func newSitePtr() *atomic.Pointer[tmpl.Site] {
	p := &atomic.Pointer[tmpl.Site]{}
	p.Store(&tmpl.Site{})
	return p
}

func TestSaveAdminPasswordOverridesAndPersists(t *testing.T) {
	s := newTestStore(t)
	site := newSitePtr()
	site.Load().Admin.Password = "yml-pass"

	if err := SaveAdminPassword(s, site, "db-pass"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if site.Load().Admin.Password != "db-pass" {
		t.Fatalf("hot update failed: password = %q", site.Load().Admin.Password)
	}
	v, err := s.GetSetting(SectionAdmin)
	if err != nil || v == "" {
		t.Fatalf("admin section not persisted: %q, %v", v, err)
	}
}

func TestSaveAdminPasswordEmptyDisablesGate(t *testing.T) {
	s := newTestStore(t)
	site := newSitePtr()
	site.Load().Admin.Password = "old"

	if err := SaveAdminPassword(s, site, ""); err != nil {
		t.Fatalf("save empty: %v", err)
	}
	if site.Load().Admin.Password != "" {
		t.Fatalf("empty password should disable gate, got %q", site.Load().Admin.Password)
	}
}

func TestRebuildSiteKeepsYmlPasswordWhenNoAdminSection(t *testing.T) {
	s := newTestStore(t)
	site := newSitePtr()
	site.Load().Admin.Password = "yml-pass"

	if err := RebuildSite(s, site); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if site.Load().Admin.Password != "yml-pass" {
		t.Fatalf("yml password should be kept when DB has no admin section, got %q", site.Load().Admin.Password)
	}
}

func TestRebuildSiteDbOverridesYml(t *testing.T) {
	s := newTestStore(t)
	if err := s.PutSetting(SectionAdmin, `{"password":"db-pass"}`); err != nil {
		t.Fatalf("seed admin section: %v", err)
	}
	site := newSitePtr()
	site.Load().Admin.Password = "yml-pass"

	if err := RebuildSite(s, site); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if site.Load().Admin.Password != "db-pass" {
		t.Fatalf("DB admin section should override yml, got %q", site.Load().Admin.Password)
	}
}

func TestBootstrapImportsAdminPassword(t *testing.T) {
	s := newTestStore(t)
	site := newSitePtr()
	site.Load().Admin.Password = "boot-pass"

	if err := Bootstrap(s, site.Load()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	v, _ := s.GetSetting(SectionAdmin)
	if v == "" {
		t.Fatal("bootstrap should import yml admin password into settings")
	}

	// 幂等：二次导入不覆盖 DB 中已修改的密码。
	if err := SaveAdminPassword(s, site, "changed"); err != nil {
		t.Fatalf("change password: %v", err)
	}
	if err := Bootstrap(s, site.Load()); err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	if site.Load().Admin.Password != "changed" {
		t.Fatalf("second bootstrap must not overwrite DB password, got %q", site.Load().Admin.Password)
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

	if err := Bootstrap(s, site.Load()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	for _, sec := range []string{SectionCover, SectionNavTabs, SectionAuthors, SectionPlugins} {
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
		{SectionNavTabs, `{"enable_custom":true,"items":[{"title":"GitHub","url":"https://github.com","external":true}]}`, func() bool { return site.Load().NavTabs.EnableCustom && len(site.Load().NavTabs.Items) == 1 && site.Load().NavTabs.Items[0].External }},
		{SectionAuthors, `{"sakura":{"name":"Sakura","description":"作者"}}`, func() bool { return site.Load().Authors["sakura"].Name == "Sakura" }},
		{SectionPlugins, `{"heti":{"enable":true},"fancybox":{"selector":"div.lazy-box>img.lazy"}}`, func() bool { return site.Load().Plugins.Heti != nil && site.Load().Plugins.Heti.Enable && site.Load().Plugins.Fancybox != nil && site.Load().Plugins.Fancybox.Selector != "" }},
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
