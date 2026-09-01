package config

import (
	"encoding/json"
	"fmt"

	tmpl "blog/internal/settings"
	"blog/internal/store"
)

// backfillRuntimeSettings upgrades the settings sections whose shape changed
// with Stellar 1.41/1.44. Existing values are decoded first, then only missing
// fields receive defaults; the normalized JSON is written back for old sites.
func backfillRuntimeSettings(st *store.Store, settings map[string]string, site *tmpl.Site) error {
	sections := []struct {
		name     string
		fallback any
		newValue func() any
	}{
		{name: SectionSearch, fallback: site.Search, newValue: func() any { return &tmpl.SearchConfig{} }},
		{name: SectionDataCache, fallback: site.DataCache, newValue: func() any { return &tmpl.DataCacheConfig{} }},
		{name: SectionTagPlugins, fallback: site.TagPlugins, newValue: func() any { return &tmpl.TagPluginsConfig{} }},
		{name: SectionServices, fallback: site.Services, newValue: func() any { return &tmpl.ServicesConfig{} }},
	}

	for _, section := range sections {
		raw, exists := settings[section.name]
		if !exists || raw == "" {
			value, err := json.Marshal(section.fallback)
			if err != nil {
				return fmt.Errorf("marshal default %s settings: %w", section.name, err)
			}
			if err := st.PutSetting(section.name, string(value)); err != nil {
				return fmt.Errorf("backfill %s settings: %w", section.name, err)
			}
			settings[section.name] = string(value)
			continue
		}

		dst := section.newValue()
		if err := json.Unmarshal([]byte(raw), dst); err != nil {
			return fmt.Errorf("parse stored %s settings: %w", section.name, err)
		}
		switch value := dst.(type) {
		case *tmpl.SearchConfig:
			value.FillDefaults()
		case *tmpl.DataCacheConfig:
			value.FillDefaults()
		case *tmpl.TagPluginsConfig:
			value.FillDefaults()
		}
		normalized, err := json.Marshal(dst)
		if err != nil {
			return fmt.Errorf("marshal normalized %s settings: %w", section.name, err)
		}
		value, err := mergeRuntimeDefaults([]byte(raw), normalized, section.name)
		if err != nil {
			return fmt.Errorf("merge normalized %s settings: %w", section.name, err)
		}
		if string(value) != raw {
			if err := st.PutSetting(section.name, string(value)); err != nil {
				return fmt.Errorf("save normalized %s settings: %w", section.name, err)
			}
		}
		settings[section.name] = string(value)
	}
	return nil
}

// mergeRuntimeDefaults adds normalized fields without discarding unknown keys.
// Settings are also an escape hatch for supported-but-not-yet-structured fields,
// so migration must preserve those keys while filling only absent defaults.
func mergeRuntimeDefaults(raw, normalized []byte, section string) ([]byte, error) {
	var current map[string]json.RawMessage
	if err := json.Unmarshal(raw, &current); err != nil || current == nil {
		return normalized, nil
	}

	// Build defaults from a zero-value config rather than the already decoded
	// payload. The latter cannot distinguish an omitted nested key from an
	// explicitly configured empty string, so it would turn a missing suffix or
	// status label into a permanent empty value during migration.
	defaultsJSON := normalized
	if generated, err := runtimeDefaultsJSON(section); err == nil {
		defaultsJSON = generated
	}
	var defaults map[string]json.RawMessage
	if err := json.Unmarshal(defaultsJSON, &defaults); err != nil || defaults == nil {
		return normalized, nil
	}
	if section == SectionTagPlugins {
		migrateLegacyQuotKeys(current)
	}
	mergeJSONObjects(current, defaults)
	return json.Marshal(current)
}

// runtimeDefaultsJSON returns a fully populated canonical shape for the
// runtime sections. It is used only as a source of missing keys; values from
// the stored payload always win, including false, zero and empty strings.
func runtimeDefaultsJSON(section string) ([]byte, error) {
	var value any
	switch section {
	case SectionSearch:
		cfg := &tmpl.SearchConfig{}
		cfg.FillDefaults()
		value = cfg
	case SectionDataCache:
		cfg := &tmpl.DataCacheConfig{}
		cfg.FillDefaults()
		value = cfg
	case SectionTagPlugins:
		cfg := &tmpl.TagPluginsConfig{}
		cfg.FillDefaults()
		value = cfg
	case SectionServices:
		value = &tmpl.ServicesConfig{}
	default:
		return nil, fmt.Errorf("unknown runtime defaults section %q", section)
	}
	return json.Marshal(value)
}

func mergeJSONObjects(current, defaults map[string]json.RawMessage) {
	for key, defaultValue := range defaults {
		value, exists := current[key]
		if !exists {
			current[key] = defaultValue
			continue
		}
		var currentObject, defaultObject map[string]json.RawMessage
		if json.Unmarshal(value, &currentObject) == nil && currentObject != nil &&
			json.Unmarshal(defaultValue, &defaultObject) == nil && defaultObject != nil {
			mergeJSONObjects(currentObject, defaultObject)
			if merged, err := json.Marshal(currentObject); err == nil {
				current[key] = merged
			}
		}
	}
}

func migrateLegacyQuotKeys(settings map[string]json.RawMessage) {
	var quot map[string]json.RawMessage
	if raw, ok := settings["quot"]; !ok || json.Unmarshal(raw, &quot) != nil || quot == nil {
		return
	}
	// prefix/suffix were the pre-1.44 flat shape. Preserve their values under
	// quot.default unless the new nested preset already exists, then remove
	// only these known legacy keys.
	if _, hasDefault := quot["default"]; !hasDefault {
		preset := make(map[string]json.RawMessage, 2)
		if prefix, ok := quot["prefix"]; ok {
			preset["prefix"] = prefix
		}
		if suffix, ok := quot["suffix"]; ok {
			preset["suffix"] = suffix
		}
		if len(preset) > 0 {
			if merged, err := json.Marshal(preset); err == nil {
				quot["default"] = merged
			}
		}
	}
	delete(quot, "prefix")
	delete(quot, "suffix")
	if merged, err := json.Marshal(quot); err == nil {
		settings["quot"] = merged
	}
}
