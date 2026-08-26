package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/config"
	"blog/internal/rag"
	"blog/internal/store"
)

const (
	ragQuestionLimit     = 1000
	ragPublicAnswerLimit = 2000
)

// RAGStatusResponse is the stable Console status envelope.
type RAGStatusResponse struct {
	Status         string `json:"status"`
	Total          int    `json:"total"`
	Done           int    `json:"done"`
	DocumentCount  int    `json:"document_count"`
	ChunkCount     int    `json:"chunk_count"`
	EmbeddingModel string `json:"embedding_model"`
	EmbeddingDims  int    `json:"embedding_dims"`
	LastBuiltAt    string `json:"last_built_at"`
	LastError      string `json:"last_error"`
}

// RAGAskRequest is shared by the private and public endpoints. BYOK fields
// replace only the final Chat model and are never persisted.
type RAGAskRequest struct {
	Question string `json:"question"`
	Endpoint string `json:"endpoint,omitempty"`
	Model    string `json:"model,omitempty"`
	APIKey   string `json:"api_key,omitempty"`
}

// RAGCitation is a source link returned with a RAG answer.
type RAGCitation struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
	Slug  string `json:"slug"`
	URL   string `json:"url"`
}

// RAGAskResponse is returned by Console and public QA.
type RAGAskResponse struct {
	Answer    string        `json:"answer"`
	Citations []RAGCitation `json:"citations"`
}

func (h *Handler) ragStatus(c *gin.Context) {
	if h.rag == nil {
		Fail(c, http.StatusServiceUnavailable, "RAG 服务不可用")
		return
	}
	status, err := h.rag.Status()
	if err != nil {
		Fail(c, http.StatusInternalServerError, "读取 RAG 状态失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, RAGStatusResponse{
		Status: status.Status, Total: status.Total, Done: status.Done,
		DocumentCount: status.DocumentCount, ChunkCount: status.ChunkCount,
		EmbeddingModel: status.EmbeddingModel, EmbeddingDims: status.EmbeddingDims,
		LastBuiltAt: status.LastBuiltAt, LastError: status.LastError,
	})
}

func (h *Handler) ragRebuild(c *gin.Context) {
	if h.rag == nil {
		Fail(c, http.StatusServiceUnavailable, "RAG 服务不可用")
		return
	}
	if !h.rag.EmbeddingConfigured() {
		Fail(c, http.StatusBadRequest, "Embedding Model 未配置，请先到设置 → AI 中填写接口地址与 Embedding Model")
		return
	}
	if !h.rag.QueueRebuild() {
		status, err := h.rag.Status()
		if err == nil && strings.TrimSpace(status.EmbeddingModel) != "" {
			// QueueRebuild can lose the race with the worker after the job has
			// been accepted. A configured service is therefore already in the
			// requested state even when metadata still says idle/error.
			c.JSON(http.StatusOK, toRAGStatusResponse(status))
			return
		}
		Fail(c, http.StatusBadRequest, "知识库重建未能入队，请稍后重试")
		return
	}
	status, err := h.rag.Status()
	if err != nil {
		Fail(c, http.StatusInternalServerError, "读取 RAG 状态失败: "+err.Error())
		return
	}
	c.JSON(http.StatusAccepted, toRAGStatusResponse(status))
}

func toRAGStatusResponse(status rag.Status) RAGStatusResponse {
	return RAGStatusResponse{
		Status: status.Status, Total: status.Total, Done: status.Done,
		DocumentCount: status.DocumentCount, ChunkCount: status.ChunkCount,
		EmbeddingModel: status.EmbeddingModel, EmbeddingDims: status.EmbeddingDims,
		LastBuiltAt: status.LastBuiltAt, LastError: status.LastError,
	}
}

func (h *Handler) ragAsk(c *gin.Context) {
	var req RAGAskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	if err := validateRAGQuestion(req.Question); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	answer, err := h.askRAG(c, req)
	if err != nil {
		h.respondRAGError(c, err)
		return
	}
	c.JSON(http.StatusOK, answer)
}

