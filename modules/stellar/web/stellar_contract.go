package web

import (
	"html/template"

	"blog/internal/post"
	"blog/internal/settings"
	"blog/internal/stellarmodels"
)

// StellarRenderData 保持旧宿主数据字段可用，严格主题和模型消费者集中在 Stellar。
// 在新布局中，旧 Site.Style/Plugins 等不得替代规范化 Theme。
type StellarRenderData struct {
	*settings.ViewData
	Stellar        *StellarPresentation
	Region         string
	markdownRender func(string, post.StellarRenderContext) (string, error)
	markdownError  error
}

// StellarPresentation 是模板的装配结果，不持久化或再次定义上游 schema。
// Client 为 clientconfig.Bundle，保留其具名 Comments/Styles/CoreScripts/Manifest。
type StellarPresentation struct {
	Theme, Regions, Data, Cover, Navigation map[string]any
	Model                                   *stellarmodels.PageModel
	Client                                  any
	CSS                                     template.CSS
	Profile, Language, ColorScheme          string
	PageType, Layout, ArticleStyle          string
	RegionNames, Root, SettingsPath         string
	Indent, SearchEnabled, MobileBrand      bool
	MobileBrandModel                        map[string]any
	Widgets                                 map[string][]StellarWidget
	RegionBodies                            map[string]template.HTML
	ListItems, Collections                  []map[string]any
	HeadInject, BodyBegin, BodyEnd          template.HTML
	BlogNativeComments, BlogContent         bool
}

// State 是官方 Widget instance（id/instanceId/item/presentations/presentation）。
// Widget 提供已有 Blog 宿主查询的数据；官方特殊组件仍读取 State/Model。
type StellarWidget struct {
	State  map[string]any
	Widget *settings.WidgetInstance
}
