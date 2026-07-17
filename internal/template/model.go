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

	// 首页文章列表（Layout=index 时使用）。
	Posts     []*PostCard // 文章卡片列表
	Paginator *Paginator  // 分页器数据（nil 或 Total<=1 则不渲染）

	// Admin 后台专用字段。前台模板不读这些字段，零值即可。
	Admin *AdminData
}

// PostCard 首页文章卡片数据，对齐 Stellar _partial/main/post_list/post_card.ejs。
type PostCard struct {
	Slug    string
	Title   string
	Excerpt string
	Cover   string    // 封面图 URL，空则不渲染 .post-cover
	Date    time.Time // 用于 <time> 标签与格式化
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
}

// AdminPostRow 后台列表/编辑表单用的文章行数据。
type AdminPostRow struct {
	Slug      string
	Title     string
	Excerpt   string
	Cover     string // 封面图 URL（编辑表单回显用）
	ContentMD string
	Status    string
	Date      string // 列表展示用（created_at 或文件 date）
}

// Site 对齐 Stellar theme + config 的站点级配置。
// yaml tag 供 config/site.yml 加载（goccy/go-yaml），snake_case 精确映射。
type Site struct {
	Title       string       `yaml:"title"`
	Subtitle    string       `yaml:"subtitle"`
	Description string       `yaml:"description"`
	Avatar      string       `yaml:"avatar"` // logo 头像 URL
	LogoTitle   string       `yaml:"logo_title"`
	LogoSub     string       `yaml:"logo_sub"`
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
