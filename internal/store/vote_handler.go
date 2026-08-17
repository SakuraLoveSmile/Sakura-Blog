package store

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// VoteHandler 投票公开 API（GET /api/vote/info、POST /api/vote/update）。
// 端点形状对齐 assets/js/services/vote.js 固化的 `${api}/info?id=` 与 `${api}/update?id=&value=` 契约。
type VoteHandler struct {
	store *Store
}

// NewVoteHandler 构造投票 handler。
func NewVoteHandler(s *Store) *VoteHandler {
	return &VoteHandler{store: s}
}

// VoteRoutes exposes vote callbacks without owning their URL paths.
// Stellar binds them to /api/vote/*.
type VoteRoutes struct {
	Info   gin.HandlerFunc
	Update gin.HandlerFunc
}

// PublicRoutes returns the vote callbacks used by Stellar.
func (h *VoteHandler) PublicRoutes() VoteRoutes {
	return VoteRoutes{Info: h.info, Update: h.update}
}

// info 获取计数：GET /api/vote/info?id=xxx → {id, votes: {up, down}}
func (h *VoteHandler) info(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}
	counts, err := h.store.GetVotes(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "votes": counts})
}

// update 投票：POST /api/vote/update?id=xxx&value=up|down → {id, votes: {up, down}}
func (h *VoteHandler) update(c *gin.Context) {
	id := c.Query("id")
	value := c.Query("value")
	if id == "" || (value != "up" && value != "down") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: id required, value up/down"})
		return
	}
	voterHash := voterIPHash(c.ClientIP(), id)
	if err := h.store.UpdateVote(id, value, voterHash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	counts, err := h.store.GetVotes(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "votes": counts})
}
