package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// RegistrationInviteRequest 由管理员选择邀请码有效期，省略时使用 7 天。
type RegistrationInviteRequest struct {
	ExpiresInDays int `json:"expires_in_days"`
}

// RegistrationInviteResponse 邀请码仅在创建成功时返回，数据库只保存其摘要。
type RegistrationInviteResponse struct {
	Code      string `json:"code"`
	ExpiresAt string `json:"expires_at"`
}

func (h *Handler) createRegistrationInvite(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req RegistrationInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	days := req.ExpiresInDays
	if days == 0 {
		days = 7
	}
	if days != 1 && days != 7 && days != 30 {
		Fail(c, http.StatusBadRequest, "邀请码有效期只能选择 1、7 或 30 天")
		return
	}
	expiresAt := time.Now().UTC().Add(time.Duration(days) * 24 * time.Hour).Truncate(time.Second)
	code, err := h.store.CreateRegistrationInvite(c.GetInt64("userID"), expiresAt)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "创建邀请码失败")
		return
	}
	c.JSON(http.StatusCreated, RegistrationInviteResponse{Code: code, ExpiresAt: expiresAt.Format(time.RFC3339)})
}
