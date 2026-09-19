package settings

import (
	"encoding/json"
	"html/template"
	"strings"
	"sync"
)

// TagPluginsConfig tag_plugins 段（对齐原版 _config.yml tag_plugins）：标签插件默认参数。
// 全部字段缺省空 = shortcode.go 内置硬编码默认值（零配置回归底线）。
type TagPluginsConfig struct {
	Note     TagPluginNote     `yaml:"note" json:"note"`
	Checkbox TagPluginCheckbox `yaml:"checkbox" json:"checkbox"`
	Quot     TagPluginQuot     `yaml:"quot" json:"quot"`
	Emoji    map[string]string `yaml:"emoji" json:"emoji"`     // 表情源：来源名 → URL 模板（{name} 占位）
	Icon     TagPluginColorDef `yaml:"icon" json:"icon"`       // {% icon %} 默认颜色
	Button   TagPluginColorDef `yaml:"button" json:"button"`   // {% button %} 默认颜色
	Mark     TagPluginColorDef `yaml:"mark" json:"mark"`       // {% mark %} 默认颜色
	Hashtag  TagPluginColorDef `yaml:"hashtag" json:"hashtag"` // {% hashtag %} 默认颜色（空=循环色板）
	Gallery  TagPluginGallery  `yaml:"gallery" json:"gallery"`
	Timeline TagPluginTimeline `yaml:"timeline" json:"timeline"`
	Copy     TagPluginCopy     `yaml:"copy" json:"copy"`
	OKR      TagPluginOKR      `yaml:"okr" json:"okr"`
}

// TagPluginNote {% note %} 默认参数。
type TagPluginNote struct {
	DefaultColor string `yaml:"default_color" json:"default_color"` // 未指定 color 时的默认配色
	Border       *bool  `yaml:"border,omitempty" json:"border,omitempty"`
}

// TagPluginCheckbox {% checkbox %}/{% radio %} 是否允许访客交互。
type TagPluginCheckbox struct {
	Interactive bool `yaml:"interactive" json:"interactive"`
}

// TagPluginQuot {% quot %} 图标预设（icons.yml key 或图片 URL）。
type TagPluginQuot struct {
	Default  *TagPluginQuotPreset `yaml:"default,omitempty" json:"default,omitempty"`
	Hashtag  *TagPluginQuotPreset `yaml:"hashtag,omitempty" json:"hashtag,omitempty"`
	Question *TagPluginQuotPreset `yaml:"question,omitempty" json:"question,omitempty"`
	// LegacyPrefix/LegacySuffix 只用于读取旧版扁平 quot.prefix/suffix，序列化时改写为 default。
	LegacyPrefix *string `yaml:"prefix,omitempty" json:"-"`
	LegacySuffix *string `yaml:"suffix,omitempty" json:"-"`
}

type TagPluginQuotPreset struct {
	Prefix string `yaml:"prefix" json:"prefix"`
	Suffix string `yaml:"suffix" json:"suffix"`
}

