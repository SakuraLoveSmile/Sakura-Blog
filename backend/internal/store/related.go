// related.go 文章页 read_next / related_posts 读查询（Spec 32）。
// 与 post.go 主体隔离：仅新增读方法，不改动既有查询。
// 安全边界：邻篇与相关文章一律排除 password 保护与未发布/回收站文章。
package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// openWhere 可公开露出的文章：已发布且无阅读口令（read_next/related 防加密文章泄漏）。
// 复用 publishedWhere（占位符绑定 StatusPublished），追加 password 排除。
const openWhere = publishedWhere + " AND password = ''"

// GetAdjacentPost 取邻篇（read_next 用）。newer=true 取时间更晚的最近一篇（较新文章），
// newer=false 取时间更早的最近一篇（较早文章）。
// 对齐 Spec 32 技术决策：category 非空时优先同分类内取邻篇，无同分类邻篇时回退全局时间序。
func (s *Store) GetAdjacentPost(slug, category string, newer bool) (*Post, error) {
	if category != "" {
		p, err := s.getAdjacent(slug, category, newer)
		if err != nil {
			return nil, err
		}
		if p != nil {
			return p, nil
		}
	}
	return s.getAdjacent(slug, "", newer)
}

// getAdjacent 单次邻篇查询。category 为空表示全局时间序。
// 时间比较用 substr 前 19 位字典序（同表同时区等价时间序，理由见 publishedOrder 注释）。
func (s *Store) getAdjacent(slug, category string, newer bool) (*Post, error) {
	cmp, order := "<", "DESC"
	if newer {
		cmp, order = ">", "ASC"
	}
	q := "SELECT " + postCols + " FROM posts WHERE " + openWhere +
		" AND substr(created_at, 1, 19) " + cmp + " (SELECT substr(created_at, 1, 19) FROM posts WHERE slug = ?)"
	args := []interface{}{StatusPublished, slug}
	if category != "" {
		q += " AND category = ?"
		args = append(args, category)
	}
	q += " ORDER BY substr(created_at, 1, 19) " + order + ", id " + order + " LIMIT 1"
	row := s.db.QueryRow(q, args...)
	p, err := scanOne(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get adjacent post for %s: %w", slug, err)
	}
	return p, nil
}

// ListRelatedPosts 相关文章推荐（related_posts 用）：SQL 单查，
// 共同标签数 desc + 发布时间 desc 排序，排除自身与不可公开文章，最多 limit 篇。
// tags 为当前文章的逗号分隔标签串；无有效标签或 limit<=0 返回空。
func (s *Store) ListRelatedPosts(slug, tags string, limit int) ([]*Post, error) {
	if limit <= 0 {
		return nil, nil
	}
	var validTags []string
	for _, t := range strings.Split(tags, ",") {
		if t = strings.TrimSpace(t); t != "" {
			validTags = append(validTags, t)
		}
	}
	if len(validTags) == 0 {
		return nil, nil
	}

	// 共同标签计数：每个标签一个 CASE WHEN LIKE 命中项，求和为 common。
	// tags 存储值经 normalizeTags 规范化（逗号包裹 LIKE 防子串误匹配，见 Step 2 坑点）。
	// 注意占位符按 SQL 文本出现顺序绑定：CASE（SELECT 列表）先于 WHERE 的 status。
	var caseExprs []string
	var args []interface{}
	for _, tag := range validTags {
		caseExprs = append(caseExprs, "(CASE WHEN (',' || tags || ',') LIKE ? THEN 1 ELSE 0 END)")
		args = append(args, "%,"+tag+",%")
	}
	commonExpr := strings.Join(caseExprs, " + ")

	q := "SELECT " + postCols + " FROM (" +
		"SELECT " + postCols + ", (" + commonExpr + ") AS common FROM posts WHERE " + openWhere + " AND slug != ?" +
		") WHERE common > 0 ORDER BY common DESC, substr(created_at, 1, 19) DESC LIMIT ?"
	args = append(args, StatusPublished, slug, limit)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list related posts for %s: %w", slug, err)
	}
	defer rows.Close()
	return scanPosts(rows)
}
