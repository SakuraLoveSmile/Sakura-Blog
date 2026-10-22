package store

import (
	"database/sql"
	"fmt"
)

func migrateNotebookOwnershipTx(tx *sql.Tx) error {
	report, err := detectLegacyNotebookSchema(tx, false)
	if err != nil {
		return err
	}
	if report.Required() {
		return errSchemaMigrationDeferred
	}
	for _, table := range []string{"notebooks", "notes"} {
		if err := ensureNotebookOwnershipTx(tx, table); err != nil {
			return err
		}
	}
	return nil
}

// 旧记录 owner=0，不能从当前会话或内容署名猜测所有权。
func ensureNotebookOwnershipTx(tx *sql.Tx, table string) error {
	if table != "notebooks" && table != "notes" {
		return fmt.Errorf("invalid notebook ownership table")
	}
	rows, err := tx.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	exists, hasOwner := false, false
	for rows.Next() {
		var cid, notnull, pk int
		var name, kind string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &kind, &notnull, &defaultValue, &pk); err != nil {
			_ = rows.Close()
			return err
		}
		exists = true
		hasOwner = hasOwner || name == "author_id"
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !exists || hasOwner {
		return nil
	}
	_, err = tx.Exec(`ALTER TABLE ` + table + ` ADD COLUMN author_id INTEGER NOT NULL DEFAULT 0`)
	return err
}

func (s *Store) ensureNotebookOwnership(table string) error {
	report, err := s.DetectLegacyNotebookSchema()
	if err != nil {
		return err
	}
	if report.Required() {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := ensureNotebookOwnershipTx(tx, table); err != nil {
		return err
	}
	return tx.Commit()
}
