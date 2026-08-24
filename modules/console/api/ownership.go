package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// requirePostOwnership enforces the article-level write boundary after the
// role middleware has established an authenticated Console user. Admins have
// unrestricted access; authors must be present in post_authors for the slug.
func (h *Handler) requirePostOwnership(c *gin.Context, slug string) bool {
	if c.GetString("userRole") == store.RoleAdmin {
		return true
	}
	userID := c.GetInt64("userID")
	owned, err := h.store.HasPostAuthor(slug, userID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "校验文章归属失败")
		return false
	}
	if !owned {
		AbortFail(c, http.StatusForbidden, "无权限：只能管理自己署名的文章")
		return false
	}
	return true
}

// requireRevisionOwnership applies the same boundary to revision restore,
// which addresses a revision ID rather than a post slug.
func (h *Handler) requireRevisionOwnership(c *gin.Context, revisionID int64) bool {
	if c.GetString("userRole") == store.RoleAdmin {
		return true
	}
	owned, err := h.store.HasRevisionAuthor(revisionID, c.GetInt64("userID"))
	if err != nil {
		Fail(c, http.StatusInternalServerError, "校验版本归属失败")
		return false
	}
	if !owned {
		AbortFail(c, http.StatusForbidden, "无权限：只能恢复自己文章的版本")
		return false
	}
	return true
}

func (h *Handler) requireShareOwnership(c *gin.Context, shareID int64) bool {
	if c.GetString("userRole") == store.RoleAdmin {
		return true
	}
	owned, err := h.store.HasShareAuthor(shareID, c.GetInt64("userID"))
	if err != nil {
		Fail(c, http.StatusInternalServerError, "校验分享链接归属失败")
		return false
	}
	if !owned {
		AbortFail(c, http.StatusForbidden, "无权限：只能管理自己文章的分享链接")
		return false
	}
	return true
}

func isAuthorRole(c *gin.Context) bool {
	return c.GetString("userRole") == store.RoleAuthor
}

// forceCurrentAuthor makes ownership of a newly created or updated article a
// server decision. The client may not replace an author session's owner with
// an arbitrary users.id. Admin requests retain their explicit author list.
func forceCurrentAuthor(c *gin.Context, req *postRequest) error {
	if !isAuthorRole(c) {
		return nil
	}
	userID := c.GetInt64("userID")
	if userID <= 0 {
		return fmt.Errorf("invalid author user id")
	}
	ref := strconv.FormatInt(userID, 10)
	req.authorRefs = []string{ref}
	req.authorIDs = []int64{userID}
	req.Author = ref
	data, err := json.Marshal(req.authorRefs)
	if err != nil {
		return err
	}
	req.Authors = data
	return nil
}
