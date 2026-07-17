// Package store 文章持久化层，基于 SQLite（modernc.org/sqlite 纯 Go 驱动，免 CGO）。
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

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
	Cover       string // 封面图 URL，空则列表卡片不渲染 .post-cover
	ContentMD   string // Markdown 原文
	ContentHTML string // goldmark 预渲染缓存
	Status      string // draft / published
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Views       int64 // 阅读量，article_banner #post-meta 展示

	// Tags/Categories 不在 posts 表列里（走 post_tags/post_categories 关联表），
	// 不入 postColumns。由 FillTagsCategories 或独立查询填充，零值为空切片。
	Tags       []string
	Categories []string
}

// Store 文章数据访问对象，持 *sql.DB 连接。
type Store struct {
	db *sql.DB
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
		content_md   TEXT NOT NULL DEFAULT '',
		content_html TEXT NOT NULL DEFAULT '',
		status       TEXT NOT NULL DEFAULT 'draft',
		views        INTEGER NOT NULL DEFAULT 0,
		created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_posts_status ON posts(status);
	CREATE INDEX IF NOT EXISTS idx_posts_slug ON posts(slug);
	CREATE TABLE IF NOT EXISTS tags (
		id   INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE
	);
	CREATE TABLE IF NOT EXISTS categories (
		id   INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE
	);
	CREATE TABLE IF NOT EXISTS post_tags (
		post_id INTEGER NOT NULL,
		tag_id  INTEGER NOT NULL,
		PRIMARY KEY (post_id, tag_id)
	);
	CREATE TABLE IF NOT EXISTS post_categories (
		post_id     INTEGER NOT NULL,
		category_id INTEGER NOT NULL,
		PRIMARY KEY (post_id, category_id)
	);`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	// 老库迁移：CREATE TABLE IF NOT EXISTS 不改已存在表结构，需补 cover 列。
	if err := s.ensureColumn("posts", "cover", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("migrate cover column: %w", err)
	}
	// 老库迁移：views 列（文章访问统计），幂等。
	if err := s.ensureColumn("posts", "views", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return fmt.Errorf("migrate views column: %w", err)
	}
	// FTS5 全文索引：external content 表，索引挂在 posts 行上避免数据重复。
	// 配 INSERT/UPDATE/DELETE 触发器同步索引，否则索引与 posts 表脱节。
	if err := s.createFTS(); err != nil {
		return fmt.Errorf("create fts: %w", err)
	}
	// 老库迁移：FTS 表后建，已存在的 posts 行不会自动入索引，补一次全量重建。
	if err := s.rebuildFTS(); err != nil {
		return fmt.Errorf("rebuild fts: %w", err)
	}
	return nil
}

// ensureColumn 检测 table 是否有 name 列，没有则 ADD COLUMN colDef。幂等。
func (s *Store) ensureColumn(table, name, colDef string) error {
	rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("pragma %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var cname, ctype string
		var notnull, pk int
		var dflt interface{}
		if err := rows.Scan(&cid, &cname, &ctype, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("scan table_info: %w", err)
		}
		if cname == name {
			return nil // 列已存在
		}
	}
	if _, err := s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, name, colDef)); err != nil {
		return fmt.Errorf("add column %s.%s: %w", table, name, err)
	}
	return nil
}

// createFTS 建 FTS5 虚拟表与同步触发器。external content 表索引 posts 的
// title/content_md/content_html，rowid 对齐 posts.id。幂等：虚拟表与触发器均 IF NOT EXISTS。
// tokenizer 用 trigram：按 3 字符滑窗切分，对英文和 3+ 字中文查询子串匹配友好
// （unicode61 把整段 CJK 当一个 token，中文几乎搜不到）。
func (s *Store) createFTS() error {
	const ddl = `CREATE VIRTUAL TABLE IF NOT EXISTS posts_fts USING fts5(
		title, content_md, content_html,
		content='posts', content_rowid='id',
		tokenize='trigram'
	);
	CREATE TRIGGER IF NOT EXISTS posts_fts_ai AFTER INSERT ON posts BEGIN
		INSERT INTO posts_fts(rowid, title, content_md, content_html)
		VALUES (new.id, new.title, new.content_md, new.content_html);
	END;
	CREATE TRIGGER IF NOT EXISTS posts_fts_ad AFTER DELETE ON posts BEGIN
		INSERT INTO posts_fts(posts_fts, rowid, title, content_md, content_html)
		VALUES ('delete', old.id, old.title, old.content_md, old.content_html);
	END;
	CREATE TRIGGER IF NOT EXISTS posts_fts_au AFTER UPDATE ON posts BEGIN
		INSERT INTO posts_fts(posts_fts, rowid, title, content_md, content_html)
		VALUES ('delete', old.id, old.title, old.content_md, old.content_html);
		INSERT INTO posts_fts(rowid, title, content_md, content_html)
		VALUES (new.id, new.title, new.content_md, new.content_html);
	END;`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create fts tables: %w", err)
	}
	return nil
}

// rebuildFTS 全量重建 FTS 索引。老库 FTS 表后建时已存在 posts 行不入索引，调一次 'rebuild' 命令补齐。
// 对空表是 no-op，对已同步表也只是重写一遍，幂等安全。
func (s *Store) rebuildFTS() error {
	if _, err := s.db.Exec(`INSERT INTO posts_fts(posts_fts) VALUES('rebuild')`); err != nil {
		return fmt.Errorf("rebuild fts: %w", err)
	}
	return nil
}

// Search 全文搜索已发布文章，支持分页，返回 (posts, total, error)。
// 查询策略按长度分流：
//   - >=3 字符：走 FTS5 trigram，按 BM25 rank 升序（相关度高的在前）
//   - <3 字符：trigram 按 3-gram 切分，短查询匹配不到任何 gram，降级走 LIKE
//     在 title/content_md 上模糊匹配，保证 "Go"/"CSS" 等短词可搜
//
// FTS5 MATCH 用双引号包裹整串做短语匹配，规避用户输入含 " * : 等特殊字符的语法错。
func (s *Store) Search(q string, page, size int) ([]*Post, int, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, 0, nil
	}
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	offset := (page - 1) * size

	if utf8.RuneCountInString(q) < 3 {
		return s.searchLike(q, page, size, offset)
	}
	return s.searchFTS(q, page, size, offset)
}

// searchFTS FTS5 全文搜索，按 BM25 rank 排序。q 已保证 >=3 字符。
func (s *Store) searchFTS(q string, page, size, offset int) ([]*Post, int, error) {
	phrase := `"` + strings.ReplaceAll(q, `"`, `""`) + `"`

	var total int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM posts_fts f JOIN posts p ON p.id = f.rowid
		 WHERE posts_fts MATCH ? AND p.status = ?`, phrase, StatusPublished,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count search %q: %w", q, err)
	}
	if total == 0 {
		return nil, 0, nil
	}

	rows, err := s.db.Query(fmt.Sprintf(`SELECT %s FROM posts_fts f
		JOIN posts p ON p.id = f.rowid
		WHERE posts_fts MATCH ? AND p.status = ?
		ORDER BY rank LIMIT ? OFFSET ?`, postColumnsPrefixed), phrase, StatusPublished, size, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("search %q: %w", q, err)
	}
	defer rows.Close()
	posts, err := scanPosts(rows)
	if err != nil {
		return nil, 0, err
	}
	return posts, total, nil
}

// searchLike LIKE 降级搜索，用于 <3 字符查询（trigram 盲区）。
// 在 title/content_md 上模糊匹配，按 created_at 倒序（无相关度概念，用时间兜底）。
// q 中的 SQL LIKE 通配符（% _ \）用 ESCAPE '\' 转义，避免搜 "%" 误匹配全部。
func (s *Store) searchLike(q string, page, size, offset int) ([]*Post, int, error) {
	like := "%" + escapeLike(q) + "%"
	const totalQ = `SELECT COUNT(*) FROM posts WHERE status = ? AND (title LIKE ? ESCAPE '\' OR content_md LIKE ? ESCAPE '\')`
	var total int
	if err := s.db.QueryRow(totalQ, StatusPublished, like, like).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count like search %q: %w", q, err)
	}
	if total == 0 {
		return nil, 0, nil
	}

	rows, err := s.db.Query(fmt.Sprintf(`SELECT %s FROM posts
		WHERE status = ? AND (title LIKE ? ESCAPE '\' OR content_md LIKE ? ESCAPE '\')
		ORDER BY datetime(created_at) DESC LIMIT ? OFFSET ?`, postColumns),
		StatusPublished, like, like, size, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("like search %q: %w", q, err)
	}
	defer rows.Close()
	posts, err := scanPosts(rows)
	if err != nil {
		return nil, 0, err
	}
	return posts, total, nil
}

