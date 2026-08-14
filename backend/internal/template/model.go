package template

import (
	"html/template"
	"strings"
	"time"

	"blog/internal/store"
)

// ViewData 贯穿所有模板的渲染上下文，对齐 Stellar layout.ejs 的 page/theme/config 三层。
type ViewData struct {
	Site    *Site
	Page    *Page
	Content template.HTML // 文章正文 HTML（goldmark 渲染后）
	TOC     template.HTML // 文章目录（ExtractTOC 从 Content 提取，空则不渲染右栏 widget）

	// MenuPages pages 模块已发布页面自动生成的左栏菜单项（fillSidebar 填充，
	// 追加在 menubar 配置之后；访问路径与 menubar 重复的页面不重复显示）。
	MenuPages []MenuItem

	// 首页文章列表（Layout=index 时使用）。
	Posts           []*PostCard    // 文章卡片列表
	Paginator       *Paginator     // 分页器数据（nil 或 Total<=1 则不渲染）
	PinnedPosts     []*PostCard    // 置顶轮播数据（flat 模式为空）
	PinnedWiki      []*WikiProject // 文库置顶项目轮播数据
	PinSliderActive bool           // 首页第一页由轮播替代置顶文章卡片

	// 归档与分类/标签页（Layout=archive/categories/tags 时使用）。
	Archives []*ArchiveYear // 归档页年份分组
	Cats     []*CatItem     // 分类索引页
	TagItems []*TagItem     // 标签索引页

	// 侧边栏动态数据（所有前台页面渲染前由 fillSidebar 填充）。
	Recent    []*RecentPost // 近期发布 widget（最新 5 篇 published）
	TagCloud  []*TagItem    // 标签云 widget（按文章数降序前 20）
	Popular   []*RecentPost // 热门文章 widget（按阅读量降序 top 5）
	SiteStats *SiteStats    // 站点统计 widget（nil 时隐藏）

	// widgets 序列化编排（Spec 33）：按页面类型解析的左右栏实例序列，
	// 由 assembleWidgets 在 fillSidebar 后装配；sidebar/rightbar 模板 range 分派渲染。
	LeftbarWidgets  []*WidgetInstance
	RightbarWidgets []*WidgetInstance
	// CustomPageBars 仅用于 Layout=page 独立页面覆盖全局 widgets.pages.page（Spec 33+页面级自定义侧栏）。
	// nil=未配置，继承 BarsFor 的 page 类型全局默认；非 nil（即使 Leftbar/Rightbar 空）= 用该字段覆盖左右栏。
	CustomPageBars *PageBars
	// PageWidgetOverrides 页面级 widget 参数覆盖（仅 Layout=page 自定义侧栏时使用）。
	// map key = 组件名（与 CustomPageBars.Leftbar/Rightbar 序列中的名字对应）；
	// value = 页面级 Params（与全局 Library 同名组件参数浅合并，BuildBar 时生效）。
	// nil 或 key 缺失时不覆盖，走全局参数。
	PageWidgetOverrides map[string]WidgetDef

	// 评论区（Layout=post 且 comments.enabled 时填充）。
	Comments     []*CommentItem // 评论树（顶级 + Children 两层）
	CommentCount int            // 已发布评论总数（"N 条评论"计数）

	// 最新评论 widget（fillSidebar 填充，comments.enabled 时）。
	LatestComments []*LatestComment

	// 文章页上下篇导航与相关文章（对齐原版 read_next.ejs：prev=较新文章，next=较早文章）。
	PrevPost     *AdjacentPost // 较新文章（时间更晚的邻篇），nil 则不渲染
	NextPost     *AdjacentPost // 较早文章（时间更早的邻篇），nil 则不渲染
	RelatedPosts []*PostCard   // 相关文章（按标签匹配，上限 site.yml article.related_posts）

	// 文章页首尾增强（Spec 32）。
	EditPage       *EditPageLink // 「编辑本文」+贡献者段（edit_this_page 映射命中时非 nil）
	ExpiredDays    int           // 距最后更新天数，>0 时文首渲染过期提醒
	WordCount      int           // 文章正文有效字数（中文按字符统计）
	ReadingMinutes int           // 预计阅读分钟数

	// 专栏体系（对齐原版 topic）：
	TopicSeries *TopicSeries // 文章页左栏同专栏系列 widget（Page.Topic 非空时填充）
	Topics      []*TopicItem // /topic 索引页的专栏卡片列表
	TopicDetail *TopicDetail // 专栏详情页头部（Layout=topic）：名称/描述/图标/封面/文章数
	HasTopics   bool         // 是否存在专栏（navbar「专栏」tab 显示条件，对齐原版 publish_list 非空）

	// 瞬间时间流（Layout=moments 时使用）。
	Moments []*MomentItem

	// 文库（Layout=wiki 时使用）：WikiProjects 为按 tag 过滤/排序后的项目，WikiTags 为顶部 tab 全量标签。
	WikiProjects []*WikiProject
	WikiTags     []string

	// wiki 文档页（Layout=wiki-doc 时使用，Spec 35a）：
	// WikiProjectID/Name 为面包屑与 tree widget 的项目上下文；WikiDoc 为当前文档；
	// WikiPrevDoc/WikiNextDoc 为同 project 内按 doc_order 的上下篇；WikiDocTree 供 leftbar tree widget 注入。
	WikiProjectID   string // 项目 ID（面包屑/树链接）
	WikiProjectName string // 项目显示名（DisplayTitle）
	// WikiHero 当前项目 hero banner（handleWikiDoc 填充，非空时文档页页首渲染）。
	WikiHero *HeroConfig
	// WikiProjectIcon/Description 为 sidebar 项目化 header 数据（handleWikiDoc 填充，
	// 仅 wiki 文档页使用）：icon 显示项目 Logo，description 作副标题。
	WikiProjectIcon        string         // 项目 Logo 图片 URL（WikiProject.Icon）
	WikiProjectDescription string         // 项目描述（WikiProject.Description）
	WikiDoc                *WikiDocView   // 当前文档（nil 时不渲染文档区）
	WikiPrevDoc            *WikiDocNav    // 上一篇（nil 不渲染）
	WikiNextDoc            *WikiDocNav    // 下一篇（nil 不渲染）
	WikiDocTree            []WikiTreeNode // 项目文档树（仅 wiki 文档页注入，不进侧栏快照）
	// CommentPageKey 评论区 page_key（fillComments 统一设置，模板 data-page-key 优先用它；
	// 为空时 comments.html 兜底回退 /post/{slug}，文章页零回归）。
	CommentPageKey string

	// 搜索框项目化（wiki 文档页左侧栏）：SearchFilter 限定当前项目（data-filter 前缀过滤），
	// SearchPlaceholder 动态 placeholder 文案；其余页面为空走默认（全站搜索）。
	SearchFilter      string
	SearchPlaceholder string
	// WikiRecentDocs wiki 文档页"最近更新"widget 数据源（当前项目已发布文档按 UpdatedAt 降序，
	// 复用 RecentPost 结构，模板零改动）；非 wiki 文档页为 nil，走全局 Recent。
	WikiRecentDocs []*RecentPost

	// 笔记(手记)（Layout=notes 时使用）：NotesContentHTML 为 content/notes.md 渲染的手记正文；近期动态复用 Moments。
	NotesContentHTML template.HTML

	// 笔记本体系（Spec 35b）：Notebooks 为列表页卡片（Layout=notebooks）；
	// NotebookDetail/Notes/NotesTotal/NotesPage/NotesPerPage 为笔记本详情页（Layout=notebook）的当前笔记本与分页笔记列表；
	// NoteDetail 为笔记详情（Layout=note）；NotebookTags 为当前笔记本标签聚合、
	// TagTreeData 供 leftbar tagtree widget 注入（数据仅本页有，不进侧栏快照）。
	Notebooks      []NotebookView
	NotebookDetail *store.Notebook
	Notes          []store.Note
	NoteDetail     *NoteView
	NotesTotal     int
	NotesPage      int
	NotesPerPage   int
	NotebookTags   []TagCount
	TagTreeData    *TagTreeWidgetData

	// 作者页（Layout=author 时使用）：AuthorProfile 为当前作者定义，Posts/Paginator 复用首页字段。
	AuthorProfile *AuthorDef

	// Admin 后台专用字段。前台模板不读这些字段，零值即可。
	Admin *AdminData
}

