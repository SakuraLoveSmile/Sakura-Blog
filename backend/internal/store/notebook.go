package store

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Notebook 笔记本。
type Notebook struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Cover       string    `json:"cover"`
	Order       int       `json:"order"`
	CreatedAt   time.Time `json:"created_at"`
}

// Note 笔记。
type Note struct {
	ID          int64     `json:"id"`
	NotebookID  int64     `json:"notebook_id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`      // Markdown 原文
	ContentHTML string    `json:"content_html"` // 渲染后 HTML
	Tags        string    `json:"tags"`         // 逗号分隔的标签字符串
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// notebookCols / noteCols 统一 SELECT 列清单。排序列用 nb_order 而非 order（SQL 保留字）。
const (
	notebookCols = "id, name, description, cover, nb_order, created_at"
	noteCols     = "id, notebook_id, title, content, content_html, tags, created_at, updated_at"
)

// EnsureNotebooksTable 幂等建 notebooks 表（不走 createTables，由调用方启动时按需调用）。
func (s *Store) EnsureNotebooksTable() error {
	const ddl = `CREATE TABLE IF NOT EXISTS notebooks (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT NOT NULL UNIQUE,
		description TEXT NOT NULL DEFAULT '',
		cover       TEXT NOT NULL DEFAULT '',
		nb_order    INTEGER NOT NULL DEFAULT 0,
		created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create notebooks table: %w", err)
	}
	return nil
}

// EnsureNotesTable 幂等建 notes 表。
func (s *Store) EnsureNotesTable() error {
	const ddl = `CREATE TABLE IF NOT EXISTS notes (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		notebook_id  INTEGER NOT NULL,
		title        TEXT NOT NULL DEFAULT '',
		content      TEXT NOT NULL DEFAULT '',
		content_html TEXT NOT NULL DEFAULT '',
		tags         TEXT NOT NULL DEFAULT '',
		created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (notebook_id) REFERENCES notebooks(id)
	);
	CREATE INDEX IF NOT EXISTS idx_notes_notebook ON notes(notebook_id);`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create notes table: %w", err)
	}
	return nil
}

// ListNotebooks 列出全部笔记本，按 nb_order, id 升序。
func (s *Store) ListNotebooks() ([]*Notebook, error) {
	rows, err := s.db.Query(`SELECT ` + notebookCols + ` FROM notebooks ORDER BY nb_order, id`)
	if err != nil {
		return nil, fmt.Errorf("list notebooks: %w", err)
	}
	defer rows.Close()
	var out []*Notebook
	for rows.Next() {
		nb, err := scanNotebook(rows)
		if err != nil {
			return nil, fmt.Errorf("scan notebook: %w", err)
		}
		out = append(out, nb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notebooks: %w", err)
	}
	return out, nil
}

// GetNotebook 按 id 查单个笔记本。未找到返回 nil, nil。
func (s *Store) GetNotebook(id int64) (*Notebook, error) {
	row := s.db.QueryRow(`SELECT `+notebookCols+` FROM notebooks WHERE id = ?`, id)
	nb, err := scanNotebook(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get notebook %d: %w", id, err)
	}
	return nb, nil
}

// GetNotebookByName 按 name 查单个笔记本。未找到返回 nil, nil。
func (s *Store) GetNotebookByName(name string) (*Notebook, error) {
	row := s.db.QueryRow(`SELECT `+notebookCols+` FROM notebooks WHERE name = ?`, name)
	nb, err := scanNotebook(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get notebook %q: %w", name, err)
	}
	return nb, nil
}

// CreateNotebook 新建笔记本，LastInsertId 回填 ID，自动设置 created_at。
// name 重复时返回 UNIQUE 约束错误。
func (s *Store) CreateNotebook(nb *Notebook) error {
	if nb.CreatedAt.IsZero() {
		nb.CreatedAt = time.Now().UTC()
	}
	res, err := s.db.Exec(`INSERT INTO notebooks (name, description, cover, nb_order, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		nb.Name, nb.Description, nb.Cover, nb.Order, nb.CreatedAt)
	if err != nil {
		return fmt.Errorf("create notebook %s: %w", nb.Name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	nb.ID = id
	return nil
}

// UpdateNotebook 按 id 更新笔记本字段。笔记本不存在返回错误。
func (s *Store) UpdateNotebook(nb *Notebook) error {
	res, err := s.db.Exec(`UPDATE notebooks SET name=?, description=?, cover=?, nb_order=? WHERE id=?`,
		nb.Name, nb.Description, nb.Cover, nb.Order, nb.ID)
	if err != nil {
		return fmt.Errorf("update notebook %d: %w", nb.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("update notebook %d: no rows affected", nb.ID)
	}
	return nil
}

// DeleteNotebook 删除笔记本，并级联删除其下所有笔记（事务保证原子性）。不存在不报错。
func (s *Store) DeleteNotebook(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin delete notebook %d: %w", id, err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM notes WHERE notebook_id = ?`, id); err != nil {
		return fmt.Errorf("delete notes of notebook %d: %w", id, err)
	}
	if _, err := tx.Exec(`DELETE FROM notebooks WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete notebook %d: %w", id, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete notebook %d: %w", id, err)
	}
	return nil
}

// ListNotesByNotebook 分页列出笔记本下的笔记（按更新时间倒序，id 倒序兜底），
// 返回笔记列表 + 总数 + error。
func (s *Store) ListNotesByNotebook(notebookID int64, limit, offset int) ([]*Note, int, error) {
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM notes WHERE notebook_id = ?`, notebookID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count notes of notebook %d: %w", notebookID, err)
	}
	rows, err := s.db.Query(`SELECT `+noteCols+` FROM notes WHERE notebook_id = ? ORDER BY updated_at DESC, id DESC LIMIT ? OFFSET ?`,
		notebookID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list notes of notebook %d: %w", notebookID, err)
	}
	defer rows.Close()
	var out []*Note
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan note: %w", err)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate notes: %w", err)
	}
	return out, total, nil
}

// GetNote 按 id 查单条笔记。未找到返回 nil, nil。
func (s *Store) GetNote(id int64) (*Note, error) {
	row := s.db.QueryRow(`SELECT `+noteCols+` FROM notes WHERE id = ?`, id)
	n, err := scanNote(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get note %d: %w", id, err)
	}
	return n, nil
}

// CreateNote 新建笔记，LastInsertId 回填 ID，自动设置 created_at/updated_at。
func (s *Store) CreateNote(note *Note) error {
	now := time.Now().UTC()
	if note.CreatedAt.IsZero() {
		note.CreatedAt = now
	}
	note.UpdatedAt = now
	res, err := s.db.Exec(`INSERT INTO notes (notebook_id, title, content, content_html, tags, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		note.NotebookID, note.Title, note.Content, note.ContentHTML, note.Tags, note.CreatedAt, note.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create note %s: %w", note.Title, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	note.ID = id
	return nil
}

// UpdateNote 按 id 更新笔记字段，自动刷新 updated_at。笔记不存在返回错误。
func (s *Store) UpdateNote(note *Note) error {
	note.UpdatedAt = time.Now().UTC()
	res, err := s.db.Exec(`UPDATE notes SET notebook_id=?, title=?, content=?, content_html=?, tags=?, updated_at=? WHERE id=?`,
		note.NotebookID, note.Title, note.Content, note.ContentHTML, note.Tags, note.UpdatedAt, note.ID)
	if err != nil {
		return fmt.Errorf("update note %d: %w", note.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("update note %d: no rows affected", note.ID)
	}
	return nil
}

// DeleteNote 硬删除单条笔记。不存在不报错。
func (s *Store) DeleteNote(id int64) error {
	if _, err := s.db.Exec(`DELETE FROM notes WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete note %d: %w", id, err)
	}
	return nil
}

// CountNoteTagsByNotebook 统计指定笔记本下所有笔记的标签出现次数（tags 逗号分隔，内存拆分计数，跳过空值）。
// 供前台 tagtree widget 与笔记列表页标签聚合用（对齐 CountByTag 的计数模式）。
func (s *Store) CountNoteTagsByNotebook(notebookID int64) (map[string]int, error) {
	rows, err := s.db.Query(`SELECT tags FROM notes WHERE notebook_id = ? AND tags != ''`, notebookID)
	if err != nil {
		return nil, fmt.Errorf("query note tags of notebook %d: %w", notebookID, err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var tags string
		if err := rows.Scan(&tags); err != nil {
			return nil, fmt.Errorf("scan note tags: %w", err)
		}
		for _, t := range strings.Split(tags, ",") {
			if t = strings.TrimSpace(t); t != "" {
				out[t]++
			}
		}
	}
	return out, rows.Err()
}

// AllNoteTags 聚合所有笔记的标签（去重排序）。
func (s *Store) AllNoteTags() ([]string, error) {
	rows, err := s.db.Query(`SELECT tags FROM notes WHERE tags != ''`)
	if err != nil {
		return nil, fmt.Errorf("query note tags: %w", err)
	}
	defer rows.Close()

	tagSet := make(map[string]bool)
	for rows.Next() {
		var tags string
		if err := rows.Scan(&tags); err != nil {
			continue
		}
		for _, t := range strings.Split(tags, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tagSet[t] = true
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate note tags: %w", err)
	}

	result := make([]string, 0, len(tagSet))
	for t := range tagSet {
		result = append(result, t)
	}
	sort.Strings(result)
	return result, nil
}

// scanNotebook 单行扫描；行顺序对齐 notebookCols。
func scanNotebook(row rowScanner) (*Notebook, error) {
	nb := &Notebook{}
	err := row.Scan(&nb.ID, &nb.Name, &nb.Description, &nb.Cover, &nb.Order, &nb.CreatedAt)
	if err != nil {
		return nil, err
	}
	return nb, nil
}

// scanNote 单行扫描；行顺序对齐 noteCols。
func scanNote(row rowScanner) (*Note, error) {
	n := &Note{}
	err := row.Scan(&n.ID, &n.NotebookID, &n.Title, &n.Content, &n.ContentHTML, &n.Tags, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return n, nil
}