// UnmarshalJSON 兼容旧版扁平 quot.prefix/suffix 结构。
func (q *TagPluginQuot) UnmarshalJSON(data []byte) error {
	var raw struct {
		Default  *TagPluginQuotPreset `json:"default"`
		Hashtag  *TagPluginQuotPreset `json:"hashtag"`
		Question *TagPluginQuotPreset `json:"question"`
		Prefix   *string              `json:"prefix"`
		Suffix   *string              `json:"suffix"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	q.Default = raw.Default
	q.Hashtag = raw.Hashtag
	q.Question = raw.Question
	q.LegacyPrefix = raw.Prefix
	q.LegacySuffix = raw.Suffix
	return nil
}

// MarshalJSON 始终写入上游 default/hashtag/question 结构，不把兼容字段泄漏到设置 API。
func (q TagPluginQuot) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Default  *TagPluginQuotPreset `json:"default,omitempty"`
		Hashtag  *TagPluginQuotPreset `json:"hashtag,omitempty"`
		Question *TagPluginQuotPreset `json:"question,omitempty"`
	}{Default: q.Default, Hashtag: q.Hashtag, Question: q.Question})
}

// TagPluginOKR OKR 边框和状态映射。
type TagPluginOKR struct {
	Border *bool                         `yaml:"border,omitempty" json:"border,omitempty"`
	Status map[string]TagPluginOKRStatus `yaml:"status,omitempty" json:"status,omitempty"`
}

type TagPluginOKRStatus struct {
	Color string `yaml:"color" json:"color"`
	Label string `yaml:"label" json:"label"`
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

var defaultOKRStatuses = map[string]TagPluginOKRStatus{
	"in_track":   {Color: "blue", Label: "正常"},
	"at_risk":    {Color: "yellow", Label: "风险"},
	"off_track":  {Color: "orange", Label: "延期"},
	"finished":   {Color: "green", Label: "已完成"},
	"unfinished": {Color: "red", Label: "未完成"},
}

// FillDefaults 补齐 Stellar v1.44 新增配置；只填缺失对象/键，不覆盖已有值。
func (c *TagPluginsConfig) FillDefaults() {
	if c.Note.Border == nil {
		v := true
		c.Note.Border = &v
	}
	if c.Quot.Default == nil && (c.Quot.LegacyPrefix != nil || c.Quot.LegacySuffix != nil) {
		preset := TagPluginQuotPreset{}
		if c.Quot.LegacyPrefix != nil {
			preset.Prefix = *c.Quot.LegacyPrefix
		}
		if c.Quot.LegacySuffix != nil {
			preset.Suffix = *c.Quot.LegacySuffix
		}
		c.Quot.Default = &preset
	}
	if c.Quot.Default == nil {
		c.Quot.Default = &TagPluginQuotPreset{Prefix: "quot:quote-left", Suffix: "quot:quote-right"}
	}
	if c.Quot.Hashtag == nil {
		c.Quot.Hashtag = &TagPluginQuotPreset{Prefix: "quot:hashtag"}
	}
	if c.Quot.Question == nil {
		c.Quot.Question = &TagPluginQuotPreset{Prefix: "quot:question"}
	}
	if c.Emoji == nil {
		c.Emoji = map[string]string{}
	}
	for key, value := range defaultEmojiSources {
		if _, ok := c.Emoji[key]; !ok {
			c.Emoji[key] = value
		}
	}
	if c.OKR.Border == nil {
		v := true
		c.OKR.Border = &v
	}
	if c.OKR.Status == nil {
		c.OKR.Status = map[string]TagPluginOKRStatus{}
	}
	for key, value := range defaultOKRStatuses {
		if _, ok := c.OKR.Status[key]; !ok {
			c.OKR.Status[key] = value
		}
	}
}

// EffectiveNoteBorder 返回 note 边框开关，缺省遵循上游 true。
func (c TagPluginsConfig) EffectiveNoteBorder() bool {
	return c.Note.Border == nil || *c.Note.Border
}

// EffectiveOKRBorder 返回 OKR 边框开关，缺省遵循上游 true。
func (c TagPluginsConfig) EffectiveOKRBorder() bool {
	return c.OKR.Border == nil || *c.OKR.Border
}

// QuotPreset 返回指定预设；未知预设与上游一致返回空预设。
func (c TagPluginsConfig) QuotPreset(name string) TagPluginQuotPreset {
	var preset *TagPluginQuotPreset
	switch strings.ToLower(name) {
	case "", "default":
		preset = c.Quot.Default
	case "hashtag":
		preset = c.Quot.Hashtag
	case "question":
		preset = c.Quot.Question
	}
	if preset == nil {
		return TagPluginQuotPreset{}
	}
	return *preset
}

// InjectCSS 生成标签插件的运行时覆盖，避免依赖编译 main.css 时的静态配置。
func (c TagPluginsConfig) InjectCSS() template.CSS {
	var b strings.Builder
	if c.EffectiveNoteBorder() {
		b.WriteString(".md-text .tag-plugin.note{border:1px solid var(--theme-border)}")
	} else {
		b.WriteString(".md-text .tag-plugin.note{border:0}")
	}
	if c.EffectiveOKRBorder() {
		b.WriteString(".md-text .tag-plugin.okr{border:1px solid var(--theme-border)}")
	} else {
		b.WriteString(".md-text .tag-plugin.okr{border:0}")
	}
	if c.Checkbox.Interactive {
		b.WriteString(".md-text .tag-plugin.checkbox input{pointer-events:auto}")
	} else {
		b.WriteString(".md-text .tag-plugin.checkbox input{pointer-events:none}")
	}
	return template.CSS(b.String())
}

var defaultEmojiSources = map[string]string{
	"default": "https://gcore.jsdelivr.net/gh/cdn-x/emoticons@3.1/qq/{name}.gif",
	"twemoji": "https://gcore.jsdelivr.net/gh/twitter/twemoji/assets/svg/{name}.svg",
	"qq":      "https://gcore.jsdelivr.net/gh/cdn-x/emoticons@3.1/qq/{name}.gif",
	"aru":     "https://gcore.jsdelivr.net/gh/cdn-x/emoticons@3.1/aru/{name}.gif",
	"tieba":   "https://gcore.jsdelivr.net/gh/cdn-x/emoticons@3.1/tieba/{name}.png",
	"blobcat": "https://gcore.jsdelivr.net/gh/cdn-x/emoticons@3.1/blobcat/{name}.gif",
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
