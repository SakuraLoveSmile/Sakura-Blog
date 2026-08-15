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
	CommentRejected = "rejected" // 管理员拒绝，前台不展示
	CommentSpam     = "spam"     // 垃圾评论，前台不展示
)

// ErrCommentNotFound 评论不存在（点赞等写操作用）。
var ErrCommentNotFound = errors.New("comment not found")

// Boost 业务错误，供公开 API 映射为稳定的 HTTP 状态码。
var (
	ErrBoostLimit = errors.New("comment boost limit reached")
	ErrBoostSelf  = errors.New("cannot boost own comment")
)

const maxCommentBoosts = 50

// BoostMeta 是创建 Boost 时写入的发布者快照。
type BoostMeta struct {
	UserID int64
	Nick   string
	Email  string
	IP     string
	IPHash string
}

// CommentBoost 评论 Boost 持久化记录。
type CommentBoost struct {
	ID          int64
	CommentID   int64
	Emoji       string
	Text        string
	UserID      int64
	Nick        string
	Email       string
	IPHash      string
	IdentityKey string
	CreatedAt   time.Time
}

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
	Status      string // approved / pending / rejected / spam
	IsAdmin     bool   // 博主回复标识（is_admin 列 0/1）
	UserID      int64  // 0=匿名，>0 对应 users 表 id（已注册用户）
	Verified    bool   // 注册用户标识：已登录用户评论显示"已验证"徽章
	IP          string
	UA          string
	Likes       int64 // 点赞计数（IP 去重后累加）
	CreatedAt   time.Time
}

// TableName Entity 接口：comments 表。
func (c *Comment) TableName() string { return "comments" }

// Columns Entity 接口：全列，首列为 id。
func (c *Comment) Columns() []string {
	return []string{"id", "page_key", "nick", "email", "link", "content_md", "content_html", "parent_id", "root_id", "status", "is_admin", "user_id", "verified", "ip", "ua", "likes", "created_at"}
}

// Values Entity 接口：写入值（不含 id，与 Columns()[1:] 对应）。
func (c *Comment) Values() []any {
	isAdmin := 0
	if c.IsAdmin {
		isAdmin = 1
	}
	userID := c.UserID
	if userID < 0 {
		userID = 0
	}
	verified := 0
	if c.Verified {
		verified = 1
	}
	return []any{c.PageKey, c.Nick, c.Email, c.Link, c.ContentMD, c.ContentHTML, c.ParentID, c.RootID, c.Status, isAdmin, userID, verified, c.IP, c.UA, c.Likes, c.CreatedAt}
}

// SetID Entity 接口：回填自增 id。
func (c *Comment) SetID(id int64) { c.ID = id }

// ScanRow Entity 接口：行扫描，顺序对齐 Columns()（is_admin/verified 0/1 转 bool）。
func (c *Comment) ScanRow(row rowScanner) error {
	var isAdmin, verified int
	if err := row.Scan(&c.ID, &c.PageKey, &c.Nick, &c.Email, &c.Link, &c.ContentMD, &c.ContentHTML, &c.ParentID, &c.RootID, &c.Status, &isAdmin, &c.UserID, &verified, &c.IP, &c.UA, &c.Likes, &c.CreatedAt); err != nil {
		// 兼容老表：缺 user_id/verified 时按零值兜底
		if isAdminErr := row.Scan(&c.ID, &c.PageKey, &c.Nick, &c.Email, &c.Link, &c.ContentMD, &c.ContentHTML, &c.ParentID, &c.RootID, &c.Status, &isAdmin, &c.IP, &c.UA, &c.Likes, &c.CreatedAt); isAdminErr == nil {
			c.UserID = 0
			c.Verified = false
			c.IsAdmin = isAdmin != 0
			return nil
		}
		return err
	}
	c.IsAdmin = isAdmin != 0
	c.Verified = verified != 0
	return nil
}

// 评论点赞 schema 迁移：不碰 post.go createTables，在首次评论读写时懒执行。
// 迁移状态（sync.Once + err）挂在 Store 实例上，见 post.go 的 Store 定义。

