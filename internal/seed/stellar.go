package seed

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"blog/internal/backup"
	"blog/internal/config"
	"blog/internal/post"
	"blog/internal/store"
)

// ImportStellar 是生产启动的唯一 seed 入口。无论显式 seed 或增量发现，
// 未迁移配置/记录都报错，不通过旧 FileMeta 自动转换旧字段。
// 在私有快照预检及写入，成功才原子替换；只可在服务尚未启动时调用。
func ImportStellar(st *store.Store, renderer *post.Service, contentRoot string, missingOnly bool) (int, error) {
	if _, err := config.LoadStellarV2(st); err != nil {
		return 0, fmt.Errorf("%w: %w", store.ErrStellarMigrationRequired, err)
	}
	manifestPath := filepath.Join(contentRoot, backup.StellarContentManifestName)
	_, statError := os.Lstat(manifestPath)
	if statError != nil && !errors.Is(statError, fs.ErrNotExist) {
		return 0, statError
	}
	var manifest *backup.StellarContentManifest
	sources := map[string][]byte{}
	if statError == nil {
		value, err := backup.ReadStellarContentDirectory(contentRoot)
		if err != nil {
			return 0, err
		}
		manifest = &value
	} else {
		entries, err := os.ReadDir(filepath.Join(contentRoot, "posts"))
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		if err != nil {
			return 0, err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
				continue
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return 0, fmt.Errorf("seed Markdown source cannot be a symbolic link")
			}
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() || info.Size() > 32<<20 {
				return 0, fmt.Errorf("seed Markdown source is not regular or exceeds 32 MiB")
			}
			data, err := os.ReadFile(filepath.Join(contentRoot, "posts", entry.Name()))
			if err != nil {
				return 0, err
			}
			slug := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			sources[slug] = data
		}
		if len(sources) == 0 {
			return 0, nil
		}
	}
	directory, err := os.MkdirTemp(filepath.Dir(st.DBPath()), ".stellar-seed-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(directory)
	snapshot := filepath.Join(directory, "candidate.db")
	if err := st.BackupTo(snapshot); err != nil {
		return 0, err
	}
	probe, err := store.OpenForRecovery(snapshot)
	if err != nil {
		return 0, err
	}
	defer probe.Close()
	count := 0
	if manifest != nil {
		for _, record := range manifest.Records {
			if record.Source == "" {
				continue
			}
			if missingOnly {
				var present int
				var id any
				if record.Table == "notebooks" || record.Table == "wiki_projects" {
					var value string
					if err := json.Unmarshal(record.Values["id"], &value); err != nil {
						return 0, err
					}
					id = value
				} else {
					var value int64
					if err := json.Unmarshal(record.Values["id"], &value); err != nil {
						return 0, err
					}
					id = value
				}
				// 表名已由完整 manifest 的白名单验证。
				if err := probe.DB().QueryRow(`SELECT COUNT(*) FROM "`+record.Table+`" WHERE id=?`, id).Scan(&present); err != nil {
					return 0, err
				}
				if present > 0 {
					continue
				}
			}
			count++
		}
		if err := backup.SeedStellarContentManifest(probe, *manifest, renderer, missingOnly); err != nil {
			return 0, err
		}
	} else {
		imported, err := backup.StageStellarPosts(probe, renderer, sources, missingOnly)
		if err != nil {
			return 0, err
		}
		count = len(imported)
	}
	if count == 0 {
		return 0, nil
	}
	if err := st.RestoreFrom(snapshot); err != nil {
		return 0, err
	}
	return count, nil
}
