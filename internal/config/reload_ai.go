package config

import (
	"blog/internal/store"
	"encoding/json"
	"sync"
)

// AIConfig 对应 settings 表 "ai" 段：OpenAI 兼容接口配置，供 AI 属性生成代理使用。
type AIConfig struct {
	BaseURL        string `json:"base_url"`
	APIKey         string `json:"api_key"`
	Model          string `json:"model"`
	EmbeddingModel string `json:"embedding_model,omitempty"`
	// Spec 53 AI 摘要开关（nil=未配置，默认开启，兼容旧配置）。
	SummaryEnabled *bool `json:"summary_enabled,omitempty"`
	AllowUserAPI   *bool `json:"allow_user_api,omitempty"`
	QAEnabled      *bool `json:"qa_enabled,omitempty"`
	QAAllowUserLLM *bool `json:"qa_allow_user_llm,omitempty"`
}

// SummaryOn 前台是否展示 AI 摘要（未配置=开）。
func (c AIConfig) SummaryOn() bool { return c.SummaryEnabled == nil || *c.SummaryEnabled }

// UserAPIOn 是否允许读者临时接入自己的 API 生成摘要（未配置=开）。
func (c AIConfig) UserAPIOn() bool { return c.AllowUserAPI == nil || *c.AllowUserAPI }

// QAOn controls the public knowledge-base question endpoint and its frontend entry.
// It defaults to off so enabling an embedding model alone never exposes a public API.
func (c AIConfig) QAOn() bool { return c.QAEnabled != nil && *c.QAEnabled }

// QAUserLLMOn controls whether public users may provide only their final Chat model.
func (c AIConfig) QAUserLLMOn() bool { return c.QAAllowUserLLM == nil || *c.QAAllowUserLLM }

// LoadAI 从 settings 读取 AI 配置（未配置时返回零值）。
// Spec 61 阶段 5：内存缓存——文章页每请求读 settings 改为 RebuildSite（设置保存）时刷新，
// 首次调用加载，成功即缓存；失败不缓存（下次重试）。
var (
	loadAIMu      sync.RWMutex
	loadAICached  AIConfig
	loadAICacheOK bool
)

func LoadAI(st *store.Store) (AIConfig, error) {
	loadAIMu.RLock()
	if loadAICacheOK {
		cfg := loadAICached
		loadAIMu.RUnlock()
		return cfg, nil
	}
	loadAIMu.RUnlock()

	loadAIMu.Lock()
	defer loadAIMu.Unlock()
	if loadAICacheOK {
		return loadAICached, nil
	}
	var cfg AIConfig
	v, err := st.GetSetting(SectionAI)
	if err != nil || v == "" {
		return cfg, err
	}
	if err := json.Unmarshal([]byte(v), &cfg); err != nil {
		return AIConfig{}, err
	}
	loadAICached = cfg
	loadAICacheOK = true
	return cfg, nil
}

// InvalidateLoadAI 失效 AI 配置缓存（RebuildSite 内调用，设置保存后下次读取即重载）。
func InvalidateLoadAI() {
	loadAIMu.Lock()
	loadAICacheOK = false
	loadAIMu.Unlock()
}
