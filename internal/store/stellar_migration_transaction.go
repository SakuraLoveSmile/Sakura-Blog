package store

import (
	"context"
	"database/sql"
)

// CreateStellarMigrationNotebookTables 仅复用正式DDL，生命周期与提交由隔离迁移调用者持有。
// 不打开DB、不启动旧表迁移、不更新内容；所有DDL在同一个调用者事务内执行。
func CreateStellarMigrationNotebookTables(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, notebooksDDL); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, notesDDL)
	return err
}
