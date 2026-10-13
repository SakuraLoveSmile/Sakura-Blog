package settings

// StellarV2Config 是完成迁移后的配置快照。RawTheme 保留公开 snake_case
// 输入（含省略、null、false 与开放参数袋），Theme 是上游校验后的 camelCase
// 运行时对象。Blog 独立保存本地扩展，不能进入上游主题配置树。
// 与 Site 一样，发布给 Manager 后调用者不得修改其中的 map。
type StellarV2Config struct {
	RawTheme map[string]any `json:"theme"`
	Theme    map[string]any `json:"-"`
	Blog     map[string]any `json:"blog_extensions"`
}
