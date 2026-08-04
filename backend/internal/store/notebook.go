package store

import (
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

// TableName Entity 接口：notebooks 表。
func (n *Notebook) TableName() string { return "notebooks" }

// Columns Entity 接口：全列，首列为 id（排序列用 nb_order 而非 order，SQL 保留字）。
func (n *Notebook) Columns() []string {
	return []string{"id", "name", "description", "cover", "nb_order", "created_at"}
}

// Values Entity 接口：写入值（不含 id，与 Columns()[1:] 对应）。
func (n *Notebook) Values() []any {
	return []any{n.Name, n.Description, n.Cover, n.Order, n.CreatedAt}
}

// SetID Entity 接口：回填自增 id。
func (n *Notebook) SetID(id int64) { n.ID = id }

// ScanRow Entity 接口：行扫描，顺序对齐 Columns()。
func (n *Notebook) ScanRow(row rowScanner) error {
	return row.Scan(&n.ID, &n.Name, &n.Description, &n.Cover, &n.Order, &n.CreatedAt)
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

// TableName Entity 接口：notes 表。
func (n *Note) TableName() string { return "notes" }

// Columns Entity 接口：全列，首列为 id。
func (n *Note) Columns() []string {
	return []string{"id", "notebook_id", "title", "content", "content_html", "tags", "created_at", "updated_at"}
}

// Values Entity 接口：写入值（不含 id，与 Columns()[1:] 对应）。
func (n *Note) Values() []any {
	return []any{n.NotebookID, n.Title, n.Content, n.ContentHTML, n.Tags, n.CreatedAt, n.UpdatedAt}
}

// SetID Entity 接口：回填自增 id。
func (n *Note) SetID(id int64) { n.ID = id }

// ScanRow Entity 接口：行扫描，顺序对齐 Columns()。
func (n *Note) ScanRow(row rowScanner) error {
	return row.Scan(&n.ID, &n.NotebookID, &n.Title, &n.Content, &n.ContentHTML, &n.Tags, &n.CreatedAt, &n.UpdatedAt)
}

// EnsureNotebooksTable 幂等建 notebooks 表（统一走 schema.go 注册表）。
func (s *Store) EnsureNotebooksTable() error {
	return s.ensureTable("notebooks")
}

// EnsureNotesTable 幂等建 notes 表。
func (s *Store) EnsureNotesTable() error {
	return s.ensureTable("notes")
}

// ListNotebooks 列出全部笔记本，按 nb_order, id 升序。
func (s *Store) ListNotebooks() ([]*Notebook, error) {
	return repo(s, &Notebook{}).List(OrderBy("nb_order, id"))
}

// GetNotebook 按 id 查单个笔记本。未找到返回 nil, nil。
func (s *Store) GetNotebook(id int64) (*Notebook, error) {
	nb, err := repo(s, &Notebook{}).GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("get notebook %d: %w", id, err)
	}
	return nb, nil
}

// GetNotebookByName 按 name 查单个笔记本。未找到返回 nil, nil。
func (s *Store) GetNotebookByName(name string) (*Notebook, error) {
	nb, err := repo(s, &Notebook{}).GetOne("name = ?", name)
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
	if _, err := repo(s, &Notebook{}).Create(nb); err != nil {
		return fmt.Errorf("create notebook %s: %w", nb.Name, err)
	}
	return nil
}

// UpdateNotebook 按 id 更新笔记本字段。笔记本不存在返回错误。
func (s *Store) UpdateNotebook(nb *Notebook) error {
	if err := repo(s, &Notebook{}).Update(nb.ID, nb); err != nil {
		return fmt.Errorf("update notebook %d: %w", nb.ID, err)
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
	total, err := repo(s, &Note{}).Count(Where("notebook_id = ?", notebookID))
	if err != nil {
		return nil, 0, fmt.Errorf("count notes of notebook %d: %w", notebookID, err)
	}
	out, err := repo(s, &Note{}).List(
		Where("notebook_id = ?", notebookID),
		OrderBy("updated_at DESC, id DESC"),
		Limit(limit), Offset(offset))
	if err != nil {
		return nil, 0, fmt.Errorf("list notes of notebook %d: %w", notebookID, err)
	}
	return out, total, nil
}

// GetNote 按 id 查单条笔记。未找到返回 nil, nil。
func (s *Store) GetNote(id int64) (*Note, error) {
	n, err := repo(s, &Note{}).GetByID(id)
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
	if _, err := repo(s, &Note{}).Create(note); err != nil {
		return fmt.Errorf("create note %s: %w", note.Title, err)
	}
	return nil
}

// UpdateNote 按 id 更新笔记字段，自动刷新 updated_at。笔记不存在返回错误。
func (s *Store) UpdateNote(note *Note) error {
	note.UpdatedAt = time.Now().UTC()
	if err := repo(s, &Note{}).Update(note.ID, note); err != nil {
		return fmt.Errorf("update note %d: %w", note.ID, err)
	}
	return nil
}

// DeleteNote 硬删除单条笔记。不存在不报错。
func (s *Store) DeleteNote(id int64) error {
	if err := repo(s, &Note{}).Delete(id); err != nil {
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
