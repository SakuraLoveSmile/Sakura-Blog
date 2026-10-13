package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ReadSettingsSnapshot 一条 SELECT 读取所有段，避免多次 GetSetting 看到
// 不同事务的主题和扩展。返回独立副本，不创建表或修改数据。
func (s *Store) ReadSettingsSnapshot() (map[string]string, error) {
	return s.AllSettings()
}

// PutSettingsSnapshot 原子更新传入段，保留其他所有段。调用者须先完成业务
// 校验；存储边界另检查段名和 JSON，任何错误都不写入。
func (s *Store) PutSettingsSnapshot(updates map[string]string) error {
	return s.UpdateSettingsSnapshot(updates, nil)
}

// UpdateSettingsSnapshot 在同一事务内读取现状、执行门禁、保存多个段。
// guard 不得再次访问 Store（单连接会死锁），也不得产生外部副作用。
// 并发调用按现有单连接约定串行，guard 看到此前已提交的完整快照。
func (s *Store) UpdateSettingsSnapshot(updates map[string]string, guard func(map[string]string) error) error {
	keys := make([]string, 0, len(updates))
	values := make(map[string]string, len(updates))
	for section, value := range updates {
		if strings.TrimSpace(section) == "" {
			return fmt.Errorf("settings snapshot: empty section name")
		}
		if !json.Valid([]byte(value)) {
			return fmt.Errorf("settings snapshot %s: invalid JSON", section)
		}
		keys = append(keys, section)
		values[section] = value
	}
	sort.Strings(keys)
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin settings snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if guard != nil {
		snapshot, err := readSettingsTransaction(tx)
		if err != nil {
			return err
		}
		if err := guard(snapshot); err != nil {
			return err
		}
	}
	for _, section := range keys {
		if _, err := tx.Exec(`INSERT INTO settings (section, value) VALUES (?, ?)
			ON CONFLICT(section) DO UPDATE SET value = excluded.value`, section, values[section]); err != nil {
			return fmt.Errorf("put settings snapshot %s: %w", section, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit settings snapshot: %w", err)
	}
	return nil
}

func readSettingsTransaction(tx *sql.Tx) (map[string]string, error) {
	rows, err := tx.Query(`SELECT section, value FROM settings`)
	if err != nil {
		return nil, fmt.Errorf("read settings snapshot: %w", err)
	}
	defer rows.Close()
	snapshot := make(map[string]string)
	for rows.Next() {
		var section, value string
		if err := rows.Scan(&section, &value); err != nil {
			return nil, fmt.Errorf("scan settings snapshot: %w", err)
		}
		snapshot[section] = value
	}
	return snapshot, rows.Err()
}
