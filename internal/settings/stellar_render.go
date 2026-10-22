package settings

import "blog/internal/stellarmodels"

// StellarRenderContext 是路由到 SSR 的内容边界；模型输入只含公开已授权数据。
// 版本与配置先经 Store.ValidateStellar/stellarv2 membership 规则校验。
type StellarRenderContext struct {
	Version           int
	Profile           string
	Input             stellarmodels.Input
	NavigationMembers map[string]string
	ListItems         []stellarmodels.PageModel
	PinnedItems       []stellarmodels.PageModel
	Collections       []stellarmodels.Object
	Index             stellarmodels.Object
	// RequestURL 保留实际请求地址；canonical展示地址由宿主SEO单独计算。
	RequestURL string
}
