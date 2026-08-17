package settings

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
