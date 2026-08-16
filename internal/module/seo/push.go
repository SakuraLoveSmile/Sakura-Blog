package seo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/module"
	"blog/internal/store"
	tmpl "blog/internal/template"
)

// pushClient 推送用 HTTP client（短超时）。
var pushClient = &http.Client{Timeout: 10 * time.Second}

// registerPush 挂站点推送相关端点。
func (m *Module) registerPush(g *gin.RouterGroup) {
	g.POST("/push", m.pushHandler)
	g.GET("/push/logs", m.pushLogsHandler)
	g.DELETE("/push/logs", m.clearPushLogsHandler)
}

// pushHandler POST /seo/push：手动推送一批 URL 到已启用引擎。
func (m *Module) pushHandler(c *gin.Context) {
	var body struct {
		URLs []string `json:"urls"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.URLs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供待推送的 URL 列表"})
		return
	}
	cfg := loadConfig(m.deps.Store)
	if len(cfg.Push.Engines) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "尚未在设置中启用任何推送引擎"})
		return
	}
	ok, fail := pushURLs(m.deps.Store, siteHostURL(m.deps.Site.Load()), cfg, body.URLs)
	c.JSON(http.StatusOK, gin.H{"ok": true, "success": ok, "failed": fail})
}

func (m *Module) pushLogsHandler(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	size := atoiDefault(c.Query("size"), 20)
	logs, total, err := m.deps.Store.ListPushLogs(page, size, c.Query("engine"), c.Query("status"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": logs, "total": total})
}

func (m *Module) clearPushLogsHandler(c *gin.Context) {
	n, err := m.deps.Store.ClearPushLogs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "deleted": n})
}

// pushURLs 把 urls 推送到 cfg.Push.Engines 里的每个引擎，逐条落库，返回成功/失败计数。
func pushURLs(st *store.Store, host string, cfg Config, urls []string) (int, int) {
	var ok, fail int
	for _, engine := range cfg.Push.Engines {
		for _, u := range urls {
			var err error
			switch engine {
			case "indexnow":
				err = pushIndexNow(cfg.Push.IndexNowKey, host, u)
			case "baidu":
				err = pushBaidu(cfg.Push.BaiduToken, host, u)
			default:
				continue
			}
			status, msg := "success", ""
			if err != nil {
				status, msg = "failed", err.Error()
				fail++
			} else {
				ok++
			}
			_ = st.AddPushLog(engine, u, status, msg)
		}
	}
	return ok, fail
}

// pushIndexNow 向 IndexNow 提交单个 URL。
func pushIndexNow(key, host, target string) error {
	if key == "" {
		return fmt.Errorf("未配置 IndexNow key")
	}
	payload := map[string]any{
		"host":    host,
		"key":     key,
		"urlList": []string{target},
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, "https://api.indexnow.org/indexnow", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := pushClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

// pushBaidu 向百度普通推送接口提交单个 URL。
func pushBaidu(token, host, target string) error {
	if token == "" {
		return fmt.Errorf("未配置百度推送 token")
	}
	endpoint := fmt.Sprintf("http://data.zz.baidu.com/urls?site=%s&token=%s", url.QueryEscape(host), url.QueryEscape(token))
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(target))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain")
	resp, err := pushClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

// atoiDefault 解析十进制，失败回退默认值。
func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return def
		}
		n = n*10 + int(ch-'0')
	}
	if n == 0 {
		return def
	}
	return n
}

// siteHostURL 从站点配置取主机（含 scheme），供推送接口的 host 参数。
func siteHostURL(site *tmpl.Site) string {
	if site == nil {
		return ""
	}
	return site.URL
}

// NotifyPublished 文章发布时的自动推送入口（main 在文章写回调里调用）。
// 仅当 SEO 模块启用、push.enabled 且配置了引擎时异步推送，不阻塞主流程。
func NotifyPublished(deps module.Deps, postURL string) {
	if deps.Modules == nil || !deps.Modules.Enabled("seo") || deps.Store == nil {
		return
	}
	cfg := loadConfig(deps.Store)
	if !cfg.Push.Enabled || len(cfg.Push.Engines) == 0 || postURL == "" {
		return
	}
	go pushURLs(deps.Store, siteHostURL(deps.Site.Load()), cfg, []string{postURL})
}
