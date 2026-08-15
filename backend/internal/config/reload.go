package config

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"blog/internal/store"
	tmpl "blog/internal/template"
)

// 站点配置分段名（settings 表的 section 主键）。
const (
	SectionSite      = "site"      // 基础信息（标题/副标题/头像/logo）
	SectionMenubar   = "menubar"   // 菜单栏
	SectionRightbar  = "rightbar"  // 右栏
	SectionFooter    = "footer"    // 页脚
	SectionComments  = "comments"  // 评论系统
	SectionArticle   = "article"   // 文章页脚
	SectionSearch    = "search"    // 本地搜索
	SectionSiteTree  = "site_tree" // 页面树（首页评论等）
	SectionWiki      = "wiki"      // 文库项目
	SectionFriends   = "friends"   // 友链动态聚合
	SectionSEO       = "seo"       // 站点级 SEO（Spec 42）
	SectionInject    = "inject"    // 代码注入（Spec 42，仅入库不入 site.yml）
	SectionDashboard = "dashboard" // 仪表盘组件自定义（仅 Console 消费）
	SectionStyle     = "style"     // 外观样式（Spec 34/47）
	SectionWidgets   = "widgets"   // 侧边栏组件编排（Spec 33/47）
	SectionAI        = "ai"        // AI 属性生成（OpenAI 兼容接口配置，仅 Console/后端代理消费）
	// 以下五段此前为 site.yml 纯文件配置，现接入 settings 热更新管线（入库后以 DB 为准）：
	SectionCover      = "cover"       // 首页 Hero cover 区
	SectionNavTabs    = "nav_tabs"    // 导航栏标签页
	SectionAuthors    = "authors"     // 多作者体系
	SectionPlugins    = "plugins"     // 前端体验增强插件开关
	SectionTagPlugins = "tag_plugins" // 标签插件默认参数（对齐原版 _config.yml tag_plugins）
	SectionDefault    = "default"     // 占位素材组（对齐原版 _config.yml default）
	SectionPages      = "pages"       // 页面级设置（friends 页等特色设置，按页面键管理）
	SectionNotebook   = "notebook"    // 笔记本默认行为

	// SectionAdmin 管理密码段（迁移用，Spec 61 阶段 4）：明文仅存于迁移窗口期——
	// Bootstrap 检测非空时迁移到 users 表（bcrypt）并清空，此后不再写入。
	SectionAdmin = "admin"
)

// siteBasic 对应 settings 表 "site" 段的基础信息。
type siteBasic struct {
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Avatar      string `json:"avatar"`
	AvatarBG    string `json:"avatar_bg"`
	LogoTitle   string `json:"logo_title"`
	LogoSub     string `json:"logo_sub"`
	// favicon/apple_touch_icon：Spec Task 9 的 Console UI 依赖；sw_cache_version 供 /sw.js 注入 CACHE_VERSION。
	FaviconURL string `json:"favicon_url"`      // Favicon URL，留空回退 /favicon.ico
	AppleIcon  string `json:"apple_touch_icon"` // Apple Touch Icon URL，留空回退默认图
	SWCacheVer string `json:"sw_cache_version"` // Service Worker 缓存版本，留空用内置缺省
}

// AIConfig 对应 settings 表 "ai" 段：OpenAI 兼容接口配置，供 AI 属性生成代理使用。
type AIConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
	// Spec 53 AI 摘要开关（nil=未配置，默认开启，兼容旧配置）。
	SummaryEnabled *bool `json:"summary_enabled,omitempty"`
	AllowUserAPI   *bool `json:"allow_user_api,omitempty"`
}

// SummaryOn 前台是否展示 AI 摘要（未配置=开）。
func (c AIConfig) SummaryOn() bool { return c.SummaryEnabled == nil || *c.SummaryEnabled }

// UserAPIOn 是否允许读者临时接入自己的 API 生成摘要（未配置=开）。
func (c AIConfig) UserAPIOn() bool { return c.AllowUserAPI == nil || *c.AllowUserAPI }

