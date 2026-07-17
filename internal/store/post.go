// Package store 文章持久化层，基于 SQLite（modernc.org/sqlite 纯 Go 驱动，免 CGO）。
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // side-effect import，注册 "sqlite" driver 到 database/sql
)

// Status 文章状态。
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
)

// Post 文章持久化记录。对齐 posts 表字段。
type Post struct {
	ID          int64
	Slug        string
	Title       string
	Excerpt     string
	Cover       string // 封面图 URL，可空
	ContentMD   string // Markdown 原文
	ContentHTML string // goldmark 预渲染缓存
	Status      string // draft / published
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Store 文章数据访问对象，持 *sql.DB 连接。
type Store struct {
	db *sql.DB
}

// New 打开 SQLite 连接并建表。dbPath 是 .db 文件绝对路径，父目录自动创建。
// SQLite 写并发受限，SetMaxOpenConns(1) 避免 "database is locked"。
func New(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir db dir %s: %w", dir, err)
	}

	// 用 file: DSN 启用 WAL 与忙等待，提升并发读 + 避免短暂锁冲突
	dsn := "file:" + dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", dbPath, err)
	}
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &Store{db: db}
	if err := s.createTables(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close 关闭底层数据库连接。
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) createTables() error {
	const ddl = `CREATE TABLE IF NOT EXISTS posts (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		slug         TEXT NOT NULL UNIQUE,
		title        TEXT NOT NULL DEFAULT '',
		excerpt      TEXT NOT NULL DEFAULT '',
		cover        TEXT NOT NULL DEFAULT '',
		content_md   TEXT NOT NULL DEFAULT '',
		content_html TEXT NOT NULL DEFAULT '',
		status       TEXT NOT NULL DEFAULT 'draft',
		created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_posts_status ON posts(status);
	CREATE INDEX IF NOT EXISTS idx_posts_slug ON posts(slug);`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	// 老库迁移：CREATE TABLE IF NOT EXISTS 不会给已存在的表加列，需检测后 ALTER。
	return s.migrateCoverColumn()
}

// migrateCoverColumn 检测 posts 表无 cover 列时 ALTER TABLE 补上（兼容 Demo 3 老库）。
func (s *Store) migrateCoverColumn() error {
	rows, err := s.db.Query(`PRAGMA table_info(posts)`)
	if err != nil {
		return fmt.Errorf("pragma table_info: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("scan table_info: %w", err)
		}
		if name == "cover" {
			return nil // 列已存在，无需迁移
		}
	}
	if _, err := s.db.Exec(`ALTER TABLE posts ADD COLUMN cover TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("alter table add cover: %w", err)
	}
	return nil
}

// List 返回全部文章，按 created_at 倒序（最新在前）。
func (s *Store) List() ([]*Post, error) {
	rows, err := s.db.Query(`SELECT id, slug, title, excerpt, cover, content_md, content_html, status, created_at, updated_at
		FROM posts ORDER BY datetime(created_at) DESC`)
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}
	defer rows.Close()
	return scanPosts(rows)
}

// ListPublished 返回已发布文章，按 created_at 倒序。
func (s *Store) ListPublished() ([]*Post, error) {
	rows, err := s.db.Query(`SELECT id, slug, title, excerpt, cover, content_md, content_html, status, created_at, updated_at
		FROM posts WHERE status = ? ORDER BY datetime(created_at) DESC`, StatusPublished)
	if err != nil {
		return nil, fmt.Errorf("list published posts: %w", err)
	}
	defer rows.Close()
	return scanPosts(rows)
}

// ListPublishedPaged 分页返回已发布文章，按 created_at 倒序。page 从 1 开始。
func (s *Store) ListPublishedPaged(page, size int) ([]*Post, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	rows, err := s.db.Query(`SELECT id, slug, title, excerpt, cover, content_md, content_html, status, created_at, updated_at
		FROM posts WHERE status = ? ORDER BY datetime(created_at) DESC LIMIT ? OFFSET ?`,
		StatusPublished, size, (page-1)*size)
	if err != nil {
		return nil, fmt.Errorf("list published posts paged: %w", err)
	}
	defer rows.Close()
	return scanPosts(rows)
}

// CountPublished 返回已发布文章总数（分页器用）。
func (s *Store) CountPublished() (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM posts WHERE status = ?`, StatusPublished).Scan(&n); err != nil {
		return 0, fmt.Errorf("count published posts: %w", err)
	}
	return n, nil
}

