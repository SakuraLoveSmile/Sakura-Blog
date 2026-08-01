package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Attachment 附件持久化记录，对齐 attachments 表字段。
type Attachment struct {
	ID        int64
	Filename  string // 原始文件名
	Path      string // 相对 upload_dir 的路径（如 2026/07/xxx.png）
	URL       string // 对外 URL（如 /uploads/2026/07/xxx.png）
	Mime      string
	Size      int64
	CreatedAt time.Time
	ThumbPath string // 缩略图相对路径，空=无
	ThumbURL  string // 缩略图对外 URL，空=无
	Width     int    // 图片宽度（px），0=非图片/未知
	Height    int    // 图片高度（px）
	GroupID   int64  // 所属分组 id，0=未分组（Spec 44）
}

const attachmentCols = `id, filename, path, url, mime, size, created_at, thumb_path, thumb_url, width, height, group_id`

// UngroupedGroupID 未分组语义常量：不建实体行，group_id=0 即未分组（Spec 44）。
const UngroupedGroupID int64 = 0

// EnsureAttachmentsTable 独立建 attachments 表（不走 post.go createTables，避免与其它 spec 冲突）。
func (s *Store) EnsureAttachmentsTable() error {
	const ddl = `CREATE TABLE IF NOT EXISTS attachments (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		filename   TEXT NOT NULL DEFAULT '',
		path       TEXT NOT NULL DEFAULT '',
		url        TEXT NOT NULL DEFAULT '',
		mime       TEXT NOT NULL DEFAULT '',
		size       INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		thumb_path TEXT NOT NULL DEFAULT '',
		thumb_url  TEXT NOT NULL DEFAULT '',
		width      INTEGER NOT NULL DEFAULT 0,
		height     INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_attachments_created ON attachments(created_at);`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create attachments table: %w", err)
	}
	// 分组表（Spec 44）：单层逻辑标签，不做嵌套；order 控制侧栏排序。
	const groupDDL = `CREATE TABLE IF NOT EXISTS attachment_groups (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		name       TEXT NOT NULL DEFAULT '',
		"order"    INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := s.db.Exec(groupDDL); err != nil {
		return fmt.Errorf("create attachment_groups table: %w", err)
	}
	// 幂等迁移：旧库可能缺新列（ADD COLUMN 已存在会报错，忽略即幂等）
	for _, alter := range []string{
		`ALTER TABLE attachments ADD COLUMN thumb_path TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE attachments ADD COLUMN thumb_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE attachments ADD COLUMN width INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE attachments ADD COLUMN height INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE attachments ADD COLUMN group_id INTEGER NOT NULL DEFAULT 0`,
	} {
		s.db.Exec(alter)
	}
	return nil
}

// CreateAttachment 新建附件记录，返回 id 并回填 a.ID。
func (s *Store) CreateAttachment(a *Attachment) (int64, error) {
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now()
	}
	res, err := s.db.Exec(`INSERT INTO attachments (filename, path, url, mime, size, created_at, thumb_path, thumb_url, width, height, group_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Filename, a.Path, a.URL, a.Mime, a.Size, a.CreatedAt, a.ThumbPath, a.ThumbURL, a.Width, a.Height, a.GroupID)
	if err != nil {
		return 0, fmt.Errorf("create attachment: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	a.ID = id
	return id, nil
}

// ListAttachments 分页列出附件（按创建时间倒序）。page 从 1 开始。
// keyword 模糊匹配 filename；kind 为 "image" 时仅返回 image/* 类型，"other" 返回非图片。
// groupID 为 nil 时不按分组筛选（全部）；为 0 时仅未分组；>0 时指定组（Spec 44）。
func (s *Store) ListAttachments(page, size int, keyword, kind string, groupID *int64) ([]*Attachment, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 24
	}
	offset := (page - 1) * size

	where, args := attachmentFilter(keyword, kind, groupID)

	rows, err := s.db.Query(`SELECT `+attachmentCols+` FROM attachments `+where+`
		ORDER BY substr(created_at, 1, 19) DESC, id DESC LIMIT ? OFFSET ?`, append(args, size, offset)...)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	defer rows.Close()
	return scanAttachments(rows)
}

// CountAttachments 返回附件总数（支持 keyword/kind/group 筛选）。
func (s *Store) CountAttachments(keyword, kind string, groupID *int64) (int, error) {
	where, args := attachmentFilter(keyword, kind, groupID)
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM attachments `+where, args...).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count attachments: %w", err)
	}
	return n, nil
}

// attachmentFilter 拼接列表/计数共用的 WHERE 子句与参数（keyword+kind+group 三重叠加）。
func attachmentFilter(keyword, kind string, groupID *int64) (string, []any) {
	where := "WHERE 1=1"
	var args []any
	if keyword != "" {
		where += " AND filename LIKE ?"
		args = append(args, "%"+keyword+"%")
	}
	switch kind {
	case "image":
		where += " AND mime LIKE 'image/%'"
	case "other":
		where += " AND mime NOT LIKE 'image/%'"
	}
	if groupID != nil {
		where += " AND group_id = ?"
		args = append(args, *groupID)
	}
	return where, args
}

// GetAttachment 按 id 查单条。未找到返回 nil, nil。
func (s *Store) GetAttachment(id int64) (*Attachment, error) {
	row := s.db.QueryRow(`SELECT `+attachmentCols+` FROM attachments WHERE id = ?`, id)
	a := &Attachment{}
	err := row.Scan(&a.ID, &a.Filename, &a.Path, &a.URL, &a.Mime, &a.Size, &a.CreatedAt, &a.ThumbPath, &a.ThumbURL, &a.Width, &a.Height, &a.GroupID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get attachment %d: %w", id, err)
	}
	return a, nil
}

// DeleteAttachment 按 id 删除记录。
func (s *Store) DeleteAttachment(id int64) error {
	res, err := s.db.Exec(`DELETE FROM attachments WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete attachment %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("delete attachment %d: no rows affected", id)
	}
	return nil
}