// AdjacentPost 文章页上下篇导航数据。
type AdjacentPost struct {
	Slug  string
	Title string
}

// ArchiveYear 归档页按年份分组的文章列表，对齐 Stellar archive.ejs。
type ArchiveYear struct {
	Year  string
	Posts []*ArchiveItem
}

// ArchiveItem 归档列表单项（time(MM-DD) + 标题 + 链接）。
type ArchiveItem struct {
	Slug  string
	Title string
	Date  time.Time
}

// CatItem 分类索引页单项（名称 + 文章数 badge）。
type CatItem struct {
	Name  string
	Count int
	Icon  string // 图标 key（article.category_icon 命中值，空则模板回退 default:category）
}

// TagItem 标签索引页单项。
type TagItem struct {
	Name  string
	Count int
	Icon  string // 图标 key（article.tag_icon 命中值，空不渲染图标）
	Size  int    // 标签云字号（px），0 表示默认
	Color string // 标签云渐变色（tagcloud color 参数开启时插值生成），空不输出 color style
	Dy    int    // 标签云垂直散落偏移 px（标签名哈希确定性生成，模板输出 --tc-dy）
	Level string // 标签云热度层级（xl/lg/md/sm/xs，按入选排名百分比分档，tagcloud.design 移植）
}

// PostCard 首页文章卡片数据，对齐 Stellar _partial/main/post_list/post_card.ejs。
type PostCard struct {
	Slug          string
	Title         string
	Excerpt       string
	Cover         string       // 封面图 URL，空则不渲染 .post-cover
	Date          time.Time    // 用于 <time> 标签与格式化
	Pinned        bool         // 置顶（meta 行 pin 图标，对齐 post_card.ejs 的 span.pin）
	Category      string       // 单分类（meta 行分类面包屑，空不渲染）
	CategoryStyle template.CSS // 分类配色 style 内容（CategoryColorStyle 生成；CSS 类型绕过 html/template 的 ZgotmplZ 过滤）
	CategoryIcon  string       // 分类图标 key（CategoryIconKey 解析；未配置回退 default:category）
	Poster        *PosterCard  // 海报数据（非 nil 且有 Cover 时渲染 photo 封面大卡）
	Tags          []string     // 文章卡片标签（模板最多展示 5 个）
}

