package perf

import (
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// queryHours 从 hours 查询参数解析时间窗（默认 24，上限 168=保留期 7 天）。
func queryHours(c *gin.Context) int {
	h, err := strconv.Atoi(c.Query("hours"))
	if err != nil || h <= 0 {
		h = 24
	}
	if h > retentionDays*24 {
		h = retentionDays * 24
	}
	return h
}

// overviewHandler GET /api/v1/perf/overview：当前运行时指标 + 近 5 分钟请求摘要。
func (m *Module) overviewHandler(c *gin.Context) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	dbSize := int64(0)
	if p := m.deps.Store.DBPath(); p != "" {
		if fi, err := os.Stat(p); err == nil {
			dbSize = fi.Size()
		}
	}

	since := time.Now().Add(-5 * time.Minute)
	count, errCount, avgMs, err := m.deps.Store.PerfRequestSummarySince(since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	errRate := 0.0
	if count > 0 {
		errRate = float64(errCount) / float64(count)
	}

	c.JSON(http.StatusOK, gin.H{
		"start_time":    startTime.Format(time.RFC3339),
		"uptime_sec":    int64(time.Since(startTime).Seconds()),
		"go_version":    runtime.Version(),
		"platform":      runtime.GOOS + "/" + runtime.GOARCH,
		"goroutines":    runtime.NumGoroutine(),
		"heap_alloc_mb": float64(ms.HeapAlloc) / 1024 / 1024,
		"sys_mem_mb":    float64(ms.Sys) / 1024 / 1024,
		"gc_count":      ms.NumGC,
		"db_size_bytes": dbSize,
		"recent": gin.H{
			"requests": count,
			"avg_ms":   avgMs,
			"errors":   errCount,
			"err_rate": errRate,
		},
	})
}

// historyHandler GET /api/v1/perf/history?hours=24：运行时指标时序（图表用）。
func (m *Module) historyHandler(c *gin.Context) {
	h := queryHours(c)
	points, err := m.deps.Store.PerfSamplesSince(time.Now().Add(-time.Duration(h) * time.Hour))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"hours": h, "points": points})
}

// requestsHandler GET /api/v1/perf/requests?hours=24：路由耗时排行 + 逐分钟吞吐。
func (m *Module) requestsHandler(c *gin.Context) {
	h := queryHours(c)
	since := time.Now().Add(-time.Duration(h) * time.Hour)
	routes, err := m.deps.Store.PerfRouteStatsSince(since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	throughput, err := m.deps.Store.PerfThroughputSince(since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"hours": h, "routes": routes, "throughput": throughput})
}
