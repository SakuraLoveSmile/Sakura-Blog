// Package module 定义博客的编译期「功能模块」框架。
//
// 设计取舍：本项目是单 Go 二进制 + go:embed 打包的 Vue SPA，不适合做 Halo 那种
// 第三方 JAR 热插拔。这里提供的是编译期模块注册表 + 运行时 DB 启停开关：
// 各功能以 Module 形式在 init() 里自注册，main 通过 blank import 引入；
// 启停状态存 SQLite，Console 可实时切换（控制菜单显隐与后端 API 可用性）。
package module

import (
	"io"
	"net/http"
	"sync/atomic"

	"github.com/gin-gonic/gin"

	"blog/internal/post"
	tmpl "blog/internal/settings"
	"blog/internal/store"
)

// Nav 模块贡献的 Console 侧栏菜单项。
type Nav struct {
	Path     string   `json:"path"`             // Console 路由，如 /seo
	Label    string   `json:"label"`            // 菜单文案
	Icon     string   `json:"icon"`             // iconify 图标名
	Group    string   `json:"group"`            // 侧栏分组（对齐 DefaultLayout navGroups）
	Roles    []string `json:"roles,omitempty"`  // 前端可见角色；空=所有 author 及以上角色
	Mobile   bool     `json:"mobile,omitempty"` // 是否出现在移动端底部导航
	Children []Nav    `json:"children,omitempty"`
}

// Meta 模块元信息，供 manifest API 与管理页展示。
type Meta struct {
	ID          string `json:"id"`               // 唯一标识，如 "seo"
	Title       string `json:"title"`            // 中文标题
	Description string `json:"description"`      // 一句话简介
	Category    string `json:"category"`         // 分类，如 "工具"
	Nav         []Nav  `json:"nav"`              // 贡献的菜单项
	DefaultOn   bool   `json:"-"`                // 无 DB 记录时的默认启停
	Locked      bool   `json:"locked,omitempty"` // 核心模块不可由用户停用
}

// Deps 模块可用的共享服务（装配时由 main 注入）。
type Deps struct {
	Store *store.Store
	Posts *post.Service
	// Site 站点配置原子指针（Spec 61 阶段 3）：模块侧 RebuildSite 复制-交换后经 .Load() 读取
	Site    *atomic.Pointer[tmpl.Site]
	Modules *Manager // 供中间件按模块启停实时判定（如爬虫埋点）
}

// Module 一个功能模块的契约。
// Migrate/RegisterPublic 可空实现（无建表/无公开路由的模块直接返回 nil / 空）。
// 依赖声明、生命周期钩子、配置命名空间为可选扩展接口（见下），
// Manager 用类型断言检测，旧实现不强制实现，保持向后兼容。
type Module interface {
	Meta() Meta
	// Migrate 建表等一次性迁移，装配时调用一次（与是否启用无关，保证表存在）。
	Migrate(deps Deps) error
	// RegisterRoutes 在 /api/v1 认证组下挂后台 API（外层已包 Gate 中间件，禁用即 403）。
	RegisterRoutes(authed *gin.RouterGroup, deps Deps)
	// RegisterPublic 挂前台/公开路由（如验证文件、埋点），可空实现。
	RegisterPublic(r *gin.Engine, deps Deps)
}

// —— 可选扩展接口（向后兼容：旧模块实现不强制，Manager 用类型断言检测）——

// DependencyModule 可选接口：声明依赖的其他模块 ID。未实现视为无依赖。
// Manager 在启动校验与启停时使用：依赖必须已注册且已启用。
type DependencyModule interface {
	Dependencies() []string
}

// LifecycleModule 可选接口：启停生命周期钩子。
// 启用时先保证依赖已启用（依赖钩子已在各自启用时调过），再调自身 OnEnable；
// 禁用时先调自身 OnDisable；任一失败回滚内存状态且不持久化。
type LifecycleModule interface {
	OnEnable(deps Deps) error
	OnDisable(deps Deps) error
}

// ConfigModule 可选接口：模块自有配置命名空间。
// ConfigSection 返回配置段名（如 "seo"）；框架经 MountConfigRoutes 自动挂
// GET/PUT /api/v1/settings/:section 到模块自己的配置存取，
// 不侵入全局 settings 表分段（site.yml 热更新链路）。
type ConfigModule interface {
	ConfigSection() string
	// GetConfig 返回脱敏后的配置对象（JSON 序列化输出）。
	GetConfig(deps Deps) (any, error)
	// PutConfig 保存配置（raw 为请求体原始 JSON）。
	PutConfig(deps Deps, raw []byte) error
}

// DependenciesOf 取模块依赖声明；未实现 DependencyModule 视为无依赖。
func DependenciesOf(m Module) []string {
	if dm, ok := m.(DependencyModule); ok {
		return dm.Dependencies()
	}
	return nil
}

// MountConfigRoutes 在模块路由组下自动挂配置读写端点：GET/PUT /settings/:section
// （:section 取 ConfigSection()，路径挂调用方传入的组——惯例是已包 Gate 的组，
// 禁用模块时 403）。模块未实现 ConfigModule 返回 false（未挂载）。
func MountConfigRoutes(g *gin.RouterGroup, deps Deps, m Module) bool {
	cm, ok := m.(ConfigModule)
	if !ok {
		return false
	}
	section := cm.ConfigSection()
	if section == "" {
		return false
	}
	path := "/settings/" + section
	g.GET(path, func(c *gin.Context) {
		v, err := cm.GetConfig(deps)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, v)
	})
	g.PUT(path, func(c *gin.Context) {
		raw, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "读取请求体失败"})
			return
		}
		if err := cm.PutConfig(deps, raw); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return true
}
