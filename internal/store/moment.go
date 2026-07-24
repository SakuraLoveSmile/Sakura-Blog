package store

import (
	"database/sql"
	"fmt"
	"time"
)

// Moment 瞬间/说说持久化记录，对齐 moments 表字段。
type Moment struct {
	ID          int64
	ContentMD   string
	ContentHTML string
	Images      string // 逗号分隔的图片 URL
	CreatedAt   time.Time
}

const momentCols = `id, content_md, content_html, images, created_at`

// EnsureMomentsTable 独立建 moments 表（不走 post.go createTables，避免与其它 spec 冲突）。
func (s *Store) EnsureMomentsTable() error {
	const ddl = `CREATE TABLE IF NOT EXISTS moments (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		content_md   TEXT NOT NULL DEFAULT '',
		content_html TEXT NOT NULL DEFAULT '',
		images       TEXT NOT NULL DEFAULT '',
		created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_moments_created ON moments(created_at);`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create moments table: %w", err)
	}
	return nil
}

// CreateMoment 新建瞬间，返回 id 并回填 m.ID。
func (s *Store) CreateMoment(m *Moment) (int64, error) {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	res, err := s.db.Exec(`INSERT INTO moments (content_md, content_html, images, created_at)
		VALUES (?, ?, ?, ?)`,
		m.ContentMD, m.ContentHTML, m.Images, m.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("create moment: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	m.ID = id
	return id, nil
}

// ListMoments 分页列出瞬间（按创建时间倒序）。page 从 1 开始。
func (s *Store) ListMoments(page, size int) ([]*Moment, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	offset := (page - 1) * size
	rows, err := s.db.Query(`SELECT `+momentCols+` FROM moments
		ORDER BY substr(created_at, 1, 19) DESC, id DESC LIMIT ? OFFSET ?`, size, offset)
	if err != nil {
		return nil, fmt.Errorf("list moments: %w", err)
	}
	defer rows.Close()
	return scanMoments(rows)
}

// CountMoments 返回瞬间总数。
func (s *Store) CountMoments() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM moments`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count moments: %w", err)
	}
	return n, nil
}

// DeleteMoment 按 id 删除记录。
func (s *Store) DeleteMoment(id int64) error {
	res, err := s.db.Exec(`DELETE FROM moments WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete moment %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("delete moment %d: no rows affected", id)
	}
	return nil
}

func scanMoments(rows *sql.Rows) ([]*Moment, error) {
	var out []*Moment
	for rows.Next() {
		m := &Moment{}
		if err := rows.Scan(&m.ID, &m.ContentMD, &m.ContentHTML, &m.Images, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan moment: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate moments: %w", err)
	}
	return out, nil
}
