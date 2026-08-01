package store

import (
	"fmt"
	"time"
)

// EnsureModuleStatesTable 建 module_states 表（功能模块启停状态持久化）。
// 无记录的模块回退到 Meta.DefaultOn（由 module.Manager 处理），此表仅存用户显式改过的状态。
func (s *Store) EnsureModuleStatesTable() error {
	const ddl = `CREATE TABLE IF NOT EXISTS module_states (
		id         TEXT PRIMARY KEY,
		enabled    INTEGER NOT NULL DEFAULT 0,
		updated_at TEXT NOT NULL DEFAULT ''
	);`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create module_states table: %w", err)
	}
	return nil
}

// GetModuleStates 返回所有已显式设置的模块启停状态（id -> enabled）。
func (s *Store) GetModuleStates() (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT id, enabled FROM module_states`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		var enabled int
		if err := rows.Scan(&id, &enabled); err != nil {
			return nil, err
		}
		out[id] = enabled != 0
	}
	return out, rows.Err()
}

// SetModuleState 写入/更新单个模块的启停状态。
func (s *Store) SetModuleState(id string, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	const q = `INSERT INTO module_states (id, enabled, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET enabled = excluded.enabled, updated_at = excluded.updated_at`
	_, err := s.db.Exec(q, id, v, time.Now().Format(time.RFC3339))
	return err
}
