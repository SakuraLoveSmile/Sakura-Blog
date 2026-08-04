package store

import (
	"fmt"
	"time"
	"unicode"
)

// maxRevisionsPerPost 每篇文章保留的最近版本数（Spec 22）。
const maxRevisionsPerPost = 20

// Revision 文章版本快照（保存前的旧内容全文）。对齐 revisions 表字段。
type Revision struct {
	ID        int64
	PostSlug  string
	Title     string
	Excerpt   string
	ContentMD string
	Tags      string
	Category  string
	Status    string
	WordCount int // 快照字数（rune 计），用于列表展示字数变化 ±N
	CreatedAt time.Time
}

// TableName Entity 接口：revisions 表。
func (r *Revision) TableName() string { return "revisions" }

// Columns Entity 接口：全列，首列为 id。
func (r *Revision) Columns() []string {
	return []string{"id", "post_slug", "title", "excerpt", "content_md", "tags", "category", "status", "word_count", "created_at"}
}

// Values Entity 接口：写入值（不含 id，与 Columns()[1:] 对应）。
func (r *Revision) Values() []any {
	return []any{r.PostSlug, r.Title, r.Excerpt, r.ContentMD, r.Tags, r.Category, r.Status, r.WordCount, r.CreatedAt}
}

// SetID Entity 接口：回填自增 id。
func (r *Revision) SetID(id int64) { r.ID = id }

// ScanRow Entity 接口：行扫描，顺序对齐 Columns()。
func (r *Revision) ScanRow(row rowScanner) error {
	return row.Scan(&r.ID, &r.PostSlug, &r.Title, &r.Excerpt, &r.ContentMD, &r.Tags, &r.Category, &r.Status, &r.WordCount, &r.CreatedAt)
}

// RevisionMeta 版本列表项（不含 content_md，省流量）。
type RevisionMeta struct {
	ID        int64     `json:"id"`
	PostSlug  string    `json:"post_slug"`
	Title     string    `json:"title"`
	Excerpt   string    `json:"excerpt"`
	Tags      string    `json:"tags"`
	Category  string    `json:"category"`
	Status    string    `json:"status"`
	WordCount int       `json:"word_count"`
	CreatedAt time.Time `json:"created_at"`
}

// EnsureRevisionsTable 独立建 revisions 表（统一走 schema.go 注册表）。
// 幂等迁移：旧库可能缺 word_count 列（ADD COLUMN 已存在会报错，忽略即幂等）。
func (s *Store) EnsureRevisionsTable() error {
	if err := s.ensureTable("revisions"); err != nil {
		return err
	}
	s.db.Exec(`ALTER TABLE revisions ADD COLUMN word_count INTEGER NOT NULL DEFAULT 0`)
	return nil
}

// CreateRevision 写入一条版本快照，返回 id 并回填 r.ID；随后顺手裁剪到上限。
func (s *Store) CreateRevision(r *Revision) (int64, error) {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	r.WordCount = CountWords(r.ContentMD)
	id, err := repo(s, &Revision{}).Create(r)
	if err != nil {
		return 0, fmt.Errorf("create revision: %w", err)
	}
	s.pruneRevisions(r.PostSlug)
	return id, nil
}

// ListRevisions 按时间倒序列出某文章的版本元数据（不含 content_md）。
func (s *Store) ListRevisions(slug string) ([]*RevisionMeta, error) {
	rows, err := s.db.Query(`SELECT id, post_slug, title, excerpt, tags, category, status, word_count, created_at
		FROM revisions WHERE post_slug = ? ORDER BY substr(created_at, 1, 19) DESC, id DESC`, slug)
	if err != nil {
		return nil, fmt.Errorf("list revisions: %w", err)
	}
	defer rows.Close()
	var out []*RevisionMeta
	for rows.Next() {
		m := &RevisionMeta{}
		if err := rows.Scan(&m.ID, &m.PostSlug, &m.Title, &m.Excerpt, &m.Tags, &m.Category, &m.Status, &m.WordCount, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan revision meta: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate revisions: %w", err)
	}
	return out, nil
}

// GetRevision 按 id 查单个版本全文。未找到返回 nil, nil。
func (s *Store) GetRevision(id int64) (*Revision, error) {
	r, err := repo(s, &Revision{}).GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("get revision %d: %w", id, err)
	}
	return r, nil
}

// pruneRevisions 裁剪某文章版本到 maxRevisionsPerPost 条（保留最新，删最旧）。
func (s *Store) pruneRevisions(slug string) {
	s.db.Exec(`DELETE FROM revisions WHERE post_slug = ? AND id NOT IN (
		SELECT id FROM revisions WHERE post_slug = ? ORDER BY substr(created_at, 1, 19) DESC, id DESC LIMIT ?
	)`, slug, slug, maxRevisionsPerPost)
}

// DeleteRevisionsByPost 删除某文章的全部版本（彻底删除文章时调用）。
func (s *Store) DeleteRevisionsByPost(slug string) error {
	if _, err := s.db.Exec(`DELETE FROM revisions WHERE post_slug = ?`, slug); err != nil {
		return fmt.Errorf("delete revisions by post %s: %w", slug, err)
	}
	return nil
}

// UpdateRevisionSlug slug 变更时同步迁移版本归属。
func (s *Store) UpdateRevisionSlug(oldSlug, newSlug string) error {
	if oldSlug == newSlug {
		return nil
	}
	if _, err := s.db.Exec(`UPDATE revisions SET post_slug = ? WHERE post_slug = ?`, newSlug, oldSlug); err != nil {
		return fmt.Errorf("update revision slug %s->%s: %w", oldSlug, newSlug, err)
	}
	return nil
}

// CountWords 统计字数：累计非空白 rune 数（中文按字、英文按字符，单人博客场景足够）。
func CountWords(s string) int {
	n := 0
	for _, r := range s {
		if !unicode.IsSpace(r) {
			n++
		}
	}
	return n
}
