package stellarcli

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/version"
	"io"
	"sort"
	"strings"

	"blog/internal/config"
	"blog/internal/stellarv2"
	"blog/internal/store"
	"github.com/goccy/go-yaml"
)

//go:embed data/widgets.yml
var widgetsYAML []byte

type Diagnostic struct {
	stellarv2.Issue
	Widget    string   `json:"widget,omitempty"`
	Layout    string   `json:"layout,omitempty"`
	Region    string   `json:"region,omitempty"`
	Supported []string `json:"supported,omitempty"`
}
type Checked struct {
	ThemeConfig bool `json:"themeConfig"`
	Collections int  `json:"collections"`
	Pages       int  `json:"pages"`
}
type DoctorResult struct {
	OK          bool         `json:"ok"`
	Environment Environment  `json:"environment"`
	Checked     Checked      `json:"checked"`
	Issues      []Diagnostic `json:"issues"`
	Warnings    []Diagnostic `json:"warnings"`
}

func diagnostic(code, source, path, actual, expected, migration string) Diagnostic {
	return Diagnostic{Issue: stellarv2.Issue{Code: code, Source: source, Path: path, ActualType: actual, Expected: expected, Migration: migration}}
}

// Doctor 在只读事务内核验数据库真值，不恢复旧字段或推断缺失迁移版本。
func Doctor(ctx context.Context, st *store.Store, env Environment) DoctorResult {
	r := DoctorResult{Environment: env, Issues: []Diagnostic{}, Warnings: []Diagnostic{}}
	if !version.IsValid(env.Go) || !version.IsValid(env.RequiredGo) || version.Compare(env.Go, env.RequiredGo) < 0 {
		r.Issues = append(r.Issues, diagnostic("unsupported_version", "environment", "go", env.Go, env.RequiredGo+" or newer", "start/requirements"))
	}
	if env.Adapter != "go-ssr-sqlite" || env.Theme != "2.0.0" || env.Commit != "9d11978a24fe6610e3a7dbab741cf62634539d05" {
		r.Issues = append(r.Issues, diagnostic("unsupported_version", "environment", "theme", "version or source mismatch", "locked Stellar 2.0.0 Go adapter", "start/requirements"))
	}
	if b, err := hex.DecodeString(env.ResourceSHA256); err != nil || len(b) != 32 {
		r.Issues = append(r.Issues, diagnostic("invalid_value", "environment", "resources", "missing fingerprint", "SHA-256 of embedded theme resources", "start/install"))
	}
	if st == nil {
		r.Issues = append(r.Issues, diagnostic("missing_database", "database", "root", "unavailable", "existing SQLite database", "start/install"))
		return r
	}
	legacy, err := st.DetectLegacyNotebookSchema()
	if err != nil {
		r.addError("notebook schema", err)
	} else {
		for _, table := range legacy.Tables {
			for _, issue := range table.Issues {
				r.Issues = append(r.Issues, diagnostic("migration_required", "sqlite:"+table.Table, "schema", "legacy SQLite schema", issue, "content-schema/migration"))
			}
		}
	}
	tx, err := st.DB().BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		r.addError("database", err)
		return r
	}
	defer tx.Rollback()
	snapshot := map[string]string{}
	rows, err := tx.QueryContext(ctx, `SELECT section,value FROM settings ORDER BY section`)
	if err != nil {
		r.addError("settings", err)
	} else {
		for rows.Next() {
			var k, v string
			if err := rows.Scan(&k, &v); err != nil {
				r.addError("settings", err)
				break
			}
			snapshot[k] = v
		}
		if err := rows.Err(); err != nil {
			r.addError("settings", err)
		}
		rows.Close()
	}
	r.Checked.ThemeConfig = snapshot[config.SectionStellarV2] != ""
	var theme map[string]any
	catalog := map[string]any{}
	if err := yaml.Unmarshal(widgetsYAML, &catalog); err != nil {
		r.addError("widgets", err)
	}
	canonical, err := config.LoadStellarV2Snapshot(snapshot)
	if err != nil {
		r.addError("settings.stellar_v2", err)
	} else {
		theme = canonical.Theme
		if data, ok := canonical.Blog["data"].(map[string]any); ok {
			if overrides, ok := data["widgets"].(map[string]any); ok {
				catalog = stellarv2.MergeWidgetCatalog(catalog, overrides)
			}
		}
		profiles, _ := theme["profiles"].(map[string]any)
		names := make([]string, 0, len(profiles))
		for name := range profiles {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, profile := range names {
			layer, _ := profiles[profile].(map[string]any)
			r.placement("settings.stellar_v2", profile, theme, catalog, layer)
		}
	}
	collections := map[string]map[string]any{}
	for _, spec := range []recordSpec{{"wiki_projects", "id", "wiki", true}, {"topics", "slug", "topic", true}, {"notebooks", "id", "notebook", true}} {
		for _, record := range r.records(ctx, tx, spec) {
			r.Checked.Collections++
			parsed, ok := r.parse(record, true)
			if !ok {
				continue
			}
			profileResult, err := stellarv2.ValidateCollectionProfile(parsed, record.source, spec.profile)
			if err != nil {
				r.addError(record.source, err)
				continue
			}
			r.addIssues(profileResult.Issues)
			key := spec.profile + "/" + record.id
			if spec.profile == "notebook" {
				key = "notebooks/" + record.id
			}
			collections[key] = profileResult.Value
			profile := spec.profile
			if profile == "notebook" {
				profile = "note"
			}
			profiles, _ := theme["profiles"].(map[string]any)
			layer, _ := profiles[profile].(map[string]any)
			r.placement(record.source, profile, theme, catalog, layer, profileResult.Value)
		}
	}
	registry := stellarv2.CreateCollectionRegistry(collections)
	for _, spec := range []recordSpec{{"posts", "slug", "post", false}, {"pages", "slug", "page", false}, {"wikidocs", "slug", "wiki", false}, {"notes", "slug", "notebook", false}} {
		for _, record := range r.records(ctx, tx, spec) {
			r.Checked.Pages++
			parsed, ok := r.parse(record, false)
			if !ok {
				continue
			}
			kind := "pages"
			if spec.profile == "post" {
				kind = "posts"
			}
			resolved := stellarv2.ResolveContentMembership(stellarv2.MembershipInput{Kind: kind, Source: record.source, PagePath: record.id, Config: parsed, Registry: registry})
			r.addIssues(resolved.Issues)
			if resolved.Config == nil {
				continue
			}
			profile := spec.profile
			var collection map[string]any
			if m, ok := resolved.Config["collection"].(map[string]any); ok {
				profile, _ = m["profile"].(string)
				id, _ := m["id"].(string)
				key := profile + "/" + id
				if profile == "notebook" {
					key = "notebooks/" + id
				}
				collection = collections[key]
			}
			final := stellarv2.ValidatePageProfile(resolved.Config, record.source, profile)
			r.addIssues(final.Issues)
			if profile == "notebook" {
				profile = "note"
			}
			if profile == "page" {
				if layout, ok := final.Value["layout"].(string); ok && layout != "" {
					profile = layout
				}
			}
			profiles, _ := theme["profiles"].(map[string]any)
			layer, _ := profiles[profile].(map[string]any)
			r.placement(record.source, profile, theme, catalog, layer, collection, final.Value)
		}
	}
	if err := tx.Commit(); err != nil {
		r.addError("database", err)
	}
	r.OK = len(r.Issues) == 0
	return r
}

