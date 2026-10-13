package routes

import (
	"context"
	"encoding/json"
	"fmt"

	"blog/internal/config"
	"blog/internal/stellarmodels"
	"blog/internal/store"
	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"
)

// 设置预览只覆盖宿主site与严格版本配置；公开内容来自单个只读快照。
func (s *webServer) renderSitePreview(sections map[string]json.RawMessage, page string) (string, error) {
	if s.site == nil || s.site.Load() == nil || s.site.Load().StellarV2 == nil {
		return "", store.ErrStellarMigrationRequired
	}
	live := s.site.Load()
	draft := *live
	rawTheme, err := json.Marshal(map[string]any{"version": 2, "theme": live.StellarV2.RawTheme})
	if err != nil {
		return "", err
	}
	rawBlog, err := json.Marshal(live.StellarV2.Blog)
	if err != nil {
		return "", err
	}
	snapshot := map[string]string{config.SectionStellarV2: string(rawTheme), config.SectionBlogExtensions: string(rawBlog)}
	for name, value := range sections {
		switch name {
		case "site":
			current := map[string]any{"title": draft.Title, "subtitle": draft.Subtitle, "description": draft.Description, "url": draft.URL, "avatar": draft.Avatar, "avatar_bg": draft.AvatarBG, "logo_title": draft.LogoTitle, "logo_sub": draft.LogoSub, "favicon_url": draft.FaviconURL, "apple_touch_icon": draft.AppleIcon, "sw_cache_version": draft.SWCacheVersion}
			var candidate map[string]any
			if err := json.Unmarshal(value, &candidate); err != nil {
				return "", err
			}
			for key, value := range candidate {
				if _, allowed := current[key]; !allowed {
					return "", fmt.Errorf("unsupported site preview field %q", key)
				}
				current[key] = value
			}
			encoded, err := json.Marshal(current)
			if err != nil {
				return "", err
			}
			config.ApplyDraftSections(&draft, map[string]json.RawMessage{"site": encoded})
		case config.SectionStellarV2, config.SectionBlogExtensions:
			snapshot[name] = string(value)
		default:
			return "", fmt.Errorf("legacy preview section %q requires migration", name)
		}
	}
	draft.StellarV2, err = config.LoadStellarV2Snapshot(snapshot)
	if err != nil {
		return "", err
	}
	previewSite, err := stellarPreviewSite(&draft, draft.URL)
	if err != nil {
		return "", err
	}
	snap, err := s.stellarSnapshotAt(context.Background(), previewSite, nil, true)
	if err != nil {
		return "", err
	}
	var data *tmpl.ViewData
	switch page {
	case "categories", "tags", "archives":
		data, err = s.stellarTaxonomyView(snap, page, "", 1)
	case "wiki":
		for _, r := range snap.records {
			if r.profile == "wiki" {
				data = s.stellarView(snap, r)
				break
			}
		}
		if data == nil {
			data, err = s.stellarCollectionIndexView(snap, "wiki", "")
		}
	case "wiki-index":
		data, err = s.stellarCollectionIndexView(snap, "wiki", "")
	case "topic":
		data, err = s.stellarCollectionIndexView(snap, "topic", "")
	case "notebooks":
		data, err = s.stellarCollectionIndexView(snap, "notebook", "")
	case "post":
		for _, r := range snap.records {
			if r.post != nil {
				data = s.stellarView(snap, r)
				break
			}
		}
	case "settings":
		data = s.stellarAggregate(snap, "settings", "设置", stellarProfilePath(snap.input.Theme, "settings"), "settings")
	case "index", "":
	default:
		return "", fmt.Errorf("unknown preview page %q", page)
	}
	if err != nil {
		return "", err
	}
	if data == nil {
		records := []*stellarRecord{}
		for _, r := range snap.records {
			if r.post != nil {
				records = append(records, r)
			}
		}
		data, err = s.stellarListingView(snap, "home", previewSite.Title, "/", records, 1, nil, nil)
		if err != nil {
			return "", err
		}
	}
	data.Stellar.Input.RuntimeData = stellarCopyObject(data.Stellar.Input.RuntimeData)
	data.Stellar.Input.RuntimeData["preview"] = true
	data.Stellar.Input.RuntimeData["nativeComments"] = stellarmodels.Object{"enabled": false}
	data.Comments = nil
	data.CommentCount = 0
	data.Page.Description = previewSite.Description
	data.Stellar.RequestURL = tmpl.JoinSiteURL(previewSite.URL, data.Page.Path)
	data.TOC = stellarweb.ExtractTOCWith(string(data.Content), s.tocOptionsFrom(previewSite))
	return s.engine.Render("layout/base", data)
}
