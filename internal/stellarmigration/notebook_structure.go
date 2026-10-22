package stellarmigration

import (
	"blog/internal/store"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// NotebookRecordMetadata 只承载用户逐记录批准的缺失宿主字段；不推断发布或所有权。
type NotebookRecordMetadata struct {
	Approved  bool    `json:"approved"`
	Slug      *string `json:"slug,omitempty"`
	Status    *string `json:"status,omitempty"`
	OwnerID   *int64  `json:"owner_id,omitempty"`
	CreatedAt *string `json:"created_at,omitempty"`
	UpdatedAt *string `json:"updated_at,omitempty"`
}

type NotebookStructureDecisions struct {
	SourceDigest string                            `json:"source_digest"`
	Records      map[string]NotebookRecordMetadata `json:"records"`
}

// NotebookStructurePreview 含正文和原值，是私密候选，不能直接发布。
type NotebookStructurePreview struct {
	SourceDigest string           `json:"source_digest"`
	Rows         map[string][]Row `json:"rows"`
	Mappings     []Mapping        `json:"mappings"`
	Diagnostics  []Diagnostic     `json:"diagnostics"`
}

func (p NotebookStructurePreview) Blocking() bool {
	for _, d := range p.Diagnostics {
		if d.Blocking {
			return true
		}
	}
	return false
}

var notebookStructureColumns = map[string][]string{
	"notebooks": {"id", "name", "description", "icon", "cover", "sort", "author_id", "created_at", "updated_at", "stellar_version", "stellar_config", "blog_extensions"},
	"notes":     {"id", "notebook_id", "slug", "title", "content_md", "content_html", "status", "tags_json", "author_id", "created_at", "updated_at", "stellar_version", "stellar_config", "blog_extensions"},
}

// PreviewNotebookStructure 无存储副作用。只转换结构；不标记 Stellar version 2，
// 不改 Markdown、不解除主 Candidate 的政策/内容/结构诊断。
func PreviewNotebookStructure(s Snapshot, decisions NotebookStructureDecisions) (NotebookStructurePreview, error) {
	if err := s.Verify(); err != nil {
		return NotebookStructurePreview{}, err
	}
	p := NotebookStructurePreview{SourceDigest: s.Digest, Rows: map[string][]Row{"notebooks": {}, "notes": {}}, Mappings: []Mapping{}, Diagnostics: []Diagnostic{}}
	issue := func(source, path, code, message string) {
		p.Diagnostics = append(p.Diagnostics, Diagnostic{Kind: "structure", Severity: "error", Code: code, Source: source, Path: path, Message: message, Blocking: true})
	}
	if len(decisions.Records) > 0 && decisions.SourceDigest != s.Digest {
		return p, fmt.Errorf("notebook metadata approval source digest changed")
	}
	known := map[string]bool{}
	parents := map[string]bool{}
	slugs := map[string]bool{}
	owners := map[string]bool{}
	for _, row := range s.Tables["users"].Rows {
		owners[stringValue(row["id"])] = true
	}
	for _, table := range []string{"notebooks", "notes"} {
		for _, original := range s.Tables[table].Rows {
			row := clone(original)
			source := table + "/" + stringValue(row["id"])
			for column, value := range original {
				if text, ok := value.(string); ok && !utf8.ValidString(text) {
					issue(source, column, "notebook_source_encoding", "源字段不能无损写入 JSON 私密归档，不能生成转换库")
				}
			}
			known[source] = true
			meta := decisions.Records[source]
			approved := map[string]any{}
			if meta.Slug != nil {
				approved["slug"] = *meta.Slug
			}
			if meta.Status != nil {
				approved["status"] = *meta.Status
			}
			if meta.OwnerID != nil {
				approved["author_id"] = *meta.OwnerID
			}
			if meta.CreatedAt != nil {
				approved["created_at"] = *meta.CreatedAt
			}
			if meta.UpdatedAt != nil {
				approved["updated_at"] = *meta.UpdatedAt
			}
			target := Row{}
			for _, column := range notebookStructureColumns[table] {
				src := column
				alias := ""
				if table == "notebooks" && column == "sort" {
					alias = "nb_order"
				}
				if table == "notes" && column == "content_md" {
					alias = "content"
				}
				value, exists := row[column]
				if alias != "" {
					if old, present := row[alias]; present {
						if exists && encode(value) != encode(old) {
							issue(source, column, "notebook_structure_source_conflict", "旧列与新列同时存在且不同，不默选其中一项")
						}
						if !exists {
							value, exists, src = old, true, alias
						}
					}
				}
				if decision, ok := approved[column]; ok {
					if !meta.Approved {
						issue(source, column, "notebook_metadata_unapproved", "逐记录元数据未批准，不能分配路由、发布时间或所有权")
					} else if exists && value != nil && stringValue(value) != stringValue(decision) {
						issue(source, column, "notebook_metadata_overwrite", "结构转换不能覆盖已有宿主元数据")
					} else {
						value, exists, src = decision, true, "approved."+column
					}
				}
				if !exists || value == nil {
					if exists && value == nil {
						issue(source, column, "notebook_null_conversion", "旧 NULL 不能隐式转换为非空默认值；原值保留在私密归档")
					}
					switch column {
					case "slug", "status", "author_id", "created_at", "updated_at", "id", "notebook_id", "content_md":
						issue(source, column, "notebook_metadata_required", "缺少明确字段或逐记录批准值；不生成默认路由、状态、owner、日期或正文")
						continue
					case "sort", "stellar_version":
						value = 0
					case "tags_json":
						tags := []string{}
						if raw, ok := row["tags"]; ok && raw != nil {
							if text, ok := raw.(string); ok {
								if text != "" {
									tags = strings.Split(text, ",")
								}
							} else {
								issue(source, "tags", "notebook_tags_invalid", "旧 tags 不是字符串，不猜分隔规则")
							}
						}
						value = encode(tags)
						src = "tags"
					default:
						value = ""
					}
				}
				target[column] = value
				p.Mappings = append(p.Mappings, Mapping{source + "." + src, table + "." + column, "结构字段保留或显式批准；原始行/未知列另存私密源归档"})
			}
			for _, column := range []string{"name", "description", "icon", "cover", "title", "content_md", "content_html", "stellar_config", "blog_extensions", "tags_json", "slug", "status", "created_at", "updated_at"} {
				if value, exists := target[column]; exists {
					if _, ok := value.(string); !ok {
						issue(source, column, "notebook_text_invalid", "旧字段不是字符串，不能隐式格式化正文或元数据")
					}
				}
			}
			if table == "notebooks" {
				id := stringValue(target["id"])
				target["id"] = id
				if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `<>:"/\|?*`) || strings.TrimSpace(id) != id || strings.IndexFunc(id, func(r rune) bool { return r < 32 }) >= 0 {
					issue(source, "id", "notebook_identity_invalid", "旧集合 ID 不能无损用作当前 TEXT 身份")
				}
				if parents[id] {
					issue(source, "id", "notebook_identity_duplicate", "集合身份重复，不能覆盖")
				}
				parents[id] = true
			} else {
				target["notebook_id"] = stringValue(target["notebook_id"])
				if !parents[stringValue(target["notebook_id"])] {
					issue(source, "notebook_id", "notebook_parent_missing", "笔记父集合不存在，不移动或猜归属")
				}
				if value, exists := target["slug"]; exists {
					slug := stringValue(value)
					if !validNotebookSlug(slug) {
						issue(source, "slug", "notebook_slug_invalid", "slug 必须是安全的非空相对路径")
					}
					key := stringValue(target["notebook_id"]) + "/" + slug
					if slugs[key] {
						issue(source, "slug", "notebook_slug_duplicate", "同集合笔记 slug 重复，不能覆盖")
					}
					slugs[key] = true
				}
				if value, exists := target["status"]; exists && value != "draft" && value != "published" {
					issue(source, "status", "notebook_status_invalid", "status 必须显式选择 draft 或 published")
				}
				tags, err := decode(stringValue(target["tags_json"]))
				list, ok := tags.([]any)
				if err != nil || !ok {
					issue(source, "tags_json", "notebook_tags_invalid", "tags_json 必须为字符串数组，原值保留")
				} else {
					for _, tag := range list {
						if _, ok := tag.(string); !ok {
							issue(source, "tags_json", "notebook_tags_invalid", "tags_json 不接受非字符串标签")
						}
					}
				}
				if legacy, exists := row["tags"]; exists && legacy != nil && row["tags_json"] != nil {
					old := []string{}
					if text, ok := legacy.(string); ok && text != "" {
						old = strings.Split(text, ",")
					}
					if encode(tags) != encode(old) {
						issue(source, "tags_json", "notebook_structure_source_conflict", "旧 tags 与 tags_json 不同，不能自动选择")
					}
				}
			}
			for _, column := range []string{"author_id", "sort", "stellar_version", "id"} {
				if column == "id" && table == "notebooks" {
					continue
				}
				v, exists := target[column]
				if !exists {
					continue
				}
				integer, err := strconv.ParseInt(stringValue(v), 10, 64)
				if err != nil || ((column == "author_id" || column == "stellar_version") && integer < 0) {
					issue(source, column, "notebook_integer_invalid", "整数身份/字段不能无损表示为 int64")
					continue
				}
				target[column] = integer
				if column == "author_id" && integer != 0 && !owners[strconv.FormatInt(integer, 10)] {
					issue(source, column, "notebook_owner_missing", "批准的拥有者不在源 users，不能建立虚构所有权")
				}
			}
			for _, column := range []string{"created_at", "updated_at"} {
				if value, exists := target[column]; exists {
					date, text := value.(string)
					if !text || !validNotebookDate(date) {
						issue(source, column, "notebook_date_invalid", "日期不能被当前 DATETIME 消费，不生成当前时间")
					}
				}
			}
			p.Rows[table] = append(p.Rows[table], target)
		}
	}
	for source, meta := range decisions.Records {
		if !known[source] {
			issue(source, "approval", "notebook_approval_unknown", "批准条目不属于当前源集合/笔记")
		}
		if strings.HasPrefix(source, "notebooks/") && (meta.Slug != nil || meta.Status != nil) {
			issue(source, "approval", "notebook_approval_invalid", "集合不能批准笔记 slug/status 字段")
		}
	}
	return p, nil
}

