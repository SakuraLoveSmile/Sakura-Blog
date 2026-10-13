// 专栏（Topic）持久化层（Spec 52）：topics 表 + posts.topic_id 关联迁移。
// 迁移策略：ALTER TABLE 加列（不重建表），存量 topic 文本一次性回填（幂等）。
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ErrTopicInUse 删除保护：专栏下仍有文章时 DeleteTopic 返回该错误，API 层映射 403。
var ErrTopicInUse = errors.New("topic still has posts")

// rowScannerQuery 行扫描 + 单行查询接口：*sql.DB / *sql.Tx / *sql.Conn 均满足，
// 供 uniqueTopicSlug 在事务内外复用（事务内走 *sql.Tx）。
type rowScannerQuery interface {
	QueryRow(query string, args ...any) *sql.Row
}

// slug 生成用预编译正则（对齐 api 包 generateSlug 的规则）。
var (
	topicReSpace       = regexp.MustCompile(`\s+`)            // 空白转连字符
	topicReNonSlugChar = regexp.MustCompile(`[^\p{L}\p{N}-]`) // 移除非 Unicode 字母/数字/连字符
	topicReMultiDash   = regexp.MustCompile(`-{2,}`)          // 折叠多个连字符
)

// Topic 专栏。
type Topic struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	Description    string    `json:"description"`
	Icon           string    `json:"icon"`
	Cover          string    `json:"cover"`
	Sort           int       `json:"sort"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	StellarVersion int       `json:"stellar_version"`
	StellarConfig  string    `json:"stellar_config"`
	BlogExtensions string    `json:"blog_extensions"`
}

// TableName Entity 接口：topics 表。
func (t *Topic) TableName() string { return "topics" }

// Columns Entity 接口：全列，首列为 id。
func (t *Topic) Columns() []string {
	return []string{"id", "name", "slug", "description", "icon", "cover", "sort", "created_at", "updated_at", "stellar_version", "stellar_config", "blog_extensions"}
}

// Values Entity 接口：写入值（不含 id，与 Columns()[1:] 对应）。
func (t *Topic) Values() []any {
	return []any{t.Name, t.Slug, t.Description, t.Icon, t.Cover, t.Sort, t.CreatedAt, t.UpdatedAt, t.StellarVersion, t.StellarConfig, t.BlogExtensions}
}

// SetID Entity 接口：回填自增 id。
func (t *Topic) SetID(id int64) { t.ID = id }

// ScanRow Entity 接口：行扫描，顺序对齐 Columns()。
func (t *Topic) ScanRow(row rowScanner) error {
	return row.Scan(&t.ID, &t.Name, &t.Slug, &t.Description, &t.Icon, &t.Cover, &t.Sort, &t.CreatedAt, &t.UpdatedAt, &t.StellarVersion, &t.StellarConfig, &t.BlogExtensions)
}

// EnsureTopicsTable 幂等建 topics 表（DDL 对齐 Spec 52：name/slug 双唯一）。
func (s *Store) EnsureTopicsTable() error {
	return s.ensureTable("topics")
}

// MigrateTopicBackfill 迁移：posts 表增加 topic_id 列（ALTER TABLE，存量默认 0=不属于专栏），
// 并把历史 topic 文本回填为 topics 表关联。幂等：列存在跳过 ALTER，无待回填数据跳过回填。
func (s *Store) MigrateTopicBackfill() error {
	// 1. ALTER TABLE 加列（幂等：先 PRAGMA 检测）
	rows, err := s.db.Query(`PRAGMA table_info(posts)`)
	if err != nil {
		return fmt.Errorf("pragma table_info(posts): %w", err)
	}
	hasTopicID := false
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			rows.Close()
			return fmt.Errorf("scan table_info: %w", err)
		}
		if name == "topic_id" {
			hasTopicID = true
		}
	}
	rows.Close()
	if !hasTopicID {
		if _, err := s.db.Exec(`ALTER TABLE posts ADD COLUMN topic_id INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("alter posts add topic_id: %w", err)
		}
	}

	// 2. 回填：仅当存在「topic 文本非空但未关联」的文章时执行（幂等保护）
	var pending int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM posts WHERE topic != '' AND topic_id = 0`).Scan(&pending); err != nil {
		return fmt.Errorf("count pending topic posts: %w", err)
	}
	if pending == 0 {
		return nil
	}

	// 3. 对存量 topic 文本去重建专栏（INSERT OR IGNORE：已存在的 name 跳过），再全量 UPDATE 回填。
	//    事务包裹保证原子性；slug 冲突时追加 -2/-3 后缀（与 API 创建逻辑同规则）。
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin topic backfill: %w", err)
	}
	defer tx.Rollback()

	topicRows, err := tx.Query(`SELECT DISTINCT topic FROM posts WHERE topic != '' AND topic_id = 0`)
	if err != nil {
		return fmt.Errorf("query distinct topics: %w", err)
	}
	var names []string
	for topicRows.Next() {
		var name string
		if err := topicRows.Scan(&name); err != nil {
			topicRows.Close()
			return fmt.Errorf("scan distinct topic: %w", err)
		}
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	topicRows.Close()
	if err := topicRows.Err(); err != nil {
		return fmt.Errorf("iterate distinct topics: %w", err)
	}

	now := time.Now().UTC()
	for _, name := range names {
		slug := uniqueTopicSlug(tx, slugify(name))
		if _, err := tx.Exec(`INSERT OR IGNORE INTO topics (name, slug, description, icon, cover, sort, created_at, updated_at)
			VALUES (?, ?, '', '', '', 0, ?, ?)`, name, slug, now, now); err != nil {
			return fmt.Errorf("insert topic %s: %w", name, err)
		}
	}

	if _, err := tx.Exec(`UPDATE posts SET topic_id = COALESCE((SELECT id FROM topics WHERE name = posts.topic), 0)
		WHERE topic != '' AND topic_id = 0`); err != nil {
		return fmt.Errorf("backfill topic_id: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit topic backfill: %w", err)
	}
	return nil
}

// uniqueTopicSlug 在给定查询器上检测 slug 冲突，冲突时追加 -2/-3 后缀直到唯一（事务内用）。
func uniqueTopicSlug(q rowScannerQuery, slug string) string {
	if slug == "" {
		slug = fmt.Sprintf("topic-%d", time.Now().Unix())
	}
	base := slug
	for n := 2; ; n++ {
		var exists int
		err := q.QueryRow(`SELECT COUNT(*) FROM topics WHERE slug = ?`, slug).Scan(&exists)
		if err != nil || exists == 0 {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", base, n)
	}
}

// slugify 从名称生成 URL 友好的 slug（对齐 api 包 generateSlug 规则；空结果用时间戳兜底）。
func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = topicReSpace.ReplaceAllString(s, "-")
	s = topicReNonSlugChar.ReplaceAllString(s, "")
	s = topicReMultiDash.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = fmt.Sprintf("topic-%d", time.Now().Unix())
	}
	return s
}

// ListTopics 列出全部专栏，按 sort 倒序 + 创建时间线性（sort 相同默认 0 时即纯时间倒序）。
func (s *Store) ListTopics() ([]*Topic, error) {
	return repo(s, &Topic{}).List(OrderBy("sort DESC, substr(created_at, 1, 19) DESC, id DESC"))
}

// GetTopicByID 按 id 查专栏。未找到返回 nil, nil。
func (s *Store) GetTopicByID(id int64) (*Topic, error) {
	return repo(s, &Topic{}).GetByID(id)
}

// GetTopicBySlug 按 slug 查专栏。未找到返回 nil, nil。
func (s *Store) GetTopicBySlug(slug string) (*Topic, error) {
	return repo(s, &Topic{}).GetOne("slug = ?", slug)
}

// GetTopicByName 按 name 查专栏。未找到返回 nil, nil。
func (s *Store) GetTopicByName(name string) (*Topic, error) {
	return repo(s, &Topic{}).GetOne("name = ?", name)
}

// CreateTopic 新建专栏，LastInsertId 回填 ID，自动设置 created_at/updated_at。
// name/slug 重复时返回 UNIQUE 约束错误（调用方映射 409）。
func (s *Store) CreateTopic(t *Topic) error {
	if t == nil {
		return fmt.Errorf("nil content")
	}
	if err := validateStellarWrite(t.StellarVersion, t.StellarConfig, t.BlogExtensions, "topic/"+t.Slug, "topic", true); err != nil {
		return err
	}
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	_, err := repo(s, &Topic{}).Create(t)
	return err
}

// UpdateTopic 按 id 更新专栏字段，自动刷新 updated_at。专栏不存在返回错误。
func (s *Store) UpdateTopic(t *Topic) error {
	if t == nil {
		return fmt.Errorf("nil content")
	}
	if err := validateStellarWrite(t.StellarVersion, t.StellarConfig, t.BlogExtensions, "topic/"+t.Slug, "topic", true); err != nil {
		return err
	}
	t.UpdatedAt = time.Now().UTC()

	// 专栏名称是文章表中的冗余展示字段，必须与 topics 更新保持原子性。
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin update topic %d: %w", t.ID, err)
	}
	defer tx.Rollback()

	var oldSlug string
	if err := tx.QueryRow(`SELECT slug FROM topics WHERE id=?`, t.ID).Scan(&oldSlug); err != nil {
		return err
	}
	if oldSlug != t.Slug {
		n, err := topicMembershipCount(tx, t.ID, oldSlug)
		if err != nil {
			return err
		}
		if n > 0 {
			return ErrTopicInUse
		}
	}

	result, err := tx.Exec(`UPDATE topics SET name=?, slug=?, description=?, icon=?, cover=?, sort=?, updated_at=?, stellar_version=?, stellar_config=?, blog_extensions=? WHERE id=? AND (stellar_version = 0 OR stellar_version = ?)`,
		t.Name, t.Slug, t.Description, t.Icon, t.Cover, t.Sort, t.UpdatedAt, t.StellarVersion, t.StellarConfig, t.BlogExtensions, t.ID, t.StellarVersion)
	if err != nil {
		return fmt.Errorf("update topic %d: %w", t.ID, err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("update topic %d: no rows affected", t.ID)
	}
	if _, err := tx.Exec(`UPDATE posts SET topic=? WHERE topic_id=?`, t.Name, t.ID); err != nil {
		return fmt.Errorf("sync topic name for %d: %w", t.ID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit update topic %d: %w", t.ID, err)
	}
	return nil
}

// DeleteTopic 删除专栏。专栏下仍有文章时返回 ErrTopicInUse（删除保护，调用方映射 403）。
// 专栏不存在不报错。
func (s *Store) DeleteTopic(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var slug string
	if err := tx.QueryRow(`SELECT slug FROM topics WHERE id=?`, id).Scan(&slug); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	n, err := topicMembershipCount(tx, id, slug)
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrTopicInUse
	}
	if _, err := tx.Exec(`DELETE FROM topics WHERE id=?`, id); err != nil {
		return fmt.Errorf("delete topic %d: %w", id, err)
	}
	return tx.Commit()
}

// CountByTopic 统计各专栏（posts.topic 文本字段）的已发布文章数（空专栏除外），
// navbar「专栏」tab 显示条件用（HasTopics）。
func (s *Store) CountByTopic() (map[string]int, error) {
	rows, err := s.db.Query("SELECT topic, COUNT(*) FROM posts WHERE "+publishedWhere+" AND topic != '' GROUP BY topic", StatusPublished)
	if err != nil {
		return nil, fmt.Errorf("count by topic: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var name string
		var n int
		if err := rows.Scan(&name, &n); err != nil {
			return nil, fmt.Errorf("scan topic count: %w", err)
		}
		out[name] = n
	}
	return out, rows.Err()
}

// CountByTopicID 统计专栏下的已发布文章数（删除保护用）。
func (s *Store) CountByTopicID(topicID int64) (int, error) {
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM posts WHERE "+publishedWhere+" AND topic_id = ?", StatusPublished, topicID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count posts of topic %d: %w", topicID, err)
	}
	return n, nil
}

// CountAllByTopicID 统计专栏下全部文章数（含草稿/回收站，Console 列表角标用）。
// topicMembershipCount 在同一事务快照中检查存储关联、严格原文及未发布草稿；坏旧 JSON 不参与 JSON 解码。
func topicMembershipCount(q rowScannerQuery, id int64, slug string) (int, error) {
	var n int
	err := q.QueryRow(`WITH source AS (
	 SELECT topic_id, stellar_version, CASE WHEN json_valid(stellar_config) THEN stellar_config ELSE '{}' END AS live,
	 CASE WHEN json_valid(draft_json) THEN draft_json ELSE '{}' END AS draft FROM posts
	), parsed AS (
	 SELECT *, CASE WHEN json_valid(json_extract(draft,'$.stellar_config')) THEN json_extract(draft,'$.stellar_config') ELSE '{}' END AS pending FROM source
	) SELECT COUNT(*) FROM parsed WHERE topic_id=?
	 OR (stellar_version=2 AND json_extract(live,'$.collection.profile')='topic' AND json_extract(live,'$.collection.id')=?)
	 OR (json_extract(draft,'$.stellar_version')=2 AND json_extract(pending,'$.collection.profile')='topic' AND json_extract(pending,'$.collection.id')=?)`, id, slug, slug).Scan(&n)
	return n, err
}

func (s *Store) CountAllByTopicID(topicID int64) (int, error) {
	var slug string
	if err := s.db.QueryRow(`SELECT slug FROM topics WHERE id=?`, topicID).Scan(&slug); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return topicMembershipCount(s.db, topicID, slug)
}
