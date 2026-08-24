// Package store 按作者查询文章（Spec 38 多作者体系）。
package store

import (
	"fmt"
)

// ListByAuthor 按作者公开 ID（通常是 username）查询已发布文章（分页）。
// 主题作者页继续使用 username 作为 URL，以兼容现有站点链接；数据库查询
// 已统一落到 users.id。
func (s *Store) ListByAuthor(authorRef string, offset, limit int) ([]*Post, error) {
	authorID, err := s.resolvePostAuthorID(authorRef)
	if err != nil {
		return nil, fmt.Errorf("resolve author %s: %w", authorRef, err)
	}
	return s.ListByAuthorID(authorID, offset, limit)
}

// ListByAuthorID 按 users.id 查询已发布文章（分页）。
func (s *Store) ListByAuthorID(authorID int64, offset, limit int) ([]*Post, error) {
	if offset < 0 {
		offset = 0
	}
	if limit < 1 {
		limit = 10
	}
	rows, err := s.db.Query("SELECT "+listCols+" FROM posts WHERE "+publishedWhere+" AND EXISTS (SELECT 1 FROM post_authors pa WHERE pa.post_id = posts.id AND pa.author_id = ?) "+publishedOrder+" LIMIT ? OFFSET ?",
		StatusPublished, authorID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list posts by author %d: %w", authorID, err)
	}
	defer rows.Close()
	return s.scanPosts(rows)
}

// CountByAuthor 按作者公开 ID（通常是 username）统计已发布文章数。
func (s *Store) CountByAuthor(authorRef string) (int, error) {
	authorID, err := s.resolvePostAuthorID(authorRef)
	if err != nil {
		return 0, fmt.Errorf("resolve author %s: %w", authorRef, err)
	}
	return s.CountByAuthorID(authorID)
}

// CountByAuthorID 按 users.id 统计已发布文章数。
func (s *Store) CountByAuthorID(authorID int64) (int, error) {
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM posts WHERE "+publishedWhere+" AND EXISTS (SELECT 1 FROM post_authors pa WHERE pa.post_id = posts.id AND pa.author_id = ?)", StatusPublished, authorID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count posts by author %d: %w", authorID, err)
	}
	return n, nil
}
