package stellarmigration

import (
	"encoding/json"
	"sort"
	"strings"

	"blog/internal/config"
)

// 公开目录与 users/post_authors 编辑权限完全独立；只接受已有且有有效 name 的 ID。
func publicAuthorCatalog(s Snapshot) map[string]bool {
	settings := map[string]string{}
	for _, row := range s.Tables["settings"].Rows {
		settings[stringValue(row["section"])] = stringValue(row["value"])
	}
	raw := settings["authors"]
	if _, versioned := settings[config.SectionStellarV2]; versioned {
		value, _ := decode(settings[config.SectionBlogExtensions])
		catalog, _ := get(object(value), "data.authors")
		raw = encode(catalog)
	}
	value, _ := decode(raw)
	catalog := map[string]bool{}
	for id, author := range object(value) {
		name, valid := object(author)["name"].(string)
		if strings.TrimSpace(id) != "" && valid && strings.TrimSpace(name) != "" {
			catalog[id] = true
		}
	}
	return catalog
}

func publicAuthorReference(s Snapshot, ref any, catalog map[string]bool) string {
	// TEXT 旧关联为公开 ID/username，只做现有目录的精确 ID 查找。
	if id, text := ref.(string); text {
		if catalog[id] {
			return id
		}
		return ""
	}
	// INTEGER 新关联是 users.id，必须有唯一用户且 username 是现有公开目录 ID。
	if _, numeric := ref.(json.Number); !numeric {
		return ""
	}
	matched, count := "", 0
	for _, user := range s.Tables["users"].Rows {
		if stringValue(user["id"]) == stringValue(ref) {
			count++
			matched = stringValue(user["username"])
		}
	}
	if count == 1 && catalog[matched] {
		return matched
	}
	return ""
}

func (b *recordBuilder) migratePostPublicAuthors(s Snapshot) {
	type authorLink struct {
		row Row
		ref any
	}
	links := []authorLink{}
	for _, source := range []struct{ table, ref string }{{"post_authors", "author_id"}, {"post_authors_legacy", "author_ref"}} {
		for _, row := range s.Tables[source.table].Rows {
			if stringValue(row["post_id"]) == b.candidate.ID {
				links = append(links, authorLink{row, row[source.ref]})
			}
		}
	}
	if len(links) == 0 {
		return
	}
	b.mapped("post_authors/post_authors_legacy", "source.snapshot.tables", "完整关联、排序、未知字段与编辑权限留私密源快照；不变更 users 或关联权限")
	sort.SliceStable(links, func(i, j int) bool {
		a, _ := strconvNumber(links[i].row["sort_order"])
		z, _ := strconvNumber(links[j].row["sort_order"])
		if a == z {
			return encode(links[i].row) < encode(links[j].row)
		}
		return a < z
	})
	catalog := publicAuthorCatalog(s)
	ids, seen := []string{}, map[string]bool{}
	unmapped := false
	for _, link := range links {
		id := publicAuthorReference(s, link.ref, catalog)
		if id == "" {
			unmapped = true
		} else if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	if unmapped {
		b.conflict("public_author_unmapped", "post_authors/post_authors_legacy", "关联权限不能确定映射已有公开作者目录 ID；不生成部分署名、不猜显示名、不公开用户 ID，须逐记录确认")
		return
	}
	set(b.blog, "content.authors", ids)
	b.mapped("post_authors.author_id/users.username/settings.authors", "blog.content.authors", "按原关联排序确定已有公开目录 ID 字符串数组，去重保持首位；不把关联对象或权限写入公开扩展")
	b.conflict("multiple_authors", "post_authors", "公开署名 ID 数组已确定且关联权限独立保留；消费者仍须显式实测声明")
}
