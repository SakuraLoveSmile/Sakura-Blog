package config

import (
	tmpl "blog/internal/settings"
	"blog/internal/store"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

var siteUpdateMu sync.Mutex

// RebuildSite 从 DB 重新加载 settings/links，构建全新 *tmpl.Site 后经 atomic.Pointer
// 复制-交换发布，消除前台渲染（无锁读）与热更新写入之间的数据竞争。
// 未覆盖字段（LogoIcon/SiteTree 等启动后不再变化）沿用当前值，读-only 共享安全。
// Admin.AuthEnabled 从 users 表 admin 用户派生（Spec 61 阶段 4 起密码唯一真值在 users 表）。
// 设置/友链保存后调用以实现热更新。
func RebuildSite(st *store.Store, site *atomic.Pointer[tmpl.Site]) error {
	siteUpdateMu.Lock()
	defer siteUpdateMu.Unlock()
	settings, err := st.AllSettings()
	if err != nil {
		return err
	}
	next, err := buildSite(st, site.Load(), settings)
	if err != nil {
		return err
	}
	site.Store(next)
	InvalidateLoadAI()
	return nil
}

// RebuildSiteFromBase 使用恢复后的文件配置作基底，在同一锁内构造并发布完整快照。
func RebuildSiteFromBase(st *store.Store, site *atomic.Pointer[tmpl.Site], base *tmpl.Site) error {
	siteUpdateMu.Lock()
	defer siteUpdateMu.Unlock()
	settings, err := st.AllSettings()
	if err != nil {
		return err
	}
	next, err := buildSite(st, base, settings)
	if err != nil {
		return err
	}
	site.Store(next)
	InvalidateLoadAI()
	return nil
}

func buildSite(st *store.Store, current *tmpl.Site, settings map[string]string) (*tmpl.Site, error) {
	if err := validateSettingsSnapshot(settings); err != nil {
		return nil, err
	}
	links, err := st.ListLinks()
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("site snapshot is nil")
	}
	next := *current // 浅拷贝当前站点，settings 覆盖字段在 rebuildFromValues 中整体替换为全新值
	// 显式版本文档一旦存在就必须通过 strict；恢复和其他本地设置热更新
	// 不能丢弃新主题快照或因校验失败退回旧主题。
	if _, exists := settings[SectionStellarV2]; exists {
		configuration, err := LoadStellarV2Snapshot(settings)
		if err != nil {
			return nil, err
		}
		next.StellarV2 = configuration
	}
	groups := LinksToGroups(links)
	rebuildFromValues(&next, settings)
	if next.StellarV2 != nil {
		if err := ApplyStellarBlogRuntime(&next); err != nil {
			return nil, err
		}
	}
	next.Links = groups
	// AuthEnabled 派生（Spec 61 阶段 4）：users 表 admin 用户存在且密码哈希非空 ⇒ 需要密码登录；
	// 无密码站点（哈希为空/无 admin 用户）保持全放行语义。
	adminUser, err := st.GetUserByUsername("admin")
	if err != nil {
		return nil, err
	}
	next.Admin.AuthEnabled = adminUser != nil && adminUser.PasswordHash != ""
	return &next, nil
}

// parsedSections 是 settings 分段的解析结果。解析与赋值分离，保证 RebuildSite 的全量替换
// 与 ApplyDraftSections 的逐段覆盖共用同一份字段映射。
type parsedSections struct {
	basic         siteBasic
	menubar       tmpl.Menubar
	rightbar      tmpl.RightbarConfig
	footer        tmpl.Footer
	comments      tmpl.CommentsConfig
	article       tmpl.ArticleConfig
	search        tmpl.SearchConfig
	siteTree      tmpl.SiteTreeConfig
	wiki          []tmpl.WikiProject
	friends       tmpl.FriendsConfig
	seo           tmpl.SEOConfig
	inject        tmpl.InjectConfig
	style         tmpl.StyleConfig
	widgets       tmpl.WidgetsConfig
	cover         tmpl.CoverConfig
	navTabs       tmpl.NavTabsConfig
	authors       map[string]tmpl.AuthorDef
	plugins       tmpl.PluginsConfig
	tagPlugins    tmpl.TagPluginsConfig
	defaultAssets tmpl.DefaultAssets
	pages         tmpl.PagesConfig
	dataCache     tmpl.DataCacheConfig
	services      tmpl.ServicesConfig
	feedback      tmpl.FeedbackConfig
}

