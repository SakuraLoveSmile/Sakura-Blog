package stellarv2

// RegionOptions consumes parsed runtime camelCase configuration. Layers are
// applied in their supplied order: global, profile/default, collection, page.
// Catalog is the explicit Widget catalog, not a legacy Go widget configuration.
type RegionOptions struct {
	Profile      string
	DefaultState string
	Layers       []map[string]any
	Catalog      map[string]any
	BrandSources map[string]any
}

var regionIDs = []string{"topbar", "leftbar", "rightbar"}

// MergeBrand preserves null inheritance and false disabling. A declared source
// replaces the base brand; an image object merges each image field independently.
func MergeBrand(base, override any, sources map[string]any) any {
	if override == nil {
		return clone(base)
	}
	if override == false {
		return false
	}
	current, ok := regionObject(override)
	if !ok {
		return clone(base)
	}
	result := map[string]any{}
	requestedSource, _ := current["source"].(string)
	if source, exists := sources[requestedSource]; requestedSource != "" && exists {
		if sourceBrand, ok := regionObject(source); ok {
			result = clone(sourceBrand).(map[string]any)
		}
	} else if previous, ok := regionObject(base); ok {
		result = clone(previous).(map[string]any)
	}
	for key, value := range current {
		if image, ok := regionObject(value); key == "image" && ok {
			previous, _ := regionObject(result["image"])
			merged := map[string]any{}
			for k, v := range previous {
				merged[k] = clone(v)
			}
			for k, v := range image {
				merged[k] = clone(v)
			}
			result[key] = merged
		} else {
			result[key] = clone(value)
		}
	}
	return result
}

func regionObject(value any) (map[string]any, bool) { m, ok := value.(map[string]any); return m, ok }

func resolveRegion(layers []map[string]any, region string, sources map[string]any) map[string]any {
	result := map[string]any{"enabled": region != "topbar", "widgets": []any{}}
	if region == "topbar" || region == "leftbar" {
		result["brand"] = false
		result["menu"] = []any{}
	}
	if region == "leftbar" {
		result["menuColumns"] = 1
		result["footer"] = map[string]any{"actions": []any{}}
	}
	for _, layer := range layers {
		current, ok := regionObject(layer[region])
		if !ok {
			continue
		}
		if enabled, ok := current["enabled"].(bool); ok {
			result["enabled"] = enabled
		}
		if region == "topbar" || region == "leftbar" {
			if brand, exists := current["brand"]; exists {
				brandSources := sources
				if region == "topbar" {
					brandSources = nil
				}
				result["brand"] = MergeBrand(result["brand"], brand, brandSources)
			}
			if menu, ok := current["menu"].([]any); ok {
				result["menu"] = clone(menu)
			}
		}
		if region == "leftbar" {
			if columns := current["menuColumns"]; columns != nil {
				result["menuColumns"] = clone(columns)
			}
			footer, _ := regionObject(current["footer"])
			if actions, ok := footer["actions"].([]any); ok {
				result["footer"].(map[string]any)["actions"] = clone(actions)
			}
		}
		if widgets, ok := current["widgets"].([]any); ok {
			result["widgets"] = clone(widgets)
		}
	}
	return result
}

// CascadeRegions resolves fixed fields and raw Widget lists without expanding
// instances. Empty arrays replace inherited lists; absent or null arrays inherit.
func CascadeRegions(layers []map[string]any, brandSources map[string]any) map[string]any {
	result := map[string]any{}
	for _, region := range regionIDs {
		result[region] = resolveRegion(layers, region, brandSources)
	}
	return result
}

// ResolveRegions expands Widgets only in enabled regions. Fixed brand/menu/footer
// fields remain available even when a region is disabled, as in upstream.
func ResolveRegions(options RegionOptions) map[string]any {
	result := map[string]any{}
	warnings := []any{}
	for _, region := range regionIDs {
		state := resolveRegion(options.Layers, region, options.BrandSources)
		widgets := state["widgets"]
		if state["enabled"] == false {
			widgets = []any{}
		}
		resolved := ResolveRegionWidgets(widgets, options.Catalog, WidgetOptions{Region: region, Profile: options.Profile, ContentOnly: region == "leftbar"})
		current := map[string]any{"widgets": resolved["instances"], "enabled": state["enabled"]}
		if region == "topbar" || region == "leftbar" {
			current["brand"] = clone(state["brand"])
			current["menu"] = clone(state["menu"])
		}
		if region == "leftbar" {
			defaultState := options.DefaultState
			if defaultState == "" {
				defaultState = "expanded"
			}
			current["menuColumns"] = state["menuColumns"]
			current["defaultState"] = defaultState
			current["footer"] = clone(state["footer"])
		}
		result[region] = current
		warnings = append(warnings, resolved["warnings"].([]any)...)
	}
	result["warnings"] = warnings
	return result
}
