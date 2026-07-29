package store

import (
	"database/sql"
	"fmt"
	"time"
)

// Vote 投票记录（Spec 30：{% vote %} 数据服务，对齐 rating.go 模式）。
type Vote struct {
	ID        int64
	TargetID  string    // 投票目标 ID（shortcode 的 id 参数）
	Value     string    // up / down
	VoterHash string    // IP hash 去重
	CreatedAt time.Time
}

// VoteCounts up/down 计数（对齐 vote.js 期望的 data.votes.{up,down}）。
type VoteCounts struct {
	Up   int `json:"up"`
	Down int `json:"down"`
}

// EnsureVotesTable 独立建 votes 表（对齐 rating.go 模式，不动 createTables）。
func (s *Store) EnsureVotesTable() error {
	const ddl = `CREATE TABLE IF NOT EXISTS votes (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		target_id  TEXT NOT NULL DEFAULT '',
		value      TEXT NOT NULL DEFAULT '',
		voter_hash TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_votes_target_voter ON votes(target_id, voter_hash);
	CREATE INDEX IF NOT EXISTS idx_votes_target ON votes(target_id);`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create votes table: %w", err)
	}
	return nil
}

// UpdateVote 投票（同 target + 同 voter_hash 覆盖更新）。
func (s *Store) UpdateVote(targetID, value, voterHash string) error {
	if value != "up" && value != "down" {
		return fmt.Errorf("value must be up or down, got %q", value)
	}
	const q = `INSERT INTO votes (target_id, value, voter_hash, created_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(target_id, voter_hash) DO UPDATE SET value = excluded.value, created_at = excluded.created_at`
	_, err := s.db.Exec(q, targetID, value, voterHash, time.Now())
	return err
}

// GetVotes 获取目标 up/down 计数。无记录返回 {0, 0}。
func (s *Store) GetVotes(targetID string) (*VoteCounts, error) {
	const q = `SELECT
		COALESCE(SUM(CASE WHEN value = 'up' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN value = 'down' THEN 1 ELSE 0 END), 0)
		FROM votes WHERE target_id = ?`
	var vc VoteCounts
	err := s.db.QueryRow(q, targetID).Scan(&vc.Up, &vc.Down)
	if err == sql.ErrNoRows {
		return &VoteCounts{}, nil
	}
	if err != nil {
		return nil, err
	}
	return &vc, nil
}
