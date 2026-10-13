package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"blog/internal/config"
	"blog/internal/maintenance"
	"blog/internal/post"
	"blog/internal/stellarv2"
	"blog/internal/store"
)

const StellarContentManifestName = "blog-content-v2.json"

// StellarContentManifest 的 SQL 列值是便携协议，不接受任意表/列。
// 三个 canonical 字段以 JSON 字符串存储，保留原始 JSON 的全部字节。
type StellarContentManifest struct {
	Version int                     `json:"version"`
	Records []StellarContentRecord  `json:"records"`
	Authors []StellarAuthorIdentity `json:"authors"`
}

type StellarContentRecord struct {
	Table  string                     `json:"table"`
	Source string                     `json:"source,omitempty"`
	Values map[string]json.RawMessage `json:"values"`
}

type StellarAuthorIdentity struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

var stellarContentTables = []string{"topics", "wiki_projects", "notebooks", "posts", "pages", "wikidocs", "notes", "categories", "post_authors", "post_categories", "wiki_project_tags"}

func stellarContentTable(table string) bool {
	for _, known := range stellarContentTables {
		if known == table {
			return true
		}
	}
	return false
}

func stellarDocumentTable(table string) bool {
	return table == "posts" || table == "pages" || table == "wikidocs" || table == "notes"
}

func stellarCollectionTable(table string) bool {
	return table == "topics" || table == "wiki_projects" || table == "notebooks"
}

