package content

// StellarPreviewInput 是宿主到前台的隔离预览候选；正文与文档由同一快照渲染。
// 原始配置保持文本，生产与预览共用前台装配逻辑，Console 不依赖 Stellar 模块。
type StellarPreviewInput struct {
	StellarVersion int
	StellarConfig  string
	BlogExtensions string
	HTML           string
	// Markdown 非 nil 时由前台在同一个公开快照内渲染正文与完整文档。
	Markdown     *string
	Title        string
	Layout       string
	Language     string
	Path         string
	RequestURL   string
	WidgetConfig string
	PreviewToken string
}

// StellarPreviewOutput 绑定同一次快照产生的正文和文档。
type StellarPreviewOutput struct {
	HTML     string
	Document string
}
