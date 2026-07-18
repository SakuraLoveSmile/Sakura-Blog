package template

import (
	"fmt"
	"html/template"
	"strings"
)

// MomentItem 前台瞬间时间流单项。
type MomentItem struct {
	ID          int64
	ContentHTML template.HTML
	Images      []string
	TimeAgo     string
	Date        string // 完整日期（title 悬浮提示）
}

// AdminMomentRow 后台瞬间列表行。
type AdminMomentRow struct {
	ID         int64
	Excerpt    string
	ImageCount int
	Date       string
}

// AdminMomentData 瞬间管理页渲染数据。
type AdminMomentData struct {
	Site *Site
	Page *Page

	Items       []*AdminMomentRow
	CurrentPage int
	Pages       []int
	CreateError string
}

// RenderMomentAdmin 渲染瞬间管理页。
func (e *Engine) RenderMomentAdmin(data *AdminMomentData) (string, error) {
	t := e.tpl.Lookup("admin/moment_list")
	if t == nil {
		return "", fmt.Errorf("template %q not found", "admin/moment_list")
	}
	var sb strings.Builder
	if err := t.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("execute admin/moment_list: %w", err)
	}
	return sb.String(), nil
}
