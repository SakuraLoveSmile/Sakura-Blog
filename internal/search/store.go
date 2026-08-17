package search

import (
	"log"

	"blog/internal/store"
	"blog/internal/textutil"
)

// ItemsFromStore returns the documents that belong in the server-side FTS
// index: published posts, published pages, and public moments. Password
// protected post bodies are intentionally omitted from the index.
func ItemsFromStore(st *store.Store) []Item {
	if st == nil {
		return nil
	}

	var items []Item
	posts, err := st.ListPublished()
	if err != nil {
		log.Printf("search: list published failed: %v", err)
	} else {
		for _, p := range posts {
			content := textutil.StripHTML(p.ContentHTML)
			if p.IsPasswordProtected() {
				content = ""
			}
			items = append(items, Item{Type: TypePost, Slug: p.Slug, Title: p.Title, Content: content})
		}
	}

	pages, err := st.ListPages()
	if err != nil {
		log.Printf("search: list pages failed: %v", err)
	} else {
		for _, p := range pages {
			if p.Status != store.StatusPublished {
				continue
			}
			items = append(items, Item{Type: TypePage, Slug: p.Slug, Title: p.Title, Content: textutil.StripHTML(p.ContentHTML)})
		}
	}

	for page := 1; ; page++ {
		moments, err := st.ListPublicMoments(page, 50)
		if err != nil {
			log.Printf("search: list moments failed: %v", err)
			break
		}
		if len(moments) == 0 {
			break
		}
		for _, m := range moments {
			content := textutil.StripHTML(m.ContentHTML)
			items = append(items, Item{
				Type: TypeMoment, Slug: store.MomentSlugKey(m.ID),
				Title: textutil.Truncate(content, 40), Content: content,
			})
		}
		if len(moments) < 50 {
			break
		}
	}
	return items
}
