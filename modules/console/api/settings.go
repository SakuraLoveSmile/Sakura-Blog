package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

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