// WriteStellarContentZip 从一致 DB 快照导出全部内容/关联及 Markdown 源。
// 不导出 users 的密码/认证字段；只记录已引用 owner 的 ID 与用户名。
func (s *Service) WriteStellarContentZip(w io.Writer) error {
	if err := os.MkdirAll(s.backupDir, 0700); err != nil {
		return err
	}
	directory, err := os.MkdirTemp(s.backupDir, ".stellar-export-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	snapshot := filepath.Join(directory, "blog.db")
	if err := s.store.BackupTo(snapshot); err != nil {
		return err
	}
	probe, err := store.OpenForRecovery(snapshot)
	if err != nil {
		return err
	}
	defer probe.Close()
	manifest := StellarContentManifest{Version: 1, Records: []StellarContentRecord{}, Authors: []StellarAuthorIdentity{}}
	for _, table := range stellarContentTables {
		records, err := exportStellarTable(probe.DB(), table)
		if err != nil {
			return err
		}
		manifest.Records = append(manifest.Records, records...)
	}
	ids := map[int64]bool{}
	for _, record := range manifest.Records {
		if record.Table == "notes" || record.Table == "notebooks" || record.Table == "post_authors" {
			id, err := stellarRecordInteger(record, "author_id")
			if err != nil || id < 0 {
				return fmt.Errorf("invalid content author identity")
			}
			if id > 0 {
				ids[id] = true
			}
		}
	}
	for id := range ids {
		user, err := probe.GetUserByID(id)
		if err != nil || user == nil {
			return fmt.Errorf("content author identity is missing")
		}
		manifest.Authors = append(manifest.Authors, StellarAuthorIdentity{ID: id, Username: user.Username})
	}
	sort.Slice(manifest.Authors, func(i, j int) bool { return manifest.Authors[i].ID < manifest.Authors[j].ID })
	sources, err := stellarContentSources(manifest)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	sources[StellarContentManifestName] = encoded
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	zw := zip.NewWriter(w)
	for _, name := range names {
		entry, err := zw.Create(name)
		if err != nil {
			_ = zw.Close()
			return err
		}
		if _, err := entry.Write(sources[name]); err != nil {
			_ = zw.Close()
			return err
		}
	}
	return zw.Close()
}

func exportStellarTable(db *sql.DB, table string) ([]StellarContentRecord, error) {
	var present int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&present); err != nil {
		return nil, err
	}
	if present == 0 {
		return nil, nil
	}
	rows, err := db.Query(`SELECT * FROM ` + stellarQuote(table) + ` ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	records := []StellarContentRecord{}
	for rows.Next() {
		values := make([]any, len(columns))
		targets := make([]any, len(columns))
		for i := range values {
			targets[i] = &values[i]
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, err
		}
		record := StellarContentRecord{Table: table, Values: map[string]json.RawMessage{}}
		for i, key := range columns {
			if text, ok := values[i].(string); ok && !utf8.ValidString(text) {
				return nil, fmt.Errorf("content text is not UTF-8; use the native SQLite snapshot to retain its bytes")
			}
			if stamp, ok := values[i].(time.Time); ok {
				values[i] = stamp.Format(time.RFC3339Nano)
			}
			encoded, err := json.Marshal(values[i])
			if err != nil {
				return nil, err
			}
			record.Values[key] = encoded
		}
		record.Source, err = stellarRecordSource(record)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func stellarRecordSource(record StellarContentRecord) (string, error) {
	var source string
	switch record.Table {
	case "posts", "pages":
		source = record.Table + "/" + stellarRecordText(record, "slug") + ".md"
	case "wikidocs":
		source = "wiki/" + stellarRecordText(record, "project_id") + "/" + stellarRecordText(record, "slug") + ".md"
	case "notes":
		if !validStellarSourceComponent(stellarRecordText(record, "notebook_id"), false) || !validStellarSourceComponent(stellarRecordText(record, "slug"), true) {
			return "", fmt.Errorf("invalid note source identity")
		}
		source = "notebooks/" + stellarRecordText(record, "notebook_id") + "/" + stellarRecordText(record, "slug") + ".md"
	case "topics":
		source = "_data/topic/" + stellarRecordText(record, "slug") + ".json"
	case "wiki_projects":
		source = "_data/wiki/" + stellarRecordText(record, "id") + ".json"
	case "notebooks":
		if !validStellarSourceComponent(stellarRecordText(record, "id"), false) {
			return "", fmt.Errorf("invalid notebook source identity")
		}
		source = "_data/notebooks/" + stellarRecordText(record, "id") + ".json"
	default:
		return "", nil
	}
	if !validStellarContentPath(source) {
		return "", fmt.Errorf("invalid content source path")
	}
	return source, nil
}

func stellarContentSources(manifest StellarContentManifest) (map[string][]byte, error) {
	if manifest.Version != 1 || manifest.Records == nil || manifest.Authors == nil {
		return nil, fmt.Errorf("unsupported stellar content manifest")
	}
	sources := map[string][]byte{}
	identities := map[string]bool{}
	for _, record := range manifest.Records {
		if !stellarContentTable(record.Table) || record.Values == nil {
			return nil, fmt.Errorf("unknown content table or missing values")
		}
		identity, err := stellarRecordIdentity(record)
		if err != nil || identities[identity] {
			return nil, fmt.Errorf("invalid or duplicate content identity")
		}
		identities[identity] = true
		expectedSource, err := stellarRecordSource(record)
		if err != nil || expectedSource != record.Source {
			return nil, fmt.Errorf("content source does not match identity")
		}
		if expectedSource == "" {
			continue
		}
		if stellarRecordText(record, "id") == "" && (record.Table == "notebooks" || record.Table == "wiki_projects") {
			return nil, fmt.Errorf("content collection identity missing")
		}
		version, err := stellarRecordInteger(record, "stellar_version")
		if err != nil || version != 2 {
			return nil, fmt.Errorf("%w: %s", store.ErrStellarMigrationRequired, record.Source)
		}
		raw, err := stellarRecordObject(record, "stellar_config")
		if err != nil {
			return nil, err
		}
		blog := map[string]any{}
		if stellarRecordText(record, "blog_extensions") != "" {
			blog, err = stellarRecordObject(record, "blog_extensions")
			if err != nil {
				return nil, err
			}
		}
		if record.Table == "notes" {
			var tags []string
			if err := json.Unmarshal([]byte(stellarRecordText(record, "tags_json")), &tags); err != nil || tags == nil {
				return nil, fmt.Errorf("%s: note tags must be a JSON string array", record.Source)
			}
			note := &store.Note{NotebookID: stellarRecordText(record, "notebook_id"), Slug: stellarRecordText(record, "slug"), StellarVersion: 2, StellarConfig: stellarRecordText(record, "stellar_config"), BlogExtensions: stellarRecordText(record, "blog_extensions")}
			if _, err := note.ValidateStellar(); err != nil {
				return nil, err
			}
		}
		var source []byte
		if stellarCollectionTable(record.Table) {
			parsed, err := stellarv2.ParseCollection(raw, stellarv2.Options{Source: record.Source})
			if err != nil {
				return nil, err
			}
			profile := map[string]string{"topics": "topic", "wiki_projects": "wiki", "notebooks": "notebook"}[record.Table]
			if _, err := stellarv2.ValidateCollectionProfile(parsed.Value, record.Source, profile); err != nil {
				return nil, err
			}
			source = []byte(stellarRecordText(record, "stellar_config"))
		} else {
			// status 属于 Go 的发布状态，在独立 Blog 扩展镜像到 Markdown，
			// manifest 仍保存原 Blog JSON 字节，单篇重新导入也不会误发布草稿。
			status := stellarRecordText(record, "status")
			if status != store.StatusDraft && status != store.StatusPublished {
				return nil, fmt.Errorf("%s: unsupported content publication status", record.Source)
			}
			blog["status"] = status
			source, err = post.SerializeStellarDocument(post.StellarDocument{Raw: raw, Blog: blog, Body: stellarRecordText(record, "content_md")})
			if err != nil {
				return nil, err
			}
		}
		if _, exists := sources[record.Source]; exists {
			return nil, fmt.Errorf("duplicate content source")
		}
		sources[record.Source] = source
	}
	// 文档的父集合必须在同一个完整包中，不能靠目标库碰巧存在而省略。
	wikiIDs, notebookIDs, topicIDs := map[string]bool{}, map[string]bool{}, map[int64]bool{}
	postIDs, categoryIDs := map[int64]bool{}, map[int64]bool{}
	authorIDs := map[int64]bool{}
	for _, author := range manifest.Authors {
		if author.ID <= 0 || author.Username == "" || authorIDs[author.ID] {
			return nil, fmt.Errorf("invalid or duplicate content author identity")
		}
		authorIDs[author.ID] = true
	}
	for _, record := range manifest.Records {
		switch record.Table {
		case "wiki_projects":
			wikiIDs[stellarRecordText(record, "id")] = true
		case "notebooks":
			notebookIDs[stellarRecordText(record, "id")] = true
		case "topics":
			id, _ := stellarRecordInteger(record, "id")
			topicIDs[id] = true
		case "posts":
			id, _ := stellarRecordInteger(record, "id")
			postIDs[id] = true
		case "categories":
			id, _ := stellarRecordInteger(record, "id")
			categoryIDs[id] = true
		}
	}
	for _, record := range manifest.Records {
		if record.Table == "wikidocs" && !wikiIDs[stellarRecordText(record, "project_id")] || record.Table == "notes" && !notebookIDs[stellarRecordText(record, "notebook_id")] || record.Table == "wiki_project_tags" && !wikiIDs[stellarRecordText(record, "project_id")] {
			return nil, fmt.Errorf("content archive is missing a parent collection")
		}
		if record.Table == "posts" {
			id, _ := stellarRecordInteger(record, "topic_id")
			if id != 0 && !topicIDs[id] {
				return nil, fmt.Errorf("content archive is missing a post topic")
			}
		}
		if record.Table == "notes" || record.Table == "notebooks" || record.Table == "post_authors" {
			id, err := stellarRecordInteger(record, "author_id")
			if err != nil || id < 0 || id > 0 && !authorIDs[id] {
				return nil, fmt.Errorf("content archive is missing an author identity")
			}
		}
		if record.Table == "post_authors" || record.Table == "post_categories" {
			id, _ := stellarRecordInteger(record, "post_id")
			if !postIDs[id] {
				return nil, fmt.Errorf("content archive is missing a relation member")
			}
		}
		if record.Table == "post_categories" {
			id, _ := stellarRecordInteger(record, "category_id")
			if !categoryIDs[id] {
				return nil, fmt.Errorf("content archive is missing a category")
			}
		}
	}
	return sources, nil
}

func ReadStellarContentZip(data []byte) (StellarContentManifest, map[string][]byte, error) {
	var manifest StellarContentManifest
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return manifest, nil, err
	}
	if err := validateZipBounds(zr); err != nil {
		return manifest, nil, err
	}
	sources := map[string][]byte{}
	caseNames := map[string]bool{}
	var total int64
	for _, entry := range zr.File {
		if !validStellarContentPath(entry.Name) || entry.FileInfo().IsDir() || !entry.Mode().IsRegular() || entry.Mode()&os.ModeSymlink != 0 || caseNames[strings.ToLower(entry.Name)] {
			return manifest, nil, fmt.Errorf("invalid or duplicate archive member")
		}
		caseNames[strings.ToLower(entry.Name)] = true
		reader, err := entry.Open()
		if err != nil {
			return manifest, nil, err
		}
		content, readErr := io.ReadAll(io.LimitReader(reader, maxZipEntrySize+1))
		closeErr := reader.Close()
		if readErr != nil || closeErr != nil || len(content) > maxZipEntrySize {
			return manifest, nil, fmt.Errorf("archive member exceeds size limit or cannot be read")
		}
		total += int64(len(content))
		if total > 500<<20 {
			return manifest, nil, fmt.Errorf("content archive exceeds total size limit")
		}
		sources[entry.Name] = content
	}
	raw, exists := sources[StellarContentManifestName]
	if !exists {
		return manifest, nil, fmt.Errorf("content manifest missing")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return manifest, nil, fmt.Errorf("invalid content manifest")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return manifest, nil, fmt.Errorf("invalid trailing content manifest data")
	}
	expected, err := stellarContentSources(manifest)
	if err != nil {
		return manifest, nil, err
	}
	delete(sources, StellarContentManifestName)
	if len(expected) != len(sources) {
		return manifest, nil, fmt.Errorf("content archive has missing or undeclared members")
	}
	for name, source := range expected {
		if !bytes.Equal(sources[name], source) {
			return manifest, nil, fmt.Errorf("%s: source/configuration differs from manifest", name)
		}
	}
	return manifest, sources, nil
}

func hasStellarContentManifest(data []byte) bool {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return false
	}
	for _, entry := range zr.File {
		if entry.Name == StellarContentManifestName {
			return true
		}
	}
	return false
}

func validStellarContentPath(name string) bool {
	return name != "" && !strings.ContainsAny(name, "\\\x00") && !path.IsAbs(name) && path.Clean(name) == name && name != "." && name != ".." && !strings.HasPrefix(name, "../")
}

func validStellarSourceComponent(value string, nested bool) bool {
	if value == "" || strings.TrimSpace(value) != value || strings.ContainsAny(value, `<>:"\|?*`) || !nested && strings.Contains(value, "/") {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	for _, character := range value {
		if character < 32 {
			return false
		}
	}
	return true
}

