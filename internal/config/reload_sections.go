package config

import (
	tmpl "blog/internal/settings"
	"blog/internal/store"
	"encoding/json"
	"fmt"
	"sync/atomic"
)

// LinksToGroups 把扁平的 store.Link 列表按 GroupTitle 分组为 tmpl.LinkGroup，保留首次出现的组顺序。
func LinksToGroups(links []*store.Link) []tmpl.LinkGroup {
	idx := map[string]int{}
	var groups []tmpl.LinkGroup
	for _, l := range links {
		i, ok := idx[l.GroupTitle]
		if !ok {
			i = len(groups)
			idx[l.GroupTitle] = i
			groups = append(groups, tmpl.LinkGroup{Title: l.GroupTitle})
		}
		groups[i].Items = append(groups[i].Items, tmpl.LinkItem{
			Name: l.Name, URL: l.URL, Avatar: l.Avatar, Snapshot: l.Snapshot,
			Desc: l.Description, RSS: l.RSS, Label: l.Label, Color: l.Color,
		})
	}
	return groups
}

// KnownSections 返回可编辑的分段名（顺序固定，供 GET 返回与校验）。
func KnownSections() []string {
	return []string{
		SectionSite, SectionMenubar, SectionRightbar, SectionFooter,
		SectionComments, SectionArticle, SectionWiki, SectionFriends,
		SectionSearch, SectionSiteTree,
		SectionSEO, SectionInject, SectionDashboard,
		SectionStyle, SectionWidgets, SectionAI,
		SectionCover, SectionNavTabs, SectionAuthors, SectionPlugins,
		SectionTagPlugins, SectionDefault,
		SectionPages, SectionDataCache,
	}
}

// LoadSectionsJSON 读取全部可编辑段的原始 JSON（缺失段用 null 兑底），供 GET /settings。
func LoadSectionsJSON(st *store.Store) (map[string]json.RawMessage, error) {
	settings, err := st.AllSettings()
	if err != nil {
		return nil, err
	}
	out := map[string]json.RawMessage{}
	for _, sec := range KnownSections() {
		if v, ok := settings[sec]; ok && v != "" {
			out[sec] = json.RawMessage(v)
		} else {
			out[sec] = json.RawMessage("null")
		}
	}
	return out, nil
}

// SaveSection 校验并保存某段配置（用对应类型解析拒绝结构错误，再规范化回存），随后 RebuildSite 热更新。
func SaveSection(st *store.Store, site *atomic.Pointer[tmpl.Site], section string, raw []byte) error {
	var dst any
	switch section {
	case SectionSite:
		dst = &siteBasic{}
	case SectionMenubar:
		dst = &tmpl.Menubar{}
	case SectionRightbar:
		dst = &tmpl.RightbarConfig{}
	case SectionFooter:
		dst = &tmpl.Footer{}
	case SectionComments:
		dst = &tmpl.CommentsConfig{}
	case SectionArticle:
		dst = &tmpl.ArticleConfig{}
	case SectionSearch:
		dst = &tmpl.SearchConfig{}
	case SectionSiteTree:
		dst = &tmpl.SiteTreeConfig{}
	case SectionWiki:
		dst = &[]tmpl.WikiProject{}
	case SectionFriends:
		dst = &tmpl.FriendsConfig{}
	case SectionSEO:
		dst = &tmpl.SEOConfig{}
	case SectionInject:
		dst = &tmpl.InjectConfig{}
	case SectionDashboard:
		dst = &dashboardConfig{}
	case SectionStyle:
		dst = &tmpl.StyleConfig{}
	case SectionWidgets:
		dst = &tmpl.WidgetsConfig{}
	case SectionAI:
		dst = &AIConfig{}
	case SectionCover:
		dst = &tmpl.CoverConfig{}
	case SectionNavTabs:
		dst = &tmpl.NavTabsConfig{}
	case SectionAuthors:
		dst = &map[string]tmpl.AuthorDef{}
	case SectionPlugins:
		dst = &tmpl.PluginsConfig{}
	case SectionTagPlugins:
		dst = &tmpl.TagPluginsConfig{}
	case SectionDefault:
		dst = &tmpl.DefaultAssets{}
	case SectionPages:
		dst = &tmpl.PagesConfig{}
	case SectionDataCache:
		dst = &tmpl.DataCacheConfig{}
	default:
		return fmt.Errorf("unknown section: %s", section)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("invalid %s payload: %w", section, err)
	}
	if cc, ok := dst.(*tmpl.CommentsConfig); ok {
		applyCommentDefaults(cc) // 评论段补运行时默认（镜像/徽章/标题不为空）
	}
	// menubar 服务端兖底校验：items 内 title/url 不得为空
	if mb, ok := dst.(*tmpl.Menubar); ok {
		for i, item := range mb.Items {
			if item.Title == "" || item.URL == "" {
				return fmt.Errorf("menubar item[%d]: title 和 url 不得为空", i)
			}
		}
	}
	norm, err := json.Marshal(dst)
	if err != nil {
		return err
	}
	if err := st.PutSetting(section, string(norm)); err != nil {
		return err
	}
	return RebuildSite(st, site)
}

// applyCommentDefaults 评论配置的运行时默认值（与 main.go 启动兑底一致）。
func applyCommentDefaults(cc *tmpl.CommentsConfig) {
	if cc.GravatarMirror == "" {
		cc.GravatarMirror = "https://weavatar.com/avatar"
	}
	if cc.AdminBadge == "" {
		cc.AdminBadge = "博主"
	}
	if cc.Title == "" {
		cc.Title = "评论"
	}
}

// defaultDashboardConfig 返回仪表盘默认配置（全部部件启用、默认顺序）。
// ID 与 Console 部件注册表（console/src/modules/dashboard/widgets/registry.ts）一致。
func defaultDashboardConfig() dashboardConfig {
	return dashboardConfig{
		Widgets: []dashboardWidget{
			{ID: "stat-posts", Enabled: true},
			{ID: "stat-comments", Enabled: true},
			{ID: "stat-views", Enabled: true},
			{ID: "stat-attachments", Enabled: true},
			{ID: "quick-actions", Enabled: true},
			{ID: "recent-comments", Enabled: true},
			{ID: "trend-chart", Enabled: true},
			{ID: "hourly-chart", Enabled: true},
		},
	}
}
