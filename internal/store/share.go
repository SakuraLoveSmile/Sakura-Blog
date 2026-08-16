package store

import (
	"crypto/rand"
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

// TableName Entity 接口：shared_posts 表。
func (sp *SharedPost) TableName() string { return "shared_posts" }

// Columns Entity 接口：全列，首列为 id。
func (sp *SharedPost) Columns() []string {
	return []string{"id", "token", "post_slug", "share_type", "expires_at", "created_at"}
}

// Values Entity 接口：写入值（不含 id，与 Columns()[1:] 对应）。
func (sp *SharedPost) Values() []any {
	return []any{sp.Token, sp.PostSlug, sp.ShareType, sp.ExpiresAt, sp.CreatedAt}
}

// SetID Entity 接口：回填自增 id。
func (sp *SharedPost) SetID(id int64) { sp.ID = id }

// ScanRow Entity 接口：行扫描，顺序对齐 Columns()。
func (sp *SharedPost) ScanRow(row rowScanner) error {
	return row.Scan(&sp.ID, &sp.Token, &sp.PostSlug, &sp.ShareType, &sp.ExpiresAt, &sp.CreatedAt)
}

// EnsureSharedPostsTable 建 shared_posts 表（token UNIQUE 索引，post_slug 索引）。
func (s *Store) EnsureSharedPostsTable() error {
	return s.ensureTable("shared_posts")
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
	sp.CreatedAt = time.Now()
	return repo(s, &SharedPost{}).Create(sp)
}

// GetShareByToken 按 token 查分享链接，未找到返回 nil, nil。
func (s *Store) GetShareByToken(token string) (*SharedPost, error) {
	return repo(s, &SharedPost{}).GetOne("token = ?", token)
}

// ListShares 全部分享链接，按 created_at 倒序。
func (s *Store) ListShares() ([]*SharedPost, error) {
	return repo(s, &SharedPost{}).List(OrderBy("created_at DESC"))
}

// ListSharesBySlug 按文章 slug 查分享链接，按 created_at 倒序。
func (s *Store) ListSharesBySlug(slug string) ([]*SharedPost, error) {
	return repo(s, &SharedPost{}).List(Where("post_slug = ?", slug), OrderBy("created_at DESC"))
}

// DeleteShare 删除分享链接。
func (s *Store) DeleteShare(id int64) error {
	return repo(s, &SharedPost{}).Delete(id)
}

// CountShares 统计分享链接数。
func (s *Store) CountShares() (int, error) {
	return repo(s, &SharedPost{}).Count()
}
