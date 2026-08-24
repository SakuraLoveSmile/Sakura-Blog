// Package seo 实现「SEO 工具集」功能模块：站点体检、搜索引擎推送、
// 爬虫记录与搜索流量分析。搜索引擎验证码由站点级 SEO 设置（设置 → SEO）统一管理。
package seo

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"blog/internal/config"
	"blog/internal/module"
	"blog/internal/store"
)

// Module SEO 工具集模块。deps 在 RegisterRoutes/RegisterPublic 时注入并留存。
type Module struct {
	deps          module.Deps
	analyzeClient interface {
		Do(*http.Request) (*http.Response, error)
	}
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
			{Path: "/seo", Label: "SEO 工具集", Icon: "ri:search-eye-line", Group: "系统", Permissions: []string{store.PermissionSystemManage}},
		},
		DefaultOn: true,
	}
}

// Migrate 建 SEO 相关表（爬虫日志、推送记录），并迁移历史共用 "seo" 设置行。
func (m *Module) Migrate(deps module.Deps) error {
	if err := deps.Store.EnsureCrawlerLogsTable(); err != nil {
		return err
	}
	if err := deps.Store.EnsureSeoPushLogsTable(); err != nil {
		return err
	}
	// 历史数据迁移：工具集配置从共用 "seo" 行拆出；站点级 SEO 段被改动时重建站点
	if migrateLegacySeoRow(deps.Store) {
		if err := config.RebuildSite(deps.Store, deps.Site); err != nil {
			return err
		}
	}
	return nil
}

// RegisterRoutes 挂 /api/v1/seo/* 后台 API（外层已包 Gate 中间件），
// 并挂框架统一配置端点 GET/PUT /api/v1/settings/seo_tools（ConfigModule）。
func (m *Module) RegisterRoutes(authed *gin.RouterGroup, deps module.Deps) {
	m.deps = deps
	seo := authed.Group("/seo")
	m.registerSettings(seo)
	m.registerOverview(seo)
	m.registerAnalyze(seo)
	m.registerCrawler(seo)
	m.registerTraffic(seo)
	m.registerPush(seo)
	// 框架自动挂载模块配置端点（Gate 已由调用方包好）
	module.MountConfigRoutes(authed, deps, m)
}

// RegisterPublic 挂前台/公开路由：爬虫埋点中间件。
func (m *Module) RegisterPublic(r *gin.Engine, deps module.Deps) {
	m.deps = deps
	// 爬虫埋点中间件（内部按模块启停 + crawler.enabled 实时判定）
	r.Use(m.crawlerMiddleware(deps))
}

// —— 模块框架扩展接口（DependencyModule / LifecycleModule / ConfigModule）——

// Dependencies 声明模块依赖：SEO 工具集不依赖其他模块。
func (m *Module) Dependencies() []string { return nil }

// ConfigSection 配置命名空间 "seo_tools"：框架自动挂 GET/PUT /api/v1/settings/seo_tools。
// 不用 "seo"：该键已被站点级 SEO 段（config.SectionSEO）占用，静态路由会劫持分段保存。
func (m *Module) ConfigSection() string { return "seo_tools" }

// OnEnable 生命周期钩子：验证码已归属站点级 SEO 段，启停不再影响前台。
func (m *Module) OnEnable(deps module.Deps) error {
	m.deps = deps
	return nil
}

// OnDisable 生命周期钩子：爬虫埋点由中间件按启停实时停止，无需额外清理。
func (m *Module) OnDisable(deps module.Deps) error { return nil }
