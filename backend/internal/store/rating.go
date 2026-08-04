package store

import (
	"database/sql"
	"fmt"
	"time"
)

// Rating 评分记录。
type Rating struct {
	ID        int64
	TargetID  string    // 评分目标 ID（shortcode 的 id 参数）
	Score     int       // 1-5 星
	VoterHash string    // IP hash 去重
	CreatedAt time.Time
}

// RatingSummary 评分汇总。
type RatingSummary struct {
	Avg   float64 `json:"avg"`
	Count int     `json:"count"`
}

// EnsureRatingsTable 独立建 ratings 表（对齐 attachment.go 模式，不动 createTables）。
func (s *Store) EnsureRatingsTable() error {
	return s.ensureTable("ratings")
}

// VoteRating 投票（同 target + 同 voter_hash 覆盖更新）。
func (s *Store) VoteRating(targetID string, score int, voterHash string) error {
	if score < 1 || score > 5 {
		return fmt.Errorf("score must be 1-5, got %d", score)
	}
	const q = `INSERT INTO ratings (target_id, score, voter_hash, created_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(target_id, voter_hash) DO UPDATE SET score = excluded.score, created_at = excluded.created_at`
	_, err := s.db.Exec(q, targetID, score, voterHash, time.Now())
	return err
}

// GetRating 获取目标评分汇总（avg + count）。无记录返回 {0, 0}。
func (s *Store) GetRating(targetID string) (*RatingSummary, error) {
	const q = `SELECT COALESCE(AVG(score), 0), COUNT(*) FROM ratings WHERE target_id = ?`
	var sum RatingSummary
	err := s.db.QueryRow(q, targetID).Scan(&sum.Avg, &sum.Count)
	if err == sql.ErrNoRows {
		return &RatingSummary{}, nil
	}
	if err != nil {
		return nil, err
	}
	// 四舍五入到 1 位小数
	sum.Avg = float64(int(sum.Avg*10+0.5)) / 10
	return &sum, nil
}
