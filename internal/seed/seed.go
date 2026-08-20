// Package seed 把 content/posts/*.md 导入 SQLite（按 slug UPSERT，幂等）。
// 从原 internal/admin 抽出，供 -seed / seed_on_start 使用，不再依赖已退役的后台包。
// 另有启动时增量自动发现（ImportMissing）：只补录 DB 缺失的文件，不触碰已有记录。
package seed

import (
	"strings"
	"time"

	"blog/internal/post"
	"blog/internal/store"
)

// Count 返回待导入文章数（仅日志用）。
func Count(posts *post.Service) (int, error) {
	slugs, err := posts.ListSlugs()
	if err != nil {
		return 0, err
	}
	return len(slugs), nil
}

// Import 扫描 content/posts/*.md，按 slug UPSERT 到 store（幂等）。
func Import(st *store.Store, posts *post.Service) error {
	slugs, err := posts.ListSlugs()
	if err != nil {
		return err
	}
	for _, slug := range slugs {
		p, err := buildPost(st, posts, slug)
		if err != nil {
			return err
		}
		if err := st.Upsert(p); err != nil {
			return err
		}
	}
	return nil
}

// buildPost 把单个 md 文件解析渲染成 store.Post（含 topic 关联）。
func buildPost(st *store.Store, posts *post.Service, slug string) (*store.Post, error) {
	meta, body, err := posts.ParseFile(slug)
	if err != nil {
		return nil, err
	}
	html, err := posts.Render(body)
	if err != nil {
		return nil, err
	}
	title := meta.Title
	if title == "" {
		title = slug
	}
	// 对齐 backup.go importOne 的写法：尊重 frontmatter status 字段，draft 导入为草稿
	status := store.StatusPublished
	if meta.Status == store.StatusDraft {
		status = store.StatusDraft
	}
	p := &store.Post{
		Slug:        slug,
		Title:       title,
		Excerpt:     meta.Excerpt,
		Cover:       meta.Cover,
		Banner:      meta.Banner,
		Category:    meta.Category,
		Categories:  append([]string(nil), meta.Categories...),
		Topic:       meta.Topic,
		Tags:        strings.Join(meta.Tags, ","),
		ContentMD:   body,
		ContentHTML: html,
		Status:      status,
		CreatedAt:   parseSeedDate(meta.Date),
		Poster: store.EncodePoster(store.PosterData{
			Headline: meta.PosterHeadline,
			Topic:    meta.PosterTopic,
			Caption:  meta.PosterCaption,
			Color:    meta.PosterColor,
		}),
		Pinned:     meta.Pinned,
		References: store.EncodeReferences(meta.References),
		Authors:    append([]string(nil), meta.Authors...),
		Author:     meta.Author,
		AILabel:    meta.AILabel,
	}
	// Spec 52：topic 文本非空时按 name 关联独立 topics 表（迁移已建表；同名未建行时
	// topic_id 保持 0，由启动迁移兜底回填）。
	if t := strings.TrimSpace(meta.Topic); t != "" {
		if tp, err := st.GetTopicByName(t); err == nil && tp != nil {
			p.TopicID = tp.ID
		}
	}
	return p, nil
}

// parseSeedDate 把 frontmatter 的 date 字段解析成 time.Time，失败用当前时间。
func parseSeedDate(s string) time.Time {
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Now()
}
