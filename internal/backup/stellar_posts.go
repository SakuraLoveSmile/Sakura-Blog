package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"blog/internal/config"
	"blog/internal/maintenance"
	"blog/internal/post"
	"blog/internal/stellarv2"
	"blog/internal/store"
)

// BuildStellarPost 使用原始 front matter；不经过旧 FileMeta 字段白名单。
// 新导入归显式的 admin 账户，已有文章保留真实 ownership，不从署名猜用户。
func BuildStellarPost(st *store.Store, renderer *post.Service, slug string, source []byte) (*store.Post, error) {
	if !validStellarSourceComponent(slug, false) || !utf8.Valid(source) {
		return nil, fmt.Errorf("invalid Markdown source name or UTF-8")
	}
	configuration, err := config.LoadStellarV2(st)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", store.ErrStellarMigrationRequired, err)
	}
	existing, err := st.GetBySlug(slug)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.StellarVersion != 2 {
		return nil, fmt.Errorf("%w: posts/%s", store.ErrStellarMigrationRequired, slug)
	}
	registry, err := stellarPostRegistry(st)
	if err != nil {
		return nil, err
	}
	document, err := post.ParseStellarDocument(source, post.StellarDocumentOptions{Source: "posts/" + slug + ".md", Profile: "post", Kind: "posts", PagePath: "posts/" + slug + ".md", Registry: registry})
	if err != nil {
		return nil, err
	}
	p := &store.Post{Slug: slug, Status: store.StatusPublished}
	if existing != nil {
		*p = *existing
	} else {
		owner, err := st.GetUserByUsername("admin")
		if err != nil || owner == nil || owner.Role != store.RoleAdmin {
			return nil, fmt.Errorf("markdown import requires the configured admin identity")
		}
		p.AuthorIDs = []int64{owner.ID}
	}
	p.StellarVersion = 2
	encoded, err := json.Marshal(document.Raw)
	if err != nil {
		return nil, err
	}
	p.StellarConfig = string(encoded)
	encoded, err = json.Marshal(document.Blog)
	if err != nil {
		return nil, err
	}
	p.BlogExtensions, p.ContentMD = string(encoded), document.Body
	p.Title = stellarPostText(document.Raw, "title", slug)
	p.Excerpt = stellarPostText(document.Raw, "excerpt", "")
	p.Cover = stellarPostText(document.Raw, "cover", "")
	p.Tags = strings.Join(stellarPostStrings(document.Raw["tags"]), ",")
	p.Categories = stellarPostStrings(document.Raw["categories"])
	p.Category = ""
	if len(p.Categories) > 0 {
		p.Category = p.Categories[0]
	}
	if status, exists := document.Blog["status"]; exists {
		if status != store.StatusDraft && status != store.StatusPublished {
			return nil, fmt.Errorf("posts/%s: blog.status must be draft or published", slug)
		}
		p.Status = status.(string)
	}
	if date, exists := document.Raw["date"]; exists {
		text, ok := date.(string)
		if !ok {
			return nil, fmt.Errorf("posts/%s: date must be a valid timestamp", slug)
		}
		valid := false
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02"} {
			if parsed, err := time.Parse(layout, text); err == nil {
				p.CreatedAt, valid = parsed, true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("posts/%s: date must be a valid timestamp", slug)
		}
	}
	p.TopicID, p.Topic = 0, ""
	if membership, ok := document.Page["collection"].(map[string]any); ok && membership["profile"] == "topic" {
		id, _ := membership["id"].(string)
		topic, err := st.GetTopicBySlug(id)
		if err != nil || topic == nil {
			return nil, fmt.Errorf("posts/%s: topic collection missing", slug)
		}
		p.TopicID, p.Topic = topic.ID, topic.Name
	}
	if renderer == nil {
		renderer = post.NewService("")
	}
	publicData, _ := configuration.Blog["data"].(map[string]any)
	p.ContentHTML, err = renderer.RenderStellarV2WithContext(document.Body, post.StellarRenderContext{Theme: configuration.Theme, Data: publicData, Page: document.Page, Language: "zh-CN", BlogExtensions: true})
	return p, err
}

func stellarPostRegistry(st *store.Store) (*stellarv2.CollectionRegistry, error) {
	values := map[string]map[string]any{}
	topics, err := st.ListTopics()
	if err != nil {
		return nil, err
	}
	for _, value := range topics {
		parsed, err := value.ValidateStellar()
		if err != nil {
			return nil, err
		}
		values["topic/"+value.Slug] = parsed.Value
	}
	wikis, err := st.ListWikiProjects()
	if err != nil {
		return nil, err
	}
	for _, value := range wikis {
		parsed, err := value.ValidateStellar()
		if err != nil {
			return nil, err
		}
		values["wiki/"+value.ID] = parsed.Value
	}
	notebooks, err := st.ListNotebooks()
	if err != nil {
		return nil, err
	}
	for _, value := range notebooks {
		parsed, err := value.ValidateStellar()
		if err != nil {
			return nil, err
		}
		values["notebooks/"+value.ID] = parsed.Value
	}
	return stellarv2.CreateCollectionRegistry(values), nil
}

func stellarPostText(values map[string]any, field, fallback string) string {
	text, ok := values[field].(string)
	if !ok || text == "" {
		return fallback
	}
	return text
}

func stellarPostStrings(value any) []string {
	if text, ok := value.(string); ok {
		return []string{text}
	}
	out := []string{}
	if values, ok := value.([]any); ok {
		for _, value := range values {
			if text, ok := value.(string); ok {
				out = append(out, text)
			}
		}
	}
	return out
}

