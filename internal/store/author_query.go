// Package store 按作者查询文章（Spec 38 多作者体系）。
package store

import (
	"fmt"
)

// ListByAuthor 按作者 ID 查询已发布文章（分页）。
func (s *Store) ListByAuthor(authorID string, offset, limit int) ([]*Post, error) {
	if offset < 0 {
		offset = 0
	}
	if limit < 1 {
		limit = 10
	}
	rows, err := s.db.Query("SELECT "+listCols+" FROM posts WHERE "+publishedWhere+" AND EXISTS (SELECT 1 FROM post_authors pa WHERE pa.post_id = posts.id AND pa.author_id = ?) "+publishedOrder+" LIMIT ? OFFSET ?",
		StatusPublished, authorID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list posts by author %s: %w", authorID, err)
	}
	defer rows.Close()
	return s.scanPosts(rows)
}

// CountByAuthor 按作者 ID 统计已发布文章数。
func (s *Store) CountByAuthor(authorID string) (int, error) {
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM posts WHERE "+publishedWhere+" AND EXISTS (SELECT 1 FROM post_authors pa WHERE pa.post_id = posts.id AND pa.author_id = ?)", StatusPublished, authorID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count posts by author %s: %w", authorID, err)
	}
	return n, nil
}
