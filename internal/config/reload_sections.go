package config

import (
	tmpl "blog/internal/settings"
	"blog/internal/store"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
			Unavailable: linkUnavailable(l),
		})
	}
	return groups
}

// linkUnavailable derives the public display state from persisted health data
// and the two conventional manually assigned health labels. A zero timestamp
// means that the default zero status has never been checked.
func linkUnavailable(l *store.Link) bool {
	if l == nil {
		return false
	}
	label := strings.TrimSpace(l.Label)
	if label == "失效" || label == "失联" {
		return true
	}
	return l.LastCheckedAt > 0 && (l.LastStatus < http.StatusOK || l.LastStatus >= http.StatusMultipleChoices)
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
		SectionPages, SectionDataCache, SectionServices,
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
	siteUpdateMu.Lock()
	defer siteUpdateMu.Unlock()
	settings, err := st.AllSettings()
	if err != nil {
		return err
	}
	raw = preserveMaskedSecrets(raw, settings[section], section)
	dst, err := sectionTarget(section)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("invalid %s payload: %w", section, err)
	}
	if sb, ok := dst.(*siteBasic); ok {
		sb.URL = strings.TrimSpace(sb.URL)
		if err := tmpl.ValidateSiteURL(sb.URL); err != nil {
			return err
		}
		sb.URL = tmpl.NormalizeSiteURL(sb.URL)
	}
	if cc, ok := dst.(*tmpl.CommentsConfig); ok {
		applyCommentDefaults(cc) // 评论段补运行时默认（镜像/徽章/标题不为空）
	}
	if search, ok := dst.(*tmpl.SearchConfig); ok {
		if search.LocalSearch.CacheTTL != nil && *search.LocalSearch.CacheTTL < 0 {
			return fmt.Errorf("search.local_search.cache_ttl must be >= 0")
		}
		search.FillDefaults()
	}
	if cache, ok := dst.(*tmpl.DataCacheConfig); ok {
		if cache.DefaultTTL != nil && *cache.DefaultTTL < 0 {
			return fmt.Errorf("data_cache.default_ttl must be >= 0")
		}
		if cache.MaxEntries != nil && *cache.MaxEntries < 1 {
			return fmt.Errorf("data_cache.max_entries must be >= 1")
		}
		for name, ttl := range cache.TTL {
			if ttl < 0 {
				return fmt.Errorf("data_cache.ttl.%s must be >= 0", name)
			}
		}
		cache.FillDefaults()
	}
	if tagPlugins, ok := dst.(*tmpl.TagPluginsConfig); ok {
		tagPlugins.FillDefaults()
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
	if isRuntimeDefaultsSection(section) {
		norm, err = mergeRuntimeDefaults(raw, norm, section)
		if err != nil {
			return fmt.Errorf("normalize %s payload: %w", section, err)
		}
	} else if section == SectionStyle {
		norm, err = mergeUnknownFields(raw, norm)
		if err != nil {
			return fmt.Errorf("normalize %s payload: %w", section, err)
		}
	}
	settings[section] = string(norm)
	if err := validateSettingsSnapshot(settings); err != nil {
		return err
	}
	next, err := buildSite(st, site.Load(), settings)
	if err != nil {
		return fmt.Errorf("build site snapshot: %w", err)
	}
	if err := st.PutSetting(section, string(norm)); err != nil {
		return err
	}
	site.Store(next)
	InvalidateLoadAI()
	return nil
}

