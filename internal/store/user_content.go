package store

import "fmt"

// ListPostsByOwner returns the current user's posts, including drafts. The
// trashed status is explicit; all other statuses exclude soft-deleted rows.
func (s *Store) ListPostsByOwner(ownerID int64, status string, page, size int) ([]*Post, int, error) {
	if ownerID <= 0 {
		return []*Post{}, 0, nil
	}
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	where := `WHERE EXISTS (SELECT 1 FROM post_authors pa WHERE pa.post_id = posts.id AND pa.author_id = ?)`
	args := []any{ownerID}
	if status == "trashed" {
		where += ` AND deleted_at != ''`
	} else {
		where += ` AND deleted_at = ''`
		if status == StatusDraft || status == StatusPublished {
			where += ` AND status = ?`
			args = append(args, status)
		}
	}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM posts `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count posts by owner %d: %w", ownerID, err)
	}
	listArgs := append(append([]any(nil), args...), size, (page-1)*size)
	rows, err := s.db.Query(`SELECT `+listCols+` FROM posts `+where+`
		ORDER BY pinned DESC, substr(updated_at, 1, 19) DESC, id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list posts by owner %d: %w", ownerID, err)
	}
	defer rows.Close()
	posts, err := s.scanPosts(rows)
	if err != nil {
		return nil, 0, err
	}
	return posts, total, nil
}
