package stellarv2

import (
	"fmt"
	"math"
	"strconv"
	"unicode/utf16"
)

// WidgetOptions selects the destination region and its profile diagnostic source.
// ContentOnly suppresses menu/settings in the fixed-content Leftbar shell.
type WidgetOptions struct {
	Region      string
	Profile     string
	ContentOnly bool
}

var widgetPresentations = map[string][]any{
	"menu":     {"topbar", "leftbar", "leftbarRail", "drawer"},
	"settings": {"topbar", "leftbar", "leftbarRail", "drawer"},
	"spacer":   {"topbar"},
	"toc":      {"topbar", "leftbar", "rightbar", "drawer"},
	"tree":     {"leftbar", "leftbarRail", "rightbar", "drawer"},
	"tagtree":  {"leftbar", "rightbar", "drawer"},
	"linklist": {"leftbar", "leftbarRail", "rightbar", "drawer"},
	"recent":   {"leftbar", "rightbar", "drawer"},
	"related":  {"leftbar", "leftbarRail", "rightbar", "drawer"},
	"ghrepo":   {"leftbar", "rightbar", "drawer"},
	"ghissues": {"leftbar", "rightbar", "drawer"},
	"ghuser":   {"leftbar", "rightbar", "drawer"},
	"author":   {"leftbar", "rightbar", "drawer"},
	"timeline": {"leftbar", "rightbar", "drawer"},
	"markdown": {"leftbar", "rightbar", "drawer"},
}

// MergeWidgetCatalog 对应官方 Doctor 的 theme/site 组件描述合并。
// null 和长度为零的值删除组件，其他描述按 Object.assign 浅覆盖。
func MergeWidgetCatalog(base, overrides map[string]any) map[string]any {
	result := assignWidget(base)
	for key, value := range overrides {
		empty := value == nil
		switch x := value.(type) {
		case []any:
			empty = len(x) == 0
		case string:
			empty = len(x) == 0
		case map[string]any:
			if n, ok := number(x["length"]); ok && n == 0 {
				empty = true
			}
		}
		if empty {
			delete(result, key)
			continue
		}
		merged := assignWidget(result[key])
		for field, value := range assignWidget(value) {
			merged[field] = value
		}
		result[key] = merged
	}
	return result
}

func jsTruthy(value any) bool {
	if value == nil {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v != ""
	}
	if n, ok := number(value); ok {
		return n != 0 && !math.IsNaN(n)
	}
	return true
}

// Widget catalogs are open YAML records. Object.assign also enumerates arrays
// and strings in upstream; preserve that behavior without a second whitelist.
func assignWidget(value any) map[string]any {
	result := map[string]any{}
	switch v := value.(type) {
	case map[string]any:
		for key, value := range v {
			result[key] = clone(value)
		}
	case []any:
		for i, item := range v {
			result[strconv.Itoa(i)] = clone(item)
		}
	case string:
		for i, unit := range utf16.Encode([]rune(v)) {
			result[strconv.Itoa(i)] = string(utf16.Decode([]uint16{unit}))
		}
	}
	return result
}

func resolveWidget(specification any, catalog map[string]any, options WidgetOptions, index int) map[string]any {
	id := ""
	hasID := false
	var override map[string]any
	hasOverride := false
	switch value := specification.(type) {
	case string:
		id = value
		hasID = true
	case map[string]any:
		override = value
		hasOverride = true
		id, hasID = value["override"].(string)
	case []any:
		override = assignWidget(value)
		hasOverride = true
	}
	var registered any
	if hasID {
		if includes([]string{"menu", "settings", "spacer"}, id) {
			registered = map[string]any{"layout": id, "system": true}
		} else if value := catalog[id]; jsTruthy(value) {
			registered = value
		}
	}
	descriptor := assignWidget(registered)
	for key, value := range override {
		descriptor[key] = clone(value)
	}
	layout, hasLayout := descriptor["layout"].(string)
	if !hasLayout {
		layout = id
	}
	widget := id
	if widget == "" {
		widget = layout
	}
	unsupported := func(code string, supported []any) map[string]any {
		warning := map[string]any{"code": code, "widget": widget, "layout": layout, "supported": clone(supported), "region": options.Region}
		return map[string]any{"instance": nil, "warning": warning}
	}
	if layout == "brand" || layout == "actions" {
		return unsupported("fixed_region_content", []any{})
	}
	if layout == "" || (!jsTruthy(registered) && !hasOverride) {
		if id == "" {
			widget = "<invalid>"
		} else {
			widget = id
		}
		if layout == "" {
			layout = "<unknown>"
		}
		return unsupported("unknown_widget", []any{})
	}
	if options.Region == "leftbar" && options.ContentOnly && (layout == "menu" || layout == "settings") {
		return unsupported("fixed_leftbar_widget", []any{"topbar"})
	}
	presentations := widgetPresentations[layout]
	if configured, ok := assignWidget(registered)["presentations"].([]any); ok {
		presentations = configured
	} else if presentations == nil {
		presentations = []any{"leftbar", "rightbar", "drawer"}
	}
	if !includes(regionIDs, options.Region) || !valuesInclude(presentations, options.Region) {
		return unsupported("unsupported_widget_presentation", presentations)
	}
	if id == "" {
		id = layout
	}
	zone := "body"
	if layout == "settings" {
		zone = "system"
	}
	instance := map[string]any{
		"instanceId": fmt.Sprintf("%s:%d:%s", options.Region, index, id), "id": id, "layout": layout,
		"system": descriptor["system"] == true, "presentation": options.Region, "presentations": clone(presentations),
		"leftbarZone": zone, "item": clone(descriptor),
	}
	return map[string]any{"instance": instance, "warning": nil}
}

// ResolveRegionWidgets preserves order, duplicates and original list positions.
// Registered capabilities are authoritative; inline presentations cannot elevate
// a Widget's destination capabilities. Unusable instances produce warnings.
func ResolveRegionWidgets(widgets any, catalog map[string]any, options WidgetOptions) map[string]any {
	instances, warnings := []any{}, []any{}
	items, _ := clone(widgets).([]any)
	for index, specification := range items {
		result := resolveWidget(specification, catalog, options, index)
		if instance := result["instance"]; instance != nil {
			instances = append(instances, instance)
		}
		if warning, ok := result["warning"].(map[string]any); ok {
			var profile any
			if options.Profile != "" {
				profile = options.Profile
			}
			warning["profile"] = profile
			warnings = append(warnings, warning)
		}
	}
	return map[string]any{"instances": instances, "warnings": warnings}
}