func stellarRecordText(record StellarContentRecord, field string) string {
	var text string
	_ = json.Unmarshal(record.Values[field], &text)
	return text
}

func stellarRecordInteger(record StellarContentRecord, field string) (int64, error) {
	raw := bytes.TrimSpace(record.Values[field])
	if len(raw) == 0 || raw[0] == '"' {
		return 0, fmt.Errorf("invalid integer content column %s", field)
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return 0, fmt.Errorf("invalid integer content column %s", field)
	}
	return number.Int64()
}

func stellarRecordObject(record StellarContentRecord, field string) (map[string]any, error) {
	var value map[string]any
	decoder := json.NewDecoder(strings.NewReader(stellarRecordText(record, field)))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil || value == nil {
		return nil, fmt.Errorf("%s: %s must be a JSON object", record.Source, field)
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("%s: invalid trailing configuration", record.Source)
	}
	return value, nil
}

func stellarRecordIdentity(record StellarContentRecord) (string, error) {
	fields := []string{"id"}
	switch record.Table {
	case "post_authors":
		fields = []string{"post_id", "author_id"}
	case "post_categories":
		fields = []string{"post_id", "category_id"}
	case "wiki_project_tags":
		fields = []string{"project_id", "tag"}
	}
	parts := []string{record.Table}
	for _, field := range fields {
		raw, exists := record.Values[field]
		if !exists || bytes.Equal(raw, []byte("null")) {
			return "", fmt.Errorf("content identity missing")
		}
		var value any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil || decoder.Decode(new(any)) != io.EOF {
			return "", fmt.Errorf("invalid content identity")
		}
		if number, ok := value.(json.Number); ok {
			integer, err := number.Int64()
			if err != nil || integer <= 0 {
				return "", fmt.Errorf("invalid content primary identity")
			}
			value = integer
		} else if text, ok := value.(string); !ok || text == "" || field != "id" && field != "project_id" && field != "tag" || field == "id" && record.Table != "wiki_projects" && record.Table != "notebooks" {
			return "", fmt.Errorf("invalid content primary identity")
		}
		canonical, _ := json.Marshal(value)
		parts = append(parts, string(canonical))
	}
	return strings.Join(parts, "\x00"), nil
}

