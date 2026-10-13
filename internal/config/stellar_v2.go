package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"blog/internal/settings"
	"blog/internal/stellarv2"
	"blog/internal/store"
)

const (
	SectionStellarV2      = "stellar_v2"
	SectionBlogExtensions = "blog_extensions"
	StellarV2Version      = 2
)

// MigrationRequiredError 只携带位置与段名，不含配置值、凭据或内容。
// 启动/API 调用者应返回可定位诊断，不得转入旧配置或 recovery。
type MigrationRequiredError struct {
	Source   string   `json:"source"`
	Path     string   `json:"path"`
	Reason   string   `json:"reason"`
	Sections []string `json:"sections"`
}

func (e *MigrationRequiredError) Error() string {
	return fmt.Sprintf("Stellar v2 migration required: %s.%s: %s; settings sections: %s", e.Source, e.Path, e.Reason, strings.Join(e.Sections, ", "))
}

type stellarV2Document struct {
	Version any            `json:"version"`
	Theme   map[string]any `json:"theme"`
}

// ValidateStellarV2 复制并严格校验候选，不读写配置文件或数据库。
// Blog 是独立参数袋，只校验它可无损表示为 JSON 对象，不以主题 schema
// 检查本地扩展；功能特有的校验仍归各扩展的输入边界负责。
func ValidateStellarV2(theme, blog map[string]any) (*settings.StellarV2Config, error) {
	if theme == nil {
		return nil, fmt.Errorf("settings.%s.theme: expected object", SectionStellarV2)
	}
	raw, err := copyStellarV2Object(theme, "settings."+SectionStellarV2+".theme")
	if err != nil {
		return nil, err
	}
	parsed, err := stellarv2.ParseTheme(raw, stellarv2.Options{Source: "settings." + SectionStellarV2 + ".theme"})
	if err != nil {
		return nil, err
	}
	if blog == nil {
		blog = map[string]any{}
	}
	extensions, err := copyStellarV2Object(blog, "settings."+SectionBlogExtensions)
	if err != nil {
		return nil, err
	}
	return &settings.StellarV2Config{RawTheme: raw, Theme: parsed.Value, Blog: extensions}, nil
}

// LoadStellarV2 读取一个一致快照。未迁移的库只返回错误，不导入、补段、
// 写版本或回退默认配置。
func LoadStellarV2(st *store.Store) (*settings.StellarV2Config, error) {
	snapshot, err := st.ReadSettingsSnapshot()
	if err != nil {
		return nil, err
	}
	return LoadStellarV2Snapshot(snapshot)
}

// LoadStellarV2Snapshot 支持启动/热更新复用同一次完整读取的 settings。
func LoadStellarV2Snapshot(snapshot map[string]string) (*settings.StellarV2Config, error) {
	raw, exists := snapshot[SectionStellarV2]
	if !exists || strings.TrimSpace(raw) == "" {
		return nil, migrationRequired(snapshot, SectionStellarV2, "missing versioned theme document")
	}
	var document stellarV2Document
	if err := decodeStellarV2JSON([]byte(raw), &document, true); err != nil {
		return nil, fmt.Errorf("settings.%s: invalid document: %w", SectionStellarV2, err)
	}
	version, numeric := document.Version.(json.Number)
	versionNumber, versionError := version.Int64()
	if !numeric || versionError != nil || versionNumber != StellarV2Version {
		return nil, migrationRequired(snapshot, SectionStellarV2+".version", "expected migration version 2")
	}
	if document.Theme == nil {
		return nil, migrationRequired(snapshot, SectionStellarV2+".theme", "missing canonical theme object")
	}
	extensions, exists := snapshot[SectionBlogExtensions]
	if !exists || strings.TrimSpace(extensions) == "" {
		return nil, migrationRequired(snapshot, SectionBlogExtensions, "missing independent Blog extension object")
	}
	var blog map[string]any
	if err := decodeStellarV2JSON([]byte(extensions), &blog, false); err != nil || blog == nil {
		// 解码错误可能包含自定义 Unmarshal 的消息；不附带扩展的敏感原值。
		return nil, fmt.Errorf("settings.%s: expected JSON object", SectionBlogExtensions)
	}
	return ValidateStellarV2(document.Theme, blog)
}

// SaveStellarV2 只更新已迁移的库。候选先校验，事务内再次检查迁移门禁，
// 成功后返回对应保存快照，供调用者原子发布；旧段和其他业务段原样保留。
func SaveStellarV2(st *store.Store, theme, blog map[string]any) (*settings.StellarV2Config, error) {
	return writeStellarV2(st, theme, blog, false)
}

// InitializeStellarV2 仅显式初始化空 settings 库。调用者传入 {} 即选择
// 上游默认；任何非空库均须先生成并审核迁移候选，不能用初始化跳过迁移。
func InitializeStellarV2(st *store.Store, theme, blog map[string]any) (*settings.StellarV2Config, error) {
	return writeStellarV2(st, theme, blog, true)
}

func writeStellarV2(st *store.Store, theme, blog map[string]any, initialize bool) (*settings.StellarV2Config, error) {
	validated, err := ValidateStellarV2(theme, blog)
	if err != nil {
		return nil, err
	}
	document, err := json.Marshal(stellarV2Document{Version: StellarV2Version, Theme: validated.RawTheme})
	if err != nil {
		return nil, fmt.Errorf("encode Stellar v2 document: %w", err)
	}
	extensions, err := json.Marshal(validated.Blog)
	if err != nil {
		return nil, fmt.Errorf("encode Blog extensions")
	}
	updates := map[string]string{SectionStellarV2: string(document), SectionBlogExtensions: string(extensions)}
	err = st.UpdateSettingsSnapshot(updates, func(snapshot map[string]string) error {
		if initialize {
			if len(snapshot) != 0 {
				return migrationRequired(snapshot, SectionStellarV2, "initialization requires empty settings; existing data needs a reviewed migration")
			}
			return nil
		}
		_, err := LoadStellarV2Snapshot(snapshot)
		return err
	})
	if err != nil {
		return nil, err
	}
	return validated, nil
}

func migrationRequired(snapshot map[string]string, path, reason string) *MigrationRequiredError {
	sections := make([]string, 0, len(snapshot))
	for section := range snapshot {
		sections = append(sections, section)
	}
	sort.Strings(sections)
	return &MigrationRequiredError{Source: "settings", Path: path, Reason: reason, Sections: sections}
}

func copyStellarV2Object(input map[string]any, source string) (map[string]any, error) {
	encoded, err := json.Marshal(input)
	if err != nil {
		// 不暴露扩展参数值或用户类型的自定义序列化错误。
		return nil, fmt.Errorf("%s: expected JSON object", source)
	}
	var out map[string]any
	if err := decodeStellarV2JSON(encoded, &out, false); err != nil {
		return nil, fmt.Errorf("%s: expected JSON object", source)
	}
	return out, nil
}

func decodeStellarV2JSON(encoded []byte, target any, strict bool) error {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	if strict {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("expected one JSON document")
	}
	return nil
}
