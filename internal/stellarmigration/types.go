package stellarmigration

import (
	"blog/internal/store"
	"fmt"
)

type Mapping struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Behavior string `json:"behavior"`
}

// Alternatives are private, as their original source values may be sensitive.
// PublicReport exposes only source/target/code/message, never values or Markdown.
type Alternative struct {
	TargetSource string `json:"target_source,omitempty"`
	Name         string `json:"name"`
	ConfigJSON   string `json:"config_json,omitempty"`
	BlogJSON     string `json:"blog_json,omitempty"`
	Markdown     string `json:"markdown,omitempty"`
}
type Diagnostic struct {
	Code         string        `json:"code"`
	Source       string        `json:"source"`
	Path         string        `json:"path"`
	Message      string        `json:"message"`
	Blocking     bool          `json:"blocking"`
	Alternatives []Alternative `json:"alternatives,omitempty"`
}
type ThemeCandidate struct {
	Version      int          `json:"version"`
	ThemeJSON    string       `json:"theme_json"`
	DocumentJSON string       `json:"document_json"`
	BlogJSON     string       `json:"blog_json"`
	Preserved    bool         `json:"preserved"`
	Mappings     []Mapping    `json:"mappings"`
	Diagnostics  []Diagnostic `json:"diagnostics"`
}
type RecordCandidate struct {
	Source         string       `json:"source"`
	Table          string       `json:"table,omitempty"`
	ID             string       `json:"id"`
	Profile        string       `json:"profile"`
	CollectionID   string       `json:"collection_id,omitempty"`
	Path           string       `json:"path"`
	SourceDigest   string       `json:"source_digest"`
	StellarVersion int          `json:"stellar_version"`
	ConfigJSON     string       `json:"stellar_config"`
	BlogJSON       string       `json:"blog_extensions"`
	Markdown       string       `json:"markdown"`
	Document       string       `json:"document,omitempty"`
	Preserved      bool         `json:"preserved"`
	Mappings       []Mapping    `json:"mappings"`
	Diagnostics    []Diagnostic `json:"diagnostics"`
}
type Candidate struct {
	Version     int               `json:"version"`
	Source      Snapshot          `json:"source"`
	Theme       ThemeCandidate    `json:"theme"`
	Records     []RecordCandidate `json:"records"`
	Diagnostics []Diagnostic      `json:"diagnostics"`
}
type RecordReport struct {
	Source          string       `json:"source"`
	Profile         string       `json:"profile"`
	Path            string       `json:"path"`
	SourceDigest    string       `json:"source_digest"`
	ConfigDigest    string       `json:"config_digest"`
	MarkdownChanged bool         `json:"markdown_changed"`
	Preserved       bool         `json:"preserved"`
	Mappings        []Mapping    `json:"mappings"`
	Diagnostics     []Diagnostic `json:"diagnostics"`
}
type Report struct {
	SourceDigest     string                     `json:"source_digest"`
	Counts           map[string]int             `json:"counts"`
	SettingsMappings []Mapping                  `json:"settings_mappings"`
	Diagnostics      []Diagnostic               `json:"diagnostics"`
	Records          []RecordReport             `json:"records"`
	Blocking         int                        `json:"blocking"`
	NotebookSchema   store.LegacyNotebookSchema `json:"notebook_schema"`
}

func publicDiagnostics(ds []Diagnostic) []Diagnostic {
	out := clone(ds)
	for i := range out {
		out[i].Alternatives = nil
	}
	return out
}
func (c Candidate) PublicReport() Report {
	r := Report{SourceDigest: c.Source.Digest, Counts: map[string]int{}, SettingsMappings: c.Theme.Mappings, Diagnostics: publicDiagnostics(append(append([]Diagnostic{}, c.Diagnostics...), c.Theme.Diagnostics...)), Records: []RecordReport{}}
	r.NotebookSchema = c.Source.NotebookSchema
	for n, t := range c.Source.Tables {
		r.Counts[n] = len(t.Rows)
	}
	r.Counts["tracked_documents"] = len(c.Source.Files)
	for _, p := range c.Records {
		before := ""
		if p.Table != "" {
			for _, row := range c.Source.Tables[p.Table].Rows {
				if stringValue(row["id"]) == p.ID {
					before = body(row)
				}
			}
		} else {
			_, body, err := readLegacyDocument(c.Source.Files[p.Source])
			if err == nil {
				before = body
			} else {
				before = c.Source.Files[p.Source]
			}
		}
		r.Records = append(r.Records, RecordReport{p.Source, p.Profile, p.Path, p.SourceDigest, digest(p.ConfigJSON), before != p.Markdown, p.Preserved, p.Mappings, publicDiagnostics(p.Diagnostics)})
	}
	for _, d := range r.Diagnostics {
		if d.Blocking {
			r.Blocking++
		}
	}
	for _, p := range r.Records {
		for _, d := range p.Diagnostics {
			if d.Blocking {
				r.Blocking++
			}
		}
	}
	return r
}

// ValidateCandidate is a future executor's mandatory preflight. It rejects a
// stale source, unresolved conflicts and invalid schema.
// It never opens a writable DB. Schema migration must be a separate operation.
func ValidateCandidate(c Candidate, current Snapshot) error {
	if err := current.Verify(); err != nil {
		return err
	}
	if err := c.Source.Verify(); err != nil {
		return err
	}
	if c.Version != 2 || c.Source.Digest != current.Digest {
		return fmt.Errorf("migration source digest changed")
	}
	if c.PublicReport().Blocking > 0 {
		return fmt.Errorf("migration has unresolved conflicts")
	}
	return validateAll(c)
}
