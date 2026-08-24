// Package perf 实现「性能监控」功能模块：服务端运行时指标采样（内存/goroutine/GC）
// 与请求耗时统计（分钟级路由聚合），数据存 SQLite，Console 面板展示。
// 模块默认关闭，用户在「功能模块」页主动启用；停用后 API 被 Gate 拦截、采集实时停止。
package perf

import (
	"context"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/module"
)

// startTime 进程启动时间（uptime 用；init 时机约等于服务启动）。
var startTime = time.Now()

// 采集参数（代码内常量，不引入配置项）：
//   - sampleInterval  运行时指标采样间隔
//   - flushInterval   内存聚合桶落库检查间隔（桶本身按分钟切分）
//   - pruneEvery      多少次 flush 检查执行一次过期清理
//   - retentionDays   性能数据保留天数
const (
	sampleInterval = 30 * time.Second
	flushInterval  = 30 * time.Second
	pruneEvery     = 2880 // 30s * 2880 ≈ 24h
	retentionDays  = 7
)

// Module 性能监控模块。deps 在 RegisterRoutes/RegisterPublic 时注入并留存。
type Module struct {
	deps module.Deps

	// 分钟级请求聚合（中间件写入，flushLoop 落库）。
	agg *aggregator

	// 采集 goroutine 生命周期（OnEnable 启动 / OnDisable context 取消）。
	workersMu     sync.Mutex
	workersCancel context.CancelFunc
}

func init() {
	module.Register(&Module{agg: newAggregator()})
}

// Meta 模块元信息。DefaultOn=false：采集类功能默认关闭，由用户主动启用。
func (m *Module) Meta() module.Meta {
	return module.Meta{
		ID:          "perf",
		Title:       "性能监控",
		Description: "服务端运行时指标与请求耗时统计，Console 面板展示",
		Category:    "工具",
		Nav: []module.Nav{
			{Path: "/perf", Label: "性能监控", Icon: "ri:speed-line", Group: "系统", Roles: []string{"admin"}},
		},
		DefaultOn: false,
	}
}

// Migrate 建性能数据表（与启用状态无关，保证表存在）。
func (m *Module) Migrate(deps module.Deps) error {
	return deps.Store.EnsurePerfTables()
}

// RegisterRoutes 挂 /api/v1/perf/* 后台 API（外层已包 Gate 中间件，禁用即 403）。
func (m *Module) RegisterRoutes(authed *gin.RouterGroup, deps module.Deps) {
	m.deps = deps
	g := authed.Group("/perf")
	g.GET("/overview", m.overviewHandler)
	g.GET("/history", m.historyHandler)
	g.GET("/requests", m.requestsHandler)
}

// RegisterPublic 挂全局请求耗时中间件。
// 中间件内部按 deps.Modules.Enabled("perf") 实时判定，与 SEO 爬虫埋点同模式：停用即零开销直通。
// 启动回放：框架不会对启动时已启用的模块重放 OnEnable，此处补启采集 goroutine，保证重启后恢复采集。
func (m *Module) RegisterPublic(r *gin.Engine, deps module.Deps) {
	m.deps = deps
	r.Use(m.timingMiddleware(deps))
	if m.enabled(deps) {
		m.startWorkers(deps)
	}
}

// —— 模块框架扩展接口（DependencyModule / LifecycleModule）——

// Dependencies 声明模块依赖：性能监控不依赖其他模块。
func (m *Module) Dependencies() []string { return nil }

// OnEnable 生命周期钩子：启动采样与落库 goroutine（幂等）。
func (m *Module) OnEnable(deps module.Deps) error {
	m.deps = deps
	m.startWorkers(deps)
	return nil
}

// OnDisable 生命周期钩子：停止采集 goroutine（context 取消），并立即落库内存中残留的聚合桶（含当前分钟）。
func (m *Module) OnDisable(deps module.Deps) error {
	m.stopWorkers()
	m.flushAll(deps)
	return nil
}
