package store

import (
	"database/sql"
	"fmt"
	"time"
)

// WikiDoc wiki 文档页。
type WikiDoc struct {
	ID          int64     `json:"id"`
	ProjectID   string    `json:"project_id"` // 对应 WikiProject.ID
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Order       int       `json:"order"`
	Parent      int64     `json:"parent"` // 父文档 ID，0 = 顶级
	ContentMD   string    `json:"content_md"`
	ContentHTML string    `json:"content_html"`
	Status      string    `json:"status"` // "published" / "draft"
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TreeNode 文档树节点（用于 BuildDocTree 返回）。
// children 不加 omitempty：叶子节点也必须输出 []，保证 API 消费方（Console 树拍平等）可安全遍历。
type TreeNode struct {
	Doc      *WikiDoc   `json:"doc"`
	Children []TreeNode `json:"children"`
}

// wikidocCols 统一 SELECT 列清单。列名用 doc_order 而非 order（SQL 保留字）。
const wikidocCols = "id, project_id, slug, title, doc_order, parent, content_md, content_html, status, created_at, updated_at"

// EnsureWikidocsTable 幂等建 wikidocs 表（不走 createTables，由调用方启动时按需调用）。
func (s *Store) EnsureWikidocsTable() error {
	const ddl = `CREATE TABLE IF NOT EXISTS wikidocs (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id   TEXT NOT NULL,
		slug         TEXT NOT NULL,
		title        TEXT NOT NULL DEFAULT '',
		doc_order    INTEGER NOT NULL DEFAULT 0,
		parent       INTEGER NOT NULL DEFAULT 0,
		content_md   TEXT NOT NULL DEFAULT '',
		content_html TEXT NOT NULL DEFAULT '',
		status       TEXT NOT NULL DEFAULT 'published',
		created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(project_id, slug)
	);
	CREATE INDEX IF NOT EXISTS idx_wikidocs_project ON wikidocs(project_id, parent);`
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("create wikidocs table: %w", err)
	}
	return nil
}

// GetWikidocBySlug 按 project_id + slug 查单篇（任意状态）。未找到返回 nil, nil。
func (s *Store) GetWikidocBySlug(projectID, slug string) (*WikiDoc, error) {
	row := s.db.QueryRow(`SELECT `+wikidocCols+` FROM wikidocs WHERE project_id = ? AND slug = ?`, projectID, slug)
	doc, err := scanWikidoc(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get wikidoc %s/%s: %w", projectID, slug, err)
	}
	return doc, nil
}

// GetWikidocByID 按 id 查单篇。未找到返回 nil, nil。
func (s *Store) GetWikidocByID(id int64) (*WikiDoc, error) {
	row := s.db.QueryRow(`SELECT `+wikidocCols+` FROM wikidocs WHERE id = ?`, id)
	doc, err := scanWikidoc(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get wikidoc %d: %w", id, err)
	}
	return doc, nil
}

