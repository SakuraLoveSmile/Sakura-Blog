package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"blog/internal/stellarv2"
)

// ErrStellarMigrationRequired 表示原始记录尚未经过用户确认的迁移。
var ErrStellarMigrationRequired = errors.New("stellar content migration required")

const StellarContentVersion = 2

var stellarContentTables = map[string]bool{
	"posts": true, "pages": true, "wikidocs": true,
	"wiki_projects": true, "topics": true, "notebooks": true, "notes": true,
}

// schema 只补充空列，不映射旧内容或改变版本标记。
func migrateStellarContentTx(tx *sql.Tx) error {
	for _, table := range []string{"posts", "pages", "wikidocs", "wiki_projects", "topics"} {
		if err := ensureStellarColumnsTx(tx, table); err != nil {
			return err
		}
	}
	return nil
}

func ensureStellarColumnsTx(tx *sql.Tx, table string) error {
	if !stellarContentTables[table] {
		return nil
	}
	rows, err := tx.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	cols := map[string]bool{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, kind string
		var def sql.NullString
		if err := rows.Scan(&cid, &name, &kind, &notnull, &def, &pk); err != nil {
			rows.Close()
			return err
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(cols) == 0 {
		return nil
	}
	for _, col := range []struct{ name, definition string }{
		{"stellar_version", "INTEGER NOT NULL DEFAULT 0"},
		{"stellar_config", "TEXT NOT NULL DEFAULT ''"},
		{"blog_extensions", "TEXT NOT NULL DEFAULT ''"},
	} {
		if cols[col.name] {
			continue
		}
		if _, err := tx.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + col.name + ` ` + col.definition); err != nil {
			return fmt.Errorf("add %s.%s: %w", table, col.name, err)
		}
	}
	return nil
}

func (s *Store) ensureStellarContentColumns(table string) error {
	if !stellarContentTables[table] {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := ensureStellarColumnsTx(tx, table); err != nil {
		return err
	}
	return tx.Commit()
}

func decodeStellarObject(raw, label string) (map[string]any, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	if object == nil {
		return nil, fmt.Errorf("%s must be a JSON object", label)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("%s: trailing JSON data", label)
	}
	return object, nil
}

func validateStellarContent(version int, raw, extensions, source, profile string, collection bool) (stellarv2.Result, error) {
	if version != StellarContentVersion {
		if version == 0 {
			return stellarv2.Result{}, fmt.Errorf("%w: %s", ErrStellarMigrationRequired, source)
		}
		return stellarv2.Result{}, fmt.Errorf("%s: unsupported stellar version %d", source, version)
	}
	object, err := decodeStellarObject(raw, source)
	if err != nil {
		return stellarv2.Result{}, err
	}
	if extensions != "" {
		if _, err := decodeStellarObject(extensions, source+" Blog extensions"); err != nil {
			return stellarv2.Result{}, err
		}
	}
	var result stellarv2.Result
	if collection {
		result, err = stellarv2.ParseCollection(object, stellarv2.Options{Source: source})
	} else {
		result, err = stellarv2.ParsePage(object, stellarv2.Options{Source: source})
	}
	if err != nil {
		return result, err
	}
	if !collection {
		if membership, ok := object["collection"].(map[string]any); ok {
			if declared, ok := membership["profile"].(string); ok {
				profile = declared
			}
		}
	}
	var consumed stellarv2.Result
	if collection {
		consumed, err = stellarv2.ValidateCollectionProfile(result.Value, source, profile)
	} else {
		consumed = stellarv2.ValidatePageProfile(result.Value, source, profile)
	}
	consumed.Issues = append(result.Issues, consumed.Issues...)
	return consumed, err
}

// 版本 0 仅保留旧调用约定；带配置却未设置版本或未知版本一律拒绝。
func validateStellarWrite(version int, raw, extensions, source, profile string, collection bool) error {
	if version == 0 && raw == "" && extensions == "" {
		return nil
	}
	_, err := validateStellarContent(version, raw, extensions, source, profile, collection)
	return err
}

// ValidateStellar 严格读取；返回归一化运行模型和官方 profile warning，原始 JSON 不改写。
func (p *Post) ValidateStellar() (stellarv2.Result, error) {
	return validateStellarContent(p.StellarVersion, p.StellarConfig, p.BlogExtensions, "posts/"+p.Slug, "post", false)
}
func (p *Page) ValidateStellar() (stellarv2.Result, error) {
	return validateStellarContent(p.StellarVersion, p.StellarConfig, p.BlogExtensions, "pages/"+p.Slug, "page", false)
}
func (p *WikiDoc) ValidateStellar() (stellarv2.Result, error) {
	return validateStellarContent(p.StellarVersion, p.StellarConfig, p.BlogExtensions, "wiki/"+p.ProjectID+"/"+p.Slug, "wiki", false)
}
func (p *WikiProject) ValidateStellar() (stellarv2.Result, error) {
	return validateStellarContent(p.StellarVersion, p.StellarConfig, p.BlogExtensions, "wiki/"+p.ID, "wiki", true)
}
func (p *Topic) ValidateStellar() (stellarv2.Result, error) {
	return validateStellarContent(p.StellarVersion, p.StellarConfig, p.BlogExtensions, "topic/"+p.Slug, "topic", true)
}

func rejectStellarDowngrade(current, candidate int) error {
	if current != 0 && current != candidate {
		return fmt.Errorf("cannot downgrade migrated stellar content from version %d to %d", current, candidate)
	}
	return nil
}

func createStellarEntityTx(tx *sql.Tx, entity Entity) (int64, error) {
	cols := entity.Columns()[1:]
	query := `INSERT INTO ` + entity.TableName() + ` (` + strings.Join(cols, ",") + `) VALUES (` + strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",") + `)`
	result, err := tx.Exec(query, entity.Values()...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func updateStellarEntityTx(tx *sql.Tx, entity Entity, id int64, version int) error {
	cols := entity.Columns()[1:]
	assignments := make([]string, len(cols))
	for i, col := range cols {
		assignments[i] = col + "=?"
	}
	args := append(entity.Values(), id, version)
	result, err := tx.Exec(`UPDATE `+entity.TableName()+` SET `+strings.Join(assignments, ",")+` WHERE id=? AND (stellar_version=0 OR stellar_version=?)`, args...)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("update %s %d: record missing or stellar version downgrade rejected", entity.TableName(), id)
	}
	return nil
}
