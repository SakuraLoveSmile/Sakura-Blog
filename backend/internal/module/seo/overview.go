package seo

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// healthItem SEO 健康单项。
type healthItem struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Status string `json:"status"` // ok / warn
	Hint   string `json:"hint"`
}

// overviewData 概览返回体。
type overviewData struct {
	TodaySearch  int          `json:"today_search"` // 今日搜索流量
	TodayCrawl   int          `json:"today_crawl"`  // 今日爬虫
	CrawlDelta   int          `json:"crawl_delta"`  // 爬虫环比百分比（vs 昨日）
	Top7Source   string       `json:"top7_source"`  // 近 7 天 TOP 来源
	Top7Crawler  string       `json:"top7_crawler"` // 近 7 天最活跃爬虫
	Health       []healthItem `json:"health"`
	FeaturePush  bool         `json:"feature_push"`
	FeatureCrawl bool         `json:"feature_crawl"`
	FeatureTraf  bool         `json:"feature_traffic"`
	HasVerify    bool         `json:"has_verify"`
}

// registerOverview 挂概览端点。
func (m *Module) registerOverview(g *gin.RouterGroup) {
	g.GET("/overview", m.overviewHandler)
}

func (m *Module) overviewHandler(c *gin.Context) {
	st := m.deps.Store
	cfg := loadConfig(st)
	out := overviewData{Health: []healthItem{}}

	// 爬虫：今日/环比/最活跃
	if ca, err := st.CrawlerAnalysis(7); err == nil {
		out.TodayCrawl = ca.Today
		out.Top7Crawler = ca.TopEngine
		if ca.Yesterday > 0 {
			out.CrawlDelta = int(float64(ca.Today-ca.Yesterday) / float64(ca.Yesterday) * 100)
		} else if ca.Today > 0 {
			out.CrawlDelta = 100
		}
	}

	// 搜索流量：今日 + 近 7 天 TOP 来源（从 page_views.referrer 派生）
	today := time.Now().Format("2006-01-02")
	if rows, err := st.ReferrerDaily(7); err == nil {
		dist := map[string]int{}
		for _, r := range rows {
			engine := referrerEngine(r.Referrer)
			if engine == "" {
				continue
			}
			dist[engine] += r.Count
			if r.Date == today {
				out.TodaySearch += r.Count
			}
		}
		top, max := "", 0
		for e, cnt := range dist {
			if cnt > max {
				top, max = e, cnt
			}
		}
		out.Top7Source = top
	}

	// SEO 健康
	site := m.deps.Site
	https := site != nil && strings.HasPrefix(site.URL, "https://")
	out.Health = append(out.Health, healthItem{Key: "https", Label: "HTTPS", Status: okWarn(https),
		Hint: pick(https, "已启用 HTTPS", "站点未使用 HTTPS，影响搜索排名与安全")})
	out.Health = append(out.Health, healthItem{Key: "sitemap", Label: "站点地图", Status: "ok",
		Hint: "已提供 /sitemap.xml"})
	out.Health = append(out.Health, healthItem{Key: "robots", Label: "robots.txt", Status: "ok",
		Hint: "已提供 /robots.txt"})
	descOK := site != nil && strings.TrimSpace(site.Description) != ""
	out.Health = append(out.Health, healthItem{Key: "description", Label: "网站描述", Status: okWarn(descOK),
		Hint: pick(descOK, "已配置站点描述", "首页缺少 Meta 描述，会影响搜索摘要展示")})

	out.FeaturePush = cfg.Push.Enabled
	out.FeatureCrawl = cfg.Crawler.Enabled
	out.FeatureTraf = cfg.Traffic.Enabled
	// 验证码已归属站点级 SEO 段（设置 → SEO），体检从 site.SEO 读取
	out.HasVerify = site != nil &&
		(site.SEO.GoogleVerification != "" || site.SEO.BingVerification != "" || site.SEO.BaiduVerification != "")

	c.JSON(http.StatusOK, out)
}

func okWarn(ok bool) string {
	if ok {
		return "ok"
	}
	return "warn"
}

func pick(ok bool, a, b string) string {
	if ok {
		return a
	}
	return b
}
