package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"blog/internal/stellarv2"
)

const notebooksDDL = `CREATE TABLE IF NOT EXISTS notebooks (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	icon TEXT NOT NULL DEFAULT '',
	cover TEXT NOT NULL DEFAULT '',
	sort INTEGER NOT NULL DEFAULT 0,
	author_id INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	stellar_version INTEGER NOT NULL DEFAULT 0,
	stellar_config TEXT NOT NULL DEFAULT '',
	blog_extensions TEXT NOT NULL DEFAULT ''
);`

// Notebook.ID 是集合源身份，路径及展示覆盖仍以 StellarConfig 为准。
type Notebook struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Icon           string    `json:"icon"`
	Cover          string    `json:"cover"`
	Sort           int       `json:"sort"`
	AuthorID       int64     `json:"author_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	StellarVersion int       `json:"stellar_version"`
	StellarConfig  string    `json:"stellar_config"`
	BlogExtensions string    `json:"blog_extensions"`
}

var (
	ErrNotebookNotFound = errors.New("notebook not found")
	ErrNotebookConflict = errors.New("notebook conflict")
	ErrNotebookInUse    = errors.New("notebook still has notes")
	ErrNotebookInvalid  = errors.New("invalid notebook")
)

func (p *Notebook) ValidateStellar() (stellarv2.Result, error) {
	return validateStellarContent(p.StellarVersion, p.StellarConfig, p.BlogExtensions, "notebooks/"+p.ID, "notebook", true)
}

func normalizeNotebookID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `<>:"/\|?*`) {
		return "", ErrNotebookInvalid
	}
	for _, c := range id {
		if c < 32 {
			return "", ErrNotebookInvalid
		}
	}
	return id, nil
}

func (s *Store) EnsureNotebooksTable() error {
	if err := s.ensureTable("notebooks"); err != nil {
		return err
	}
	return s.ensureNotebookOwnership("notebooks")
}

const notebookColumns = `id,name,description,icon,cover,sort,author_id,created_at,updated_at,stellar_version,stellar_config,blog_extensions`

func scanNotebook(row rowScanner) (*Notebook, error) {
	p := &Notebook{}
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.Icon, &p.Cover, &p.Sort, &p.AuthorID, &p.CreatedAt, &p.UpdatedAt, &p.StellarVersion, &p.StellarConfig, &p.BlogExtensions); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) GetNotebook(id string) (*Notebook, error) {
	p, err := scanNotebook(s.db.QueryRow(`SELECT `+notebookColumns+` FROM notebooks WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get notebook %s: %w", id, err)
	}
	return p, nil
}

func (s *Store) ListNotebooks() ([]*Notebook, error) {
	rows, err := s.db.Query(`SELECT ` + notebookColumns + ` FROM notebooks ORDER BY sort,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Notebook{}
	for rows.Next() {
		p, err := scanNotebook(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) CreateNotebook(p *Notebook) error {
	if p == nil || p.AuthorID < 0 {
		return ErrNotebookInvalid
	}
	id, err := normalizeNotebookID(p.ID)
	if err != nil {
		return err
	}
	p.ID = id
	if _, err := p.ValidateStellar(); err != nil {
		return err
	}
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	_, err = s.db.Exec(`INSERT INTO notebooks (`+notebookColumns+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, p.ID, p.Name, p.Description, p.Icon, p.Cover, p.Sort, p.AuthorID, p.CreatedAt, p.UpdatedAt, p.StellarVersion, p.StellarConfig, p.BlogExtensions)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return ErrNotebookConflict
		}
		return fmt.Errorf("create notebook: %w", err)
	}
	return nil
}

func (s *Store) UpdateNotebook(id string, p *Notebook) error {
	if p == nil || p.AuthorID < 0 || (p.ID != "" && p.ID != id) {
		return ErrNotebookInvalid
	}
	if _, err := normalizeNotebookID(id); err != nil {
		return err
	}
	p.ID = id
	if _, err := p.ValidateStellar(); err != nil {
		return err
	}
	now := time.Now().UTC()
	result, err := s.db.Exec(`UPDATE notebooks SET name=?,description=?,icon=?,cover=?,sort=?,author_id=?,updated_at=?,stellar_version=?,stellar_config=?,blog_extensions=? WHERE id=? AND (stellar_version=0 OR stellar_version=?)`, p.Name, p.Description, p.Icon, p.Cover, p.Sort, p.AuthorID, now, p.StellarVersion, p.StellarConfig, p.BlogExtensions, id, p.StellarVersion)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotebookNotFound
	}
	p.UpdatedAt = now
	return nil
}

// DeleteNotebook 不级联删除笔记，避免删除集合时隐式丢失正文。
func (s *Store) DeleteNotebook(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM notebooks WHERE id=?`, id).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return ErrNotebookNotFound
	}
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM notes WHERE notebook_id=?`, id).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return ErrNotebookInUse
	}
	if _, err := tx.Exec(`DELETE FROM notebooks WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}