// LoadAI 从 settings 读取 AI 配置（未配置时返回零值）。
// Spec 61 阶段 5：内存缓存——文章页每请求读 settings 改为 RebuildSite（设置保存）时刷新，
// 首次调用加载，成功即缓存；失败不缓存（下次重试）。
var (
	loadAIMu      sync.RWMutex
	loadAICached  AIConfig
	loadAICacheOK bool
)

func LoadAI(st *store.Store) (AIConfig, error) {
	loadAIMu.RLock()
	if loadAICacheOK {
		cfg := loadAICached
		loadAIMu.RUnlock()
		return cfg, nil
	}
	loadAIMu.RUnlock()

	loadAIMu.Lock()
	defer loadAIMu.Unlock()
	if loadAICacheOK {
		return loadAICached, nil
	}
	var cfg AIConfig
	v, err := st.GetSetting(SectionAI)
	if err != nil || v == "" {
		return cfg, err
	}
	if err := json.Unmarshal([]byte(v), &cfg); err != nil {
		return AIConfig{}, err
	}
	loadAICached = cfg
	loadAICacheOK = true
	return cfg, nil
}

// InvalidateLoadAI 失效 AI 配置缓存（RebuildSite 内调用，设置保存后下次读取即重载）。
func InvalidateLoadAI() {
	loadAIMu.Lock()
	loadAICacheOK = false
	loadAIMu.Unlock()
}

// dashboardWidget 对应 settings 表 "dashboard" 段的单个组件配置。
type dashboardWidget struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

// dashboardConfig 对应 settings 表 "dashboard" 段（仪表盘组件开关与排序）。
type dashboardConfig struct {
	Widgets []dashboardWidget `json:"widgets"`
}

