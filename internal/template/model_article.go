package template

// ArticleConfig 文章页脚配置（site.yml article 段）。
type ArticleConfig struct {
	Copyright CopyrightConfig `yaml:"copyright" json:"copyright"`
	Reward    RewardConfig    `yaml:"reward" json:"reward"`
	Share     ShareConfig     `yaml:"share" json:"share"`
	// CategoryColor 分类配色表（分类名→色值），对齐原版 theme.article.category_color，
	// 首页卡片分类 meta 的 --text-p2/--theme-block 配色来源。
	CategoryColor map[string]string `yaml:"category_color" json:"category_color"`
}

// CopyrightConfig 版权声明块。
type CopyrightConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	License    string `yaml:"license" json:"license"`
	LicenseURL string `yaml:"license_url" json:"license_url"`
}

// RewardConfig 打赏块（懒展开收款码）。
type RewardConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Wechat  string `yaml:"wechat" json:"wechat"`
	Alipay  string `yaml:"alipay" json:"alipay"`
	Text    string `yaml:"text" json:"text"`
}

// ShareConfig 社交分享块。
type ShareConfig struct {
	Enabled   bool     `yaml:"enabled" json:"enabled"`
	Platforms []string `yaml:"platforms" json:"platforms"`
}
