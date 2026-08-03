package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// 评论状态。
const (
	CommentApproved = "approved"
	CommentPending  = "pending"
)

// ErrCommentNotFound 评论不存在（点赞等写操作用）。
var ErrCommentNotFound = errors.New("comment not found")

// Comment 评论持久化记录，对齐 comments 表字段。
// 两层嵌套模型（参考 Artalk）：parent_id 指向被回复评论，root_id 指向所属顶级评论；
// 顶级评论两者皆为 0。回复子评论时 root_id 沿用被回复评论的 root_id，层级不超两层。
type Comment struct {
	ID          int64
	PageKey     string // 页面定位键，如 /post/{slug}
	Nick        string
	Email       string // 不公开，仅 Gravatar 与邮件通知用
	Link        string // 个人网站，可空
	ContentMD   string // Markdown 原文
	ContentHTML string // goldmark 渲染 + bluemonday 消毒后的 HTML
	ParentID    int64  // 0=顶级评论，否则为被回复评论 id
	RootID      int64  // 0=顶级评论，否则为所属顶级评论 id
	Status      string // approved / pending
	IsAdmin     bool   // 博主回复标识（is_admin 列 0/1）
	IP          string
	UA          string
	Likes       int64 // 点赞计数（IP 去重后累加）
	CreatedAt   time.Time
}

// commentCols comments 表全列，各查询共用，保证与 scanComments 的扫描顺序一致。
const commentCols = `id, page_key, nick, email, link, content_md, content_html, parent_id, root_id, status, is_admin, ip, ua, likes, created_at`

// 评论点赞 schema 迁移：不碰 post.go createTables，在首次评论读写时懒执行。
// 迁移状态（sync.Once + err）挂在 Store 实例上，见 post.go 的 Store 定义。

// ensureCommentReactions 为 comments 补 likes 列，并建 comment_likes 去重表。
func (s *Store) ensureCommentReactions() error {
	s.commentReactionsOnce.Do(func() {
		// 老库迁移：CREATE TABLE IF NOT EXISTS 不会给已存在的表加列。
		rows, err := s.db.Query(`PRAGMA table_info(comments)`)
		if err != nil {
			s.commentReactionsErr = fmt.Errorf("pragma table_info(comments): %w", err)
			return
		}
		hasLikes := false
		for rows.Next() {
			var cid, notnull, pk int
			var name, ctype string
			var dflt sql.NullString
			if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
				rows.Close()
				s.commentReactionsErr = fmt.Errorf("scan table_info(comments): %w", err)
				return
			}
			if name == "likes" {
				hasLikes = true
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			s.commentReactionsErr = fmt.Errorf("iterate table_info(comments): %w", err)
			return
		}
		if !hasLikes {
			if _, err := s.db.Exec(`ALTER TABLE comments ADD COLUMN likes INTEGER NOT NULL DEFAULT 0`); err != nil {
				s.commentReactionsErr = fmt.Errorf("alter comments add likes: %w", err)
				return
			}
		}
		const likesDDL = `CREATE TABLE IF NOT EXISTS comment_likes (
			comment_id INTEGER NOT NULL,
			ip_hash    TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(comment_id, ip_hash)
		)`
		if _, err := s.db.Exec(likesDDL); err != nil {
			s.commentReactionsErr = fmt.Errorf("create comment_likes table: %w", err)
			return
		}
		// 评论区按页查询（ListCommentsByPage/CountCommentsByPage）的走索引优化，
		// 幂等建索引，老库升级安全。
		if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_comments_page ON comments(page_key)`); err != nil {
			s.commentReactionsErr = fmt.Errorf("create index comments page_key: %w", err)
			return
		}
	})
	return s.commentReactionsErr
}

// ListCommentsByPage 返回指定页面的已发布评论，按时间正序（先发的在前，评论区展示序）。
func (s *Store) ListCommentsByPage(pageKey string) ([]*Comment, error) {
	if err := s.ensureCommentReactions(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT `+commentCols+` FROM comments
		WHERE page_key = ? AND status = ? ORDER BY substr(created_at, 1, 19) ASC, id ASC`, pageKey, CommentApproved)
	if err != nil {
		return nil, fmt.Errorf("list comments by page %s: %w", pageKey, err)
	}
	defer rows.Close()
	return scanComments(rows)
}

// ListRecentComments 返回最新 n 条已发布评论（侧边栏 widget 用），按时间倒序。
func (s *Store) ListRecentComments(n int) ([]*Comment, error) {
	if err := s.ensureCommentReactions(); err != nil {
		return nil, err
	}
	if n < 1 {
		n = 5
	}
	rows, err := s.db.Query(`SELECT `+commentCols+` FROM comments
		WHERE status = ? ORDER BY substr(created_at, 1, 19) DESC, id DESC LIMIT ?`, CommentApproved, n)
	if err != nil {
		return nil, fmt.Errorf("list recent comments: %w", err)
	}
	defer rows.Close()
	return scanComments(rows)
}

// ListComments 管理页分页查询，按时间倒序。status 为空查全部，否则按状态过滤。page 从 1 开始。
func (s *Store) ListComments(status string, page, size int) ([]*Comment, error) {
	if err := s.ensureCommentReactions(); err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	q := `SELECT ` + commentCols + ` FROM comments`
	args := []interface{}{}
	if status != "" {
		q += ` WHERE status = ?`
		args = append(args, status)
	}
	q += ` ORDER BY substr(created_at, 1, 19) DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, size, (page-1)*size)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list comments (status=%q page=%d): %w", status, page, err)
	}
	defer rows.Close()
	return scanComments(rows)
}

// CountComments 管理页分页总数（与 ListComments 同过滤条件）。
func (s *Store) CountComments(status string) (int, error) {
	q := `SELECT COUNT(*) FROM comments`
	args := []interface{}{}
	if status != "" {
		q += ` WHERE status = ?`
		args = append(args, status)
	}
	var n int
	if err := s.db.QueryRow(q, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("count comments (status=%q): %w", status, err)
	}
	return n, nil
}

// CountCommentsByPage 返回指定页面的已发布评论数（评论区"N 条评论"计数用）。
func (s *Store) CountCommentsByPage(pageKey string) (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM comments WHERE page_key = ? AND status = ?`,
		pageKey, CommentApproved).Scan(&n); err != nil {
		return 0, fmt.Errorf("count comments by page %s: %w", pageKey, err)
	}
	return n, nil
}