// Bootstrap 首次启动导入：settings/links 表为空时，把 site（DefaultSite 合并 site.yml）写入 DB。
// 幂等——非空表跳过，二次启动不重复导入。
func Bootstrap(st *store.Store, site *tmpl.Site) error {
	if err := st.EnsureSettingsTable(); err != nil {
		return err
	}
	if err := st.EnsureLinksTable(); err != nil {
		return err
	}

	n, err := st.CountSettings()
	if err != nil {
		return err
	}
	if n == 0 {
		if err := writeSiteSections(st, site); err != nil {
			return err
		}
	}

	settings, err := st.AllSettings()
	if err != nil {
		return err
	}
	// admin 段兼容补导（Spec 61 阶段 4 收紧）：仅当 users 表 admin 尚无密码哈希时，
	// 才把 site.yml 初始密码补入 admin 段供下方迁移消费；已迁移的库不再从 site.yml 回灌明文。
	if _, ok := settings[SectionAdmin]; !ok && site.Admin.Password != "" {
		if adminUser, _ := st.GetUserByUsername("admin"); adminUser == nil || adminUser.PasswordHash == "" {
			b, err := json.Marshal(map[string]string{"password": site.Admin.Password})
			if err != nil {
				return err
			}
			if err := st.PutSetting(SectionAdmin, string(b)); err != nil {
				return err
			}
		}
	}

	// seo/inject 段兼容导入（Spec 42）：旧库升级时 settings 非空但缺段，补导一次，
	// 避免 RebuildSite 用零值覆盖 site.yml 加载的 seo 配置。
	if _, ok := settings[SectionSEO]; !ok {
		b, err := json.Marshal(site.SEO)
		if err != nil {
			return err
		}
		if err := st.PutSetting(SectionSEO, string(b)); err != nil {
			return err
		}
	}
	if _, ok := settings[SectionSearch]; !ok {
		b, err := json.Marshal(site.Search)
		if err != nil {
			return err
		}
		if err := st.PutSetting(SectionSearch, string(b)); err != nil {
			return err
		}
	}
	if _, ok := settings[SectionSiteTree]; !ok {
		b, err := json.Marshal(site.SiteTree)
		if err != nil {
			return err
		}
		if err := st.PutSetting(SectionSiteTree, string(b)); err != nil {
			return err
		}
	}
	if _, ok := settings[SectionNotebook]; !ok {
		b, err := json.Marshal(site.Notebook)
		if err != nil {
			return err
		}
		if err := st.PutSetting(SectionNotebook, string(b)); err != nil {
			return err
		}
	}
	if _, ok := settings[SectionInject]; !ok {
		b, err := json.Marshal(site.Inject)
		if err != nil {
			return err
		}
		if err := st.PutSetting(SectionInject, string(b)); err != nil {
			return err
		}
	}
	if _, ok := settings[SectionStyle]; !ok {
		b, _ := json.Marshal(site.Style)
		st.PutSetting(SectionStyle, string(b))
	}
	if _, ok := settings[SectionWidgets]; !ok {
		b, _ := json.Marshal(site.Widgets)
		st.PutSetting(SectionWidgets, string(b))
	}
	// cover/nav_tabs/authors/plugins 段兼容导入：旧库升级时 settings 非空但缺段，
	// 从 site.yml 初值补导一次，之后以 DB 为准（与 seo/style/widgets 同语义）。
	if _, ok := settings[SectionCover]; !ok {
		b, _ := json.Marshal(site.Cover)
		st.PutSetting(SectionCover, string(b))
	}
	if _, ok := settings[SectionNavTabs]; !ok {
		b, _ := json.Marshal(site.NavTabs)
		st.PutSetting(SectionNavTabs, string(b))
	}
	if _, ok := settings[SectionAuthors]; !ok {
		b, _ := json.Marshal(site.Authors)
		st.PutSetting(SectionAuthors, string(b))
	}
	if _, ok := settings[SectionPlugins]; !ok {
		b, _ := json.Marshal(site.Plugins)
		st.PutSetting(SectionPlugins, string(b))
	}

	// site 段 favicon 兼容导入：旧库 site 段缺 favicon_url/apple_touch_icon 键时，
	// 从 site.yml 初值合并补写，避免 RebuildSite 把已加载值清零（site.yml 留空则不补）。
	if raw, ok := settings[SectionSite]; ok && raw != "" {
		var sec map[string]json.RawMessage
		if json.Unmarshal([]byte(raw), &sec) == nil {
			changed := false
			if _, has := sec["favicon_url"]; !has && site.FaviconURL != "" {
				b, _ := json.Marshal(site.FaviconURL)
				sec["favicon_url"] = b
				changed = true
			}
			if _, has := sec["apple_touch_icon"]; !has && site.AppleIcon != "" {
				b, _ := json.Marshal(site.AppleIcon)
				sec["apple_touch_icon"] = b
				changed = true
			}
			if changed {
				if b, err := json.Marshal(sec); err == nil {
					st.PutSetting(SectionSite, string(b))
				}
			}
		}
	}

	nl, err := st.CountLinks()
	if err != nil {
		return err
	}
	if nl == 0 && len(site.Links) > 0 {
		if err := writeSiteLinks(st, site.Links); err != nil {
			return err
		}
	}

	// Spec 61 阶段 4: admin 密码迁移到 users 表（幂等）。
	// settings admin 段非空（旧版本明文密码）时：确保 users 表 admin 用户存在且 bcrypt 匹配该密码
	// （缺失/不匹配则创建/更新），随后清空 admin 段；之后密码唯一真值在 users 表。
	if err := st.EnsureUsersTable(); err != nil {
		log.Printf("[warn] EnsureUsersTable: %v", err)
	}
	if settings, _ := st.AllSettings(); settings[SectionAdmin] != "" {
		var adminCfg struct {
			Password string `json:"password"`
		}
		if json.Unmarshal([]byte(settings[SectionAdmin]), &adminCfg) == nil && adminCfg.Password != "" {
			admin, err := st.GetUserByUsername("admin")
			if err != nil {
				return err
			}
			needCreate := admin == nil
			needUpdate := !needCreate && store.CheckPassword(admin.PasswordHash, adminCfg.Password) != nil
			if needCreate {
				hash, err := store.HashPassword(adminCfg.Password)
				if err != nil {
					return err
				}
				admin = &store.User{
					Username:     "admin",
					PasswordHash: hash,
					DisplayName:  "管理员",
					AuthSecret:   store.GenerateAuthSecret(),
					Role:         store.RoleAdmin,
				}
				if err := st.CreateUser(admin); err != nil {
					log.Printf("[warn] migrate admin user: %v", err)
				} else {
					log.Printf("[info] Spec 61: migrated admin password to users table (user: admin)")
				}
			} else if needUpdate {
				hash, err := store.HashPassword(adminCfg.Password)
				if err != nil {
					return err
				}
				if err := st.UpdatePassword(admin.ID, hash, admin.AuthSecret); err != nil {
					log.Printf("[warn] update admin password hash: %v", err)
				} else {
					log.Printf("[info] Spec 61: refreshed admin password hash from settings admin section")
				}
			}
		}
		// 无论是否迁移成功，清空明文 admin 段（此后密码只存在于 users 表）
		if err := st.PutSetting(SectionAdmin, ""); err != nil {
			log.Printf("[warn] clear admin section: %v", err)
		}
	}

	return nil
}

