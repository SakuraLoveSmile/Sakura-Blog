package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"blog/internal/og"
	"blog/internal/search"
	"blog/internal/store"
)

// setupSearchIndex FTS5 全文索引装配（Spec 25）：建表 + 空表回填。
// 失败仅记日志返回 nil，搜索降级回 /search.json 前端匹配，不阻断启动。
// 需在 seed 前装配，保证导入也能同步（写路径回调见 newPostWriteHook）。
func setupSearchIndex(st *store.Store) *search.Index {
	searchIdx := search.New(st.DB())
	if err := searchIdx.Ensure(); err != nil {
		log.Printf("search: ensure fts failed (%v), server search disabled", err)
		return nil
	}
	// 空表回填：旧库首次启用 FTS 时全量建索引（幂等，非空不重建）。
	if n, err := searchIdx.Count(); err == nil && n == 0 {
		if published, err := st.ListPublished(); err == nil && len(published) > 0 {
			items := make([]search.Item, 0, len(published))
			for _, p := range published {
				content := stripHTML(p.ContentHTML)
				if p.IsPasswordProtected() {
					content = ""
				}
				items = append(items, search.Item{Slug: p.Slug, Title: p.Title, Content: content})
			}
			if err := searchIdx.Rebuild(items); err != nil {
				log.Printf("search: rebuild failed: %v", err)
			} else {
				log.Printf("search: fts index backfilled, %d posts", len(items))
			}
		}
	}
	return searchIdx
}

// rebuildSearchIndex FTS 全量重建（整站恢复后用，Spec 41）：不同于 setupSearchIndex 的空表回填，
// 无条件重建全部已发布文章的索引（密码文章正文不入索引，与回填同策略）。
func rebuildSearchIndex(st *store.Store, searchIdx *search.Index) {
	if searchIdx == nil {
		return
	}
	published, err := st.ListPublished()
	if err != nil {
		log.Printf("search: rebuild list failed: %v", err)
		return
	}
	items := make([]search.Item, 0, len(published))
	for _, p := range published {
		content := stripHTML(p.ContentHTML)
		if p.IsPasswordProtected() {
			content = ""
		}
		items = append(items, search.Item{Slug: p.Slug, Title: p.Title, Content: content})
	}
	if err := searchIdx.Rebuild(items); err != nil {
		log.Printf("search: rebuild after restore failed: %v", err)
		return
	}
	log.Printf("search: fts index rebuilt after restore, %d posts", len(items))
}

// newPostWriteHook store 写路径统一回调：搜索索引同步 + OG 缓存失效。
// searchIdx 可空（FTS 装配失败降级），此时只做 OG 失效。
func newPostWriteHook(st *store.Store, searchIdx *search.Index, ogGen *og.Generator) func(op, slug string) {
	return func(op, slug string) {
		// OG 缓存失效（所有写操作均失效）
		ogGen.Invalidate(slug)
		// 搜索索引同步
		if searchIdx == nil {
			return
		}
		switch op {
		case "delete", "purge":
			_ = searchIdx.Unindex(slug)
		default: // create / update / restore
			p, err := st.GetBySlug(slug)
			if err != nil || p == nil {
				return
			}
			if p.Status != store.StatusPublished || p.DeletedAt != "" || (!p.PublishAt.IsZero() && p.PublishAt.After(time.Now())) {
				_ = searchIdx.Unindex(slug)
				return
			}
			content := stripHTML(p.ContentHTML)
			if p.IsPasswordProtected() {
				content = ""
			}
			if err := searchIdx.IndexPost(p.Slug, p.Title, content); err != nil {
				log.Printf("search: index %s failed: %v", slug, err)
			}
		}
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