func sectionTarget(section string) (any, error) {
	switch section {
	case SectionSite:
		return &siteBasic{}, nil
	case SectionMenubar:
		return &tmpl.Menubar{}, nil
	case SectionRightbar:
		return &tmpl.RightbarConfig{}, nil
	case SectionFooter:
		return &tmpl.Footer{}, nil
	case SectionComments:
		return &tmpl.CommentsConfig{}, nil
	case SectionArticle:
		return &tmpl.ArticleConfig{}, nil
	case SectionSearch:
		return &tmpl.SearchConfig{}, nil
	case SectionSiteTree:
		return &tmpl.SiteTreeConfig{}, nil
	case SectionWiki:
		return &[]tmpl.WikiProject{}, nil
	case SectionFriends:
		return &tmpl.FriendsConfig{}, nil
	case SectionSEO:
		return &tmpl.SEOConfig{}, nil
	case SectionInject:
		return &tmpl.InjectConfig{}, nil
	case SectionDashboard:
		return &dashboardConfig{}, nil
	case SectionStyle:
		return &tmpl.StyleConfig{}, nil
	case SectionWidgets:
		return &tmpl.WidgetsConfig{}, nil
	case SectionAI:
		return &AIConfig{}, nil
	case SectionCover:
		return &tmpl.CoverConfig{}, nil
	case SectionNavTabs:
		return &tmpl.NavTabsConfig{}, nil
	case SectionAuthors:
		return &map[string]tmpl.AuthorDef{}, nil
	case SectionPlugins:
		return &tmpl.PluginsConfig{}, nil
	case SectionTagPlugins:
		return &tmpl.TagPluginsConfig{}, nil
	case SectionDefault:
		return &tmpl.DefaultAssets{}, nil
	case SectionPages:
		return &tmpl.PagesConfig{}, nil
	case SectionDataCache:
		return &tmpl.DataCacheConfig{}, nil
	case SectionServices:
		return &tmpl.ServicesConfig{}, nil
	default:
		return nil, fmt.Errorf("unknown section: %s", section)
	}
}

func validateSettingsSnapshot(settings map[string]string) error {
	for _, section := range KnownSections() {
		raw, ok := settings[section]
		if !ok || strings.TrimSpace(raw) == "" {
			continue
		}
		dst, err := sectionTarget(section)
		if err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(raw), dst); err != nil {
			return fmt.Errorf("invalid stored %s payload: %w", section, err)
		}
	}
	return nil
}

// preserveMaskedSecrets keeps stored credentials when a Console form submits
// the redacted value or an empty field. It runs while the settings snapshot is
// held by SaveSection, so validation and persistence use one coherent input.
func preserveMaskedSecrets(raw []byte, existing, section string) []byte {
	if existing == "" || (section != SectionComments && section != SectionAI) {
		return raw
	}
	var incoming, stored map[string]any
	if json.Unmarshal(raw, &incoming) != nil || json.Unmarshal([]byte(existing), &stored) != nil {
		return raw
	}
	if section == SectionAI {
		key, _ := incoming["api_key"].(string)
		if key == "" || key == "****" {
			if value, ok := stored["api_key"].(string); ok {
				incoming["api_key"] = value
			}
		}
	} else {
		notify, _ := incoming["notify"].(map[string]any)
		storedNotify, _ := stored["notify"].(map[string]any)
		if notify == nil || storedNotify == nil {
			return raw
		}
		password, _ := notify["password"].(string)
		if password == "" || password == "****" {
			if value, ok := storedNotify["password"].(string); ok {
				notify["password"] = value
			}
		}
	}
	result, err := json.Marshal(incoming)
	if err != nil {
		return raw
	}
	return result
}

func mergeUnknownFields(raw, normalized []byte) ([]byte, error) {
	var current map[string]json.RawMessage
	if err := json.Unmarshal(raw, &current); err != nil || current == nil {
		return normalized, nil
	}
	var normMap map[string]json.RawMessage
	if err := json.Unmarshal(normalized, &normMap); err != nil || normMap == nil {
		return normalized, nil
	}
	for k, v := range current {
		if _, exists := normMap[k]; !exists {
			normMap[k] = v
		}
	}
	return json.Marshal(normMap)
}

func isRuntimeDefaultsSection(section string) bool {
	switch section {
	case SectionSearch, SectionDataCache, SectionTagPlugins, SectionServices:
		return true
	default:
		return false
	}
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
