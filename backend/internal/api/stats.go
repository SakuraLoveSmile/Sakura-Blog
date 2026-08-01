package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// statsTrend GET /api/v1/stats/trend?days=7|30：每日 PV/UV 趋势序列。
func (h *Handler) statsTrend(c *gin.Context) {
	days := parseDays(c, 7)
	trend, err := h.store.TrendPVUV(days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"days": days, "trend": trend})
}

// statsPopular GET /api/v1/stats/popular?days=7&limit=10：区间内热门文章（仅 post 类）。
func (h *Handler) statsPopular(c *gin.Context) {
	days := parseDays(c, 7)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	popular, err := h.store.PopularByViews(days, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 仅统计文章 (/post/*)，过滤 /、/archives、/pages/* 等非文章路径
	filtered := make([]*store.PopularStat, 0)
	for _, ps := range popular {
		if strings.HasPrefix(ps.Path, "/post/") {
			ps.Title = ""
			if slug, ok := strings.CutPrefix(ps.Path, "/post/"); ok {
				if p, err := h.store.GetBySlug(slug); err == nil && p != nil {
					title := p.Title
					if title == "" {
						title = slug
					}
					ps.Title = title
				}
			}
			if ps.Title == "" {
				ps.Title = ps.Path
			}
			filtered = append(filtered, &ps) // 取地址
		}
	}
	c.JSON(http.StatusOK, gin.H{"days": days, "items": filtered})
}

// statsReferrers GET /api/v1/stats/referrers?days=7：来源域名 Top N。
func (h *Handler) statsReferrers(c *gin.Context) {
	days := parseDays(c, 7)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	refs, err := h.store.TopReferrers(days, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"days": days, "items": refs})
}

// statsHourly GET /api/v1/stats/hourly?days=7：24 小时时段分布。
func (h *Handler) statsHourly(c *gin.Context) {
	days := parseDays(c, 7)
	hourly, err := h.store.HourlyVisits(days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"days": days, "distribution": hourly})
}

// parseDays 解析 days 参数，仅允许 7 或 30，其余 fallback 到默认值。
func parseDays(c *gin.Context, def int) int {
	d, _ := strconv.Atoi(c.DefaultQuery("days", strconv.Itoa(def)))
	if d != 7 && d != 30 {
		return def
	}
	return d
}