// escapeLike 转义 SQL LIKE 通配符（% _ \），配合 ESCAPE '\' 使用。
// 用户搜 "%折扣%" 时把 % 转义成 \% 当字面量，而非"任意字符"通配符。
func escapeLike(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`%`, `\%`,
		`_`, `\_`,
	)
	return r.Replace(s)
}

// postColumns 是 posts 表的查询列，统一顺序，避免 Scan 错位。
// views 在 updated_at 之后，Scan 时对应 Post.Views。
const postColumns = `id, slug, title, excerpt, cover, content_md, content_html, status, created_at, updated_at, views`

// postColumnsPrefixed 是带 p. 前缀的查询列，用于 JOIN posts_fts 时消除列名歧义
// （title/content_md/content_html 在 posts 与 posts_fts 两表都存在）。
const postColumnsPrefixed = `p.id, p.slug, p.title, p.excerpt, p.cover, p.content_md, p.content_html, p.status, p.created_at, p.updated_at, p.views`

// List 返回全部文章，按 created_at 倒序（最新在前）。
func (s *Store) List() ([]*Post, error) {
	rows, err := s.db.Query(fmt.Sprintf(`SELECT %s FROM posts ORDER BY datetime(created_at) DESC`, postColumns))
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}
	defer rows.Close()
	return scanPosts(rows)
}

