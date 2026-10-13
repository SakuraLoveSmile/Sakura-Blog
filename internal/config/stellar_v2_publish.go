package config

import (
	"fmt"
	"sync/atomic"

	"blog/internal/settings"
	"blog/internal/store"
)

const StellarV2SecretMask = "****"

// 只屏蔽本地功能的已知私密参数，不能按字段名扫描上游或开放参数袋。
var stellarV2SecretPaths = [][]string{
	{"native_comments", "notify", "password"},
	{"comments", "notify", "password"},
	{"notify", "password"},
	{"ai", "api_key"},
}

// MaskStellarV2Blog 返回脱敏的独立副本，不修改前台正在读取的配置。
func MaskStellarV2Blog(blog map[string]any) (map[string]any, error) {
	masked, err := copyStellarV2Object(blog, "settings."+SectionBlogExtensions)
	if err != nil {
		return nil, err
	}
	for _, path := range stellarV2SecretPaths {
		parent, exists := stellarV2SecretParent(masked, path)
		if !exists {
			continue
		}
		key := path[len(path)-1]
		value, exists := parent[key]
		if exists && value != nil && value != "" {
			parent[key] = StellarV2SecretMask
		}
	}
	return masked, nil
}

// SaveAndPublishStellarV2 把候选保存和指针发布放在共享的 siteUpdateMu 内。
// 与旧 SaveSection 串行，避免保存成功的两个请求乱序发布旧候选。
// 同时发布本地服务的 typed bridge，候选失败不修改数据库或指针。
func SaveAndPublishStellarV2(st *store.Store, site *atomic.Pointer[settings.Site], theme, blog map[string]any) (*settings.StellarV2Config, error) {
	candidate, err := ValidateStellarV2(theme, blog)
	if err != nil {
		return nil, err
	}
	siteUpdateMu.Lock()
	defer siteUpdateMu.Unlock()
	if site == nil || site.Load() == nil {
		return nil, fmt.Errorf("site snapshot is nil")
	}
	current, err := LoadStellarV2(st)
	if err != nil {
		return nil, err
	}
	for _, path := range stellarV2SecretPaths {
		parent, exists := stellarV2SecretParent(candidate.Blog, path)
		if !exists || parent[path[len(path)-1]] != StellarV2SecretMask {
			continue
		}
		stored, exists := stellarV2SecretParent(current.Blog, path)
		key := path[len(path)-1]
		if !exists {
			return nil, fmt.Errorf("settings.%s: masked secret has no stored value", SectionBlogExtensions)
		}
		value, exists := stored[key]
		if !exists {
			return nil, fmt.Errorf("settings.%s: masked secret has no stored value", SectionBlogExtensions)
		}
		parent[key] = value
	}
	next := *site.Load()
	next.StellarV2 = candidate
	if err := ApplyStellarBlogRuntime(&next); err != nil {
		return nil, err
	}
	saved, err := SaveStellarV2(st, candidate.RawTheme, candidate.Blog)
	if err != nil {
		return nil, err
	}
	next.StellarV2 = saved
	site.Store(&next)
	return saved, nil
}

func stellarV2SecretParent(blog map[string]any, path []string) (map[string]any, bool) {
	parent := blog
	for _, key := range path[:len(path)-1] {
		next, ok := parent[key].(map[string]any)
		if !ok {
			return nil, false
		}
		parent = next
	}
	return parent, true
}
