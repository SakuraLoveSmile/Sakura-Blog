package routes

import (
	"context"
	"fmt"

	"blog/internal/content"
	"blog/internal/post"
	"blog/internal/search"
	"blog/internal/store"
	"blog/internal/textutil"
)

// RenderToolSearchItems与Offline SSR使用同一只读完整公开快照和当前严格Markdown模型。
// content_html是旧派生缓存，不能作为迁移后索引正文。保护/draft/隐私过滤由快照统一执行。
func RenderToolSearchItems(ctx context.Context, d Dependencies) ([]search.Item, error) {
	if d.Site == nil || d.Store == nil || d.Posts == nil {
		return nil, fmt.Errorf("stellar search snapshot dependencies unavailable")
	}
	ws := newWebServer(webDeps{Site: d.Site, Store: d.Store, Posts: d.Posts, Engine: d.Engine, Icons: d.Icons, Root: d.Root, UploadDir: d.UploadDir, NotesPath: d.NotesPath})
	return ws.stellarCanonicalSearchItems(ctx)
}

func (ws *webServer) stellarCanonicalSearchItems(ctx context.Context) ([]search.Item, error) {
	snap, err := ws.stellarPublicContextAt(ctx, ws.site.Load())
	if err != nil {
		return nil, err
	}
	items := []search.Item{}
	for _, r := range snap.records {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		body, err := ws.stellarBody(snap, r)
		if err != nil {
			return nil, &content.StellarBodyRenderError{Source: r.source, Err: err}
		}
		if stellarGet(r.model.Item, "visibility.searchable") == false {
			continue
		}
		kind, slug := "", ""
		if r.post != nil {
			kind, slug = search.TypePost, r.post.Slug
		}
		if r.pageRow != nil {
			kind, slug = search.TypePage, r.pageRow.Slug
		}
		// 现有Go FTS身份仅post/page/moment；Wiki/Note由严格公开搜索模型直接检索。
		if kind == "" {
			continue
		}
		items = append(items, search.Item{Type: kind, Slug: slug, Title: stellarString(r.model.Item["title"]), Content: textutil.StripHTML(body)})
	}
	for _, m := range snap.inventory.moments {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		body, e := ws.posts.RenderStellarV2WithContext(m.ContentMD, post.StellarRenderContext{Theme: snap.input.Theme, Data: snap.input.RuntimeData, Language: stellarLanguage(snap.input, nil), BlogExtensions: true, Offline: true})
		if e != nil {
			return nil, &content.StellarBodyRenderError{Source: "moments/" + store.MomentSlugKey(m.ID), Err: e}
		}
		content := textutil.StripHTML(body)
		items = append(items, search.Item{Type: search.TypeMoment, Slug: store.MomentSlugKey(m.ID), Title: textutil.Truncate(content, 40), Content: content})
	}
	return items, nil
}

func (s *Server) RenderSearchItems(ctx context.Context) ([]search.Item, error) {
	return RenderToolSearchItems(ctx, s.deps)
}
