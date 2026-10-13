// Package backup 数据备份与导出：SQLite 快照 + uploads 打包、文章 Markdown 导出/导入、每日自动备份留存。
// 全量恢复见 restore.go（Spec 41）：zip 包/本地快照在线一键还原，恢复前自动兜底快照。
package backup

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"blog/internal/post"
	"blog/internal/store"
)

// Service 备份服务，持有 store（DB 快照/文章数据）、post service（Markdown 解析渲染）与路径配置。
type Service struct {
	store     *store.Store
	posts     *post.Service
	uploadDir string // 附件根目录，打包进 zip 的 uploads/ 前缀下
	sitePath  string // config/site.yml 绝对路径，存在时一并打包
	backupDir string // 自动备份输出目录（data/backups）
	keep      int    // 自动备份留存份数
	auto      bool   // 自动备份是否开启（status API 展示用）
	// restoreHook 恢复完成回调（Spec 41）：main.go 装配 FTS 重建/配置热更新/缓存清理。
	restoreHook         func()
	restoreRebuild      func() error
	operationLock       func() (func(), error)
	backupLock          func() (func(), error)
	restoreMu           sync.Mutex
	restoreStep         func(string) error
	uncertainTask       string
	restoreJournalWrite func(string, []byte) error
	activeRestore       string
	startOnce           sync.Once
	closeOnce           sync.Once
	stop                chan struct{}
	done                chan struct{}
}

// SetBackupLock lets application composition wire automatic backups to the
// same maintenance coordinator used by restores and updates.
func (s *Service) SetBackupLock(fn func() (func(), error)) { s.backupLock = fn }

// Config 备份服务初始化参数。
type Config struct {
	Store     *store.Store
	Posts     *post.Service
	UploadDir string
	SitePath  string
	BackupDir string
	Keep      int
	Auto      bool
}

// snapshotMaxAge WriteZip 临时快照文件的最长留存：正常备份分钟级结束即被 defer 删除，
// 活过该阈值必为进程中断（kill/crash）遗留的孤儿临时文件。
const snapshotMaxAge = 24 * time.Hour

// New 创建备份服务。Keep <= 0 时默认留存 7 份。
// 装配时顺带清理 backupDir 下陈旧的 .snapshot-*.db 孤儿临时文件
// （WriteZip 的 VACUUM INTO 目标，进程中断时 defer 清理不会执行）。
func New(cfg Config) *Service {
	keep := cfg.Keep
	if keep <= 0 {
		keep = 7
	}
	if n := cleanStaleSnapshots(cfg.BackupDir, snapshotMaxAge); n > 0 {
		log.Printf("backup: cleaned %d stale snapshot temp file(s) in %s", n, cfg.BackupDir)
	}
	return &Service{
		store:     cfg.Store,
		posts:     cfg.Posts,
		uploadDir: cfg.UploadDir,
		sitePath:  cfg.SitePath,
		backupDir: cfg.BackupDir,
		keep:      keep,
		auto:      cfg.Auto,
		stop:      make(chan struct{}),
	}
}

// cleanStaleSnapshots 清理 backupDir 下超过 maxAge 的 .snapshot-*.db 孤儿临时文件，
// 返回删除数量。目录不可读或单文件删除失败均静默跳过（清理尽力而为，不影响主流程）。
func cleanStaleSnapshots(backupDir string, maxAge time.Duration) int {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return 0
	}
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, ".snapshot-") || !strings.HasSuffix(name, ".db") {
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		if os.Remove(filepath.Join(backupDir, name)) == nil {
			removed++
		}
	}
	return removed
}

// WriteZip 生成全量备份 zip 写入 w：blog.db（VACUUM INTO 一致性快照）+ uploads/ + site.yml。
func (s *Service) WriteZip(w io.Writer) error {
	// 1. DB 快照到临时文件。放 backupDir（与 DB 同卷的 data 目录下）而非系统 TempDir，
	// 避免受限环境下 TempDir 不可写；VACUUM INTO 要求目标不存在，用纳秒时间戳避免撞名。
	if err := os.MkdirAll(s.backupDir, 0o755); err != nil {
		return fmt.Errorf("mkdir backup dir: %w", err)
	}
	tmp := filepath.Join(s.backupDir, fmt.Sprintf(".snapshot-%d.db", time.Now().UnixNano()))
	if err := s.store.BackupTo(tmp); err != nil {
		return err
	}
	defer os.Remove(tmp)

	zw := zip.NewWriter(w)

	// 2. blog.db
	if err := s.addFile(zw, tmp, "blog.db"); err != nil {
		zw.Close()
		return err
	}

	// 3. uploads/ 全目录（目录不存在时跳过，零附件站点也能备份）
	if info, err := os.Stat(s.uploadDir); err == nil && info.IsDir() {
		err := filepath.WalkDir(s.uploadDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(s.uploadDir, path)
			if err != nil {
				return err
			}
			return s.addFile(zw, path, "uploads/"+filepath.ToSlash(rel))
		})
		if err != nil {
			zw.Close()
			return fmt.Errorf("walk uploads: %w", err)
		}
	}

	// 4. site.yml（存在才打包；settings 表已在 db 内，这里是文件级冗余）
	if s.sitePath != "" {
		if _, err := os.Stat(s.sitePath); err == nil {
			if err := s.addFile(zw, s.sitePath, "site.yml"); err != nil {
				zw.Close()
				return err
			}
		}
	}

	return zw.Close()
}

