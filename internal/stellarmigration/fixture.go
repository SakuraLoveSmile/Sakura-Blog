package stellarmigration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"blog/internal/post"
	"blog/internal/store"
)

type MigrationFixture struct {
	Directory               string   `json:"directory"`
	DBPath                  string   `json:"db_path"`
	SourcePath              string   `json:"source_path"`
	ResolvedCandidateDigest string   `json:"resolved_candidate_digest"`
	SnapshotCoverage        []string `json:"snapshot_coverage"`
	ExcludedResources       []string `json:"excluded_resources"`
}

// BuildMigrationFixture 只创建全新0700系统临时root；不存在任意目标/生产Apply入口。
func BuildMigrationFixture(ctx context.Context, c Candidate, approval MigrationApproval, decisions NotebookStructureDecisions) (MigrationFixture, error) {
	return buildMigrationFixture(ctx, c, approval, decisions, nil)
}

func buildMigrationFixture(ctx context.Context, c Candidate, approval MigrationApproval, decisions NotebookStructureDecisions, hook func(string, string, *sql.Tx) error) (out MigrationFixture, err error) {
	resolved, nb, log, err := resolveMigration(c, approval, decisions)
	if err != nil {
		return out, err
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	dir, err := os.MkdirTemp("", "stellar-migration-fixture-")
	if err != nil {
		return out, err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dir)
			out = MigrationFixture{}
		}
	}()
	for _, name := range []string{"data", "data/uploads", "data/backups", "config", "content", "content/_data", "content/_data/caches", "cache", "runtime"} {
		if err = os.MkdirAll(filepath.Join(dir, name), 0700); err != nil {
			return out, err
		}
	}
	out = MigrationFixture{Directory: dir, DBPath: filepath.Join(dir, "data/blog.db"), SourcePath: filepath.Join(dir, "runtime/source.private.json"), ResolvedCandidateDigest: CandidateDigest(resolved),
		SnapshotCoverage:  []string{"captured SQLite tables and typed rows restored, including FTS5 roots/shadows; approved Notebook schema replaces its legacy definitions", "raw Notebook rows and unknown columns remain in private source archive; active tables use reviewed current structure", "reviewed canonical Stellar v2 metadata and explicitly approved Markdown", "users, permissions, comments, statuses, IDs, relationships and existing dates captured by snapshot", "tracked documents captured by snapshot; all runtime locations isolated"},
		ExcludedResources: []string{"source indexes, triggers and views are not captured; only required new Notebook indexes are created", "upload bytes and attachments, original app/site configuration files, runtime caches and untracked content are not captured", "source virtual table rowid is preserved through captured FTS5 shadow tables; unsupported virtual modules are rejected", "fixture creation does not prove complete frontend/media/external service acceptance or authorize production migration"}}
	archive := struct {
		Reviewed          Candidate                  `json:"reviewed"`
		Resolved          Candidate                  `json:"resolved"`
		Approval          MigrationApproval          `json:"approval"`
		NotebookDecisions NotebookStructureDecisions `json:"notebook_decisions"`
		Resolutions       []AppliedResolution        `json:"resolutions"`
		Coverage          MigrationFixture           `json:"coverage"`
	}{c, resolved, approval, decisions, log, out}
	if err = WritePrivate(out.SourcePath, archive); err != nil {
		return out, err
	}
	rootJSON, _ := json.Marshal(dir)
	app := []byte("listen_addr: '127.0.0.1:0'\nroot: " + string(rootJSON) + "\ndb_path: data/blog.db\nupload_dir: data/uploads\nseed_on_start: false\nauto_backup: false\n")
	if err = os.WriteFile(filepath.Join(dir, "config/app.yml"), app, 0600); err != nil {
		return out, err
	}
	file, err := os.OpenFile(out.DBPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return out, err
	}
	if err = file.Close(); err != nil {
		return out, err
	}
	db, err := sql.Open("sqlite", out.DBPath)
	if err != nil {
		return out, err
	}
	defer func() {
		if e := db.Close(); err == nil && e != nil {
			err = e
		}
	}()
	db.SetMaxOpenConns(1)
	if _, err = db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		return out, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "PRAGMA defer_foreign_keys=ON"); err != nil {
		return out, err
	}
	if err = restoreSnapshotTables(ctx, tx, c.Source, nb); err != nil {
		return out, err
	}
	if hook != nil {
		if err = hook("after-source", dir, tx); err != nil {
			return out, err
		}
	}
	if err = writeCanonicalMigration(ctx, tx, resolved); err != nil {
		return out, err
	}
	if err = restoreSequence(ctx, tx, c.Source); err != nil {
		return out, err
	}
	if hook != nil {
		if err = hook("after-canonical", dir, tx); err != nil {
			return out, err
		}
	}
	// 文件只写私密新root；文件失败时同样rollback整个数据库并删除本任务新root。
	for _, p := range resolved.Records {
		if p.Table != "" {
			continue
		}
		if !filepath.IsLocal(p.Source) || !strings.HasPrefix(p.Source, "content/") {
			return out, fmt.Errorf("tracked source path is not local content")
		}
		raw, e := decode(p.ConfigJSON)
		if e != nil {
			return out, e
		}
		blog, e := decode(p.BlogJSON)
		if e != nil {
			return out, e
		}
		doc, e := post.SerializeStellarDocument(post.StellarDocument{Raw: object(raw), Blog: object(blog), Body: p.Markdown})
		if e != nil {
			return out, e
		}
		name := filepath.Join(dir, p.Source)
		if err = os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			return out, err
		}
		if err = os.WriteFile(name, doc, 0600); err != nil {
			return out, err
		}
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	if err = tx.Commit(); err != nil {
		return out, err
	}
	return out, nil
}

