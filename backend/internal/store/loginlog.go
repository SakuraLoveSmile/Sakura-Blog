package store

import (
	"fmt"
	"strings"
	"time"
)

// loginLogCols 登录日志查询列清单（含 id，顺序对齐 Columns()）。
var loginLogCols = strings.Join((&LoginLog{}).Columns(), ", ")

// LoginLog 登录日志模型（Wave 3a：登录/2FA 校验落库，Console 可查）。
// user_id 为 0 表示旧版 admin（无用户表记录）或失败时未识别用户；
// status 取 success/fail；source 取 password（用户表密码）/legacy（旧 admin）/2fa。
type LoginLog struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Username  string    `json:"username"`
	IP        string    `json:"ip"`
	UA        string    `json:"ua"`
	Status    string    `json:"status"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName Entity 接口：login_logs 表。
func (l *LoginLog) TableName() string { return "login_logs" }

// Columns Entity 接口：全列，首列为 id。
func (l *LoginLog) Columns() []string {
	return []string{"id", "user_id", "username", "ip", "ua", "status", "source", "created_at"}
}

// Values Entity 接口：写入值（不含 id，与 Columns()[1:] 对应）。
func (l *LoginLog) Values() []any {
	return []any{l.UserID, l.Username, l.IP, l.UA, l.Status, l.Source, l.CreatedAt}
}

// SetID Entity 接口：回填自增 id。
func (l *LoginLog) SetID(id int64) { l.ID = id }

// ScanRow Entity 接口：行扫描，顺序对齐 Columns()。
func (l *LoginLog) ScanRow(row rowScanner) error {
	return row.Scan(&l.ID, &l.UserID, &l.Username, &l.IP, &l.UA, &l.Status, &l.Source, &l.CreatedAt)
}

// EnsureLoginLogsTable 独立建 login_logs 表（统一走 schema.go 注册表）。
func (s *Store) EnsureLoginLogsTable() error {
	return s.ensureTable("login_logs")
}

// CreateLoginLog 写入一条登录日志（登录/2FA 校验成功或失败时调用）。
func (s *Store) CreateLoginLog(userID int64, username, ip, ua, status, source string) error {
	l := &LoginLog{
		UserID:    userID,
		Username:  username,
		IP:        ip,
		UA:        ua,
		Status:    status,
		Source:    source,
		CreatedAt: time.Now().UTC(),
	}
	if _, err := repo(s, &LoginLog{}).Create(l); err != nil {
		return fmt.Errorf("create login log: %w", err)
	}
	return nil
}

// ListLoginLogs 分页查询登录日志（按时间倒序）。status 非空时按状态过滤（success/fail）。
// 返回条目与总条数（分页组件用）。
func (s *Store) ListLoginLogs(page, pageSize int, status string) ([]*LoginLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	where := ""
	args := []any{}
	if status != "" {
		where = "WHERE status = ?"
		args = append(args, status)
	}

	var total int64
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM login_logs ` + where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count login logs: %w", err)
	}

	rows, err := s.db.Query(`SELECT `+loginLogCols+` FROM login_logs `+where+
		` ORDER BY datetime(created_at) DESC, id DESC LIMIT ? OFFSET ?`,
		append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list login logs: %w", err)
	}
	defer rows.Close()

	var out []*LoginLog
	for rows.Next() {
		l := &LoginLog{}
		if err := l.ScanRow(rows); err != nil {
			return nil, 0, fmt.Errorf("scan login log: %w", err)
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate login logs: %w", err)
	}
	return out, total, nil
}