// NewPostCard store.Post 转文章卡片视图模型，首页/分类标签列表/相关文章共用。
// 分类配色查 site.yml article.category_color（对齐原版 category_color helper）；
// 海报 DecodePoster 非 nil 时，与 Cover 一起触发首页 photo 封面大卡。
func NewPostCard(p *store.Post, categoryColors, categoryIcons map[string]string) *PostCard {
	card := &PostCard{
		Slug:          p.Slug,
		Title:         p.Title,
		Excerpt:       p.Excerpt,
		Cover:         p.Cover,
		Date:          p.CreatedAt,
		Pinned:        p.Pinned,
		Category:      p.Category,
		CategoryStyle: CategoryColorStyle(p.Category, categoryColors),
		CategoryIcon:  CategoryIconKey(p.Category, categoryIcons),
		Tags:          ParseTags(p.Tags),
	}
	if pd := store.DecodePoster(p.Poster); pd != nil {
		card.Poster = &PosterCard{Headline: pd.Headline, Topic: pd.Topic, Caption: pd.Caption, Color: pd.Color}
	}
	return card
}

// ParseTags 拆分文章 tags 的逗号存储格式，兼容中英文逗号并去除空白。
func ParseTags(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '，' })
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}

// PosterCard 海报封面数据，对齐原版 post.poster（headline/topic/caption/color）。
// 外层 a.post-card.photo > div.cover > div.cover-info，样式在 main.css 已有。
type PosterCard struct {
	Headline string // 海报主标题（photo 卡触发字段，空则无海报）
	Topic    string // 专栏/系列名（非空时 cover-info 吸顶 position=top，否则沉底 bottom）
	Caption  string // 副标题
	Color    string // 文字颜色（如 #fff），空用默认白
}

// CategoryColorStyle 对齐原版 scripts/helpers/category_color.js：
// 返回 style 属性内容（--text-p2:色值;--theme-block:同色系透明底色），未配置返回空。
// 色值补 # 前缀；#fff（3 位）底色后缀 2、#ffffff（6 位）后缀 20，与原版一致。
// 返回 template.CSS：style 属性上下文中普通字符串会被 html/template 过滤为 ZgotmplZ。
func CategoryColorStyle(cat string, colors map[string]string) template.CSS {
	if cat == "" || len(colors) == 0 {
		return ""
	}
	color := colors[cat]
	if color == "" {
		return ""
	}
	textColor := color
	if !strings.HasPrefix(textColor, "#") {
		textColor = "#" + textColor
	}
	bgColor := textColor
	if len(textColor) == 4 { // #fff
		bgColor += "2"
	} else if len(textColor) == 7 { // #ffffff
		bgColor += "20"
	}
	return template.CSS("--text-p2:" + textColor + ";--theme-block:" + bgColor)
}

// CategoryIconKey 解析分类图标 key：命中 article.category_icon 配置则返回配置值
// （icons.yml key 或图片 URL，icon 模板函数两者都支持），未命中回退 default:category（与改造前硬编码一致）。
func CategoryIconKey(cat string, icons map[string]string) string {
	if cat != "" {
		if v := icons[cat]; v != "" {
			return v
		}
	}
	return "default:category"
}

// Paginator 分页器数据，对齐 Stellar _partial/main/post_list/paginator.ejs。
type Paginator struct {
	Current int
	Total   int   // 总页数
	Pages   []int // 页码序列 [1..Total]
}

// AdminData 后台模板渲染数据。
type AdminData struct {
	Posts     []*AdminPostRow // 列表页
	EditPost  *AdminPostRow   // 编辑页（新建时为空对象）
	SaveError string          // 保存失败提示
	IsEdit    bool            // true=编辑现有，false=新建

	// 评论管理页（/admin/comments）。
	Comments      []*AdminCommentRow // 评论列表
	CommentFilter string             // 当前筛选：all / pending / approved
	CommentPage   int                // 当前页码（从 1 开始）
	CommentPages  []int              // 页码序列（分页器用）

	// 登录页（/admin/login）。
	LoginError string // 登录失败提示（密码错误）
	LoginFrom  string // 登录成功后的回跳地址（authRequired 跳转时带入的原始路径）

	// 仪表盘（/admin/dashboard）。
	Dashboard *AdminDashboard
}

