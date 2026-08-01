package seo

import (
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
)

// registerTraffic 挂搜索流量相关端点。
func (m *Module) registerTraffic(g *gin.RouterGroup) {
	g.GET("/traffic/analysis", m.trafficAnalysisHandler)
	g.GET("/traffic/landing", m.trafficLandingHandler)
}

type engineCount struct {
	Engine string `json:"engine"`
	Count  int    `json:"count"`
}

type trafficTrendPoint struct {
	Date   string `json:"date"`
	Engine string `json:"engine"`
	Count  int    `json:"count"`
}

type trafficAnalysis struct {
	Today        int                 `json:"today"`
	Yesterday    int                 `json:"yesterday"`
	RangeTotal   int                 `json:"range_total"`
	TopSource    string              `json:"top_source"`
	Trend        []trafficTrendPoint `json:"trend"`
	Distribution []engineCount       `json:"distribution"`
}

// trafficAnalysisHandler 从 page_views.referrer 派生搜索流量分析（仅计搜索引擎来源）。
func (m *Module) trafficAnalysisHandler(c *gin.Context) {
	days := rangeDays(c)
	rows, err := m.deps.Store.ReferrerDaily(days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	trendMap := map[string]int{}   // date|engine -> count
	distMap := map[string]int{}    // engine -> count
	out := trafficAnalysis{Trend: []trafficTrendPoint{}, Distribution: []engineCount{}}
	for _, r := range rows {
		engine := referrerEngine(r.Referrer)
		if engine == "" {
			continue
		}
		trendMap[r.Date+"|"+engine] += r.Count
		distMap[engine] += r.Count
		out.RangeTotal += r.Count
		if r.Date == today {
			out.Today += r.Count
		}
		if r.Date == yesterday {
			out.Yesterday += r.Count
		}
	}
	for k, v := range trendMap {
		date, engine := k[:10], k[11:]
		out.Trend = append(out.Trend, trafficTrendPoint{Date: date, Engine: engine, Count: v})
	}
	sort.Slice(out.Trend, func(i, j int) bool { return out.Trend[i].Date < out.Trend[j].Date })
	for engine, cnt := range distMap {
		out.Distribution = append(out.Distribution, engineCount{Engine: engine, Count: cnt})
	}
	sort.Slice(out.Distribution, func(i, j int) bool { return out.Distribution[i].Count > out.Distribution[j].Count })
	if len(out.Distribution) > 0 {
		out.TopSource = out.Distribution[0].Engine
	}
	c.JSON(http.StatusOK, out)
}

type landingRow struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
}

// trafficLandingHandler 搜索来源的着陆页 Top（按访问量降序）。
func (m *Module) trafficLandingHandler(c *gin.Context) {
	days := rangeDays(c)
	rows, err := m.deps.Store.ReferrerLanding(days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	agg := map[string]int{}
	for _, r := range rows {
		if referrerEngine(r.Referrer) == "" {
			continue
		}
		agg[r.Path] += r.Count
	}
	out := make([]landingRow, 0, len(agg))
	for p, cnt := range agg {
		out = append(out, landingRow{Path: p, Count: cnt})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	if len(out) > 20 {
		out = out[:20]
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}
