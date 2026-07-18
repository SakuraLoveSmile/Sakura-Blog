package template

import (
	"fmt"
	"strings"
)

// AdminMediaRow 附件管理页网格单项。
type AdminMediaRow struct {
	ID       int64
	Filename string // 原始文件名
	URL      string // 对外 /uploads/... URL
	Mime     string
	Size     string // 人类可读大小（如 128 KB）
	Date     string // 列表展示用
}

// AdminMediaData 附件管理页渲染数据（独立于 AdminData，不改 model.go）。
// 作为 admin/media 模板的根数据（含 Site/Page），经 Engine.RenderMedia 渲染。
type AdminMediaData struct {
	Site *Site
	Page *Page

	Items       []*AdminMediaRow
	CurrentPage int   // 当前页码（从 1 开始）
	Pages       []int // 页码序列
	UploadError string
	MaxUploadMB int
}

// RenderMedia 渲染附件管理页。与 Render 分离，避免改 ViewData / model.go。
func (e *Engine) RenderMedia(data *AdminMediaData) (string, error) {
	t := e.tpl.Lookup("admin/media")
	if t == nil {
		return "", fmt.Errorf("template %q not found", "admin/media")
	}
	var sb strings.Builder
	if err := t.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("execute admin/media: %w", err)
	}
	return sb.String(), nil
}
