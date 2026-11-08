package stellarv2

import (
	"net/url"
	"strings"
)

// CollectionBasePath 返回集合（文库、专栏、笔记本）的公开基础路径：带前导斜杠，无尾斜杠。
// 集合自己的 route.path 优先；否则取主题 profiles.<indexProfile>.path，再追加集合 ID。
// 前台路由与 Console 的“在博客中查看”共用这一份，避免两处各算一遍后漂移。
func CollectionBasePath(theme map[string]any, indexProfile, id, routePath string) string {
	if routePath != "" {
		return "/" + strings.Trim(routePath, "/")
	}
	var configured any
	if value, ok := Get(theme, "profiles."+indexProfile+".path"); ok {
		configured = value
	}
	index := "/" + ProfilePath(configured)
	return strings.TrimSuffix(index, "/") + "/" + url.PathEscape(id)
}
