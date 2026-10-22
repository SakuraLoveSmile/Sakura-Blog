package stellar

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"blog/internal/settings"
)

func TestToolLocalAssetsWhitelistAndSingleDecode(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a b.png"), []byte("image"), 0600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.png")); err != nil {
		t.Fatal(err)
	}
	icons, err := settings.LoadIconsEmbed()
	if err != nil {
		t.Fatal(err)
	}
	read, closeAssets, err := ToolLocalAssets(root, icons)
	if err != nil {
		t.Fatal(err)
	}
	defer closeAssets()
	for _, path := range []string{"uploads/a%20b.png", "assets/img/share/link.svg", "js/icons/default.json", "assets/js/icons/default.json"} {
		r, err := read(context.Background(), path)
		if err != nil {
			t.Fatalf("allowed %s: %v", path, err)
		}
		b, err := io.ReadAll(r)
		r.Close()
		if err != nil || len(b) == 0 {
			t.Fatalf("empty asset %s %v", path, err)
		}
	}
	for _, path := range []string{"uploads/../secret", "uploads/%2e%2e/secret", "uploads/a%2fb.png", "uploads/a%5cb.png", "uploads/%252e%252e/secret", "uploads/%00", "uploads/%ff", "uploads/escape.png", "/uploads/a%20b.png", "config/app.yml", "file:///etc/passwd", "assets/../templates/layout/base.html", "uploads/a.png?x=1", "uploads/a#b", "uploads/"} {
		if r, err := read(context.Background(), path); err == nil {
			r.Close()
			t.Fatalf("unsafe path accepted: %q", path)
		}
	}
}

func TestToolEnvironmentReportsEmbeddedIdentity(t *testing.T) {
	env, err := ToolEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	if env.Theme != StellarVersion || env.Adapter != "go-ssr-sqlite" || !strings.HasPrefix(env.Go, "go1.") || len(env.ResourceSHA256) != 64 {
		t.Fatalf("invalid environment: %#v", env)
	}
	again, err := ToolEnvironment()
	if err != nil || again != env {
		t.Fatal("unstable embedded fingerprint")
	}
}
