package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrPostAuthorNotFound indicates that a post author reference does not map to
// an existing users.id. New writes fail loudly instead of creating an
// ownership row that cannot be checked later.
var ErrPostAuthorNotFound = errors.New("post author not found")

const legacyPostAuthorsTable = "post_authors_legacy"

// EnsurePostAuthorsTable brings the article-author association to its current
// schema. It must run after EnsureUsersTable so legacy usernames can be
// resolved to users.id.
//
// Old installations stored the author username in post_authors.author_id.
// Those rows are copied to the integer table when possible. Rows that cannot
// be resolved are retained in post_authors_legacy and remain visible through
// posts.author; they are never silently discarded.
func (s *Store) EnsurePostAuthorsTable() error {
	typeName, exists, err := s.postAuthorsColumnType()
	if err != nil {
		return err
	}
	if !exists {
		if err := s.createPostAuthorsTable("post_authors"); err != nil {
			return err
		}
	} else if !isIntegerColumn(typeName) {
		if err := s.migrateLegacyPostAuthors(); err != nil {
			return err
		}
	}

	if err := s.ensurePostAuthorsIndex(); err != nil {
		return err
	}
	if err := s.reconcileLegacyPostAuthors(); err != nil {
		return err
	}
	return s.backfillPostAuthorsFromPosts()
}

// ensurePostAuthorsTableCreated is used by Store.New, before the application
// has necessarily created users. A legacy TEXT table is deliberately left in
// place until EnsurePostAuthorsTable can run with users available.
func (s *Store) ensurePostAuthorsTableCreated() error {
	typeName, exists, err := s.postAuthorsColumnType()
	if err != nil {
		return err
	}
	if !exists {
		return s.createPostAuthorsTable("post_authors")
	}
	if isIntegerColumn(typeName) {
		return s.ensurePostAuthorsIndex()
	}
	return nil
}

func (s *Store) postAuthorsColumnType() (string, bool, error) {
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'post_authors'`).Scan(&exists); err != nil {
		return "", false, fmt.Errorf("check post_authors table: %w", err)
	}
	if exists == 0 {
		return "", false, nil
	}
	rows, err := s.db.Query(`PRAGMA table_info(post_authors)`)
	if err != nil {
		return "", true, fmt.Errorf("table info post_authors: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, pk int
		var name, typeName string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &typeName, &notNull, &defaultValue, &pk); err != nil {
			return "", true, fmt.Errorf("scan post_authors table info: %w", err)
		}
		if name == "author_id" {
			return typeName, true, rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return "", true, fmt.Errorf("iterate post_authors table info: %w", err)
	}
	return "", true, errors.New("post_authors.author_id column is missing")
}

func (s *Store) tableExists(table string) (bool, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&n); err != nil {
		return false, fmt.Errorf("check %s table: %w", table, err)
	}
	return n > 0, nil
}

func isIntegerColumn(typeName string) bool {
	return strings.Contains(strings.ToUpper(strings.TrimSpace(typeName)), "INT")
}

func (s *Store) createPostAuthorsTable(table string) error {
	if table != "post_authors" && table != "post_authors_new" {
		return fmt.Errorf("invalid post authors table %q", table)
	}
	ddl := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		post_id    INTEGER NOT NULL,
		author_id  INTEGER NOT NULL,
		sort_order INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (post_id, author_id),
		FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
		FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE RESTRICT
	)`, table)
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create %s table: %w", table, err)
	}
	return nil
}

func (s *Store) ensurePostAuthorsIndex() error {
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_post_authors_author_order ON post_authors(author_id, sort_order, post_id)`); err != nil {
		return fmt.Errorf("create idx_post_authors_author_order: %w", err)
	}
	return nil
}

func (s *Store) ensureLegacyPostAuthorsTable(tx *sql.Tx) error {
	_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS post_authors_legacy (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		post_id     INTEGER NOT NULL,
		author_ref  TEXT NOT NULL,
		sort_order  INTEGER NOT NULL DEFAULT 0,
		reason      TEXT NOT NULL DEFAULT '',
		UNIQUE(post_id, author_ref, sort_order)
	)`)
	if err != nil {
		return fmt.Errorf("create post_authors_legacy table: %w", err)
	}
	return nil
}