// ensureCommentReactions 为 comments 补 likes/user_id/verified 列，并建评论反应去重表。
func (s *Store) ensureCommentReactions() error {
	s.commentReactionsOnce.Do(func() {
		// 老库迁移：CREATE TABLE IF NOT EXISTS 不会给已存在的表加列。
		rows, err := s.db.Query(`PRAGMA table_info(comments)`)
		if err != nil {
			s.commentReactionsErr = fmt.Errorf("pragma table_info(comments): %w", err)
			return
		}
		hasLikes := false
		hasUserID := false
		hasVerified := false
		for rows.Next() {
			var cid, notnull, pk int
			var name, ctype string
			var dflt sql.NullString
			if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
				rows.Close()
				s.commentReactionsErr = fmt.Errorf("scan table_info(comments): %w", err)
				return
			}
			switch name {
			case "likes":
				hasLikes = true
			case "user_id":
				hasUserID = true
			case "verified":
				hasVerified = true
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
		if !hasUserID {
			if _, err := s.db.Exec(`ALTER TABLE comments ADD COLUMN user_id INTEGER NOT NULL DEFAULT 0`); err != nil {
				s.commentReactionsErr = fmt.Errorf("alter comments add user_id: %w", err)
				return
			}
		}
		if !hasVerified {
			if _, err := s.db.Exec(`ALTER TABLE comments ADD COLUMN verified INTEGER NOT NULL DEFAULT 0`); err != nil {
				s.commentReactionsErr = fmt.Errorf("alter comments add verified: %w", err)
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
		const boostsDDL = `CREATE TABLE IF NOT EXISTS comment_boosts (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			comment_id  INTEGER NOT NULL,
			emoji       TEXT NOT NULL,
			text        TEXT NOT NULL DEFAULT '',
			user_id     INTEGER NOT NULL DEFAULT 0,
			nick        TEXT NOT NULL DEFAULT '',
			email       TEXT NOT NULL DEFAULT '',
			ip_hash     TEXT NOT NULL DEFAULT '',
			identity_key TEXT NOT NULL,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(comment_id, identity_key)
		)`
		if _, err := s.db.Exec(boostsDDL); err != nil {
			s.commentReactionsErr = fmt.Errorf("create comment_boosts table: %w", err)
			return
		}
		if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_comment_boosts_comment ON comment_boosts(comment_id)`); err != nil {
			s.commentReactionsErr = fmt.Errorf("create index comment_boosts comment_id: %w", err)
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
	return repo(s, &Comment{}).List(
		Where("page_key = ? AND status = ?", pageKey, CommentApproved),
		OrderBy("substr(created_at, 1, 19) ASC, id ASC"))
}

// ListRecentComments 返回最新 n 条已发布评论（侧边栏 widget 用），按时间倒序。
func (s *Store) ListRecentComments(n int) ([]*Comment, error) {
	if err := s.ensureCommentReactions(); err != nil {
		return nil, err
	}
	if n < 1 {
		n = 5
	}
	return repo(s, &Comment{}).List(
		Where("status = ?", CommentApproved),
		OrderBy("substr(created_at, 1, 19) DESC, id DESC"), Limit(n))
}

// commentSearchConds 管理页评论列表过滤条件（status 筛选 + keyword 跨字段 LIKE）。
// keyword 为空时仅 status 条件（与旧 ListComments 等价）。
func commentSearchConds(status, keyword string) (string, []any) {
	conds := []string{}
	args := []any{}
	if status != "" {
		conds = append(conds, "status = ?")
		args = append(args, status)
	}
	if kw := strings.TrimSpace(keyword); kw != "" {
		like := "%" + kw + "%"
		conds = append(conds, "(nick LIKE ? OR email LIKE ? OR content LIKE ?)")
		args = append(args, like, like, like)
	}
	return strings.Join(conds, " AND "), args
}

// ListCommentsSearch 管理页评论列表（status 筛选 + keyword 全文 LIKE + 分页）。
// Spec 62：console 评论搜索从「本地过滤当前页」改为后端 keyword，跨页可搜。
func (s *Store) ListCommentsSearch(status, keyword string, page, size int) ([]*Comment, error) {
	if err := s.ensureCommentReactions(); err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	cond, args := commentSearchConds(status, keyword)
	opts := []QueryOption{
		Where(cond, args...),
		OrderBy("substr(created_at, 1, 19) DESC, id DESC"),
		Limit(size), Offset((page - 1) * size),
	}
	return repo(s, &Comment{}).List(opts...)
}

// CountCommentsSearch 管理页分页总数（与 ListCommentsSearch 同过滤条件）。
func (s *Store) CountCommentsSearch(status, keyword string) (int, error) {
	if err := s.ensureCommentReactions(); err != nil {
		return 0, err
	}
	cond, args := commentSearchConds(status, keyword)
	return repo(s, &Comment{}).Count(Where(cond, args...))
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
	opts := []QueryOption{OrderBy("substr(created_at, 1, 19) DESC, id DESC"), Limit(size), Offset((page - 1) * size)}
	if status != "" {
		opts = append(opts, Where("status = ?", status))
	}
	return repo(s, &Comment{}).List(opts...)
}

// CountComments 管理页分页总数（与 ListComments 同过滤条件）。
func (s *Store) CountComments(status string) (int, error) {
	if status == "" {
		return repo(s, &Comment{}).Count()
	}
	return repo(s, &Comment{}).Count(Where("status = ?", status))
}

// CountCommentsByPage 返回指定页面的已发布评论数（评论区"N 条评论"计数用）。
func (s *Store) CountCommentsByPage(pageKey string) (int, error) {
	return repo(s, &Comment{}).Count(Where("page_key = ? AND status = ?", pageKey, CommentApproved))
}

// CountCommentStatuses 按状态分组统计评论数（管理页 tab 计数用）。
// 返回 map 含 "all" 与各状态键；无记录的状态键不出现在 map 中。
func (s *Store) CountCommentStatuses() (map[string]int, error) {
	if err := s.ensureCommentReactions(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM comments GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("group count comments by status: %w", err)
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, fmt.Errorf("scan comment status counts: %w", err)
		}
		counts[status] = n
		counts["all"] += n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate comment status counts: %w", err)
	}
	return counts, nil
}

// GetComment 按 id 查单条评论。未找到返回 nil, nil。
func (s *Store) GetComment(id int64) (*Comment, error) {
	if err := s.ensureCommentReactions(); err != nil {
		return nil, err
	}
	return repo(s, &Comment{}).GetByID(id)
}

// GetCommentsByIDs 按 id 集合批量查评论（Spec 62：评论列表父评论昵称从逐条 GetComment
// 收敛为单条 IN 查询），返回 id → 评论 映射；未收录的 id 不存在。
func (s *Store) GetCommentsByIDs(ids []int64) (map[int64]*Comment, error) {
	if len(ids) == 0 {
		return map[int64]*Comment{}, nil
	}
	if err := s.ensureCommentReactions(); err != nil {
		return nil, err
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	out, err := repo(s, &Comment{}).List(Where("id IN ("+placeholders+")", args...))
	if err != nil {
		return nil, fmt.Errorf("get comments by ids: %w", err)
	}
	m := make(map[int64]*Comment, len(out))
	for _, cm := range out {
		m[cm.ID] = cm
	}
	return m, nil
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

// BoostAuthorGuard 判断提交者是否为评论作者。登录身份优先按 user_id 匹配，
// 同时保留 IP 匹配以覆盖匿名评论作者后来登录的情况。
func (s *Store) BoostAuthorGuard(commentID int64, meta BoostMeta) (bool, error) {
	if err := s.ensureCommentReactions(); err != nil {
		return false, err
	}
	var authorUserID int64
	var authorIP string
	err := s.db.QueryRow(`SELECT user_id, ip FROM comments WHERE id = ?`, commentID).Scan(&authorUserID, &authorIP)
	if err == sql.ErrNoRows {
		return false, ErrCommentNotFound
	}
	if err != nil {
		return false, fmt.Errorf("select comment %d author: %w", commentID, err)
	}
	return (meta.UserID > 0 && authorUserID > 0 && meta.UserID == authorUserID) ||
		(meta.IP != "" && authorIP != "" && meta.IP == authorIP), nil
}

// AddBoost 创建或更新一条评论 Boost。返回值为记录、是否新建、错误。
// 计数、作者保护和 UPSERT 在同一事务中完成，避免并发请求突破每楼上限。
func (s *Store) AddBoost(commentID int64, identity, emoji, text string, meta BoostMeta) (*CommentBoost, bool, error) {
	if err := s.ensureCommentReactions(); err != nil {
		return nil, false, err
	}
	identity = strings.TrimSpace(identity)
	if identity == "" {
		return nil, false, errors.New("boost identity is empty")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, false, fmt.Errorf("begin boost comment %d: %w", commentID, err)
	}
	defer tx.Rollback()

	var authorUserID int64
	var authorIP string
	if err := tx.QueryRow(`SELECT user_id, ip FROM comments WHERE id = ?`, commentID).Scan(&authorUserID, &authorIP); err == sql.ErrNoRows {
		return nil, false, ErrCommentNotFound
	} else if err != nil {
		return nil, false, fmt.Errorf("select comment %d author: %w", commentID, err)
	}
	if (meta.UserID > 0 && authorUserID > 0 && meta.UserID == authorUserID) ||
		(meta.IP != "" && authorIP != "" && meta.IP == authorIP) {
		return nil, false, ErrBoostSelf
	}

	var existingID int64
	err = tx.QueryRow(`SELECT id FROM comment_boosts WHERE comment_id = ? AND identity_key = ?`, commentID, identity).Scan(&existingID)
	created := err == sql.ErrNoRows
	if err != nil && err != sql.ErrNoRows {
		return nil, false, fmt.Errorf("find boost comment %d: %w", commentID, err)
	}
	if created {
		var count int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM comment_boosts WHERE comment_id = ?`, commentID).Scan(&count); err != nil {
			return nil, false, fmt.Errorf("count boosts comment %d: %w", commentID, err)
		}
		if count >= maxCommentBoosts {
			return nil, false, ErrBoostLimit
		}
		res, err := tx.Exec(`INSERT INTO comment_boosts
			(comment_id, emoji, text, user_id, nick, email, ip_hash, identity_key, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			commentID, emoji, text, meta.UserID, meta.Nick, meta.Email, meta.IPHash, identity, time.Now())
		if err != nil {
			return nil, false, fmt.Errorf("insert boost comment %d: %w", commentID, err)
		}
		if existingID, err = res.LastInsertId(); err != nil {
			return nil, false, fmt.Errorf("boost comment %d id: %w", commentID, err)
		}
	} else if _, err := tx.Exec(`UPDATE comment_boosts SET emoji = ?, text = ?, user_id = ?, nick = ?, email = ?, ip_hash = ?, created_at = ? WHERE id = ?`,
		emoji, text, meta.UserID, meta.Nick, meta.Email, meta.IPHash, time.Now(), existingID); err != nil {
		return nil, false, fmt.Errorf("update boost comment %d: %w", commentID, err)
	}

	boost := &CommentBoost{}
	if err := tx.QueryRow(`SELECT id, comment_id, emoji, text, user_id, nick, email, ip_hash, identity_key, created_at
		FROM comment_boosts WHERE id = ?`, existingID).Scan(
		&boost.ID, &boost.CommentID, &boost.Emoji, &boost.Text, &boost.UserID, &boost.Nick,
		&boost.Email, &boost.IPHash, &boost.IdentityKey, &boost.CreatedAt); err != nil {
		return nil, false, fmt.Errorf("reload boost comment %d: %w", commentID, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("commit boost comment %d: %w", commentID, err)
	}
	return boost, created, nil
}

// DeleteBoost 删除指定身份在评论上的 Boost；不存在时返回 false。
func (s *Store) DeleteBoost(commentID int64, identity string) (bool, error) {
	if err := s.ensureCommentReactions(); err != nil {
		return false, err
	}
	res, err := s.db.Exec(`DELETE FROM comment_boosts WHERE comment_id = ? AND identity_key = ?`, commentID, strings.TrimSpace(identity))
	if err != nil {
		return false, fmt.Errorf("delete boost comment %d: %w", commentID, err)
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// HasBoost 判断指定身份是否已经回应过评论，用于允许同身份更新而不绕过新身份限流。
func (s *Store) HasBoost(commentID int64, identity string) (bool, error) {
	if err := s.ensureCommentReactions(); err != nil {
		return false, err
	}
	var id int64
	err := s.db.QueryRow(`SELECT id FROM comment_boosts WHERE comment_id = ? AND identity_key = ?`, commentID, strings.TrimSpace(identity)).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("find boost comment %d: %w", commentID, err)
	}
	return true, nil
}

// CountBoostsByComment 返回一条评论的 Boost 数量。
func (s *Store) CountBoostsByComment(commentID int64) (int, error) {
	if err := s.ensureCommentReactions(); err != nil {
		return 0, err
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM comment_boosts WHERE comment_id = ?`, commentID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count boosts comment %d: %w", commentID, err)
	}
	return count, nil
}

// ListBoostsByPage 一次返回页面上所有已发布评论的 Boost，避免评论列表逐条查询。
func (s *Store) ListBoostsByPage(pageKey string) (map[int64][]*CommentBoost, error) {
	if err := s.ensureCommentReactions(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT b.id, b.comment_id, b.emoji, b.text, b.user_id, b.nick, b.email,
		b.ip_hash, b.identity_key, b.created_at
		FROM comment_boosts b JOIN comments c ON c.id = b.comment_id
		WHERE c.page_key = ? AND c.status = ?
		ORDER BY b.comment_id ASC, b.created_at ASC, b.id ASC`, pageKey, CommentApproved)
	if err != nil {
		return nil, fmt.Errorf("list boosts page %q: %w", pageKey, err)
	}
	defer rows.Close()
	out := map[int64][]*CommentBoost{}
	for rows.Next() {
		boost := &CommentBoost{}
		if err := rows.Scan(&boost.ID, &boost.CommentID, &boost.Emoji, &boost.Text, &boost.UserID, &boost.Nick,
			&boost.Email, &boost.IPHash, &boost.IdentityKey, &boost.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan boost page %q: %w", pageKey, err)
		}
		out[boost.CommentID] = append(out[boost.CommentID], boost)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate boosts page %q: %w", pageKey, err)
	}
	return out, nil
}

// CreateComment 新建评论，返回新记录 id，c.ID 同步回填。
func (s *Store) CreateComment(c *Comment) (int64, error) {
	if err := s.ensureCommentReactions(); err != nil {
		return 0, err
	}
	if c.Status == "" {
		c.Status = CommentPending
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	return repo(s, &Comment{}).Create(c)
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

// SetCommentsStatus 批量更新评论状态，单 SQL IN 实现。
// 返回实际更新的条数（不存在的 id 不计入）；空 ids 直接返回 0。
func (s *Store) SetCommentsStatus(ids []int64, status string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	ph, args := idPlaceholders(ids)
	res, err := s.db.Exec(`UPDATE comments SET status = ? WHERE id IN (`+ph+`)`,
		append([]interface{}{status}, args...)...)
	if err != nil {
		return 0, fmt.Errorf("set comments status batch: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("set comments status rows affected: %w", err)
	}
	return int(n), nil
}

// ApproveComments 批量审核通过，单 SQL IN 实现。
// 返回实际更新的条数（不存在的 id 不计入）；空 ids 直接返回 0。
func (s *Store) ApproveComments(ids []int64) (int, error) {
	return s.SetCommentsStatus(ids, CommentApproved)
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
