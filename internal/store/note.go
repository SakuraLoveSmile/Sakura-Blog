package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"blog/internal/stellarv2"
)

const notesDDL = `CREATE TABLE IF NOT EXISTS notes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	notebook_id TEXT NOT NULL,
	slug TEXT NOT NULL,
	title TEXT NOT NULL DEFAULT '',
	content_md TEXT NOT NULL DEFAULT '',
	content_html TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','published')),
	tags_json TEXT NOT NULL DEFAULT '[]',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	stellar_version INTEGER NOT NULL DEFAULT 0,
	stellar_config TEXT NOT NULL DEFAULT '',
	blog_extensions TEXT NOT NULL DEFAULT '',
	UNIQUE(notebook_id,slug),
	FOREIGN KEY(notebook_id) REFERENCES notebooks(id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_notes_notebook_status ON notes(notebook_id,status);`

// Tags 原样保留顺序、重复、大小写；字符串中的 '/' 表示上游标签层级。
type Note struct {
	ID             int64     `json:"id"`
	NotebookID     string    `json:"notebook_id"`
	Slug           string    `json:"slug"`
	Title          string    `json:"title"`
	ContentMD      string    `json:"content_md"`
	ContentHTML    string    `json:"content_html"`
	Status         string    `json:"status"`
	Tags           []string  `json:"tags"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	StellarVersion int       `json:"stellar_version"`
	StellarConfig  string    `json:"stellar_config"`
	BlogExtensions string    `json:"blog_extensions"`
}

var (
	ErrNoteNotFound = errors.New("note not found")
	ErrNoteConflict = errors.New("note slug conflict")
	ErrNoteInvalid  = errors.New("invalid note")
)

func (p *Note) ValidateStellar() (stellarv2.Result, error) {
	result, err := validateStellarContent(p.StellarVersion, p.StellarConfig, p.BlogExtensions, "notebooks/"+p.NotebookID+"/"+p.Slug, "notebook", false)
	if err != nil {
		return result, err
	}
	if membership, ok := result.Value["collection"].(map[string]any); ok {
		if membership["profile"] != "notebook" || membership["id"] != p.NotebookID {
			return result, fmt.Errorf("note collection does not match notebook %s", p.NotebookID)
		}
	}
	return result, nil
}

func (p *Note) TableName() string { return "notes" }
func (p *Note) Columns() []string {
	return []string{"id", "notebook_id", "slug", "title", "content_md", "content_html", "status", "tags_json", "created_at", "updated_at", "stellar_version", "stellar_config", "blog_extensions"}
}
func (p *Note) Values() []any {
	tags := p.Tags
	if tags == nil {
		tags = []string{}
	}
	raw, _ := json.Marshal(tags)
	return []any{p.NotebookID, p.Slug, p.Title, p.ContentMD, p.ContentHTML, p.Status, string(raw), p.CreatedAt, p.UpdatedAt, p.StellarVersion, p.StellarConfig, p.BlogExtensions}
}
func (p *Note) SetID(id int64) { p.ID = id }
func (p *Note) ScanRow(row rowScanner) error {
	var tags string
	if err := row.Scan(&p.ID, &p.NotebookID, &p.Slug, &p.Title, &p.ContentMD, &p.ContentHTML, &p.Status, &tags, &p.CreatedAt, &p.UpdatedAt, &p.StellarVersion, &p.StellarConfig, &p.BlogExtensions); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(tags), &p.Tags); err != nil {
		return fmt.Errorf("note %d tags: %w", p.ID, err)
	}
	return nil
}

func (s *Store) EnsureNotesTable() error {
	if err := s.EnsureNotebooksTable(); err != nil {
		return err
	}
	return s.ensureTable("notes")
}

func validateNoteSource(p *Note) error {
	if p == nil {
		return ErrNoteInvalid
	}
	if id, err := normalizeNotebookID(p.NotebookID); err != nil || id != p.NotebookID {
		return ErrNoteInvalid
	}
	if p.Slug == "" || strings.ContainsAny(p.Slug, `<>:"\|?*`) {
		return ErrNoteInvalid
	}
	for _, part := range strings.Split(p.Slug, "/") {
		if part == "" || part == "." || part == ".." {
			return ErrNoteInvalid
		}
	}
	for _, c := range p.Slug {
		if c < 32 {
			return ErrNoteInvalid
		}
	}
	if p.Status != "" && p.Status != StatusDraft && p.Status != StatusPublished {
		return ErrNoteInvalid
	}
	_, err := p.ValidateStellar()
	return err
}