func (s *Store) migrateLegacyPostAuthors() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin post authors migration: %w", err)
	}
	defer tx.Rollback()

	if err := s.ensureLegacyPostAuthorsTable(tx); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE IF EXISTS post_authors_new`); err != nil {
		return fmt.Errorf("reset post authors migration table: %w", err)
	}
	if _, err := tx.Exec(`CREATE TABLE post_authors_new (
		post_id    INTEGER NOT NULL,
		author_id  INTEGER NOT NULL,
		sort_order INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (post_id, author_id),
		FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
		FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE RESTRICT
	)`); err != nil {
		return fmt.Errorf("create migrated post authors table: %w", err)
	}

	if _, err := tx.Exec(`INSERT OR IGNORE INTO post_authors_legacy (post_id, author_ref, sort_order, reason)
		SELECT pa.post_id, TRIM(pa.author_id), pa.sort_order,
			'legacy username does not match users.username'
		FROM post_authors pa
		LEFT JOIN users u ON u.username = TRIM(pa.author_id)
		WHERE u.id IS NULL`); err != nil {
		return fmt.Errorf("quarantine unmapped post authors: %w", err)
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO post_authors_new (post_id, author_id, sort_order)
		SELECT pa.post_id, u.id, pa.sort_order
		FROM post_authors pa
		JOIN users u ON u.username = TRIM(pa.author_id)`); err != nil {
		return fmt.Errorf("map legacy post authors: %w", err)
	}
	if _, err := tx.Exec(`DROP INDEX IF EXISTS idx_post_authors_author_order`); err != nil {
		return fmt.Errorf("drop legacy post authors index: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE post_authors`); err != nil {
		return fmt.Errorf("drop legacy post authors table: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE post_authors_new RENAME TO post_authors`); err != nil {
		return fmt.Errorf("rename migrated post authors table: %w", err)
	}
	if _, err := tx.Exec(`CREATE INDEX idx_post_authors_author_order ON post_authors(author_id, sort_order, post_id)`); err != nil {
		return fmt.Errorf("create migrated post authors index: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit post authors migration: %w", err)
	}
	return nil
}

func (s *Store) reconcileLegacyPostAuthors() error {
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, legacyPostAuthorsTable).Scan(&exists); err != nil {
		return fmt.Errorf("check legacy post authors table: %w", err)
	}
	if exists == 0 {
		return nil
	}
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO post_authors (post_id, author_id, sort_order)
		SELECT l.post_id, u.id, l.sort_order
		FROM post_authors_legacy l
		JOIN users u ON u.username = l.author_ref`); err != nil {
		return fmt.Errorf("reconcile legacy post authors: %w", err)
	}
	if _, err := s.db.Exec(`DELETE FROM post_authors_legacy
		WHERE EXISTS (SELECT 1 FROM users u WHERE u.username = post_authors_legacy.author_ref)`); err != nil {
		return fmt.Errorf("remove reconciled post authors: %w", err)
	}
	return nil
}

func (s *Store) backfillPostAuthorsFromPosts() error {
	if err := s.ensureLegacyPostAuthorsTableOutsideTx(); err != nil {
		return err
	}
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO post_authors_legacy (post_id, author_ref, sort_order, reason)
		SELECT p.id, TRIM(p.author), 0, 'posts.author does not match users.username'
		FROM posts p
		LEFT JOIN users u ON u.username = TRIM(p.author)
		WHERE TRIM(p.author) != '' AND u.id IS NULL`); err != nil {
		return fmt.Errorf("quarantine unmapped posts.author: %w", err)
	}
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO post_authors (post_id, author_id, sort_order)
		SELECT p.id, u.id, 0
		FROM posts p
		JOIN users u ON u.username = TRIM(p.author)
		WHERE TRIM(p.author) != ''`); err != nil {
		return fmt.Errorf("backfill post authors: %w", err)
	}
	return nil
}

func (s *Store) ensureLegacyPostAuthorsTableOutsideTx() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS post_authors_legacy (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		post_id     INTEGER NOT NULL,
		author_ref  TEXT NOT NULL,
		sort_order  INTEGER NOT NULL DEFAULT 0,
		reason      TEXT NOT NULL DEFAULT '',
		UNIQUE(post_id, author_ref, sort_order)
	)`)
	if err != nil {
		return fmt.Errorf("create post_authors_legacy table: %w", err)
	}
	return nil
}

