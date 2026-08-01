// Package module 定义博客的编译期「功能模块」框架。
//
// 设计取舍：本项目是单 Go 二进制 + go:embed 打包的 Vue SPA，不适合做 Halo 那种
// 第三方 JAR 热插拔。这里提供的是编译期模块注册表 + 运行时 DB 启停开关：
// 各功能以 Module 形式在 init() 里自注册，main 通过 blank import 引入；
// 启停状态存 SQLite，Console 可实时切换（控制菜单显隐与后端 API 可用性）。
package module

import (
	"github.com/gin-gonic/gin"

	"blog/internal/post"
	"blog/internal/store"
	tmpl "blog/internal/template"
)

// Nav 模块贡献的 Console 侧栏菜单项。
type Nav struct {
	Path  string `json:"path"`  // Console 路由，如 /seo
	Label string `json:"label"` // 菜单文案
	Icon  string `json:"icon"`  // iconify 图标名
	Group string `json:"group"` // 侧栏分组（对齐 DefaultLayout navGroups）
}

// Meta 模块元信息，供 manifest API 与管理页展示。
type Meta struct {
	ID          string `json:"id"`          // 唯一标识，如 "seo"
	Title       string `json:"title"`       // 中文标题
	Description string `json:"description"` // 一句话简介
	Category    string `json:"category"`    // 分类，如 "工具"
	Nav         []Nav  `json:"nav"`         // 贡献的菜单项
	DefaultOn   bool   `json:"-"`           // 无 DB 记录时的默认启停
}

// Deps 模块可用的共享服务（装配时由 main 注入）。
type Deps struct {
	Store   *store.Store
	Posts   *post.Service
	Site    *tmpl.Site
	Modules *Manager // 供中间件按模块启停实时判定（如爬虫埋点）
}

// Module 一个功能模块的契约。
// Migrate/RegisterPublic 可空实现（无建表/无公开路由的模块直接返回 nil / 空）。
type Module interface {
	Meta() Meta
	// Migrate 建表等一次性迁移，装配时调用一次（与是否启用无关，保证表存在）。
	Migrate(deps Deps) error
	// RegisterRoutes 在 /api/v1 认证组下挂后台 API（外层已包 Gate 中间件，禁用即 403）。
	RegisterRoutes(authed *gin.RouterGroup, deps Deps)
	// RegisterPublic 挂前台/公开路由（如验证文件、埋点），可空实现。
	RegisterPublic(r *gin.Engine, deps Deps)
}
