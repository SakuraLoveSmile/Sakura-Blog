package backup

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

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
