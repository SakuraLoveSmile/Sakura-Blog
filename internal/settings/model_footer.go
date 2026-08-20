package settings

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
)

// SocialItem is the normalized footer social model. Social accepts both the
// historical ordered list and Stellar's mapping form; dropdown children use
// the same model recursively.
type SocialItem struct {
	ID      string       `yaml:"id,omitempty" json:"id,omitempty"`
	Type    string       `yaml:"type,omitempty" json:"type,omitempty"`
	Spacer  bool         `yaml:"spacer,omitempty" json:"spacer,omitempty"`
	Title   string       `yaml:"title,omitempty" json:"title,omitempty"`
	URL     string       `yaml:"url,omitempty" json:"url,omitempty"`
	Icon    string       `yaml:"icon,omitempty" json:"icon,omitempty"` // legacy raw HTML
	IconKey string       `yaml:"icon_key,omitempty" json:"icon_key,omitempty"`
	OnClick string       `yaml:"onclick,omitempty" json:"onclick,omitempty"`
	Items   []SocialItem `yaml:"items,omitempty" json:"items,omitempty"`
}

func (s SocialItem) IsSpacer() bool {
	return s.Spacer || strings.EqualFold(strings.TrimSpace(s.Type), "spacer") || s.ID == "spacer"
}

func (s SocialItem) IsDropdown() bool {
	return strings.EqualFold(strings.TrimSpace(s.Type), "dropdown") || len(s.Items) > 0
}

func (s SocialItem) External() bool {
	return strings.Contains(s.URL, "://")
}

func (s SocialItem) Target() string {
	if s.External() {
		return "_blank"
	}
	return ""
}

func (s SocialItem) Rel() string {
	if s.External() {
		return "external nofollow noopener noreferrer"
	}
	return "noopener noreferrer"
}

func (s SocialItem) HasIcon() bool { return s.IconKey != "" || s.Icon != "" }

func (s SocialItem) Renderable() bool {
	return !s.IsSpacer() && (s.URL != "" || s.OnClick != "" || s.IsDropdown())
}

// NormalizeSocialItem is also useful for programmatic DefaultSite values.
func NormalizeSocialItem(item SocialItem) SocialItem {
	if item.IconKey == "" && item.Icon != "" && !strings.HasPrefix(strings.TrimSpace(item.Icon), "<") {
		item.IconKey = item.Icon
		item.Icon = ""
	}
	for i := range item.Items {
		item.Items[i] = NormalizeSocialItem(item.Items[i])
	}
	return item
}

// UnmarshalYAML preserves mapping order by walking the goccy AST instead of
// decoding social into a Go map. Other Footer fields retain ordinary decoding.
func (f *Footer) UnmarshalYAML(node ast.Node) error {
	b, err := node.MarshalYAML()
	if err != nil {
		return err
	}
	var raw struct {
		Sitemap []SitemapGroup `yaml:"sitemap"`
		Beian   string         `yaml:"beian"`
		Content string         `yaml:"content"`
	}
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return err
	}
	f.Sitemap, f.Beian, f.Content = raw.Sitemap, raw.Beian, raw.Content
	f.Social = nil
	if mapping, ok := node.(ast.MapNode); ok {
		iter := mapping.MapRange()
		for iter.Next() {
			key := scalarString(iter.Key())
			if key != "social" {
				continue
			}
			f.Social, err = parseYAMLSocial(iter.Value())
			return err
		}
	}
	return nil
}

func parseYAMLSocial(node ast.Node) ([]SocialItem, error) {
	if node == nil {
		return nil, nil
	}
	if seq, ok := node.(ast.ArrayNode); ok {
		out := make([]SocialItem, 0, seq.ArrayRange().Len())
		it := seq.ArrayRange()
		for it.Next() {
			item, err := decodeSocialNode(it.Value())
			if err != nil {
				return nil, err
			}
			if item.IsSpacer() || item.Renderable() {
				out = append(out, NormalizeSocialItem(item))
			}
		}
		return out, nil
	}
	if mapping, ok := node.(ast.MapNode); ok {
		out := make([]SocialItem, 0)
		it := mapping.MapRange()
		for it.Next() {
			id := scalarString(it.Key())
			if id == "" {
				continue
			}
			if strings.EqualFold(id, "spacer") {
				out = append(out, SocialItem{ID: "spacer", Spacer: true})
				continue
			}
			item, err := decodeSocialNode(it.Value())
			if err != nil {
				return nil, err
			}
			item.ID = id
			if item.IsSpacer() || item.Renderable() {
				out = append(out, NormalizeSocialItem(item))
			}
		}
		return out, nil
	}
	return nil, nil
}

