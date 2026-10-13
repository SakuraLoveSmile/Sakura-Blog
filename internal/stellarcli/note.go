package stellarcli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"blog/internal/post"
	"blog/internal/store"
)

type NoteOptions struct {
	Notebook, Title, Tags, DBPath string
	Author                        int64
	GeneratedAt                   time.Time
	DryRun                        bool
}
type NotePlan struct {
	Notebook string   `json:"notebook"`
	Title    string   `json:"title"`
	Tags     []string `json:"tags"`
	AuthorID int64    `json:"authorId"`
	Source   string   `json:"source"`
	Target   string   `json:"target"`
	Content  string   `json:"content"`
	DryRun   bool     `json:"dryRun"`
	Created  bool     `json:"created"`
	ID       int64    `json:"id,omitempty"`
	note     store.Note
}

func SafeSegment(value, label string) (string, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return "", fmt.Errorf("%s 必须是非空字符串", label)
	}
	if v == "." || v == ".." || strings.ContainsAny(v, `<>:"/\|?*`) {
		return "", fmt.Errorf("%s 不能包含路径或文件系统保留字符", label)
	}
	for _, r := range v {
		if r < 32 {
			return "", fmt.Errorf("%s 不能包含路径或文件系统保留字符", label)
		}
	}
	return v, nil
}

func NormalizeTags(value string) ([]string, error) {
	tags := []string{}
	seen := map[string]bool{}
	for _, part := range strings.Split(value, ",") {
		tag := strings.TrimSpace(part)
		if tag == "" || seen[tag] {
			continue
		}
		for _, r := range tag {
			if r < 32 {
				return nil, fmt.Errorf("--tags 不能包含换行或空字符")
			}
		}
		tags = append(tags, tag)
		seen[tag] = true
	}
	return tags, nil
}

func BuildNotePlan(st *store.Store, options NoteOptions) (NotePlan, error) {
	var plan NotePlan
	notebook, err := SafeSegment(options.Notebook, "--notebook")
	if err != nil {
		return plan, err
	}
	title, err := SafeSegment(options.Title, "--title")
	if err != nil {
		return plan, err
	}
	tags, err := NormalizeTags(options.Tags)
	if err != nil {
		return plan, err
	}
	if options.GeneratedAt.IsZero() {
		return plan, fmt.Errorf("generatedAt 必须是合法日期")
	}
	nb, err := st.GetNotebook(notebook)
	if err != nil {
		return plan, err
	}
	if nb == nil {
		return plan, store.ErrNotebookNotFound
	}
	if _, err := nb.ValidateStellar(); err != nil {
		return plan, err
	}
	owner := options.Author
	if owner == 0 {
		owner = nb.AuthorID
	}
	if owner <= 0 {
		return plan, fmt.Errorf("notebook owner unknown: provide --author with an existing author id")
	}
	if nb.AuthorID > 0 && owner != nb.AuthorID {
		return plan, fmt.Errorf("--author must match Notebook owner")
	}
	user, err := st.GetUserByID(owner)
	if err != nil {
		return plan, err
	}
	if user == nil || user.Status != store.UserStatusActive || !hasContentWrite(user.Permissions) {
		return plan, fmt.Errorf("--author requires an active existing user with content:write permission")
	}
	if old, err := st.GetNoteBySlug(notebook, title); err != nil {
		return plan, err
	} else if old != nil {
		return plan, store.ErrNoteConflict
	}
	raw := map[string]any{"date": options.GeneratedAt.Format("2006-01-02 15:04"), "title": title, "collection": map[string]any{"profile": "notebook", "id": notebook}}
	if len(tags) > 0 {
		raw["tags"] = tags
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return plan, err
	}
	note := store.Note{NotebookID: notebook, Slug: title, Title: title, Status: store.StatusDraft, Tags: append([]string{}, tags...), AuthorID: owner, CreatedAt: options.GeneratedAt.UTC(), StellarVersion: 2, StellarConfig: string(encoded), BlogExtensions: "{}"}
	if _, err := note.ValidateStellar(); err != nil {
		return plan, err
	}
	content, err := post.SerializeStellarDocument(post.StellarDocument{Raw: raw, Body: "\n"})
	if err != nil {
		return plan, err
	}
	plan = NotePlan{Notebook: notebook, Title: title, Tags: append([]string{}, tags...), AuthorID: owner, Source: "sqlite:notebooks/" + notebook, Target: options.DBPath + "#notes/" + notebook + "/" + title, Content: string(content), DryRun: options.DryRun, note: note}
	return plan, nil
}

func hasContentWrite(permissions []string) bool {
	for _, p := range permissions {
		if p == "*" || p == store.PermissionContentWrite {
			return true
		}
	}
	return false
}

// WriteNotePlan 在单一事务内重新检查归属、作者权限、严格父配置和唯一目标；失败不留下草稿。
// SQLite 是内容真值，Content 是可导出的预览，不创建平行 Markdown 文件。
func WriteNotePlan(st *store.Store, plan *NotePlan) error {
	if plan == nil || plan.DryRun || plan.note.StellarVersion != 2 || plan.Created {
		return fmt.Errorf("invalid executable note plan")
	}
	tx, err := st.DB().Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var nb store.Notebook
	if err := tx.QueryRow(`SELECT id,author_id,stellar_version,stellar_config,blog_extensions FROM notebooks WHERE id=?`, plan.note.NotebookID).Scan(&nb.ID, &nb.AuthorID, &nb.StellarVersion, &nb.StellarConfig, &nb.BlogExtensions); err != nil {
		return err
	}
	if _, err := nb.ValidateStellar(); err != nil {
		return err
	}
	if nb.AuthorID > 0 && nb.AuthorID != plan.note.AuthorID {
		return fmt.Errorf("notebook owner changed since preview")
	}
	if err := validateAuthorTx(tx, plan.note.AuthorID); err != nil {
		return err
	}
	if _, err := plan.note.ValidateStellar(); err != nil {
		return err
	}
	tags, err := json.Marshal(plan.note.Tags)
	if err != nil {
		return err
	}
	p := plan.note
	result, err := tx.Exec(`INSERT INTO notes(notebook_id,slug,title,content_md,content_html,status,tags_json,author_id,created_at,updated_at,stellar_version,stellar_config,blog_extensions) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, p.NotebookID, p.Slug, p.Title, p.ContentMD, p.ContentHTML, p.Status, string(tags), p.AuthorID, p.CreatedAt, time.Now().UTC(), p.StellarVersion, p.StellarConfig, p.BlogExtensions)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return store.ErrNoteConflict
		}
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	plan.Created = true
	plan.ID = id
	return nil
}

func validateAuthorTx(tx *sql.Tx, id int64) error {
	var allowed int
	err := tx.QueryRow(`SELECT COUNT(*) FROM users u WHERE u.id=? AND u.status=? AND EXISTS (SELECT 1 FROM user_roles ur JOIN role_permissions rp ON rp.role=ur.role WHERE ur.user_id=u.id AND rp.permission IN (?, '*'))`, id, store.UserStatusActive, store.PermissionContentWrite).Scan(&allowed)
	if err != nil {
		return err
	}
	if allowed == 0 {
		return fmt.Errorf("author no longer has active content-write permission")
	}
	return nil
}
