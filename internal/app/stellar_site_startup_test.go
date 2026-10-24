package app

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"blog/internal/config"
	"blog/internal/store"
)

func canonicalStartupFixture(t *testing.T, admin bool) (string, *store.Store) {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "app.yml"), []byte("auto_backup: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	st, err := store.New(filepath.Join(root, "data", "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	for _, ensure := range []func() error{st.EnsureSettingsTable, st.EnsureLinksTable, st.EnsureUsersTable} {
		if err := ensure(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := config.InitializeStellarV2(st, map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	if err := st.PutSetting(config.SectionSite, `{"title":"Stored canonical site","url":"https://fixture.example"}`); err != nil {
		t.Fatal(err)
	}
	// 此旧段刻意与现有哈希不同；DB启动不得再次执行明文身份迁移。
	if err := st.PutSetting(config.SectionAdmin, `{"password":"different-legacy-password"}`); err != nil {
		t.Fatal(err)
	}
	if admin {
		hash, err := store.HashPassword("existing-admin-password")
		if err != nil {
			t.Fatal(err)
		}
		if err := st.CreateUser(&store.User{Username: "admin", PasswordHash: hash, AuthSecret: "retained-auth-secret", Role: store.RoleAdmin}); err != nil {
			t.Fatal(err)
		}
	}
	return root, st
}

func TestBootstrapCanonicalDatabaseWithoutSiteFile(t *testing.T) {
	root, st := canonicalStartupFixture(t, true)
	before, err := st.AllSettings()
	if err != nil {
		t.Fatal(err)
	}
	user, err := st.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	services, err := Bootstrap(Options{Root: root})
	if err != nil {
		t.Fatalf("canonical database must start without an initialization file: %v", err)
	}
	defer func() { _ = services.Close(context.Background()) }()
	site := services.Site.Load()
	if site.Title != "Stored canonical site" || site.URL != "https://fixture.example" || site.StellarV2 == nil || !site.Admin.AuthEnabled {
		t.Fatal("stored configuration or authentication was lost")
	}
	after, err := st.AllSettings()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("canonical startup rewrote settings")
	}
	currentUser, err := st.GetUserByUsername("admin")
	if err != nil || !reflect.DeepEqual(user, currentUser) {
		t.Fatal("canonical startup migrated the legacy password or identity")
	}
	if err := st.PutSetting(config.SectionSite, `{"title":"Restored canonical site"}`); err != nil {
		t.Fatal(err)
	}
	if err := services.rebuildRestoredState(); err != nil {
		t.Fatalf("canonical restore must rebuild without a site file: %v", err)
	}
	if services.Site.Load().Title != "Restored canonical site" {
		t.Fatal("restore did not publish stored site")
	}
	if err := st.PutSetting(config.SectionStellarV2, `{"version":2,"theme":{"style":{}}}`); err != nil {
		t.Fatal(err)
	}
	published := services.Site.Load()
	if err := services.rebuildRestoredState(); err == nil || services.Site.Load() != published {
		t.Fatal("invalid restored canonical settings replaced the live snapshot")
	}
}

func TestBootstrapMissingSiteRejectsIncompleteCanonicalDatabase(t *testing.T) {
	for _, tc := range []struct {
		name    string
		section string
		value   string
		admin   bool
		remove  bool
		drop    string
	}{
		{name: "invalid-theme", section: config.SectionStellarV2, value: `{"version":2,"theme":{"style":{}}}`, admin: true},
		{name: "legacy-only", section: config.SectionStellarV2, value: `{"version":1,"theme":{}}`, admin: true},
		{name: "missing-canonical", section: config.SectionStellarV2, remove: true, admin: true},
		{name: "missing-site", section: config.SectionSite, remove: true, admin: true},
		{name: "empty-site", section: config.SectionSite, admin: true},
		{name: "null-site", section: config.SectionSite, value: "null", admin: true},
		{name: "scalar-site", section: config.SectionSite, value: `"incomplete"`, admin: true},
		{name: "missing-admin"},
		{name: "missing-links-table", admin: true, drop: "links"},
		{name: "missing-users-table", admin: true, drop: "users"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, st := canonicalStartupFixture(t, tc.admin)
			if tc.remove {
				if _, err := st.DB().Exec("DELETE FROM settings WHERE section=?", tc.section); err != nil {
					t.Fatal(err)
				}
			} else if tc.section != "" {
				if err := st.PutSetting(tc.section, tc.value); err != nil {
					t.Fatal(err)
				}
			}
			if tc.drop != "" {
				if _, err := st.DB().Exec("DROP TABLE " + tc.drop); err != nil {
					t.Fatal(err)
				}
			}
			before, err := st.AllSettings()
			if err != nil {
				t.Fatal(err)
			}
			if services, err := Bootstrap(Options{Root: root}); err == nil {
				_ = services.Close(context.Background())
				t.Fatal("incomplete canonical database started")
			}
			after, err := st.AllSettings()
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("rejected startup rewrote settings")
			}
			if tc.drop != "" {
				var count int
				if err := st.DB().QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", tc.drop).Scan(&count); err != nil || count != 0 {
					t.Fatal("rejected startup recreated a missing table")
				}
			} else {
				user, err := st.GetUserByUsername("admin")
				if err != nil || (!tc.admin && user != nil) {
					t.Fatal("rejected startup created an administrator")
				}
			}
		})
	}
}

func TestBootstrapMissingSiteDoesNotCreateDatabase(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "app.yml"), []byte("auto_backup: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if services, err := Bootstrap(Options{Root: root}); err == nil {
		_ = services.Close(context.Background())
		t.Fatal("missing persisted configuration started")
	}
	if _, err := os.Stat(filepath.Join(root, "data")); !os.IsNotExist(err) {
		t.Fatal("rejected startup created the database directory")
	}
}

func TestBootstrapCanonicalDatabaseRejectsMalformedSiteFile(t *testing.T) {
	root, _ := canonicalStartupFixture(t, true)
	if err := os.WriteFile(filepath.Join(root, "config", "site.yml"), []byte("site: [broken\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if services, err := Bootstrap(Options{Root: root}); err == nil {
		_ = services.Close(context.Background())
		t.Fatal("invalid existing site file was ignored")
	}
}