func snapshotSQLValue(value any, class string) (any, error) {
	switch class {
	case "null":
		if value == nil {
			return nil, nil
		}
	case "blob":
		if b, ok, err := blobBytes(value); ok || err != nil {
			return b, err
		}
	case "integer", "real":
		if v, ok := value.(json.Number); ok {
			if class == "integer" {
				return v.Int64()
			}
			return v.Float64()
		}
	case "text":
		if v, ok := value.(string); ok && utf8.ValidString(v) {
			return v, nil
		}
	}
	return nil, fmt.Errorf("snapshot value does not match explicit SQLite storage class")
}

func tableKinds(ctx context.Context, tx *sql.Tx) (map[string]string, error) {
	rows, err := tx.QueryContext(ctx, "SELECT name,type FROM pragma_table_list WHERE schema='main'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	kinds := map[string]string{}
	for rows.Next() {
		var name, kind string
		if err = rows.Scan(&name, &kind); err != nil {
			return nil, err
		}
		kinds[name] = kind
	}
	return kinds, rows.Err()
}

func restoreSnapshotTables(ctx context.Context, tx *sql.Tx, s Snapshot, nb NotebookStructurePreview) error {
	for _, table := range s.Tables {
		if err := validateTableStorage(table); err != nil {
			return err
		}
	}
	// 先建立virtual roots，由SQLite生成shadow；绝不把shadow当普通表顺序CREATE。
	for _, name := range sortedKeys(s.Tables) {
		table := s.Tables[name]
		if table.Kind != "virtual" {
			continue
		}
		if !strings.Contains(strings.ToLower(table.Schema), "using fts5") {
			return fmt.Errorf("unsupported source virtual module")
		}
		if err := safeTableStatement(table.Schema); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, table.Schema); err != nil {
			return fmt.Errorf("create virtual root %s: %w", name, err)
		}
	}
	if err := store.CreateStellarMigrationNotebookTables(ctx, tx); err != nil {
		return err
	}
	kinds, err := tableKinds(ctx, tx)
	if err != nil {
		return err
	}
	for _, name := range sortedKeys(s.Tables) {
		table := s.Tables[name]
		if name == "notebooks" || name == "notes" || name == "sqlite_sequence" {
			continue
		}
		if table.Kind == "virtual" {
			if kinds[name] != "virtual" {
				return fmt.Errorf("virtual root identity mismatch")
			}
			continue
		}
		if table.Kind == "shadow" {
			if kinds[name] != "shadow" {
				return fmt.Errorf("orphan or unexpected shadow table: %s", name)
			}
			continue
		}
		if table.Kind != "table" {
			return fmt.Errorf("unsupported snapshot table object class")
		}
		if _, exists := kinds[name]; exists {
			return fmt.Errorf("source table collides with generated object")
		}
		if err := safeTableStatement(table.Schema); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, table.Schema); err != nil {
			return fmt.Errorf("create source table %s: %w", name, err)
		}
	}
	for _, name := range sortedKeys(s.Tables) {
		table := s.Tables[name]
		if name == "notebooks" || name == "notes" || name == "sqlite_sequence" || table.Kind == "virtual" {
			continue
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+quoteIdentifier(name)); err != nil {
			return err
		}
		if err := insertSnapshotRows(ctx, tx, name, table); err != nil {
			return err
		}
	}
	if err := insertNotebookStructureWithIdentity(ctx, tx, nb, s); err != nil {
		return err
	}
	return nil
}

