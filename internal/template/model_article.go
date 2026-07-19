package template

// ArticleConfig 文章页脚配置（site.yml article 段）。
type ArticleConfig struct {
	Copyright CopyrightConfig `yaml:"copyright"`
	Reward    RewardConfig    `yaml:"reward"`
	Share     ShareConfig     `yaml:"share"`
	// CategoryColor 分类配色表（分类名→色值），对齐原版 theme.article.category_color，
	// 首页卡片分类 meta 的 --text-p2/--theme-block 配色来源。
	CategoryColor map[string]string `yaml:"category_color"`
}

// CopyrightConfig 版权声明块。
type CopyrightConfig struct {
	Enabled    bool   `yaml:"enabled"`
	License    string `yaml:"license"`
	LicenseURL string `yaml:"license_url"`
}

// RewardConfig 打赏块（懒展开收款码）。
type RewardConfig struct {
	Enabled bool   `yaml:"enabled"`
	Wechat  string `yaml:"wechat"`
	Alipay  string `yaml:"alipay"`
	Text    string `yaml:"text"`
}

// ShareConfig 社交分享块。
type ShareConfig struct {
	Enabled   bool     `yaml:"enabled"`
	Platforms []string `yaml:"platforms"`
}
