package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestSchemaMigrationLedgerRecordsLegacyBaseline(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ledger.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close first store: %v", err)
	}

	s, err = New(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count migration ledger: %v", err)
	}
	if count != len(schemaMigrations) {
		t.Fatalf("migration ledger rows = %d, want %d", count, len(schemaMigrations))
	}
	var version int
	var name string
	if err := s.DB().QueryRow(`SELECT version, name FROM schema_migrations WHERE version = 1`).Scan(&version, &name); err != nil {
		t.Fatalf("read baseline migration: %v", err)
	}
	if version != 1 || name != "legacy-baseline" {
		t.Fatalf("baseline = %d/%q, want 1/legacy-baseline", version, name)
	}
}

func TestSchemaMigrationRunsOnlyOnce(t *testing.T) {
	s := newMigrationStore(t)
	runs := 0
	migrations := []Migration{{Version: 100, Name: "create-test-table", Up: func(tx *sql.Tx) error {
		runs++
		_, err := tx.Exec(`CREATE TABLE migration_test (value TEXT NOT NULL)`)
		return err
	}}}

	if err := s.applyMigrations(migrations); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	if err := s.applyMigrations(migrations); err != nil {
		t.Fatalf("reapply migration: %v", err)
	}
	if runs != 1 {
		t.Fatalf("migration runs = %d, want 1", runs)
	}
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 100`).Scan(&count); err != nil {
		t.Fatalf("count applied migration: %v", err)
	}
	if count != 1 {
		t.Fatalf("version 2 ledger rows = %d, want 1", count)
	}
}

func TestSchemaMigrationFailureDoesNotRecordSuccess(t *testing.T) {
	s := newMigrationStore(t)
	wantErr := errors.New("forced migration failure")
	failing := []Migration{{Version: 100, Name: "failed-migration", Up: func(tx *sql.Tx) error {
		if _, err := tx.Exec(`CREATE TABLE rollback_test (value TEXT NOT NULL)`); err != nil {
			return err
		}
		return wantErr
	}}}
	if err := s.applyMigrations(failing); !errors.Is(err, wantErr) {
		t.Fatalf("failed migration error = %v, want %v", err, wantErr)
	}

	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 100`).Scan(&count); err != nil {
		t.Fatalf("count failed migration: %v", err)
	}
	if count != 0 {
		t.Fatalf("failed migration ledger rows = %d, want 0", count)
	}
	var tableCount int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'rollback_test'`).Scan(&tableCount); err != nil {
		t.Fatalf("check rollback table: %v", err)
	}
	if tableCount != 0 {
		t.Fatal("failed migration DDL should be rolled back")
	}

	if err := s.applyMigrations([]Migration{{Version: 100, Name: "failed-migration", Up: func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE rollback_test (value TEXT NOT NULL)`)
		return err
	}}}); err != nil {
		t.Fatalf("retry migration: %v", err)
	}
}

func TestSchemaMigrationOrderMustBeStrictlyIncreasing(t *testing.T) {
	s := newMigrationStore(t)
	for _, migrations := range [][]Migration{
		{{Version: 100, Name: "second"}, {Version: 1, Name: "first"}},
		{{Version: 100, Name: "duplicate-a"}, {Version: 100, Name: "duplicate-b"}},
	} {
		if err := s.applyMigrations(migrations); err == nil {
			t.Fatalf("unordered or duplicate migrations should fail: %#v", migrations)
		}
	}
}

func newMigrationStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "migration.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
