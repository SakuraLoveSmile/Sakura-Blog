package store

import (
	"fmt"
	"time"
)

// SeoPushLog 单条搜索引擎推送记录。
type SeoPushLog struct {
	ID        int64  `json:"id"`
	Engine    string `json:"engine"` // indexnow / baidu
	URL       string `json:"url"`
	Status    string `json:"status"` // success / failed
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

// EnsureSeoPushLogsTable 建 seo_push_logs 表（站点推送记录用）。
func (s *Store) EnsureSeoPushLogsTable() error {
	return s.ensureTable("seo_push_logs")
}

// AddPushLog 追加一条推送记录。
func (s *Store) AddPushLog(engine, url, status, message string) error {
	_, err := s.db.Exec(
		`INSERT INTO seo_push_logs(engine, url, status, message) VALUES (?, ?, ?, ?)`,
		engine, url, status, message)
	if err != nil {
		return fmt.Errorf("add push log: %w", err)
	}
	return nil
}

// ListPushLogs 分页列出推送记录（可按引擎/状态过滤），返回行 + 总数。
func (s *Store) ListPushLogs(page, size int, engine, status string) ([]*SeoPushLog, int, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 200 {
		size = 20
	}
	conds := ""
	args := []any{}
	if engine != "" {
		conds += " AND engine = ?"
		args = append(args, engine)
	}
	if status != "" {
		conds += " AND status = ?"
		args = append(args, status)
	}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM seo_push_logs WHERE 1=1`+conds, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count push logs: %w", err)
	}
	args = append(args, size, (page-1)*size)
	rows, err := s.db.Query(
		`SELECT id, engine, url, status, message, created_at FROM seo_push_logs WHERE 1=1`+conds+
			` ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list push logs: %w", err)
	}
	defer rows.Close()
	out := []*SeoPushLog{}
	for rows.Next() {
		var l SeoPushLog
		if err := rows.Scan(&l.ID, &l.Engine, &l.URL, &l.Status, &l.Message, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, &l)
	}
	return out, total, rows.Err()
}

// ClearPushLogs 清空全部推送记录，返回删除行数。
func (s *Store) ClearPushLogs() (int64, error) {
	res, err := s.db.Exec(`DELETE FROM seo_push_logs`)
	if err != nil {
		return 0, fmt.Errorf("clear push logs: %w", err)
	}
	return res.RowsAffected()
}

// --- 搜索流量（复用 page_views.referrer 派生，引擎分类在 SEO 模块完成） ---

// ReferrerDailyRow 单日单来源的访问计数。
type ReferrerDailyRow struct {
	Date     string
	Referrer string
	Count    int
}

// ReferrerLandingRow 单来源单着陆页的访问计数。
type ReferrerLandingRow struct {
	Path     string
	Referrer string
	Count    int
}

// ReferrerDaily 返回近 days 天按（日, 来源）聚合的访问计数（referrer 非空）。
// 引擎归类由调用方（SEO 模块）按域名完成，保持 store 通用。
func (s *Store) ReferrerDaily(days int) ([]ReferrerDailyRow, error) {
	if days <= 0 {
		days = 7
	}
	since := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	rows, err := s.db.Query(`
		SELECT date(created_at) d, referrer, COUNT(*) c
		FROM page_views
		WHERE date(created_at) >= ? AND referrer != ''
		GROUP BY d, referrer`, since)
	if err != nil {
		return nil, fmt.Errorf("referrer daily: %w", err)
	}
	defer rows.Close()
	out := []ReferrerDailyRow{}
	for rows.Next() {
		var r ReferrerDailyRow
		if err := rows.Scan(&r.Date, &r.Referrer, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ReferrerLanding 返回近 days 天按（着陆页, 来源）聚合的访问计数（referrer 非空）。
func (s *Store) ReferrerLanding(days int) ([]ReferrerLandingRow, error) {
	if days <= 0 {
		days = 7
	}
	since := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	rows, err := s.db.Query(`
		SELECT path, referrer, COUNT(*) c
		FROM page_views
		WHERE date(created_at) >= ? AND referrer != ''
		GROUP BY path, referrer`, since)
	if err != nil {
		return nil, fmt.Errorf("referrer landing: %w", err)
	}
	defer rows.Close()
	out := []ReferrerLandingRow{}
	for rows.Next() {
		var r ReferrerLandingRow
		if err := rows.Scan(&r.Path, &r.Referrer, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
