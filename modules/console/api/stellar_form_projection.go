package api

import (
	"encoding/json"
	"strings"
)

// 原文是严格配置的真值。仅把明确出现的公开字段投影至列表/搜索使用的宿主列；
// 这里不回填原文、不从旧列生成配置，也不解除版本与 schema 校验。
func stellarFormObject(raw string) map[string]any {
	value, _ := decodeStellarJSONObject(raw)
	return value
}
func stellarFormValue(object map[string]any, path string) (any, bool) {
	keys := strings.Split(path, ".")
	for _, key := range keys[:len(keys)-1] {
		nested, ok := object[key].(map[string]any)
		if !ok {
			return nil, false
		}
		object = nested
	}
	value, ok := object[keys[len(keys)-1]]
	return value, ok
}
func stellarFormString(object map[string]any, path string, target *string) {
	if value, exists := stellarFormValue(object, path); exists {
		if value == nil {
			*target = ""
		} else if text, ok := value.(string); ok {
			*target = text
		}
	}
}
func stellarFormInt(object map[string]any, path string, target *int) {
	if value, exists := stellarFormValue(object, path); exists {
		if n, ok := value.(json.Number); ok {
			if number, err := n.Int64(); err == nil && int64(int(number)) == number {
				*target = int(number)
			}
		}
	}
}
func stellarFormStrings(object map[string]any, path string) ([]string, bool) {
	value, exists := stellarFormValue(object, path)
	if !exists {
		return nil, false
	}
	values, ok := value.([]any)
	if !ok {
		return nil, false
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	return result, true
}
func projectPostForm(req *postRequest) {
	if req.StellarVersion != 2 {
		return
	}
	raw, blog := stellarFormObject(req.StellarConfig), stellarFormObject(req.BlogExtensions)
	stellarFormString(raw, "title", &req.Title)
	stellarFormString(raw, "excerpt", &req.Excerpt)
	stellarFormString(raw, "cover", &req.Cover)
	stellarFormString(raw, "description", &req.MetaDescription)
	stellarFormString(raw, "keywords", &req.MetaKeywords)
	stellarFormString(raw, "article.author", &req.Author)
	stellarFormString(raw, "article.ai_label", &req.AILabel)
	stellarFormString(blog, "content.banner_image", &req.Banner)
	if tags, exists := stellarFormStrings(raw, "tags"); exists {
		req.Tags = strings.Join(tags, ",")
	}
	if categories, exists := stellarFormStrings(raw, "categories"); exists {
		req.Categories = categories
		req.CategoryIDs = nil
		req.Category = ""
	}
	if _, exists := stellarFormValue(raw, "listing.priority"); exists {
		n := 0
		stellarFormInt(raw, "listing.priority", &n)
		req.Pinned = n > 0
	}
}
func projectPageForm(req *pageRequest) {
	if req.StellarVersion != 2 {
		return
	}
	raw := stellarFormObject(req.StellarConfig)
	stellarFormString(raw, "title", &req.Title)
}
