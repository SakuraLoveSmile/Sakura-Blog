package app

import (
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
	if err := services.Close(nil); err != nil {
		t.Fatalf("close: %v", err)
	}
}
