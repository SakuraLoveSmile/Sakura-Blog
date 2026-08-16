// 整站恢复行为测试（Spec 41）：全流程往返、zip slip 拒绝、快照名白名单、schema 护栏。
package backup

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"blog/internal/store"
)

// newTestService 建临时库 + 备份服务（uploads/backups 均在 t.TempDir 下）。
func newTestService(t *testing.T) (*Service, *store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.New(filepath.Join(dir, "blog.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	uploadDir := filepath.Join(dir, "uploads")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		t.Fatalf("mkdir uploads: %v", err)
	}
	svc := New(Config{
		Store:     st,
		UploadDir: uploadDir,
		BackupDir: filepath.Join(dir, "backups"),
		Keep:      3,
	})
	return svc, st, dir
}

// TestRestoreRoundTrip 导出 → 改数据 → 恢复 → 数据回到导出时点；恢复前留 .pre-restore 快照。
func TestRestoreRoundTrip(t *testing.T) {
	svc, st, _ := newTestService(t)

	// 初始数据 + 一个附件文件
	if err := st.Upsert(&store.Post{Slug: "keep", Title: "原标题", Status: store.StatusPublished}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := os.WriteFile(filepath.Join(svc.uploadDir, "a.png"), []byte("v1"), 0o644); err != nil {
		t.Fatalf("write upload: %v", err)
	}

	// 导出全量包
	var buf bytes.Buffer
	if err := svc.WriteZip(&buf); err != nil {
		t.Fatalf("write zip: %v", err)
	}

	// 改数据：改标题 + 加新文章 + 改附件
	if err := st.Upsert(&store.Post{Slug: "keep", Title: "改过的标题", Status: store.StatusPublished}); err != nil {
		t.Fatalf("upsert2: %v", err)
	}
	if err := st.Upsert(&store.Post{Slug: "extra", Title: "新文章", Status: store.StatusPublished}); err != nil {
		t.Fatalf("upsert3: %v", err)
	}
	if err := os.WriteFile(filepath.Join(svc.uploadDir, "a.png"), []byte("v2"), 0o644); err != nil {
		t.Fatalf("rewrite upload: %v", err)
	}

	hookFired := false
	svc.SetRestoreHook(func() { hookFired = true })

	// 恢复
	if err := svc.Restore(buf.Bytes(), false); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if !hookFired {
		t.Fatalf("restore hook not fired")
	}

	// 标题回到导出时点
	p, err := st.GetBySlug("keep")
	if err != nil || p == nil {
		t.Fatalf("get keep: %v", err)
	}
	if p.Title != "原标题" {
		t.Fatalf("title = %q, want 原标题", p.Title)
	}
	// 导出后新增的文章被覆盖掉
	if p2, _ := st.GetBySlug("extra"); p2 != nil {
		t.Fatalf("extra post should be gone after restore")
	}
	// 附件内容回退
	if data, _ := os.ReadFile(filepath.Join(svc.uploadDir, "a.png")); string(data) != "v1" {
		t.Fatalf("upload content = %q, want v1", data)
	}
	// 兜底快照存在
	entries, _ := os.ReadDir(svc.backupDir)
	found := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".pre-restore-") {
			found = true
		}
	}
	if !found {
		t.Fatalf("pre-restore snapshot missing")
	}
}

// TestRestoreRejectsZipSlip 路径穿越条目直接拒绝，且不产生任何恢复副作用。
func TestRestoreRejectsZipSlip(t *testing.T) {
	svc, _, _ := newTestService(t)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("uploads/../../evil.txt")
	w.Write([]byte("evil")) //nolint:errcheck
	w2, _ := zw.Create("blog.db")
	w2.Write([]byte("fake")) //nolint:errcheck
	zw.Close()

	if err := svc.Restore(buf.Bytes(), false); err == nil || !strings.Contains(err.Error(), "path traversal") {
		t.Fatalf("expect path traversal rejection, got %v", err)
	}
}