// writeSiteSections 把 site 各段序列化为 JSON 写入 settings 表。
func writeSiteSections(st *store.Store, site *tmpl.Site) error {
	put := func(section string, v any) error {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		return st.PutSetting(section, string(b))
	}
	if err := put(SectionSite, siteBasic{
		Title: site.Title, Subtitle: site.Subtitle, Description: site.Description,
		URL: site.URL, Avatar: site.Avatar, AvatarBG: site.AvatarBG,
		LogoTitle: site.LogoTitle, LogoSub: site.LogoSub,
	}); err != nil {
		return err
	}
	if err := put(SectionMenubar, site.Menubar); err != nil {
		return err
	}
	if err := put(SectionRightbar, site.Rightbar); err != nil {
		return err
	}
	if err := put(SectionFooter, site.Footer); err != nil {
		return err
	}
	if err := put(SectionComments, site.Comments); err != nil {
		return err
	}
	if err := put(SectionArticle, site.Article); err != nil {
		return err
	}
	if err := put(SectionSearch, site.Search); err != nil {
		return err
	}
	if err := put(SectionSiteTree, site.SiteTree); err != nil {
		return err
	}
	if err := put(SectionWiki, site.Wiki); err != nil {
		return err
	}
	if err := put(SectionFriends, site.Friends); err != nil {
		return err
	}
	if err := put(SectionSEO, site.SEO); err != nil {
		return err
	}
	if err := put(SectionInject, site.Inject); err != nil {
		return err
	}
	if err := put(SectionDashboard, defaultDashboardConfig()); err != nil {
		return err
	}
	if err := put(SectionStyle, site.Style); err != nil {
		return err
	}
	if err := put(SectionWidgets, site.Widgets); err != nil {
		return err
	}
	if err := put(SectionCover, site.Cover); err != nil {
		return err
	}
	if err := put(SectionNavTabs, site.NavTabs); err != nil {
		return err
	}
	if err := put(SectionAuthors, site.Authors); err != nil {
		return err
	}
	if err := put(SectionPlugins, site.Plugins); err != nil {
		return err
	}
	if err := put(SectionNotebook, site.Notebook); err != nil {
		return err
	}
	if err := put(SectionPages, site.Pages); err != nil {
		return err
	}
	// lang 段不写初始值：settings 无 lang 行 = 无文案覆盖（全用内置 zhCN）。
	return nil
}

