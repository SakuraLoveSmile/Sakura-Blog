package store

import (
	"database/sql"
	"fmt"
	"time"
)

// Attachment 附件持久化记录，对齐 attachments 表字段。
type Attachment struct {
	ID        int64
	Filename  string // 原始文件名
	Path      string // 相对 upload_dir 的路径（如 2026/07/xxx.png）
	URL       string // 对外 URL（如 /uploads/2026/07/xxx.png）
	Mime      string
	Size      int64
	CreatedAt time.Time
}

const attachmentCols = `id, filename, path, url, mime, size, created_at`

// EnsureAttachmentsTable 独立建 attachments 表（不走 post.go createTables，避免与其它 spec 冲突）。
func (s *Store) EnsureAttachmentsTable() error {
	const ddl = `CREATE TABLE IF NOT EXISTS attachments (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		filename   TEXT NOT NULL DEFAULT '',
		path       TEXT NOT NULL DEFAULT '',
		url        TEXT NOT NULL DEFAULT '',
		mime       TEXT NOT NULL DEFAULT '',
		size       INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_attachments_created ON attachments(created_at);`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create attachments table: %w", err)
	}
	return nil
}

// CreateAttachment 新建附件记录，返回 id 并回填 a.ID。
func (s *Store) CreateAttachment(a *Attachment) (int64, error) {
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now()
	}
	res, err := s.db.Exec(`INSERT INTO attachments (filename, path, url, mime, size, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		a.Filename, a.Path, a.URL, a.Mime, a.Size, a.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("create attachment: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	a.ID = id
	return id, nil
}

// ListAttachments 分页列出附件（按创建时间倒序）。page 从 1 开始。
func (s *Store) ListAttachments(page, size int) ([]*Attachment, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 24
	}
	offset := (page - 1) * size
	rows, err := s.db.Query(`SELECT `+attachmentCols+` FROM attachments
		ORDER BY datetime(created_at) DESC, id DESC LIMIT ? OFFSET ?`, size, offset)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	defer rows.Close()
	return scanAttachments(rows)
}

// CountAttachments 返回附件总数。
func (s *Store) CountAttachments() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM attachments`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count attachments: %w", err)
	}
	return n, nil
}

// GetAttachment 按 id 查单条。未找到返回 nil, nil。
func (s *Store) GetAttachment(id int64) (*Attachment, error) {
	row := s.db.QueryRow(`SELECT `+attachmentCols+` FROM attachments WHERE id = ?`, id)
	a := &Attachment{}
	err := row.Scan(&a.ID, &a.Filename, &a.Path, &a.URL, &a.Mime, &a.Size, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get attachment %d: %w", id, err)
	}
	return a, nil
}

// DeleteAttachment 按 id 删除记录。
func (s *Store) DeleteAttachment(id int64) error {
	res, err := s.db.Exec(`DELETE FROM attachments WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete attachment %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("delete attachment %d: no rows affected", id)
	}
	return nil
}

func scanAttachments(rows *sql.Rows) ([]*Attachment, error) {
	var out []*Attachment
	for rows.Next() {
		a := &Attachment{}
		if err := rows.Scan(&a.ID, &a.Filename, &a.Path, &a.URL, &a.Mime, &a.Size, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attachments: %w", err)
	}
	return out, nil
}
