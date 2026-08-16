package store

import (
	"database/sql"
	"fmt"
)

// EnsureSettingsTable 建 settings 表（section 主键 + JSON value）。
// 站点配置由 site.yml 配置驱动迁移到 DB 后，各段以 JSON blob 存于此表。
// 独立建表（不走 post.go createTables），与其它模块 Ensure 模式一致。
func (s *Store) EnsureSettingsTable() error {
	return s.ensureTable("settings")
}

// GetSetting 读取某段配置的 JSON 字符串；段不存在返回 ("", nil)。
func (s *Store) GetSetting(section string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE section = ?`, section).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get setting %s: %w", section, err)
	}
	return v, nil
}

// PutSetting UPSERT 某段配置（section 冲突则覆盖 value）。
func (s *Store) PutSetting(section, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings (section, value) VALUES (?, ?)
		ON CONFLICT(section) DO UPDATE SET value = excluded.value`, section, value)
	if err != nil {
		return fmt.Errorf("put setting %s: %w", section, err)
	}
	return nil
}

// AllSettings 读取全部段（section -> JSON value）。
func (s *Store) AllSettings() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT section, value FROM settings`)
	if err != nil {
		return nil, fmt.Errorf("all settings: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var sec, val string
		if err := rows.Scan(&sec, &val); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		out[sec] = val
	}
	return out, rows.Err()
}

// CountSettings 统计段数（Bootstrap 判定空表用）。
func (s *Store) CountSettings() (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM settings`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count settings: %w", err)
	}
	return n, nil
}