// ListPublished 返回已发布文章，按 created_at 倒序。
func (s *Store) ListPublished() ([]*Post, error) {
	rows, err := s.db.Query(fmt.Sprintf(`SELECT %s FROM posts WHERE status = ? ORDER BY datetime(created_at) DESC`, postColumns), StatusPublished)
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
	offset := (page - 1) * size
	rows, err := s.db.Query(fmt.Sprintf(`SELECT %s FROM posts WHERE status = ? ORDER BY datetime(created_at) DESC LIMIT ? OFFSET ?`, postColumns), StatusPublished, size, offset)
	if err != nil {
		return nil, fmt.Errorf("list published paged: %w", err)
	}
	defer rows.Close()
	return scanPosts(rows)
}

// CountPublished 返回已发布文章总数，用于首页分页计算。
func (s *Store) CountPublished() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM posts WHERE status = ?`, StatusPublished).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count published: %w", err)
	}
	return n, nil
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
	q := fmt.Sprintf(`SELECT %s FROM posts WHERE slug = ?`, postColumns)
	args := []interface{}{slug}
	if publishedOnly {
		q += ` AND status = ?`
		args = append(args, StatusPublished)
	}
	row := s.db.QueryRow(q, args...)
	p := &Post{}
	err := row.Scan(&p.ID, &p.Slug, &p.Title, &p.Excerpt, &p.Cover, &p.ContentMD, &p.ContentHTML, &p.Status, &p.CreatedAt, &p.UpdatedAt, &p.Views)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get post by slug %s: %w", slug, err)
	}
	return p, nil
}

// GetPrevNext 查当前文章的上下篇（已发布），按 created_at 语义对齐 Stellar read_next.ejs：
//
//	prev = 较新文章（created_at 严格大于当前的那篇里最近的一篇）
//	next = 较早文章（created_at 严格小于当前的那篇里最旧的一篇）
//
// 两者都过滤 status=published 并排除当前 id。无相邻时对应返回 nil（非 error）。
// 只 2 篇文章时：新篇 prev=nil、next=旧篇；旧篇 prev=新篇、next=nil。
//
// 注意：不用 datetime() 函数包裹，直接字符串比较。created_at 列存的是 modernc/sqlite
// 对 time.Time 序列化的 "2026-07-16 00:00:00 +0000 UTC" 格式，datetime() 解析这种带
// "+0000 UTC" 后缀的字符串会返回 NULL，导致 < NULL 永不命中。该格式年月日时分秒对齐、
// 字典序等价时间序，且参数 p.CreatedAt 走同一条 driver 序列化路径，格式必然一致。
func (s *Store) GetPrevNext(p *Post) (prev, next *Post, err error) {
	// prev：created_at > 当前，按 created_at 升序取第一条（最近的那篇）
	prev = &Post{}
	err = s.db.QueryRow(fmt.Sprintf(`SELECT %s FROM posts
		WHERE status = ? AND id != ? AND created_at > ?
		ORDER BY created_at ASC LIMIT 1`, postColumns),
		StatusPublished, p.ID, p.CreatedAt).
		Scan(&prev.ID, &prev.Slug, &prev.Title, &prev.Excerpt, &prev.Cover, &prev.ContentMD, &prev.ContentHTML, &prev.Status, &prev.CreatedAt, &prev.UpdatedAt, &prev.Views)
	if err == sql.ErrNoRows {
		prev = nil
	} else if err != nil {
		return nil, nil, fmt.Errorf("get prev of %s: %w", p.Slug, err)
	}

	// next：created_at < 当前，按 created_at 降序取第一条（最旧的那篇相邻）
	next = &Post{}
	err = s.db.QueryRow(fmt.Sprintf(`SELECT %s FROM posts
		WHERE status = ? AND id != ? AND created_at < ?
		ORDER BY created_at DESC LIMIT 1`, postColumns),
		StatusPublished, p.ID, p.CreatedAt).
		Scan(&next.ID, &next.Slug, &next.Title, &next.Excerpt, &next.Cover, &next.ContentMD, &next.ContentHTML, &next.Status, &next.CreatedAt, &next.UpdatedAt, &next.Views)
	if err == sql.ErrNoRows {
		next = nil
		return prev, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get next of %s: %w", p.Slug, err)
	}
	return prev, next, nil
}

// IncrViews 按 slug 自增文章阅读量。文章不存在时静默返回 nil（不报错，避免 /post/:slug
// 渲染流程因计数失败中断）。同步写：SQLite WAL + SetMaxOpenConns(1) 下写串行排队，
// 本地/低并发场景够用；高并发需改异步队列（本期不做）。
func (s *Store) IncrViews(slug string) error {
	_, err := s.db.Exec(`UPDATE posts SET views = views + 1 WHERE slug = ?`, slug)
	if err != nil {
		return fmt.Errorf("incr views %s: %w", slug, err)
	}
	return nil
}
func (s *Store) Create(p *Post) (int64, error) {
	if p.Status == "" {
		p.Status = StatusDraft
	}
	now := time.Now()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	res, err := s.db.Exec(`INSERT INTO posts (slug, title, excerpt, cover, content_md, content_html, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Slug, p.Title, p.Excerpt, p.Cover, p.ContentMD, p.ContentHTML, p.Status, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return 0, fmt.Errorf("create post %s: %w", p.Slug, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	p.ID = id
	return id, nil
}

// Update 按 slug 更新文章字段（title/excerpt/cover/content_md/content_html/status），刷新 updated_at。
func (s *Store) Update(slug string, p *Post) error {
	p.UpdatedAt = time.Now()
	res, err := s.db.Exec(`UPDATE posts SET title=?, excerpt=?, cover=?, content_md=?, content_html=?, status=?, updated_at=?
		WHERE slug=?`,
		p.Title, p.Excerpt, p.Cover, p.ContentMD, p.ContentHTML, p.Status, p.UpdatedAt, slug)
	if err != nil {
		return fmt.Errorf("update post %s: %w", slug, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("update post %s: no rows affected", slug)
	}
	return nil
}

// Delete 按 slug 删除文章。不存在不报错。
func (s *Store) Delete(slug string) error {
	_, err := s.db.Exec(`DELETE FROM posts WHERE slug=?`, slug)
	if err != nil {
		return fmt.Errorf("delete post %s: %w", slug, err)
	}
	return nil
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

// scanPosts 批量扫描 rows 到 []*Post。
func scanPosts(rows *sql.Rows) ([]*Post, error) {
	var out []*Post
	for rows.Next() {
		p := &Post{}
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Excerpt, &p.Cover, &p.ContentMD, &p.ContentHTML, &p.Status, &p.CreatedAt, &p.UpdatedAt, &p.Views); err != nil {
			return nil, fmt.Errorf("scan post: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate posts: %w", err)
	}
	return out, nil
}

// TagCount 标签云用：标签名 + 关联文章数。
type TagCount struct {
	Name  string
	Count int
}

// CategoryCount 分类云用：分类名 + 关联文章数。
type CategoryCount struct {
	Name  string
	Count int
}

// ArchiveYear 归档页用：某年 + 该年文章列表（按 created_at 倒序）。
type ArchiveYear struct {
	Year  int
	Posts []*Post
}

// SetTags 用事务替换 postID 的全部 tag 关联：删旧 + 插新（含 tags 表 upsert）。
// 显式事务，不依赖 SQLite foreign_keys pragma。空 tags 则仅清空关联。
func (s *Store) SetTags(postID int64, tags []string) error {
	return s.setRelations(postID, tags, "tags", "post_tags", "tag_id")
}

// SetCategories 用事务替换 postID 的全部 category 关联。同 SetTags。
func (s *Store) SetCategories(postID int64, cats []string) error {
	return s.setRelations(postID, cats, "categories", "post_categories", "category_id")
}

// setRelations 是 SetTags/SetCategories 的共用实现。
// table=tags|categories，relTable=post_tags|post_categories，relCol=tag_id|category_id。
func (s *Store) setRelations(postID int64, names []string, table, relTable, relCol string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() // 已提交时 Rollback 是 no-op

	if _, err := tx.Exec(fmt.Sprintf(`DELETE FROM %s WHERE post_id = ?`, relTable), postID); err != nil {
		return fmt.Errorf("delete old %s: %w", relTable, err)
	}

	seen := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		var id int64
		if err := tx.QueryRow(fmt.Sprintf(`INSERT INTO %s(name) VALUES(?) ON CONFLICT(name) DO UPDATE SET name=excluded.name RETURNING id`, table), name).Scan(&id); err != nil {
			return fmt.Errorf("upsert %s %q: %w", table, name, err)
		}
		if _, err := tx.Exec(fmt.Sprintf(`INSERT INTO %s(post_id, %s) VALUES(?, ?) ON CONFLICT DO NOTHING`, relTable, relCol), postID, id); err != nil {
			return fmt.Errorf("insert %s: %w", relTable, err)
		}
	}
	return tx.Commit()
}

// FillTagsCategories 批量填充 posts 的 Tags/Categories，避免 N+1。
// 各发一次 JOIN 查询拿全部关联，内存按 post_id 分组。
func (s *Store) FillTagsCategories(posts []*Post) error {
	if len(posts) == 0 {
		return nil
	}
	ids := make([]interface{}, len(posts))
	idToPost := make(map[int64]*Post, len(posts))
	for i, p := range posts {
		ids[i] = p.ID
		idToPost[p.ID] = p
		p.Tags = nil
		p.Categories = nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]

	q := fmt.Sprintf(`SELECT pt.post_id, t.name FROM post_tags pt JOIN tags t ON t.id = pt.tag_id WHERE pt.post_id IN (%s)`, placeholders)
	rows, err := s.db.Query(q, ids...)
	if err != nil {
		return fmt.Errorf("query post_tags: %w", err)
	}
	for rows.Next() {
		var pid int64
		var name string
		if err := rows.Scan(&pid, &name); err != nil {
			rows.Close()
			return fmt.Errorf("scan post_tags: %w", err)
		}
		if p := idToPost[pid]; p != nil {
			p.Tags = append(p.Tags, name)
		}
	}
	rows.Close()

	q = fmt.Sprintf(`SELECT pc.post_id, c.name FROM post_categories pc JOIN categories c ON c.id = pc.category_id WHERE pc.post_id IN (%s)`, placeholders)
	rows, err = s.db.Query(q, ids...)
	if err != nil {
		return fmt.Errorf("query post_categories: %w", err)
	}
	for rows.Next() {
		var pid int64
		var name string
		if err := rows.Scan(&pid, &name); err != nil {
			rows.Close()
			return fmt.Errorf("scan post_categories: %w", err)
		}
		if p := idToPost[pid]; p != nil {
			p.Categories = append(p.Categories, name)
		}
	}
	rows.Close()
	return nil
}

// ListTags 返回所有标签及文章数，按文章数倒序、名称升序。标签云用。
func (s *Store) ListTags() ([]TagCount, error) {
	rows, err := s.db.Query(`SELECT t.name, COUNT(pt.post_id) AS c
		FROM tags t LEFT JOIN post_tags pt ON pt.tag_id = t.id
		GROUP BY t.id ORDER BY c DESC, t.name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()
	var out []TagCount
	for rows.Next() {
		var tc TagCount
		if err := rows.Scan(&tc.Name, &tc.Count); err != nil {
			return nil, fmt.Errorf("scan tags: %w", err)
		}
		out = append(out, tc)
	}
	return out, rows.Err()
}

// ListCategories 返回所有分类及文章数，按名称升序。本期平铺无父子。
func (s *Store) ListCategories() ([]CategoryCount, error) {
	rows, err := s.db.Query(`SELECT c.name, COUNT(pc.post_id) AS c
		FROM categories c LEFT JOIN post_categories pc ON pc.category_id = c.id
		GROUP BY c.id ORDER BY c.name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()
	var out []CategoryCount
	for rows.Next() {
		var cc CategoryCount
		if err := rows.Scan(&cc.Name, &cc.Count); err != nil {
			return nil, fmt.Errorf("scan categories: %w", err)
		}
		out = append(out, cc)
	}
	return out, rows.Err()
}

// listByRelation 是 ListByTag/ListByCategory 的共用实现。
// table=tags|categories，relTable=post_tags|post_categories，relCol=tag_id|category_id。
// 只返回已发布文章，按 created_at 倒序，支持分页。
func (s *Store) listByRelation(name string, page, size int, table, relTable, relCol string) ([]*Post, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	offset := (page - 1) * size
	q := fmt.Sprintf(`SELECT %s FROM posts p
		WHERE p.status = ? AND p.id IN (
			SELECT pt.post_id FROM %s pt JOIN %s r ON r.id = pt.%s WHERE r.name = ?
		)
		ORDER BY datetime(p.created_at) DESC LIMIT ? OFFSET ?`, postColumns, relTable, table, relCol)
	rows, err := s.db.Query(q, StatusPublished, name, size, offset)
	if err != nil {
		return nil, fmt.Errorf("list by %s %q: %w", table, name, err)
	}
	defer rows.Close()
	return scanPosts(rows)
}

// countByRelation 是 CountByTag/CountByCategory 的共用实现。
func (s *Store) countByRelation(name string, table, relTable, relCol string) (int, error) {
	q := fmt.Sprintf(`SELECT COUNT(*) FROM posts p
		WHERE p.status = ? AND p.id IN (
			SELECT pt.post_id FROM %s pt JOIN %s r ON r.id = pt.%s WHERE r.name = ?
		)`, relTable, table, relCol)
	var n int
	err := s.db.QueryRow(q, StatusPublished, name).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count by %s %q: %w", table, name, err)
	}
	return n, nil
}

// ListByTag 返回含指定 tag 的已发布文章，按 created_at 倒序，支持分页。
func (s *Store) ListByTag(tag string, page, size int) ([]*Post, error) {
	return s.listByRelation(tag, page, size, "tags", "post_tags", "tag_id")
}

// CountByTag 返回含指定 tag 的已发布文章数。
func (s *Store) CountByTag(tag string) (int, error) {
	return s.countByRelation(tag, "tags", "post_tags", "tag_id")
}

// ListByCategory 返回含指定 category 的已发布文章，按 created_at 倒序，支持分页。
func (s *Store) ListByCategory(cat string, page, size int) ([]*Post, error) {
	return s.listByRelation(cat, page, size, "categories", "post_categories", "category_id")
}

// CountByCategory 返回含指定 category 的已发布文章数。
func (s *Store) CountByCategory(cat string) (int, error) {
	return s.countByRelation(cat, "categories", "post_categories", "category_id")
}

// Archive 返回全量已发布文章，按年分组，年内按 created_at 倒序。
func (s *Store) Archive() ([]ArchiveYear, error) {
	rows, err := s.db.Query(fmt.Sprintf(`SELECT %s FROM posts WHERE status = ? ORDER BY datetime(created_at) DESC`, postColumns), StatusPublished)
	if err != nil {
		return nil, fmt.Errorf("archive: %w", err)
	}
	defer rows.Close()
	posts, err := scanPosts(rows)
	if err != nil {
		return nil, err
	}
	var groups []ArchiveYear
	yearIdx := map[int]int{}
	for _, p := range posts {
		y := p.CreatedAt.Year()
		idx, ok := yearIdx[y]
		if !ok {
			yearIdx[y] = len(groups)
			groups = append(groups, ArchiveYear{Year: y})
			idx = len(groups) - 1
		}
		groups[idx].Posts = append(groups[idx].Posts, p)
	}
	return groups, nil
}