// GetBySlug 按 slug 查单篇（任意状态）。未找到返回 nil, nil。
func (s *Store) GetBySlug(slug string) (*Post, error) {
	return s.getBySlug(slug, false)
}

// GetPublishedBySlug 按 slug 查已发布文章。未找到返回 nil, nil。
func (s *Store) GetPublishedBySlug(slug string) (*Post, error) {
	return s.getBySlug(slug, true)
}

func (s *Store) getBySlug(slug string, publishedOnly bool) (*Post, error) {
	q := `SELECT id, slug, title, excerpt, cover, content_md, content_html, status, created_at, updated_at FROM posts WHERE slug = ?`
	args := []interface{}{slug}
	if publishedOnly {
		q += ` AND status = ?`
		args = append(args, StatusPublished)
	}
	row := s.db.QueryRow(q, args...)
	p := &Post{}
	err := row.Scan(&p.ID, &p.Slug, &p.Title, &p.Excerpt, &p.Cover, &p.ContentMD, &p.ContentHTML, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get post by slug %s: %w", slug, err)
	}
	return p, nil
}

// Create 新建文章，返回新记录 id。slug 冲突时返回错误。
func (s *Store) Create(p *Post) (int64, error) {
	if p.Status == "" {
		p.Status = StatusDraft
	}
	now := time.Now()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	res, err := s.db.Exec(`INSERT INTO posts (slug, title, excerpt, cover, content_md, content_html, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Slug, p.Title, p.Excerpt, p.Cover, p.ContentMD, p.ContentHTML, p.Status, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return 0, fmt.Errorf("create post %s: %w", p.Slug, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	p.ID = id
	return id, nil
}

// Update 按 slug 更新文章字段（title/excerpt/cover/content_md/content_html/status），刷新 updated_at。
func (s *Store) Update(slug string, p *Post) error {
	p.UpdatedAt = time.Now()
	res, err := s.db.Exec(`UPDATE posts SET title=?, excerpt=?, cover=?, content_md=?, content_html=?, status=?, updated_at=?
		WHERE slug=?`,
		p.Title, p.Excerpt, p.Cover, p.ContentMD, p.ContentHTML, p.Status, p.UpdatedAt, slug)
	if err != nil {
		return fmt.Errorf("update post %s: %w", slug, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("update post %s: no rows affected", slug)
	}
	return nil
}

// Delete 按 slug 删除文章。不存在不报错。
func (s *Store) Delete(slug string) error {
	_, err := s.db.Exec(`DELETE FROM posts WHERE slug=?`, slug)
	if err != nil {
		return fmt.Errorf("delete post %s: %w", slug, err)
	}
	return nil
}

// Upsert 按 slug 存在性决定 Create 或 Update，seed 导入用。content_html 由调用方渲染好传入。
func (s *Store) Upsert(p *Post) error {
	existing, err := s.GetBySlug(p.Slug)
	if err != nil {
		return err
	}
	if existing == nil {
		_, err = s.Create(p)
		return err
	}
	// 保留原 created_at，仅更新内容字段
	p.ID = existing.ID
	p.CreatedAt = existing.CreatedAt
	return s.Update(p.Slug, p)
}

// scanPosts 批量扫描 rows 到 []*Post。
func scanPosts(rows *sql.Rows) ([]*Post, error) {
	var out []*Post
	for rows.Next() {
		p := &Post{}
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Excerpt, &p.Cover, &p.ContentMD, &p.ContentHTML, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan post: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate posts: %w", err)
	}
	return out, nil
}
