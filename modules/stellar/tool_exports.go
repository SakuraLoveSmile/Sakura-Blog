package stellar

import (
	"context"

	"blog/internal/content"
	"blog/internal/stellarimages"
)

// RenderContentPreview 暴露组合根需要的完整前台预览，Console 不导入 Stellar 内部路由。
func (s *Server) RenderContentPreview(input content.StellarPreviewInput) (content.StellarPreviewOutput, error) {
	return s.Server.RenderContentPreviewSnapshot(input)
}

// StellarWidgetCatalog 供 Console 使用同一完整公开组件库。
func (s *Server) StellarWidgetCatalog() (map[string]any, error) {
	return s.Server.StellarWidgetCatalog()
}

// RenderSnapshots 暴露同一公开内容快照的实际 SSR 结果，供图片生命周期使用。
func (s *Server) RenderSnapshots(ctx context.Context) ([]stellarimages.PageSnapshot, error) {
	return s.Server.RenderSnapshots(ctx)
}

// RenderStellarMarkdown 供宿主保存与预览复用前台公开正文上下文。
func (s *Server) RenderStellarMarkdown(input content.StellarBodyInput) (string, error) {
	return s.Server.RenderStellarMarkdown(input)
}
