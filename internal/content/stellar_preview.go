package content

// StellarPreviewInput 是宿主到前台的隔离预览快照；HTML 已经严格正文渲染。
// 原始配置保持文本，生产与预览共用前台装配逻辑，Console 不依赖 Stellar 模块。
type StellarPreviewInput struct {
	StellarVersion int
	StellarConfig  string
	BlogExtensions string
	HTML           string
	Title          string
	Layout         string
	Language       string
	Path           string
	RequestURL     string
	PreviewToken   string
}
