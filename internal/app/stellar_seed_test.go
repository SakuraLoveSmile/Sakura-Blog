package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"blog/internal/config"
	"blog/internal/store"
)

func stellarAppSeedRoot(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "content", "posts"), 0700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "config", "app.yml")
	if err := os.WriteFile(configPath, []byte("auto_backup: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "site.yml"), []byte("title: Test\nadmin:\n  password: test-password\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return root, configPath
}

func TestBootstrapExplicitStellarSeedAndNewTables(t *testing.T) {
	root, configPath := stellarAppSeedRoot(t)
	if err := os.MkdirAll(filepath.Join(root, "data"), 0700); err != nil {
		t.Fatal(err)
	}
	st, err := store.New(filepath.Join(root, "data", "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureSettingsTable(); err != nil {
		t.Fatal(err)
	}
	if _, err := config.InitializeStellarV2(st, map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	_ = st.Close()
	if err := os.WriteFile(filepath.Join(root, "content", "posts", "new.md"), []byte("---\ntitle: Canonical\nblog:\n  status: draft\n---\n正文"), 0600); err != nil {
		t.Fatal(err)
	}
	services, err := Bootstrap(Options{Root: root, ConfigPath: configPath, Seed: true})
	if err != nil {
		t.Fatal(err)
	}
	defer services.Close(context.Background())
	p, err := services.Store.GetBySlug("new")
	if err != nil || p == nil || p.StellarVersion != 2 || p.Status != store.StatusDraft {
		t.Fatalf("production seed not canonical: %v", err)
	}
	if notebooks, err := services.Store.ListNotebooks(); err != nil || len(notebooks) != 0 {
		t.Fatal("notebook table initialization migrated data")
	}
	if notes, err := services.Store.ListNotes("missing"); err != nil || len(notes) != 0 {
		t.Fatal("note table initialization migrated data")
	}
}

func TestBootstrapExplicitUnmigratedSeedRejectsBeforeLegacyBackfill(t *testing.T) {
	root, configPath := stellarAppSeedRoot(t)
	if err := os.MkdirAll(filepath.Join(root, "data"), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "data", "blog.db")
	st, err := store.New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureSettingsTable(); err != nil {
		t.Fatal(err)
	}
	if err := st.PutSetting("style", `{"prefers_theme":"dark"}`); err != nil {
		t.Fatal(err)
	}
	if err := st.Upsert(&store.Post{Slug: "legacy", ContentMD: "旧正文", Status: store.StatusDraft}); err != nil {
		t.Fatal(err)
	}
	before, _ := st.ReadSettingsSnapshot()
	_ = st.Close()
	if _, err := Bootstrap(Options{Root: root, ConfigPath: configPath, Seed: true}); !errors.Is(err, store.ErrStellarMigrationRequired) {
		t.Fatalf("legacy explicit seed did not require migration: %v", err)
	}
	reopened, err := store.New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	after, _ := reopened.ReadSettingsSnapshot()
	p, _ := reopened.GetBySlug("legacy")
	if !reflect.DeepEqual(before, after) || p.ContentMD != "旧正文" || p.StellarVersion != 0 {
		t.Fatal("failed explicit seed backfilled/migrated legacy configuration or content")
	}
}
