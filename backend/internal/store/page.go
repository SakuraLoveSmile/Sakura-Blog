package store

import (
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

// TableName Entity 接口：pages 表。
func (p *Page) TableName() string { return "pages" }

// Columns Entity 接口：全列，首列为 id。
func (p *Page) Columns() []string {
	return []string{"id", "slug", "title", "content_md", "content_html", "status", "created_at", "updated_at", "deleted_at"}
}

// Values Entity 接口：写入值（不含 id，与 Columns()[1:] 对应）。
func (p *Page) Values() []any {
	return []any{p.Slug, p.Title, p.ContentMD, p.ContentHTML, p.Status, p.CreatedAt, p.UpdatedAt, p.DeletedAt}
}

// SetID Entity 接口：回填自增 id。
func (p *Page) SetID(id int64) { p.ID = id }

// ScanRow Entity 接口：行扫描，顺序对齐 Columns()。
func (p *Page) ScanRow(row rowScanner) error {
	return row.Scan(&p.ID, &p.Slug, &p.Title, &p.ContentMD, &p.ContentHTML, &p.Status, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt)
}

// EnsurePagesTable 独立建 pages 表（统一走 schema.go 注册表）。
// 幂等迁移：旧库可能缺 deleted_at 列（ADD COLUMN 已存在会报错，忽略即幂等）。
func (s *Store) EnsurePagesTable() error {
	if err := s.ensureTable("pages"); err != nil {
		return err
	}
	s.db.Exec(`ALTER TABLE pages ADD COLUMN deleted_at TEXT NOT NULL DEFAULT ''`)
	return nil
}

// ListPages 列出全部未删除页面（按更新时间倒序）。
func (s *Store) ListPages() ([]*Page, error) {
	return repo(s, &Page{}).List(Where("deleted_at = ''"), OrderBy("substr(updated_at, 1, 19) DESC, id DESC"))
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
	where := "slug = ? AND deleted_at = ''"
	args := []any{slug}
	if publishedOnly {
		where += " AND status = ?"
		args = append(args, StatusPublished)
	}
	p, err := repo(s, &Page{}).GetOne(where, args...)
	if err != nil {
		return nil, fmt.Errorf("get page by slug %s: %w", slug, err)
	}
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
	id, err := repo(s, &Page{}).Create(p)
	if err != nil {
		return 0, fmt.Errorf("create page %s: %w", p.Slug, err)
	}
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
	return repo(s, &Page{}).List(Where("deleted_at != ''"), OrderBy("datetime(deleted_at) DESC"))
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