func parseSections(settings map[string]string) parsedSections {
	var parsed parsedSections
	var unmarshal = func(section string, dst any) {
		if v, ok := settings[section]; ok && v != "" {
			_ = json.Unmarshal([]byte(v), dst)
		}
	}
	unmarshal(SectionSite, &parsed.basic)
	unmarshal(SectionMenubar, &parsed.menubar)
	unmarshal(SectionRightbar, &parsed.rightbar)
	unmarshal(SectionFooter, &parsed.footer)
	unmarshal(SectionComments, &parsed.comments)
	unmarshal(SectionArticle, &parsed.article)
	// 1.41 ai_label：article 段整段覆盖，未配置 ai_label 键时补回主题内置四档
	parsed.article.AILabel.FillDefaults()
	unmarshal(SectionSearch, &parsed.search)
	unmarshal(SectionSiteTree, &parsed.siteTree)
	unmarshal(SectionWiki, &parsed.wiki)
	unmarshal(SectionFriends, &parsed.friends)
	if raw, ok := settings[SectionFriends]; ok {
		var fields map[string]json.RawMessage
		if json.Unmarshal([]byte(raw), &fields) == nil {
			if _, exists := fields["check_interval"]; !exists {
				parsed.friends.CheckInterval = 360
			}
			if _, exists := fields["snapshot_interval"]; !exists {
				parsed.friends.SnapshotInterval = 1440
			}
			if _, exists := fields["snapshot_provider"]; !exists || parsed.friends.SnapshotProvider == "" {
				parsed.friends.SnapshotProvider = tmpl.DefaultSnapshotProvider
			}
		}
	}
	unmarshal(SectionSEO, &parsed.seo)
	unmarshal(SectionInject, &parsed.inject)
	unmarshal(SectionStyle, &parsed.style)
	if parsed.style.CornerShape == "" {
		parsed.style.CornerShape = "superellipse(1.25)"
	}
	if parsed.style.Leftbar.UIStyle == "" {
		parsed.style.Leftbar.UIStyle = "card"
	}
	if parsed.style.Gradient.Avatar == "" {
		parsed.style.Gradient.Avatar = "conic-gradient(from 0deg, #04f3ff, #08ffc6, #ddf730, #ffbd19, #ff1fe0, #c418ff, #3b5bff, #04f3ff)"
	}
	if parsed.article.PinStyle == "" {
		parsed.article.PinStyle = "carousel"
	}
	parsed.search.FillDefaults()
	unmarshal(SectionWidgets, &parsed.widgets)
	unmarshal(SectionCover, &parsed.cover)
	unmarshal(SectionNavTabs, &parsed.navTabs)
	unmarshal(SectionAuthors, &parsed.authors)
	unmarshal(SectionPlugins, &parsed.plugins)
	unmarshal(SectionTagPlugins, &parsed.tagPlugins)
	parsed.tagPlugins.FillDefaults()
	unmarshal(SectionDefault, &parsed.defaultAssets)
	unmarshal(SectionPages, &parsed.pages)
	unmarshal(SectionDataCache, &parsed.dataCache)
	parsed.dataCache.FillDefaults()
	unmarshal(SectionServices, &parsed.services)
	unmarshal(SectionFeedback, &parsed.feedback)
	return parsed
}

// applyParsedSections 写入解析后的分段。all=true 复现 RebuildSite 的全量零值替换；
// all=false 时仅覆盖 settings 中实际提供的分段，供预览草稿保留未编辑配置。
func applyParsedSections(site *tmpl.Site, parsed parsedSections, settings map[string]string, all bool) {
	assign := func(section string, fn func()) {
		if all {
			fn()
			return
		}
		if _, ok := settings[section]; ok {
			fn()
		}
	}
	assign(SectionSite, func() {
		site.Title = parsed.basic.Title
		site.Subtitle = parsed.basic.Subtitle
		site.Description = parsed.basic.Description
		site.URL = tmpl.NormalizeSiteURL(parsed.basic.URL)
		site.Avatar = parsed.basic.Avatar
		site.AvatarBG = parsed.basic.AvatarBG
		site.LogoTitle = parsed.basic.LogoTitle
		site.LogoSub = parsed.basic.LogoSub
		site.FaviconURL = parsed.basic.FaviconURL
		site.AppleIcon = parsed.basic.AppleIcon
		site.SWCacheVersion = parsed.basic.SWCacheVer
	})
	assign(SectionMenubar, func() { site.Menubar = parsed.menubar })
	assign(SectionRightbar, func() { site.Rightbar = parsed.rightbar })
	assign(SectionFooter, func() { site.Footer = parsed.footer })
	assign(SectionComments, func() { site.Comments = parsed.comments })
	assign(SectionArticle, func() { site.Article = parsed.article })
	assign(SectionSearch, func() { site.Search = parsed.search })
	assign(SectionSiteTree, func() { site.SiteTree = parsed.siteTree })
	assign(SectionWiki, func() { site.Wiki = parsed.wiki })
	assign(SectionFriends, func() { site.Friends = parsed.friends })
	assign(SectionSEO, func() { site.SEO = parsed.seo })
	assign(SectionInject, func() { site.Inject = parsed.inject })
	assign(SectionStyle, func() { site.Style = parsed.style })
	assign(SectionWidgets, func() { site.Widgets = parsed.widgets })
	assign(SectionCover, func() { site.Cover = parsed.cover })
	assign(SectionNavTabs, func() { site.NavTabs = parsed.navTabs })
	assign(SectionAuthors, func() { site.Authors = parsed.authors })
	assign(SectionPlugins, func() { site.Plugins = parsed.plugins })
	assign(SectionTagPlugins, func() { site.TagPlugins = parsed.tagPlugins })
	assign(SectionDefault, func() { site.Default = parsed.defaultAssets })
	assign(SectionPages, func() { site.Pages = parsed.pages })
	assign(SectionDataCache, func() { site.DataCache = parsed.dataCache })
	assign(SectionServices, func() { site.Services = parsed.services })
	assign(SectionFeedback, func() { site.Feedback = parsed.feedback })
}

// rebuildFromValues 解析各分段 JSON 并写入 site 字段（不含 links/admin/lang 覆盖，调用方自理）。
// RebuildSite 使用全量替换：DB settings 是站点配置的完整真值。
func rebuildFromValues(site *tmpl.Site, settings map[string]string) {
	parsed := parseSections(settings)
	applyParsedSections(site, parsed, settings, true)
}

// ApplyDraftSections 把草稿分段 JSON 就地应用到传入的 Site 副本（设置页「前端预览」用）：
// 与 RebuildSite 同一解析映射，但不触碰 DB、全局共享 site、links、admin 密码与全局文案覆盖。
// raw 中未提供的分段保持 site 原值。
func ApplyDraftSections(site *tmpl.Site, raw map[string]json.RawMessage) {
	settings := make(map[string]string, len(raw))
	for k, v := range raw {
		settings[k] = string(v)
	}
	parsed := parseSections(settings)
	applyParsedSections(site, parsed, settings, false)
}
