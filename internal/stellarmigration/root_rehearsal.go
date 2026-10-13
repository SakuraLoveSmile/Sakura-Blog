package stellarmigration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"blog/internal/config"
	"blog/internal/post"
	"blog/internal/store"
	"github.com/goccy/go-yaml"
)

type RootMigrationApproval struct {
	MigrationApproval
	RootDigest   string `json:"root_digest"`
	ReviewDigest string `json:"review_digest"`
}

// RootReviewDigest绑定整份已选值候选、全部具名选择及独立Notebook宿主字段。
// 原CandidateDigest只包含原来源，不能替代此摘要；计算摘要也不授予最终批准。
func RootReviewDigest(r RootCapture, c Candidate, resolutions []MigrationResolution, decisions NotebookStructureDecisions) (string, error) {
	if err := r.Verify(); err != nil {
		return "", err
	}
	if r.Source.Digest != c.Source.Digest {
		return "", fmt.Errorf("root review source changed")
	}
	p, err := PreviewReviewedSelections(c, resolutions, decisions)
	if err != nil {
		return "", err
	}
	return digest(struct {
		RootDigest      string
		CandidateDigest string
		Resolutions     []MigrationResolution
		Notebook        NotebookStructureDecisions
		Preview         ReviewedMigrationPreview
	}{r.Digest, CandidateDigest(c), resolutions, decisions, p}), nil
}

func validateRootReview(r RootCapture, c Candidate, a RootMigrationApproval, decisions NotebookStructureDecisions) error {
	expected, err := RootReviewDigest(r, c, a.Resolutions, decisions)
	if err != nil {
		return err
	}
	if a.ReviewDigest == "" || a.ReviewDigest != expected {
		return fmt.Errorf("complete reviewed selections/structure approval missing or changed")
	}
	return nil
}

type RootRehearsal struct {
	Directory               string     `json:"directory"`
	Layout                  RootLayout `json:"layout"`
	RootDigest              string     `json:"root_digest"`
	ResolvedCandidateDigest string     `json:"resolved_candidate_digest"`
}

func validateRootApproval(r RootCapture, c Candidate, a RootMigrationApproval) error {
	if err := r.Verify(); err != nil {
		return err
	}
	if a.RootDigest != r.Digest || c.Source.Digest != r.Source.Digest {
		return fmt.Errorf("root/candidate approval does not bind the same complete source")
	}
	layout, err := normalizeRootLayout(r.Layout)
	if err != nil {
		return err
	}
	if encode(layout) != encode(r.Layout) {
		return fmt.Errorf("captured runtime layout changed")
	}
	seen := map[string]bool{}
	virtualSeen := map[string]bool{}
	for _, resource := range r.Resources {
		virtual, err := resourceVirtualPath(r.Layout, resource.Path)
		if err != nil || virtual != resource.VirtualPath || !filepath.IsLocal(filepath.FromSlash(virtual)) || seen[resource.Path] || virtualSeen[virtual] {
			return fmt.Errorf("root resource identity invalid or duplicated")
		}
		if resource.Kind != "file" && resource.Kind != "directory" && resource.Kind != "absent" {
			return fmt.Errorf("root resource kind invalid")
		}
		if resource.Kind != "file" && len(resource.Data) != 0 {
			return fmt.Errorf("non-file root resource contains bytes")
		}
		seen[resource.Path] = true
		virtualSeen[virtual] = true
	}
	if !seen[r.Layout.DBPath] {
		return fmt.Errorf("root capture lacks original database bytes")
	}
	return nil
}

func relocatedLayout(r RootCapture, dir string) (RootLayout, error) {
	next := r.Layout
	next.Root = filepath.Join(dir, "root")
	targets := []*string{&next.DBPath, &next.UploadDir, &next.AppPath}
	for _, target := range targets {
		virtual, err := resourceVirtualPath(r.Layout, *target)
		if err != nil {
			return next, err
		}
		*target = filepath.Join(dir, filepath.FromSlash(virtual))
	}
	// 源操作锁可能在root外；它是活跃互斥边界，不复制到演练目录。
	next.OperationLock = filepath.Join(filepath.Dir(next.DBPath), "maintenance.lock")
	next.ExtraRuntimePaths = append([]string{}, r.Layout.ExtraRuntimePaths...)
	for i, p := range next.ExtraRuntimePaths {
		virtual, err := resourceVirtualPath(r.Layout, p)
		if err != nil {
			return next, err
		}
		next.ExtraRuntimePaths[i] = filepath.Join(dir, filepath.FromSlash(virtual))
	}
	return next, nil
}

