package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestCleanStaleSnapshots 陈旧快照清理：只删超龄的 .snapshot-*.db，
// 新临时文件、正式备份 zip、同名目录与无关文件不受影响。
func TestCleanStaleSnapshots(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, ".snapshot-100.db") // 25h 前，超龄
	fresh := filepath.Join(dir, ".snapshot-200.db") // 刚生成，留存
	keep := []string{
		fresh,
		filepath.Join(dir, "blog-backup-20260101.zip"), // 正式备份不动
		filepath.Join(dir, "other.db"),                 // 非 snapshot 命名不动
	}
	for _, p := range keep {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	if err := os.WriteFile(stale, []byte("x"), 0o644); err != nil {
		t.Fatalf("write stale: %v", err)
	}
	old := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatalf("chtimes stale: %v", err)
	}
	// 同名模式的目录不应被当作文件删除。
	if err := os.Mkdir(filepath.Join(dir, ".snapshot-300.db"), 0o755); err != nil {
		t.Fatalf("mkdir decoy: %v", err)
	}

	if n := cleanStaleSnapshots(dir, 24*time.Hour); n != 1 {
		t.Fatalf("cleanStaleSnapshots removed %d, want 1", n)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale snapshot still exists: %v", err)
	}
	for _, p := range keep {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("unexpectedly removed %s: %v", p, err)
		}
	}
}

// TestCleanStaleSnapshotsMissingDir 目录不存在时静默返回 0（fresh clone / 未备份过的站点）。
func TestCleanStaleSnapshotsMissingDir(t *testing.T) {
	if n := cleanStaleSnapshots(filepath.Join(t.TempDir(), "nope"), time.Hour); n != 0 {
		t.Fatalf("missing dir removed %d, want 0", n)
	}
}