// AttachmentGroup 附件分组（单层逻辑标签，Spec 44）。
type AttachmentGroup struct {
	ID        int64
	Name      string
	Order     int
	CreatedAt time.Time
	Count     int // 组内附件数（列表时回填，非表字段）
}

// ListAttachmentGroups 列出全部分组（按 order,id 升序），并回填各组附件数。
func (s *Store) ListAttachmentGroups() ([]*AttachmentGroup, error) {
	rows, err := s.db.Query(`SELECT g.id, g.name, g."order", g.created_at,
		(SELECT COUNT(*) FROM attachments a WHERE a.group_id = g.id) AS cnt
		FROM attachment_groups g ORDER BY g."order" ASC, g.id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list attachment groups: %w", err)
	}
	defer rows.Close()
	var out []*AttachmentGroup
	for rows.Next() {
		g := &AttachmentGroup{}
		if err := rows.Scan(&g.ID, &g.Name, &g.Order, &g.CreatedAt, &g.Count); err != nil {
			return nil, fmt.Errorf("scan attachment group: %w", err)
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attachment groups: %w", err)
	}
	return out, nil
}

// CreateAttachmentGroup 新建分组，返回 id 并回填 g.ID。name 需非空（调用方保证）。
func (s *Store) CreateAttachmentGroup(g *AttachmentGroup) (int64, error) {
	if g.CreatedAt.IsZero() {
		g.CreatedAt = time.Now()
	}
	res, err := s.db.Exec(`INSERT INTO attachment_groups (name, "order", created_at) VALUES (?, ?, ?)`,
		g.Name, g.Order, g.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("create attachment group: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	g.ID = id
	return id, nil
}

// UpdateAttachmentGroup 更新分组名称与排序。
func (s *Store) UpdateAttachmentGroup(id int64, name string, order int) error {
	res, err := s.db.Exec(`UPDATE attachment_groups SET name = ?, "order" = ? WHERE id = ?`, name, order, id)
	if err != nil {
		return fmt.Errorf("update attachment group %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("update attachment group %d: no rows affected", id)
	}
	return nil
}

// DeleteAttachmentGroup 删除分组：组内附件归为未分组（group_id=0，不删文件），再删组。
func (s *Store) DeleteAttachmentGroup(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE attachments SET group_id = ? WHERE group_id = ?`, UngroupedGroupID, id); err != nil {
		return fmt.Errorf("reset attachments group %d: %w", id, err)
	}
	res, err := tx.Exec(`DELETE FROM attachment_groups WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete attachment group %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("delete attachment group %d: no rows affected", id)
	}
	return tx.Commit()
}

// AttachmentGroupExists 判断分组是否存在（groupID=0 视为未分组，恒真）。
func (s *Store) AttachmentGroupExists(id int64) (bool, error) {
	if id == UngroupedGroupID {
		return true, nil
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM attachment_groups WHERE id = ?`, id).Scan(&n); err != nil {
		return false, fmt.Errorf("check attachment group %d: %w", id, err)
	}
	return n > 0, nil
}

// MoveAttachments 批量把附件移入目标分组（groupID=0 表示移出到未分组）。
// 幂等：重复移动到同一组结果一致；返回实际匹配到的附件条数。ids 为空返回 0。
func (s *Store) MoveAttachments(ids []int64, groupID int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+1)
	args = append(args, groupID)
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	q := `UPDATE attachments SET group_id = ? WHERE id IN (` + strings.Join(placeholders, ",") + `)`
	res, err := s.db.Exec(q, args...)
	if err != nil {
		return 0, fmt.Errorf("move attachments: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func scanAttachments(rows *sql.Rows) ([]*Attachment, error) {
	var out []*Attachment
	for rows.Next() {
		a := &Attachment{}
		if err := rows.Scan(&a.ID, &a.Filename, &a.Path, &a.URL, &a.Mime, &a.Size, &a.CreatedAt, &a.ThumbPath, &a.ThumbURL, &a.Width, &a.Height, &a.GroupID); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attachments: %w", err)
	}
	return out, nil
}

// DB 暴露底层 *sql.DB（用于 api 层引用扫描等跨表查询）。
func (s *Store) DB() *sql.DB {
	return s.db
}
