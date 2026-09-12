package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const ftsTablePrefix = "posts_fts"

func (s *Store) RestoreFrom(srcPath string) error {
	if err := s.ValidateRestoreFrom(srcPath); err != nil {
		return err
	}
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("get restore connection: %w", err)
	}
	defer conn.Close()
	if err := attachReadOnly(ctx, conn, srcPath); err != nil {
		return err
	}
	defer conn.ExecContext(ctx, `DETACH DATABASE restore_src`)
	mainTables, err := listTablesConn(ctx, conn, "main")
	if err != nil {
		return err
	}
	srcTables, err := listTablesConn(ctx, conn, "restore_src")
	if err != nil {
		return err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin restore tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.ExecContext(ctx, `PRAGMA defer_foreign_keys = ON`); err != nil {
		return err
	}
	for _, table := range mainTables {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+quoteIdent(table)); err != nil {
			return fmt.Errorf("clear table %s: %w", table, err)
		}
	}
	if err := copyTablesTx(ctx, tx, mainTables, srcTables); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit restore tx: %w", err)
	}
	committed = true
	return nil
}

// ValidateRestoreFrom performs a complete dry-run against a temporary copy of
// the current database, so malformed data cannot change the live connection.
func (s *Store) ValidateRestoreFrom(srcPath string) error {
	if _, err := os.Stat(srcPath); err != nil {
		return fmt.Errorf("stat backup db: %w", err)
	}
	validationDir, err := os.MkdirTemp(filepath.Dir(s.path), ".restore-validate-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(validationDir)
	tmpPath := filepath.Join(validationDir, "validation.db")
	if err := s.BackupTo(tmpPath); err != nil {
		return fmt.Errorf("snapshot validation db: %w", err)
	}
	probe, err := OpenForRecovery(tmpPath)
	if err != nil {
		return err
	}
	defer probe.Close()
	ctx := context.Background()
	conn, err := probe.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := attachReadOnly(ctx, conn, srcPath); err != nil {
		return err
	}
	defer conn.ExecContext(ctx, `DETACH DATABASE restore_src`)
	var integrity string
	if err := conn.QueryRowContext(ctx, `PRAGMA restore_src.integrity_check`).Scan(&integrity); err != nil {
		return fmt.Errorf("check backup integrity: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("backup integrity check failed: %s", integrity)
	}
	mainTables, err := listTablesConn(ctx, conn, "main")
	if err != nil {
		return err
	}
	srcTables, err := listTablesConn(ctx, conn, "restore_src")
	if err != nil {
		return err
	}
	if len(srcTables) == 0 {
		return fmt.Errorf("backup database has no application tables")
	}
	required := map[string]bool{"posts": false, "comments": false}
	for _, table := range srcTables {
		if _, ok := required[table]; ok {
			required[table] = true
		}
	}
	for table, present := range required {
		if !present {
			return fmt.Errorf("backup is missing legacy core table %s", table)
		}
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `PRAGMA defer_foreign_keys = ON`); err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, table := range mainTables {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+quoteIdent(table)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("validate clear table %s: %w", table, err)
		}
	}
	if err := copyTablesTx(ctx, tx, mainTables, srcTables); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("backup data violates current schema: %w", err)
	}
	return nil
}

func copyTablesTx(ctx context.Context, tx *sql.Tx, mainTables, srcTables []string) error {
	known := make(map[string]bool, len(mainTables))
	for _, t := range mainTables {
		known[t] = true
	}
	for _, table := range srcTables {
		if !known[table] {
			return fmt.Errorf("backup schema is newer (unknown table %q), please upgrade the binary first", table)
		}
		mainCols, err := tableColumnsTx(ctx, tx, "main", table)
		if err != nil {
			return err
		}
		srcCols, err := tableColumnsTx(ctx, tx, "restore_src", table)
		if err != nil {
			return err
		}
		mainSet := make(map[string]bool, len(mainCols))
		for _, c := range mainCols {
			mainSet[c] = true
		}
		for _, c := range srcCols {
			if !mainSet[c] {
				return fmt.Errorf("backup schema is newer (table %s has unknown column %q), please upgrade the binary first", table, c)
			}
		}
		if len(srcCols) == 0 {
			continue
		}
		quoted := make([]string, len(srcCols))
		for i, c := range srcCols {
			quoted[i] = quoteIdent(c)
		}
		q := `INSERT INTO ` + quoteIdent(table) + ` (` + strings.Join(quoted, ", ") + `) SELECT ` + strings.Join(quoted, ", ") + ` FROM restore_src.` + quoteIdent(table)
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("copy table %s: %w", table, err)
		}
	}
	return nil
}

func attachReadOnly(ctx context.Context, conn *sql.Conn, path string) error {
	dsn := (&url.URL{Scheme: "file", Path: path}).String() + "?mode=ro"
	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS restore_src`, dsn); err != nil {
		return fmt.Errorf("attach backup db: %w", err)
	}
	return nil
}
func listTablesConn(ctx context.Context, conn interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, schema string) ([]string, error) {
	rows, err := conn.QueryContext(ctx, `SELECT name FROM `+quoteIdent(schema)+`.sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		if !strings.HasPrefix(n, ftsTablePrefix) {
			out = append(out, n)
		}
	}
	return out, rows.Err()
}
func tableColumnsTx(ctx context.Context, tx *sql.Tx, schema, table string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT name FROM pragma_table_info(?, ?)`, table, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
func quoteIdent(value string) string { return `"` + strings.ReplaceAll(value, `"`, `""`) + `"` }

// OpenForRecovery opens the native Store without CREATE TABLE or migrations.
func OpenForRecovery(path string) (*Store, error) {
	dsn := (&url.URL{Scheme: "file", Path: path}).String() + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open recovery sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db, path: path}, nil
}
