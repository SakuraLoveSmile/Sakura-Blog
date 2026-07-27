package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// SharedPost 文章分享链接记录。Token 用于 URL 路径，ShareType 区分已发布版/草稿版。
type SharedPost struct {
	ID        int64
	Token     string // 32 位 hex token，用于 URL 路径
	PostSlug  string // 关联文章 slug
	ShareType string // "published" 或 "draft"
	ExpiresAt string // RFC3339 过期时间，空串=永久有效
	CreatedAt time.Time
}

const shareCols = `id, token, post_slug, share_type, expires_at, created_at`

// EnsureSharedPostsTable 建 shared_posts 表（token UNIQUE 索引，post_slug 索引）。
func (s *Store) EnsureSharedPostsTable() error {
	const ddl = `CREATE TABLE IF NOT EXISTS shared_posts (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		token      TEXT NOT NULL UNIQUE,
		post_slug  TEXT NOT NULL DEFAULT '',
		share_type TEXT NOT NULL DEFAULT 'published',
		expires_at TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_shared_posts_token ON shared_posts(token);
	CREATE INDEX IF NOT EXISTS idx_shared_posts_slug ON shared_posts(post_slug);`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create shared_posts table: %w", err)
	}
	return nil
}

// scanShares 扫描分享链接行集，返回切片。
func scanShares(rows *sql.Rows) ([]*SharedPost, error) {
	var out []*SharedPost
	for rows.Next() {
		sp := &SharedPost{}
		if err := rows.Scan(&sp.ID, &sp.Token, &sp.PostSlug, &sp.ShareType,
			&sp.ExpiresAt, &sp.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan shared_post: %w", err)
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

// generateShareToken 生成 32 位 hex token（16 字节随机数经 hex 编码）。
// 冲突时重试，确保唯一性。
func (s *Store) generateShareToken() (string, error) {
	for i := 0; i < 8; i++ {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return "", fmt.Errorf("generate share token: %w", err)
		}
		token := hex.EncodeToString(b)
		// 冲突检查（概率极低，但保证 UNIQUE 约束不报错）
		existing, err := s.GetShareByToken(token)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return token, nil
		}
	}
	return "", fmt.Errorf("generate share token: too many collisions")
}

// CreateShare 插入分享链接，回填 ID。Token 为空时自动生成唯一 token。
func (s *Store) CreateShare(sp *SharedPost) (int64, error) {
	if sp.Token == "" {
		token, err := s.generateShareToken()
		if err != nil {
			return 0, err
		}
		sp.Token = token
	}
	res, err := s.db.Exec(`INSERT INTO shared_posts (token, post_slug, share_type, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		sp.Token, sp.PostSlug, sp.ShareType, sp.ExpiresAt, time.Now())
	if err != nil {
		return 0, fmt.Errorf("create shared_post: %w", err)
	}
	id, _ := res.LastInsertId()
	sp.ID = id
	return id, nil
}

// GetShareByToken 按 token 查分享链接，未找到返回 nil, nil。
func (s *Store) GetShareByToken(token string) (*SharedPost, error) {
	rows, err := s.db.Query(`SELECT `+shareCols+` FROM shared_posts WHERE token = ?`, token)
	if err != nil {
		return nil, fmt.Errorf("get shared_post by token: %w", err)
	}
	defer rows.Close()
	shares, err := scanShares(rows)
	if err != nil || len(shares) == 0 {
		return nil, err
	}
	return shares[0], nil
}

// ListShares 全部分享链接，按 created_at 倒序。
func (s *Store) ListShares() ([]*SharedPost, error) {
	rows, err := s.db.Query(`SELECT ` + shareCols + ` FROM shared_posts ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list shared_posts: %w", err)
	}
	defer rows.Close()
	return scanShares(rows)
}

// ListSharesBySlug 按文章 slug 查分享链接，按 created_at 倒序。
func (s *Store) ListSharesBySlug(slug string) ([]*SharedPost, error) {
	rows, err := s.db.Query(`SELECT `+shareCols+` FROM shared_posts WHERE post_slug = ? ORDER BY created_at DESC`, slug)
	if err != nil {
		return nil, fmt.Errorf("list shared_posts by slug: %w", err)
	}
	defer rows.Close()
	return scanShares(rows)
}

// DeleteShare 删除分享链接。
func (s *Store) DeleteShare(id int64) error {
	if _, err := s.db.Exec(`DELETE FROM shared_posts WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete shared_post %d: %w", id, err)
	}
	return nil
}

// CountShares 统计分享链接数。
func (s *Store) CountShares() (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM shared_posts`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count shared_posts: %w", err)
	}
	return n, nil
}