// addFile 把磁盘文件写入 zip（Deflate 压缩，保留修改时间）。
func (s *Service) addFile(zw *zip.Writer, srcPath, zipName string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", srcPath, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", srcPath, err)
	}
	hdr := &zip.FileHeader{Name: zipName, Method: zip.Deflate, Modified: info.ModTime()}
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return fmt.Errorf("zip create %s: %w", zipName, err)
	}
	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("zip write %s: %w", zipName, err)
	}
	return nil
}

// WritePostsZip 导出完整 Stellar v2 内容包（兼容已有下载入口名称）。
// 旧记录必须先经过迁移预览，不能再导出一份自动转换的旧 front matter。
func (s *Service) WritePostsZip(w io.Writer) error {
	return s.WriteStellarContentZip(w)
}

// zip 导入防 zip-bomb（Spec 61 阶段 3）：条目数上限 + 单条解压后大小上限。
// 超出即整体拒绝，避免解压放大导致内存/磁盘耗尽。
const (
	maxZipEntries   = 500      // zip 内条目数上限（正常导入远小于此）
	maxZipEntrySize = 32 << 20 // 单条解压后上限 32MB（markdown 正文远小于此）
)

// ImportResult Markdown 导入结果。
type ImportResult struct {
	Imported []string `json:"imported"` // 成功导入的 slug
	Skipped  []string `json:"skipped"`  // 跳过原因（"名称: 原因"）
}

// Import 严格导入单篇 Markdown、文章 ZIP 或完整 manifest 内容包。
// 整批预检、隔离构建，再复用恢复维护窗口原子应用；发布状态用 blog.status。
func (s *Service) Import(filename string, data []byte) (*ImportResult, error) {
	return s.ImportContext(context.Background(), filename, data)
}

func (s *Service) ImportContext(ctx context.Context, filename string, data []byte) (*ImportResult, error) {
	if strings.HasSuffix(strings.ToLower(filename), ".zip") && hasStellarContentManifest(data) {
		return s.importStellarContent(ctx, data)
	}
	return s.importStellarPosts(ctx, filename, data)
}

// Info 备份文件信息（status API 展示用）。
type Info struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

// AutoEnabled / Keep 自动备份配置只读访问（status API 用）。
func (s *Service) AutoEnabled() bool { return s.auto }
func (s *Service) Keep() int         { return s.keep }

// ListBackups 列出 backupDir 下的备份 zip，按文件名倒序（时间戳命名，新的在前）。
func (s *Service) ListBackups() ([]Info, error) {
	entries, err := os.ReadDir(s.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Info{}, nil
		}
		return nil, fmt.Errorf("read backup dir: %w", err)
	}
	var out []Info
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".zip") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, Info{Name: e.Name(), Size: info.Size(), ModTime: info.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	if out == nil {
		out = []Info{}
	}
	return out, nil
}

// StartAuto 启动自动备份 goroutine：启动先做一次（保证至少一份），之后每 interval 一次。
func (s *Service) StartAuto(interval time.Duration) {
	s.startOnce.Do(func() {
		s.done = make(chan struct{})
		go func() {
			defer close(s.done)
			select {
			case <-s.stop:
				return
			default:
			}
			s.runAutoBackup()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-s.stop:
					return
				case <-ticker.C:
					s.runAutoBackup()
				}
			}
		}()
	})
}

// Close stops the automatic backup loop before its Store is closed.
func (s *Service) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.closeOnce.Do(func() { close(s.stop) })
	if s.done != nil {
		select {
		case <-s.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		s.restoreMu.Lock()
		running := s.activeRestore != ""
		s.restoreMu.Unlock()
		if !running {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// runAutoBackup 生成一份全量备份到 backupDir 并按 keep 清理旧份。失败只记日志不中断服务。
func (s *Service) runAutoBackup() {
	if s.backupLock == nil && s.store != nil {
		s.backupLock = s.store.Maintenance().Acquire
	}
	if s.backupLock != nil {
		release, err := s.backupLock()
		if err != nil {
			log.Printf("auto backup: maintenance busy: %v", err)
			return
		}
		defer release()
	}
	if err := os.MkdirAll(s.backupDir, 0o755); err != nil {
		log.Printf("auto backup: mkdir %s: %v", s.backupDir, err)
		return
	}
	name := "blog-backup-" + time.Now().Format("20060102-150405") + ".zip"
	path := filepath.Join(s.backupDir, name)
	// 安全审计 2026-08-16：备份 zip 含全库凭据（bcrypt/auth_secret/TOTP 种子/PAT 哈希/
	// 明文文章口令/site.yml），统一 0600 落盘（此前 os.Create 默认 0644，同机其他用户可读）。
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		log.Printf("auto backup: create %s: %v", path, err)
		return
	}
	if err := s.WriteZip(f); err != nil {
		f.Close()
		os.Remove(path)
		log.Printf("auto backup: write %s: %v", name, err)
		return
	}
	f.Close()
	log.Printf("auto backup: %s written", name)
	s.prune()
}

// prune 按留存份数清理旧备份（文件名时间戳排序，删最旧的）。
func (s *Service) prune() {
	backups, err := s.ListBackups()
	if err != nil {
		log.Printf("auto backup: prune list: %v", err)
		return
	}
	for i := s.keep; i < len(backups); i++ {
		path := filepath.Join(s.backupDir, backups[i].Name)
		if err := os.Remove(path); err != nil {
			log.Printf("auto backup: prune %s: %v", backups[i].Name, err)
		}
	}
}
