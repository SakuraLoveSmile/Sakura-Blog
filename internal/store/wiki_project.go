package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// WikiHero describes the optional project landing-page hero. It deliberately
// lives in store so the Console/API layer does not depend on Stellar templates.
type WikiHero struct {
	Background  string             `json:"background,omitempty"`
	Animation   *WikiHeroAnimation `json:"animation,omitempty"`
	Title       string             `json:"title,omitempty"`
	Description string             `json:"description,omitempty"`
	Start       string             `json:"start,omitempty"`
	Preview     *WikiHeroPreview   `json:"preview,omitempty"`
	Actions     []WikiHeroAction   `json:"actions,omitempty"`
}

// WikiHeroAnimation is the structured Stellar 1.44 background animation
// configuration. Params are intentionally kept as JSON values so the frontend
// can apply the upstream per-field normalization without a database migration.
type WikiHeroAnimation struct {
	Type   string         `json:"type,omitempty"`
	Params map[string]any `json:"params,omitempty"`
}

type WikiHeroPreview struct {
	Type     string            `json:"type,omitempty"`
	Commands []WikiHeroCommand `json:"commands,omitempty"`
	Image    string            `json:"image,omitempty"`
}

type WikiHeroCommand struct {
	Codes []string `json:"codes,omitempty"`
}

type WikiHeroAction struct {
	Title string `json:"title,omitempty"`
	URL   string `json:"url,omitempty"`
	Icon  string `json:"icon,omitempty"`
}

// WikiProject is the database representation of a Wiki project.
type WikiProject struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Title       string    `json:"title"`
	Headline    string    `json:"headline"`
	Subtitle    string    `json:"subtitle"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	Cover       string    `json:"cover"`
	Available   string    `json:"available"`
	URL         string    `json:"url"`
	Repo        string    `json:"repo"`
	Branch      string    `json:"branch"`
	Sort        int       `json:"sort"`
	Pin         bool      `json:"pin"`
	Visible     bool      `json:"visible"`
	Hero        *WikiHero `json:"hero,omitempty"`
	Tags        []string  `json:"tags"`

	DocCount          int  `json:"doc_count,omitempty"`
	PublishedDocCount int  `json:"published_doc_count,omitempty"`
	HasDocs           bool `json:"has_docs,omitempty"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

var wikiProjectIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

var (
	ErrWikiProjectNotFound = errors.New("wiki project not found")
	ErrWikiProjectConflict = errors.New("wiki project conflict")
	ErrWikiProjectInvalid  = errors.New("invalid wiki project")
)

func NormalizeWikiProjectID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || !wikiProjectIDPattern.MatchString(id) || id == "tags" {
		return "", ErrWikiProjectInvalid
	}
	return id, nil
}

func encodeWikiHero(hero *WikiHero) string {
	if hero == nil {
		return ""
	}
	normalized := normalizeWikiHero(hero)
	b, err := json.Marshal(normalized)
	if err != nil {
		return ""
	}
	return string(b)
}

func decodeWikiHero(raw string) *WikiHero {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var hero WikiHero
	if json.Unmarshal([]byte(raw), &hero) != nil {
		return nil
	}
	hero = *normalizeWikiHero(&hero)
	if hero.Background == "" && hero.Animation == nil && hero.Title == "" && hero.Description == "" && hero.Start == "" && hero.Preview == nil && len(hero.Actions) == 0 {
		return nil
	}
	return &hero
}

// normalizeWikiHero converts the pre-1.44 background: galaxy shorthand to the
// structured animation field. The returned value is a copy, so callers can
// safely normalize data while rendering or persisting it.
func normalizeWikiHero(hero *WikiHero) *WikiHero {
	if hero == nil {
		return nil
	}
	out := *hero
	if hero.Animation != nil {
		animation := *hero.Animation
		animation.Type = strings.TrimSpace(animation.Type)
		if len(animation.Params) > 0 {
			animation.Params = cloneJSONMap(animation.Params)
		}
		out.Animation = &animation
	}
	if strings.EqualFold(strings.TrimSpace(out.Background), "galaxy") {
		out.Background = ""
		if out.Animation == nil || out.Animation.Type == "" {
			out.Animation = &WikiHeroAnimation{Type: "galaxy"}
		}
	}
	return &out
}

func cloneJSONMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return nil
	}
	out := make(map[string]any, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}

func (s *Store) EnsureWikiProjectsTable() error {
	return s.ensureTable("wiki_projects")
}

