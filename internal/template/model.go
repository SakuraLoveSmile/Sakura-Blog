package template

import (
	"html/template"
	"time"
)

// ViewData 贯穿所有模板的渲染上下文，对齐 Stellar layout.ejs 的 page/theme/config 三层。
type ViewData struct {
	Site    *Site
	Page    *Page
	Content template.HTML // 文章正文 HTML（goldmark 渲染后）

	// Posts 首页/单标签/单分类文章卡片列表（Layout=="index" 或 "list" 时渲染）。
	// 文章页不读此字段，零值即可。
	Posts []*PostCard

	// Paginator 分页器数据。nil 或 Total<=1 时不渲染。
	Paginator *Paginator

	// NavTabs 共享 navbar 的 tab 状态。nil 时不渲染 navbar（如文章详情页）。
	NavTabs *NavTabs

	// Tags 标签云（Layout=="tags" 时渲染）。
	Tags []TagItem
	// Categories 分类云（Layout=="categories" 时渲染）。
	Categories []CategoryItem
	// Archive 按年归档（Layout=="archive" 时渲染）。
	Archive []ArchiveGroup

	// TOCHTML 文章详情页右侧目录的嵌套 HTML（Layout=="post" 且文章有 H2/H3 时填充）。
	// 空值不渲染 toc widget。结构对齐 Stellar toc helper：<ol class="toc"><li class="toc-item">
	// <a class="toc-link">...</a><ol class="toc-child">...</ol></li></ol>，H3 缩进靠 toc-child 嵌套。
	TOCHTML template.HTML

	// SearchQuery 站内搜索页的查询词（Layout=="search" 时回显到搜索框与标题）。
	SearchQuery string

	// Admin 后台专用字段。前台模板不读这些字段，零值即可。
	Admin *AdminData
}

// PostCard 首页文章卡片视图模型，对齐 Stellar post_card.ejs 的默认卡片结构。
type PostCard struct {
	Slug    string
	Title   string
	Excerpt string
	Cover   string // 封面图 URL，空则不渲染 .post-cover
	Date    time.Time
}

// Paginator 分页器数据，对齐 Stellar paginator.ejs（Hexo 内置 paginator helper）。
type Paginator struct {
	Current  int   // 当前页码（从 1 开始）
	Total    int   // 总页数
	Pages    []int // 页码序列，如 [1,2,3]
	BaseURL  string // 分页 URL 前缀：首页留空走 /page/N；列表页如 "/tags/Go" 走 /tags/Go?page=N
}

// TagItem 标签云页的单个标签。
type TagItem struct {
	Name  string
	Count int
}

// CategoryItem 分类云页的单个分类。本期平铺无父子层级。
type CategoryItem struct {
	Name  string
	Count int
}

// ArchiveGroup 归档页的年份分组，Posts 按 created_at 倒序。
type ArchiveGroup struct {
	Year  int
	Posts []*PostCard
}

// NavTabs 共享 navbar 的 tab 状态，对齐 Stellar nav_tabs_blog.ejs。
// ShowTags/ShowCategories/ShowArchive 控制对应 tab 是否显示（无标签则不显示标签 tab）。
// Active 指明哪个 tab 高亮："home"/"tags"/"categories"/"archive"/"search"/"tag:name"/"category:name"。
// TagName/CategoryName 非空时对应 tab 文本变成 "标签:Name" 形式并 active。
type NavTabs struct {
	ShowTags       bool
	ShowCategories bool
	ShowArchive    bool
	Active         string // home / tags / categories / archive / search
	TagName        string // 非空 -> 标签 tab 显示 "标签:TagName" 且 active
	CategoryName   string // 非空 -> 分类 tab 显示 "分类:CategoryName" 且 active
}

// AdminData 后台模板渲染数据。
type AdminData struct {
	Posts     []*AdminPostRow // 列表页
	EditPost  *AdminPostRow   // 编辑页（新建时为空对象）
	SaveError string          // 保存失败提示
	IsEdit    bool            // true=编辑现有，false=新建
}

// AdminPostRow 后台列表/编辑表单用的文章行数据。
type AdminPostRow struct {
	Slug       string
	Title      string
	Excerpt    string
	Cover      string
	ContentMD  string
	Status     string
	Date       string // 列表展示用（created_at 或文件 date）
	Tags       []string
	Categories []string
}

// Site 对齐 Stellar theme + config 的站点级配置。
// yaml tag 用于从 config/site.yml 解析（goccy/go-yaml）。
type Site struct {
	Title       string       `yaml:"title"`
	Subtitle    string       `yaml:"subtitle"`
	Description string       `yaml:"description"`
	Avatar      string       `yaml:"avatar"` // logo 头像 URL
	LogoTitle   string       `yaml:"logo_title"`
	LogoSub     string       `yaml:"logo_sub"`
	URL         string       `yaml:"url"`   // 站点根 URL，如 https://example.com（RSS/sitemap 构建绝对链接）
	Author      string       `yaml:"author"` // 作者名，RSS feed 的 <author><name>
	Menubar     Menubar      `yaml:"menubar"`
	Footer      Footer       `yaml:"footer"`
	RecentPosts []RecentPost `yaml:"recent_posts"`
	Tags        []string     `yaml:"tags"`
}

// Menubar 对齐 _config.yml menubar.items。
type Menubar struct {
	Columns int        `yaml:"columns"`
	Items   []MenuItem `yaml:"items"`
}

// MenuItem 对齐 menubar 单项。Theme 是 inline color，控制 SVG 的 currentColor。
type MenuItem struct {
	ID    string `yaml:"id"`
	Title string `yaml:"title"`
	URL   string `yaml:"url"`
	Icon  string `yaml:"icon"`  // iconify key，如 solar:documents-bold-duotone
	Theme string `yaml:"theme"` // 高亮色，如 #1BCDFC
}

// Footer 对齐 theme.footer.social。
type Footer struct {
	Social []SocialItem `yaml:"social"`
}

// SocialItem 侧边栏底部社交图标。
// IconKey 非空时走 icon helper（内联 SVG），否则用 Icon 原样输出（如 <img src>）。
type SocialItem struct {
	Title   string `yaml:"title"`
	URL     string `yaml:"url"`
	Icon    string `yaml:"icon"`     // raw HTML，如 <img src="...">
	IconKey string `yaml:"icon_key"` // icons.yml 的 key，如 solar:xxx
}

// RecentPost 近期发布 widget 的一项。
type RecentPost struct {
	Title string `yaml:"title"`
	URL   string `yaml:"url"`
}

// Page 对齐 Hexo page 对象，文章页用。
type Page struct {
	Title   string
	Slug    string
	Layout  string // post / page / index
	Type    string // tech / story
	Lang    string
	MenuID  string // 当前高亮的菜单 id
	Author  string
	Date    string // 原始日期字符串
	Updated string
	Excerpt string
}
