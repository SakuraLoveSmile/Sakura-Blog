package store

import (
	"database/sql"
	"fmt"
	"time"
)

// Page 独立页面持久化记录，对齐 pages 表字段。
type Page struct {
	ID          int64
	Slug        string
	Title       string
	ContentMD   string
	ContentHTML string
	Status      string // draft / published
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   string // 软删除时间 RFC3339，空串=未删除
}

const pageCols = `id, slug, title, content_md, content_html, status, created_at, updated_at, deleted_at`

// EnsurePagesTable 独立建 pages 表（不走 post.go createTables，避免与其它 spec 冲突）。
func (s *Store) EnsurePagesTable() error {
	const ddl = `CREATE TABLE IF NOT EXISTS pages (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		slug         TEXT NOT NULL UNIQUE,
		title        TEXT NOT NULL DEFAULT '',
		content_md   TEXT NOT NULL DEFAULT '',
		content_html TEXT NOT NULL DEFAULT '',
		status       TEXT NOT NULL DEFAULT 'draft',
		created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		deleted_at   TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_pages_status ON pages(status);
	CREATE INDEX IF NOT EXISTS idx_pages_created ON pages(created_at);`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create pages table: %w", err)
	}
	// 幂等迁移：旧库可能缺 deleted_at 列
	s.db.Exec(`ALTER TABLE pages ADD COLUMN deleted_at TEXT NOT NULL DEFAULT ''`)
	return nil
}

// ListPages 列出全部未删除页面（按更新时间倒序）。
func (s *Store) ListPages() ([]*Page, error) {
	rows, err := s.db.Query(`SELECT ` + pageCols + ` FROM pages WHERE deleted_at = '' ORDER BY substr(updated_at, 1, 19) DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list pages: %w", err)
	}
	defer rows.Close()
	return scanPages(rows)
}

// GetPageBySlug 按 slug 查单页（任意状态）。未找到返回 nil, nil。
func (s *Store) GetPageBySlug(slug string) (*Page, error) {
	return s.getPageBySlug(slug, false)
}

// GetPublished 按 slug 查已发布页面。未找到返回 nil, nil。
func (s *Store) GetPublished(slug string) (*Page, error) {
	return s.getPageBySlug(slug, true)
}

func (s *Store) getPageBySlug(slug string, publishedOnly bool) (*Page, error) {
	q := `SELECT ` + pageCols + ` FROM pages WHERE slug = ? AND deleted_at = ''`
	args := []interface{}{slug}
	if publishedOnly {
		q += ` AND status = ?`
		args = append(args, StatusPublished)
	}
	row := s.db.QueryRow(q, args...)
	p := &Page{}
	var deletedAt string
	err := row.Scan(&p.ID, &p.Slug, &p.Title, &p.ContentMD, &p.ContentHTML, &p.Status, &p.CreatedAt, &p.UpdatedAt, &deletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get page by slug %s: %w", slug, err)
	}
	p.DeletedAt = deletedAt
	return p, nil
}

// CreatePage 新建页面，返回新记录 id。slug 冲突时返回错误。
func (s *Store) CreatePage(p *Page) (int64, error) {
	if p.Status == "" {
		p.Status = StatusDraft
	}
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	res, err := s.db.Exec(`INSERT INTO pages (slug, title, content_md, content_html, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.Slug, p.Title, p.ContentMD, p.ContentHTML, p.Status, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return 0, fmt.Errorf("create page %s: %w", p.Slug, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	p.ID = id
	return id, nil
}

// UpdatePage 按 slug 更新页面字段，刷新 updated_at。
func (s *Store) UpdatePage(slug string, p *Page) error {
	p.UpdatedAt = time.Now().UTC()
	res, err := s.db.Exec(`UPDATE pages SET slug=?, title=?, content_md=?, content_html=?, status=?, updated_at=?
		WHERE slug=?`,
		p.Slug, p.Title, p.ContentMD, p.ContentHTML, p.Status, p.UpdatedAt, slug)
	if err != nil {
		return fmt.Errorf("update page %s: %w", slug, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("update page %s: no rows affected", slug)
	}
	return nil
}

// DeletePage 软删除：标记 deleted_at，页面进入回收站。不存在不报错。
func (s *Store) DeletePage(slug string) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	_, err := s.db.Exec(`UPDATE pages SET deleted_at = ? WHERE slug = ? AND deleted_at = ''`, now, slug)
	if err != nil {
		return fmt.Errorf("soft delete page %s: %w", slug, err)
	}
	return nil
}

// ListTrashedPages 返回回收站中的页面，按删除时间倒序。
func (s *Store) ListTrashedPages() ([]*Page, error) {
	rows, err := s.db.Query(`SELECT ` + pageCols + ` FROM pages WHERE deleted_at != '' ORDER BY datetime(deleted_at) DESC`)
	if err != nil {
		return nil, fmt.Errorf("list trashed pages: %w", err)
	}
	defer rows.Close()
	return scanPages(rows)
}

// RestorePage 从回收站恢复页面。
func (s *Store) RestorePage(slug string) error {
	_, err := s.db.Exec(`UPDATE pages SET deleted_at = '' WHERE slug = ? AND deleted_at != ''`, slug)
	if err != nil {
		return fmt.Errorf("restore page %s: %w", slug, err)
	}
	return nil
}

// PurgePage 彻底删除页面（物理删除）。
func (s *Store) PurgePage(slug string) error {
	_, err := s.db.Exec(`DELETE FROM pages WHERE slug = ? AND deleted_at != ''`, slug)
	if err != nil {
		return fmt.Errorf("purge page %s: %w", slug, err)
	}
	return nil
}

func scanPages(rows *sql.Rows) ([]*Page, error) {
	var out []*Page
	for rows.Next() {
		p := &Page{}
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &p.ContentMD, &p.ContentHTML, &p.Status, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt); err != nil {
			return nil, fmt.Errorf("scan page: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pages: %w", err)
	}
	return out, nil
}