func (h *Handler) ragAskPublic(c *gin.Context) {
	cfg, err := loadPublicRAGConfig(h.store)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !cfg.qaEnabled {
		Fail(c, http.StatusForbidden, "本站未开启 AI 问答")
		return
	}
	if !ragPublicRateAllow(c.ClientIP()) {
		Fail(c, http.StatusTooManyRequests, "请求过于频繁，请稍后再试")
		return
	}
	var req RAGAskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	if err := validateRAGQuestion(req.Question); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.Endpoint != "" || req.Model != "" || req.APIKey != "" {
		if !cfg.allowUserLLM {
			Fail(c, http.StatusForbidden, "本站未允许使用自己的 AI 模型")
			return
		}
		if _, err := validateGuestAIEndpoint(req.Endpoint); err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, errPrivateHost) {
				status = http.StatusForbidden
			}
			Fail(c, status, "接口地址不可用: "+err.Error())
			return
		}
		if strings.TrimSpace(req.Model) == "" {
			Fail(c, http.StatusBadRequest, "使用自己的 AI 模型时必须填写模型")
			return
		}
	}
	answer, err := h.askRAG(c, req)
	if err != nil {
		h.respondRAGError(c, err)
		return
	}
	if runes := []rune(answer.Answer); len(runes) > ragPublicAnswerLimit {
		answer.Answer = string(runes[:ragPublicAnswerLimit])
	}
	c.JSON(http.StatusOK, answer)
}

func (h *Handler) askRAG(c *gin.Context, req RAGAskRequest) (RAGAskResponse, error) {
	if h.rag == nil {
		return RAGAskResponse{}, rag.ErrNotConfigured
	}
	var override *rag.ChatOverride
	if req.Endpoint != "" || req.Model != "" || req.APIKey != "" {
		override = &rag.ChatOverride{BaseURL: req.Endpoint, APIKey: req.APIKey, Model: req.Model}
	}
	answer, err := h.rag.Ask(c.Request.Context(), req.Question, override)
	if err != nil {
		return RAGAskResponse{}, err
	}
	response := RAGAskResponse{Answer: answer.Answer}
	for _, citation := range answer.Citations {
		response.Citations = append(response.Citations, RAGCitation{ID: citation.ID, Type: citation.Type, Title: citation.Title, Slug: citation.Slug, URL: citation.URL})
	}
	return response, nil
}

func (h *Handler) respondRAGError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, rag.ErrNotConfigured):
		Fail(c, http.StatusServiceUnavailable, "RAG 尚未配置 Embedding Model 或 Chat Model")
	case errors.Is(err, rag.ErrEmptyKnowledge):
		Fail(c, http.StatusUnprocessableEntity, "当前知识库没有足够资料回答这个问题")
	case errors.Is(err, rag.ErrInvalidQuestion):
		Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		Fail(c, http.StatusGatewayTimeout, "AI 请求已取消")
	default:
		Fail(c, http.StatusBadGateway, "AI 问答失败: "+err.Error())
	}
}

func validateRAGQuestion(question string) error {
	if strings.TrimSpace(question) == "" {
		return errors.New("问题不能为空")
	}
	if len([]rune(question)) > ragQuestionLimit {
		return errors.New("问题不能超过 1000 字")
	}
	return nil
}

type publicRAGConfig struct {
	qaEnabled    bool
	allowUserLLM bool
}

func loadPublicRAGConfig(st *store.Store) (publicRAGConfig, error) {
	cfg, err := config.LoadAI(st)
	if err != nil {
		return publicRAGConfig{}, err
	}
	return publicRAGConfig{qaEnabled: cfg.QAOn(), allowUserLLM: cfg.QAUserLLMOn()}, nil
}

var (
	ragPublicRateMu sync.Mutex
	ragPublicHits   = map[string][]time.Time{}
)

func ragPublicRateAllow(ip string) bool {
	const limit = 10
	now := time.Now()
	ragPublicRateMu.Lock()
	defer ragPublicRateMu.Unlock()
	hits := ragPublicHits[ip]
	kept := hits[:0]
	for _, at := range hits {
		if now.Sub(at) < time.Minute {
			kept = append(kept, at)
		}
	}
	if len(kept) >= limit {
		ragPublicHits[ip] = kept
		return false
	}
	ragPublicHits[ip] = append(kept, now)
	return true
}
