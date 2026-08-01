package config

import (
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

func TestSaveAdminPasswordOverridesAndPersists(t *testing.T) {
	s := newTestStore(t)
	site := &tmpl.Site{}
	site.Admin.Password = "yml-pass"

	if err := SaveAdminPassword(s, site, "db-pass"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if site.Admin.Password != "db-pass" {
		t.Fatalf("hot update failed: password = %q", site.Admin.Password)
	}
	v, err := s.GetSetting(SectionAdmin)
	if err != nil || v == "" {
		t.Fatalf("admin section not persisted: %q, %v", v, err)
	}
}

func TestSaveAdminPasswordEmptyDisablesGate(t *testing.T) {
	s := newTestStore(t)
	site := &tmpl.Site{}
	site.Admin.Password = "old"

	if err := SaveAdminPassword(s, site, ""); err != nil {
		t.Fatalf("save empty: %v", err)
	}
	if site.Admin.Password != "" {
		t.Fatalf("empty password should disable gate, got %q", site.Admin.Password)
	}
}

func TestRebuildSiteKeepsYmlPasswordWhenNoAdminSection(t *testing.T) {
	s := newTestStore(t)
	site := &tmpl.Site{}
	site.Admin.Password = "yml-pass"

	if err := RebuildSite(s, site); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if site.Admin.Password != "yml-pass" {
		t.Fatalf("yml password should be kept when DB has no admin section, got %q", site.Admin.Password)
	}
}

func TestRebuildSiteDbOverridesYml(t *testing.T) {
	s := newTestStore(t)
	if err := s.PutSetting(SectionAdmin, `{"password":"db-pass"}`); err != nil {
		t.Fatalf("seed admin section: %v", err)
	}
	site := &tmpl.Site{}
	site.Admin.Password = "yml-pass"

	if err := RebuildSite(s, site); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if site.Admin.Password != "db-pass" {
		t.Fatalf("DB admin section should override yml, got %q", site.Admin.Password)
	}
}

func TestBootstrapImportsAdminPassword(t *testing.T) {
	s := newTestStore(t)
	site := &tmpl.Site{}
	site.Admin.Password = "boot-pass"

	if err := Bootstrap(s, site); err != nil {
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
	if err := Bootstrap(s, site); err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	if site.Admin.Password != "changed" {
		t.Fatalf("second bootstrap must not overwrite DB password, got %q", site.Admin.Password)
	}
}
