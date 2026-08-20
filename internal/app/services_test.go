package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBootstrapOwnsSharedServices(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config", "app.yml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("auto_backup: false\n"), 0o644); err != nil {
		t.Fatalf("write app config: %v", err)
	}
	sitePath := filepath.Join(root, "config", "site.yml")
	if err := os.WriteFile(sitePath, []byte("title: Test Blog\nadmin:\n  password: test-password\n"), 0o600); err != nil {
		t.Fatalf("write site config: %v", err)
	}

	services, err := Bootstrap(Options{Root: root, ConfigPath: configPath})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if services.Store == nil || services.Settings == nil || services.Post == nil || services.Comment == nil || services.Identity == nil {
		t.Fatal("bootstrap did not construct core shared services")
	}
	if services.Icons == nil || services.Icons.Len() == 0 {
		t.Fatal("embedded icons were not loaded")
	}
	services.StartBackground()
	services.StartBackground()
	if err := services.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestBootstrapProductionRequiresSiteConfig(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config", "app.yml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("auto_backup: false\n"), 0o644); err != nil {
		t.Fatalf("write app config: %v", err)
	}

	if _, err := Bootstrap(Options{Root: root, ConfigPath: configPath}); err == nil {
		t.Fatal("production bootstrap should reject missing site.yml")
	}
}

func TestBootstrapDevModeFallsBackWithoutSiteConfig(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config", "app.yml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("auto_backup: false\n"), 0o644); err != nil {
		t.Fatalf("write app config: %v", err)
	}
	iconsPath := filepath.Join(root, "icons.yml")
	if err := os.WriteFile(iconsPath, []byte("test:icon: <svg></svg>\n"), 0o600); err != nil {
		t.Fatalf("write icons config: %v", err)
	}

	services, err := Bootstrap(Options{Root: root, ConfigPath: configPath, IconsPath: iconsPath, DevMode: true})
	if err != nil {
		t.Fatalf("dev bootstrap should allow missing site.yml: %v", err)
	}
	if err := services.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestBootstrapProductionRequiresAdminPassword(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config", "app.yml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("auto_backup: false\n"), 0o644); err != nil {
		t.Fatalf("write app config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "site.yml"), []byte("title: Test Blog\n"), 0o600); err != nil {
		t.Fatalf("write site config: %v", err)
	}

	if _, err := Bootstrap(Options{Root: root, ConfigPath: configPath}); err == nil {
		t.Fatal("production bootstrap should reject a site without an admin password")
	}
}
