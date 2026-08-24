package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalAttachmentStorageSaveThumbRemove(t *testing.T) {
	root := t.TempDir()
	storage := NewLocalAttachmentStorage(root)
	if err := storage.Save("2026/08/main.png", []byte("main")); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := storage.Thumb("2026/08/thumb_main.png", []byte("thumb")); err != nil {
		t.Fatalf("thumb: %v", err)
	}
	main, err := os.ReadFile(filepath.Join(root, "2026/08/main.png"))
	if err != nil || string(main) != "main" {
		t.Fatalf("saved main = %q, err=%v", main, err)
	}
	if err := storage.Remove("2026/08/main.png"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := storage.Remove("2026/08/main.png"); err != nil {
		t.Fatalf("remove missing should be idempotent: %v", err)
	}
	if err := storage.Save("../outside", []byte("no")); err == nil {
		t.Fatal("path traversal should be rejected")
	}
}
