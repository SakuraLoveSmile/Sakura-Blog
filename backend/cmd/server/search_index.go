package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"blog/internal/og"
	"blog/internal/search"
	"blog/internal/store"
)

// setupSearchIndex FTS5 全文索引装配（Spec 25 + 搜索范围扩展）：建表 + 空表回填。
// 失败仅记日志返回 nil，搜索降级回 /search.json 前端匹配，不阻断启动。
// 需在 seed 前装配，保证导入也能同步（写路径回调见 newPostWriteHook 等）。
func setupSearchIndex(st *store.Store) *search.Index {
	searchIdx := search.New(st.DB())
	if err := searchIdx.Ensure(); err != nil {
		log.Printf("search: ensure fts failed (%v), server search disabled", err)
		return nil
	}
	// 空表回填：旧库首次启用 FTS / 结构迁移重建后全量建索引（幂等，非空不重建）。
	if n, err := searchIdx.Count(); err == nil && n == 0 {
		items := searchItemsAll(st)
		if len(items) > 0 {
			if err := searchIdx.Rebuild(items); err != nil {
				log.Printf("search: rebuild failed: %v", err)
			} else {
				log.Printf("search: fts index backfilled, %d docs", len(items))
			}
		}
	}
	return searchIdx
}

// rebuildSearchIndex FTS 全量重建（整站恢复后用，Spec 41）：不同于 setupSearchIndex 的空表回填，
// 无条件重建全部已发布文章/页面与公开瞬间（密码文章正文不入索引，与回填同策略）。
func rebuildSearchIndex(st *store.Store, searchIdx *search.Index) {
	if searchIdx == nil {
		return
	}
	items := searchItemsAll(st)
	if err := searchIdx.Rebuild(items); err != nil {
		log.Printf("search: rebuild after restore failed: %v", err)
		return
	}
	log.Printf("search: fts index rebuilt after restore, %d docs", len(items))
}

// searchItemsAll 全量索引条目：已发布文章 + 已发布页面 + 公开瞬间。
// 密码文章正文置空；瞬间无标题，用正文前 40 字截断充当。
func searchItemsAll(st *store.Store) []search.Item {
	var items []search.Item
	published, err := st.ListPublished()
	if err != nil {
		log.Printf("search: list published failed: %v", err)
	} else {
		for _, p := range published {
			content := stripHTML(p.ContentHTML)
			if p.IsPasswordProtected() {
				content = ""
			}
			items = append(items, search.Item{Type: search.TypePost, Slug: p.Slug, Title: p.Title, Content: content})
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
			items = append(items, search.Item{Type: search.TypePage, Slug: p.Slug, Title: p.Title, Content: stripHTML(p.ContentHTML)})
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
			content := stripHTML(m.ContentHTML)
			items = append(items, search.Item{
				Type:    search.TypeMoment,
				Slug:    store.MomentSlugKey(m.ID),
				Title:   truncateText(content, 40),
				Content: content,
			})
		}
		if len(moments) < 50 {
			break
		}
	}
	return items
}

// syncSearchIndex 通用写路径 → FTS 同步：delete/purge 直接移除；
// 其余 op 由 fetch 提供正文与是否入索引的判定，未命中（草稿/回收站/私密）同样移除。
func syncSearchIndex(ix *search.Index, docType, op, slug string, fetch func() (title, content string, indexed bool)) {
	if ix == nil {
		return
	}
	switch op {
	case "delete", "purge":
		_ = ix.Unindex(docType, slug)
	default: // create / update / restore
		title, content, indexed := fetch()
		if !indexed {
			_ = ix.Unindex(docType, slug)
			return
		}
		if err := ix.IndexDoc(docType, slug, title, content); err != nil {
			log.Printf("search: index %s %s failed: %v", docType, slug, err)
		}
	}
}

// newPostWriteHook store 文章写路径统一回调：搜索索引同步 + OG 缓存失效。
// searchIdx 可空（FTS 装配失败降级），此时只做 OG 失效。
func newPostWriteHook(st *store.Store, searchIdx *search.Index, ogGen *og.Generator) func(op, slug string) {
	return func(op, slug string) {
		// OG 缓存失效（所有写操作均失效）
		ogGen.Invalidate(slug)
		// 搜索索引同步
		syncSearchIndex(searchIdx, search.TypePost, op, slug, func() (string, string, bool) {
			p, err := st.GetBySlug(slug)
			if err != nil || p == nil {
				return "", "", false
			}
			if p.Status != store.StatusPublished || p.DeletedAt != "" || (!p.PublishAt.IsZero() && p.PublishAt.After(time.Now())) {
				return "", "", false
			}
			content := stripHTML(p.ContentHTML)
			if p.IsPasswordProtected() {
				content = ""
			}
			return p.Title, content, true
		})
	}
}

// newPageWriteHook store 页面写路径回调：搜索索引同步（页面无 OG 图，不做缓存失效）。
func newPageWriteHook(st *store.Store, searchIdx *search.Index) func(op, slug string) {
	return func(op, slug string) {
		syncSearchIndex(searchIdx, search.TypePage, op, slug, func() (string, string, bool) {
			p, err := st.GetPageBySlug(slug)
			if err != nil || p == nil {
				return "", "", false
			}
			if p.Status != store.StatusPublished || p.DeletedAt != "" {
				return "", "", false
			}
			return p.Title, stripHTML(p.ContentHTML), true
		})
	}
}

// newMomentWriteHook store 瞬间写路径回调：搜索索引同步（仅公开瞬间入索引）。
func newMomentWriteHook(st *store.Store, searchIdx *search.Index) func(op, slug string) {
	return func(op, slug string) {
		syncSearchIndex(searchIdx, search.TypeMoment, op, slug, func() (string, string, bool) {
			id, err := strconv.ParseInt(strings.TrimPrefix(slug, "moment-"), 10, 64)
			if err != nil {
				return "", "", false
			}
			m, err := st.GetMoment(id)
			if err != nil || m == nil {
				return "", "", false
			}
			if m.Visibility != store.MomentVisibilityPublic {
				return "", "", false
			}
			content := stripHTML(m.ContentHTML)
			return truncateText(content, 40), content, true
		})
	}
}

// clearOGCache 清空 OG 分享图缓存目录（整站恢复后用，Spec 41）：只删 .png，保留 embed 模式落盘的字体文件。
func clearOGCache(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".png") {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
}
