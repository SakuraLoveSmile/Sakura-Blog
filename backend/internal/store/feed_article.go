package store

import (
	"fmt"
	"time"
)

// FeedArticle 友链 RSS 抓取到的单篇文章（订阅阅读器持久化用）。
// guid 唯一（缺省用 link），upsert 时保留已读/收藏/稍后状态。
type FeedArticle struct {
	ID          int64  `json:"id"`
	FriendTitle string `json:"friend_title"`
	FriendURL   string `json:"friend_url"`
	Title       string `json:"title"`
	Link        string `json:"link"`
	Summary     string `json:"summary"`
	Published   string `json:"published"`
	GUID        string `json:"guid"`
	FetchedAt   string `json:"fetched_at"`
	Read        bool   `json:"read"`
	Favorite    bool   `json:"favorite"`
	Later       bool   `json:"later"`
}

// EnsureFeedArticlesTable 建 feed_articles 表（对齐 attachment.go/rating.go 模式，不动 createTables）。
func (s *Store) EnsureFeedArticlesTable() error {
	const ddl = `CREATE TABLE IF NOT EXISTS feed_articles (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		friend_title TEXT NOT NULL DEFAULT '',
		friend_url   TEXT NOT NULL DEFAULT '',
		title        TEXT NOT NULL DEFAULT '',
		link         TEXT NOT NULL DEFAULT '',
		summary      TEXT NOT NULL DEFAULT '',
		published    TEXT NOT NULL DEFAULT '',
		guid         TEXT NOT NULL DEFAULT '',
		fetched_at   TEXT NOT NULL DEFAULT '',
		read         INTEGER NOT NULL DEFAULT 0,
		favorite     INTEGER NOT NULL DEFAULT 0,
		later        INTEGER NOT NULL DEFAULT 0
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_feed_articles_guid ON feed_articles(guid);
	CREATE INDEX IF NOT EXISTS idx_feed_articles_friend ON feed_articles(friend_url);`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create feed_articles table: %w", err)
	}
	return nil
}

// UpsertFeedArticle 按 guid 幂等写入；已存在则只更新元数据，保留 read/favorite/later 状态。
func (s *Store) UpsertFeedArticle(a *FeedArticle) error {
	if a.GUID == "" {
		a.GUID = a.Link
	}
	if a.GUID == "" {
		return nil // 无稳定标识，跳过
	}
	const q = `INSERT INTO feed_articles
		(friend_title, friend_url, title, link, summary, published, guid, fetched_at, read, favorite, later)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, 0, 0)
		ON CONFLICT(guid) DO UPDATE SET
			friend_title = excluded.friend_title,
			friend_url   = excluded.friend_url,
			title        = excluded.title,
			link         = excluded.link,
			summary      = excluded.summary,
			published    = excluded.published,
			fetched_at   = excluded.fetched_at`
	_, err := s.db.Exec(q, a.FriendTitle, a.FriendURL, a.Title, a.Link, a.Summary,
		a.Published, a.GUID, time.Now().Format(time.RFC3339))
	return err
}

// FeedSource 订阅源聚合（供左栏列表：名称 + 未读/总数）。
type FeedSource struct {
	FriendTitle string `json:"friend_title"`
	FriendURL   string `json:"friend_url"`
	Total       int    `json:"total"`
	Unread      int    `json:"unread"`
}

// ListFeedSources 按友链分组统计文章数与未读数（按未读降序、总数降序）。
func (s *Store) ListFeedSources() ([]FeedSource, error) {
	const q = `SELECT friend_title, friend_url, COUNT(*) AS total,
		SUM(CASE WHEN read = 0 THEN 1 ELSE 0 END) AS unread
		FROM feed_articles GROUP BY friend_url ORDER BY unread DESC, total DESC`
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FeedSource
	for rows.Next() {
		var f FeedSource
		if err := rows.Scan(&f.FriendTitle, &f.FriendURL, &f.Total, &f.Unread); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// FeedArticleFilter 文章查询过滤条件。
type FeedArticleFilter struct {
	FriendURL string // 空 = 全部源
	Status    string // all | unread | read | favorite | later
	Limit     int
	Offset    int
}

// ListFeedArticles 按过滤条件查询文章（按 published 倒序，缺失时按 fetched_at）。
func (s *Store) ListFeedArticles(f FeedArticleFilter) ([]FeedArticle, error) {
	q := `SELECT id, friend_title, friend_url, title, link, summary, published, guid, fetched_at, read, favorite, later
		FROM feed_articles WHERE 1=1`
	var args []any
	if f.FriendURL != "" {
		q += " AND friend_url = ?"
		args = append(args, f.FriendURL)
	}
	switch f.Status {
	case "unread":
		q += " AND read = 0"
	case "read":
		q += " AND read = 1"
	case "favorite":
		q += " AND favorite = 1"
	case "later":
		q += " AND later = 1"
	}
	q += " ORDER BY published DESC, fetched_at DESC"
	if f.Limit > 0 {
		q += " LIMIT ? OFFSET ?"
		args = append(args, f.Limit, f.Offset)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]FeedArticle, 0)
	for rows.Next() {
		var a FeedArticle
		var read, fav, later int
		if err := rows.Scan(&a.ID, &a.FriendTitle, &a.FriendURL, &a.Title, &a.Link,
			&a.Summary, &a.Published, &a.GUID, &a.FetchedAt, &read, &fav, &later); err != nil {
			return nil, err
		}
		a.Read, a.Favorite, a.Later = read == 1, fav == 1, later == 1
		out = append(out, a)
	}
	return out, rows.Err()
}

// SetFeedArticleFlag 设置单篇文章的 read/favorite/later 标志。
func (s *Store) SetFeedArticleFlag(id int64, flag string, val bool) error {
	col := map[string]string{"read": "read", "favorite": "favorite", "later": "later"}[flag]
	if col == "" {
		return fmt.Errorf("invalid flag: %s", flag)
	}
	v := 0
	if val {
		v = 1
	}
	_, err := s.db.Exec("UPDATE feed_articles SET "+col+" = ? WHERE id = ?", v, id)
	return err
}

// MarkAllFeedRead 标记全部（或指定源）为已读。
func (s *Store) MarkAllFeedRead(friendURL string) error {
	if friendURL == "" {
		_, err := s.db.Exec("UPDATE feed_articles SET read = 1 WHERE read = 0")
		return err
	}
	_, err := s.db.Exec("UPDATE feed_articles SET read = 1 WHERE read = 0 AND friend_url = ?", friendURL)
	return err
}
