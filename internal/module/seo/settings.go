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

// settingKey SEO 工具集配置在 settings 表中的键。
// 注意：必须与站点级 SEO 段（config.SectionSEO = "seo"）分离——历史上两者共用 "seo" 行，
// 两个 UI 保存时各自只回存自己的结构体，导致互相覆盖；且框架按 ConfigSection 挂载的静态路由
// PUT /settings/seo 会劫持核心设置的分段保存路由。现工具集独立用 "seo_tools"，旧行由 migrateLegacySeoRow 迁移。
const settingKey = "seo_tools"

// legacySeoKey 历史共用键：旧版工具集配置曾与站点级 SEO 段（config.SectionSEO）同存于此。
const legacySeoKey = "seo"

const maskValue = "****"

// Config SEO 工具集配置。
// 验证码（verify）已统一到站点级 SEO 段（设置 → SEO Tab 的 *_verification 字段），此处不再存储。
type Config struct {
	Push    PushConfig   `json:"push"`
	Crawler ToggleConfig `json:"crawler"`
	Traffic ToggleConfig `json:"traffic"`
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
// 框架统一端点 GET/PUT /api/v1/settings/seo_tools 与这里共用同一实现。
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

// GetConfig ConfigModule 接口：返回脱敏配置（框架端点 GET /api/v1/settings/seo_tools）。
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

// PutConfig ConfigModule 接口：保存配置（框架端点 PUT /api/v1/settings/seo_tools）。
// 脱敏占位（****/空）时保留原密钥。
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
	return nil
}

// migrateLegacySeoRow 一次性迁移历史数据：旧版 SEO 工具集与站点级 SEO 段共用 settings "seo" 行。
// 拆分策略：push/crawler/traffic 迁到独立行 "seo_tools"；verify 并入站点级 SEO 段的
// *_verification 字段（仅在站点级为空时回填）；最后从 "seo" 行移除工具集键。
// 返回 seoRowChanged 表示站点级 SEO 行被改动（调用方需触发站点重建）。
func migrateLegacySeoRow(st *store.Store) (seoRowChanged bool) {
	if raw, err := st.GetSetting(settingKey); err == nil && raw != "" {
		return false // 已迁移过
	}
	raw, err := st.GetSetting(legacySeoKey)
	if err != nil || raw == "" {
		return false
	}
	var row map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &row) != nil {
		return false
	}
	toolKeys := []string{"push", "crawler", "traffic", "verify"}
	found := false
	for _, k := range toolKeys {
		if _, ok := row[k]; ok {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	// verify → 站点级验证码（站点级已有值则不覆盖）
	if v, ok := row["verify"]; ok {
		var vc struct {
			Google string `json:"google"`
			Bing   string `json:"bing"`
			Baidu  string `json:"baidu"`
		}
		if json.Unmarshal(v, &vc) == nil {
			fill := func(key, val string) {
				if val == "" {
					return
				}
				if cur, exists := row[key]; !exists || string(cur) == `""` {
					if b, err := json.Marshal(val); err == nil {
						row[key] = b
						seoRowChanged = true
					}
				}
			}
			fill("google_verification", vc.Google)
			fill("bing_verification", vc.Bing)
			fill("baidu_verification", vc.Baidu)
		}
	}
	// push/crawler/traffic → 独立行 "seo_tools"
	tools := map[string]json.RawMessage{}
	for _, k := range toolKeys[:3] {
		if v, ok := row[k]; ok {
			tools[k] = v
		}
	}
	if b, err := json.Marshal(tools); err == nil {
		_ = st.PutSetting(settingKey, string(b))
	}
	// 从站点级 SEO 行移除工具集键
	for _, k := range toolKeys {
		delete(row, k)
	}
	if b, err := json.Marshal(row); err == nil {
		_ = st.PutSetting(legacySeoKey, string(b))
		seoRowChanged = true
	}
	return seoRowChanged
}
