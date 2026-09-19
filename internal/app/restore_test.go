package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"blog/internal/store"
)

func TestBootstrapCompensatesBeforeParsingSiteYAML(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "app.yml"), []byte("auto_backup: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(root, "data", "blog.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Upsert(&store.Post{Slug: "keep", Title: "before", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	id := "restore-0123456789abcdef0123456789abcdef"
	backupDir := filepath.Join(root, "data", "backups")
	before := filepath.Join(backupDir, "restore-tasks", id, "preimage")
	if err := os.MkdirAll(filepath.Join(before, "uploads"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := st.BackupTo(filepath.Join(before, "blog.db")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(before, "site.yml"), []byte("title: before\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(before, "uploads", "state.png"), []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := st.Upsert(&store.Post{Slug: "keep", Title: "after", Status: store.StatusPublished}); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	uploadDir := filepath.Join(root, "data", "uploads")
	if err := os.MkdirAll(uploadDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uploadDir, "state.png"), []byte("after"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "site.yml"), []byte("title: [interrupted"), 0600); err != nil {
		t.Fatal(err)
	}
	journal := map[string]any{"version": 1, "id": id, "status": "applying", "preimage_ready": true, "has_preimage": true, "restore_site": true, "site_existed": true, "site_mode": 0600, "uploads_existed": true}
	data, err := json.Marshal(journal)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, ".restore-"+id+".json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	iconsPath := filepath.Join(root, "icons.yml")
	if err := os.WriteFile(iconsPath, []byte("test:icon: <svg></svg>\n"), 0600); err != nil {
		t.Fatal(err)
	}
	services, err := Bootstrap(Options{Root: root, DevMode: true, IconsPath: iconsPath})
	if err != nil {
		t.Fatal(err)
	}
	defer services.Close(context.Background())
	if services.Maintenance.Active() {
		t.Fatal("bootstrap stayed in recovery mode")
	}
	post, err := services.Store.GetBySlug("keep")
	if err != nil || post == nil || post.Title != "before" {
		t.Fatalf("database was not recovered before bootstrap: %v", err)
	}
	if services.Site.Load().Title != "before" {
		t.Fatal("site YAML was not recovered before parsing")
	}
	image, err := os.ReadFile(filepath.Join(uploadDir, "state.png"))
	if err != nil || string(image) != "before" {
		t.Fatalf("upload not recovered: %q %v", image, err)
	}
}
func TestBootstrapBadRestoreJournalOnlyEnablesMaintenance(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "data", "backups")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	id := "restore-0123456789abcdef0123456789abcdef"
	if err := os.WriteFile(filepath.Join(dir, ".restore-"+id+".json"), []byte(`{"version":99}`), 0600); err != nil {
		t.Fatal(err)
	}
	iconsPath := filepath.Join(root, "icons.yml")
	if err := os.WriteFile(iconsPath, []byte("test:icon: <svg></svg>\n"), 0600); err != nil {
		t.Fatal(err)
	}
	services, err := Bootstrap(Options{Root: root, DevMode: true, IconsPath: iconsPath})
	if err != nil {
		t.Fatal(err)
	}
	if !services.Maintenance.Active() || services.Store != nil || services.Site != nil {
		t.Fatal("unsafe bootstrap continued past recovery error")
	}
	services.StartBackground()
	if err := services.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}
