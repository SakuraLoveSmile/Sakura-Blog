package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"blog/internal/config"
)

func (s *Service) restoreSource(input RestoreInput) ([]byte, bool, error) {
	if input.Snapshot == "" {
		if len(input.Data) > 500<<20 {
			return nil, false, fmt.Errorf("restore upload exceeds size limit")
		}
		return input.Data, false, nil
	}
	name := input.Snapshot
	if name != filepath.Base(name) || strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return nil, false, fmt.Errorf("invalid snapshot name")
	}
	extension := strings.ToLower(filepath.Ext(name))
	if extension != ".db" && extension != ".zip" {
		return nil, false, fmt.Errorf("unsupported snapshot type")
	}
	limit := int64(500 << 20)
	if extension == ".db" {
		limit = maxDBEntryUncomp
	}
	data, err := readRegularFile(filepath.Join(s.backupDir, name), limit)
	return data, extension == ".db", err
}

func (s *Service) prepareRestore(ctx context.Context, j *restoreJournal, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if j.ContentImport {
		if _, _, err := ReadStellarContentZip(data); err != nil {
			return err
		}
		if err := writeFileSync(s.inputPath(j.ID, "content.zip"), data, 0600); err != nil {
			return err
		}
		// 预检只构建隔离副本。进入维护窗口后基于最新 DB 再构建一次，
		// 不能将预检期间的并发写入覆盖为旧快照。
		if err := s.buildStellarImportCandidate(j); err != nil {
			return err
		}
	} else if j.DatabaseOnly {
		if err := writeFileSync(s.inputPath(j.ID, "blog.db"), data, 0600); err != nil {
			return err
		}
	} else {
		reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return fmt.Errorf("invalid zip: %w", err)
		}
		if err := validateZipBounds(reader); err != nil {
			return err
		}
		hasDatabase := false
		for _, entry := range reader.File {
			if entry.Name == "blog.db" {
				hasDatabase = true
			}
		}
		if !hasDatabase {
			return fmt.Errorf("not a full backup zip: blog.db missing")
		}
		entries := make(map[string]*zip.File, len(reader.File))
		isDirectory := map[string]bool{}
		casePaths := map[string]string{}
		var total int64
		for _, entry := range reader.File {
			name := entry.Name
			clean := strings.TrimSuffix(name, "/")
			if clean == "" || strings.Contains(name, "\\") || strings.Contains(name, "\x00") || path.IsAbs(name) || path.Clean(clean) != clean || clean == ".." || strings.HasPrefix(clean, "../") {
				return fmt.Errorf("zip entry rejected: path traversal")
			}
			if entry.Mode()&os.ModeSymlink != 0 || (!entry.FileInfo().IsDir() && !entry.Mode().IsRegular()) {
				return fmt.Errorf("zip entry rejected: non-regular file")
			}
			if _, exists := entries[clean]; exists {
				return fmt.Errorf("zip entry rejected: duplicate target")
			}
			// 跨文件系统恢复时，大小写不同的文件或父目录不能映射到同一目标。
			for candidate := clean; candidate != "."; candidate = path.Dir(candidate) {
				folded := strings.ToLower(candidate)
				if previous, exists := casePaths[folded]; exists && previous != candidate {
					return fmt.Errorf("zip entry rejected: case-insensitive target conflict")
				}
				casePaths[folded] = candidate
			}
			directory := entry.FileInfo().IsDir() || strings.HasSuffix(name, "/")
			if clean != "blog.db" && clean != "site.yml" && clean != "uploads" && !strings.HasPrefix(clean, "uploads/") {
				return fmt.Errorf("unexpected full backup entry")
			}
			if (clean == "blog.db" || clean == "site.yml") && directory {
				return fmt.Errorf("backup database/configuration must be regular files")
			}
			if clean == "uploads" && !directory {
				return fmt.Errorf("uploads must be a directory")
			}
			if !directory && strings.HasPrefix(clean, "uploads/") && !isAllowedUploadExt(clean) {
				return fmt.Errorf("uploads 只允许图片附件扩展名")
			}
			entries[clean] = entry
			isDirectory[clean] = directory
			total += int64(entry.UncompressedSize64)
		}
		for name := range entries {
			for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
				if _, exists := entries[parent]; exists && !isDirectory[parent] {
					return fmt.Errorf("zip entry rejected: file/directory conflict")
				}
			}
		}
		if entries["blog.db"] == nil {
			return fmt.Errorf("not a full backup zip: blog.db missing")
		}
		if err := checkFreeSpace(s.backupDir, total); err != nil {
			return err
		}
		for name, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			target := s.inputPath(j.ID, filepath.FromSlash(name))
			if isDirectory[name] {
				if err := os.MkdirAll(target, 0700); err != nil {
					return err
				}
				continue
			}
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				return err
			}
			if err := extractTo(entry, target, entryUncompLimit(name)); err != nil {
				return err
			}
			if name == "site.yml" {
				j.RestoreSite = s.sitePath != ""
				if _, err := config.LoadSite(target); err != nil {
					return fmt.Errorf("invalid restore YAML: %w", err)
				}
			}
		}
	}
	if err := s.store.ValidateRestoreFrom(s.inputPath(j.ID, "blog.db")); err != nil {
		return fmt.Errorf("restore database preflight: %w", err)
	}
	return s.checkRestoreTargets(j)
}

func (s *Service) checkRestoreTargets(j *restoreJournal) error {
	if err := writableDirectory(s.backupDir); err != nil {
		return err
	}
	if s.uploadDir != "" {
		if info, err := os.Lstat(s.uploadDir); err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			return fmt.Errorf("uploads target is not a regular directory")
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := writableDirectory(filepath.Dir(s.uploadDir)); err != nil {
			return err
		}
	}
	if j.RestoreSite {
		info, err := os.Lstat(s.sitePath)
		if err != nil {
			return fmt.Errorf("site.yml must exist before restore: %w", err)
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0222 == 0 {
			return fmt.Errorf("site.yml target is not writable")
		}
		// 不在 config 目录创建探针：systemd 只授权既有 site.yml 文件可写。
		file, err := os.OpenFile(s.sitePath, os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	uploadSize, err := treeSize(s.uploadDir)
	if err != nil {
		return err
	}
	dbSize, err := treeSize(s.store.DBPath())
	if err != nil {
		return err
	}
	inputSize, err := treeSize(filepath.Join(s.taskRoot(j.ID), "input"))
	if err != nil {
		return err
	}
	needed := 3 * (uploadSize + dbSize + inputSize)
	if err := checkFreeSpace(s.backupDir, needed); err != nil {
		return err
	}
	if s.uploadDir != "" {
		return checkFreeSpace(filepath.Dir(s.uploadDir), needed)
	}
	return nil
}
