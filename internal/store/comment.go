package store

import (
	"database/sql"
	"fmt"
	"time"
)

// 评论状态。
const (
	CommentApproved = "approved"
	CommentPending  = "pending"
)

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
	CreatedAt   time.Time
}

// commentCols comments 表全列，各查询共用，保证与 scanComments 的扫描顺序一致。
const commentCols = `id, page_key, nick, email, link, content_md, content_html, parent_id, root_id, status, is_admin, ip, ua, created_at`

// ListCommentsByPage 返回指定页面的已发布评论，按时间正序（先发的在前，评论区展示序）。
func (s *Store) ListCommentsByPage(pageKey string) ([]*Comment, error) {
	rows, err := s.db.Query(`SELECT `+commentCols+` FROM comments
		WHERE page_key = ? AND status = ? ORDER BY datetime(created_at) ASC, id ASC`, pageKey, CommentApproved)
	if err != nil {
		return nil, fmt.Errorf("list comments by page %s: %w", pageKey, err)
	}
	defer rows.Close()
	return scanComments(rows)
}

// ListRecentComments 返回最新 n 条已发布评论（侧边栏 widget 用），按时间倒序。
func (s *Store) ListRecentComments(n int) ([]*Comment, error) {
	if n < 1 {
		n = 5
	}
	rows, err := s.db.Query(`SELECT `+commentCols+` FROM comments
		WHERE status = ? ORDER BY datetime(created_at) DESC, id DESC LIMIT ?`, CommentApproved, n)
	if err != nil {
		return nil, fmt.Errorf("list recent comments: %w", err)
	}
	defer rows.Close()
	return scanComments(rows)
}

// ListComments 管理页分页查询，按时间倒序。status 为空查全部，否则按状态过滤。page 从 1 开始。
func (s *Store) ListComments(status string, page, size int) ([]*Comment, error) {
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
	q += ` ORDER BY datetime(created_at) DESC, id DESC LIMIT ? OFFSET ?`
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
	row := s.db.QueryRow(`SELECT `+commentCols+` FROM comments WHERE id = ?`, id)
	c := &Comment{}
	var isAdmin int
	err := row.Scan(&c.ID, &c.PageKey, &c.Nick, &c.Email, &c.Link, &c.ContentMD, &c.ContentHTML, &c.ParentID, &c.RootID, &c.Status, &isAdmin, &c.IP, &c.UA, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get comment %d: %w", id, err)
	}
	c.IsAdmin = isAdmin != 0
	return c, nil
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

// scanComments 批量扫描 rows 到 []*Comment。is_admin 是 INTEGER 0/1，转 bool。
func scanComments(rows *sql.Rows) ([]*Comment, error) {
	var out []*Comment
	for rows.Next() {
		c := &Comment{}
		var isAdmin int
		if err := rows.Scan(&c.ID, &c.PageKey, &c.Nick, &c.Email, &c.Link, &c.ContentMD, &c.ContentHTML, &c.ParentID, &c.RootID, &c.Status, &isAdmin, &c.IP, &c.UA, &c.CreatedAt); err != nil {
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
