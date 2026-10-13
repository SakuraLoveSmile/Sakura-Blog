// Package stellarmigration builds reviewable candidates; it never migrates a database.
package stellarmigration

import (
	"blog/internal/store"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

type Row map[string]any
type Table struct {
	Schema  string   `json:"schema"`
	Columns []string `json:"columns"`
	Rows    []Row    `json:"rows"`
}

// Snapshot includes all tables, not only migrated content. Its digest detects
// concurrent changes to ownership, drafts, comments and otherwise untouched data.
// Snapshot and Candidate are private artifacts: they can contain credentials.
type Snapshot struct {
	Tables         map[string]Table           `json:"tables"`
	Files          map[string]string          `json:"files"`
	Digest         string                     `json:"digest"`
	NotebookSchema store.LegacyNotebookSchema `json:"notebook_schema"`
}

// ReadSnapshot opens only an existing database, in SQLite read-only mode. It does
// not bootstrap, alter schema, set journal mode or run store lifecycle callbacks.
func ReadSnapshot(ctx context.Context, path string, files map[string]string) (Snapshot, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Snapshot{}, err
	}
	u := url.URL{Scheme: "file", Path: abs}
	q := u.Query()
	q.Set("mode", "ro")
	q.Set("_pragma", "query_only(1)")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return Snapshot{}, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Snapshot{}, err
	}
	defer tx.Rollback()
	var queryOnly int
	if err = tx.QueryRowContext(ctx, "PRAGMA query_only").Scan(&queryOnly); err != nil || queryOnly != 1 {
		return Snapshot{}, fmt.Errorf("snapshot requires SQLite query_only=1")
	}
	names, err := tx.QueryContext(ctx, "SELECT name,coalesce(sql,'') FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		return Snapshot{}, err
	}
	s := Snapshot{Tables: map[string]Table{}, Files: map[string]string{}}
	for names.Next() {
		var name, ddl string
		if err = names.Scan(&name, &ddl); err != nil {
			names.Close()
			return Snapshot{}, err
		}
		s.Tables[name] = Table{Schema: ddl}
	}
	if err = names.Err(); err != nil {
		names.Close()
		return Snapshot{}, err
	}
	names.Close()
	for _, name := range sortedKeys(s.Tables) {
		t := s.Tables[name]
		rows, e := tx.QueryContext(ctx, "SELECT * FROM "+quoteIdentifier(name))
		if e != nil {
			return Snapshot{}, e
		}
		t.Columns, e = rows.Columns()
		if e != nil {
			rows.Close()
			return Snapshot{}, e
		}
		t.Rows = []Row{}
		for rows.Next() {
			values := make([]any, len(t.Columns))
			targets := make([]any, len(values))
			for i := range values {
				targets[i] = &values[i]
			}
			if e = rows.Scan(targets...); e != nil {
				rows.Close()
				return Snapshot{}, e
			}
			r := Row{}
			for i, v := range values {
				switch x := v.(type) {
				case int64:
					v = json.Number(fmt.Sprint(x))
				case float64:
					v = json.Number(fmt.Sprint(x))
				case []byte:
					v = string(x)
				}
				r[t.Columns[i]] = v
			}
			t.Rows = append(t.Rows, r)
		}
		if e = rows.Err(); e != nil {
			rows.Close()
			return Snapshot{}, e
		}
		rows.Close()
		sort.Slice(t.Rows, func(i, j int) bool { return encode(t.Rows[i]) < encode(t.Rows[j]) })
		s.Tables[name] = t
	}
	for path, content := range files {
		s.Files[path] = content
	}
	st, err := store.OpenExisting(abs, true)
	if err != nil {
		return Snapshot{}, err
	}
	s.NotebookSchema, err = st.DetectLegacyNotebookSchema()
	closeErr := st.Close()
	if err != nil {
		return Snapshot{}, err
	}
	if closeErr != nil {
		return Snapshot{}, closeErr
	}
	for _, table := range s.NotebookSchema.Tables {
		captured, exists := s.Tables[table.Table]
		if exists != table.Exists || (exists && len(captured.Columns) != len(table.Columns)) {
			return Snapshot{}, fmt.Errorf("source notebook schema changed during snapshot")
		}
		for _, column := range captured.Columns {
			if _, ok := table.Columns[column]; !ok {
				return Snapshot{}, fmt.Errorf("source notebook schema changed during snapshot")
			}
		}
	}
	s.Digest = s.SourceDigest()
	return s, nil
}
func quoteIdentifier(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
func (s Snapshot) SourceDigest() string {
	return digest(struct {
		Tables         map[string]Table
		Files          map[string]string
		NotebookSchema store.LegacyNotebookSchema
	}{s.Tables, s.Files, s.NotebookSchema})
}
func (s Snapshot) Verify() error {
	if s.Digest == "" || s.Digest != s.SourceDigest() {
		return fmt.Errorf("migration source digest changed")
	}
	return nil
}
func digest(v any) string            { return bytesDigest([]byte(encode(v))) }
func bytesDigest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func encode(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
func decode(raw string) (any, error) {
	d := json.NewDecoder(strings.NewReader(raw))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("expected one JSON value")
	}
	return v, nil
}
func object(v any) map[string]any {
	m, _ := v.(map[string]any)
	if m == nil {
		return map[string]any{}
	}
	return m
}
func stringValue(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
func clone[T any](v T) T {
	var out T
	d := json.NewDecoder(bytes.NewBufferString(encode(v)))
	d.UseNumber()
	if err := d.Decode(&out); err != nil {
		panic(err)
	}
	return out
}

// WritePrivate creates a new 0600 artifact. Existing files are never overwritten,
// avoiding accidental publication via an existing symlink or permissive file.
func WritePrivate(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(append(b, '\n'))
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
