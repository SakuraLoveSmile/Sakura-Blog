package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// PAT constants used by the Console token-management endpoints.
const (
	patTokenPrefix = "pat_"
	patScopeRead   = "read"
)

// generatePAT generates a token whose plaintext is returned only once.
func generatePAT() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate pat: %w", err)
	}
	return patTokenPrefix + hex.EncodeToString(b), nil
}

func formatPATExpiry(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func parsePATExpiry(s string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}

// listTokens GET /api/v1/tokens: token list with plaintext and hashes omitted.
func (h *Handler) listTokens(c *gin.Context) {
	pats, err := h.store.ListPATs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]gin.H, 0, len(pats))
	for _, p := range pats {
		items = append(items, gin.H{
			"id":           p.ID,
			"name":         p.Name,
			"description":  p.Description,
			"scopes":       p.Scopes,
			"prefix":       patTokenPrefix + "****",
			"expires_at":   formatPATExpiry(p.ExpiresAt),
			"last_used_at": formatPATExpiry(p.LastUsedAt),
			"created_at":   p.CreatedAt.Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// createToken POST /api/v1/tokens: plaintext is returned only in this response.
func (h *Handler) createToken(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Scopes      string `json:"scopes"`
		ExpiresAt   string `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "令牌名称不能为空"})
		return
	}
	scopes := strings.TrimSpace(req.Scopes)
	if scopes == "" {
		scopes = patScopeRead
	}
	for _, scope := range strings.Split(scopes, ",") {
		if strings.TrimSpace(scope) != patScopeRead {
			c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 read scope"})
			return
		}
	}
	var expires time.Time
	if value := strings.TrimSpace(req.ExpiresAt); value != "" {
		expires = parsePATExpiry(value)
		if expires.IsZero() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "过期时间格式无效"})
			return
		}
	}
	plain, err := generatePAT()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	pat := &store.PAT{
		Name: name, TokenHash: store.HashToken(plain), Description: strings.TrimSpace(req.Description),
		Scopes: scopes, ExpiresAt: expires,
	}
	if err := h.store.CreatePAT(pat); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id": pat.ID, "name": pat.Name, "token": plain, "scopes": pat.Scopes,
		"expires_at": formatPATExpiry(pat.ExpiresAt),
	})
}

// deleteToken DELETE /api/v1/tokens/:id: revocation is idempotent.
func (h *Handler) deleteToken(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法令牌 id"})
		return
	}
	if err := h.store.DeletePAT(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