// 已批准的Notebook schema转换也不能顺带重编号隐藏rowid。
// 若新INTEGER PRIMARY KEY与捕获身份不可能同时成立，则阻断整个隔离执行。
func insertNotebookStructureWithIdentity(ctx context.Context, tx *sql.Tx, p NotebookStructurePreview, source Snapshot) error {
	for _, name := range []string{"notebooks", "notes"} {
		table := source.Tables[name]
		identities := map[string]int64{}
		if !table.WithoutRowID {
			for i, row := range table.Rows {
				id, _ := table.RowIDs[i].Int64()
				identities[stringValue(row["id"])] = id
			}
		}
		columns := notebookStructureColumns[name]
		for _, row := range p.Rows[name] {
			quoted := append([]string{}, columns...)
			values := []any{}
			rowID, preserve := identities[stringValue(row["id"])]
			if preserve {
				quoted = append([]string{"rowid"}, quoted...)
				values = append(values, rowID)
			}
			for _, column := range columns {
				values = append(values, row[column])
			}
			query := "INSERT INTO " + name + " (" + strings.Join(quoted, ",") + ") VALUES (" + strings.TrimSuffix(strings.Repeat("?,", len(quoted)), ",") + ")"
			if _, err := tx.ExecContext(ctx, query, values...); err != nil {
				return fmt.Errorf("restore Notebook row identity: %w", err)
			}
			if preserve {
				var restored int64
				if err := tx.QueryRowContext(ctx, "SELECT rowid FROM "+name+" WHERE id=?", row["id"]).Scan(&restored); err != nil || restored != rowID {
					return fmt.Errorf("notebook rowid cannot be preserved with approved primary key structure")
				}
			}
		}
	}
	return nil
}

func validateTableStorage(table Table) error {
	if len(table.StorageClasses) != len(table.Rows) || (table.WithoutRowID && len(table.RowIDs) != 0) || (!table.WithoutRowID && len(table.RowIDs) != len(table.Rows)) {
		return fmt.Errorf("snapshot row identity or storage classes incomplete")
	}
	seen := map[int64]bool{}
	for i, row := range table.Rows {
		if len(row) != len(table.Columns) || len(table.StorageClasses[i]) != len(table.Columns) {
			return fmt.Errorf("snapshot row columns or storage classes incomplete")
		}
		if !table.WithoutRowID {
			id, err := table.RowIDs[i].Int64()
			if err != nil || seen[id] {
				return fmt.Errorf("snapshot rowid invalid or duplicated")
			}
			seen[id] = true
		}
		for _, column := range table.Columns {
			value, exists := row[column]
			if !exists {
				return fmt.Errorf("snapshot row column missing")
			}
			if _, err := snapshotSQLValue(value, table.StorageClasses[i][column]); err != nil {
				return err
			}
		}
	}
	return nil
}

func insertSnapshotRows(ctx context.Context, tx *sql.Tx, name string, table Table) error {
	columns, rows := table.Columns, table.Rows
	if err := validateTableStorage(table); err != nil {
		return err
	}
	var withoutRowID bool
	if err := tx.QueryRowContext(ctx, "SELECT wr FROM pragma_table_list WHERE schema='main' AND name=?", name).Scan(&withoutRowID); err != nil {
		return err
	}
	if withoutRowID != table.WithoutRowID {
		return fmt.Errorf("snapshot rowid schema identity mismatch")
	}
	info, err := tx.QueryContext(ctx, "PRAGMA table_info("+quoteIdentifier(name)+")")
	if err != nil {
		return err
	}
	types := map[string]string{}
	for info.Next() {
		var seq, notNull, pk int
		var column, kind string
		var defaultValue any
		if err = info.Scan(&seq, &column, &kind, &notNull, &defaultValue, &pk); err != nil {
			info.Close()
			return err
		}
		types[column] = kind
	}
	if err = info.Err(); err != nil {
		info.Close()
		return err
	}
	info.Close()
	quoted := []string{}
	for _, column := range columns {
		if _, exists := types[column]; !exists {
			return fmt.Errorf("source column is absent from reconstructed schema")
		}
		quoted = append(quoted, quoteIdentifier(column))
	}
	if len(columns) != len(types) {
		return fmt.Errorf("snapshot source columns incomplete")
	}
	if !withoutRowID {
		alias, err := accessibleRowID(columns)
		if err != nil {
			return err
		}
		quoted = append([]string{quoteIdentifier(alias)}, quoted...)
	}
	query := "INSERT INTO " + quoteIdentifier(name) + " (" + strings.Join(quoted, ",") + ") VALUES (" + strings.TrimSuffix(strings.Repeat("?,", len(quoted)), ",") + ")"
	for i, row := range rows {
		if len(row) != len(columns) {
			return fmt.Errorf("snapshot row columns incomplete")
		}
		values := []any{}
		if !withoutRowID {
			id, _ := table.RowIDs[i].Int64()
			values = append(values, id)
		}
		for _, column := range columns {
			v, e := snapshotSQLValue(row[column], table.StorageClasses[i][column])
			if e != nil {
				return fmt.Errorf("table %s column %s: %w", name, column, e)
			}
			values = append(values, v)
		}
		if _, err := tx.ExecContext(ctx, query, values...); err != nil {
			return fmt.Errorf("restore source table %s: %w", name, err)
		}
	}
	return nil
}

