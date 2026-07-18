package template

import (
	"fmt"
	"strings"
)

// AdminPageRow 后台页面列表/编辑表单用的行数据。
type AdminPageRow struct {
	Slug      string
	Title     string
	ContentMD string
	Status    string
	Date      string // 列表展示用（updated_at）
}

// AdminPageData 页面管理后台模板根数据（独立于 AdminData，不改 model.go）。
type AdminPageData struct {
	Site *Site
	Page *Page

	Items     []*AdminPageRow // 列表页
	EditPage  *AdminPageRow   // 编辑页
	SaveError string
	IsEdit    bool
}

// RenderPageAdmin 渲染页面管理后台模板（admin/page_list、admin/page_edit）。
func (e *Engine) RenderPageAdmin(name string, data *AdminPageData) (string, error) {
	t := e.tpl.Lookup(name)
	if t == nil {
		return "", fmt.Errorf("template %q not found", name)
	}
	var sb strings.Builder
	if err := t.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("execute %s: %w", name, err)
	}
	return sb.String(), nil
}