func (s *Store) CreateNote(p *Note) (int64, error) {
	if err := validateNoteSource(p); err != nil {
		return 0, err
	}
	if p.Status == "" {
		p.Status = StatusDraft
	}
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err := validateNoteNotebookTx(tx, p.NotebookID); err != nil {
		return 0, err
	}
	id, err := createStellarEntityTx(tx, p)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return 0, ErrNoteConflict
		}
		return 0, fmt.Errorf("create note: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	p.ID = id
	return id, nil
}

func (s *Store) UpdateNote(p *Note) error {
	if err := validateNoteSource(p); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var notebook string
	var created time.Time
	if err := tx.QueryRow(`SELECT notebook_id,created_at FROM notes WHERE id=?`, p.ID).Scan(&notebook, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoteNotFound
		}
		return fmt.Errorf("get note for update: %w", err)
	}
	// Notebook 是源身份；移动必须由单独迁移流程显式处理。
	if notebook != p.NotebookID {
		return ErrNoteInvalid
	}
	if err := validateNoteNotebookTx(tx, notebook); err != nil {
		return err
	}
	p.CreatedAt = created
	p.UpdatedAt = time.Now().UTC()
	if err := updateStellarEntityTx(tx, p, p.ID, p.StellarVersion); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return ErrNoteConflict
		}
		return err
	}
	return tx.Commit()
}

func (s *Store) GetNoteByID(id int64) (*Note, error) { return repo(s, &Note{}).GetByID(id) }
func (s *Store) GetNoteBySlug(notebookID, slug string) (*Note, error) {
	return repo(s, &Note{}).GetOne("notebook_id=? AND slug=?", notebookID, slug)
}
func (s *Store) GetPublishedNote(notebookID, slug string) (*Note, error) {
	return repo(s, &Note{}).GetOne("notebook_id=? AND slug=? AND status=?", notebookID, slug, StatusPublished)
}
func (s *Store) ListNotes(notebookID string) ([]*Note, error) {
	return repo(s, &Note{}).List(Where("notebook_id=?", notebookID), OrderBy("substr(updated_at,1,19) DESC,id DESC"))
}
func (s *Store) ListPublishedNotes(notebookID string) ([]*Note, error) {
	return repo(s, &Note{}).List(Where("notebook_id=? AND status=?", notebookID, StatusPublished), OrderBy("substr(updated_at,1,19) DESC,id DESC"))
}
func (s *Store) ListNotesByTag(notebookID, tag string, publishedOnly bool) ([]*Note, error) {
	var notes []*Note
	var err error
	if publishedOnly {
		notes, err = s.ListPublishedNotes(notebookID)
	} else {
		notes, err = s.ListNotes(notebookID)
	}
	if err != nil {
		return nil, err
	}
	if tag == "" {
		return notes, nil
	}
	id := notebookTagID(tag)
	out := []*Note{}
	for _, note := range notes {
		for _, value := range note.Tags {
			candidate := notebookTagID(value)
			if candidate == id || strings.HasPrefix(candidate, id+"/") {
				out = append(out, note)
				break
			}
		}
	}
	return out, nil
}
func (s *Store) DeleteNote(id int64) error { return repo(s, &Note{}).Delete(id) }

func validateNoteNotebookTx(tx *sql.Tx, id string) error {
	nb, err := scanNotebook(tx.QueryRow(`SELECT `+notebookColumns+` FROM notebooks WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotebookNotFound
	}
	if err != nil {
		return err
	}
	_, err = nb.ValidateStellar()
	return err
}

func notebookTagID(value string) string {
	parts := []string{}
	for _, part := range strings.Split(value, "/") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.ToLower(strings.Join(parts, "/"))
}