// GetComment 按 id 查单条评论。未找到返回 nil, nil。
func (s *Store) GetComment(id int64) (*Comment, error) {
	if err := s.ensureCommentReactions(); err != nil {
		return nil, err
	}
	row := s.db.QueryRow(`SELECT `+commentCols+` FROM comments WHERE id = ?`, id)
	c := &Comment{}
	var isAdmin int
	err := row.Scan(&c.ID, &c.PageKey, &c.Nick, &c.Email, &c.Link, &c.ContentMD, &c.ContentHTML, &c.ParentID, &c.RootID, &c.Status, &isAdmin, &c.IP, &c.UA, &c.Likes, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get comment %d: %w", id, err)
	}
	c.IsAdmin = isAdmin != 0
	return c, nil
}

// LikeComment 对评论点赞（按 ipHash 去重）。插入成功则 likes+1 并 liked=true；
// 重复点赞返回当前计数、liked=false。评论不存在返回 ErrCommentNotFound。
func (s *Store) LikeComment(id int64, ipHash string) (liked bool, likes int64, err error) {
	if err := s.ensureCommentReactions(); err != nil {
		return false, 0, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, 0, fmt.Errorf("begin like comment %d: %w", id, err)
	}
	defer tx.Rollback()

	err = tx.QueryRow(`SELECT likes FROM comments WHERE id = ?`, id).Scan(&likes)
	if err == sql.ErrNoRows {
		return false, 0, ErrCommentNotFound
	}
	if err != nil {
		return false, 0, fmt.Errorf("select comment %d likes: %w", id, err)
	}

	res, err := tx.Exec(`INSERT OR IGNORE INTO comment_likes (comment_id, ip_hash, created_at) VALUES (?, ?, ?)`,
		id, ipHash, time.Now())
	if err != nil {
		return false, 0, fmt.Errorf("insert comment_likes %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, 0, fmt.Errorf("comment_likes rows affected: %w", err)
	}
	if n == 0 {
		// 已点过：不增不减，返回当前计数
		if err := tx.Commit(); err != nil {
			return false, 0, fmt.Errorf("commit like comment %d: %w", id, err)
		}
		return false, likes, nil
	}

	if _, err := tx.Exec(`UPDATE comments SET likes = likes + 1 WHERE id = ?`, id); err != nil {
		return false, 0, fmt.Errorf("incr comment %d likes: %w", id, err)
	}
	if err := tx.QueryRow(`SELECT likes FROM comments WHERE id = ?`, id).Scan(&likes); err != nil {
		return false, 0, fmt.Errorf("reload comment %d likes: %w", id, err)
	}
	if err := tx.Commit(); err != nil {
		return false, 0, fmt.Errorf("commit like comment %d: %w", id, err)
	}
	return true, likes, nil
}

// HasCommentLike 判断该 ipHash 是否已对评论点过赞。评论不存在返回 ErrCommentNotFound。
func (s *Store) HasCommentLike(id int64, ipHash string) (bool, int64, error) {
	if err := s.ensureCommentReactions(); err != nil {
		return false, 0, err
	}
	var likes int64
	err := s.db.QueryRow(`SELECT likes FROM comments WHERE id = ?`, id).Scan(&likes)
	if err == sql.ErrNoRows {
		return false, 0, ErrCommentNotFound
	}
	if err != nil {
		return false, 0, fmt.Errorf("select comment %d likes: %w", id, err)
	}
	var n int
	err = s.db.QueryRow(`SELECT COUNT(*) FROM comment_likes WHERE comment_id = ? AND ip_hash = ?`, id, ipHash).Scan(&n)
	if err != nil {
		return false, 0, fmt.Errorf("count comment_likes %d: %w", id, err)
	}
	return n > 0, likes, nil
}

// CreateComment 新建评论，返回新记录 id，c.ID 同步回填。
func (s *Store) CreateComment(c *Comment) (int64, error) {
	if c.Status == "" {
		c.Status = CommentPending
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	isAdmin := 0
	if c.IsAdmin {
		isAdmin = 1
	}
	res, err := s.db.Exec(`INSERT INTO comments (page_key, nick, email, link, content_md, content_html, parent_id, root_id, status, is_admin, ip, ua, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.PageKey, c.Nick, c.Email, c.Link, c.ContentMD, c.ContentHTML, c.ParentID, c.RootID, c.Status, isAdmin, c.IP, c.UA, c.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("create comment on %s: %w", c.PageKey, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	c.ID = id
	return id, nil
}

// UpdateCommentStatus 更新评论状态（审核通过 / 打回待审）。
func (s *Store) UpdateCommentStatus(id int64, status string) error {
	res, err := s.db.Exec(`UPDATE comments SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return fmt.Errorf("update comment %d status: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("update comment %d status: no rows affected", id)
	}
	return nil
}

// DeleteCommentCascade 删除评论及其所有子评论（root_id 指向它的）。
// 两层嵌套下子评论不会再有子级，一条 SQL 覆盖两种情形。
func (s *Store) DeleteCommentCascade(id int64) error {
	_, err := s.db.Exec(`DELETE FROM comments WHERE id = ? OR root_id = ?`, id, id)
	if err != nil {
		return fmt.Errorf("delete comment %d cascade: %w", id, err)
	}
	return nil
}

// idPlaceholders 构造 IN 子句占位符与参数列表（"?,?,..." + args）。
func idPlaceholders(ids []int64) (string, []interface{}) {
	ph := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return ph, args
}

// ApproveComments 批量审核通过，单 SQL IN 实现。
// 返回实际更新的条数（不存在的 id 不计入）；空 ids 直接返回 0。
func (s *Store) ApproveComments(ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	ph, args := idPlaceholders(ids)
	res, err := s.db.Exec(`UPDATE comments SET status = ? WHERE id IN (`+ph+`)`,
		append([]interface{}{CommentApproved}, args...)...)
	if err != nil {
		return 0, fmt.Errorf("approve comments batch: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("approve comments rows affected: %w", err)
	}
	return int(n), nil
}

// DeleteComments 批量删除（级联子评论），事务内先按 id 统计命中数再单 SQL IN 删除。
// 返回请求 ids 中实际删除的条数（级联删掉的子评论不计入）；空 ids 直接返回 0。
func (s *Store) DeleteComments(ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	ph, args := idPlaceholders(ids)
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin delete comments batch: %w", err)
	}
	defer tx.Rollback()

	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM comments WHERE id IN (`+ph+`)`, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("count comments batch: %w", err)
	}
	// 两层嵌套下子评论不会再有子级：id 命中或 root_id 命中即覆盖级联
	if _, err := tx.Exec(`DELETE FROM comments WHERE id IN (`+ph+`) OR root_id IN (`+ph+`)`,
		append(append([]interface{}{}, args...), args...)...); err != nil {
		return 0, fmt.Errorf("delete comments batch: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit delete comments batch: %w", err)
	}
	return n, nil
}

// scanComments 批量扫描 rows 到 []*Comment。is_admin 是 INTEGER 0/1，转 bool。
func scanComments(rows *sql.Rows) ([]*Comment, error) {
	var out []*Comment
	for rows.Next() {
		c := &Comment{}
		var isAdmin int
		if err := rows.Scan(&c.ID, &c.PageKey, &c.Nick, &c.Email, &c.Link, &c.ContentMD, &c.ContentHTML, &c.ParentID, &c.RootID, &c.Status, &isAdmin, &c.IP, &c.UA, &c.Likes, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		c.IsAdmin = isAdmin != 0
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate comments: %w", err)
	}
	return out, nil
}