// AdminCommentRow 评论管理页列表行数据。
type AdminCommentRow struct {
	ID        int64
	PageKey   string // 所在页面（文章链接）
	Nick      string
	Email     string
	Excerpt   string // 内容纯文本摘要（截断展示）
	Status    string // approved / pending
	IsAdmin   bool
	Avatar    string // Gravatar URL
	Date      string // 列表展示用
	ReplyNick string // 被回复者昵称（子评论时）
}

// AdminPostRow 后台列表/编辑表单用的文章行数据。
type AdminPostRow struct {
	Slug      string
	Title     string
	Excerpt   string
	Cover     string // 封面图 URL（编辑表单回显用）
	Category  string // 分类（编辑表单回显用）
	Topic     string // 专栏（编辑表单回显用）
	Tags      string // 标签，逗号分隔（编辑表单回显用）
	ContentMD string
	Status    string
	Date      string // 列表展示用（created_at 或文件 date）
	Pinned    bool
	Password  string
	PublishAt string // 表单用 datetime-local / RFC3339 文本
	// 海报封面（对齐原版 post.poster，编辑表单回显/保存用）
	PosterHeadline string
	PosterTopic    string
	PosterCaption  string
	PosterColor    string
}

// Site 对齐 Stellar theme + config 的站点级配置。
// yaml tag 供 config/site.yml 加载（goccy/go-yaml），snake_case 精确映射。
type Site struct {
	Title       string          `yaml:"title" json:"title"`
	Subtitle    string          `yaml:"subtitle" json:"subtitle"`
	Description string          `yaml:"description" json:"description"`
	URL         string          `yaml:"url" json:"url"`             // 站点基础 URL（如 https://example.com），用于 canonical/OG/sitemap
	Avatar      string          `yaml:"avatar" json:"avatar"`       // logo 头像 URL
	AvatarBG    string          `yaml:"avatar_bg" json:"avatar_bg"` // 头像 hover 旋转背景图，对应原版 style.animated_avatar.background
	LogoIcon    string          `yaml:"logo_icon" json:"logo_icon"` // logo icon 图片 URL（对齐上游 logo.icon，优先于 avatar）
	LogoTitle   string          `yaml:"logo_title" json:"logo_title"`
	LogoSub     string          `yaml:"logo_sub" json:"logo_sub"`
	FaviconURL  string          `yaml:"favicon_url" json:"favicon_url"`           // Favicon URL，留空回退 /favicon.ico
	AppleIcon   string          `yaml:"apple_touch_icon" json:"apple_touch_icon"` // Apple Touch Icon URL，留空回退 /apple-touch-icon.png
	Menubar     Menubar         `yaml:"menubar" json:"menubar"`
	Rightbar    RightbarConfig  `yaml:"rightbar" json:"rightbar"`
	Footer      Footer          `yaml:"footer" json:"footer"`
	Comments    CommentsConfig  `yaml:"comments" json:"comments"`
	Admin       AdminConfig     `yaml:"admin" json:"-"`
	Links       []LinkGroup     `yaml:"links" json:"links"`     // 友链分组（site.yml links 段）
	Friends     FriendsConfig   `yaml:"friends" json:"friends"` // 友链动态聚合配置（site.yml friends 段）
	Wiki        []WikiProject   `yaml:"wiki" json:"wiki"`       // 文库项目（site.yml wiki 段，/wiki 页）
	Article     ArticleConfig   `yaml:"article" json:"article"`
	SEO         SEOConfig       `yaml:"seo" json:"seo"`               // 站点级 SEO 配置（Spec 42）
	Inject      InjectConfig    `yaml:"-" json:"inject"`              // 代码注入（Spec 42）：仅入库不入 site.yml（技术决策）
	Style       StyleConfig     `yaml:"style"   json:"style"`         // 外观样式可配置化（Spec 34/47）
	Widgets     WidgetsConfig   `yaml:"widgets" json:"widgets"`       // 侧边栏组件配置化编排（Spec 33/47）
	DataCache   DataCacheConfig `yaml:"data_cache" json:"data_cache"` // 动态数据客户端缓存（1.36 data_cache）
	SiteTree    SiteTreeConfig  `yaml:"site_tree" json:"site_tree"`   // 页面树配置（1.38 site_tree.home.comments 等）
	// NavTabs 导航栏标签页配置（Spec 50）。
	// EnableCustom 为 true 时完全由配置驱动，false 时保持当前固定 tab + 可配额外项。
	NavTabs NavTabsConfig `yaml:"nav_tabs,omitempty" json:"nav_tabs,omitempty"`
	// 首页 Hero cover 区配置（site.yml cover 段），仅首页第一页且 title 非空时渲染。
	Cover CoverConfig `yaml:"cover,omitempty" json:"cover,omitempty"`
	// 多作者体系（Spec 38）：site.yml authors 段，key 为作者 ID。
	Authors map[string]AuthorDef `yaml:"authors,omitempty" json:"authors,omitempty"`
	// 前端体验增强插件（Spec 36）：site.yml plugins 段。缺省全关，opt-in 启用；
	// 各插件 Enable 为 false 时前端不加载对应脚本。
	Plugins PluginsConfig `yaml:"plugins,omitempty" json:"plugins,omitempty"`
	// TagPlugins 标签插件默认参数（对齐原版 _config.yml tag_plugins 段）。
	TagPlugins TagPluginsConfig `yaml:"tag_plugins" json:"tag_plugins"`
	// Default 占位素材组（对齐原版 _config.yml default 段）。
	Default DefaultAssets `yaml:"default" json:"default"`
	// Pages 页面级设置（site.yml / settings 表 pages 段）：按页面键管理，当前仅 friends。
	Pages PagesConfig `yaml:"pages,omitempty" json:"pages,omitempty"`
	// SWCacheVersion Service Worker 缓存版本（settings site 段 sw_cache_version，仅入库不入 site.yml）：
	// 非空时 /sw.js 以此注入 CACHE_VERSION，缓存策略变更后 bump 使旧 SW 失效。
	SWCacheVersion string `yaml:"-" json:"-"`
}

