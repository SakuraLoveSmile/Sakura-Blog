package settings

import "sync"

// TagPluginsConfig tag_plugins 段（对齐原版 _config.yml tag_plugins）：标签插件默认参数。
// 全部字段缺省空 = shortcode.go 内置硬编码默认值（零配置回归底线）。
type TagPluginsConfig struct {
	Note     TagPluginNote     `yaml:"note" json:"note"`
	Quot     TagPluginQuot     `yaml:"quot" json:"quot"`
	Emoji    map[string]string `yaml:"emoji" json:"emoji"`     // 表情源：来源名 → URL 模板（{name} 占位）
	Icon     TagPluginColorDef `yaml:"icon" json:"icon"`       // {% icon %} 默认颜色
	Button   TagPluginColorDef `yaml:"button" json:"button"`   // {% button %} 默认颜色
	Mark     TagPluginColorDef `yaml:"mark" json:"mark"`       // {% mark %} 默认颜色
	Hashtag  TagPluginColorDef `yaml:"hashtag" json:"hashtag"` // {% hashtag %} 默认颜色（空=循环色板）
	Gallery  TagPluginGallery  `yaml:"gallery" json:"gallery"`
	Timeline TagPluginTimeline `yaml:"timeline" json:"timeline"`
	Copy     TagPluginCopy     `yaml:"copy" json:"copy"`
}

// TagPluginNote {% note %} 默认参数。
type TagPluginNote struct {
	DefaultColor string `yaml:"default_color" json:"default_color"` // 未指定 color 时的默认配色
}

// TagPluginQuot {% quot %} 默认前后缀图标（icons.yml key 或图片 URL）。
type TagPluginQuot struct {
	Prefix string `yaml:"prefix" json:"prefix"`
	Suffix string `yaml:"suffix" json:"suffix"`
}

// TagPluginColorDef 单色默认值容器。
type TagPluginColorDef struct {
	DefaultColor string `yaml:"default_color" json:"default_color"`
}

// TagPluginGallery {% gallery %} 默认布局参数。
type TagPluginGallery struct {
	Layout string `yaml:"layout" json:"layout"` // grid / flow
	Size   string `yaml:"size" json:"size"`     // s / m / l / xl / mix
	Ratio  string `yaml:"ratio" json:"ratio"`   // origin / square
}

// TagPluginTimeline {% timeline %} 默认参数。
type TagPluginTimeline struct {
	MaxHeight string `yaml:"max_height" json:"max_height"` // 如 80vh，空=不限高
}

// TagPluginCopy {% copy %} 默认参数。
type TagPluginCopy struct {
	Toast string `yaml:"toast" json:"toast"` // 复制成功提示文案
}

// 全局当前值：config.RebuildSite 写入，post shortcode 渲染时读取。
// 预览草稿不写全局，保证线上渲染不受设置页草稿影响。
var (
	tagPluginsMu sync.RWMutex
	tagPlugins   TagPluginsConfig
)

// SetTagPlugins 更新全局标签插件默认参数（仅 RebuildSite 调用）。
func SetTagPlugins(c TagPluginsConfig) {
	tagPluginsMu.Lock()
	tagPlugins = c
	tagPluginsMu.Unlock()
}

// CurrentTagPlugins 返回当前标签插件默认参数（零值为内置默认）。
func CurrentTagPlugins() TagPluginsConfig {
	tagPluginsMu.RLock()
	defer tagPluginsMu.RUnlock()
	return tagPlugins
}