// TestRestoreRejectsNonImageUploads 安全审计 2026-08-16：恢复包 uploads/ 条目
// 仅放行图片扩展名——/uploads 同源直读，植入 .html/.js 即存储型 XSS/后门面。
func TestRestoreRejectsNonImageUploads(t *testing.T) {
	svc, st, dir := newTestService(t)
	// 真实 DB 快照作 blog.db（伪字节会在 attach 阶段先行失败，到不了 uploads 校验）
	snap := filepath.Join(dir, "src.db")
	if err := st.BackupTo(snap); err != nil {
		t.Fatalf("backup to: %v", err)
	}
	dbBytes, err := os.ReadFile(snap)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("blog.db")
	w.Write(dbBytes) //nolint:errcheck
	w2, _ := zw.Create("uploads/evil.html")
	w2.Write([]byte("<script>alert(1)</script>")) //nolint:errcheck
	zw.Close()

	if err := svc.Restore(buf.Bytes(), false); err == nil || !strings.Contains(err.Error(), "图片附件扩展名") {
		t.Fatalf("expect non-image uploads rejection, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(svc.uploadDir, "evil.html")); !os.IsNotExist(err) {
		t.Fatalf("evil.html should not be written, stat err = %v", err)
	}
}

// TestRestoreRejectsNonBackupZip 缺 blog.db 的包拒绝。
func TestRestoreRejectsNonBackupZip(t *testing.T) {
	svc, _, _ := newTestService(t)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("readme.txt")
	w.Write([]byte("hi")) //nolint:errcheck
	zw.Close()
	if err := svc.Restore(buf.Bytes(), false); err == nil || !strings.Contains(err.Error(), "blog.db missing") {
		t.Fatalf("expect blog.db missing rejection, got %v", err)
	}
}

// TestRestoreSnapshotWhitelist 快照名白名单：路径成分/目录外条目/不存在文件均拒绝。
func TestRestoreSnapshotWhitelist(t *testing.T) {
	svc, _, dir := newTestService(t)

	// 目录外诱饵文件
	outside := filepath.Join(dir, "outside.db")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatalf("write outside: %v", err)
	}

	for _, name := range []string{"../outside.db", "/etc/passwd", "a/../b.db", "", "nonexist.db", "note.txt"} {
		if err := svc.RestoreSnapshot(name); err == nil {
			t.Fatalf("RestoreSnapshot(%q) should fail", name)
		}
	}
}

// TestRestoreSnapshotDB .db 快照恢复走通（用 BackupTo 生成的真快照）。
func TestRestoreSnapshotDB(t *testing.T) {
	svc, st, _ := newTestService(t)
	if err := st.Upsert(&store.Post{Slug: "s1", Title: "v1", Status: store.StatusPublished}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := os.MkdirAll(svc.backupDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := st.BackupTo(filepath.Join(svc.backupDir, "manual.db")); err != nil {
		t.Fatalf("backup to: %v", err)
	}
	if err := st.Upsert(&store.Post{Slug: "s1", Title: "v2", Status: store.StatusPublished}); err != nil {
		t.Fatalf("upsert2: %v", err)
	}
	if err := svc.RestoreSnapshot("manual.db"); err != nil {
		t.Fatalf("restore snapshot: %v", err)
	}
	p, _ := st.GetBySlug("s1")
	if p == nil || p.Title != "v1" {
		t.Fatalf("title after restore = %v, want v1", p)
	}
}

// TestRestoreFromRejectsNewerSchema 备份含未知表 → 拒绝并提示升级。
func TestRestoreFromRejectsNewerSchema(t *testing.T) {
	svc, st, dir := newTestService(t)
	_ = svc

	// 用另一个 store 生成快照后，直接给快照加一张未知表模拟「新版本备份」
	src := filepath.Join(dir, "newer.db")
	if err := st.BackupTo(src); err != nil {
		t.Fatalf("backup to: %v", err)
	}
	st2, err := store.New(src)
	if err != nil {
		t.Fatalf("open newer: %v", err)
	}
	if _, err := st2.DB().Exec(`CREATE TABLE future_feature (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create future table: %v", err)
	}
	st2.Close()

	if err := st.RestoreFrom(src); err == nil || !strings.Contains(err.Error(), "schema is newer") {
		t.Fatalf("expect newer-schema rejection, got %v", err)
	}
}
