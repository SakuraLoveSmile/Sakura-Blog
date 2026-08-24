package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// UCMeResponse is the account summary consumed by the User Center shell.
type UCMeResponse struct {
	ID          int64    `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	Email       string   `json:"email"`
	Avatar      string   `json:"avatar"`
	Role        string   `json:"role"`
	Status      string   `json:"status"`
	Has2FA      bool     `json:"has_2fa"`
	Permissions []string `json:"permissions"`
}

type UCPostListResponse struct {
	Items []*UCPostResponse `json:"items"`
	Total int               `json:"total"`
	Page  int               `json:"page"`
	Size  int               `json:"size"`
}

type UCAttachmentListResponse struct {
	Items []*UCAttachmentResponse `json:"items"`
	Total int                     `json:"total"`
	Page  int                     `json:"page"`
	Size  int                     `json:"size"`
}

// UCPostResponse is the stable User Center article projection. It deliberately
// contains no site-setting mutation contract; the editor only sends the
// limited fields handled by mergeUCPostBody.
type UCPostResponse struct {
	Slug      string          `json:"slug"`
	Title     string          `json:"title"`
	Excerpt   string          `json:"excerpt"`
	ContentMD string          `json:"content_md"`
	Status    string          `json:"status"`
	Cover     string          `json:"cover"`
	Tags      string          `json:"tags"`
	Authors   []string        `json:"authors"`
	Author    string          `json:"author"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
	Views     int64           `json:"views"`
	HasDraft  bool            `json:"has_draft"`
	Draft     json.RawMessage `json:"draft,omitempty"`
}