// DefaultAssets 对齐原版 _config.yml default 段：各场景占位素材 URL，空则回退内置硬编码值。
type DefaultAssets struct {
	Avatar       string `yaml:"avatar" json:"avatar"`
	Link         string `yaml:"link" json:"link"`
	Cover        string `yaml:"cover" json:"cover"`
	Image        string `yaml:"image" json:"image"`
	Project      string `yaml:"project" json:"project"`
	Banner       string `yaml:"banner" json:"banner"`
	Topic        string `yaml:"topic" json:"topic"`
	Loading      string `yaml:"loading" json:"loading"`
	ImageOnError string `yaml:"image_onerror" json:"image_onerror"`
}

// DefaultAuthorName 返回站点默认作者名：存在多作者配置时取第一个，否则回退站点标题。
func (s *Site) DefaultAuthorName() string {
	for _, a := range s.Authors {
		return a.Name
	}
	return s.Title
}

// PluginsConfig 前端体验增强插件配置（Spec 36）。缺省全关，opt-in 启用。
// 各子配置均为指针：nil 表示未配置该插件（缺省关闭），非 nil 时按字段生效。
type PluginsConfig struct {
	Preload      *PreloadConfig      `yaml:"preload,omitempty" json:"preload,omitempty"`
	ScrollReveal *ScrollRevealConfig `yaml:"scrollreveal,omitempty" json:"scrollreveal,omitempty"`
	Heti         *HetiConfig         `yaml:"heti,omitempty" json:"heti,omitempty"`
	Fancybox     *FancyboxConfig     `yaml:"fancybox,omitempty" json:"fancybox,omitempty"`
}

// PreloadConfig 页面资源预加载（如 Flying Pages）。Service 为第三方库名，空用内置缺省。
type PreloadConfig struct {
	Enable  bool   `yaml:"enable" json:"enable"`
	Service string `yaml:"service,omitempty" json:"service,omitempty"`
}

// ScrollRevealConfig 滚动渐现动画配置。
type ScrollRevealConfig struct {
	Enable   bool   `yaml:"enable" json:"enable"`
	Distance string `yaml:"distance,omitempty" json:"distance,omitempty"`
	Duration int    `yaml:"duration,omitempty" json:"duration,omitempty"`
	Interval int    `yaml:"interval,omitempty" json:"interval,omitempty"`
}

// HetiConfig 中文排版优化（赫蹏）。
type HetiConfig struct {
	Enable bool `yaml:"enable" json:"enable"`
}

// FancyboxConfig 图片灯箱配置。Selector 为空用内置缺省选择器。
type FancyboxConfig struct {
	Selector string `yaml:"selector,omitempty" json:"selector,omitempty"`
}

// DataCacheConfig 动态数据客户端缓存（1.36 data_cache）。Enable 缺省 true；DefaultTtl 秒。
type DataCacheConfig struct {
	Enable     *bool          `yaml:"enable,omitempty" json:"enable,omitempty"`
	DefaultTTL int            `yaml:"default_ttl,omitempty" json:"default_ttl,omitempty"`
	TTL        map[string]int `yaml:"ttl,omitempty" json:"ttl,omitempty"`
}

// SiteTreeConfig 页面树配置（1.38）。Home.Comments 首页可配置评论区（默认关）。
type SiteTreeConfig struct {
	Home SiteTreeHome `yaml:"home,omitempty" json:"home,omitempty"`
}

// SiteTreeHome 首页配置。
type SiteTreeHome struct {
	Comments bool `yaml:"comments,omitempty" json:"comments,omitempty"`
}

