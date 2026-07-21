package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
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

// statsPopular GET /api/v1/stats/popular?days=7&limit=10：区间内热门路径（附标题解析）。
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
	// path → 标题解析：/post/:slug 查 store，其余用路径本身
	for i := range popular {
		if slug, ok := strings.CutPrefix(popular[i].Path, "/post/"); ok {
			if p, err := h.store.GetBySlug(slug); err == nil && p != nil {
				popular[i].Title = p.Title
			}
		}
		if popular[i].Title == "" {
			popular[i].Title = popular[i].Path
		}
	}
	c.JSON(http.StatusOK, gin.H{"days": days, "items": popular})
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

// parseDays 解析 days 参数，仅允许 7 或 30，其余 fallback 到默认值。
func parseDays(c *gin.Context, def int) int {
	d, _ := strconv.Atoi(c.DefaultQuery("days", strconv.Itoa(def)))
	if d != 7 && d != 30 {
		return def
	}
	return d
}
