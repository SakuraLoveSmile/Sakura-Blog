package store

import (
	"encoding/json"
	"strings"
)

// LinkLabel 友链角标：文案 + 颜色（6 位 hex，不带 #）。
// 第一条角标仍存 links.label/color（linkwatch 的自动「失效」标签与旧客户端依赖它们），
// 第 2、3 条以 JSON 数组存 links.labels。
type LinkLabel struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

const (
	// MaxLinkLabels 单条友链最多角标数（含主角标）。
	MaxLinkLabels = 3
	// MaxLinkLabelNameRunes 单个角标文案上限（字符），避免角标撑破卡片。
	MaxLinkLabelNameRunes = 12
)

// NormalizeLinkColor 归一化角标颜色：去 #、转小写、3 位补成 6 位；非 hex 返回空串。
func NormalizeLinkColor(raw string) string {
	c := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(raw), "#"))
	switch {
	case len(c) == 6 && isHexColor(c):
		return c
	case len(c) == 3 && isHexColor(c):
		return string([]byte{c[0], c[0], c[1], c[1], c[2], c[2]})
	}
	return ""
}

func isHexColor(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// NormalizeLinkLabels 去掉空文案、归一化颜色并保留顺序；全空时返回 nil。
func NormalizeLinkLabels(in []LinkLabel) []LinkLabel {
	var out []LinkLabel
	for _, l := range in {
		name := strings.TrimSpace(l.Name)
		if name == "" {
			continue
		}
		out = append(out, LinkLabel{Name: name, Color: NormalizeLinkColor(l.Color)})
	}
	return out
}

// ParseLinkLabels 解析 labels 列的 JSON；坏数据按空处理，最多返回 MaxLinkLabels 枚。
func ParseLinkLabels(raw string) []LinkLabel {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []LinkLabel
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	out = NormalizeLinkLabels(out)
	if len(out) > MaxLinkLabels {
		out = out[:MaxLinkLabels]
	}
	return out
}

// EncodeLinkLabels 序列化附加角标；空列表写空串，便于列默认值比较。
func EncodeLinkLabels(labels []LinkLabel) string {
	labels = NormalizeLinkLabels(labels)
	if len(labels) == 0 {
		return ""
	}
	b, err := json.Marshal(labels)
	if err != nil {
		return ""
	}
	return string(b)
}

// ExtraLabels 解析附加角标（不含主角标 label/color）。
func (l *Link) ExtraLabels() []LinkLabel {
	if l == nil {
		return nil
	}
	return ParseLinkLabels(l.LabelsJSON)
}