// AuthorDef 描述一位作者（来自 site.yml authors 段）。
type AuthorDef struct {
	Name        string `yaml:"name" json:"name"`
	Avatar      string `yaml:"avatar,omitempty" json:"avatar,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	URL         string `yaml:"url,omitempty" json:"url,omitempty"`
	Hidden      bool   `yaml:"hidden,omitempty" json:"hidden,omitempty"`
}

// NavTabsConfig 导航栏标签页配置（Spec 50）。
// EnableCustom 为 true 时完全由配置驱动，false 时保持当前固定 tab + 可配额外项（向后兼容）。
type NavTabsConfig struct {
	EnableCustom bool         `yaml:"enable_custom,omitempty" json:"enable_custom,omitempty"`
	Items        []NavTabItem `yaml:"items,omitempty" json:"items,omitempty"`
}

// NavTabItem 导航栏标签页单项（site.yml nav_tabs 段）。
// Spec 50 增强：Icon 为图标 key（可选），ActiveRules 为路径匹配规则（可选，缺省按 URL 精确匹配）。
type NavTabItem struct {
	Title       string `yaml:"title" json:"title"`
	URL         string `yaml:"url" json:"url"`
	External    bool   `yaml:"external,omitempty" json:"external,omitempty"`         // true 时 target="_blank"
	Icon        string `yaml:"icon,omitempty" json:"icon,omitempty"`                 // 图标 key（可选），如 solar:home-bold
	ActiveRules string `yaml:"active_rules,omitempty" json:"active_rules,omitempty"` // 路径匹配正则（可选），缺省按 URL 精确匹配
}

// CoverConfig 首页 Hero cover 区配置（site.yml cover 段）。
// 对齐 Stellar wiki_cover.ejs 的 HTML 结构与 class 名。
type CoverConfig struct {
	Title       string         `yaml:"title" json:"title"`
	Description string         `yaml:"description,omitempty" json:"description,omitempty"`
	Scheme      string         `yaml:"scheme,omitempty" json:"scheme,omitempty"`
	Features    []CoverFeature `yaml:"features,omitempty" json:"features,omitempty"`
}

// CoverFeature cover 区功能按钮单项。
type CoverFeature struct {
	Title string `yaml:"title" json:"title"`
	URL   string `yaml:"url" json:"url"`
	Icon  string `yaml:"icon,omitempty" json:"icon,omitempty"`
}

// SEOConfig 站点级 SEO 配置（site.yml seo 段 / settings 表 seo 段，Spec 42）。
type SEOConfig struct {
	DefaultDescription string `yaml:"default_description" json:"default_description"` // 默认 meta description（文章级未配置时的回退值）
	DefaultKeywords    string `yaml:"default_keywords" json:"default_keywords"`       // 默认 meta keywords，逗号分隔
	RobotsExtra        string `yaml:"robots_extra" json:"robots_extra"`               // robots.txt 附加规则（多行原样追加）
	NoIndex            bool   `yaml:"no_index" json:"no_index"`                       // true=全站输出 noindex meta（零值=允许收录，旧库升级安全）
	GoogleVerification string `yaml:"google_verification" json:"google_verification"` // google-site-verification 验证码
	BingVerification   string `yaml:"bing_verification" json:"bing_verification"`     // msvalidate.01 验证码
	BaiduVerification  string `yaml:"baidu_verification" json:"baidu_verification"`   // baidu-site-verification 验证码
}

// InjectConfig 代码注入配置（仅 settings 表 inject 段，Spec 42）。
// template.HTML 类型直接原样输出不转义（注入本意即执行脚本，不经 bluemonday）。
type InjectConfig struct {
	Head   template.HTML `json:"inject_head"`   // </head> 前原样输出
	Footer template.HTML `json:"inject_footer"` // </body> 前原样输出
}

// AdminConfig 管理后台配置（site.yml admin 段）。
// Password 仅作启动迁移的初始输入（Spec 61 阶段 4 起密码唯一真值在 users 表 bcrypt 哈希）；
// 运行时密码门禁一律看 AuthEnabled（RebuildSite 时从 users 表 admin 用户派生）。
type AdminConfig struct {
	Password    string `yaml:"password" json:"password"` // 启动迁移用初始密码；迁移后不再参与鉴权
	AuthEnabled bool   `yaml:"-" json:"-"`               // 运行时派生：users 表 admin 用户有密码哈希时为 true
}

// CommentsConfig 评论系统配置（site.yml comments 段）。
type CommentsConfig struct {
	Enabled        bool         `yaml:"enabled" json:"enabled"`                 // 总开关：false 时文章页不渲染评论区、API 关闭
	Title          string       `yaml:"title" json:"title"`                     // 评论区标题
	Moderation     bool         `yaml:"moderation" json:"moderation"`           // true=先审后发（新评论 pending）
	AdminEmail     string       `yaml:"admin_email" json:"admin_email"`         // 博主邮箱：新评论通知收件人 + 博主身份匹配
	AdminBadge     string       `yaml:"admin_badge" json:"admin_badge"`         // 博主徽章文案
	GravatarMirror string       `yaml:"gravatar_mirror" json:"gravatar_mirror"` // Gravatar 镜像基址（头像 = {mirror}/{md5(email)}）
	Notify         NotifyConfig `yaml:"notify" json:"notify"`                   // 邮件通知；smtp_host 为空则整体关闭
}

// NotifyConfig 邮件通知配置（net/smtp + PlainAuth）。
type NotifyConfig struct {
	SMTPHost string `yaml:"smtp_host" json:"smtp_host"`
	SMTPPort int    `yaml:"smtp_port" json:"smtp_port"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
	From     string `yaml:"from" json:"from"` // 发件人地址，空则用 username
}

