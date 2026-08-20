package store

import (
	"errors"
	"fmt"
	"strings"
)

// normalizeTagsCSV 逗号拆分、逐项 trim、去空去重（保留首次出现顺序），逗号重连。
// 与 api.normalizeTags / parser.parseList 语义一致，保证 tags 存储值规范（防 LIKE 口径不一致）。
func normalizeTagsCSV(parts []string) string {
	seen := map[string]bool{}
	var out []string
	for _, t := range parts {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return strings.Join(out, ",")
}

// RenameCategory 保留旧的按名称 API。新代码应使用 category ID API；这里同步
// 分类实体和关联，并保留旧返回值语义。
func (s *Store) RenameCategory(old, newName string) (int64, error) {
	id, err := s.categoryIDByPath(old)
	if err != nil {
		// 极老数据回退到旧字段改写，避免迁移异常时管理接口不可用。
		res, legacyErr := s.db.Exec(`UPDATE posts SET category = ? WHERE category = ?`, newName, old)
		if legacyErr != nil {
			return 0, fmt.Errorf("rename category: %w", legacyErr)
		}
		n, _ := res.RowsAffected()
		return n, nil
	}
	var affected int64
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM post_categories WHERE category_id = ?`, id).Scan(&affected); err != nil {
		return 0, err
	}
	node, err := s.GetCategoryByID(id)
	if err != nil {
		return 0, err
	}
	if _, err := s.UpdateCategory(id, newName, node.ParentID); err != nil {
		return 0, err
	}
	return affected, nil
}

// DeleteCategory 保留旧的按名称 API。ID API 采用安全删除策略；名称 API
// 继续清空关联以兼容已有 Console 客户端，新的 Console 使用 ID 路由。
func (s *Store) DeleteCategory(name string) (int64, error) {
	id, err := s.categoryIDByPath(name)
	if err != nil {
		res, legacyErr := s.db.Exec(`UPDATE posts SET category = '' WHERE category = ?`, name)
		if legacyErr != nil {
			return 0, fmt.Errorf("delete category: %w", legacyErr)
		}
		n, _ := res.RowsAffected()
		return n, nil
	}
	var affected int64
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM post_categories WHERE category_id = ?`, id).Scan(&affected); err != nil {
		return 0, err
	}
	if _, err := s.db.Exec(`DELETE FROM post_categories WHERE category_id = ?`, id); err != nil {
		return 0, err
	}
	if _, err := s.db.Exec(`UPDATE posts SET category = '' WHERE category = ?`, name); err != nil {
		return 0, err
	}
	if err := s.DeleteCategoryByID(id); err != nil && !errors.Is(err, ErrCategoryNotFound) {
		return 0, err
	}
	return affected, nil
}

// rewriteTags 对所有含 tags 的文章逐行改写标签 CSV。
// 单连接（SetMaxOpenConns(1)）下必须先收完 rows 再执行 UPDATE，否则死锁。
func (s *Store) rewriteTags(transform func(tags []string) []string) (int64, error) {
	rows, err := s.db.Query(`SELECT id, tags FROM posts WHERE tags != ''`)
	if err != nil {
		return 0, fmt.Errorf("query tags: %w", err)
	}
	type rec struct {
		id   int64
		tags string
	}
	var recs []rec
	for rows.Next() {
		var r rec
		if err := rows.Scan(&r.id, &r.tags); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan tags: %w", err)
		}
		recs = append(recs, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	var affected int64
	for _, r := range recs {
		newCSV := normalizeTagsCSV(transform(strings.Split(r.tags, ",")))
		if newCSV == r.tags {
			continue
		}
		if _, err := s.db.Exec(`UPDATE posts SET tags = ? WHERE id = ?`, newCSV, r.id); err != nil {
			return affected, fmt.Errorf("update tags for post %d: %w", r.id, err)
		}
		affected++
	}
	return affected, nil
}

// RenameTag 把标签 old 重命名为 newName（CSV 内逐项替换 + 规范化）。
func (s *Store) RenameTag(old, newName string) (int64, error) {
	old = strings.TrimSpace(old)
	newName = strings.TrimSpace(newName)
	return s.rewriteTags(func(tags []string) []string {
		out := make([]string, 0, len(tags))
		for _, t := range tags {
			if strings.TrimSpace(t) == old {
				out = append(out, newName)
			} else {
				out = append(out, t)
			}
		}
		return out
	})
}

// DeleteTag 从所有文章移除指定标签。
func (s *Store) DeleteTag(name string) (int64, error) {
	name = strings.TrimSpace(name)
	return s.rewriteTags(func(tags []string) []string {
		out := make([]string, 0, len(tags))
		for _, t := range tags {
			if strings.TrimSpace(t) != name {
				out = append(out, t)
			}
		}
		return out
	})
}

// MergeTags 把 sources 中的各标签合并到 target（去重由规范化保证）。
func (s *Store) MergeTags(sources []string, target string) (int64, error) {
	target = strings.TrimSpace(target)
	srcSet := map[string]bool{}
	for _, s := range sources {
		if s = strings.TrimSpace(s); s != "" && s != target {
			srcSet[s] = true
		}
	}
	return s.rewriteTags(func(tags []string) []string {
		out := make([]string, 0, len(tags))
		for _, t := range tags {
			if srcSet[strings.TrimSpace(t)] {
				out = append(out, target)
			} else {
				out = append(out, t)
			}
		}
		return out
	})
}