// MigrateWikiProjects imports the old site.yml/settings representation once.
// The marker is stored in settings so an empty import is also remembered.
func (s *Store) MigrateWikiProjects(source []*WikiProject) error {
	if err := s.EnsureSettingsTable(); err != nil {
		return err
	}
	if err := s.EnsureWikiProjectsTable(); err != nil {
		return err
	}
	marker, err := s.GetSetting("wiki_projects_migrated")
	if err != nil {
		return err
	}
	if marker != "1" {
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("begin wiki project migration: %w", err)
		}
		for _, project := range source {
			if project == nil {
				continue
			}
			id, err := NormalizeWikiProjectID(project.ID)
			if err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("wiki project %q: %w", project.ID, err)
			}
			tags := normalizeWikiTags(project.Tags)
			hero := encodeWikiHero(project.Hero)
			_, err = tx.Exec(`INSERT OR IGNORE INTO wiki_projects
				(id, name, title, headline, subtitle, description, icon, cover, available, url, repo, branch, sort, pin, visible, hero_json)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				id, project.Name, project.Title, project.Headline, project.Subtitle, project.Description,
				project.Icon, project.Cover, project.Available, project.URL, project.Repo, project.Branch,
				project.Sort, boolInt(project.Pin), boolInt(project.Visible), hero)
			if err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("insert wiki project %s: %w", id, err)
			}
			for i, tag := range tags {
				if _, err := tx.Exec(`INSERT OR IGNORE INTO wiki_project_tags (project_id, tag, position) VALUES (?, ?, ?)`, id, tag, i); err != nil {
					_ = tx.Rollback()
					return fmt.Errorf("insert wiki project tag %s/%s: %w", id, tag, err)
				}
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit wiki project migration: %w", err)
		}
		if err := s.PutSetting("wiki_projects_migrated", "1"); err != nil {
			return err
		}
	}
	return s.ensureWikiProjectPlaceholders()
}

func (s *Store) ensureWikiProjectPlaceholders() error {
	if err := s.EnsureWikidocsTable(); err != nil {
		return err
	}
	rows, err := s.db.Query(`SELECT DISTINCT project_id FROM wikidocs WHERE TRIM(project_id) != ''`)
	if err != nil {
		return fmt.Errorf("list wiki document projects: %w", err)
	}
	// Store 使用单连接 SQLite；先把项目 ID 读完并关闭 rows，再进行
	// QueryRow/INSERT，避免在持有查询游标时等待同一条连接而死锁。
	var projectIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		projectIDs = append(projectIDs, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range projectIDs {
		if _, err := NormalizeWikiProjectID(id); err != nil {
			continue
		}
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM wiki_projects WHERE id = ?`, id).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			if _, err := s.db.Exec(`INSERT INTO wiki_projects (id, name, title, visible) VALUES (?, ?, ?, 1)`, id, id, id); err != nil {
				return fmt.Errorf("create wiki project placeholder %s: %w", id, err)
			}
		}
	}
	return nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func normalizeWikiTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return out
}

const wikiProjectColumns = `id, name, title, headline, subtitle, description, icon, cover, available, url, repo, branch, sort, pin, visible, hero_json, created_at, updated_at`

