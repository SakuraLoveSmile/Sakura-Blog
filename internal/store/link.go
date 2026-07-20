package store

import (
	"database/sql"
	"fmt"
)

// Link 友链条目（DB 持久化）。GroupTitle 承载 site.yml 的分组标题（扁平化）。
// 字段命名与前端 friends_and_posts 渲染对齐；desc 列名用 description（避开 SQL 关键字 DESC）。
type Link struct {
	ID          int64
	GroupTitle  string
	Name        string
	URL         string
	Avatar      string
	Description string
	RSS         string
	Label       string
	Color       string
	Sort        int
}

const linkCols = `id, group_title, name, url, avatar, description, rss, label, color, sort`

// EnsureLinksTable 建 links 表（独立建表，友链由 site.yml 迁移到 DB）。
func (s *Store) EnsureLinksTable() error {
	const ddl = `CREATE TABLE IF NOT EXISTS links (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		group_title TEXT NOT NULL DEFAULT '',
		name        TEXT NOT NULL DEFAULT '',
		url         TEXT NOT NULL DEFAULT '',
		avatar      TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		rss         TEXT NOT NULL DEFAULT '',
		label       TEXT NOT NULL DEFAULT '',
		color       TEXT NOT NULL DEFAULT '',
		sort        INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_links_group ON links(group_title);`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create links table: %w", err)
	}
	return nil
}

func scanLinks(rows *sql.Rows) ([]*Link, error) {
	var out []*Link
	for rows.Next() {
		l := &Link{}
		if err := rows.Scan(&l.ID, &l.GroupTitle, &l.Name, &l.URL, &l.Avatar,
			&l.Description, &l.RSS, &l.Label, &l.Color, &l.Sort); err != nil {
			return nil, fmt.Errorf("scan link: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// ListLinks 全部友链。sort 降序优先（越大越靠前），其次 id 升序（保留插入=site.yml 顺序）。
func (s *Store) ListLinks() ([]*Link, error) {
	rows, err := s.db.Query(`SELECT ` + linkCols + ` FROM links ORDER BY sort DESC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}
	defer rows.Close()
	return scanLinks(rows)
}

// GetLink 按 id 查友链，未找到返回 nil, nil。
func (s *Store) GetLink(id int64) (*Link, error) {
	rows, err := s.db.Query(`SELECT `+linkCols+` FROM links WHERE id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("get link %d: %w", id, err)
	}
	defer rows.Close()
	links, err := scanLinks(rows)
	if err != nil || len(links) == 0 {
		return nil, err
	}
	return links[0], nil
}

// CreateLink 插入友链，回填 ID。
func (s *Store) CreateLink(l *Link) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO links (group_title, name, url, avatar, description, rss, label, color, sort)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		l.GroupTitle, l.Name, l.URL, l.Avatar, l.Description, l.RSS, l.Label, l.Color, l.Sort)
	if err != nil {
		return 0, fmt.Errorf("create link: %w", err)
	}
	id, _ := res.LastInsertId()
	l.ID = id
	return id, nil
}

// UpdateLink 更新友链全部字段。
func (s *Store) UpdateLink(id int64, l *Link) error {
	_, err := s.db.Exec(`UPDATE links SET group_title=?, name=?, url=?, avatar=?, description=?, rss=?, label=?, color=?, sort=? WHERE id=?`,
		l.GroupTitle, l.Name, l.URL, l.Avatar, l.Description, l.RSS, l.Label, l.Color, l.Sort, id)
	if err != nil {
		return fmt.Errorf("update link %d: %w", id, err)
	}
	return nil
}

// DeleteLink 删除友链。
func (s *Store) DeleteLink(id int64) error {
	if _, err := s.db.Exec(`DELETE FROM links WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete link %d: %w", id, err)
	}
	return nil
}

// CountLinks 统计友链数（Bootstrap 判定空表用）。
func (s *Store) CountLinks() (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM links`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count links: %w", err)
	}
	return n, nil
}
