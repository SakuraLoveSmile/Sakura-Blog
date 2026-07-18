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
	TOC     template.HTML // 文章目录（ExtractTOC 从 Content 提取，空则不渲染右栏 widget）

	// 首页文章列表（Layout=index 时使用）。
	Posts     []*PostCard // 文章卡片列表
	Paginator *Paginator  // 分页器数据（nil 或 Total<=1 则不渲染）

	// 归档与分类/标签页（Layout=archive/categories/tags 时使用）。
	Archives []*ArchiveYear // 归档页年份分组
	Cats     []*CatItem     // 分类索引页
	TagItems []*TagItem     // 标签索引页

	// 侧边栏动态数据（所有前台页面渲染前由 fillSidebar 填充）。
	Recent   []*RecentPost // 近期发布 widget（最新 5 篇 published）
	TagCloud []*TagItem    // 标签云 widget（按文章数降序前 20）

	// 评论区（Layout=post 且 comments.enabled 时填充）。
	Comments     []*CommentItem // 评论树（顶级 + Children 两层）
	CommentCount int            // 已发布评论总数（"N 条评论"计数）

	// 最新评论 widget（fillSidebar 填充，comments.enabled 时）。
	LatestComments []*LatestComment

	// 文章页上下篇导航与相关文章。
	PrevPost     *AdjacentPost // 上一篇（时间更早），nil 则不渲染
	NextPost     *AdjacentPost // 下一篇（时间更晚），nil 则不渲染
	RelatedPosts []*PostCard   // 相关文章（按标签匹配，最多 4 篇）

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
}

// TagItem 标签索引页单项。
type TagItem struct {
	Name  string
	Count int
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

	// 评论管理页（/admin/comments）。
	Comments      []*AdminCommentRow // 评论列表
	CommentFilter string             // 当前筛选：all / pending / approved
	CommentPage   int                // 当前页码（从 1 开始）
	CommentPages  []int              // 页码序列（分页器用）

	// 登录页（/admin/login）。
	LoginError string // 登录失败提示（密码错误）
	LoginFrom  string // 登录成功后的回跳地址（authRequired 跳转时带入的原始路径）
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
	Tags      string // 标签，逗号分隔（编辑表单回显用）
	ContentMD string
	Status    string
	Date      string // 列表展示用（created_at 或文件 date）
}

// Site 对齐 Stellar theme + config 的站点级配置。
// yaml tag 供 config/site.yml 加载（goccy/go-yaml），snake_case 精确映射。
type Site struct {
	Title       string         `yaml:"title"`
	Subtitle    string         `yaml:"subtitle"`
	Description string         `yaml:"description"`
	URL         string         `yaml:"url"`          // 站点基础 URL（如 https://example.com），用于 canonical/OG/sitemap
	Avatar      string         `yaml:"avatar"`    // logo 头像 URL
	AvatarBG    string         `yaml:"avatar_bg"` // 头像 hover 旋转背景图，对应原版 style.animated_avatar.background
	LogoTitle   string         `yaml:"logo_title"`
	LogoSub     string         `yaml:"logo_sub"`
	Menubar     Menubar        `yaml:"menubar"`
	Footer      Footer         `yaml:"footer"`
	Comments    CommentsConfig `yaml:"comments"`
	Admin       AdminConfig    `yaml:"admin"`
	Links       []LinkGroup    `yaml:"links"` // 友链分组（site.yml links 段）
}

// AdminConfig 管理后台配置（site.yml admin 段）。
type AdminConfig struct {
	Password string `yaml:"password"` // /admin 访问密码；留空则不做密码 gate（本地开发用）
}

// CommentsConfig 评论系统配置（site.yml comments 段）。
type CommentsConfig struct {
	Enabled        bool         `yaml:"enabled"`         // 总开关：false 时文章页不渲染评论区、API 关闭
	Title          string       `yaml:"title"`           // 评论区标题
	Moderation     bool         `yaml:"moderation"`      // true=先审后发（新评论 pending）
	AdminEmail     string       `yaml:"admin_email"`     // 博主邮箱：新评论通知收件人 + 博主身份匹配
	AdminBadge     string       `yaml:"admin_badge"`     // 博主徽章文案
	GravatarMirror string       `yaml:"gravatar_mirror"` // Gravatar 镜像基址（头像 = {mirror}/{md5(email)}）
	Notify         NotifyConfig `yaml:"notify"`          // 邮件通知；smtp_host 为空则整体关闭
}

// NotifyConfig 邮件通知配置（net/smtp + PlainAuth）。
type NotifyConfig struct {
	SMTPHost string `yaml:"smtp_host"`
	SMTPPort int    `yaml:"smtp_port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"` // 发件人地址，空则用 username
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

// CommentItem 前台评论区单条评论视图模型（含两层嵌套的子评论）。
// json tag 供 /api/comments 响应序列化（前端无刷新插入用）。
type CommentItem struct {
	ID          int64          `json:"id"`
	Nick        string         `json:"nick"`
	Link        string         `json:"link"`         // 个人网站，非空时昵称带链
	ContentHTML template.HTML  `json:"content_html"` // 已消毒的服务端渲染 HTML，直接输出
	IsAdmin     bool           `json:"is_admin"`
	Avatar      string         `json:"avatar"`   // Gravatar URL（{mirror}/{md5(email)}）
	TimeAgo     string         `json:"time_ago"` // 相对时间，如"3 天前"
	Date        string         `json:"date"`     // 完整日期（title 悬浮提示）
	Children    []*CommentItem `json:"children,omitempty"`
}

// LatestComment 侧边栏"最新评论"widget 单项。
type LatestComment struct {
	Nick    string
	Excerpt string // 内容纯文本摘要（截断）
	URL     string // page_key + #comment-{id} 锚点
	TimeAgo string
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
	Date         string // 原始日期字符串
	Updated      string
	Excerpt      string
	Views        int64  // 文章阅读量（仅文章页 Layout=post 使用，已含本次访问）
	Category     string // 分类文章列表页的当前分类名（navbar 显示"分类：xxx"）
	Tag          string // 标签文章列表页的当前标签名
	CanonicalURL string // 当前页面完整 URL（用于 canonical 和 og:url）
	Description  string // 页面描述（用于 og:description，文章页用 Excerpt，非文章页用 Site.Description）
}