// StageStellarPosts 在私有副本构建完整候选；调用者不得传 live DB。
func StageStellarPosts(st *store.Store, renderer *post.Service, sources map[string][]byte, missingOnly bool) ([]string, error) {
	prepared := []*store.Post{}
	for slug, source := range sources {
		existing, err := st.GetBySlug(slug)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.StellarVersion != 2 {
			return nil, fmt.Errorf("%w: posts/%s", store.ErrStellarMigrationRequired, slug)
		}
		if existing != nil && missingOnly {
			continue
		}
		value, err := BuildStellarPost(st, renderer, slug, source)
		if err != nil {
			return nil, err
		}
		prepared = append(prepared, value)
	}
	imported := []string{}
	for _, value := range prepared {
		if err := st.Upsert(value); err != nil {
			return nil, err
		}
		imported = append(imported, value.Slug)
	}
	return imported, nil
}

func (s *Service) importStellarPosts(ctx context.Context, filename string, data []byte) (*ImportResult, error) {
	if _, err := config.LoadStellarV2(s.store); err != nil {
		return nil, fmt.Errorf("%w: %w", store.ErrStellarMigrationRequired, err)
	}
	sources := map[string][]byte{}
	if strings.HasSuffix(strings.ToLower(filename), ".md") {
		if filename != filepath.Base(filename) || len(data) > maxZipEntrySize {
			return nil, fmt.Errorf("invalid Markdown file name or size")
		}
		sources[strings.TrimSuffix(filename, filepath.Ext(filename))] = data
	} else if strings.HasSuffix(strings.ToLower(filename), ".zip") {
		reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, err
		}
		if len(reader.File) > maxZipEntries {
			return nil, fmt.Errorf("markdown archive has too many members")
		}
		var total int
		for _, entry := range reader.File {
			if entry.Name != filepath.Base(entry.Name) || !strings.HasSuffix(strings.ToLower(entry.Name), ".md") || !entry.Mode().IsRegular() || entry.Mode()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("markdown archive member rejected; multi-kind archives require the manifest protocol")
			}
			slug := strings.TrimSuffix(entry.Name, filepath.Ext(entry.Name))
			if _, duplicate := sources[slug]; duplicate {
				return nil, fmt.Errorf("duplicate Markdown source")
			}
			member, err := entry.Open()
			if err != nil {
				return nil, err
			}
			body, err := io.ReadAll(io.LimitReader(member, maxZipEntrySize+1))
			_ = member.Close()
			total += len(body)
			if err != nil || len(body) > maxZipEntrySize || total > 500<<20 {
				return nil, fmt.Errorf("markdown archive exceeds size limit or cannot be read")
			}
			sources[slug] = body
		}
	} else {
		return nil, fmt.Errorf("only Markdown or portable ZIP content is supported")
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("markdown archive has no sources")
	}
	directory, err := os.MkdirTemp(filepath.Dir(s.store.DBPath()), ".stellar-import-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(directory)
	snapshot := filepath.Join(directory, "blog.db")
	if err := s.store.BackupTo(snapshot); err != nil {
		return nil, err
	}
	probe, err := store.OpenForRecovery(snapshot)
	if err != nil {
		return nil, err
	}
	defer probe.Close()
	imported, err := StageStellarPosts(probe, s.posts, sources, false)
	if err != nil {
		return nil, err
	}
	candidateService := New(Config{Store: probe, BackupDir: filepath.Join(directory, "export")})
	var complete bytes.Buffer
	if err := candidateService.WriteStellarContentZip(&complete); err != nil {
		return nil, err
	}
	manifest, _, err := ReadStellarContentZip(complete.Bytes())
	if err != nil {
		return nil, err
	}
	ids := map[int64]bool{}
	for _, slug := range imported {
		value, err := probe.GetBySlug(slug)
		if err != nil {
			return nil, err
		}
		ids[value.ID] = true
	}
	selected := []StellarContentRecord{}
	for _, record := range manifest.Records {
		if stellarCollectionTable(record.Table) || record.Table == "categories" || record.Table == "wiki_project_tags" {
			selected = append(selected, record)
		} else if record.Table == "posts" {
			id, _ := stellarRecordInteger(record, "id")
			if ids[id] {
				selected = append(selected, record)
			}
		} else if record.Table == "post_authors" || record.Table == "post_categories" {
			id, _ := stellarRecordInteger(record, "post_id")
			if ids[id] {
				selected = append(selected, record)
			}
		}
	}
	manifest.Records = selected
	neededAuthors := map[int64]bool{}
	for _, record := range selected {
		if record.Table == "notebooks" || record.Table == "notes" || record.Table == "post_authors" {
			id, _ := stellarRecordInteger(record, "author_id")
			neededAuthors[id] = true
		}
	}
	authors := []StellarAuthorIdentity{}
	for _, author := range manifest.Authors {
		if neededAuthors[author.ID] {
			authors = append(authors, author)
		}
	}
	manifest.Authors = authors
	portable, err := MarshalStellarContentZip(manifest)
	if err != nil {
		return nil, err
	}
	maintenance.ReleaseRequest(ctx)
	task, err := s.StartRestore(ctx, RestoreInput{Data: portable, ContentImport: true, ContentMergeDefinitions: true})
	if err != nil {
		return nil, err
	}
	if err := s.WaitRestore(ctx, task.ID); err != nil {
		return nil, err
	}
	return &ImportResult{Imported: imported, Skipped: []string{}}, nil
}

func MarshalStellarContentZip(manifest StellarContentManifest) ([]byte, error) {
	sources, err := stellarContentSources(manifest)
	if err != nil {
		return nil, err
	}
	sources[StellarContentManifestName], err = json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for name, source := range sources {
		entry, err := writer.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write(source); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
