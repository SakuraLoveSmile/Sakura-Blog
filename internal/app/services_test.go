package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"blog/internal/config"
)

func TestBootstrapUsesResolvedPathSnapshot(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config", "app.yml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "site.yml"), []byte("title: Snapshot Test\nadmin:\n  password: test-password\n"), 0600); err != nil {
		t.Fatal(err)
	}
	snapshot := config.DefaultApp()
	snapshot.Root = root
	snapshot.DBPath = filepath.Join(root, "data", "locked.db")
	snapshot.UploadDir = filepath.Join(root, "data", "locked-uploads")
	snapshot.AutoBackup = false
	// 模拟持锁后配置与环境变化，不能让 Store 打开未持锁的新路径。
	unexpected := filepath.Join(t.TempDir(), "unlocked.db")
	if err := os.WriteFile(configPath, []byte("db_path: "+unexpected+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BLOG_DB", unexpected)
	t.Setenv("BLOG_ROOT", t.TempDir())
	services, err := Bootstrap(Options{Root: t.TempDir(), ConfigPath: configPath, ResolvedApp: snapshot})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := services.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	if services.App.DBPath != snapshot.DBPath || services.App.Root != snapshot.Root || services.App.UploadDir != snapshot.UploadDir || services.App == snapshot {
		t.Fatal("bootstrap did not own a copy of the locked configuration snapshot")
	}
	if _, err := os.Stat(snapshot.DBPath); err != nil {
		t.Fatalf("locked database was not opened: %v", err)
	}
	if _, err := os.Stat(unexpected); !os.IsNotExist(err) {
		t.Fatal("bootstrap opened the newly configured unlocked database")
	}
}

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