func stellarQuote(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func (s *Service) importStellarContent(ctx context.Context, data []byte) (*ImportResult, error) {
	manifest, _, err := ReadStellarContentZip(data)
	if err != nil {
		return nil, err
	}
	maintenance.ReleaseRequest(ctx)
	task, err := s.StartRestore(ctx, RestoreInput{Data: data, ContentImport: true})
	if err != nil {
		return nil, err
	}
	if err := s.WaitRestore(ctx, task.ID); err != nil {
		return nil, err
	}
	result := &ImportResult{Imported: []string{}, Skipped: []string{}}
	for _, record := range manifest.Records {
		if record.Source != "" {
			result.Imported = append(result.Imported, record.Source)
		}
	}
	return result, nil
}

func (s *Service) buildStellarImportCandidate(j *restoreJournal) error {
	data, err := readRegularFile(s.inputPath(j.ID, "content.zip"), 500<<20)
	if err != nil {
		return err
	}
	manifest, _, err := ReadStellarContentZip(data)
	if err != nil {
		return err
	}
	target := s.inputPath(j.ID, "blog.db")
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := s.store.BackupTo(target); err != nil {
		return err
	}
	probe, err := store.OpenForRecovery(target)
	if err != nil {
		return err
	}
	defer probe.Close()
	return applyStellarContentManifest(probe, manifest, s.posts, false, j.ContentMergeDefinitions)
}

// ApplyStellarContentManifest 只作用于调用者持有的隔离副本或启动 seed。
// 真正在线应用必须走 Service 的恢复任务/维护窗口，不能直接用于 live DB。
func ApplyStellarContentManifest(probe *store.Store, manifest StellarContentManifest, renderer *post.Service) error {
	return applyStellarContentManifest(probe, manifest, renderer, false, false)
}

func SeedStellarContentManifest(probe *store.Store, manifest StellarContentManifest, renderer *post.Service, missingOnly bool) error {
	return applyStellarContentManifest(probe, manifest, renderer, missingOnly, false)
}

func applyStellarContentManifest(probe *store.Store, manifest StellarContentManifest, renderer *post.Service, missingOnly, preserveDefinitions bool) error {
	if _, err := stellarContentSources(manifest); err != nil {
		return err
	}
	configuration, err := config.LoadStellarV2(probe)
	if err != nil {
		return err
	}
	for _, author := range manifest.Authors {
		user, err := probe.GetUserByID(author.ID)
		if err != nil || user == nil || user.Username != author.Username {
			return fmt.Errorf("archive author identity does not match target account; no identity was guessed")
		}
	}
	if renderer == nil {
		renderer = post.NewService("")
	}
	// 不改变预检调用者持有的 manifest 或其原始元数据。
	records := make([]StellarContentRecord, len(manifest.Records))
	for index, record := range manifest.Records {
		records[index] = record
		records[index].Values = make(map[string]json.RawMessage, len(record.Values))
		for key, value := range record.Values {
			records[index].Values[key] = value
		}
	}
	manifest.Records = records
	publicData, _ := configuration.Blog["data"].(map[string]any)
	for index := range manifest.Records {
		record := &manifest.Records[index]
		if !stellarDocumentTable(record.Table) {
			continue
		}
		raw, err := stellarRecordObject(*record, "stellar_config")
		if err != nil {
			return err
		}
		parsed, err := stellarv2.ParsePage(raw, stellarv2.Options{Source: record.Source})
		if err != nil {
			return err
		}
		rendered, err := renderer.RenderStellarV2WithContext(stellarRecordText(*record, "content_md"), post.StellarRenderContext{Theme: configuration.Theme, Data: publicData, Page: parsed.Value, Language: "zh-CN", BlogExtensions: true})
		if err != nil {
			return fmt.Errorf("%s: Markdown rendering failed: %w", record.Source, err)
		}
		record.Values["content_html"], _ = json.Marshal(rendered)
	}
	tx, err := probe.DB().Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`PRAGMA defer_foreign_keys=ON`); err != nil {
		return err
	}
	keptPosts, keptWiki := map[int64]bool{}, map[string]bool{}
	if missingOnly {
		for _, record := range manifest.Records {
			if record.Source == "" {
				continue
			}
			exists, err := stellarRecordExists(tx, record)
			if err != nil {
				return err
			}
			if exists && record.Table == "posts" {
				id, _ := stellarRecordInteger(record, "id")
				keptPosts[id] = true
			} else if exists && record.Table == "wiki_projects" {
				keptWiki[stellarRecordText(record, "id")] = true
			}
		}
	}
	// 替换包中成员的结构关联，包外内容不变。
	for _, record := range manifest.Records {
		if record.Table == "posts" {
			id, _ := stellarRecordInteger(record, "id")
			if keptPosts[id] {
				continue
			}
			for _, table := range []string{"post_authors", "post_categories"} {
				if _, err := tx.Exec(`DELETE FROM `+stellarQuote(table)+` WHERE post_id=?`, id); err != nil {
					return err
				}
			}
		}
		if record.Table == "wiki_projects" {
			if keptWiki[stellarRecordText(record, "id")] || preserveDefinitions {
				continue
			}
			if _, err := tx.Exec(`DELETE FROM wiki_project_tags WHERE project_id=?`, stellarRecordText(record, "id")); err != nil {
				return err
			}
		}
	}
	for _, table := range stellarContentTables {
		for _, record := range manifest.Records {
			if record.Table != table {
				continue
			}
			skipWrite := false
			if record.Table == "post_authors" || record.Table == "post_categories" {
				id, _ := stellarRecordInteger(record, "post_id")
				skipWrite = keptPosts[id]
			}
			if record.Table == "wiki_project_tags" && (keptWiki[stellarRecordText(record, "project_id")] || preserveDefinitions) {
				skipWrite = true
			}
			if err := applyStellarRecord(tx, record, missingOnly || preserveDefinitions && (stellarCollectionTable(record.Table) || record.Table == "categories"), skipWrite); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func applyStellarRecord(tx *sql.Tx, record StellarContentRecord, preserveExisting, skipWrite bool) error {
	rows, err := tx.Query(`SELECT name,type FROM pragma_table_info(?)`, record.Table)
	if err != nil {
		return err
	}
	columns := []string{}
	columnTypes := map[string]string{}
	for rows.Next() {
		var name, kind string
		if err := rows.Scan(&name, &kind); err != nil {
			_ = rows.Close()
			return err
		}
		columns = append(columns, name)
		columnTypes[name] = strings.ToUpper(kind)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(columns) != len(record.Values) {
		return fmt.Errorf("content table columns differ from current schema")
	}
	quoted, bindings, assignments := []string{}, []string{}, []string{}
	values := []any{}
	for _, column := range columns {
		raw, exists := record.Values[column]
		if !exists {
			return fmt.Errorf("content table column missing")
		}
		var value any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil || decoder.Decode(new(any)) != io.EOF {
			return fmt.Errorf("invalid content column JSON")
		}
		if number, ok := value.(json.Number); ok {
			integer, err := strconv.ParseInt(string(number), 10, 64)
			if err != nil {
				return fmt.Errorf("invalid integer content column")
			}
			value = integer
		}
		switch value.(type) {
		case nil, string, int64, bool:
		default:
			return fmt.Errorf("invalid content column type")
		}
		if value != nil {
			switch columnTypes[column] {
			case "TEXT", "DATETIME":
				text, ok := value.(string)
				if !ok {
					return fmt.Errorf("content text column has incompatible type")
				}
				if columnTypes[column] == "DATETIME" {
					if _, err := time.Parse(time.RFC3339Nano, text); err != nil {
						return fmt.Errorf("content timestamp has incompatible format")
					}
				}
			case "INTEGER":
				if _, ok := value.(int64); !ok {
					return fmt.Errorf("content integer column has incompatible type")
				}
			}
		}
		quoted = append(quoted, stellarQuote(column))
		bindings = append(bindings, "?")
		assignments = append(assignments, stellarQuote(column)+"=excluded."+stellarQuote(column))
		values = append(values, value)
	}
	if record.Table == "categories" {
		id, err := stellarRecordInteger(record, "id")
		if err != nil {
			return err
		}
		parent, err := stellarRecordInteger(record, "parent_id")
		if err != nil || parent < 0 {
			return fmt.Errorf("invalid category parent identity")
		}
		var existingID, existingParent int64
		var existingName string
		err = tx.QueryRow(`SELECT id,parent_id,name FROM categories WHERE id=? OR (parent_id=? AND name=?)`, id, parent, stellarRecordText(record, "name")).Scan(&existingID, &existingParent, &existingName)
		if err != sql.ErrNoRows && (err != nil || existingID != id || existingParent != parent || existingName != stellarRecordText(record, "name")) {
			return fmt.Errorf("category identity conflict; no identity was remapped")
		}
	} else if record.Source != "" {
		if err := checkStellarExistingIdentity(tx, record); err != nil {
			return err
		}
	}
	if preserveExisting && (record.Source != "" || record.Table == "categories") {
		if exists, err := stellarRecordExists(tx, record); err != nil {
			return err
		} else if exists {
			return nil
		}
	}
	if skipWrite {
		return nil
	}
	_, err = tx.Exec(`INSERT INTO `+stellarQuote(record.Table)+` (`+strings.Join(quoted, ",")+`) VALUES (`+strings.Join(bindings, ",")+`) ON CONFLICT DO UPDATE SET `+strings.Join(assignments, ","), values...)
	return err
}

func stellarRecordExists(tx *sql.Tx, record StellarContentRecord) (bool, error) {
	var id any
	if record.Table == "notebooks" || record.Table == "wiki_projects" {
		id = stellarRecordText(record, "id")
	} else {
		value, err := stellarRecordInteger(record, "id")
		if err != nil {
			return false, err
		}
		id = value
	}
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM `+stellarQuote(record.Table)+` WHERE id=?`, id).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func checkStellarExistingIdentity(tx *sql.Tx, record StellarContentRecord) error {
	var existingID, existingSource string
	var version int
	idRaw := record.Values["id"]
	var id any
	decoder := json.NewDecoder(bytes.NewReader(idRaw))
	decoder.UseNumber()
	if err := decoder.Decode(&id); err != nil {
		return err
	}
	if number, ok := id.(json.Number); ok {
		integer, err := number.Int64()
		if err != nil || integer <= 0 {
			return fmt.Errorf("invalid content primary identity")
		}
		id = integer
	}
	sourceExpression := "slug"
	if stellarCollectionTable(record.Table) && record.Table != "topics" {
		sourceExpression = "id"
	} else if record.Table == "notes" {
		sourceExpression = "notebook_id || '/' || slug"
	} else if record.Table == "wikidocs" {
		sourceExpression = "project_id || '/' || slug"
	}
	expectedSource := stellarRecordText(record, "slug")
	if record.Table == "notes" {
		expectedSource = stellarRecordText(record, "notebook_id") + "/" + expectedSource
	} else if record.Table == "wikidocs" {
		expectedSource = stellarRecordText(record, "project_id") + "/" + expectedSource
	} else if sourceExpression == "id" {
		expectedSource = stellarRecordText(record, "id")
	}
	err := tx.QueryRow(`SELECT CAST(id AS TEXT),`+sourceExpression+`,stellar_version FROM `+stellarQuote(record.Table)+` WHERE id=? OR `+sourceExpression+`=?`, id, expectedSource).Scan(&existingID, &existingSource, &version)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if version != 2 {
		return fmt.Errorf("%w: %s", store.ErrStellarMigrationRequired, record.Source)
	}
	if existingSource != expectedSource || fmt.Sprint(id) != existingID {
		return fmt.Errorf("content identity conflict; cross-collection moves and ID remapping are not allowed")
	}
	return nil
}
