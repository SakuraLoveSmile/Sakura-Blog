package stellarcli

import (
	"path/filepath"
	"testing"

	"blog/internal/config"
)

func TestResolvedToolAppUsesLockedSnapshotWithoutEnvironmentOverride(t *testing.T) {
	root := t.TempDir()
	locked := &config.App{Root: root, DBPath: filepath.Join(root, "locked.db"), UploadDir: filepath.Join(root, "uploads")}
	t.Setenv("BLOG_DB", filepath.Join(root, "unlocked.db"))
	t.Setenv("BLOG_ROOT", filepath.Join(root, "other-root"))
	resolved, err := ResolveApp(Options{ResolvedApp: locked, ConfigPath: filepath.Join(root, "invalid-app.yml")})
	if err != nil || resolved.DBPath != locked.DBPath || resolved.Root != locked.Root {
		t.Fatalf("locked paths overridden: %+v %v", resolved, err)
	}
	resolved.DBPath = "changed"
	if locked.DBPath == "changed" {
		t.Fatal("command mutated the held configuration snapshot")
	}
}
