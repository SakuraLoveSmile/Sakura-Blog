package settings

// HeroConfig 页面级 hero banner 三要素（形状取 CoverConfig 的最小子集）：
// title 主标题、description 副标题、background 背景图 URL。
type HeroConfig struct {
	Title       string       `yaml:"title" json:"title"`
	Description string       `yaml:"description,omitempty" json:"description,omitempty"`
	Background  string       `yaml:"background,omitempty" json:"background,omitempty"`
	Start       string       `yaml:"start,omitempty" json:"start,omitempty"`
	Preview     *HeroPreview `yaml:"preview,omitempty" json:"preview,omitempty"`
	Actions     []HeroAction `yaml:"actions,omitempty" json:"actions,omitempty"`
}

type HeroPreview struct {
	Type     string        `yaml:"type,omitempty" json:"type,omitempty"`
	Commands []HeroCommand `yaml:"commands,omitempty" json:"commands,omitempty"`
	Image    string        `yaml:"image,omitempty" json:"image,omitempty"`
}

type HeroCommand struct {
	Codes []string `yaml:"codes,omitempty" json:"codes,omitempty"`
}

type HeroAction struct {
	Title string `yaml:"title,omitempty" json:"title,omitempty"`
	URL   string `yaml:"url,omitempty" json:"url,omitempty"`
	Icon  string `yaml:"icon,omitempty" json:"icon,omitempty"`
}

// On hero 是否配置（title 或 background 任一非空）。
func (h *HeroConfig) On() bool { return h != nil && (h.Title != "" || h.Background != "") }

// PageFriendsConfig 友链页页面级设置（pages.friends 段）。
// Title 覆盖页面标题（空回退 handler 缺省「友链」）；Hero 为页首 hero；
// ShowPosts 为 RSS 订阅聚合组件开关（nil=开，兼容现状）；
// RefreshInterval/PostsPerLink <=0 时回退旧 friends 段同名字段。
type PageFriendsConfig struct {
	Title           string      `yaml:"title,omitempty" json:"title,omitempty"`
	Hero            *HeroConfig `yaml:"hero,omitempty" json:"hero,omitempty"`
	ShowPosts       *bool       `yaml:"show_posts,omitempty" json:"show_posts,omitempty"`
	RefreshInterval int         `yaml:"refresh_interval,omitempty" json:"refresh_interval,omitempty"`
	PostsPerLink    int         `yaml:"posts_per_link,omitempty" json:"posts_per_link,omitempty"`
}

// ShowPostsOn RSS 订阅组件是否渲染（未配置=开）。
func (c PageFriendsConfig) ShowPostsOn() bool { return c.ShowPosts == nil || *c.ShowPosts }

// PagesConfig 页面级设置段（pages 段）：按页面键管理，当前仅 friends，预留按页扩展。
type PagesConfig struct {
	Friends PageFriendsConfig `yaml:"friends" json:"friends"`
}

// FriendsRefreshInterval 生效的 RSS 抓取刷新间隔（分钟）：pages.friends 优先，<=0 回退 friends 段。
func (s *Site) FriendsRefreshInterval() int {
	if n := s.Pages.Friends.RefreshInterval; n > 0 {
		return n
	}
	return s.Friends.RefreshInterval
}

// FriendsPostsPerLink 生效的每友链最新文章数：pages.friends 优先，<=0 回退 friends 段。
func (s *Site) FriendsPostsPerLink() int {
	if n := s.Pages.Friends.PostsPerLink; n > 0 {
		return n
	}
	return s.Friends.MaxPostsPerFeed
}