// BuildRootRehearsal复制完整资源到新私密root；不接受任意已有可写目标。
// 原DB bytes/WAL只留完整前像，运行库由批准快照事务重建，全部原SQL对象重建并验证。
func BuildRootRehearsal(ctx context.Context, r RootCapture, c Candidate, a RootMigrationApproval, decisions NotebookStructureDecisions) (RootRehearsal, error) {
	return buildRootRehearsal(ctx, r, c, a, decisions, "")
}
func buildRootRehearsal(ctx context.Context, r RootCapture, c Candidate, a RootMigrationApproval, decisions NotebookStructureDecisions, parent string) (out RootRehearsal, err error) {
	if err = validateRootApproval(r, c, a); err != nil {
		return out, err
	}
	if err = validateRootReview(r, c, a, decisions); err != nil {
		return out, err
	}
	resolved, _, _, err := resolveMigration(c, a.MigrationApproval, decisions)
	if err != nil {
		return out, err
	}
	dir, err := os.MkdirTemp(parent, "stellar-root-rehearsal-")
	if err != nil {
		return out, err
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return out, err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dir)
			out = RootRehearsal{}
		}
	}()
	layout, err := relocatedLayout(r, dir)
	if err != nil {
		return out, err
	}
	if err = os.MkdirAll(layout.Root, 0700); err != nil {
		return out, err
	}
	for _, resource := range r.Resources {
		if err = ctx.Err(); err != nil {
			return out, err
		}
		if resource.Kind == "absent" || resource.Path == r.Layout.DBPath || resource.Path == r.Layout.DBPath+"-wal" || resource.Path == r.Layout.DBPath+"-journal" {
			continue
		}
		path := filepath.Join(dir, filepath.FromSlash(resource.VirtualPath))
		if resource.Kind == "directory" {
			err = os.MkdirAll(path, os.FileMode(resource.Mode))
		} else {
			err = writeRootFile(path, resource.Data, os.FileMode(resource.Mode))
		}
		if err != nil {
			return out, err
		}
	}
	// 父目录可能先被MkdirAll创建；按捕获值恢复全部目录权限。
	for i := len(r.Resources) - 1; i >= 0; i-- {
		resource := r.Resources[i]
		if resource.Kind == "directory" {
			if err = os.Chmod(filepath.Join(dir, filepath.FromSlash(resource.VirtualPath)), os.FileMode(resource.Mode)); err != nil {
				return out, err
			}
		}
	}
	fixture, err := buildMigrationFixture(ctx, c, a.MigrationApproval, decisions, func(stage, _ string, tx *sql.Tx) error {
		switch stage {
		case "before-source":
			switch r.DatabaseState.Encoding {
			case "UTF-8", "UTF-16le", "UTF-16be":
			default:
				return fmt.Errorf("unsupported source database encoding")
			}
			_, err := tx.ExecContext(ctx, "PRAGMA encoding="+quoteIdentifier(r.DatabaseState.Encoding))
			if err != nil {
				return err
			}
			return preserveSourceNotebookNameUnique(ctx, tx, r)
		case "after-canonical":
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version=%d", r.DatabaseState.UserVersion)); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA application_id=%d", r.DatabaseState.ApplicationID)); err != nil {
				return err
			}
			return restoreDatabaseObjects(ctx, tx, r.Objects)
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	defer os.RemoveAll(fixture.Directory)
	generated, err := os.ReadFile(fixture.DBPath)
	if err != nil {
		return out, err
	}
	if err = writeRootFile(layout.DBPath, generated, 0600); err != nil {
		return out, err
	}
	for _, record := range resolved.Records {
		if record.Table != "" {
			continue
		}
		raw, e := decode(record.ConfigJSON)
		if e != nil {
			return out, e
		}
		blog, e := decode(record.BlogJSON)
		if e != nil {
			return out, e
		}
		data, e := post.SerializeStellarDocument(post.StellarDocument{Raw: object(raw), Blog: object(blog), Body: record.Markdown})
		if e != nil {
			return out, e
		}
		if err = writeRootFile(filepath.Join(layout.Root, record.Source), data, 0600); err != nil {
			return out, err
		}
	}
	// 仅演练app重定位；真实Apply不改app/site配置或上传/其余content。
	app, err := config.LoadApp(layout.AppPath)
	if err != nil {
		return out, err
	}
	app.Root, app.DBPath, app.UploadDir = layout.Root, layout.DBPath, layout.UploadDir
	app.ListenAddr = "127.0.0.1:0"
	app.SeedOnStart = false
	app.AutoBackup = false
	data, err := yaml.Marshal(app)
	if err != nil {
		return out, err
	}
	if err = writeRootFile(layout.AppPath, data, 0600); err != nil {
		return out, err
	}
	if err = WritePrivate(filepath.Join(dir, "root-capture.private.json"), r); err != nil {
		return out, err
	}
	if err = WritePrivate(filepath.Join(dir, "approval.private.json"), struct {
		Approval RootMigrationApproval
		Notebook NotebookStructureDecisions
	}{a, decisions}); err != nil {
		return out, err
	}
	st, err := store.OpenExisting(layout.DBPath, true)
	if err != nil {
		return out, err
	}
	_, loadErr := config.LoadStellarV2(st)
	closeErr := st.Close()
	if err = errors.Join(loadErr, closeErr); err != nil {
		return out, err
	}
	out = RootRehearsal{Directory: dir, Layout: layout, RootDigest: r.Digest, ResolvedCandidateDigest: CandidateDigest(resolved)}
	return out, nil
}

