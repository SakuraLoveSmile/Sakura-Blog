package store

import (
	"fmt"
	"strings"
	"time"
)

// WikiDoc wiki 文档页。
type WikiDoc struct {
	ID             int64     `json:"id"`
	ProjectID      string    `json:"project_id"` // 对应 WikiProject.ID
	Slug           string    `json:"slug"`
	Title          string    `json:"title"`
	Order          int       `json:"order"`
	Parent         int64     `json:"parent"` // 父文档 ID，0 = 顶级
	ContentMD      string    `json:"content_md"`
	ContentHTML    string    `json:"content_html"`
	Status         string    `json:"status"` // "published" / "draft"
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	StellarVersion int       `json:"stellar_version"`
	StellarConfig  string    `json:"stellar_config"`
	BlogExtensions string    `json:"blog_extensions"`
}

// TableName Entity 接口：wikidocs 表。
func (d *WikiDoc) TableName() string { return "wikidocs" }

// Columns Entity 接口：全列，首列为 id（列名用 doc_order 而非 order，SQL 保留字）。
func (d *WikiDoc) Columns() []string {
	return []string{"id", "project_id", "slug", "title", "doc_order", "parent", "content_md", "content_html", "status", "created_at", "updated_at", "stellar_version", "stellar_config", "blog_extensions"}
}

// Values Entity 接口：写入值（不含 id，与 Columns()[1:] 对应）。
func (d *WikiDoc) Values() []any {
	return []any{d.ProjectID, d.Slug, d.Title, d.Order, d.Parent, d.ContentMD, d.ContentHTML, d.Status, d.CreatedAt, d.UpdatedAt, d.StellarVersion, d.StellarConfig, d.BlogExtensions}
}

// SetID Entity 接口：回填自增 id。
func (d *WikiDoc) SetID(id int64) { d.ID = id }

// ScanRow Entity 接口：行扫描，顺序对齐 Columns()。
func (d *WikiDoc) ScanRow(row rowScanner) error {
	return row.Scan(&d.ID, &d.ProjectID, &d.Slug, &d.Title, &d.Order, &d.Parent, &d.ContentMD, &d.ContentHTML, &d.Status, &d.CreatedAt, &d.UpdatedAt, &d.StellarVersion, &d.StellarConfig, &d.BlogExtensions)
}

// TreeNode 文档树节点（用于 BuildDocTree 返回）。
// children 不加 omitempty：叶子节点也必须输出 []，保证 API 消费方（Console 树拍平等）可安全遍历。
type TreeNode struct {
	Doc      *WikiDoc   `json:"doc"`
	Children []TreeNode `json:"children"`
}

// EnsureWikidocsTable 幂等建 wikidocs 表（统一走 schema.go 注册表）。
func (s *Store) EnsureWikidocsTable() error {
	return s.ensureTable("wikidocs")
}

// GetWikidocBySlug 按 project_id + slug 查单篇（任意状态）。未找到返回 nil, nil。
func (s *Store) GetWikidocBySlug(projectID, slug string) (*WikiDoc, error) {
	doc, err := repo(s, &WikiDoc{}).GetOne("project_id = ? AND slug = ?", projectID, slug)
	if err != nil {
		return nil, fmt.Errorf("get wikidoc %s/%s: %w", projectID, slug, err)
	}
	return doc, nil
}

// GetWikidocByID 按 id 查单篇。未找到返回 nil, nil。
func (s *Store) GetWikidocByID(id int64) (*WikiDoc, error) {
	doc, err := repo(s, &WikiDoc{}).GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("get wikidoc %d: %w", id, err)
	}
	return doc, nil
}

// ListWikidocs 列出指定 project 的全部文档，按 doc_order, id 升序。
func (s *Store) ListWikidocs(projectID string) ([]*WikiDoc, error) {
	return repo(s, &WikiDoc{}).List(Where("project_id = ?", projectID), OrderBy("doc_order, id"))
}

// CountPublishedWikidocsByProjects 各项目的已发布文档数（Spec 62：文库页 HasDocs
// 从每项目一次 ListWikidocs 收敛为单条 GROUP BY 查询）。未收录的 id 计数为 0。
func (s *Store) CountPublishedWikidocsByProjects(ids []string) (map[string]int, error) {
	out := make(map[string]int, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+1)
	for _, id := range ids {
		args = append(args, id)
		out[id] = 0
	}
	args = append(args, StatusPublished)
	rows, err := s.db.Query(
		`SELECT project_id, COUNT(*) FROM wikidocs WHERE project_id IN (`+placeholders+`) AND status = ? GROUP BY project_id`,
		args...)
	if err != nil {
		return nil, fmt.Errorf("count wikidocs by projects: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, fmt.Errorf("scan wikidoc count: %w", err)
		}
		out[id] = n
	}
	return out, rows.Err()
}

// CreateWikidoc 新建文档，LastInsertId 回填 ID，自动设置 created_at/updated_at。
// (project_id, slug) 重复时返回 UNIQUE 约束错误。
func (s *Store) CreateWikidoc(doc *WikiDoc) error {
	if doc == nil {
		return fmt.Errorf("nil content")
	}
	if err := validateStellarWrite(doc.StellarVersion, doc.StellarConfig, doc.BlogExtensions, "wiki/"+doc.ProjectID+"/"+doc.Slug, "wiki", false); err != nil {
		return err
	}
	if doc.Status == "" {
		doc.Status = StatusPublished
	}
	now := time.Now().UTC()
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = now
	}
	doc.UpdatedAt = now
	if _, err := repo(s, &WikiDoc{}).Create(doc); err != nil {
		return fmt.Errorf("create wikidoc %s/%s: %w", doc.ProjectID, doc.Slug, err)
	}
	return nil
}

// UpdateWikidoc 按 id 更新文档字段，自动刷新 updated_at。文档不存在返回错误。
func (s *Store) UpdateWikidoc(doc *WikiDoc) error {
	if doc == nil {
		return fmt.Errorf("nil content")
	}
	if err := validateStellarWrite(doc.StellarVersion, doc.StellarConfig, doc.BlogExtensions, "wiki/"+doc.ProjectID+"/"+doc.Slug, "wiki", false); err != nil {
		return err
	}
	doc.UpdatedAt = time.Now().UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := updateStellarEntityTx(tx, doc, doc.ID, doc.StellarVersion); err != nil {
		return fmt.Errorf("update wikidoc %d: %w", doc.ID, err)
	}
	return tx.Commit()
}

// DeleteWikidoc 硬删除文档（wiki 文档无需软删除）。删除前把子文档 parent 批量置 0 升为顶级，
// 避免子文档成为指向已删除 ID 的孤儿节点从树中消失。不存在不报错。
func (s *Store) DeleteWikidoc(id int64) error {
	// 删除前：子文档升为顶级
	if _, err := s.db.Exec(`UPDATE wikidocs SET parent = 0 WHERE parent = ?`, id); err != nil {
		return fmt.Errorf("reset children parent: %w", err)
	}
	if err := repo(s, &WikiDoc{}).Delete(id); err != nil {
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
