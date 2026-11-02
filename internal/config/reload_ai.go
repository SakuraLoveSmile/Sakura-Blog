package config

import (
	"strings"

	"blog/internal/store"
)

// AIEndpoint 是一组 OpenAI 兼容接口配置；对话与向量接口各自独立。
type AIEndpoint struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

// Configured 报告该接口的地址与模型是否就绪，空白字符视为未配置。
func (e AIEndpoint) Configured() bool {
	return strings.TrimSpace(e.BaseURL) != "" && strings.TrimSpace(e.Model) != ""
}

// AIConfig 是本地 AI 服务使用的 typed bridge；生产真值为 canonical Blog.ai。
// Chat 只用于摘要/属性生成/问答回答，Embedding 只用于知识库向量化与检索。
type AIConfig struct {
	Chat      AIEndpoint `json:"chat"`
	Embedding AIEndpoint `json:"embedding"`
	// Spec 53 AI 摘要开关（nil=未配置，默认开启，兼容旧配置）。
	SummaryEnabled *bool `json:"summary_enabled,omitempty"`
	AllowUserAPI   *bool `json:"allow_user_api,omitempty"`
	QAEnabled      *bool `json:"qa_enabled,omitempty"`
	QAAllowUserLLM *bool `json:"qa_allow_user_llm,omitempty"`
}

// legacyAISectionConfig 只服务于旧 settings.ai 分段的校验与规范化回存。
// canonical Blog.ai 已改为嵌套 chat/embedding；历史平铺字段留作逐字段回退，
// 旧分段的读写语义保持原样。
type legacyAISectionConfig struct {
	BaseURL        string `json:"base_url"`
	APIKey         string `json:"api_key"`
	Model          string `json:"model"`
	EmbeddingModel string `json:"embedding_model,omitempty"`
	SummaryEnabled *bool  `json:"summary_enabled,omitempty"`
	AllowUserAPI   *bool  `json:"allow_user_api,omitempty"`
	QAEnabled      *bool  `json:"qa_enabled,omitempty"`
	QAAllowUserLLM *bool  `json:"qa_allow_user_llm,omitempty"`
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

// LoadAI 从当前 Store 的一致快照读取，绝不共享另一数据库的凭据。
// 只接受 canonical Blog.ai；未迁移或损坏时失败，绝不读取旧 ai 段。
func LoadAI(st *store.Store) (AIConfig, error) {
	snapshot, err := st.ReadSettingsSnapshot()
	if err != nil {
		return AIConfig{}, err
	}
	canonical, err := LoadStellarV2Snapshot(snapshot)
	if err != nil {
		return AIConfig{}, err
	}
	runtime, err := ParseStellarBlogRuntime(canonical.Blog)
	return runtime.AI, err
}

// InvalidateLoadAI 保留旧生命周期入口；配置读取不再缓存秘密。
func InvalidateLoadAI() {}