// writeSiteLinks 把 site.Links（分组）扁平化写入 links 表。
func writeSiteLinks(st *store.Store, groups []tmpl.LinkGroup) error {
	for _, g := range groups {
		for _, it := range g.Items {
			if _, err := st.CreateLink(&store.Link{
				GroupTitle:  g.Title,
				Name:        it.Name,
				URL:         it.URL,
				Avatar:      it.Avatar,
				Description: it.Desc,
				RSS:         it.RSS,
				Label:       it.Label,
				Color:       it.Color,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// RebuildSite 从 DB 重新加载 settings/links，构建全新 *tmpl.Site 后经 atomic.Pointer
// 复制-交换发布，消除前台渲染（无锁读）与热更新写入之间的数据竞争。
// 未覆盖字段（LogoIcon/DataCache/SiteTree 等启动后不再变化）沿用当前值，读-only 共享安全。
// Admin.AuthEnabled 从 users 表 admin 用户派生（Spec 61 阶段 4 起密码唯一真值在 users 表）。
// 设置/友链保存后调用以实现热更新。
func RebuildSite(st *store.Store, site *atomic.Pointer[tmpl.Site]) error {
	settings, err := st.AllSettings()
	if err != nil {
		return err
	}
	links, err := st.ListLinks()
	if err != nil {
		return err
	}

	next := *site.Load() // 浅拷贝当前站点，settings 覆盖字段在 rebuildFromValues 中整体替换为全新值
	groups := LinksToGroups(links)
	rebuildFromValues(&next, settings)
	next.Links = groups
	tmpl.SetTagPlugins(next.TagPlugins) // 标签插件默认参数全局生效（预览草稿不触碰）
	// AuthEnabled 派生（Spec 61 阶段 4）：users 表 admin 用户存在且密码哈希非空 ⇒ 需要密码登录；
	// 无密码站点（哈希为空/无 admin 用户）保持全放行语义。
	adminUser, err := st.GetUserByUsername("admin")
	if err != nil {
		return err
	}
	next.Admin.AuthEnabled = adminUser != nil && adminUser.PasswordHash != ""
	site.Store(&next)
	InvalidateLoadAI() // Spec 61 阶段 5：设置保存后 AI 配置缓存失效，下次读取重载
	return nil
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
	notebook      tmpl.NotebookConfig
	tagPlugins    tmpl.TagPluginsConfig
	defaultAssets tmpl.DefaultAssets
	pages         tmpl.PagesConfig
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
	if parsed.search.Service == "" {
		parsed.search.Service = "local_search"
	}
	if parsed.search.LocalSearch.Field == "" {
		parsed.search.LocalSearch.Field = "all"
	}
	if parsed.search.LocalSearch.Path == "" {
		parsed.search.LocalSearch.Path = "/search.json"
	}
	if parsed.search.LocalSearch.Content == nil {
		v := true
		parsed.search.LocalSearch.Content = &v
	}
	unmarshal(SectionWidgets, &parsed.widgets)
	unmarshal(SectionCover, &parsed.cover)
	unmarshal(SectionNavTabs, &parsed.navTabs)
	unmarshal(SectionAuthors, &parsed.authors)
	unmarshal(SectionPlugins, &parsed.plugins)
	unmarshal(SectionNotebook, &parsed.notebook)
	if raw, ok := settings[SectionNotebook]; !ok || raw == "" {
		parsed.notebook.AutoExcerpt = 128
		parsed.notebook.OrderBy = "-updated"
	} else {
		var fields map[string]json.RawMessage
		if json.Unmarshal([]byte(raw), &fields) == nil {
			if _, exists := fields["auto_excerpt"]; !exists && parsed.notebook.AutoExcerpt == 0 {
				parsed.notebook.AutoExcerpt = 128
			}
			if _, exists := fields["order_by"]; !exists && parsed.notebook.OrderBy == "" {
				parsed.notebook.OrderBy = "-updated"
			}
		}
	}
	unmarshal(SectionTagPlugins, &parsed.tagPlugins)
	unmarshal(SectionDefault, &parsed.defaultAssets)
	unmarshal(SectionPages, &parsed.pages)
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
		site.URL = parsed.basic.URL
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
	assign(SectionNotebook, func() { site.Notebook = parsed.notebook })
	assign(SectionTagPlugins, func() { site.TagPlugins = parsed.tagPlugins })
	assign(SectionDefault, func() { site.Default = parsed.defaultAssets })
	assign(SectionPages, func() { site.Pages = parsed.pages })
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
		SectionSearch, SectionSiteTree, SectionNotebook,
		SectionSEO, SectionInject, SectionDashboard,
		SectionStyle, SectionWidgets, SectionAI,
		SectionCover, SectionNavTabs, SectionAuthors, SectionPlugins,
		SectionTagPlugins, SectionDefault,
		SectionPages,
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
	case SectionNotebook:
		dst = &tmpl.NotebookConfig{}
	case SectionTagPlugins:
		dst = &tmpl.TagPluginsConfig{}
	case SectionDefault:
		dst = &tmpl.DefaultAssets{}
	case SectionPages:
		dst = &tmpl.PagesConfig{}
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
