package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagerLocalBackendAndURL(t *testing.T) {
	root := t.TempDir()
	m, err := NewManager(root, DefaultConfig())
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if err := m.Save("2026/08/image.png", []byte("main")); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "2026", "08", "image.png")); err != nil || string(got) != "main" {
		t.Fatalf("saved bytes = %q, err=%v", got, err)
	}
	if got := m.URL("2026/08/image.png"); got != "/uploads/2026/08/image.png" {
		t.Fatalf("local url = %q", got)
	}
	if err := m.Remove("2026/08/image.png"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "2026", "08", "image.png")); !os.IsNotExist(err) {
		t.Fatalf("file still exists, err=%v", err)
	}
}

func TestS3ConfigAndURL(t *testing.T) {
	cfg := Config{
		Provider: ProviderS3, Endpoint: "https://s3.example.test", Region: "cn-test-1",
		Bucket: "blog", AccessKey: "access", SecretKey: "secret", PublicURL: "https://cdn.example.test/blog",
		Prefix: "attachments",
	}
	s3, err := NewS3AttachmentStorage(cfg)
	if err != nil {
		t.Fatalf("new s3 storage: %v", err)
	}
	if got := s3.URL("2026/08/hello world.png"); got != "https://cdn.example.test/blog/attachments/2026/08/hello%20world.png" {
		t.Fatalf("s3 public url = %q", got)
	}
	if err := s3.Save("../escape.png", []byte("no")); err == nil {
		t.Fatal("path traversal should be rejected")
	}
}

func TestValidateStorageConfig(t *testing.T) {
	if _, err := ValidateConfig(Config{Provider: ProviderS3, Endpoint: "http://127.0.0.1", Bucket: "b", AccessKey: "a", SecretKey: "s"}); err != nil {
		t.Fatalf("private endpoint is a configured storage target, not an outbound fetch: %v", err)
	}
	for _, cfg := range []Config{
		{Provider: "unknown"},
		{Provider: ProviderS3, Endpoint: "ftp://s3.example.test", Bucket: "b", AccessKey: "a", SecretKey: "s"},
		{Provider: ProviderS3, Endpoint: "https://s3.example.test", Bucket: "b", AccessKey: "a"},
		{Provider: ProviderLocal, Prefix: "../bad"},
	} {
		if _, err := ValidateConfig(cfg); err == nil {
			t.Fatalf("config %+v should be rejected", cfg)
		}
	}
}
