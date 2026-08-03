// Package store 文章持久化层，基于 SQLite（modernc.org/sqlite 纯 Go 驱动，免 CGO）。
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite" // side-effect import，注册 "sqlite" driver 到 database/sql
)

// Status 文章状态。
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
)

// Post 文章持久化记录。对齐 posts 表字段。
type Post struct {
	ID          int64
	Slug        string
	Title       string
	Excerpt     string
	Cover       string // 封面图 URL，可空
	Category    string // 单分类，可空
	Tags        string // 多标签，逗号分隔存储（如 "Go,Gin"），可空
	ContentMD   string // Markdown 原文
	ContentHTML string // goldmark 预渲染缓存
	Status      string // draft / published
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Pinned      bool      // 置顶
	Password    string    // 阅读口令，空=无保护
	PublishAt   time.Time // 定时发布（零值=立即）；未到点不公开
	Poster      string    // 海报封面 JSON（PosterData），空=无海报；与 cover 同时存在时首页渲染 photo 大卡
	Topic       string    // 专栏名，空=普通文章；非空时文章页左栏显示同专栏系列列表，/navbar 出现「专栏」tab
	TopicID     int64     // 专栏 ID（Spec 52 独立 topics 表外键）：新代码统一走 topic_id，topic 文本仅作迁移源与兼容查询
	References  string    // 参考文献 JSON（[]string），空=无参考文献；文章页脚渲染用
	Banner      string    // 文首横幅图 URL（front matter banner），空=无横幅；文章页顶部渲染用
	DeletedAt   string    // 软删除时间 RFC3339，空串=未删除
	DraftJSON   string    // 草稿头 JSON（整份编辑表单），空=无未发布草稿；仅已发布文章使用

	// 文章级 SEO（Spec 42）：非空时覆盖站点级默认 meta 输出。
	MetaDescription string // 自定义 meta description，空=用自动摘要/站点默认
	MetaKeywords    string // 自定义 meta keywords，逗号分隔，空=用站点默认

	// 多作者体系（Spec 38）：作者 ID，空串=无作者（回退站长）。
	Author string
}

// PosterData 海报封面字段，对齐原版 post.poster（headline/topic/caption/color）。
type PosterData struct {
	Headline string `json:"headline"` // 海报主标题（photo 卡触发字段）
	Topic    string `json:"topic"`    // 专栏/系列名（非空时文字吸顶，否则沉底）
	Caption  string `json:"caption"`  // 副标题
	Color    string `json:"color"`    // 文字颜色（如 #fff）
}

// EncodePoster 序列化海报字段为 JSON 存储串；headline 为空视为无海报，返回空串。
func EncodePoster(p PosterData) string {
	if strings.TrimSpace(p.Headline) == "" {
		return ""
	}
	b, err := json.Marshal(p)
	if err != nil {
		return ""
	}
	return string(b)
}

// DecodePoster 反序列化 poster 列；空串/解析失败/headline 为空返回 nil。
func DecodePoster(s string) *PosterData {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var p PosterData
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return nil
	}
	if strings.TrimSpace(p.Headline) == "" {
		return nil
	}
	return &p
}

// EncodeReferences 序列化参考文献切片为 JSON 数组串；空切片返回空串。
func EncodeReferences(refs []string) string {
	if len(refs) == 0 {
		return ""
	}
	b, err := json.Marshal(refs)
	if err != nil {
		return ""
	}
	return string(b)
}

// DecodeReferences 反序列化 references 列；空串/解析失败返回 nil。
func DecodeReferences(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var refs []string
	if err := json.Unmarshal([]byte(s), &refs); err != nil {
		return nil
	}
	return refs
}

// postCols 统一 SELECT 列清单（含 Spec 15 增强列 + Spec 17 回收站列 + 草稿头列 + Spec 38 作者列 + Spec 52 topic_id 列）。
const postCols = "id, slug, title, excerpt, cover, category, tags, content_md, content_html, status, created_at, updated_at, pinned, password, publish_at, poster, topic, topic_id, \"references\", banner, deleted_at, draft_json, meta_description, meta_keywords, author"

