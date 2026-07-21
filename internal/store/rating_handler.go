package store

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RatingHandler 评分公开 API（GET/POST /api/rating）。
type RatingHandler struct {
	store *Store
}

// NewRatingHandler 构造评分 handler。
func NewRatingHandler(s *Store) *RatingHandler {
	return &RatingHandler{store: s}
}

// Register 注册公开路由。
func (h *RatingHandler) Register(r *gin.Engine) {
	r.GET("/api/rating", h.get)
	r.POST("/api/rating", h.vote)
}

// get 获取评分：GET /api/rating?id=xxx → {avg, count}
func (h *RatingHandler) get(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}
	summary, err := h.store.GetRating(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

// vote 投票：POST /api/rating {id, score} → {avg, count}
func (h *RatingHandler) vote(c *gin.Context) {
	var req struct {
		ID    string `json:"id"`
		Score int    `json:"score"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == "" || req.Score < 1 || req.Score > 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: id required, score 1-5"})
		return
	}
	voterHash := voterIPHash(c.ClientIP(), req.ID)
	if err := h.store.VoteRating(req.ID, req.Score, voterHash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	summary, err := h.store.GetRating(req.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

// voterIPHash IP + targetID 哈希去重（同 IP 对同目标只算一票，覆盖更新）。
func voterIPHash(ip, targetID string) string {
	h := sha256.Sum256([]byte(ip + "|" + targetID))
	return hex.EncodeToString(h[:16])
}
