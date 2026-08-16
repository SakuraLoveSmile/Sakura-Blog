package store

import (
	"fmt"
	"time"
)

// 时间格式约定（与 crawler_logs 同语义，本地时区文本比较）：
// perf_samples.ts 精确到秒；perf_requests.minute_ts 精确到分钟。
const (
	perfTsFormat     = "2006-01-02 15:04:05"
	perfMinuteFormat = "2006-01-02 15:04"
)

// PerfSample 一条运行时采样点（内存/goroutine 等）。
type PerfSample struct {
	Ts     string  `json:"ts"`
	Metric string  `json:"metric"`
	Value  float64 `json:"value"`
}

// PerfRequestBucket 一分钟粒度、单路由的请求耗时聚合（写入用）。
type PerfRequestBucket struct {
	Minute   time.Time
	Method   string
	Route    string
	Count    int64
	TotalMs  float64
	MaxMs    float64
	ErrCount int64
}

// PerfRouteStat 区间内按路由聚合的耗时统计（查询用）。
type PerfRouteStat struct {
	Method   string  `json:"method"`
	Route    string  `json:"route"`
	Count    int64   `json:"count"`
	AvgMs    float64 `json:"avg_ms"`
	MaxMs    float64 `json:"max_ms"`
	ErrCount int64   `json:"err_count"`
}

// PerfMinuteStat 一分钟粒度的全站吞吐（查询用）。
type PerfMinuteStat struct {
	Minute   string `json:"minute"`
	Count    int64  `json:"count"`
	ErrCount int64  `json:"err_count"`
}

// EnsurePerfTables 建 perf_samples / perf_requests 两张表（性能监控模块用）。
func (s *Store) EnsurePerfTables() error {
	if err := s.ensureTable("perf_samples"); err != nil {
		return err
	}
	return s.ensureTable("perf_requests")
}

// InsertPerfSamples 同一时刻批量写入多个指标的采样点（单事务）。
func (s *Store) InsertPerfSamples(ts time.Time, metrics map[string]float64) error {
	if len(metrics) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin perf_samples tx: %w", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO perf_samples(ts, metric, value) VALUES (?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("prepare perf_samples insert: %w", err)
	}
	tsStr := ts.Format(perfTsFormat)
	for metric, value := range metrics {
		if _, err := stmt.Exec(tsStr, metric, value); err != nil {
			stmt.Close()
			tx.Rollback()
			return fmt.Errorf("insert perf_samples: %w", err)
		}
	}
	stmt.Close()
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit perf_samples: %w", err)
	}
	return nil
}

// InsertPerfRequestBuckets 批量写入分钟级路由耗时聚合（单事务）。
func (s *Store) InsertPerfRequestBuckets(buckets []PerfRequestBucket) error {
	if len(buckets) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin perf_requests tx: %w", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO perf_requests(minute_ts, method, route, count, total_ms, max_ms, err_count)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("prepare perf_requests insert: %w", err)
	}
	for _, b := range buckets {
		if _, err := stmt.Exec(b.Minute.Format(perfMinuteFormat), b.Method, b.Route,
			b.Count, b.TotalMs, b.MaxMs, b.ErrCount); err != nil {
			stmt.Close()
			tx.Rollback()
			return fmt.Errorf("insert perf_requests: %w", err)
		}
	}
	stmt.Close()
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit perf_requests: %w", err)
	}
	return nil
}

// PerfSamplesSince 返回 since 之后的全部采样点（按时间升序）。
func (s *Store) PerfSamplesSince(since time.Time) ([]PerfSample, error) {
	rows, err := s.db.Query(
		`SELECT ts, metric, value FROM perf_samples WHERE ts >= ? ORDER BY ts`,
		since.Format(perfTsFormat))
	if err != nil {
		return nil, fmt.Errorf("query perf_samples: %w", err)
	}
	defer rows.Close()
	out := []PerfSample{}
	for rows.Next() {
		var p PerfSample
		if err := rows.Scan(&p.Ts, &p.Metric, &p.Value); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PerfRouteStatsSince 区间内按 method+route 聚合的耗时排行（按平均耗时降序）。
func (s *Store) PerfRouteStatsSince(since time.Time) ([]PerfRouteStat, error) {
	rows, err := s.db.Query(`
		SELECT method, route, SUM(count) c,
		       CASE WHEN SUM(count) > 0 THEN SUM(total_ms) / SUM(count) ELSE 0 END avg_ms,
		       MAX(max_ms) max_ms, SUM(err_count) e
		FROM perf_requests WHERE minute_ts >= ?
		GROUP BY method, route ORDER BY avg_ms DESC`,
		since.Format(perfMinuteFormat))
	if err != nil {
		return nil, fmt.Errorf("query perf route stats: %w", err)
	}
	defer rows.Close()
	out := []PerfRouteStat{}
	for rows.Next() {
		var r PerfRouteStat
		if err := rows.Scan(&r.Method, &r.Route, &r.Count, &r.AvgMs, &r.MaxMs, &r.ErrCount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// PerfThroughputSince 区间内逐分钟吞吐（count/err_count），按分钟升序。
func (s *Store) PerfThroughputSince(since time.Time) ([]PerfMinuteStat, error) {
	rows, err := s.db.Query(`
		SELECT minute_ts, SUM(count) c, SUM(err_count) e
		FROM perf_requests WHERE minute_ts >= ?
		GROUP BY minute_ts ORDER BY minute_ts`,
		since.Format(perfMinuteFormat))
	if err != nil {
		return nil, fmt.Errorf("query perf throughput: %w", err)
	}
	defer rows.Close()
	out := []PerfMinuteStat{}
	for rows.Next() {
		var m PerfMinuteStat
		if err := rows.Scan(&m.Minute, &m.Count, &m.ErrCount); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// PerfRequestSummarySince 区间总量摘要：请求数、平均耗时、错误数。
func (s *Store) PerfRequestSummarySince(since time.Time) (count, errCount int64, avgMs float64, err error) {
	row := s.db.QueryRow(`
		SELECT COALESCE(SUM(count), 0), COALESCE(SUM(err_count), 0),
		       CASE WHEN SUM(count) > 0 THEN SUM(total_ms) / SUM(count) ELSE 0 END
		FROM perf_requests WHERE minute_ts >= ?`,
		since.Format(perfMinuteFormat))
	if err := row.Scan(&count, &errCount, &avgMs); err != nil {
		return 0, 0, 0, fmt.Errorf("query perf summary: %w", err)
	}
	return count, errCount, avgMs, nil
}

// PrunePerfData 清理超过 keepDays 天的性能数据（默认 7 天）。
func (s *Store) PrunePerfData(keepDays int) error {
	if keepDays <= 0 {
		keepDays = 7
	}
	cutoffDay := time.Now().AddDate(0, 0, -keepDays).Format("2006-01-02")
	if _, err := s.db.Exec(`DELETE FROM perf_samples WHERE substr(ts,1,10) < ?`, cutoffDay); err != nil {
		return fmt.Errorf("prune perf_samples: %w", err)
	}
	cutoffMinute := time.Now().AddDate(0, 0, -keepDays).Format(perfMinuteFormat)
	if _, err := s.db.Exec(`DELETE FROM perf_requests WHERE minute_ts < ?`, cutoffMinute); err != nil {
		return fmt.Errorf("prune perf_requests: %w", err)
	}
	return nil
}
