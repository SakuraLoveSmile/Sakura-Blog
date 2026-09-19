package search

import (
	"blog/internal/store"
	"blog/internal/textutil"
	"fmt"
)

// ItemsForRestore 不允许读取失败被解释为空索引，供恢复事务的必要重建使用。
func ItemsForRestore(st *store.Store) ([]Item, error) {
	posts, err := st.ListPublished()
	if err != nil {
		return nil, fmt.Errorf("read posts for restored index: %w", err)
	}
	var items []Item
	for _, p := range posts {
		body := textutil.StripHTML(p.ContentHTML)
		if p.IsPasswordProtected() {
			body = ""
		}
		items = append(items, Item{Type: TypePost, Slug: p.Slug, Title: p.Title, Content: body})
	}
	pages, err := st.ListPages()
	if err != nil {
		return nil, fmt.Errorf("read pages for restored index: %w", err)
	}
	for _, p := range pages {
		if p.Status == store.StatusPublished {
			items = append(items, Item{Type: TypePage, Slug: p.Slug, Title: p.Title, Content: textutil.StripHTML(p.ContentHTML)})
		}
	}
	for page := 1; ; page++ {
		moments, err := st.ListPublicMoments(page, 50)
		if err != nil {
			return nil, fmt.Errorf("read moments for restored index: %w", err)
		}
		for _, moment := range moments {
			body := textutil.StripHTML(moment.ContentHTML)
			items = append(items, Item{Type: TypeMoment, Slug: store.MomentSlugKey(moment.ID), Title: textutil.Truncate(body, 40), Content: body})
		}
		if len(moments) < 50 {
			break
		}
	}
	return items, nil
}
