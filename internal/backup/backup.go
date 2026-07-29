// Package backup 数据备份与导出：SQLite 快照 + uploads 打包、文章 Markdown 导出/导入、每日自动备份留存。
// 全量恢复（整库替换）不提供在线接口：停服 → 解包覆盖 data/ → 重启。
package backup

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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
}

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

// New 创建备份服务。Keep <= 0 时默认留存 7 份。
func New(cfg Config) *Service {
	keep := cfg.Keep
	if keep <= 0 {
		keep = 7
	}
	return &Service{
		store:     cfg.Store,
		posts:     cfg.Posts,
		uploadDir: cfg.UploadDir,
		sitePath:  cfg.SitePath,
		backupDir: cfg.BackupDir,
		keep:      keep,
		auto:      cfg.Auto,
	}
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

// WritePostsZip 把全部文章（含草稿）导出为 Markdown zip，每篇 <slug>.md。
// frontmatter 字段与 post.ParseBytes 解析格式对齐，可被导入接口 / seed 重新解析。
func (s *Service) WritePostsZip(w io.Writer) error {
	posts, err := s.store.List()
	if err != nil {
		return err
	}
	zw := zip.NewWriter(w)
	for _, p := range posts {
		hdr := &zip.FileHeader{Name: p.Slug + ".md", Method: zip.Deflate, Modified: p.UpdatedAt}
		fw, err := zw.CreateHeader(hdr)
		if err != nil {
			zw.Close()
			return fmt.Errorf("zip create %s.md: %w", p.Slug, err)
		}
		if _, err := fw.Write([]byte(marshalPost(p))); err != nil {
			zw.Close()
			return fmt.Errorf("zip write %s.md: %w", p.Slug, err)
		}
	}
	return zw.Close()
}

// marshalPost 生成「frontmatter + 正文」的 Markdown 文本。只写非空字段，键名对齐轻量 parser。
func marshalPost(p *store.Post) string {
	var b strings.Builder
	b.WriteString("---\n")
	writeKV := func(key, val string) {
		if strings.TrimSpace(val) == "" {
			return
		}
		// 轻量 parser 按首个冒号切分并 trim 引号，值统一加引号防首尾空白丢失
		b.WriteString(key + ": \"" + strings.ReplaceAll(val, "\"", "'") + "\"\n")
	}
	writeList := func(key string, items []string) {
		if len(items) == 0 {
			return
		}
		b.WriteString(key + ": [" + strings.Join(items, ", ") + "]\n")
	}
	writeKV("title", p.Title)
	b.WriteString("date: " + p.CreatedAt.Format("2006-01-02 15:04:05") + "\n")
	writeKV("excerpt", p.Excerpt)
	writeKV("cover", p.Cover)
	writeKV("banner", p.Banner)
	writeKV("category", p.Category)
	writeKV("topic", p.Topic)
	if p.Tags != "" {
		writeList("tags", strings.Split(p.Tags, ","))
	}
	if poster := store.DecodePoster(p.Poster); poster != nil {
		writeKV("poster_headline", poster.Headline)
		writeKV("poster_topic", poster.Topic)
		writeKV("poster_caption", poster.Caption)
		writeKV("poster_color", poster.Color)
	}
	writeList("references", store.DecodeReferences(p.References))
	b.WriteString("status: " + p.Status + "\n")
	if p.Pinned {
		b.WriteString("pinned: true\n")
	}
	b.WriteString("---\n\n")
	b.WriteString(p.ContentMD)
	return b.String()
}

// slugPattern 导入文件名合法性：字母数字连字符下划线（防路径穿越与奇异 slug 入库）。
var slugPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// ImportResult Markdown 导入结果。
type ImportResult struct {
	Imported []string `json:"imported"` // 成功导入的 slug
	Skipped  []string `json:"skipped"`  // 跳过原因（"名称: 原因"）
}

// Import 导入 Markdown：filename 以 .zip 结尾按 zip 解包（取根层 *.md），否则视为单个 .md。
// 每篇按 slug（文件名去扩展名）UPSERT，幂等；frontmatter status: draft 按草稿导入，否则 published。
func (s *Service) Import(filename string, data []byte) (*ImportResult, error) {
	res := &ImportResult{Imported: []string{}, Skipped: []string{}}
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, fmt.Errorf("解析 zip 失败: %w", err)
		}
		for _, zf := range zr.File {
			name := zf.Name
			base := filepath.Base(name)
			if zf.FileInfo().IsDir() || !strings.HasSuffix(strings.ToLower(base), ".md") || strings.HasPrefix(base, ".") {
				continue
			}
			rc, err := zf.Open()
			if err != nil {
				res.Skipped = append(res.Skipped, base+": 读取失败")
				continue
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				res.Skipped = append(res.Skipped, base+": 读取失败")
				continue
			}
			s.importOne(base, content, res)
		}
	case strings.HasSuffix(lower, ".md"):
		s.importOne(filepath.Base(filename), data, res)
	default:
		return nil, fmt.Errorf("不支持的文件类型: %s（仅 .md / .zip）", filename)
	}
	return res, nil
}

// importOne 导入单篇：解析 frontmatter → 渲染 → Upsert。失败记入 skipped。
func (s *Service) importOne(base string, data []byte, res *ImportResult) {
	slug := strings.TrimSuffix(base, filepath.Ext(base))
	if !slugPattern.MatchString(slug) {
		res.Skipped = append(res.Skipped, base+": 文件名不是合法 slug")
		return
	}
	meta, body := s.posts.ParseBytes(data)
	html, err := s.posts.Render(body)
	if err != nil {
		res.Skipped = append(res.Skipped, base+": 渲染失败")
		return
	}
	title := meta.Title
	if title == "" {
		title = slug
	}
	status := store.StatusPublished
	if meta.Status == store.StatusDraft {
		status = store.StatusDraft
	}
	p := &store.Post{
		Slug:        slug,
		Title:       title,
		Excerpt:     meta.Excerpt,
		Cover:       meta.Cover,
		Banner:      meta.Banner,
		Category:    meta.Category,
		Topic:       meta.Topic,
		Tags:        strings.Join(meta.Tags, ","),
		ContentMD:   body,
		ContentHTML: html,
		Status:      status,
		Pinned:      meta.Pinned,
		CreatedAt:   parseDate(meta.Date),
		Poster: store.EncodePoster(store.PosterData{
			Headline: meta.PosterHeadline,
			Topic:    meta.PosterTopic,
			Caption:  meta.PosterCaption,
			Color:    meta.PosterColor,
		}),
		References: store.EncodeReferences(meta.References),
	}
	if err := s.store.Upsert(p); err != nil {
		res.Skipped = append(res.Skipped, base+": 写入失败")
		return
	}
	res.Imported = append(res.Imported, slug)
}

// parseDate 解析 frontmatter date（与 seed 同一组 layout），失败用当前时间。
func parseDate(v string) time.Time {
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, v); err == nil {
			return t
		}
	}
	return time.Now()
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
	go func() {
		s.runAutoBackup()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			s.runAutoBackup()
		}
	}()
}

// runAutoBackup 生成一份全量备份到 backupDir 并按 keep 清理旧份。失败只记日志不中断服务。
func (s *Service) runAutoBackup() {
	if err := os.MkdirAll(s.backupDir, 0o755); err != nil {
		log.Printf("auto backup: mkdir %s: %v", s.backupDir, err)
		return
	}
	name := "blog-backup-" + time.Now().Format("20060102-150405") + ".zip"
	path := filepath.Join(s.backupDir, name)
	f, err := os.Create(path)
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
