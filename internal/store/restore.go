// 整库恢复（Spec 41）：把备份 DB 的数据灌回当前库。
// 方案：ATTACH 备份库 + 表级「清空 + 列交集复制」，全程复用现有连接——
// 避免「关连接换文件」导致全局 *sql.DB 失效；备份 schema 比当前二进制新时拒绝恢复。
// SQL 拼接安全性：表名/列名均先经当前库 sqlite_master / pragma_table_info 白名单校验（不在白名单内直接拒绝），
// srcPath 来自服务端临时文件而非请求输入，不存在请求可控的标识符注入面。
package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// ftsTablePrefix FTS 虚表及影子表（posts_fts / posts_fts_data / ...）不参与复制，
// 恢复后由上层触发全量重建索引。
const ftsTablePrefix = "posts_fts"

// RestoreFrom 从备份 DB 文件恢复全部业务表数据（覆盖式）。
// 兼容旧 schema 备份（缺列走默认值）；备份含当前库没有的表/列时视为更新 schema，拒绝并提示先升级程序。
func (s *Store) RestoreFrom(srcPath string) error {
	// ATTACH 走只读模式，防止恢复过程反向写坏备份文件
	if _, err := s.db.Exec(`ATTACH DATABASE 'file:` + strings.ReplaceAll(srcPath, `'`, `''`) + `?mode=ro' AS restore_src`); err != nil {
		return fmt.Errorf("attach backup db: %w", err)
	}
	defer s.db.Exec(`DETACH DATABASE restore_src`) //nolint:errcheck

	mainTables, err := s.listTables("main")
	if err != nil {
		return err
	}
	srcTables, err := s.listTables("restore_src")
	if err != nil {
		return err
	}

	// schema 版本护栏：备份里有当前库不认识的表 → 备份来自更新版本
	mainSet := make(map[string]bool, len(mainTables))
	for _, t := range mainTables {
		mainSet[t] = true
	}
	for _, t := range srcTables {
		if !mainSet[t] {
			return fmt.Errorf("backup schema is newer (unknown table %q), please upgrade the binary first", t)
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin restore tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	for _, table := range srcTables {
		mainCols, err := tableColumns(tx, "main", table)
		if err != nil {
			return err
		}
		srcCols, err := tableColumns(tx, "restore_src", table)
		if err != nil {
			return err
		}
		// schema 版本护栏：备份表里有当前库不认识的列 → 备份来自更新版本
		mainColSet := make(map[string]bool, len(mainCols))
		for _, c := range mainCols {
			mainColSet[c] = true
		}
		for _, c := range srcCols {
			if !mainColSet[c] {
				return fmt.Errorf("backup schema is newer (table %s has unknown column %q), please upgrade the binary first", table, c)
			}
		}

		if _, err := tx.Exec(`DELETE FROM main."` + table + `"`); err != nil {
			return fmt.Errorf("clear table %s: %w", table, err)
		}
		colList := `"` + strings.Join(srcCols, `", "`) + `"`
		if _, err := tx.Exec(`INSERT INTO main."` + table + `" (` + colList + `) SELECT ` + colList + ` FROM restore_src."` + table + `"`); err != nil {
			return fmt.Errorf("copy table %s: %w", table, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit restore tx: %w", err)
	}
	return nil
}

// listTables 列出指定 schema 下的业务表（排除 sqlite 内部表与 FTS 虚表族）。
func (s *Store) listTables(schema string) ([]string, error) {
	rows, err := s.db.Query(`SELECT name FROM ` + schema + `.sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list tables of %s: %w", schema, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if strings.HasPrefix(name, ftsTablePrefix) {
			continue
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// tableColumns 返回表的列名列表（pragma table_info 顺序）。
func tableColumns(tx *sql.Tx, schema, table string) ([]string, error) {
	rows, err := tx.Query(`SELECT name FROM pragma_table_info(?, ?)`, table, schema)
	if err != nil {
		return nil, fmt.Errorf("table info %s.%s: %w", schema, table, err)
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
}
