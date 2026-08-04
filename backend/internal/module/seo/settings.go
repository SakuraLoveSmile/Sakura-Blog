package seo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"blog/internal/module"
	"blog/internal/store"
)

// settingKey SEO 配置在 settings 表中的键（独立存储，不并入 site.yml KnownSections）。
const settingKey = "seo"

const maskValue = "****"

// Config SEO 工具集配置。
type Config struct {
	Push    PushConfig   `json:"push"`
	Crawler ToggleConfig `json:"crawler"`
	Traffic ToggleConfig `json:"traffic"`
	Verify  VerifyConfig `json:"verify"`
}

// PushConfig 站点推送配置。
type PushConfig struct {
	Enabled     bool     `json:"enabled"`
	IndexNowKey string   `json:"indexnow_key"`
	BaiduToken  string   `json:"baidu_token"`
	Engines     []string `json:"engines"` // 启用的推送引擎：indexnow / baidu
}

// ToggleConfig 单开关子功能（爬虫记录 / 搜索流量）。
type ToggleConfig struct {
	Enabled bool `json:"enabled"`
}

// VerifyConfig 搜索引擎站长验证码。
type VerifyConfig struct {
	Google string `json:"google"`
	Bing   string `json:"bing"`
	Baidu  string `json:"baidu"`
}

// defaultConfig 模块默认配置（爬虫/流量默认开，推送默认关需填 key）。
func defaultConfig() Config {
	return Config{
		Push:    PushConfig{Engines: []string{}},
		Crawler: ToggleConfig{Enabled: true},
		Traffic: ToggleConfig{Enabled: true},
	}
}

// loadConfig 从 settings 表读 SEO 配置，缺失/损坏回退默认。
func loadConfig(st *store.Store) Config {
	cfg := defaultConfig()
	raw, err := st.GetSetting(settingKey)
	if err != nil || raw == "" {
		return cfg
	}
	_ = json.Unmarshal([]byte(raw), &cfg)
	if cfg.Push.Engines == nil {
		cfg.Push.Engines = []string{}
	}
	return cfg
}

// saveConfig 持久化 SEO 配置。
func saveConfig(st *store.Store, cfg Config) error {
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return st.PutSetting(settingKey, string(b))
}

// registerSettings 挂 SEO 配置读写端点（/api/v1/seo/settings，Console 在用）。
// 配置存取逻辑已迁移为 ConfigModule 接口方法（GetConfig/PutConfig），
// 框架统一端点 GET/PUT /api/v1/settings/seo 与这里共用同一实现。
func (m *Module) registerSettings(g *gin.RouterGroup) {
	g.GET("/settings", m.getSettings)
	g.PUT("/settings", m.putSettings)
}

// getSettings GET /api/v1/seo/settings：返回配置（密钥脱敏）。
func (m *Module) getSettings(c *gin.Context) {
	v, err := m.GetConfig(m.deps)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, v)
}

// putSettings PUT /api/v1/seo/settings：保存配置。
func (m *Module) putSettings(c *gin.Context) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取请求体失败"})
		return
	}
	if err := m.PutConfig(m.deps, raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetConfig ConfigModule 接口：返回脱敏配置（框架端点 GET /api/v1/settings/seo）。
func (m *Module) GetConfig(deps module.Deps) (any, error) {
	cfg := loadConfig(deps.Store)
	if cfg.Push.IndexNowKey != "" {
		cfg.Push.IndexNowKey = maskValue
	}
	if cfg.Push.BaiduToken != "" {
		cfg.Push.BaiduToken = maskValue
	}
	return cfg, nil
}

// PutConfig ConfigModule 接口：保存配置（框架端点 PUT /api/v1/settings/seo）。
// 脱敏占位（****/空）时保留原密钥；保存后刷新前台验证码注入。
func (m *Module) PutConfig(deps module.Deps, raw []byte) error {
	var incoming Config
	if err := json.Unmarshal(raw, &incoming); err != nil {
		return fmt.Errorf("配置格式错误: %v", err)
	}
	existing := loadConfig(deps.Store)
	if incoming.Push.IndexNowKey == "" || incoming.Push.IndexNowKey == maskValue {
		incoming.Push.IndexNowKey = existing.Push.IndexNowKey
	}
	if incoming.Push.BaiduToken == "" || incoming.Push.BaiduToken == maskValue {
		incoming.Push.BaiduToken = existing.Push.BaiduToken
	}
	if incoming.Push.Engines == nil {
		incoming.Push.Engines = []string{}
	}
	if err := saveConfig(deps.Store, incoming); err != nil {
		return err
	}
	// 刷新前台验证码注入
	m.applyVerifications(deps, incoming)
	return nil
}

// applyVerifications 把验证码写入 site（前台 head 注入用）。
func (m *Module) applyVerifications(deps module.Deps, cfg Config) {
	if deps.Site == nil {
		return
	}
	if deps.Site.Verifications == nil {
		deps.Site.Verifications = map[string]string{}
	}
	deps.Site.Verifications["google"] = cfg.Verify.Google
	deps.Site.Verifications["bing"] = cfg.Verify.Bing
	deps.Site.Verifications["baidu"] = cfg.Verify.Baidu
}
