package web

import (
	"strings"
	"testing"
)

// TestLoadIconsEmbed141 新 icons.yml（1.41.0 语义化命名空间）加载后关键键存在。
func TestLoadIconsEmbed141(t *testing.T) {
	reg, err := LoadIconsEmbed()
	if err != nil {
		t.Fatalf("LoadIconsEmbed: %v", err)
	}
	for _, key := range []string{
		"default:documents", "default:category", "default:pin",
		"default:arrow-left", "default:arrow-right", "default:bookmark.active",
		"example:notebook", "example:planet", "example:chat",
		"quot:hashtag", "quot:quote-left", "quot:quote-right", "quot:question",
	} {
		if !reg.Has(key) {
			t.Errorf("icons.yml 缺少 1.41 键 %q（Len=%d）", key, reg.Len())
		}
	}
	// 迁移后主表不应再含 solar:/ph:/bxs: 前缀键
	for _, key := range reg.Keys() {
		if strings.HasPrefix(key, "solar:") || strings.HasPrefix(key, "ph:") || strings.HasPrefix(key, "bxs:") {
			t.Errorf("icons.yml 仍含旧命名空间键 %q", key)
		}
	}
}
