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
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

type Row map[string]any
type Table struct {
	Kind           string              `json:"object_type"`
	Schema         string              `json:"schema"`
	Columns        []string            `json:"columns"`
	Rows           []Row               `json:"rows"`
	WithoutRowID   bool                `json:"without_rowid"`
	RowIDs         []json.Number       `json:"rowids"`
	StorageClasses []map[string]string `json:"storage_classes"`
}

// Snapshot includes all tables, not only migrated content. Its digest detects
// concurrent changes to ownership, drafts, comments and otherwise untouched data.
// Snapshot and Candidate are private artifacts: they can contain credentials.
type Snapshot struct {
	FormatVersion  int                        `json:"snapshot_format_version"`
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
	names, err := tx.QueryContext(ctx, "SELECT name,coalesce(sql,''),coalesce((SELECT type FROM pragma_table_list WHERE schema='main' AND name=m.name),''),coalesce((SELECT wr FROM pragma_table_list WHERE schema='main' AND name=m.name),0) FROM sqlite_master m WHERE m.type='table' ORDER BY m.name")
	if err != nil {
		return Snapshot{}, err
	}
	s := Snapshot{FormatVersion: SnapshotFormatVersion, Tables: map[string]Table{}, Files: map[string]string{}}
	for names.Next() {
		var name, ddl, kind string
		var withoutRowID bool
		if err = names.Scan(&name, &ddl, &kind, &withoutRowID); err != nil {
			names.Close()
			return Snapshot{}, err
		}
		s.Tables[name] = Table{Schema: ddl, Kind: kind, WithoutRowID: withoutRowID}
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
		rows.Close()
		// 一元+是SQLite原值恒等表达式，但不继承声明类型元数据；避免driver
		// 把BOOLEAN/TIMESTAMP的INTEGER/TEXT自动转换成bool/time.Time而失去存储原值。
		projections := make([]string, len(t.Columns))
		for i, column := range t.Columns {
			projections[i] = "+" + quoteIdentifier(column)
		}
		selectColumns := strings.Join(projections, ",")
		offset := 0
		if !t.WithoutRowID {
			alias, err := accessibleRowID(t.Columns)
			if err != nil {
				return Snapshot{}, err
			}
			selectColumns = quoteIdentifier(alias) + "," + selectColumns
			offset = 1
		}
		rows, e = tx.QueryContext(ctx, "SELECT "+selectColumns+" FROM "+quoteIdentifier(name))
		if e != nil {
			return Snapshot{}, e
		}
		t.Rows = []Row{}
		t.RowIDs = []json.Number{}
		t.StorageClasses = []map[string]string{}
		for rows.Next() {
			values := make([]any, len(t.Columns)+offset)
			targets := make([]any, len(values))
			for i := range values {
				targets[i] = &values[i]
			}
			if e = rows.Scan(targets...); e != nil {
				rows.Close()
				return Snapshot{}, e
			}
			r := Row{}
			classes := map[string]string{}
			if offset == 1 {
				id, ok := values[0].(int64)
				if !ok {
					rows.Close()
					return Snapshot{}, fmt.Errorf("source rowid is not an integer")
				}
				t.RowIDs = append(t.RowIDs, json.Number(fmt.Sprint(id)))
			}
			for i, v := range values[offset:] {
				switch x := v.(type) {
				case nil:
					classes[t.Columns[i]] = "null"
				case string:
					classes[t.Columns[i]] = "text"
					if !utf8.ValidString(x) {
						rows.Close()
						return Snapshot{}, fmt.Errorf("source SQLite TEXT cannot be represented losslessly")
					}
				case int64:
					classes[t.Columns[i]] = "integer"
					v = json.Number(fmt.Sprint(x))
				case float64:
					if math.IsInf(x, 0) || math.IsNaN(x) {
						rows.Close()
						return Snapshot{}, fmt.Errorf("nonfinite source REAL cannot be represented losslessly")
					}
					classes[t.Columns[i]] = "real"
					v = json.Number(fmt.Sprint(x))
				case []byte:
					classes[t.Columns[i]] = "blob"
					v = newBlobValue(x)
				default:
					rows.Close()
					return Snapshot{}, fmt.Errorf("unsupported source SQLite storage class")
				}
				r[t.Columns[i]] = v
			}
			t.Rows = append(t.Rows, r)
			t.StorageClasses = append(t.StorageClasses, classes)
		}
		if e = rows.Err(); e != nil {
			rows.Close()
			return Snapshot{}, e
		}
		rows.Close()
		order := make([]int, len(t.Rows))
		for i := range order {
			order[i] = i
		}
		sort.Slice(order, func(i, j int) bool {
			a, z := order[i], order[j]
			if !t.WithoutRowID {
				x, _ := t.RowIDs[a].Int64()
				y, _ := t.RowIDs[z].Int64()
				return x < y
			}
			return encode(t.Rows[a]) < encode(t.Rows[z])
		})
		originalRows, originalIDs, originalClasses := t.Rows, t.RowIDs, t.StorageClasses
		t.Rows, t.RowIDs, t.StorageClasses = []Row{}, []json.Number{}, []map[string]string{}
		for _, i := range order {
			t.Rows = append(t.Rows, originalRows[i])
			t.StorageClasses = append(t.StorageClasses, originalClasses[i])
			if !t.WithoutRowID {
				t.RowIDs = append(t.RowIDs, originalIDs[i])
			}
		}
		s.Tables[name] = t
	}
	for path, content := range files {
		if !utf8.ValidString(content) {
			return Snapshot{}, fmt.Errorf("source tracked document cannot be represented losslessly")
		}
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

// SQLite 的三个rowid别名都可被普通列遮蔽；不能猜测不可访问的隐藏身份。
func accessibleRowID(columns []string) (string, error) {
	for _, alias := range []string{"rowid", "_rowid_", "oid"} {
		shadowed := false
		for _, column := range columns {
			if strings.EqualFold(alias, column) {
				shadowed = true
			}
		}
		if !shadowed {
			return alias, nil
		}
	}
	return "", fmt.Errorf("source rowid is inaccessible because all aliases are shadowed")
}
func (s Snapshot) SourceDigest() string {
	return digest(struct {
		FormatVersion  int
		Tables         map[string]Table
		Files          map[string]string
		NotebookSchema store.LegacyNotebookSchema
	}{s.FormatVersion, s.Tables, s.Files, s.NotebookSchema})
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
