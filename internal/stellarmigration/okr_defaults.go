package stellarmigration

// 固定 9d11978a24fe6610e3a7dbab741cf62634539d05：
// scripts/tags/lib/okr.js 的五色、languages/zh-CN.yml 的五文案，
// source/css/_components/tag-plugins/okr.styl 的固定 1px 边框。
// 仅匹配完整的旧 settings 对象；缺键、额外键、自定义或错误类型不归为等效。
func isOfficialLegacyOKR(value any) bool {
	expected := map[string]any{
		"border": true,
		"status": map[string]any{
			"in_track":   map[string]any{"color": "blue", "label": "正常"},
			"at_risk":    map[string]any{"color": "yellow", "label": "风险"},
			"off_track":  map[string]any{"color": "orange", "label": "延期"},
			"finished":   map[string]any{"color": "green", "label": "已完成"},
			"unfinished": map[string]any{"color": "red", "label": "未完成"},
		},
	}
	return encode(value) == encode(expected)
}
