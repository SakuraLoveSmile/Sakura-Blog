package settings

import (
	"strings"
	"testing"
)

// TestIconRegistryLegacyAlias 旧键（settings 表历史持久化值）经别名表回退到新键的 SVG，
// 而不是渲染 icon-missing 占位。
func TestIconRegistryLegacyAlias(t *testing.T) {
	reg, err := LoadIconsEmbed()
	if err != nil {
		t.Fatalf("LoadIconsEmbed: %v", err)
	}
	for oldKey, newKey := range legacyIconAlias {
		got := reg.Get(oldKey)
		if strings.Contains(got, "icon-missing") {
			t.Errorf("旧键 %q 命中占位，别名 %q 未生效", oldKey, newKey)
		}
		if want := reg.Get(newKey); got != want {
			t.Errorf("旧键 %q 渲染结果与新键 %q 不一致", oldKey, newKey)
		}
	}
	// 未在别名表中的未知键仍走占位兜底（防注入转义路径不回归）
	if got := reg.Get("not:a-key"); !strings.Contains(got, "icon-missing") {
		t.Errorf("未知键应命中占位，got: %s", got)
	}
}
