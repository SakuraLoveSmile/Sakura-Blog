// 启动时增量自动发现：content/posts/ 里新增的 md 文件自动补录进 DB，
// 已存在的 slug 一律跳过——Console 编辑过的记录绝不被文件覆盖。
// 与 Import（-seed / seed_on_start 的全量 UPSERT）互补：
// 发现只负责「新文件可见」，全量重同步仍走显式 -seed。
package seed

import (
	"errors"
	"io/fs"

	"blog/internal/post"
	"blog/internal/store"
)

// ImportMissing 扫描 content/posts/*.md，仅导入 DB 中不存在的 slug（任意状态都算存在，
// 草稿同样跳过），返回实际导入篇数。已存在记录不做任何更新。
// posts 目录不存在时视为无文件（自动发现是增强路径，不因缺目录阻断启动）。
// Deprecated: 仅保留历史单测；生产调用 ImportStellar(..., true)。
func ImportMissing(st *store.Store, posts *post.Service) (int, error) {
	slugs, err := posts.ListSlugs()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	imported := 0
	for _, slug := range slugs {
		existing, err := st.GetBySlug(slug)
		if err != nil {
			return imported, err
		}
		if existing != nil {
			continue
		}
		p, err := buildPost(st, posts, slug)
		if err != nil {
			return imported, err
		}
		if err := st.Upsert(p); err != nil {
			return imported, err
		}
		imported++
	}
	return imported, nil
}
