package store

import (
	"fmt"
	"time"
)

// CrawlerLog 单条搜索引擎爬虫访问记录。
type CrawlerLog struct {
	ID        int64  `json:"id"`
	Engine    string `json:"engine"` // 归一化引擎名：Baidu/Google/Bing/...
	UA        string `json:"ua"`
	Path      string `json:"path"`
	IPHash    string `json:"ip_hash"`
	CreatedAt string `json:"created_at"`
}

// CrawlerEngineCount 引擎维度计数（分布/占比用）。
type CrawlerEngineCount struct {
	Engine string `json:"engine"`
	Count  int    `json:"count"`
}

// CrawlerTrendPoint 单日单引擎爬取计数（趋势用）。
type CrawlerTrendPoint struct {
	Date   string `json:"date"`
	Engine string `json:"engine"`
	Count  int    `json:"count"`
}

// CrawlerAnalysisResult 爬虫分析聚合结果。
type CrawlerAnalysisResult struct {
	Today        int                  `json:"today"`
	Yesterday    int                  `json:"yesterday"`
	RangeTotal   int                  `json:"range_total"`
	TopEngine    string               `json:"top_engine"`
	Trend        []CrawlerTrendPoint  `json:"trend"`
	Distribution []CrawlerEngineCount `json:"distribution"`
}

// EnsureCrawlerLogsTable 建 crawler_logs 表（SEO 爬虫记录用）。
func (s *Store) EnsureCrawlerLogsTable() error {
	return s.ensureTable("crawler_logs")
}

// RecordCrawler 记录一次爬虫访问。
func (s *Store) RecordCrawler(engine, ua, path, ipHash string) error {
	_, err := s.db.Exec(
		`INSERT INTO crawler_logs(engine, ua, path, ip_hash) VALUES (?, ?, ?, ?)`,
		engine, ua, path, ipHash)
	if err != nil {
		return fmt.Errorf("record crawler: %w", err)
	}
	return nil
}

// CrawlerAnalysis 聚合近 days 天爬虫数据：今日/昨日/区间总量 + 最活跃引擎 + 逐日逐引擎趋势 + 引擎占比。
func (s *Store) CrawlerAnalysis(days int) (*CrawlerAnalysisResult, error) {
	if days <= 0 {
		days = 7
	}
	since := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	res := &CrawlerAnalysisResult{Trend: []CrawlerTrendPoint{}, Distribution: []CrawlerEngineCount{}}

	// 今日 / 昨日
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM crawler_logs WHERE date(created_at)=date('now','localtime')`).Scan(&res.Today)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM crawler_logs WHERE date(created_at)=date('now','localtime','-1 day')`).Scan(&res.Yesterday)

	// 区间分布（引擎占比）+ 区间总量 + 最活跃引擎
	drows, err := s.db.Query(`
		SELECT engine, COUNT(*) c FROM crawler_logs
		WHERE date(created_at) >= ?
		GROUP BY engine ORDER BY c DESC`, since)
	if err != nil {
		return nil, fmt.Errorf("crawler distribution: %w", err)
	}
	defer drows.Close()
	for drows.Next() {
		var ec CrawlerEngineCount
		if err := drows.Scan(&ec.Engine, &ec.Count); err != nil {
			return nil, err
		}
		res.Distribution = append(res.Distribution, ec)
		res.RangeTotal += ec.Count
		if res.TopEngine == "" {
			res.TopEngine = ec.Engine // 已按 count DESC 排序，首条即最活跃
		}
	}
	if err := drows.Err(); err != nil {
		return nil, err
	}

	// 逐日逐引擎趋势
	trows, err := s.db.Query(`
		SELECT date(created_at) d, engine, COUNT(*) c FROM crawler_logs
		WHERE date(created_at) >= ?
		GROUP BY d, engine ORDER BY d`, since)
	if err != nil {
		return nil, fmt.Errorf("crawler trend: %w", err)
	}
	defer trows.Close()
	for trows.Next() {
		var p CrawlerTrendPoint
		if err := trows.Scan(&p.Date, &p.Engine, &p.Count); err != nil {
			return nil, err
		}
		res.Trend = append(res.Trend, p)
	}
	return res, trows.Err()
}

// ListCrawlerLogs 分页列出爬虫明细（可按引擎过滤），返回行 + 总数。
func (s *Store) ListCrawlerLogs(page, size int, engine string) ([]*CrawlerLog, int, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 200 {
		size = 20
	}
	where := ""
	args := []any{}
	if engine != "" {
		where = "WHERE engine = ?"
		args = append(args, engine)
	}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM crawler_logs `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count crawler_logs: %w", err)
	}
	q := `SELECT id, engine, ua, path, ip_hash, created_at FROM crawler_logs ` + where +
		` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, size, (page-1)*size)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list crawler_logs: %w", err)
	}
	defer rows.Close()
	out := []*CrawlerLog{}
	for rows.Next() {
		var l CrawlerLog
		if err := rows.Scan(&l.ID, &l.Engine, &l.UA, &l.Path, &l.IPHash, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, &l)
	}
	return out, total, rows.Err()
}

// PruneCrawlerLogs 清理超过 keepDays 天的爬虫记录（默认 90 天）。
func (s *Store) PruneCrawlerLogs(keepDays int) (int64, error) {
	if keepDays <= 0 {
		keepDays = 90
	}
	cutoff := time.Now().AddDate(0, 0, -keepDays).Format("2006-01-02")
	res, err := s.db.Exec(`DELETE FROM crawler_logs WHERE substr(created_at,1,10) < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("prune crawler_logs: %w", err)
	}
	return res.RowsAffected()
}
