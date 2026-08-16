// 整站恢复（Spec 41）：zip 全量包 / 本地快照 一键还原。
// 恢复前自动做 .pre-restore 兜底快照；zip 解包全程防路径穿越（zip slip）。
package backup

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SetRestoreHook 注册恢复完成回调（main.go 装配：FTS 重建 + 站点配置热更新 + OG 缓存清理）。
func (s *Service) SetRestoreHook(fn func()) {
	s.restoreHook = fn
}

func (s *Service) fireRestore() {
	if s.restoreHook != nil {
		s.restoreHook()
	}
}

// Restore 从全量备份 zip 恢复：blog.db 表级灌回 + uploads 覆盖合并 + site.yml 还原。
// strict 为真时先清空 uploads 再解包（严格模式）；缺 blog.db 的包直接拒绝。
// 安全审计 2026-08-16：补 zip bomb 防护（条目数/单条解压后大小/总解压量上限）与
// uploads 扩展名白名单（拒绝向同源可直读的 /uploads 植入 .html/.js 等可执行内容）。
func (s *Service) Restore(data []byte, strict bool) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("invalid zip: %w", err)
	}
	if err := validateZipBounds(zr); err != nil {
		return err
	}

	// 1. 结构校验：必须含根层 blog.db；所有条目防 zip slip
	var dbEntry *zip.File
	for _, zf := range zr.File {
		name := filepath.ToSlash(zf.Name)
		if strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
			return fmt.Errorf("zip entry %q rejected: path traversal", zf.Name)
		}
		if name == "blog.db" {
			dbEntry = zf
		}
	}
	if dbEntry == nil {
		return fmt.Errorf("not a full backup zip: blog.db missing")
	}

	// 2. 恢复前兜底快照
	if err := s.preRestoreSnapshot(); err != nil {
		return err
	}

	// 3. blog.db 解到临时文件 → 表级灌回
	tmp := filepath.Join(s.backupDir, fmt.Sprintf(".restore-src-%d.db", time.Now().UnixNano()))
	if err := extractTo(dbEntry, tmp, maxDBEntryUncomp); err != nil {
		return err
	}
	defer os.Remove(tmp)
	if err := s.store.RestoreFrom(tmp); err != nil {
		return fmt.Errorf("restore db: %w", err)
	}

	// 4. uploads：严格模式先清空；包内文件覆盖同名，其余保留
	if strict && s.uploadDir != "" {
		if err := clearDir(s.uploadDir); err != nil {
			log.Printf("restore: clear uploads: %v", err)
		}
	}
	for _, zf := range zr.File {
		name := filepath.ToSlash(zf.Name)
		if zf.FileInfo().IsDir() || !strings.HasPrefix(name, "uploads/") {
			continue
		}
		rel := strings.TrimPrefix(name, "uploads/")
		if rel == "" {
			continue
		}
		// 安全审计 2026-08-16：uploads 目录经 /uploads/* 同源直读（nosniff 不阻止
		// text/html 按文档渲染），恢复包此前对条目类型零限制——可植入 .html/.js
		// 承载同源钓鱼/脚本。对齐附件上传的图片扩展名白名单。
		if !isAllowedUploadExt(rel) {
			return fmt.Errorf("zip entry %q rejected: uploads 只允许图片附件扩展名", zf.Name)
		}
		dst := filepath.Join(s.uploadDir, filepath.FromSlash(rel))
		// 双保险：拼接后再校验仍在 uploadDir 内
		if !strings.HasPrefix(dst, filepath.Clean(s.uploadDir)+string(os.PathSeparator)) {
			return fmt.Errorf("zip entry %q rejected: escapes upload dir", zf.Name)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("mkdir for %s: %w", rel, err)
		}
		if err := extractTo(zf, dst, maxZipEntryUncomp); err != nil {
			return err
		}
	}

	// 5. site.yml 还原（包内存在且配置了路径才写）
	if s.sitePath != "" {
		for _, zf := range zr.File {
			if filepath.ToSlash(zf.Name) != "site.yml" {
				continue
			}
			if err := extractTo(zf, s.sitePath, maxZipEntryUncomp); err != nil {
				return err
			}
			break
		}
	}

	s.fireRestore()
	return nil
}

