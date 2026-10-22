package web

import (
	"blog/internal/settings"
	"blog/internal/stellarhtml"
)

// GeneratedStellarAssets 返回可信图标的公开派生资源；路径与stock deferred-icons一致。
// 调用者同时挂 /assets/js/icons/ 的实际路径和 /js/icons/ 官方别名。
func GeneratedStellarAssets(reg *settings.IconRegistry) ([]stellarhtml.Asset, error) {
	icons := map[string]string{}
	for _, key := range reg.Keys() {
		icons[key] = reg.Get(key)
	}
	assets, err := stellarhtml.IconAssets(icons)
	if err != nil {
		return nil, err
	}
	fallback, err := stellarhtml.ImageFallbackAsset(reg.Get("image:onerror"))
	if err != nil {
		return nil, err
	}
	if fallback != nil {
		assets = append(assets, *fallback)
	}
	return assets, nil
}
