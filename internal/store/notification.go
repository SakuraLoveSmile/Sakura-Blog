package store

import (
	"fmt"
	"strings"
	"time"
)

// Notification 站内通知模型（Wave 3b：通知中心）。
// type 取值 comment_new / comment_reply / comment_approved；
// read 0=未读 1=已读；link 为跳转目标（评论所在页面路径）。
type Notification struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Link      string    `json:"link"`
	Read      int       `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName Entity 接口：notifications 表。
func (n *Notification) TableName() string { return "notifications" }

// Columns Entity 接口：全列，首列为 id。
func (n *Notification) Columns() []string {
	return []string{"id", "user_id", "type", "title", "content", "link", "read", "created_at"}
}

// Values Entity 接口：写入值（不含 id，与 Columns()[1:] 对应）。
func (n *Notification) Values() []any {
	return []any{n.UserID, n.Type, n.Title, n.Content, n.Link, n.Read, n.CreatedAt}
}

// SetID Entity 接口：回填自增 id。
func (n *Notification) SetID(id int64) { n.ID = id }

// ScanRow Entity 接口：行扫描，顺序对齐 Columns()。
func (n *Notification) ScanRow(row rowScanner) error {
	return row.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Content, &n.Link, &n.Read, &n.CreatedAt)
}

// EnsureNotificationsTable 独立建 notifications 表（统一走 schema.go 注册表）。
func (s *Store) EnsureNotificationsTable() error {
	return s.ensureTable("notifications")
}

// CreateNotification 写入一条站内通知。
func (s *Store) CreateNotification(n *Notification) error {
	if _, err := repo(s, &Notification{}).Create(n); err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	return nil
}

// ListNotifications 分页查询某用户的通知：未读优先，同未读态按时间倒序。
// 返回条目与总条数（分页组件用）。
func (s *Store) ListNotifications(userID int64, page, pageSize int) ([]*Notification, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	cols := strings.Join((&Notification{}).Columns(), ", ")
	var total int64
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM notifications WHERE user_id = ?`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}
	rows, err := s.db.Query(`SELECT `+cols+` FROM notifications WHERE user_id = ?
		ORDER BY read ASC, datetime(created_at) DESC, id DESC LIMIT ? OFFSET ?`,
		userID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()
	var out []*Notification
	for rows.Next() {
		n := &Notification{}
		if err := n.ScanRow(rows); err != nil {
			return nil, 0, fmt.Errorf("scan notification: %w", err)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate notifications: %w", err)
	}
	return out, total, nil
}

// MarkNotificationRead 标记单条已读（仅限本人的通知）。返回是否命中并更新。
func (s *Store) MarkNotificationRead(userID, id int64) (bool, error) {
	res, err := s.db.Exec(`UPDATE notifications SET read = 1 WHERE id = ? AND user_id = ? AND read = 0`, id, userID)
	if err != nil {
		return false, fmt.Errorf("mark notification read: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// MarkAllNotificationsRead 全部标记已读，返回更新的条数。
func (s *Store) MarkAllNotificationsRead(userID int64) (int64, error) {
	res, err := s.db.Exec(`UPDATE notifications SET read = 1 WHERE user_id = ? AND read = 0`, userID)
	if err != nil {
		return 0, fmt.Errorf("mark all notifications read: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// CountUnreadNotifications 未读通知数（铃铛角标用）。
func (s *Store) CountUnreadNotifications(userID int64) (int64, error) {
	var n int64
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM notifications WHERE user_id = ? AND read = 0`, userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return n, nil
}