// ListWikidocs 列出指定 project 的全部文档，按 doc_order, id 升序。
func (s *Store) ListWikidocs(projectID string) ([]*WikiDoc, error) {
	rows, err := s.db.Query(`SELECT `+wikidocCols+` FROM wikidocs WHERE project_id = ? ORDER BY doc_order, id`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list wikidocs %s: %w", projectID, err)
	}
	defer rows.Close()
	var out []*WikiDoc
	for rows.Next() {
		doc, err := scanWikidoc(rows)
		if err != nil {
			return nil, fmt.Errorf("scan wikidoc: %w", err)
		}
		out = append(out, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate wikidocs: %w", err)
	}
	return out, nil
}

// CreateWikidoc 新建文档，LastInsertId 回填 ID，自动设置 created_at/updated_at。
// (project_id, slug) 重复时返回 UNIQUE 约束错误。
func (s *Store) CreateWikidoc(doc *WikiDoc) error {
	if doc.Status == "" {
		doc.Status = StatusPublished
	}
	now := time.Now().UTC()
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = now
	}
	doc.UpdatedAt = now
	res, err := s.db.Exec(`INSERT INTO wikidocs (project_id, slug, title, doc_order, parent, content_md, content_html, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		doc.ProjectID, doc.Slug, doc.Title, doc.Order, doc.Parent, doc.ContentMD, doc.ContentHTML, doc.Status, doc.CreatedAt, doc.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create wikidoc %s/%s: %w", doc.ProjectID, doc.Slug, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	doc.ID = id
	return nil
}

// UpdateWikidoc 按 id 更新文档字段，自动刷新 updated_at。文档不存在返回错误。
func (s *Store) UpdateWikidoc(doc *WikiDoc) error {
	doc.UpdatedAt = time.Now().UTC()
	res, err := s.db.Exec(`UPDATE wikidocs SET slug=?, title=?, doc_order=?, parent=?, content_md=?, content_html=?, status=?, updated_at=?
		WHERE id=?`,
		doc.Slug, doc.Title, doc.Order, doc.Parent, doc.ContentMD, doc.ContentHTML, doc.Status, doc.UpdatedAt, doc.ID)
	if err != nil {
		return fmt.Errorf("update wikidoc %d: %w", doc.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("update wikidoc %d: no rows affected", doc.ID)
	}
	return nil
}

// DeleteWikidoc 硬删除文档（wiki 文档无需软删除）。删除前把子文档 parent 批量置 0 升为顶级，
// 避免子文档成为指向已删除 ID 的孤儿节点从树中消失。不存在不报错。
func (s *Store) DeleteWikidoc(id int64) error {
	// 删除前：子文档升为顶级
	if _, err := s.db.Exec(`UPDATE wikidocs SET parent = 0 WHERE parent = ?`, id); err != nil {
		return fmt.Errorf("reset children parent: %w", err)
	}
	if _, err := s.db.Exec(`DELETE FROM wikidocs WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete wikidoc %d: %w", id, err)
	}
	return nil
}

// BuildDocTree 按 projectID 组装文档树（≤3 层递归，内存完成）。
// 第四层及更深、parent 指向不存在文档的孤儿节点会被截断丢弃。
func (s *Store) BuildDocTree(projectID string) ([]TreeNode, error) {
	docs, err := s.ListWikidocs(projectID)
	if err != nil {
		return nil, err
	}
	return BuildDocTreeFromDocs(docs), nil
}

// BuildDocTreeFromDocs 从已加载的文档列表构建文档树（纯内存，≤3 层递归）。
// 用于避免 handleWikiDoc 中重复 ListWikidocs 查询；孤儿与超深节点截断规则同 BuildDocTree。
func BuildDocTreeFromDocs(docs []*WikiDoc) []TreeNode {
	// 按 parent 分组
	children := make(map[int64][]*WikiDoc)
	var roots []*WikiDoc
	for _, d := range docs {
		if d.Parent == 0 {
			roots = append(roots, d)
		} else {
			children[d.Parent] = append(children[d.Parent], d)
		}
	}

	// 递归构建树（最多 3 层）
	var build func(parentID int64, depth int) []TreeNode
	build = func(parentID int64, depth int) []TreeNode {
		if depth >= 3 {
			return nil
		}
		items := children[parentID]
		if parentID == 0 {
			items = roots
		}
		nodes := make([]TreeNode, 0, len(items))
		for _, d := range items {
			node := TreeNode{
				Doc:      d,
				Children: build(d.ID, depth+1),
			}
			nodes = append(nodes, node)
		}
		return nodes
	}

	return build(0, 0)
}

// scanWikidoc 单行扫描；行顺序对齐 wikidocCols。
func scanWikidoc(row rowScanner) (*WikiDoc, error) {
	doc := &WikiDoc{}
	err := row.Scan(&doc.ID, &doc.ProjectID, &doc.Slug, &doc.Title, &doc.Order, &doc.Parent, &doc.ContentMD, &doc.ContentHTML, &doc.Status, &doc.CreatedAt, &doc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return doc, nil
}
