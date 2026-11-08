package store

import "fmt"

// NoteCountRow 是按笔记本、作者和状态分组的笔记数量。
// Console 列表只需要数量，不必为此加载每篇笔记的正文。
type NoteCountRow struct {
	NotebookID string
	AuthorID   int64
	Status     string
	Count      int
}

// CountNotes 返回全部笔记的分组数量；作者维度保留给调用方按权限过滤。
func (s *Store) CountNotes() ([]NoteCountRow, error) {
	if err := s.requireNotebookSchema(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT notebook_id, author_id, status, COUNT(*) FROM notes GROUP BY notebook_id, author_id, status ORDER BY notebook_id, author_id, status`)
	if err != nil {
		return nil, fmt.Errorf("count notes: %w", err)
	}
	defer rows.Close()
	out := []NoteCountRow{}
	for rows.Next() {
		var row NoteCountRow
		if err := rows.Scan(&row.NotebookID, &row.AuthorID, &row.Status, &row.Count); err != nil {
			return nil, fmt.Errorf("scan note count: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("count notes: %w", err)
	}
	return out, nil
}