func (s *Store) preparePostAuthors(p *Post) error {
	if p == nil {
		return nil
	}
	ids := append([]int64(nil), p.AuthorIDs...)
	if len(ids) == 0 {
		refs := normalizeAuthorRefs(p.Authors, p.Author)
		for _, ref := range refs {
			id, err := s.resolvePostAuthorRef(ref)
			if err != nil {
				return err
			}
			ids = append(ids, id)
		}
	}
	ids = normalizeAuthorIDs(ids)
	usernames := make([]string, 0, len(ids))
	for _, id := range ids {
		user, err := s.GetUserByID(id)
		if err != nil {
			return fmt.Errorf("load post author %d: %w", id, err)
		}
		if user == nil {
			return fmt.Errorf("%w: %d", ErrPostAuthorNotFound, id)
		}
		usernames = append(usernames, user.Username)
	}
	p.AuthorIDs = ids
	p.Authors = usernames
	if len(usernames) > 0 {
		p.Author = usernames[0]
	} else {
		p.Author = ""
	}
	return nil
}

func normalizeAuthorRefs(authors []string, legacy string) []string {
	if len(authors) == 0 && strings.TrimSpace(legacy) != "" {
		authors = []string{legacy}
	}
	seen := make(map[string]struct{}, len(authors))
	out := make([]string, 0, len(authors))
	for _, author := range authors {
		author = strings.TrimSpace(author)
		if author == "" {
			continue
		}
		if _, ok := seen[author]; ok {
			continue
		}
		seen[author] = struct{}{}
		out = append(out, author)
	}
	return out
}

func normalizeAuthorIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (s *Store) resolvePostAuthorRef(ref string) (int64, error) {
	ref = strings.TrimSpace(ref)
	if id, err := strconv.ParseInt(ref, 10, 64); err == nil && id > 0 {
		user, lookupErr := s.GetUserByID(id)
		if lookupErr != nil {
			return 0, lookupErr
		}
		if user == nil {
			return 0, fmt.Errorf("%w: %s", ErrPostAuthorNotFound, ref)
		}
		return user.ID, nil
	}
	user, err := s.GetUserByUsername(ref)
	if err != nil {
		return 0, err
	}
	if user == nil {
		return 0, fmt.Errorf("%w: %s", ErrPostAuthorNotFound, ref)
	}
	return user.ID, nil
}

func (s *Store) resolvePostAuthorID(ref string) (int64, error) {
	return s.resolvePostAuthorRef(ref)
}

// HasPostAuthor reports whether userID is assigned to the article identified
// by slug. It intentionally includes trashed posts so restore/purge cannot be
// used to bypass the same ownership boundary.
func (s *Store) HasPostAuthor(slug string, userID int64) (bool, error) {
	if userID <= 0 {
		return false, nil
	}
	typeName, exists, err := s.postAuthorsColumnType()
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	var n int
	if isIntegerColumn(typeName) {
		err = s.db.QueryRow(`SELECT EXISTS(
			SELECT 1 FROM posts p JOIN post_authors pa ON pa.post_id = p.id
			WHERE p.slug = ? AND pa.author_id = ?
		)`, slug, userID).Scan(&n)
	} else {
		user, lookupErr := s.GetUserByID(userID)
		if lookupErr != nil || user == nil {
			return false, lookupErr
		}
		err = s.db.QueryRow(`SELECT EXISTS(
			SELECT 1 FROM posts p JOIN post_authors pa ON pa.post_id = p.id
			WHERE p.slug = ? AND pa.author_id = ?
		)`, slug, user.Username).Scan(&n)
	}
	if err != nil {
		return false, fmt.Errorf("check post author %s/%d: %w", slug, userID, err)
	}
	return n != 0, nil
}

// HasRevisionAuthor is the revision equivalent of HasPostAuthor and avoids
// loading a revision belonging to another author before returning 403.
func (s *Store) HasRevisionAuthor(revisionID, userID int64) (bool, error) {
	if revisionID <= 0 || userID <= 0 {
		return false, nil
	}
	var n int
	err := s.db.QueryRow(`SELECT EXISTS(
		SELECT 1 FROM revisions r
		JOIN posts p ON p.slug = r.post_slug
		JOIN post_authors pa ON pa.post_id = p.id
		WHERE r.id = ? AND pa.author_id = ?
	)`, revisionID, userID).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("check revision author %d/%d: %w", revisionID, userID, err)
	}
	return n != 0, nil
}

// HasShareAuthor applies article ownership to a share-link mutation.
func (s *Store) HasShareAuthor(shareID, userID int64) (bool, error) {
	if shareID <= 0 || userID <= 0 {
		return false, nil
	}
	var n int
	err := s.db.QueryRow(`SELECT EXISTS(
		SELECT 1 FROM shared_posts sp
		JOIN posts p ON p.slug = sp.post_slug
		JOIN post_authors pa ON pa.post_id = p.id
		WHERE sp.id = ? AND pa.author_id = ?
	)`, shareID, userID).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("check share author %d/%d: %w", shareID, userID, err)
	}
	return n != 0, nil
}