func restoreDatabaseObjects(ctx context.Context, tx *sql.Tx, objects []DatabaseObject) error {
	for _, object := range objects {
		if object.Kind != "index" && object.Kind != "view" && object.Kind != "trigger" {
			return fmt.Errorf("unsupported captured schema object")
		}
		if object.SQL == "" {
			if object.Kind != "index" {
				return fmt.Errorf("source schema object has no declaration")
			}
			var table string
			if err := tx.QueryRowContext(ctx, "SELECT tbl_name FROM sqlite_master WHERE type='index' AND name=? AND sql IS NULL", object.Name).Scan(&table); err != nil || table != object.Table {
				return fmt.Errorf("source implicit index cannot be preserved by new schema")
			}
			if object.Index == nil {
				return fmt.Errorf("source implicit index shape missing")
			}
			index, err := readDatabaseIndex(ctx, tx, object.Name, object.Table)
			if err != nil || !reflect.DeepEqual(index, *object.Index) {
				return fmt.Errorf("source implicit index constraint/columns cannot be preserved: %s", object.Name)
			}
			continue
		}
		if err := safeDatabaseObjectStatement(object); err != nil {
			return err
		}
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type=? AND name=?", object.Kind, object.Name).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			if _, err := tx.ExecContext(ctx, "DROP "+strings.ToUpper(object.Kind)+" "+quoteIdentifier(object.Name)); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, object.SQL); err != nil {
			return fmt.Errorf("captured schema object incompatible with migration: %s", object.Name)
		}
		var actual DatabaseObject
		if err := tx.QueryRowContext(ctx, "SELECT type,name,tbl_name,coalesce(sql,'') FROM sqlite_master WHERE type=? AND name=?", object.Kind, object.Name).Scan(&actual.Kind, &actual.Name, &actual.Table, &actual.SQL); err != nil {
			return err
		}
		if actual.Kind == "index" {
			index, err := readDatabaseIndex(ctx, tx, actual.Name, actual.Table)
			if err != nil {
				return err
			}
			actual.Index = &index
		}
		if !reflect.DeepEqual(actual, object) {
			return fmt.Errorf("captured schema declaration changed during restore")
		}
	}
	for _, object := range objects {
		if object.Kind == "view" {
			rows, err := tx.QueryContext(ctx, "SELECT * FROM "+quoteIdentifier(object.Name)+" LIMIT 0")
			if err != nil {
				return fmt.Errorf("captured view no longer resolves: %s", object.Name)
			}
			rows.Close()
		}
		if object.Kind == "trigger" {
			columns, err := tx.QueryContext(ctx, "SELECT name FROM pragma_table_info(?) ORDER BY cid", object.Table)
			if err != nil {
				return err
			}
			var assignments []string
			for columns.Next() {
				var column string
				if err = columns.Scan(&column); err != nil {
					columns.Close()
					return err
				}
				assignments = append(assignments, quoteIdentifier(column)+"="+quoteIdentifier(column))
			}
			if err = columns.Err(); err != nil {
				columns.Close()
				return err
			}
			columns.Close()
			for _, statement := range []string{"DELETE FROM " + quoteIdentifier(object.Table) + " WHERE 0", "INSERT INTO " + quoteIdentifier(object.Table) + " DEFAULT VALUES", "UPDATE " + quoteIdentifier(object.Table) + " SET " + strings.Join(assignments, ",") + " WHERE 0"} {
				rows, err := tx.QueryContext(ctx, "EXPLAIN "+statement)
				if err != nil {
					return fmt.Errorf("captured trigger no longer compiles: %s", object.Name)
				}
				rows.Close()
			}
		}
	}
	var check string
	if err := tx.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&check); err != nil || check != "ok" {
		return fmt.Errorf("migration candidate SQLite integrity check failed")
	}
	rows, err := tx.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return fmt.Errorf("migration candidate foreign key check failed")
	}
	return rows.Err()
}