func validNotebookSlug(slug string) bool {
	if slug == "" || strings.ContainsAny(slug, `<>:"\|?*`) || strings.IndexFunc(slug, func(r rune) bool { return r < 32 }) >= 0 {
		return false
	}
	for _, part := range strings.Split(slug, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}
func validNotebookDate(value string) bool {
	// modernc.org/sqlite v1.54.0 conn.parseTimeString 已消费此Go time.String形状。
	// 保真快照不再把原TEXT预先转换成time.Time，校验须接受同一正式格式。
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02 15:04:05.999999999", "2006-01-02", "2006-01-02 15:04:05.999999999 -0700 MST"} {
		if _, err := time.Parse(layout, value); err == nil {
			return true
		}
	}
	return false
}

type NotebookStructureFixture struct {
	Directory  string
	DBPath     string
	SourcePath string
}

// BuildNotebookStructureFixture 仅创建私密的全新系统临时目录，没有任意目标 DB
// 或真实源写入口。它证明结构转换，不是整站恢复/生产 Apply，也不赋 version 2。
func BuildNotebookStructureFixture(ctx context.Context, s Snapshot, decisions NotebookStructureDecisions) (fixture NotebookStructureFixture, err error) {
	preview, err := PreviewNotebookStructure(s, decisions)
	if err != nil {
		return fixture, err
	}
	if preview.Blocking() {
		return fixture, fmt.Errorf("notebook structure has unresolved metadata or source conflicts")
	}
	dir, err := os.MkdirTemp("", "stellar-notebook-structure-")
	if err != nil {
		return fixture, err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dir)
		}
	}()
	fixture = NotebookStructureFixture{Directory: dir, DBPath: filepath.Join(dir, "fixture.db"), SourcePath: filepath.Join(dir, "source.private.json")}
	archive := struct {
		SourceDigest string                     `json:"source_digest"`
		Tables       map[string]Table           `json:"tables"`
		Decisions    NotebookStructureDecisions `json:"decisions"`
	}{s.Digest, map[string]Table{"notebooks": s.Tables["notebooks"], "notes": s.Tables["notes"]}, decisions}
	if err = WritePrivate(fixture.SourcePath, archive); err != nil {
		return NotebookStructureFixture{}, err
	}
	st, err := store.New(fixture.DBPath)
	if err != nil {
		return NotebookStructureFixture{}, err
	}
	defer func() {
		if closeErr := st.Close(); err == nil && closeErr != nil {
			err = closeErr
			fixture = NotebookStructureFixture{}
		}
	}()
	// Ensure 只运行在空新库；旧源表没有传入这个可写连接。
	if err = st.EnsureNotebooksTable(); err != nil {
		return NotebookStructureFixture{}, err
	}
	if err = st.EnsureNotesTable(); err != nil {
		return NotebookStructureFixture{}, err
	}
	tx, err := st.DB().BeginTx(ctx, nil)
	if err != nil {
		return NotebookStructureFixture{}, err
	}
	defer tx.Rollback()
	if err = insertNotebookStructure(ctx, tx, preview); err != nil {
		return NotebookStructureFixture{}, err
	}
	if err = tx.Commit(); err != nil {
		return NotebookStructureFixture{}, err
	}
	return fixture, nil
}
func insertNotebookStructure(ctx context.Context, tx *sql.Tx, p NotebookStructurePreview) error {
	for _, table := range []string{"notebooks", "notes"} {
		columns := notebookStructureColumns[table]
		query := `INSERT INTO ` + table + ` (` + strings.Join(columns, ",") + `) VALUES (` + strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",") + `)`
		for _, row := range p.Rows[table] {
			values := make([]any, len(columns))
			for i, column := range columns {
				values[i] = row[column]
			}
			if _, err := tx.ExecContext(ctx, query, values...); err != nil {
				return fmt.Errorf("insert notebook structure: %w", err)
			}
		}
	}
	return nil
}