// Menubar 对齐 _config.yml menubar.items。
type Menubar struct {
	Columns int        `yaml:"columns" json:"columns"`
	Items   []MenuItem `yaml:"items" json:"items"`
}

// MenuItem 对齐 menubar 单项。Theme 是 inline color，控制 SVG 的 currentColor。
type MenuItem struct {
	ID    string `yaml:"id" json:"id"`
	Title string `yaml:"title" json:"title"`
	URL   string `yaml:"url" json:"url"`
	Icon  string `yaml:"icon" json:"icon"`   // iconify key，如 solar:documents-bold-duotone
	Theme string `yaml:"theme" json:"theme"` // 高亮色，如 #1BCDFC
}

// RightbarConfig 右栏配置（site.yml rightbar 段）。
type RightbarConfig struct {
	// WelcomeLinklist 欢迎语 markdown widget 内的链接列表，对齐原版 markdown.ejs 的 item.linklist。
	WelcomeLinklist LinkListConfig `yaml:"welcome_linklist" json:"welcome_linklist"`
	// Intro 欢迎语正文（Spec Task 5），留空时模板显示默认文案。纯文本，换行由模板 pre-wrap 保留。
	Intro string `yaml:"intro" json:"intro"`
}

// LinkListConfig 链接列表（对齐原版 widgets/components/linklist.ejs）。
type LinkListConfig struct {
	Columns int            `yaml:"columns" json:"columns"` // 列数，0=按链接数
	Items   []LinkListItem `yaml:"items" json:"items"`
}

// LinkListItem 链接列表单项（icon 为 icons.yml key，可空）。
// 命名区别于友链 model_links.go 的 LinkItem（Name/Avatar/Desc 字段组不同）。
type LinkListItem struct {
	Title string `yaml:"title" json:"title"`
	URL   string `yaml:"url" json:"url"`
	Icon  string `yaml:"icon" json:"icon"`
}

// EffectiveColumns 对齐原版 columns || links.length。
func (l LinkListConfig) EffectiveColumns() int {
	if l.Columns > 0 {
		return l.Columns
	}
	return len(l.Items)
}

// AlignClass 对齐原版：columns==1 且链接数>1 时左对齐（left），否则居中（center）。
func (l LinkListConfig) AlignClass() string {
	if l.EffectiveColumns() == 1 && len(l.Items) > 1 {
		return "left"
	}
	return "center"
}

// ShowText 对齐原版 link.ejs：columns<=2 时才显示文字（否则纯图标）。
func (l LinkListConfig) ShowText() bool {
	return l.EffectiveColumns() <= 2
}

// Footer 对齐 theme.footer（social + sitemap）。
type Footer struct {
	Social  []SocialItem   `yaml:"social" json:"social"`
	Sitemap []SitemapGroup `yaml:"sitemap" json:"sitemap"` // 页脚站点地图多列（对齐原版 footer.ejs sitemap）
	Beian   string         `yaml:"beian" json:"beian"`     // ICP 备案号（大陆部署合规）；空则页脚不渲染
	// Content 页脚自定义文案（Markdown，对齐原版 footer.content）；空则回退内置 Powered-by 行。
	Content string `yaml:"content" json:"content"`
}

// SitemapGroup 页脚站点地图分组（组标题 + 链接列表），对齐 theme.footer.sitemap。
type SitemapGroup struct {
	Title string        `yaml:"title" json:"title"`
	Items []SitemapItem `yaml:"items" json:"items"`
}

// SitemapItem 页脚站点地图链接项。
type SitemapItem struct {
	Title string `yaml:"title" json:"title"`
	URL   string `yaml:"url" json:"url"`
}

// SitemapColumns 计算 sitemap 栏数，对齐原版 footer.ejs：6 组→3 栏，7-8 组→4 栏，其余 min(len,5)。
func (f *Footer) SitemapColumns() int {
	n := len(f.Sitemap)
	switch {
	case n == 6:
		return 3
	case n == 7 || n == 8:
		return 4
	case n > 5:
		return 5
	default:
		return n
	}
}

// SocialItem 侧边栏底部社交图标。
// IconKey 非空时走 icon helper（内联 SVG），否则用 Icon 原样输出（如 <img src>）。
type SocialItem struct {
	Title   string `yaml:"title" json:"title"`
	URL     string `yaml:"url" json:"url"`
	Icon    string `yaml:"icon" json:"icon"`         // raw HTML，如 <img src="...">
	IconKey string `yaml:"icon_key" json:"icon_key"` // icons.yml 的 key，如 solar:xxx
}

// RecentPost 近期发布 widget 的一项。
type RecentPost struct {
	Title string `yaml:"title"`
	URL   string `yaml:"url"`
}