func scanWikiProject(row interface{ Scan(...any) error }) (*WikiProject, error) {
	var p WikiProject
	var pin, visible int
	var hero string
	if err := row.Scan(&p.ID, &p.Name, &p.Title, &p.Headline, &p.Subtitle, &p.Description, &p.Icon, &p.Cover, &p.Available, &p.URL, &p.Repo, &p.Branch, &p.Sort, &pin, &visible, &hero, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	p.Pin = pin != 0
	p.Visible = visible != 0
	p.Hero = decodeWikiHero(hero)
	return &p, nil
}

func (s *Store) getWikiProjectByQuery(query string, args ...any) (*WikiProject, error) {
	p, err := scanWikiProject(s.db.QueryRow(`SELECT `+wikiProjectColumns+` FROM wiki_projects WHERE `+query+` LIMIT 1`, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get wiki project: %w", err)
	}
	if err := s.loadWikiProjectTags(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) GetWikiProject(id string) (*WikiProject, error) {
	return s.getWikiProjectByQuery("id = ?", id)
}

func (s *Store) ListWikiProjects() ([]*WikiProject, error) {
	rows, err := s.db.Query(`SELECT ` + wikiProjectColumns + ` FROM wiki_projects ORDER BY pin DESC, sort DESC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list wiki projects: %w", err)
	}
	var out []*WikiProject
	for rows.Next() {
		p, err := scanWikiProject(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// rows 必须先关闭再查询标签；Store 采用单连接 SQLite。
	for _, p := range out {
		if err := s.loadWikiProjectTags(p); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Store) ListVisibleWikiProjects() ([]*WikiProject, error) {
	projects, err := s.ListWikiProjects()
	if err != nil {
		return nil, err
	}
	out := projects[:0]
	for _, p := range projects {
		if p.Visible {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *Store) loadWikiProjectTags(p *WikiProject) error {
	rows, err := s.db.Query(`SELECT tag FROM wiki_project_tags WHERE project_id = ? ORDER BY position ASC, tag ASC`, p.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return err
		}
		p.Tags = append(p.Tags, tag)
	}
	return rows.Err()
}

func (s *Store) PopulateWikiProjectCounts(p *WikiProject) error {
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM wikidocs WHERE project_id = ?`, p.ID).Scan(&p.DocCount); err != nil {
		return err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM wikidocs WHERE project_id = ? AND status = ?`, p.ID, StatusPublished).Scan(&p.PublishedDocCount); err != nil {
		return err
	}
	p.HasDocs = p.PublishedDocCount > 0
	return nil
}

func (s *Store) ListWikiProjectsWithCounts() ([]*WikiProject, error) {
	projects, err := s.ListWikiProjects()
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		if err := s.PopulateWikiProjectCounts(p); err != nil {
			return nil, err
		}
	}
	return projects, nil
}

func (s *Store) CreateWikiProject(p *WikiProject) error {
	id, err := NormalizeWikiProjectID(p.ID)
	if err != nil {
		return err
	}
	p.ID = id
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		p.Name = p.ID
	}
	now := time.Now().UTC()
	_, err = s.db.Exec(`INSERT INTO wiki_projects (id, name, title, headline, subtitle, description, icon, cover, available, url, repo, branch, sort, pin, visible, hero_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Title, p.Headline, p.Subtitle, p.Description, p.Icon, p.Cover, p.Available, p.URL, p.Repo, p.Branch, p.Sort, boolInt(p.Pin), boolInt(p.Visible), encodeWikiHero(p.Hero), now, now)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return ErrWikiProjectConflict
		}
		return fmt.Errorf("create wiki project: %w", err)
	}
	return s.replaceWikiProjectTags(p.ID, p.Tags)
}

func (s *Store) UpdateWikiProject(id string, p *WikiProject) error {
	if _, err := NormalizeWikiProjectID(id); err != nil {
		return err
	}
	if p == nil {
		return ErrWikiProjectInvalid
	}
	if p.ID != "" && p.ID != id {
		return ErrWikiProjectInvalid
	}
	if strings.TrimSpace(p.Name) == "" {
		p.Name = id
	}
	res, err := s.db.Exec(`UPDATE wiki_projects SET name = ?, title = ?, headline = ?, subtitle = ?, description = ?, icon = ?, cover = ?, available = ?, url = ?, repo = ?, branch = ?, sort = ?, pin = ?, visible = ?, hero_json = ?, updated_at = ? WHERE id = ?`,
		p.Name, p.Title, p.Headline, p.Subtitle, p.Description, p.Icon, p.Cover, p.Available, p.URL, p.Repo, p.Branch, p.Sort, boolInt(p.Pin), boolInt(p.Visible), encodeWikiHero(p.Hero), time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("update wiki project %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrWikiProjectNotFound
	}
	p.ID = id
	return s.replaceWikiProjectTags(id, p.Tags)
}

func (s *Store) replaceWikiProjectTags(id string, tags []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM wiki_project_tags WHERE project_id = ?`, id); err != nil {
		_ = tx.Rollback()
		return err
	}
	for i, tag := range normalizeWikiTags(tags) {
		if _, err := tx.Exec(`INSERT INTO wiki_project_tags (project_id, tag, position) VALUES (?, ?, ?)`, id, tag, i); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) DeleteWikiProject(id string) error {
	p, err := s.GetWikiProject(id)
	if err != nil {
		return err
	}
	if p == nil {
		return ErrWikiProjectNotFound
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM wikidocs WHERE project_id = ?`, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrWikiProjectConflict
	}
	if _, err := s.db.Exec(`DELETE FROM wiki_project_tags WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM wiki_projects WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete wiki project %s: %w", id, err)
	}
	return nil
}

// WikiProjectTagNames returns all known tags in stable first-seen order.
func (s *Store) WikiProjectTagNames() ([]string, error) {
	rows, err := s.db.Query(`SELECT tag, MIN(position) AS first_position FROM wiki_project_tags GROUP BY tag ORDER BY first_position ASC, tag ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var tag string
		var ignored int
		if err := rows.Scan(&tag, &ignored); err != nil {
			return nil, err
		}
		out = append(out, tag)
	}
	return out, rows.Err()
}

// WikiProjectByTag filters visible projects without exposing tag storage to templates.
func WikiProjectByTag(projects []*WikiProject, tag string) []*WikiProject {
	if strings.TrimSpace(tag) == "" {
		return projects
	}
	var out []*WikiProject
	for _, p := range projects {
		for _, candidate := range p.Tags {
			if candidate == tag {
				out = append(out, p)
				break
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
