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

// GridSize 依图片数返回九宫格 size 属性（复用 main.css .tag-plugin.gallery.grid-box[size]）。
// 1 张：大图；2-4 张：两列；5-9 张：三列自适应（对齐 spec 的 1/2-4/9 三种布局）。
func (m *MomentItem) GridSize() string {
	switch n := len(m.Images); {
	case n <= 1:
		return "l"
	case n <= 4:
		return "xl"
	default:
		return "l"
	}
}

// GridRatio 依图片数返回九宫格 ratio 属性：单图保原比例，多图裁方形（九宫格观感）。
func (m *MomentItem) GridRatio() string {
	if len(m.Images) <= 1 {
		return "origin"
	}
	return "square"
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
