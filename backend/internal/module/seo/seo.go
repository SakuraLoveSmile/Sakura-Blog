// Package seo 实现「SEO 工具集」功能模块：站点体检、搜索引擎推送、
// 爬虫记录与搜索流量分析、搜索引擎验证。作为模块框架的第一个完整模块。
package seo

import (
	"github.com/gin-gonic/gin"

	"blog/internal/module"
)

// Module SEO 工具集模块。deps 在 RegisterRoutes/RegisterPublic 时注入并留存。
type Module struct {
	deps module.Deps
}

func init() {
	module.Register(&Module{})
}

// Meta 模块元信息。
func (m *Module) Meta() module.Meta {
	return module.Meta{
		ID:          "seo",
		Title:       "SEO 工具集",
		Description: "站点体检、搜索引擎推送、爬虫与搜索流量分析、站长验证",
		Category:    "工具",
		Nav: []module.Nav{
			{Path: "/seo", Label: "SEO 工具集", Icon: "ri:search-eye-line", Group: "系统"},
		},
		DefaultOn: true,
	}
}

// Migrate 建 SEO 相关表（爬虫日志、推送记录）。
func (m *Module) Migrate(deps module.Deps) error {
	if err := deps.Store.EnsureCrawlerLogsTable(); err != nil {
		return err
	}
	return deps.Store.EnsureSeoPushLogsTable()
}

// RegisterRoutes 挂 /api/v1/seo/* 后台 API（外层已包 Gate 中间件）。
func (m *Module) RegisterRoutes(authed *gin.RouterGroup, deps module.Deps) {
	m.deps = deps
	seo := authed.Group("/seo")
	m.registerSettings(seo)
	m.registerOverview(seo)
	m.registerAnalyze(seo)
	m.registerCrawler(seo)
	m.registerTraffic(seo)
	m.registerPush(seo)
}

// RegisterPublic 挂前台/公开路由：爬虫埋点中间件 + 启动时加载验证码注入。
func (m *Module) RegisterPublic(r *gin.Engine, deps module.Deps) {
	m.deps = deps
	// 启动时把已存验证码写入 site（前台 head 注入）
	m.applyVerifications(loadConfig(deps.Store))
	// 爬虫埋点中间件（内部按模块启停 + crawler.enabled 实时判定）
	r.Use(m.crawlerMiddleware(deps))
}
