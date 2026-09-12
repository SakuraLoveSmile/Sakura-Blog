package perf

import (
	"context"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/module"
	"blog/internal/store"
)

// bucketKey 分钟桶键：分钟起点 + 方法 + 路由模板（c.FullPath()，避免参数高基数）。
type bucketKey struct {
	minute     time.Time
	method     string
	route      string
	generation uint64
}

// aggregator 分钟级请求耗时聚合（内存 map，flushLoop 定期落库）。
type aggregator struct {
	mu      sync.Mutex
	buckets map[bucketKey]*store.PerfRequestBucket
}

func newAggregator() *aggregator {
	return &aggregator{buckets: map[bucketKey]*store.PerfRequestBucket{}}
}

// recordAt 按维护世代累计请求，旧数据不能跨恢复窗口落库。
func (a *aggregator) recordAt(method, route string, status int, d time.Duration, generation uint64) {
	ms := float64(d.Microseconds()) / 1000
	key := bucketKey{minute: time.Now().Truncate(time.Minute), method: method, route: route, generation: generation}
	a.mu.Lock()
	defer a.mu.Unlock()
	b, ok := a.buckets[key]
	if !ok {
		b = &store.PerfRequestBucket{Minute: key.minute, Method: method, Route: route}
		a.buckets[key] = b
	}
	b.Count++
	b.TotalMs += ms
	if ms > b.MaxMs {
		b.MaxMs = ms
	}
	if status >= http.StatusBadRequest {
		b.ErrCount++
	}
}

// takeOlder 取走并移除 minute 早于 cutoff 的桶（当前分钟桶继续累积）。
func (a *aggregator) takeOlder(cutoff time.Time, generation uint64) []store.PerfRequestBucket {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []store.PerfRequestBucket
	for k, b := range a.buckets {
		if k.generation != generation {
			delete(a.buckets, k)
			continue
		}
		if k.minute.Before(cutoff) {
			out = append(out, *b)
			delete(a.buckets, k)
		}
	}
	return out
}

// takeAll 取走并移除全部桶（含当前未完成分钟，停用时避免数据丢失）。
func (a *aggregator) takeAll(generation uint64) []store.PerfRequestBucket {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]store.PerfRequestBucket, 0, len(a.buckets))
	for k, b := range a.buckets {
		if k.generation == generation {
			out = append(out, *b)
		}
		delete(a.buckets, k)
	}
	return out
}

// timingSkipPrefixes 埋点跳过的静态/非业务路径前缀。
var timingSkipPrefixes = []string{"/assets", "/uploads", "/og/", "/console"}

// timingMiddleware 返回中间件：模块启用时记录每个请求的耗时/状态，按分钟桶聚合。
// 与 SEO 爬虫埋点同模式：运行时实时判定启停，禁用即零开销直通。
func (m *Module) timingMiddleware(deps module.Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Modules == nil || !deps.Modules.Enabled("perf") {
			c.Next()
			return
		}
		p := c.Request.URL.Path
		for _, pre := range timingSkipPrefixes {
			if strings.HasPrefix(p, pre) {
				c.Next()
				return
			}
		}
		if p == "/sw.js" || p == "/manifest.webmanifest" || p == "/favicon.ico" {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()
		route := c.FullPath() // 404 等未匹配路由为空，跳过避免高基数
		if route == "" {
			return
		}
		generation := uint64(0)
		if deps.Store != nil {
			generation = deps.Store.Maintenance().Generation()
		}
		m.agg.recordAt(c.Request.Method, route, c.Writer.Status(), time.Since(start), generation)
	}
}

// enabled 模块当前是否启用（deps.Modules 缺失视为禁用）。
func (m *Module) enabled(deps module.Deps) bool {
	return deps.Modules != nil && deps.Modules.Enabled("perf")
}

// startWorkers 幂等启动采样与落库 goroutine（OnEnable 与启动时已启用回放共用）。
func (m *Module) startWorkers(deps module.Deps) {
	m.workersMu.Lock()
	defer m.workersMu.Unlock()
	if m.workersCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.workersCancel = cancel
	go m.sampleLoop(ctx, deps)
	go m.flushLoop(ctx, deps)
}

// stopWorkers 停止采样与落库 goroutine（OnDisable 调用，context 取消）。
func (m *Module) stopWorkers() {
	m.workersMu.Lock()
	defer m.workersMu.Unlock()
	if m.workersCancel != nil {
		m.workersCancel()
		m.workersCancel = nil
	}
}

// workersRunning 采集 goroutine 是否在运行（测试与状态判定用）。
func (m *Module) workersRunning() bool {
	m.workersMu.Lock()
	defer m.workersMu.Unlock()
	return m.workersCancel != nil
}

// sampleLoop 定时采样运行时指标（内存/goroutine/GC），模块禁用时跳过；ctx 取消即退出。
func (m *Module) sampleLoop(ctx context.Context, deps module.Deps) {
	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if !m.enabled(deps) {
			continue
		}
		metrics := captureRuntimeMetrics()
		if deps.Store != nil {
			if release, err := deps.Store.Maintenance().Acquire(); err == nil {
				_ = deps.Store.InsertPerfSamples(time.Now(), metrics)
				release()
			}
		}
	}
}

// captureRuntimeMetrics 取当前运行时指标（heap/sys 转 MB 存储，图表直接用）。
func captureRuntimeMetrics() map[string]float64 {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return map[string]float64{
		"heap_alloc_mb": float64(ms.HeapAlloc) / 1024 / 1024,
		"sys_mem_mb":    float64(ms.Sys) / 1024 / 1024,
		"goroutines":    float64(runtime.NumGoroutine()),
		"gc_count":      float64(ms.NumGC),
	}
}

// flushLoop 定期把已完成分钟的聚合桶落库，并周期性清理过期数据；ctx 取消即退出。
// 落库不看启停（把禁用前残留的桶也落掉），采样侧已按启停截断来源。
func (m *Module) flushLoop(ctx context.Context, deps module.Deps) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	ticks := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		m.flush(deps, time.Now())
		ticks++
		if ticks%pruneEvery == 0 && deps.Store != nil {
			if release, err := deps.Store.Maintenance().Acquire(); err == nil {
				_ = deps.Store.PrunePerfData(retentionDays)
				release()
			}
		}
	}
}

// flush 落库 cutoff 之前分钟的全部聚合桶（定时任务用）。
func (m *Module) flush(deps module.Deps, now time.Time) {
	if deps.Store == nil {
		return
	}
	release, err := deps.Store.Maintenance().Acquire()
	if err != nil {
		return
	}
	defer release()
	buckets := m.agg.takeOlder(now.Truncate(time.Minute), deps.Store.Maintenance().Generation())
	if len(buckets) == 0 {
		return
	}
	_ = deps.Store.InsertPerfRequestBuckets(buckets)
}

// flushAll 落库全部聚合桶（含当前未完成分钟，OnDisable 用）。
func (m *Module) flushAll(deps module.Deps) {
	if deps.Store == nil {
		return
	}
	release, err := deps.Store.Maintenance().Acquire()
	if err != nil {
		return
	}
	defer release()
	buckets := m.agg.takeAll(deps.Store.Maintenance().Generation())
	if len(buckets) == 0 {
		return
	}
	_ = deps.Store.InsertPerfRequestBuckets(buckets)
}