// 旧Notebook name UNIQUE必须保持。先声明其UNIQUE，再声明新的TEXT id PK，
// 使旧autoindex_1仍代表name约束；不把同名的新id索引误认作旧索引。
func preserveSourceNotebookNameUnique(ctx context.Context, tx *sql.Tx, r RootCapture) error {
	var source *DatabaseObject
	for i := range r.Objects {
		object := &r.Objects[i]
		if object.Kind == "index" && object.Table == "notebooks" && object.SQL == "" && object.Index != nil && object.Index.Origin == "u" {
			source = object
			break
		}
	}
	if source == nil {
		return nil
	}
	if source.Name != "sqlite_autoindex_notebooks_1" || !source.Index.Unique || source.Index.Partial || len(source.Index.Columns) != 2 {
		return nil
	}
	key, aux := source.Index.Columns[0], source.Index.Columns[1]
	if key.Name == nil || *key.Name != "name" || !key.Key || key.Descending || key.Collation == nil || *key.Collation != "BINARY" || aux.Name != nil || aux.Key {
		return nil
	}
	if err := store.CreateStellarMigrationNotebookTables(ctx, tx); err != nil {
		return err
	}
	var ddl string
	if err := tx.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type='table' AND name='notebooks'").Scan(&ddl); err != nil {
		return err
	}
	if strings.Count(ddl, "id TEXT PRIMARY KEY,") != 1 {
		return fmt.Errorf("formal Notebook primary key declaration changed")
	}
	ddl = strings.Replace(ddl, "id TEXT PRIMARY KEY,", "id TEXT NOT NULL,", 1)
	end := strings.LastIndex(ddl, ")")
	if end < 0 {
		return fmt.Errorf("formal Notebook declaration invalid")
	}
	ddl = ddl[:end] + ",\nUNIQUE(name),\nPRIMARY KEY(id)\n" + ddl[end:]
	if err := safeTableStatement(ddl); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DROP TABLE notes"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DROP TABLE notebooks"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, ddl); err != nil {
		return err
	}
	return store.CreateStellarMigrationNotebookTables(ctx, tx)
}

func writeRootFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if err := noSymlinkPath(filepath.Dir(path)); err != nil {
		return err
	}
	fd, err := unix.Open(path, unix.O_WRONLY|unix.O_CREAT|unix.O_TRUNC|unix.O_NOFOLLOW|unix.O_CLOEXEC, uint32(mode.Perm()))
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), path)
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Chmod(mode.Perm()); err != nil {
		file.Close()
		return err
	}
	return errors.Join(file.Sync(), file.Close())
}