// CommentItem 前台评论区单条评论视图模型（含两层嵌套的子评论）。
// json tag 供 /api/comments 响应序列化（前端无刷新插入用）。
type CommentItem struct {
	ID          int64          `json:"id"`
	Nick        string         `json:"nick"`
	Link        string         `json:"link"`         // 个人网站，非空时昵称带链
	ContentHTML template.HTML  `json:"content_html"` // 已消毒的服务端渲染 HTML，直接输出
	IsAdmin     bool           `json:"is_admin"`
	Verified    bool           `json:"verified"`            // 注册用户标识：已登录注册用户显示"已验证"徽章
	Avatar      string         `json:"avatar"`              // Gravatar URL（{mirror}/{md5(email)}）
	TimeAgo     string         `json:"time_ago"`            // 相对时间，如"3 天前"
	Date        string         `json:"date"`                // 完整日期（title 悬浮提示）
	Likes       int64          `json:"likes"`               // 点赞数（IP 去重）
	ReplyRef    string         `json:"reply_ref,omitempty"` // 回复引用，格式 "▸ 被回复人昵称"（仅子评论）
	Children    []*CommentItem `json:"children,omitempty"`
}

// LatestComment 侧边栏"最新评论"widget 单项。
type LatestComment struct {
	Nick    string
	Excerpt template.HTML // 保留表情小图的摘要 HTML（comment.ExcerptHTML 白名单重建）
	URL     string        // page_key + #comment-{id} 锚点
	TimeAgo string
}

// TopicSeries 文章页左栏同专栏系列文章列表。
type TopicSeries struct {
	Name  string
	Posts []*TopicSeriesPost
}

// TopicSeriesPost 系列列表单项（Active=当前文章，渲染 active class + bookmark 图标）。
type TopicSeriesPost struct {
	Slug   string
	Title  string
	Active bool
}

// TopicItem /topic 索引页专栏卡片（名称 + 文章数 + 图标）。
type TopicItem struct {
	Name  string
	Count int
	Icon  string // 卡片图标（缺省用主题 default.topic 占位图）
}

// TopicDetail 专栏详情页头部数据（/topic/:slug）：有封面时沉浸 banner，无封面时紧凑卡片。
type TopicDetail struct {
	Name        string
	Description string
	Icon        string
	Cover       string
	Count       int
	Hue         int    // 无图标时渐变首字头像的色相（名称哈希 0-359）
	Initial     string // 首字头像文字（名称首 rune）
}

// Page 对齐 Hexo page 对象，文章页用。
type Page struct {
	Title        string
	Slug         string
	Layout       string // post / page / index / archive / categories / tags
	Type         string // tech / story
	Lang         string
	MenuID       string // 当前高亮的菜单 id
	Author       string
	AuthorName   string // 多作者命中 Site.Authors 时的展示名（Spec 53：banner 作者链接用）
	Date         string // 原始日期字符串
	Updated      string
	Excerpt      string
	Views        int64    // 文章阅读量（仅文章页 Layout=post 使用，已含本次访问）
	Category     string   // 分类文章列表页的当前分类名（navbar 显示"分类：xxx"）
	Tag          string   // 标签文章列表页的当前标签名
	Topic        string   // 专栏名：文章页（面包屑/左栏系列卡）与专栏列表页（navbar 显示"专栏：xxx"）
	CanonicalURL string   // 当前页面完整 URL（用于 canonical 和 og:url）
	Description  string   // 页面描述（用于 og:description，文章页用 Excerpt，非文章页用 Site.Description）
	References   []string // 参考文献列表（对齐原版 article_footer.ejs page.references）
	Banner       string   // 文首横幅图 URL（front matter banner，对齐原版 article_banner.ejs banner.url）
	Subtitle     string   // banner 副标题（对齐原版 banner .text.subtitle；友链页取 pages.friends.hero.description）

	// 文章级 SEO（Spec 42）：非空时优先于站点级默认值输出 meta。
	MetaDescription string // 自定义 meta description（覆盖自动摘要）
	MetaKeywords    string // 自定义 meta keywords，逗号分隔

	// AI 文章摘要（Spec 53）：AISummary 为后台预生成的导读摘要（空=无）；
	// AISummaryOn/AIUserAPIOn 来自 settings ai 段开关，控制前台摘要块与自助面板渲染。
	AISummary   string
	AISummaryOn bool
	AIUserAPIOn bool
}

// MetaDescription 计算 head.html 的 meta description 输出（Spec 42 优先级）：
// 文章级 Page.MetaDescription → 站点级 seo.default_description → 现状回退（Page.Excerpt → Site.Description）。
func (d *ViewData) MetaDescription() string {
	if d.Page != nil && d.Page.MetaDescription != "" {
		return d.Page.MetaDescription
	}
	if d.Site != nil && d.Site.SEO.DefaultDescription != "" {
		return d.Site.SEO.DefaultDescription
	}
	if d.Page != nil && d.Page.Excerpt != "" {
		return d.Page.Excerpt
	}
	if d.Site != nil {
		return d.Site.Description
	}
	return ""
}

// MetaKeywords 计算 meta keywords 输出：文章级 → 站点级默认；均空则不输出该 meta。
func (d *ViewData) MetaKeywords() string {
	if d.Page != nil && d.Page.MetaKeywords != "" {
		return d.Page.MetaKeywords
	}
	if d.Site != nil {
		return d.Site.SEO.DefaultKeywords
	}
	return ""
}