func (r *DoctorResult) addIssues(issues []stellarv2.Issue) {
	for _, i := range issues {
		d := Diagnostic{Issue: i}
		if i.Severity == "warning" {
			r.Warnings = append(r.Warnings, d)
		} else {
			r.Issues = append(r.Issues, d)
		}
	}
}
func (r *DoctorResult) addError(source string, err error) {
	var schema *stellarv2.SchemaError
	var migration *config.MigrationRequiredError
	if errors.As(err, &schema) {
		r.addIssues(schema.Issues)
		return
	}
	if errors.As(err, &migration) {
		r.Issues = append(r.Issues, diagnostic("migration_required", migration.Source, migration.Path, "unmigrated", migration.Reason, "configuration/v2"))
		return
	}
	r.Issues = append(r.Issues, diagnostic("invalid_document", source, "root", "unreadable or invalid document", "current SQLite schema and valid JSON objects", "configuration/v2"))
}

type recordSpec struct {
	table, id, profile string
	collection         bool
}
type doctorRecord struct {
	id, source, raw, blog string
	version               int
}

func (r *DoctorResult) records(ctx context.Context, tx *sql.Tx, spec recordSpec) []doctorRecord {
	// 标识符全部来自上方固定宿主表清单，不消费命令输入。
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(`+spec.table+`)`)
	if err != nil {
		r.addError(spec.table, err)
		return nil
	}
	cols := map[string]bool{}
	for rows.Next() {
		var cid, nn, pk int
		var name, kind string
		var def sql.NullString
		if err := rows.Scan(&cid, &name, &kind, &nn, &def, &pk); err != nil {
			rows.Close()
			r.addError(spec.table, err)
			return nil
		}
		cols[name] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		r.addError(spec.table, err)
		return nil
	}
	if !cols[spec.id] {
		r.Issues = append(r.Issues, diagnostic("missing_schema", spec.table, "root", "missing table or identity column", "current content schema", "content-schema/migration"))
		return nil
	}
	for _, required := range []string{"stellar_version", "stellar_config", "blog_extensions"} {
		if !cols[required] {
			r.Issues = append(r.Issues, diagnostic("missing_schema", spec.table, required, "missing column", "current versioned content schema", "content-schema/migration"))
		}
	}
	field := func(name, fallback string) string {
		if cols[name] {
			return name
		}
		return fallback
	}
	parent := "''"
	if spec.table == "wikidocs" {
		parent = field("project_id", "''")
	}
	if spec.table == "notes" {
		parent = field("notebook_id", "''")
	}
	query := `SELECT ` + spec.id + `,` + field("stellar_version", "0") + `,` + field("stellar_config", "''") + `,` + field("blog_extensions", "''") + `,` + parent + ` FROM ` + spec.table
	if cols["deleted_at"] {
		query += ` WHERE deleted_at IS NULL`
	}
	query += ` ORDER BY ` + spec.id
	rows, err = tx.QueryContext(ctx, query)
	if err != nil {
		r.addError(spec.table, err)
		return nil
	}
	defer rows.Close()
	out := []doctorRecord{}
	for rows.Next() {
		var v doctorRecord
		var parent string
		if err := rows.Scan(&v.id, &v.version, &v.raw, &v.blog, &parent); err != nil {
			r.addError(spec.table, err)
			break
		}
		v.source = "sqlite:" + spec.table + "/" + v.id
		if !spec.collection {
			switch spec.table {
			case "posts":
				v.source = "source/_posts/" + v.id + ".md"
			case "pages":
				v.source = "source/" + v.id + ".md"
			case "wikidocs":
				v.source = "source/wiki/" + parent + "/" + v.id + ".md"
			case "notes":
				v.source = "source/notebooks/" + parent + "/" + v.id + ".md"
			}
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		r.addError(spec.table, err)
	}
	return out
}
func decodeObject(raw string) (map[string]any, error) {
	d := json.NewDecoder(bytes.NewBufferString(raw))
	d.UseNumber()
	var out map[string]any
	if err := d.Decode(&out); err != nil {
		return nil, err
	}
	if out == nil {
		return nil, fmt.Errorf("expected object")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("expected one JSON object")
	}
	return out, nil
}
func (r *DoctorResult) parse(record doctorRecord, collection bool) (map[string]any, bool) {
	if record.version != 2 {
		r.Issues = append(r.Issues, diagnostic("migration_required", record.source, "stellar_version", "unmigrated", "migration version 2", "content-schema/migration"))
		return nil, false
	}
	raw, err := decodeObject(record.raw)
	if err != nil {
		r.addError(record.source, err)
		return nil, false
	}
	if _, err := decodeObject(record.blog); err != nil {
		r.addError(record.source+".blog", err)
		return nil, false
	}
	var parsed stellarv2.Result
	if collection {
		parsed, err = stellarv2.ParseCollection(raw, stellarv2.Options{Source: record.source})
	} else {
		parsed, err = stellarv2.ParsePage(raw, stellarv2.Options{Source: record.source})
	}
	if err != nil {
		r.addError(record.source, err)
		return nil, false
	}
	r.addIssues(parsed.Issues)
	return parsed.Value, true
}
func (r *DoctorResult) placement(source, profile string, theme, catalog map[string]any, layers ...map[string]any) {
	if theme == nil {
		return
	}
	state, _ := stellarv2.Get(theme, "leftbar.defaultState")
	defaultState, _ := state.(string)
	all := append([]map[string]any{theme}, layers...)
	resolved := stellarv2.ResolveRegions(stellarv2.RegionOptions{Profile: profile, DefaultState: defaultState, Catalog: catalog, Layers: all})
	warnings, _ := resolved["warnings"].([]any)
	for _, value := range warnings {
		m, _ := value.(map[string]any)
		code, _ := m["code"].(string)
		widget, _ := m["widget"].(string)
		layout, _ := m["layout"].(string)
		region, _ := m["region"].(string)
		supported := []string{}
		if list, ok := m["supported"].([]any); ok {
			for _, s := range list {
				if text, ok := s.(string); ok {
					supported = append(supported, text)
				}
			}
		}
		r.Warnings = append(r.Warnings, Diagnostic{Issue: stellarv2.Issue{Code: code, Severity: "warning", Source: source, Path: "layout=" + profile + "; region=" + region, Action: "skipped"}, Widget: widget, Layout: layout, Region: region, Supported: supported})
	}
}
func FormatDoctorText(r DoctorResult) string {
	status := "FAIL"
	if r.OK {
		status = "PASS"
	}
	lines := []string{"Stellar doctor: " + status, "Adapter: Go SSR / SQLite (Node.js and Hexo are not runtime dependencies)", "Go: " + r.Environment.Go + " (required " + r.Environment.RequiredGo + ")", "Stellar: " + r.Environment.Theme + " @ " + r.Environment.Commit, "Resources SHA-256: " + r.Environment.ResourceSHA256, fmt.Sprintf("Checked: themeConfig=%t, %d collections, %d content records", r.Checked.ThemeConfig, r.Checked.Collections, r.Checked.Pages)}
	if len(r.Issues) > 0 {
		lines = append(lines, fmt.Sprintf("Issues (%d):", len(r.Issues)))
	}
	for _, i := range r.Issues {
		lines = append(lines, fmt.Sprintf("- %s: %s (%s); actual=%s; expected=%s; migration=%v", i.Source, i.Path, i.Code, i.ActualType, i.Expected, i.Migration))
	}
	if len(r.Warnings) > 0 {
		lines = append(lines, fmt.Sprintf("Warnings (%d):", len(r.Warnings)))
	}
	for _, i := range r.Warnings {
		if i.Widget != "" {
			supported := strings.Join(i.Supported, ",")
			if supported == "" {
				supported = "none"
			}
			lines = append(lines, fmt.Sprintf("- Warning %s: Widget %s (layout=%s) does not support %s; supported=%s; skipped", i.Source, i.Widget, i.Layout, i.Region, supported))
		} else {
			lines = append(lines, fmt.Sprintf("- Warning %s: %s (%s); %s", i.Source, i.Path, i.Code, i.Action))
		}
	}
	return strings.Join(lines, "\n")
}
