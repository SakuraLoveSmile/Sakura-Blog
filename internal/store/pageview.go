package store

import (
	"fmt"
	"time"
)

// DailyStat 单日 PV/UV 聚合。
type DailyStat struct {
	Date string `json:"date"` // YYYY-MM-DD
	PV   int    `json:"pv"`
	UV   int    `json:"uv"`
}

// PopularStat 区间内按 PV 排序的热门路径。
type PopularStat struct {
	Path  string `json:"path"`
	Title string `json:"title"` // 由调用方解析填充（path → 文章标题）
	PV    int    `json:"pv"`
	UV    int    `json:"uv"`
}

// ReferrerStat 来源域名统计。
type ReferrerStat struct {
	Referrer string `json:"referrer"`
	Count    int    `json:"count"`
}

// EnsurePageViewsTable 独立建 page_views 明细表（对齐 revision.go 模式，不动 createTables）。
// 索引：created_at（趋势范围扫描）、path+created_at（热门路径聚合）。
func (s *Store) EnsurePageViewsTable() error {
	const ddl = `CREATE TABLE IF NOT EXISTS page_views (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		path       TEXT NOT NULL DEFAULT '',
		referrer   TEXT NOT NULL DEFAULT '',
		ua_hash    TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now','localtime'))
	)`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create page_views: %w", err)
	}
	const idx1 = `CREATE INDEX IF NOT EXISTS idx_pv_created ON page_views(created_at)`
	const idx2 = `CREATE INDEX IF NOT EXISTS idx_pv_path_created ON page_views(path, created_at)`
	if _, err := s.db.Exec(idx1); err != nil {
		return fmt.Errorf("create idx_pv_created: %w", err)
	}
	if _, err := s.db.Exec(idx2); err != nil {
		return fmt.Errorf("create idx_pv_path_created: %w", err)
	}
	return nil
}

// RecordView 记录一次页面访问。PV 逐次累加；UV 通过 ua_hash + path + 自然日去重
// （查询时 COUNT DISTINCT，写入不做唯一约束，保持 append-only 简洁）。
func (s *Store) RecordView(path, referrer, uaHash string) error {
	_, err := s.db.Exec(
		`INSERT INTO page_views(path, referrer, ua_hash) VALUES (?, ?, ?)`,
		path, referrer, uaHash)
	if err != nil {
		return fmt.Errorf("record view %s: %w", path, err)
	}
	return nil
}

// TrendPVUV 返回近 days 天每日 PV/UV 序列（无访问日补零）。
// UV = 同 ua_hash 同 path 同自然日算 1（COUNT DISTINCT ua_hash || path）。
func (s *Store) TrendPVUV(days int) ([]DailyStat, error) {
	if days <= 0 {
		days = 7
	}
	since := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	rows, err := s.db.Query(`
		SELECT date(created_at) AS d,
		       COUNT(*)         AS pv,
		       COUNT(DISTINCT ua_hash || '|' || path) AS uv
		FROM page_views
		WHERE date(created_at) >= ?
		GROUP BY d ORDER BY d`, since)
	if err != nil {
		return nil, fmt.Errorf("trend query: %w", err)
	}
	defer rows.Close()

	// 先收集有数据的日期
	m := make(map[string]DailyStat, days)
	for rows.Next() {
		var ds DailyStat
		if err := rows.Scan(&ds.Date, &ds.PV, &ds.UV); err != nil {
			return nil, fmt.Errorf("scan trend: %w", err)
		}
		m[ds.Date] = ds
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trend: %w", err)
	}

	// 补零：生成完整日期序列
	out := make([]DailyStat, 0, days)
	for i := 0; i < days; i++ {
		d := time.Now().AddDate(0, 0, -(days-1-i)).Format("2006-01-02")
		if ds, ok := m[d]; ok {
			out = append(out, ds)
		} else {
			out = append(out, DailyStat{Date: d, PV: 0, UV: 0})
		}
	}
	return out, nil
}

// PopularByViews 返回近 days 天内按 PV 降序的热门路径（前 limit 条）。
func (s *Store) PopularByViews(days, limit int) ([]PopularStat, error) {
	if days <= 0 {
		days = 7
	}
	if limit <= 0 {
		limit = 10
	}
	since := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	rows, err := s.db.Query(`
		SELECT path,
		       COUNT(*) AS pv,
		       COUNT(DISTINCT ua_hash) AS uv
		FROM page_views
		WHERE date(created_at) >= ?
		GROUP BY path ORDER BY pv DESC LIMIT ?`, since, limit)
	if err != nil {
		return nil, fmt.Errorf("popular query: %w", err)
	}
	defer rows.Close()

	var out []PopularStat
	for rows.Next() {
		var ps PopularStat
		if err := rows.Scan(&ps.Path, &ps.PV, &ps.UV); err != nil {
			return nil, fmt.Errorf("scan popular: %w", err)
		}
		out = append(out, ps)
	}
	if out == nil {
		out = []PopularStat{}
	}
	return out, rows.Err()
}

// TopReferrers 返回近 days 天内 referrer 域名 Top N（空 referrer 不计）。
func (s *Store) TopReferrers(days, limit int) ([]ReferrerStat, error) {
	if days <= 0 {
		days = 7
	}
	if limit <= 0 {
		limit = 10
	}
	since := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	rows, err := s.db.Query(`
		SELECT referrer, COUNT(*) AS cnt
		FROM page_views
		WHERE date(created_at) >= ? AND referrer != ''
		GROUP BY referrer ORDER BY cnt DESC LIMIT ?`, since, limit)
	if err != nil {
		return nil, fmt.Errorf("referrers query: %w", err)
	}
	defer rows.Close()

	var out []ReferrerStat
	for rows.Next() {
		var rs ReferrerStat
		if err := rows.Scan(&rs.Referrer, &rs.Count); err != nil {
			return nil, fmt.Errorf("scan referrer: %w", err)
		}
		out = append(out, rs)
	}
	if out == nil {
		out = []ReferrerStat{}
	}
	return out, rows.Err()
}

// PrunePageViews 清理超过 keepDays 天的明细记录（默认保留 90 天），返回删除行数。
func (s *Store) PrunePageViews(keepDays int) (int64, error) {
	if keepDays <= 0 {
		keepDays = 90
	}
	cutoff := time.Now().AddDate(0, 0, -keepDays).Format("2006-01-02")
	res, err := s.db.Exec(`DELETE FROM page_views WHERE substr(created_at, 1, 10) < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("prune page_views: %w", err)
	}
	return res.RowsAffected()
}