func decodeSocialNode(node ast.Node) (SocialItem, error) {
	b, err := node.MarshalYAML()
	if err != nil {
		return SocialItem{}, err
	}
	var raw map[string]any
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return SocialItem{}, err
	}
	return socialItemFromMap(raw), nil
}

func socialItemFromMap(raw map[string]any) SocialItem {
	item := SocialItem{
		Type:    anyString(raw["type"]),
		Spacer:  anyBool(raw["spacer"]),
		Title:   anyString(raw["title"]),
		URL:     anyString(raw["url"]),
		OnClick: anyString(raw["onclick"]),
	}
	icon := anyString(raw["icon"])
	if strings.HasPrefix(strings.TrimSpace(icon), "<") {
		item.Icon = icon
	} else {
		item.IconKey = icon
	}
	if key := anyString(raw["icon_key"]); key != "" {
		item.IconKey = key
	}
	if children, ok := raw["items"].([]any); ok {
		for _, child := range children {
			if m, ok := child.(map[string]any); ok {
				item.Items = append(item.Items, socialItemFromMap(m))
			}
		}
	}
	return item
}

func scalarString(key ast.MapKeyNode) string {
	if scalar, ok := key.(ast.ScalarNode); ok {
		return fmt.Sprint(scalar.GetValue())
	}
	return strings.Trim(strings.TrimSpace(key.String()), "\"'")
}

func anyString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func anyBool(v any) bool {
	b, _ := v.(bool)
	return b
}

// UnmarshalJSON mirrors the YAML compatibility while retaining object order.
func (f *Footer) UnmarshalJSON(data []byte) error {
	var raw struct {
		Social  json.RawMessage `json:"social"`
		Sitemap []SitemapGroup  `json:"sitemap"`
		Beian   string          `json:"beian"`
		Content string          `json:"content"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	f.Sitemap, f.Beian, f.Content = raw.Sitemap, raw.Beian, raw.Content
	var err error
	f.Social, err = parseJSONSocial(raw.Social)
	return err
}

func parseJSONSocial(data []byte) ([]SocialItem, error) {
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	if data[0] == '[' {
		var entries []json.RawMessage
		if err := json.Unmarshal(data, &entries); err != nil {
			return nil, err
		}
		out := make([]SocialItem, 0, len(entries))
		for _, entry := range entries {
			item, err := decodeJSONSocial(entry)
			if err != nil {
				return nil, err
			}
			if item.IsSpacer() || item.Renderable() {
				out = append(out, NormalizeSocialItem(item))
			}
		}
		return out, nil
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	var out []SocialItem
	for dec.More() {
		keyToken, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := keyToken.(string)
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		if strings.EqualFold(key, "spacer") {
			out = append(out, SocialItem{ID: "spacer", Spacer: true})
			continue
		}
		item, err := decodeJSONSocial(value)
		if err != nil {
			return nil, err
		}
		item.ID = key
		if item.IsSpacer() || item.Renderable() {
			out = append(out, NormalizeSocialItem(item))
		}
	}
	return out, nil
}

func decodeJSONSocial(data []byte) (SocialItem, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return SocialItem{}, err
	}
	item := SocialItem{}
	decode := func(key string, dst any) {
		if b, ok := raw[key]; ok {
			_ = json.Unmarshal(b, dst)
		}
	}
	decode("type", &item.Type)
	decode("spacer", &item.Spacer)
	decode("title", &item.Title)
	decode("url", &item.URL)
	decode("onclick", &item.OnClick)
	decode("icon_key", &item.IconKey)
	var icon string
	decode("icon", &icon)
	if strings.HasPrefix(strings.TrimSpace(icon), "<") {
		item.Icon = icon
	} else if item.IconKey == "" {
		item.IconKey = icon
	}
	var children []json.RawMessage
	if b, ok := raw["items"]; ok && json.Unmarshal(b, &children) == nil {
		for _, child := range children {
			parsed, err := decodeJSONSocial(child)
			if err != nil {
				return SocialItem{}, err
			}
			item.Items = append(item.Items, parsed)
		}
	}
	return item, nil
}
