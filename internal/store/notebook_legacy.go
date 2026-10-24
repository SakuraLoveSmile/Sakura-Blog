package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// LegacyNotebookSchema 只报告结构，不读取正文、ID 或所有者。
// 缺少整张表属于新库初始化；已有表的源身份/正文列不匹配才需要审核迁移。
type LegacyNotebookSchema struct {
	Tables []LegacyNotebookTableSchema
}

type LegacyNotebookTableSchema struct {
	Table   string
	Exists  bool
	Columns map[string]string
	Issues  []string
}

func (r LegacyNotebookSchema) Required() bool {
	for _, table := range r.Tables {
		if len(table.Issues) > 0 {
			return true
		}
	}
	return false
}

func (r LegacyNotebookSchema) Error() error {
	issues := []string{}
	for _, table := range r.Tables {
		for _, issue := range table.Issues {
			issues = append(issues, table.Table+" schema: "+issue)
		}
	}
	if len(issues) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrStellarMigrationRequired, strings.Join(issues, "; "))
}

type notebookSchemaQuerier interface {
	Query(string, ...any) (*sql.Rows, error)
}

// DetectLegacyNotebookSchema 是普通启动、迁移预检共用的只读结构边界。
// author_id 和三个 Stellar 空元数据列可以幂等补列，不算旧布局转换。
func (s *Store) DetectLegacyNotebookSchema() (LegacyNotebookSchema, error) {
	return detectLegacyNotebookSchema(s.db, false)
}

func detectLegacyNotebookSchema(db notebookSchemaQuerier, complete bool) (LegacyNotebookSchema, error) {
	report := LegacyNotebookSchema{Tables: []LegacyNotebookTableSchema{}}
	for _, spec := range []struct {
		table string
		cols  []string
	}{
		{"notebooks", []string{"id", "name", "description", "icon", "cover", "sort", "created_at", "updated_at"}},
		{"notes", []string{"id", "notebook_id", "slug", "title", "content_md", "content_html", "status", "tags_json", "created_at", "updated_at"}},
	} {
		table := LegacyNotebookTableSchema{Table: spec.table, Columns: map[string]string{}, Issues: []string{}}
		rows, err := db.Query(`PRAGMA table_info(` + spec.table + `)`)
		if err != nil {
			return report, fmt.Errorf("inspect %s schema: %w", spec.table, err)
		}
		for rows.Next() {
			var cid, notnull, pk int
			var name, kind string
			var def sql.NullString
			if err := rows.Scan(&cid, &name, &kind, &notnull, &def, &pk); err != nil {
				rows.Close()
				return report, err
			}
			table.Columns[name] = kind
		}
		err = rows.Err()
		closeErr := rows.Close()
		if err != nil {
			return report, err
		}
		if closeErr != nil {
			return report, closeErr
		}
		table.Exists = len(table.Columns) > 0
		if table.Exists || complete {
			cols := spec.cols
			if complete {
				cols = append(cols, "author_id", "stellar_version", "stellar_config", "blog_extensions")
			}
			for _, name := range cols {
				if _, ok := table.Columns[name]; !ok {
					table.Issues = append(table.Issues, "missing column "+name)
				}
			}
			for _, name := range []string{"id", "notebook_id"} {
				kind, ok := table.Columns[name]
				if !ok {
					continue
				}
				want := "TEXT"
				if spec.table == "notes" && name == "id" {
					want = "INTEGER"
				}
				if strings.ToUpper(strings.TrimSpace(kind)) != want {
					table.Issues = append(table.Issues, fmt.Sprintf("column %s has type %s; expected %s", name, kind, want))
				}
			}
		}
		report.Tables = append(report.Tables, table)
	}
	return report, nil
}

func (s *Store) requireNotebookSchema() error {
	report, err := detectLegacyNotebookSchema(s.db, true)
	if err != nil {
		return err
	}
	return report.Error()
}
