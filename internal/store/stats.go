package store

import "fmt"

// PopularPost 热门文章只读条目（按阅读量降序）。
type PopularPost struct {
	Slug  string
	Title string
	Views int64
}

// ListPopular 返回已发布文章中阅读量最高的 limit 篇，views 相同时按创建时间倒序。
func (s *Store) ListPopular(limit int) ([]PopularPost, error) {
	if limit < 1 {
		limit = 5
	}
	rows, err := s.db.Query(
		"SELECT slug, title, views FROM posts WHERE "+publishedWhere+" ORDER BY views DESC, datetime(created_at) DESC LIMIT ?",
		StatusPublished, limit)
	if err != nil {
		return nil, fmt.Errorf("list popular posts: %w", err)
	}
	defer rows.Close()

	var out []PopularPost
	for rows.Next() {
		var p PopularPost
		if err := rows.Scan(&p.Slug, &p.Title, &p.Views); err != nil {
			return nil, fmt.Errorf("scan popular post: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate popular posts: %w", err)
	}
	return out, nil
}

// CountPosts 返回已发布文章总数（复用 CountPublished）。
func (s *Store) CountPosts() (int, error) {
	return s.CountPublished()
}
