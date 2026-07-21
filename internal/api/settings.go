package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"blog/internal/config"
)

// getSettings GET /api/v1/settings：返回全部可编辑分段的原始 JSON。
// 敏感字段脱敏：comments.notify.password 非空时返回 "****"。
func (h *Handler) getSettings(c *gin.Context) {
	secs, err := config.LoadSectionsJSON(h.store)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	c.JSON(http.StatusOK, secs)
}

// updateSection PUT /api/v1/settings/:section：保存单个分段配置，成功后热更新站点。
// comments 段特殊处理：password 为空或 "****" 时保留原值（避免脱敏回写清除密码）。
func (h *Handler) updateSection(c *gin.Context) {
	section := c.Param("section")
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取请求体失败"})
		return
	}
	// comments 段 password 保留逻辑
	if section == "comments" {
		raw = h.preservePassword(raw)
	}
	if err := config.SaveSection(h.store, h.site, section, raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
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

// testNotify POST /api/v1/settings/notify/test：向 admin_email 发送测试邮件，验证 SMTP 配置。
func (h *Handler) testNotify(c *gin.Context) {
	if h.comments == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "评论系统未初始化"})
		return
	}
	if err := h.comments.SendTestMail(); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
