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
	if contextHasPermission(c, store.PermissionContentManageAny) {
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
	if contextHasPermission(c, store.PermissionContentManageAny) {
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
	if contextHasPermission(c, store.PermissionContentManageAny) {
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

// requireAttachmentOwnership enforces attachment ownership.
// Admins with PermissionContentManageAny can manage any attachment (including historical owner_id=0).
// Authors can only manage attachments where owner_id == userID.
func (h *Handler) requireAttachmentOwnership(c *gin.Context, att *store.Attachment) bool {
	if contextHasPermission(c, store.PermissionContentManageAny) {
		return true
	}
	userID := c.GetInt64("userID")
	if att == nil || att.OwnerID <= 0 || att.OwnerID != userID {
		AbortFail(c, http.StatusForbidden, "无权限：只能管理自己的附件")
		return false
	}
	return true
}

// forceCurrentAuthor makes ownership of a newly created or updated article a
// server decision. The client may not replace an author session's owner with
// an arbitrary users.id. Admin requests retain their explicit author list.
func forceCurrentAuthor(c *gin.Context, req *postRequest) error {
	if contextHasPermission(c, store.PermissionContentManageAny) {
		return nil
	}
	if !contextHasPermission(c, store.PermissionContentWrite) {
		return fmt.Errorf("current user cannot write content")
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
