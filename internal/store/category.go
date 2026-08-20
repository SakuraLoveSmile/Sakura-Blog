package store

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// CategoryNode 是前台、Console 和公开 API 共用的递归分类节点。
// Path 是从顶级分类开始、以 / 分隔的完整路径。
type CategoryNode struct {
	ID       int64           `json:"id"`
	Name     string          `json:"name"`
	Path     string          `json:"path"`
	ParentID int64           `json:"parent_id"`
	Count    int             `json:"count"`
	Children []*CategoryNode `json:"children"`
}

var (
	ErrCategoryNotFound = errors.New("category not found")
	ErrCategoryConflict = errors.New("category conflict")
	ErrCategoryInvalid  = errors.New("invalid category")
)

const categoryDDL = `CREATE TABLE IF NOT EXISTS categories (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	name       TEXT NOT NULL,
	parent_id  INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE (parent_id, name)
);
CREATE INDEX IF NOT EXISTS idx_categories_parent ON categories(parent_id);
CREATE TABLE IF NOT EXISTS post_categories (
	post_id    INTEGER NOT NULL,
	category_id INTEGER NOT NULL,
	position   INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (post_id, category_id),
	FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
	FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_post_categories_category_position ON post_categories(category_id, position, post_id);`

// migrateCategories 建立分类实体及文章关联，并把旧 posts.category 迁移为顶级分类。
// INSERT OR IGNORE 使启动迁移可以安全重复执行。
func (s *Store) migrateCategories() error {
	if _, err := s.db.Exec(categoryDDL); err != nil {
		return fmt.Errorf("create category tables: %w", err)
	}
	rows, err := s.db.Query(`SELECT id, category FROM posts WHERE TRIM(category) != ''`)
	if err != nil {
		return fmt.Errorf("scan legacy categories: %w", err)
	}
	type legacyPost struct {
		id   int64
		name string
	}
	var legacy []legacyPost
	for rows.Next() {
		var p legacyPost
		if err := rows.Scan(&p.id, &p.name); err != nil {
			rows.Close()
			return fmt.Errorf("scan legacy category: %w", err)
		}
		legacy = append(legacy, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, p := range legacy {
		paths := []string{strings.TrimSpace(p.name)}
		ids, err := s.ensureCategoryPaths(paths)
		if err != nil {
			return fmt.Errorf("migrate category %q: %w", p.name, err)
		}
		if len(ids) == 0 {
			continue
		}
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO post_categories (post_id, category_id, position) VALUES (?, ?, 0)`, p.id, ids[0]); err != nil {
			return fmt.Errorf("backfill post category %d: %w", p.id, err)
		}
	}
	return nil
}

func validateCategoryName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, "/") {
		return "", ErrCategoryInvalid
	}
	return name, nil
}

func splitCategoryPath(path string) ([]string, error) {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil, ErrCategoryInvalid
	}
	parts := strings.Split(path, "/")
	for i, part := range parts {
		var err error
		parts[i], err = validateCategoryName(part)
		if err != nil {
			return nil, err
		}
	}
	return parts, nil
}

func (s *Store) ensureCategoryPaths(paths []string) ([]int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	ids := make([]int64, 0, len(paths))
	for _, path := range paths {
		parts, err := splitCategoryPath(path)
		if err != nil {
			return nil, err
		}
		parent := int64(0)
		var id int64
		for _, name := range parts {
			err := tx.QueryRow(`SELECT id FROM categories WHERE parent_id = ? AND name = ?`, parent, name).Scan(&id)
			if err == sql.ErrNoRows {
				res, insertErr := tx.Exec(`INSERT INTO categories (name, parent_id, created_at, updated_at) VALUES (?, ?, ?, ?)`, name, parent, time.Now().UTC(), time.Now().UTC())
				if insertErr != nil {
					return nil, fmt.Errorf("create category %q: %w", name, insertErr)
				}
				id, err = res.LastInsertId()
				if err != nil {
					return nil, err
				}
			} else if err != nil {
				return nil, err
			}
			parent = id
		}
		ids = append(ids, id)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return ids, nil
}

// ReplacePostCategories 替换文章的完整分类关联。paths 顺序即 position 顺序。
// 空 paths 会清空关联；为了兼容旧调用方，legacy 会在 paths 为空时作为单分类回退。
func (s *Store) ReplacePostCategories(postID int64, paths []string, legacy string) error {
	paths = normalizeCategoryPaths(paths)
	if len(paths) == 0 && strings.TrimSpace(legacy) != "" {
		paths = []string{strings.TrimSpace(legacy)}
	}
	ids, err := s.ensureCategoryPaths(paths)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM post_categories WHERE post_id = ?`, postID); err != nil {
		return err
	}
	for position, id := range ids {
		if _, err := tx.Exec(`INSERT INTO post_categories (post_id, category_id, position) VALUES (?, ?, ?)`, postID, id, position); err != nil {
			return err
		}
	}
	legacyValue := ""
	if len(paths) > 0 {
		legacyValue = paths[0]
	}
	if _, err := tx.Exec(`UPDATE posts SET category = ? WHERE id = ?`, legacyValue, postID); err != nil {
		return err
	}
	return tx.Commit()
}

func normalizeCategoryPaths(paths []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.Trim(path, " /")
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	return out
}

func (s *Store) hydratePostCategories(posts []*Post) error {
	if len(posts) == 0 {
		return nil
	}
	all, err := s.allCategoryRecords()
	if err != nil {
		return err
	}
	byID := make(map[int64]string, len(all))
	for _, c := range all {
		byID[c.id] = c.path
	}
	placeholders := make([]string, len(posts))
	args := make([]any, len(posts))
	byPostID := make(map[int64]*Post, len(posts))
	for i, p := range posts {
		placeholders[i] = "?"
		args[i] = p.ID
		byPostID[p.ID] = p
		p.Categories = nil
	}
	rows, err := s.db.Query(`SELECT pc.post_id, pc.category_id FROM post_categories pc WHERE pc.post_id IN (`+strings.Join(placeholders, ",")+
		`) ORDER BY pc.post_id, pc.position, pc.category_id`, args...)
	if err != nil {
		return fmt.Errorf("load post categories: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var postID, categoryID int64
		if err := rows.Scan(&postID, &categoryID); err != nil {
			return err
		}
		if p := byPostID[postID]; p != nil {
			if path := byID[categoryID]; path != "" {
				p.Categories = append(p.Categories, path)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, p := range posts {
		if len(p.Categories) > 0 {
			p.Category = p.Categories[0]
		}
	}
	return nil
}

type categoryRecord struct {
	id       int64
	name     string
	parentID int64
	path     string
}

func (s *Store) allCategoryRecords() ([]categoryRecord, error) {
	rows, err := s.db.Query(`SELECT id, name, parent_id FROM categories`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []categoryRecord
	for rows.Next() {
		var c categoryRecord
		if err := rows.Scan(&c.id, &c.name, &c.parentID); err != nil {
			return nil, err
		}
		records = append(records, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	byID := make(map[int64]*categoryRecord, len(records))
	for i := range records {
		byID[records[i].id] = &records[i]
	}
	var path func(*categoryRecord, map[int64]bool) string
	path = func(c *categoryRecord, visiting map[int64]bool) string {
		if c.path != "" {
			return c.path
		}
		if visiting[c.id] {
			return c.name
		}
		visiting[c.id] = true
		if parent := byID[c.parentID]; parent != nil {
			c.path = path(parent, visiting) + "/" + c.name
		} else {
			c.path = c.name
		}
		delete(visiting, c.id)
		return c.path
	}
	for i := range records {
		path(&records[i], map[int64]bool{})
	}
	return records, nil
}

// CategoryTree 返回完整分类树，Count 是当前节点及其所有后代的已发布文章数。
func (s *Store) CategoryTree() ([]*CategoryNode, error) {
	records, err := s.allCategoryRecords()
	if err != nil {
		return nil, err
	}
	nodes := make(map[int64]*CategoryNode, len(records))
	for _, c := range records {
		nodes[c.id] = &CategoryNode{ID: c.id, Name: c.name, Path: c.path, ParentID: c.parentID, Children: []*CategoryNode{}}
	}
	for _, c := range records {
		n := nodes[c.id]
		if parent := nodes[c.parentID]; parent != nil {
			parent.Children = append(parent.Children, n)
		}
	}
	for _, n := range nodes {
		sort.Slice(n.Children, func(i, j int) bool { return n.Children[i].Path < n.Children[j].Path })
	}
	for _, n := range nodes {
		var count int
		if err := s.db.QueryRow(`WITH RECURSIVE descendants(id) AS (
			SELECT ? UNION ALL SELECT c.id FROM categories c JOIN descendants d ON c.parent_id = d.id
		) SELECT COUNT(DISTINCT pc.post_id) FROM post_categories pc
		JOIN posts p ON p.id = pc.post_id
		WHERE pc.category_id IN (SELECT id FROM descendants) AND `+publishedWhere, n.ID, StatusPublished).Scan(&count); err != nil {
			return nil, err
		}
		n.Count = count
	}
	roots := make([]*CategoryNode, 0)
	for _, n := range nodes {
		if nodes[n.ParentID] == nil {
			roots = append(roots, n)
		}
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].Path < roots[j].Path })
	return roots, nil
}

// FlattenCategoryTree returns nodes in complete-path order for old clients.
func FlattenCategoryTree(roots []*CategoryNode) []*CategoryNode {
	var out []*CategoryNode
	var walk func([]*CategoryNode)
	walk = func(nodes []*CategoryNode) {
		for _, n := range nodes {
			out = append(out, n)
			walk(n.Children)
		}
	}
	walk(roots)
	return out
}

func (s *Store) categoryIDByPath(path string) (int64, error) {
	path = strings.Trim(path, "/")
	records, err := s.allCategoryRecords()
	if err != nil {
		return 0, err
	}
	for _, c := range records {
		if c.path == path {
			return c.id, nil
		}
	}
	return 0, ErrCategoryNotFound
}

// ListByCategoryPath 返回分类节点及其后代分类下的已发布文章，结果去重。
func (s *Store) ListByCategoryPath(path string) ([]*Post, error) {
	id, err := s.categoryIDByPath(path)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT DISTINCT `+listCols+` FROM posts p
		JOIN post_categories pc ON pc.post_id = p.id
		WHERE p.`+publishedWhere+` AND pc.category_id IN (
			WITH RECURSIVE descendants(id) AS (SELECT ? UNION ALL SELECT c.id FROM categories c JOIN descendants d ON c.parent_id = d.id) SELECT id FROM descendants
		) `+publishedOrder, StatusPublished, id)
	if err != nil {
		return nil, fmt.Errorf("list posts by category %s: %w", path, err)
	}
	defer rows.Close()
	return s.scanPosts(rows)
}

func (s *Store) CountByCategory() (map[string]int, error) {
	roots, err := s.CategoryTree()
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, n := range FlattenCategoryTree(roots) {
		out[n.Path] = n.Count
	}
	return out, nil
}

// GetCategoryByID resolves an API category ID.
func (s *Store) GetCategoryByID(id int64) (*CategoryNode, error) {
	roots, err := s.CategoryTree()
	if err != nil {
		return nil, err
	}
	for _, n := range FlattenCategoryTree(roots) {
		if n.ID == id {
			return n, nil
		}
	}
	return nil, ErrCategoryNotFound
}

// CreateCategory creates one node under parentID. parentID=0 means top-level.
func (s *Store) CreateCategory(name string, parentID int64) (*CategoryNode, error) {
	name, err := validateCategoryName(name)
	if err != nil {
		return nil, err
	}
	if parentID > 0 {
		if _, err := s.GetCategoryByID(parentID); err != nil {
			return nil, err
		}
	}
	res, err := s.db.Exec(`INSERT INTO categories (name, parent_id, updated_at) VALUES (?, ?, ?)`, name, parentID, time.Now().UTC())
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, ErrCategoryConflict
		}
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetCategoryByID(id)
}

// UpdateCategory renames and/or moves a node while preventing cycles and sibling duplicates.
func (s *Store) UpdateCategory(id int64, name string, parentID int64) (*CategoryNode, error) {
	name, err := validateCategoryName(name)
	if err != nil {
		return nil, err
	}
	current, err := s.GetCategoryByID(id)
	if err != nil {
		return nil, err
	}
	if parentID == id {
		return nil, ErrCategoryConflict
	}
	if parentID > 0 {
		parent, err := s.GetCategoryByID(parentID)
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(parent.Path+"/", current.Path+"/") || parent.Path == current.Path {
			return nil, ErrCategoryConflict
		}
	}
	if _, err := s.db.Exec(`UPDATE categories SET name = ?, parent_id = ?, updated_at = ? WHERE id = ?`, name, parentID, time.Now().UTC(), id); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, ErrCategoryConflict
		}
		return nil, err
	}
	updated, err := s.GetCategoryByID(id)
	if err != nil {
		return nil, err
	}
	// 兼容字段只代表主分类；其余分类由关联表决定。更新路径时同步主分类
	// 文本，避免旧 API 读到过期路径。
	_, _ = s.db.Exec(`UPDATE posts SET category = ? WHERE category = ?`, updated.Path, current.Path)
	return updated, nil
}

// DeleteCategoryByID only deletes empty leaves. It returns the number of removed associations (always 0 for valid leaves).
func (s *Store) DeleteCategoryByID(id int64) error {
	if _, err := s.GetCategoryByID(id); err != nil {
		return err
	}
	var children, posts int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM categories WHERE parent_id = ?`, id).Scan(&children); err != nil {
		return err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM post_categories WHERE category_id = ?`, id).Scan(&posts); err != nil {
		return err
	}
	if children > 0 || posts > 0 {
		return ErrCategoryConflict
	}
	_, err := s.db.Exec(`DELETE FROM categories WHERE id = ?`, id)
	return err
}

// LegacyCategoryID resolves a path or a legacy category name.
func (s *Store) LegacyCategoryID(name string) (int64, error) {
	return s.categoryIDByPath(name)
}