type UCAttachmentResponse struct {
	ID        int64  `json:"id"`
	Filename  string `json:"filename"`
	URL       string `json:"url"`
	Mime      string `json:"mime"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"created_at"`
	ThumbURL  string `json:"thumb_url"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	GroupID   int64  `json:"group_id"`
}

func toUCPostResponse(p *postJSON) *UCPostResponse {
	if p == nil {
		return nil
	}
	return &UCPostResponse{
		Slug: p.Slug, Title: p.Title, Excerpt: p.Excerpt, ContentMD: p.ContentMD, Status: p.Status,
		Cover: p.Cover, Tags: p.Tags, Authors: append([]string(nil), p.Authors...), Author: p.Author,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt, Views: p.Views, HasDraft: p.HasDraft, Draft: p.Draft,
	}
}

func toUCAttachmentResponse(a *attachmentJSON) *UCAttachmentResponse {
	if a == nil {
		return nil
	}
	return &UCAttachmentResponse{ID: a.ID, Filename: a.Filename, URL: a.URL, Mime: a.Mime, Size: a.Size, CreatedAt: a.CreatedAt, ThumbURL: a.ThumbURL, Width: a.Width, Height: a.Height, GroupID: a.GroupID}
}

func (h *Handler) ucMe(c *gin.Context) {
	user, err := h.store.GetUserByID(c.GetInt64("userID"))
	if err != nil || user == nil {
		Fail(c, http.StatusUnauthorized, "登录状态无效")
		return
	}
	c.JSON(http.StatusOK, UCMeResponse{
		ID: user.ID, Username: user.Username, DisplayName: displayName(user), Email: user.Email,
		Avatar: user.Avatar, Role: user.Role, Status: user.Status, Has2FA: strings.TrimSpace(user.TOTPSecret) != "", Permissions: user.Permissions,
	})
}

func displayName(user *store.User) string {
	if strings.TrimSpace(user.DisplayName) != "" {
		return user.DisplayName
	}
	return user.Username
}

func (h *Handler) listUCPosts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	posts, total, err := h.store.ListPostsByOwner(c.GetInt64("userID"), c.Query("status"), page, size)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]*UCPostResponse, 0, len(posts))
	for _, p := range posts {
		items = append(items, toUCPostResponse(toPostJSON(p, p.Views)))
	}
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	c.JSON(http.StatusOK, UCPostListResponse{Items: items, Total: total, Page: page, Size: size})
}

func (h *Handler) getUCPost(c *gin.Context) {
	if !h.requirePostOwnership(c, c.Param("slug")) {
		return
	}
	p, err := h.store.GetBySlug(c.Param("slug"))
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if p == nil {
		Fail(c, http.StatusNotFound, "文章不存在")
		return
	}
	response := toPostJSON(p, p.Views)
	if response.HasDraft {
		response.Draft = json.RawMessage(p.DraftJSON)
	}
	c.JSON(http.StatusOK, toUCPostResponse(response))
}

type ucPostPayload struct {
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Excerpt   string `json:"excerpt"`
	ContentMD string `json:"content_md"`
	Status    string `json:"status"`
	Tags      string `json:"tags"`
}

func (h *Handler) createUCPost(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 2<<20))
	if err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	var input ucPostPayload
	if err := json.Unmarshal(body, &input); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	encoded, err := json.Marshal(postRequest{
		Slug: input.Slug, Title: input.Title, Excerpt: input.Excerpt,
		ContentMD: input.ContentMD, Status: input.Status, Tags: input.Tags,
	})
	if err != nil {
		Fail(c, http.StatusInternalServerError, "请求序列化失败")
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(encoded))
	h.createPost(c)
}

// mergeUCPostBody keeps site-level fields intact while exposing only the
// restricted editor fields to the UC client. The existing Console save
// pipeline remains the single writer after the request is expanded.
func (h *Handler) mergeUCPostBody(c *gin.Context) bool {
	slug := c.Param("slug")
	existing, err := h.store.GetBySlug(slug)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return false
	}
	if existing == nil {
		Fail(c, http.StatusNotFound, "文章不存在")
		return false
	}
	var input ucPostPayload
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 2<<20))
	if err != nil || json.Unmarshal(body, &input) != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return false
	}
	if input.Status != store.StatusDraft && input.Status != store.StatusPublished {
		input.Status = existing.Status
	}
	publishAt := ""
	if !existing.PublishAt.IsZero() {
		publishAt = existing.PublishAt.Format("2006-01-02T15:04")
	}
	authors, _ := json.Marshal(existing.Authors)
	full := postRequest{
		Slug: existing.Slug, Title: input.Title, Excerpt: input.Excerpt,
		Cover: existing.Cover, Banner: existing.Banner, Category: existing.Category,
		Categories: append([]string(nil), existing.Categories...), Topic: existing.Topic,
		TopicID: json.RawMessage(strconv.FormatInt(existing.TopicID, 10)), Tags: input.Tags,
		ContentMD: input.ContentMD, Status: input.Status, Pinned: existing.Pinned,
		Password: existing.Password, PublishAt: publishAt, MetaDescription: existing.MetaDescription,
		MetaKeywords: existing.MetaKeywords, Authors: authors, Author: existing.Author, AILabel: existing.AILabel,
	}
	encoded, err := json.Marshal(full)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "请求序列化失败")
		return false
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(encoded))
	return true
}

func (h *Handler) updateUCPost(c *gin.Context) {
	if !h.requirePostOwnership(c, c.Param("slug")) || !h.mergeUCPostBody(c) {
		return
	}
	h.updatePost(c)
}

func (h *Handler) saveUCDraft(c *gin.Context) {
	if !h.requirePostOwnership(c, c.Param("slug")) || !h.mergeUCPostBody(c) {
		return
	}
	h.savePostDraft(c)
}

func (h *Handler) publishUCPost(c *gin.Context) {
	if !h.requirePostOwnership(c, c.Param("slug")) || !h.mergeUCPostBody(c) {
		return
	}
	h.publishPost(c)
}

func (h *Handler) listUCAttachments(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "24"))
	keyword := strings.TrimSpace(c.Query("keyword"))
	kind := c.Query("kind")
	groupID := parseGroupParam(c.Query("group"))
	ownerID := c.GetInt64("userID")
	total, err := h.store.CountAttachmentsByOwner(ownerID, keyword, kind, groupID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	attachments, err := h.store.ListAttachmentsByOwner(ownerID, page, size, keyword, kind, groupID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]*UCAttachmentResponse, 0, len(attachments))
	for _, attachment := range attachments {
		items = append(items, toUCAttachmentResponse(toAttachmentJSON(attachment)))
	}
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 24
	}
	c.JSON(http.StatusOK, UCAttachmentListResponse{Items: items, Total: total, Page: page, Size: size})
}

func (h *Handler) deleteUCAttachment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		Fail(c, http.StatusBadRequest, "无效的附件 ID")
		return
	}
	attachment, err := h.store.GetAttachment(id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if attachment == nil {
		Fail(c, http.StatusNotFound, "附件不存在")
		return
	}
	if attachment.OwnerID != c.GetInt64("userID") {
		AbortFail(c, http.StatusForbidden, "无权限：只能管理自己的附件")
		return
	}
	if attachment.Path != "" {
		h.storage.Remove(attachment.Path)
	}
	if attachment.ThumbPath != "" {
		h.storage.Remove(attachment.ThumbPath)
	}
	if err := h.store.DeleteAttachment(id); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, struct {
		OK bool `json:"ok"`
	}{OK: true})
}