// publishedWhere 已发布且定时已到且未删除。占位符绑定 StatusPublished。
const publishedWhere = "status = ? AND deleted_at = '' AND (publish_at IS NULL OR publish_at = '' OR publish_at <= strftime('%Y-%m-%dT%H:%M:%SZ','now'))"

// publishedOrder 排序用 substr 而非 datetime()：驱动把 time.Time 序列化为 "2006-01-02 15:04:05 -0700 MST"
// 格式，SQLite datetime() 无法解析（返回 NULL 导致排序失效）；前 19 位字典序对同表同时区数据即时间序。
const publishedOrder = "ORDER BY pinned DESC, substr(created_at, 1, 19) DESC"

func formatPublishAt(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

func parsePublishAt(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02 15:04:05", "2006-01-02T15:04"} {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// IsPasswordProtected 是否需要阅读口令。
func (p *Post) IsPasswordProtected() bool {
	return p != nil && strings.TrimSpace(p.Password) != ""
}

// Store 文章数据访问对象，持 *sql.DB 连接。
type Store struct {
	db *sql.DB
	// postWriteHook 文章写路径回调（Spec 25 FTS 同步用），op 取 create/update/delete/restore/purge。
	// 写操作成功后同步调用；回调内部自行容错，不影响主流程。
	postWriteHook func(op, slug string)

	// 评论点赞 schema 懒迁移状态（逐 Store 实例，避免包级 Once 在多库实例下漏迁移）。
	commentReactionsOnce sync.Once
	commentReactionsErr  error
}

// SetPostWriteHook 注册文章写路径回调（main.go 装配）。
func (s *Store) SetPostWriteHook(h func(op, slug string)) {
	s.postWriteHook = h
}

// firePostWrite 触发写路径回调（未注册则静默）。
func (s *Store) firePostWrite(op, slug string) {
	if s.postWriteHook != nil {
		s.postWriteHook(op, slug)
	}
}

// New 打开 SQLite 连接并建表。dbPath 是 .db 文件绝对路径，父目录自动创建。
// SQLite 写并发受限，SetMaxOpenConns(1) 避免 "database is locked"。
func New(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir db dir %s: %w", dir, err)
	}

	// 用 file: DSN 启用 WAL 与忙等待，提升并发读 + 避免短暂锁冲突
	dsn := "file:" + dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", dbPath, err)
	}
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &Store{db: db}
	if err := s.createTables(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close 关闭底层数据库连接。
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) createTables() error {
	const ddl = `CREATE TABLE IF NOT EXISTS posts (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		slug         TEXT NOT NULL UNIQUE,
		title        TEXT NOT NULL DEFAULT '',
		excerpt      TEXT NOT NULL DEFAULT '',
		cover        TEXT NOT NULL DEFAULT '',
		category     TEXT NOT NULL DEFAULT '',
		tags         TEXT NOT NULL DEFAULT '',
		content_md   TEXT NOT NULL DEFAULT '',
		content_html TEXT NOT NULL DEFAULT '',
		status       TEXT NOT NULL DEFAULT 'draft',
		created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_posts_status ON posts(status);
	CREATE INDEX IF NOT EXISTS idx_posts_slug ON posts(slug);
	CREATE INDEX IF NOT EXISTS idx_posts_category ON posts(category);`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	// comments 表：自建评论系统（参考 Artalk 的 page_key 定位 + 两层嵌套模型）。
	// parent_id/root_id 0=顶级评论；status: approved/pending；is_admin: 0/1。
	const commentDDL = `CREATE TABLE IF NOT EXISTS comments (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		page_key     TEXT NOT NULL DEFAULT '',
		nick         TEXT NOT NULL DEFAULT '',
		email        TEXT NOT NULL DEFAULT '',
		link         TEXT NOT NULL DEFAULT '',
		content_md   TEXT NOT NULL DEFAULT '',
		content_html TEXT NOT NULL DEFAULT '',
		parent_id    INTEGER NOT NULL DEFAULT 0,
		root_id      INTEGER NOT NULL DEFAULT 0,
		status       TEXT NOT NULL DEFAULT 'pending',
		is_admin     INTEGER NOT NULL DEFAULT 0,
		ip           TEXT NOT NULL DEFAULT '',
		ua           TEXT NOT NULL DEFAULT '',
		created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_comments_page ON comments(page_key, status);
	CREATE INDEX IF NOT EXISTS idx_comments_root ON comments(root_id);`
	if _, err := s.db.Exec(commentDDL); err != nil {
		return fmt.Errorf("create comments table: %w", err)
	}
	// 老库迁移：CREATE TABLE IF NOT EXISTS 不会给已存在的表加列，需检测后 ALTER。
	return s.migrateColumns()
}

// migrateColumns 检测 posts 表缺失的列并 ALTER TABLE 补上（兼容老库）。
// SQLite 的 ADD COLUMN 要求默认值是常量（DEFAULT ” 合法）。
func (s *Store) migrateColumns() error {
	want := map[string]string{
		"cover":      `ALTER TABLE posts ADD COLUMN cover TEXT NOT NULL DEFAULT ''`,
		"category":   `ALTER TABLE posts ADD COLUMN category TEXT NOT NULL DEFAULT ''`,
		"tags":       `ALTER TABLE posts ADD COLUMN tags TEXT NOT NULL DEFAULT ''`,
		"views":      `ALTER TABLE posts ADD COLUMN views INTEGER NOT NULL DEFAULT 0`,
		"pinned":     `ALTER TABLE posts ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0`,
		"password":   `ALTER TABLE posts ADD COLUMN password TEXT NOT NULL DEFAULT ''`,
		"publish_at": `ALTER TABLE posts ADD COLUMN publish_at TEXT NOT NULL DEFAULT ''`,
		"poster":     `ALTER TABLE posts ADD COLUMN poster TEXT NOT NULL DEFAULT ''`,
		"topic":      `ALTER TABLE posts ADD COLUMN topic TEXT NOT NULL DEFAULT ''`,
		// Spec 52 专栏关联列（与 topics 表同步迁移；对齐 Spec 52「走现有 migrate 模式」决策）
		"topic_id": `ALTER TABLE posts ADD COLUMN topic_id INTEGER NOT NULL DEFAULT 0`,
		"references": `ALTER TABLE posts ADD COLUMN "references" TEXT NOT NULL DEFAULT ''`,
		"banner":     `ALTER TABLE posts ADD COLUMN banner TEXT NOT NULL DEFAULT ''`,
		"deleted_at": `ALTER TABLE posts ADD COLUMN deleted_at TEXT NOT NULL DEFAULT ''`,
		"draft_json": `ALTER TABLE posts ADD COLUMN draft_json TEXT NOT NULL DEFAULT ''`,
		// Spec 42 文章级 SEO 两列（幂等 ALTER，常量默认值）
		"meta_description": `ALTER TABLE posts ADD COLUMN meta_description TEXT NOT NULL DEFAULT ''`,
		"meta_keywords":    `ALTER TABLE posts ADD COLUMN meta_keywords TEXT NOT NULL DEFAULT ''`,
		// Spec 38 多作者体系
		"author": `ALTER TABLE posts ADD COLUMN author TEXT NOT NULL DEFAULT ''`,
	}
	rows, err := s.db.Query(`PRAGMA table_info(posts)`)
	if err != nil {
		return fmt.Errorf("pragma table_info: %w", err)
	}
	defer rows.Close()
	existing := map[string]bool{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("scan table_info: %w", err)
		}
		existing[name] = true
	}
	for col, alter := range want {
		if existing[col] {
			continue // 列已存在，无需迁移
		}
		if _, err := s.db.Exec(alter); err != nil {
			return fmt.Errorf("alter table add %s: %w", col, err)
		}
	}
	// P2-16：(status, publish_at) 复合索引加速 publishedWhere 过滤（文章上千后有感）。
	// 依赖迁移列 publish_at，须在列补齐后创建，故不放初始 DDL。
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_posts_status_publish ON posts(status, publish_at)`); err != nil {
		return fmt.Errorf("create idx_posts_status_publish: %w", err)
	}
	return nil
}

// List 返回全部未删除文章，按 created_at 倒序（最新在前）。
func (s *Store) List() ([]*Post, error) {
	rows, err := s.db.Query("SELECT " + postCols + " FROM posts WHERE deleted_at = '' ORDER BY pinned DESC, substr(created_at, 1, 19) DESC")
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}
	defer rows.Close()
	return scanPosts(rows)
}

// ListPublished 返回已发布文章，按 created_at 倒序。
func (s *Store) ListPublished() ([]*Post, error) {
	rows, err := s.db.Query("SELECT "+postCols+" FROM posts WHERE "+publishedWhere+" "+publishedOrder, StatusPublished)
	if err != nil {
		return nil, fmt.Errorf("list published posts: %w", err)
	}
	defer rows.Close()
	return scanPosts(rows)
}

// ListPublishedPaged 分页返回已发布文章，按 created_at 倒序。page 从 1 开始。
func (s *Store) ListPublishedPaged(page, size int) ([]*Post, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	rows, err := s.db.Query("SELECT "+postCols+" FROM posts WHERE "+publishedWhere+" "+publishedOrder+" LIMIT ? OFFSET ?",
		StatusPublished, size, (page-1)*size)
	if err != nil {
		return nil, fmt.Errorf("list published posts paged: %w", err)
	}
	defer rows.Close()
	return scanPosts(rows)
}

// CountPublished 返回已发布文章总数（分页器用）。
func (s *Store) CountPublished() (int, error) {
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM posts WHERE "+publishedWhere, StatusPublished).Scan(&n); err != nil {
		return 0, fmt.Errorf("count published posts: %w", err)
	}
	return n, nil
}

// ListByCategory 返回指定分类的已发布文章，按 created_at 倒序。
func (s *Store) ListByCategory(name string) ([]*Post, error) {
	rows, err := s.db.Query("SELECT "+postCols+" FROM posts WHERE "+publishedWhere+" AND category = ? "+publishedOrder, StatusPublished, name)
	if err != nil {
		return nil, fmt.Errorf("list posts by category %s: %w", name, err)
	}
	defer rows.Close()
	return scanPosts(rows)
}

// ListByTag 返回包含指定标签的已发布文章，按 created_at 倒序。
// tags 逗号分隔存储，用逗号包裹 LIKE 防子串误匹配（如 Go 匹配到 Golang）。
func (s *Store) ListByTag(name string) ([]*Post, error) {
	rows, err := s.db.Query("SELECT "+postCols+" FROM posts WHERE "+publishedWhere+" AND (',' || tags || ',') LIKE ? "+publishedOrder,
		StatusPublished, "%,"+name+",%")
	if err != nil {
		return nil, fmt.Errorf("list posts by tag %s: %w", name, err)
	}
	defer rows.Close()
	return scanPosts(rows)
}

// CountByCategory 统计各分类的已发布文章数（空分类除外）。
func (s *Store) CountByCategory() (map[string]int, error) {
	rows, err := s.db.Query("SELECT category, COUNT(*) FROM posts WHERE "+publishedWhere+" AND category != '' GROUP BY category", StatusPublished)
	if err != nil {
		return nil, fmt.Errorf("count by category: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var name string
		var n int
		if err := rows.Scan(&name, &n); err != nil {
			return nil, fmt.Errorf("scan category count: %w", err)
		}
		out[name] = n
	}
	return out, rows.Err()
}

// ListByTopic 返回指定专栏的已发布文章，按 created_at 倒序。
func (s *Store) ListByTopic(name string) ([]*Post, error) {
	rows, err := s.db.Query("SELECT "+postCols+" FROM posts WHERE "+publishedWhere+" AND topic = ? "+publishedOrder, StatusPublished, name)
	if err != nil {
		return nil, fmt.Errorf("list posts by topic %s: %w", name, err)
	}
	defer rows.Close()
	return scanPosts(rows)
}

// ListPostsByTopicID 返回指定专栏 ID 的已发布文章（Spec 52），按发布时间倒序（对齐 publishedOrder）。
// 新代码统一走 topic_id 关联，替代按 topic 文本查询的 ListByTopic。
func (s *Store) ListPostsByTopicID(topicID int64) ([]*Post, error) {
	rows, err := s.db.Query("SELECT "+postCols+" FROM posts WHERE "+publishedWhere+" AND topic_id = ? "+publishedOrder, StatusPublished, topicID)
	if err != nil {
		return nil, fmt.Errorf("list posts by topic id %d: %w", topicID, err)
	}
	defer rows.Close()
	return scanPosts(rows)
}

// CountByTopic 统计各专栏的已发布文章数（空专栏除外）。
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

// CountByTag 统计各标签的已发布文章数。tags 逗号分隔存储，内存拆分计数（跳过空值）。
func (s *Store) CountByTag() (map[string]int, error) {
	rows, err := s.db.Query("SELECT tags FROM posts WHERE "+publishedWhere+" AND tags != ''", StatusPublished)
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var tags string
		if err := rows.Scan(&tags); err != nil {
			return nil, fmt.Errorf("scan tags: %w", err)
		}
		for _, t := range strings.Split(tags, ",") {
			if t = strings.TrimSpace(t); t != "" {
				out[t]++
			}
		}
	}
	return out, rows.Err()
}

// GetBySlug 按 slug 查单篇（任意状态）。未找到返回 nil, nil。
func (s *Store) GetBySlug(slug string) (*Post, error) {
	return s.getBySlug(slug, false)
}

// GetPublishedBySlug 按 slug 查已发布文章。未找到返回 nil, nil。
func (s *Store) GetPublishedBySlug(slug string) (*Post, error) {
	return s.getBySlug(slug, true)
}

func (s *Store) getBySlug(slug string, publishedOnly bool) (*Post, error) {
	q := "SELECT " + postCols + " FROM posts WHERE slug = ?"
	args := []interface{}{slug}
	if publishedOnly {
		q += " AND " + publishedWhere
		// publishedWhere 以 status=? 开头，需再绑 StatusPublished
		args = append(args, StatusPublished)
	}
	row := s.db.QueryRow(q, args...)
	p, err := scanOne(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get post by slug %s: %w", slug, err)
	}
	return p, nil
}

// GetPrevPost 按 created_at 查前一篇已发布文章（时间更早的最近一篇）。
func (s *Store) GetPrevPost(slug string) (*Post, error) {
	q := "SELECT " + postCols + " FROM posts WHERE " + publishedWhere + " AND substr(created_at, 1, 19) < (SELECT substr(created_at, 1, 19) FROM posts WHERE slug = ?) ORDER BY substr(created_at, 1, 19) DESC, id DESC LIMIT 1"
	row := s.db.QueryRow(q, StatusPublished, slug)
	p, err := scanOne(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get prev post for %s: %w", slug, err)
	}
	return p, nil
}

// GetNextPost 按 created_at 查后一篇已发布文章（时间更晚的最近一篇）。
func (s *Store) GetNextPost(slug string) (*Post, error) {
	q := "SELECT " + postCols + " FROM posts WHERE " + publishedWhere + " AND substr(created_at, 1, 19) > (SELECT substr(created_at, 1, 19) FROM posts WHERE slug = ?) ORDER BY substr(created_at, 1, 19) ASC, id ASC LIMIT 1"
	row := s.db.QueryRow(q, StatusPublished, slug)
	p, err := scanOne(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get next post for %s: %w", slug, err)
	}
	return p, nil
}

// ListRelatedByTags 按标签匹配相关文章，排除自身，最多 limit 篇。
func (s *Store) ListRelatedByTags(slug string, tags string, limit int) ([]*Post, error) {
	tagList := strings.Split(tags, ",")
	var validTags []string
	for _, t := range tagList {
		if t = strings.TrimSpace(t); t != "" {
			validTags = append(validTags, t)
		}
	}
	if len(validTags) == 0 {
		return nil, nil
	}

	seen := map[string]bool{}
	var result []*Post
	for _, tag := range validTags {
		rows, err := s.db.Query("SELECT "+postCols+" FROM posts WHERE "+publishedWhere+" AND slug != ? AND (',' || tags || ',') LIKE ? "+publishedOrder,
			StatusPublished, slug, "%,"+tag+",%")
		if err != nil {
			return nil, fmt.Errorf("query related posts by tag %s: %w", tag, err)
		}
		posts, err := scanPosts(rows)
		rows.Close()
		if err != nil {
			return nil, err
		}
		for _, p := range posts {
			if !seen[p.Slug] {
				seen[p.Slug] = true
				result = append(result, p)
				if len(result) >= limit {
					return result, nil
				}
			}
		}
	}
	return result, nil
}

// Create 新建文章，返回新记录 id。slug 冲突时返回错误。
func (s *Store) Create(p *Post) (int64, error) {
	if p.Status == "" {
		p.Status = StatusDraft
	}
	// UTC 写入：与 seed 导入数据时区一致，保证 substr 字典序排序在同表内等价于时间序。
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	pinned := 0
	if p.Pinned {
		pinned = 1
	}
	res, err := s.db.Exec(`INSERT INTO posts (slug, title, excerpt, cover, category, tags, content_md, content_html, status, created_at, updated_at, pinned, password, publish_at, poster, topic, topic_id, "references", banner, meta_description, meta_keywords, author)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Slug, p.Title, p.Excerpt, p.Cover, p.Category, p.Tags, p.ContentMD, p.ContentHTML, p.Status, p.CreatedAt, p.UpdatedAt, pinned, p.Password, formatPublishAt(p.PublishAt), p.Poster, p.Topic, p.TopicID, p.References, p.Banner, p.MetaDescription, p.MetaKeywords, p.Author)
	if err != nil {
		return 0, fmt.Errorf("create post %s: %w", p.Slug, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	p.ID = id
	s.firePostWrite("create", p.Slug)
	return id, nil
}

// Update 按 slug 更新文章字段（title/excerpt/cover/category/tags/content_md/content_html/status/poster），刷新 updated_at。
// draft_json 不在更新列中（ADD COLUMN 默认值兼容），由 SaveDraft/ClearDraft 单独维护。
// topic_id 直接写 p.TopicID（Spec 52）：调用方负责传正确的关联值——API 层在 savePostWithSnapshot 中
// 保留 existing.TopicID（请求体不携带 topic_id），seed 导入显式按专栏名解析；0 表示解除关联。
func (s *Store) Update(slug string, p *Post) error {
	p.UpdatedAt = time.Now()
	pinned := 0
	if p.Pinned {
		pinned = 1
	}
	res, err := s.db.Exec(`UPDATE posts SET title=?, excerpt=?, cover=?, category=?, tags=?, content_md=?, content_html=?, status=?, updated_at=?, pinned=?, password=?, publish_at=?, poster=COALESCE((SELECT poster FROM posts WHERE slug=?), ''), topic=?, topic_id=?, "references"=COALESCE((SELECT "references" FROM posts WHERE slug=?), ''), banner=COALESCE((SELECT banner FROM posts WHERE slug=?), ''), meta_description=?, meta_keywords=?, author=?
		WHERE slug=?`,
		p.Title, p.Excerpt, p.Cover, p.Category, p.Tags, p.ContentMD, p.ContentHTML, p.Status, p.UpdatedAt, pinned, p.Password, formatPublishAt(p.PublishAt), slug, p.Topic, p.TopicID, slug, slug, p.MetaDescription, p.MetaKeywords, p.Author, slug)
	if err != nil {
		return fmt.Errorf("update post %s: %w", slug, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("update post %s: no rows affected", slug)
	}
	s.firePostWrite("update", slug)
	return nil
}

// SaveDraft 写入草稿头 JSON（不碰线上内容/updated_at）。文章不存在返回错误。
func (s *Store) SaveDraft(slug, draftJSON string) error {
	res, err := s.db.Exec(`UPDATE posts SET draft_json = ? WHERE slug = ? AND deleted_at = ''`, draftJSON, slug)
	if err != nil {
		return fmt.Errorf("save draft for %s: %w", slug, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("save draft for %s: no rows affected", slug)
	}
	return nil
}

// ClearDraft 清空草稿头（发布应用或放弃草稿后调用）。文章不存在静默。
func (s *Store) ClearDraft(slug string) error {
	if _, err := s.db.Exec(`UPDATE posts SET draft_json = '' WHERE slug = ?`, slug); err != nil {
		return fmt.Errorf("clear draft for %s: %w", slug, err)
	}
	return nil
}

// Delete 软删除：标记 deleted_at，文章进入回收站。不存在不报错。
func (s *Store) Delete(slug string) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	_, err := s.db.Exec(`UPDATE posts SET deleted_at = ? WHERE slug = ? AND deleted_at = ''`, now, slug)
	if err != nil {
		return fmt.Errorf("soft delete post %s: %w", slug, err)
	}
	s.firePostWrite("delete", slug)
	return nil
}

// ListTrashed 返回回收站中的文章（deleted_at 非空），按删除时间倒序。
func (s *Store) ListTrashed() ([]*Post, error) {
	rows, err := s.db.Query("SELECT " + postCols + " FROM posts WHERE deleted_at != '' ORDER BY datetime(deleted_at) DESC")
	if err != nil {
		return nil, fmt.Errorf("list trashed posts: %w", err)
	}
	defer rows.Close()
	return scanPosts(rows)
}

// RestorePost 从回收站恢复文章（清空 deleted_at）。
func (s *Store) RestorePost(slug string) error {
	_, err := s.db.Exec(`UPDATE posts SET deleted_at = '' WHERE slug = ? AND deleted_at != ''`, slug)
	if err != nil {
		return fmt.Errorf("restore post %s: %w", slug, err)
	}
	s.firePostWrite("restore", slug)
	return nil
}

// PurgePost 彻底删除文章（物理删除记录）。
func (s *Store) PurgePost(slug string) error {
	_, err := s.db.Exec(`DELETE FROM posts WHERE slug = ? AND deleted_at != ''`, slug)
	if err != nil {
		return fmt.Errorf("purge post %s: %w", slug, err)
	}
	s.firePostWrite("purge", slug)
	return nil
}

// IncrViews 把指定 slug 文章的阅读量 +1。文章不存在时静默（0 行受影响不报错）。
// 用原子 UPDATE 自增，避免读-改-写竞态；SetMaxOpenConns(1) 已串行化写入。
func (s *Store) IncrViews(slug string) error {
	if _, err := s.db.Exec(`UPDATE posts SET views = views + 1 WHERE slug = ?`, slug); err != nil {
		return fmt.Errorf("incr views for %s: %w", slug, err)
	}
	return nil
}

// SumViews 返回全部文章阅读量之和。
func (s *Store) SumViews() (int64, error) {
	var total int64
	if err := s.db.QueryRow(`SELECT COALESCE(SUM(views), 0) FROM posts`).Scan(&total); err != nil {
		return 0, fmt.Errorf("sum views: %w", err)
	}
	return total, nil
}

// GetViews 返回指定 slug 文章的当前阅读量。文章不存在返回 0（不报错）。
func (s *Store) GetViews(slug string) (int64, error) {
	var views int64
	err := s.db.QueryRow(`SELECT views FROM posts WHERE slug = ?`, slug).Scan(&views)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get views for %s: %w", slug, err)
	}
	return views, nil
}

// Upsert 按 slug 存在性决定 Create 或 Update，seed 导入用。content_html 由调用方渲染好传入。
func (s *Store) Upsert(p *Post) error {
	existing, err := s.GetBySlug(p.Slug)
	if err != nil {
		return err
	}
	if existing == nil {
		_, err = s.Create(p)
		return err
	}
	// 保留原 created_at，仅更新内容字段
	p.ID = existing.ID
	p.CreatedAt = existing.CreatedAt
	return s.Update(p.Slug, p)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanOne(row rowScanner) (*Post, error) {
	p := &Post{}
	var pinned int
	var password, publishAt, deletedAt string
	err := row.Scan(&p.ID, &p.Slug, &p.Title, &p.Excerpt, &p.Cover, &p.Category, &p.Tags, &p.ContentMD, &p.ContentHTML, &p.Status, &p.CreatedAt, &p.UpdatedAt, &pinned, &password, &publishAt, &p.Poster, &p.Topic, &p.TopicID, &p.References, &p.Banner, &deletedAt, &p.DraftJSON, &p.MetaDescription, &p.MetaKeywords, &p.Author)
	if err != nil {
		return nil, err
	}
	p.Pinned = pinned != 0
	p.Password = password
	p.PublishAt = parsePublishAt(publishAt)
	p.DeletedAt = deletedAt
	return p, nil
}

// scanPosts 批量扫描 rows 到 []*Post。
func scanPosts(rows *sql.Rows) ([]*Post, error) {
	var out []*Post
	for rows.Next() {
		p, err := scanOne(rows)
		if err != nil {
			return nil, fmt.Errorf("scan post: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate posts: %w", err)
	}
	return out, nil
}