func writeCanonicalMigration(ctx context.Context, tx *sql.Tx, c Candidate) error {
	if _, err := tx.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS settings(section TEXT PRIMARY KEY,value TEXT NOT NULL)"); err != nil {
		return err
	}
	for _, v := range []struct{ name, value string }{{"stellar_v2", c.Theme.DocumentJSON}, {"blog_extensions", c.Theme.BlogJSON}} {
		if _, err := tx.ExecContext(ctx, "INSERT INTO settings(section,value) VALUES(?,?) ON CONFLICT(section) DO UPDATE SET value=excluded.value", v.name, v.value); err != nil {
			return err
		}
	}
	for _, table := range []string{"posts", "pages", "wiki_projects", "wikidocs", "topics", "notebooks", "notes"} {
		var exists int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			continue
		}
		for _, column := range []struct{ name, ddl string }{{"stellar_version", "INTEGER NOT NULL DEFAULT 0"}, {"stellar_config", "TEXT NOT NULL DEFAULT ''"}, {"blog_extensions", "TEXT NOT NULL DEFAULT ''"}} {
			var present int
			if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?", table, column.name).Scan(&present); err != nil {
				return err
			}
			if present == 0 {
				if _, err := tx.ExecContext(ctx, "ALTER TABLE "+quoteIdentifier(table)+" ADD COLUMN "+quoteIdentifier(column.name)+" "+column.ddl); err != nil {
					return err
				}
			}
		}
	}
	for _, p := range c.Records {
		if p.Table == "" {
			continue
		}
		query := "UPDATE " + quoteIdentifier(p.Table) + " SET stellar_version=2,stellar_config=?,blog_extensions=?"
		values := []any{p.ConfigJSON, p.BlogJSON}
		originalBody := ""
		for _, row := range c.Source.Tables[p.Table].Rows {
			if stringValue(row["id"]) == p.ID {
				originalBody = body(row)
			}
		}
		if !isCollection(p.Table) && p.Markdown != originalBody {
			bodyColumn := "content_md"
			table := c.Source.Tables[p.Table]
			if p.Table != "notes" && !contains(table.Columns, bodyColumn) {
				bodyColumn = "content"
			}
			query += "," + quoteIdentifier(bodyColumn) + "=?"
			values = append(values, p.Markdown)
		}
		query += " WHERE id=?"
		values = append(values, p.ID)
		result, err := tx.ExecContext(ctx, query, values...)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil || count != 1 {
			return fmt.Errorf("canonical source record missing or duplicated")
		}
	}
	return nil
}

func restoreSequence(ctx context.Context, tx *sql.Tx, s Snapshot) error {
	if table, exists := s.Tables["sqlite_sequence"]; exists {
		if _, err := tx.ExecContext(ctx, "DELETE FROM sqlite_sequence"); err != nil {
			return err
		}
		return insertSnapshotRows(ctx, tx, "sqlite_sequence", table)
	}
	return nil
}

// SQLite master的CREATE TABLE是单语句；防止伪造候选借DDL附加外部写入。
func safeTableStatement(stmt string) error {
	upper := strings.ToUpper(strings.TrimSpace(stmt))
	if !strings.HasPrefix(upper, "CREATE TABLE ") && !strings.HasPrefix(upper, "CREATE VIRTUAL TABLE ") {
		return fmt.Errorf("snapshot schema is not a table declaration")
	}
	quote := byte(0)
	terminated := false
	for i := 0; i < len(stmt); i++ {
		ch := stmt[i]
		if quote != 0 {
			if ch == quote {
				if i+1 < len(stmt) && stmt[i+1] == quote {
					i++
				} else {
					quote = 0
				}
			}
			continue
		}
		if ch == '\'' || ch == '"' || ch == '`' {
			if terminated {
				return fmt.Errorf("multiple snapshot SQL statements")
			}
			quote = ch
			continue
		}
		if ch == '[' {
			if terminated {
				return fmt.Errorf("multiple snapshot SQL statements")
			}
			end := strings.IndexByte(stmt[i+1:], ']')
			if end < 0 {
				return fmt.Errorf("unclosed schema identifier")
			}
			i += end + 1
			continue
		}
		if ch == ';' {
			if terminated {
				return fmt.Errorf("multiple snapshot SQL statements")
			}
			terminated = true
			continue
		}
		if terminated && ch != ' ' && ch != '\n' && ch != '\r' && ch != '\t' {
			return fmt.Errorf("multiple snapshot SQL statements")
		}
	}
	if quote != 0 {
		return fmt.Errorf("unclosed snapshot schema quote")
	}
	return nil
}
