package template

import (
	"sort"
	"strings"
	"time"
)

// ArticleConfig 文章页脚配置（site.yml article 段）。
type ArticleConfig struct {
	Copyright CopyrightConfig `yaml:"copyright" json:"copyright"`
	Reward    RewardConfig    `yaml:"reward" json:"reward"`
	Share     ShareConfig     `yaml:"share" json:"share"`
	// CategoryColor 分类配色表（分类名→色值），对齐原版 theme.article.category_color，
	// 首页卡片分类 meta 的 --text-p2/--theme-block 配色来源。
	CategoryColor map[string]string `yaml:"category_color" json:"category_color"`

	// Spec 32 文章页首尾增强：
	ReadNext     bool    `yaml:"read_next" json:"read_next"`         // read_next 下一篇卡片开关
	RelatedPosts int     `yaml:"related_posts" json:"related_posts"` // 相关文章数上限，0=关
	ExpireDays   int     `yaml:"expire_days" json:"expire_days"`     // 过期提醒阈值（天），0=缺省 180
	BannerRatio  float64 `yaml:"banner_ratio" json:"banner_ratio"`   // 文首横幅宽高比，0=主题默认 2.5
	// EditThisPage 「编辑本文」路径映射：源路径前缀 → 仓库 blob URL 前缀
	// （对齐原版 contributors.ejs 的 map 参数，如 posts/ → https://github.com/u/r/blob/main/content/posts/）。
	EditThisPage map[string]string `yaml:"edit_this_page" json:"edit_this_page"`
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

// defaultExpireDays 过期提醒缺省阈值（Spec 32 技术决策）。
const defaultExpireDays = 180

// ExpireAfterDays 过期提醒阈值；未配置（<=0）回退缺省 180 天。
func (a ArticleConfig) ExpireAfterDays() int {
	if a.ExpireDays > 0 {
		return a.ExpireDays
	}
	return defaultExpireDays
}

// ExpiredDays 计算距最后更新的天数；未超过阈值返回 0（不提醒）。
// updated 为空回退 created；日期与 funcs.go date helper 同格式，解析失败返回 0。
func ExpiredDays(created, updated string, threshold int, now time.Time) int {
	raw := updated
	if raw == "" {
		raw = created
	}
	if raw == "" {
		return 0
	}
	var t time.Time
	var err error
	if t, err = time.Parse("2006-01-02 15:04:05", raw); err != nil {
		if t, err = time.Parse("2006-01-02", raw); err != nil {
			return 0
		}
	}
	days := int(now.Sub(t).Hours() / 24)
	if days > threshold {
		return days
	}
	return 0
}

// EditPageLink 「编辑本文」按钮与贡献者 commits API 数据（对齐原版 contributors.ejs）。
type EditPageLink struct {
	EditURL string // 编辑本文按钮链接（仓库 blob URL）
	APIURL  string // ds-contributors 拉 commits 的 API 地址，空=不渲染贡献者列表
}

// ResolveEditPage 按映射表前缀替换 source（如 posts/hello.md）生成编辑链接；
// commits API 按原版规则从 blob URL 推导：/github.com/ → /api.github.com/repos/，
// /blob/main/ → /commits?path=。无匹配前缀返回 nil。
// map 遍历无序，按前缀长度 desc 匹配保证多前缀时结果确定。
func ResolveEditPage(source string, mapping map[string]string) *EditPageLink {
	if source == "" || len(mapping) == 0 {
		return nil
	}
	keys := make([]string, 0, len(mapping))
	for k := range mapping {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, k := range keys {
		if k == "" || mapping[k] == "" || !strings.HasPrefix(source, k) {
			continue
		}
		edit := mapping[k] + strings.TrimPrefix(source, k)
		api := strings.Replace(edit, "/github.com/", "/api.github.com/repos/", 1)
		api = strings.Replace(api, "/blob/main/", "/commits?path=", 1)
		// 非 github.com + main 分支结构时无法推导 commits API，仅保留编辑按钮
		if !strings.Contains(api, "api.github.com") || !strings.Contains(api, "/commits?path=") {
			api = ""
		}
		return &EditPageLink{EditURL: edit, APIURL: api}
	}
	return nil
}
