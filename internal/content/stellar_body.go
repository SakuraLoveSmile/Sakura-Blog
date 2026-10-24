package content

import "errors"

// StellarBodyInput 只携带作者本次正文与已验证的页面配置；公开站点数据由前台捕获。
type StellarBodyInput struct {
	Markdown string
	Page     map[string]any
	Language string
}

var ErrStellarRendererUnavailable = errors.New("stellar Markdown renderer unavailable")
