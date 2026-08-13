package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	yaml "github.com/goccy/go-yaml"

	"blog/internal/config"
	tmpl "blog/internal/template"
)

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
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		Fail(c, http.StatusBadRequest, "读取请求体失败")
		return
	}
	// comments 段 password 保留逻辑
	if section == "comments" {
		raw = h.preservePassword(raw)
	}
	// ai 段 api_key 保留逻辑（脱敏回写不覆盖原 Key）
	if section == "ai" {
		raw = h.preserveAIKey(raw)
	}
	if err := config.SaveSection(h.store, h.site, section, raw); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if h.onContentChanged != nil {
		h.onContentChanged()
	}
	// friends 段影响友链聚合（每友链文章数/刷新间隔），保存后即时重建聚合缓存
	if section == "friends" && h.onLinksChanged != nil {
		h.onLinksChanged()
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// preservePassword 若提交的 password 为空或 "****"，从已存配置中恢复原密码。
func (h *Handler) preservePassword(raw []byte) []byte {
	var incoming map[string]any
	if json.Unmarshal(raw, &incoming) != nil {
		return raw
	}
	notify, _ := incoming["notify"].(map[string]any)
	if notify == nil {
		return raw
	}
	pw, _ := notify["password"].(string)
	if pw != "" && pw != "****" {
		return raw // 用户填了新密码，直接用
	}
	// 从已存配置取原密码
	existing, err := h.store.GetSetting("comments")
	if err != nil || existing == "" {
		return raw
	}
	var stored map[string]any
	if json.Unmarshal([]byte(existing), &stored) != nil {
		return raw
	}
	storedNotify, _ := stored["notify"].(map[string]any)
	if storedNotify == nil {
		return raw
	}
	notify["password"] = storedNotify["password"]
	if b, err := json.Marshal(incoming); err == nil {
		return b
	}
	return raw
}

// preserveAIKey 若提交的 ai.api_key 为空或 "****"，从已存配置中恢复原 Key。
func (h *Handler) preserveAIKey(raw []byte) []byte {
	var incoming map[string]any
	if json.Unmarshal(raw, &incoming) != nil {
		return raw
	}
	key, _ := incoming["api_key"].(string)
	if key != "" && key != "****" {
		return raw // 用户填了新 Key，直接用
	}
	existing, err := h.store.GetSetting("ai")
	if err != nil || existing == "" {
		return raw
	}
	var stored map[string]any
	if json.Unmarshal([]byte(existing), &stored) != nil {
		return raw
	}
	if key == "****" || key == "" {
		incoming["api_key"] = stored["api_key"]
	}
	if b, err := json.Marshal(incoming); err == nil {
		return b
	}
	return raw
}

// getStyleYAML GET /api/v1/settings/style/yaml：返回当前 style 段的 YAML 文本（主题 YAML 编辑器用）。
func (h *Handler) getStyleYAML(c *gin.Context) {
	b, err := yaml.Marshal(h.site.Load().Style)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"yaml": string(b)})
}

// updateStyleYAML PUT /api/v1/settings/style/yaml：接收 YAML 文本，解析校验后存 style 段并热更新。
// 解析失败返回 400 + 错误明细（含行号），不写库。
func (h *Handler) updateStyleYAML(c *gin.Context) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		Fail(c, http.StatusBadRequest, "读取请求体失败")
		return
	}
	var sc tmpl.StyleConfig
	if err := yaml.Unmarshal(raw, &sc); err != nil {
		Fail(c, http.StatusBadRequest, "YAML 解析失败: "+err.Error())
		return
	}
	jb, err := json.Marshal(sc)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := config.SaveSection(h.store, h.site, config.SectionStyle, jb); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
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
