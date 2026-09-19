package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	yaml "github.com/goccy/go-yaml"

	"blog/internal/config"
	tmpl "blog/internal/settings"
)

const maxSettingsBodyBytes = 1 << 20

// getSettings GET /api/v1/settings：返回全部可编辑分段的原始 JSON。
// 敏感字段脱敏：comments.notify.password / ai.api_key 非空时返回 "****"。
func (h *Handler) getSettings(c *gin.Context) {
	secs, err := config.LoadSectionsJSON(h.store)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	// 脱敏 comments.notify.password
	if raw, ok := secs["comments"]; ok {
		var cm map[string]any
		if json.Unmarshal(raw, &cm) == nil {
			if notify, ok := cm["notify"].(map[string]any); ok {
				if pw, _ := notify["password"].(string); pw != "" {
					notify["password"] = "****"
					if b, err := json.Marshal(cm); err == nil {
						secs["comments"] = json.RawMessage(b)
					}
				}
			}
		}
	}
	// 脱敏 ai.api_key
	if raw, ok := secs["ai"]; ok {
		var ai map[string]any
		if json.Unmarshal(raw, &ai) == nil {
			if key, _ := ai["api_key"].(string); key != "" {
				ai["api_key"] = "****"
				if b, err := json.Marshal(ai); err == nil {
					secs["ai"] = json.RawMessage(b)
				}
			}
		}
	}
	c.JSON(http.StatusOK, secs)
}

// updateSection PUT /api/v1/settings/:section：保存单个分段配置，成功后热更新站点。
// comments 段特殊处理：password 为空或 "****" 时保留原值（避免脱敏回写清除密码）。
func (h *Handler) updateSection(c *gin.Context) {
	section := c.Param("section")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSettingsBodyBytes)
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			Fail(c, http.StatusRequestEntityTooLarge, "设置请求体超过 1 MiB")
			return
		}
		Fail(c, http.StatusBadRequest, "读取请求体失败")
		return
	}
	if err := config.SaveSection(h.store, h.site, section, raw); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	h.publishContentChanged()
	// friends 段影响友链聚合（每友链文章数/刷新间隔），保存后即时重建聚合缓存
	if section == "friends" && h.onLinksChanged != nil {
		h.onLinksChanged()
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// StyleYAMLResponse GET /api/v1/settings/style/yaml 响应结构。
type StyleYAMLResponse struct {
	YAML string `json:"yaml"`
}

// StyleYAMLUpdateResponse PUT /api/v1/settings/style/yaml 响应结构。
type StyleYAMLUpdateResponse struct {
	OK bool `json:"ok"`
}

// getStyleYAML GET /api/v1/settings/style/yaml：返回当前 style 段的 YAML 文本（主题样式 YAML 编辑器用）。

func (h *Handler) getStyleYAML(c *gin.Context) {
	b, err := yaml.Marshal(h.site.Load().Style)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, StyleYAMLResponse{YAML: string(b)})
}

// updateStyleYAML PUT /api/v1/settings/style/yaml：接收 YAML 文本，解析校验后存 style 段并热更新。
// 解析失败返回 400 + 错误明细（含行号），不写库。保留未知字段通道。
func (h *Handler) updateStyleYAML(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSettingsBodyBytes)
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			Fail(c, http.StatusRequestEntityTooLarge, "设置请求体超过 1 MiB")
			return
		}
		Fail(c, http.StatusBadRequest, "读取请求体失败")
		return
	}
	var sc tmpl.StyleConfig
	if err := yaml.Unmarshal(raw, &sc); err != nil {
		Fail(c, http.StatusBadRequest, "YAML 解析失败: "+err.Error())
		return
	}
	var rawMap map[string]any
	if err := yaml.Unmarshal(raw, &rawMap); err != nil {
		Fail(c, http.StatusBadRequest, "YAML 解析失败: "+err.Error())
		return
	}
	jb, err := json.Marshal(rawMap)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := config.SaveSection(h.store, h.site, config.SectionStyle, jb); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, StyleYAMLUpdateResponse{OK: true})
}

// testNotify POST /api/v1/settings/notify/test：向 admin_email 发送测试邮件，验证 SMTP 配置。
func (h *Handler) testNotify(c *gin.Context) {
	if h.comments == nil {
		Fail(c, http.StatusBadRequest, "评论系统未初始化")
		return
	}
	if err := h.comments.SendTestMail(); err != nil {
		// 业务失败仍 200（Console 依赖 ok/error 字段），附带统一 Problem 字段
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error(), "type": "mail_failed", "title": "发送失败", "status": http.StatusOK, "detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// FeedbackCheckRequest POST /api/v1/settings/feedback/check 请求体：
// api_base 取当前表单草稿，让管理员「先测再存」。
type FeedbackCheckRequest struct {
	APIBase string `json:"api_base"`
}

// FeedbackCheckResponse POST /api/v1/settings/feedback/check 响应结构。
// 只回传状态码与延迟，不回传响应体：端点虽仅 admin 可达，仍避免形成内容回显。
// api_base 允许指向内网/回环（同机部署是标准形态），因此不做 safefetch 的
// IP 段限制——与 feedback 段自身校验（仅要求 http/https scheme）保持一致。
type FeedbackCheckResponse struct {
	OK       bool   `json:"ok"`     // /healthz 返回 200 视为可达且健康
	Status   int    `json:"status"` // 实际 HTTP 状态码（0 = 未能建立连接）
	Latency  int64  `json:"latency_ms"`
	Error    string `json:"error,omitempty"`
	Endpoint string `json:"endpoint"` // 实际探测的完整 URL（api_base + /healthz）
}

// checkFeedback POST /api/v1/settings/feedback/check：探测 Feedback 服务 /healthz。
// 4s 超时、不跟随重定向；仅校验 scheme/host，允许回环与内网地址（管理员自配置项）。
func (h *Handler) checkFeedback(c *gin.Context) {
	var req FeedbackCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	base := strings.TrimSpace(req.APIBase)
	u, err := url.ParseRequestURI(base)
	if err != nil || u == nil || u.Host == "" || u.User != nil ||
		(u.Scheme != "http" && u.Scheme != "https") {
		Fail(c, http.StatusBadRequest, "api_base 必须是合法的 http(s) 地址")
		return
	}
	endpoint := strings.TrimSuffix(base, "/") + "/healthz"
	client := &http.Client{
		Timeout: 4 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	start := time.Now()
	resp, err := client.Get(endpoint)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		c.JSON(http.StatusOK, FeedbackCheckResponse{OK: false, Endpoint: endpoint, Latency: latency, Error: err.Error()})
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	c.JSON(http.StatusOK, FeedbackCheckResponse{
		OK:       resp.StatusCode == http.StatusOK,
		Status:   resp.StatusCode,
		Latency:  latency,
		Endpoint: endpoint,
	})
}