// RestoreSnapshot 从 backups 目录内的本地条目恢复。
// name 白名单校验：仅目录内的 .db 快照或 blog-backup-*.zip，禁止任何路径成分。
func (s *Service) RestoreSnapshot(name string) error {
	if name == "" || name != filepath.Base(name) || strings.Contains(name, "..") {
		return fmt.Errorf("invalid snapshot name")
	}
	path := filepath.Join(s.backupDir, name)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return fmt.Errorf("snapshot %q not found", name)
	}
	switch {
	case strings.HasSuffix(name, ".zip"):
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read snapshot: %w", err)
		}
		return s.Restore(data, false)
	case strings.HasSuffix(name, ".db"):
		if err := s.preRestoreSnapshot(); err != nil {
			return err
		}
		if err := s.store.RestoreFrom(path); err != nil {
			return fmt.Errorf("restore db: %w", err)
		}
		s.fireRestore()
		return nil
	default:
		return fmt.Errorf("unsupported snapshot type: %s", name)
	}
}

// preRestoreSnapshot 恢复前对当前库做一次兜底快照（.pre-restore-<ts>.db），失败中止恢复。
func (s *Service) preRestoreSnapshot() error {
	if err := os.MkdirAll(s.backupDir, 0o755); err != nil {
		return fmt.Errorf("mkdir backup dir: %w", err)
	}
	snap := filepath.Join(s.backupDir, fmt.Sprintf(".pre-restore-%d.db", time.Now().UnixNano()))
	if err := s.store.BackupTo(snap); err != nil {
		return fmt.Errorf("pre-restore snapshot: %w", err)
	}
	log.Printf("restore: pre-restore snapshot saved: %s", filepath.Base(snap))
	return nil
}

// zip bomb 防护上限（安全审计 2026-08-16）：与 Markdown 导入的 500 条/32MB 同量级，
// 另加总解压量约束（500MB 压缩包理论上可解出千倍体积写满磁盘）。
// blog.db 单列大上限：正常运行多年的库（含 page_views/评论）可合法超过附件上限。
const (
	maxRestoreEntries = 2000     // 条目总数上限
	maxZipEntryUncomp = 64 << 20 // 单条目（uploads 附件）声明解压后大小上限
	maxDBEntryUncomp  = 1 << 30  // blog.db 条目声明解压后大小上限
	maxZipTotalUncomp = 2 << 30  // 全包声明解压总量上限（2GB）
)

// entryUncompLimit 按条目名返回解压上限（blog.db 放宽，其余附件 64MB）。
func entryUncompLimit(name string) int64 {
	if filepath.ToSlash(name) == "blog.db" {
		return maxDBEntryUncomp
	}
	return maxZipEntryUncomp
}

// validateZipBounds 校验 zip 包的条目数与解压后体积声明（zip bomb 防护）。
// UncompressedSize64 为 zip 条目头声明值，恶意包可谎报——extractTo 内的
// LimitReader 作为第二道兜底。
func validateZipBounds(zr *zip.Reader) error {
	if len(zr.File) > maxRestoreEntries {
		return fmt.Errorf("zip contains %d entries (max %d)", len(zr.File), maxRestoreEntries)
	}
	var total uint64
	for _, zf := range zr.File {
		limit := uint64(entryUncompLimit(zf.Name))
		if zf.UncompressedSize64 > limit {
			return fmt.Errorf("zip entry %q declares %d bytes (max %d)", zf.Name, zf.UncompressedSize64, limit)
		}
		total += zf.UncompressedSize64
		if total > maxZipTotalUncomp {
			return fmt.Errorf("zip total uncompressed size exceeds %d bytes", maxZipTotalUncomp)
		}
	}
	return nil
}

// isAllowedUploadExt uploads 条目扩展名白名单（对齐 api 包附件上传的 allowedExtMIME）。
// svg 保留（/uploads 对 .svg 单独下发 CSP sandbox，脚本不可执行）。
func isAllowedUploadExt(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg":
		return true
	}
	return false
}

// extractTo 解压单个 zip 条目到目标路径（覆盖写），limit 为该条目解压量上限。
// Spec 62：0600 权限（备份/恢复含 DB 文件，防同机其他用户读取）。
// 安全审计 2026-08-16：io.Copy 套 LimitReader——恶意包可谎报 UncompressedSize64，
// 实际解压流超限即报错，作为 validateZipBounds 的第二道兜底。
func extractTo(zf *zip.File, dst string, limit int64) error {
	rc, err := zf.Open()
	if err != nil {
		return fmt.Errorf("open zip entry %s: %w", zf.Name, err)
	}
	defer rc.Close()
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer f.Close()
	n, err := io.Copy(f, io.LimitReader(rc, limit+1))
	if err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	if n > limit {
		return fmt.Errorf("zip entry %s exceeds uncompressed size limit", zf.Name)
	}
	return nil
}

// clearDir 清空目录内容（保留目录本身），uploads 严格模式用。
func clearDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
