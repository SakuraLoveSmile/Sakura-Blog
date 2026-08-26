// Package core publishes the Console's always-on frontend manifest entry.
package core

import (
	"github.com/gin-gonic/gin"

	"blog/internal/module"
	"blog/internal/store"
)

type Module struct{}

func init() { module.Register(&Module{}) }

func (m *Module) Meta() module.Meta {
	admin := []string{store.PermissionSystemManage}
	return module.Meta{
		ID:          "core",
		Title:       "Console 核心",
		Description: "Console 基础内容、互动、媒体和系统管理入口",
		Category:    "系统",
		DefaultOn:   true,
		Locked:      true,
		Nav: []module.Nav{
			{Path: "/dashboard", Label: "仪表盘", Icon: "ri:dashboard-3-line", Group: "", Mobile: true},
			{Path: "/posts", Label: "文章", Icon: "ri:book-read-line", Group: "内容", Mobile: true, Children: []module.Nav{{Path: "/posts/shares", Label: "分享链接", Icon: "ri:share-forward-line", Group: "内容"}}},
			{Path: "/topics", Label: "专栏", Icon: "ri:book-shelf-line", Group: "内容", Mobile: true},
			{Path: "/pages", Label: "页面", Icon: "ri:pages-line", Group: "内容", Mobile: true},
			{Path: "/wiki", Label: "文库", Icon: "ri:book-2-line", Group: "内容", Children: []module.Nav{{Path: "/wiki/projects", Label: "项目管理", Icon: "ri:stack-line", Group: "内容"}}},
			{Path: "/notebooks", Label: "笔记本", Icon: "ri:book-open-line", Group: "内容"},
			{Path: "/comments", Label: "评论", Icon: "ri:chat-3-line", Group: "互动", Mobile: true},
			{Path: "/comment-users", Label: "评论用户", Icon: "ri:user-community-line", Group: "互动", Permissions: admin},
			{Path: "/notifications", Label: "通知中心", Icon: "ri:notification-3-line", Group: "互动"},
			{Path: "/friends", Label: "友链", Icon: "ri:links-line", Group: "互动"},
			{Path: "/subscriptions", Label: "订阅", Icon: "ri:rss-line", Group: "互动"},
			{Path: "/attachments", Label: "附件", Icon: "ri:folder-2-line", Group: "媒体", Mobile: true},
			{Path: "/taxonomy", Label: "分类标签", Icon: "ri:price-tag-3-line", Group: "系统"},
			{Path: "/users", Label: "用户", Icon: "ri:user-star-line", Group: "系统", Permissions: admin},
			{Path: "/login-logs", Label: "登录日志", Icon: "ri:shield-user-line", Group: "系统", Permissions: admin},
			{Path: "/plugins", Label: "插件", Icon: "ri:puzzle-line", Group: "系统", Permissions: admin},
			{Path: "/modules", Label: "模块", Icon: "ri:apps-2-line", Group: "系统", Permissions: admin},
			{Path: "/settings", Label: "设置", Icon: "ri:settings-3-line", Group: "系统", Permissions: admin},
			{Path: "/backup", Label: "备份", Icon: "ri:archive-line", Group: "系统", Permissions: admin},
			{Path: "/system-update", Label: "系统更新", Icon: "ri:rocket-2-line", Group: "系统", Permissions: admin},
			{Path: "/assistant", Label: "AI 助手", Icon: "ri:sparkling-2-line", Group: "工具", Permissions: admin},
		},
	}
}

func (m *Module) Migrate(module.Deps) error                    { return nil }
func (m *Module) RegisterRoutes(*gin.RouterGroup, module.Deps) {}
func (m *Module) RegisterPublic(*gin.Engine, module.Deps)      {}
