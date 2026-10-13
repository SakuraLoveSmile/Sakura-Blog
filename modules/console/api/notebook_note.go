package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"blog/internal/store"

	"github.com/gin-gonic/gin"
)

type NotebookNoteRequest struct {
	NotebookID     *string   `json:"notebook_id,omitempty"`
	Slug           *string   `json:"slug,omitempty"`
	ContentMD      *string   `json:"content_md,omitempty"`
	Status         *string   `json:"status,omitempty"`
	Tags           *[]string `json:"tags,omitempty"`
	StellarVersion *int      `json:"stellar_version,omitempty"`
	StellarConfig  *string   `json:"stellar_config,omitempty"`
	BlogExtensions *string   `json:"blog_extensions,omitempty"`
}

type NotebookNoteResponse struct {
	ID             int64     `json:"id"`
	NotebookID     string    `json:"notebook_id"`
	Slug           string    `json:"slug"`
	Title          string    `json:"title"`
	ContentMD      string    `json:"content_md"`
	ContentHTML    string    `json:"content_html"`
	Status         string    `json:"status"`
	Tags           []string  `json:"tags"`
	AuthorID       int64     `json:"author_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	StellarVersion int       `json:"stellar_version"`
	StellarConfig  string    `json:"stellar_config"`
	BlogExtensions string    `json:"blog_extensions"`
}

type NotebookNoteListResponse struct {
	Items []NotebookNoteResponse `json:"items"`
	Total int                    `json:"total"`
	Page  int                    `json:"page"`
	Size  int                    `json:"size"`
}

func notebookNoteResponse(p *store.Note) NotebookNoteResponse {
	tags := p.Tags
	if tags == nil {
		tags = []string{}
	}
	return NotebookNoteResponse{ID: p.ID, NotebookID: p.NotebookID, Slug: p.Slug, Title: p.Title, ContentMD: p.ContentMD, ContentHTML: p.ContentHTML, Status: p.Status, Tags: tags, AuthorID: p.AuthorID, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt, StellarVersion: p.StellarVersion, StellarConfig: p.StellarConfig, BlogExtensions: p.BlogExtensions}
}

func (h *Handler) listNotebookNotes(c *gin.Context) {
	notebook, ok := h.requireNotebook(c)
	if !ok {
		return
	}
	page, size, ok := notebookPagination(c)
	if !ok {
		return
	}
	status := c.Query("status")
	if status != "" && status != store.StatusDraft && status != store.StatusPublished {
		notebookError(c, store.ErrNoteInvalid)
		return
	}
	notes, err := h.store.ListNotesByTag(notebook.ID, c.Query("tag"), false)
	if err != nil {
		notebookError(c, err)
		return
	}
	items := []NotebookNoteResponse{}
	query := strings.ToLower(c.Query("q"))
	for _, note := range notes {
		if !notebookOwned(c, note.AuthorID) {
			continue
		}
		if _, err := note.ValidateStellar(); err != nil {
			notebookError(c, err)
			return
		}
		if status != "" && note.Status != status {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(note.Slug+" "+note.Title+" "+note.ContentMD), query) {
			continue
		}
		items = append(items, notebookNoteResponse(note))
	}
	total := len(items)
	start, end := notebookPageBounds(total, page, size)
	c.JSON(http.StatusOK, NotebookNoteListResponse{Items: items[start:end], Total: total, Page: page, Size: size})
}

func (h *Handler) getNotebookNote(c *gin.Context) {
	_, note, ok := h.requireNotebookNote(c)
	if ok {
		c.JSON(http.StatusOK, notebookNoteResponse(note))
	}
}

func (h *Handler) createNotebookNote(c *gin.Context) {
	notebook, ok := h.requireNotebook(c)
	if !ok {
		return
	}
	var request NotebookNoteRequest
	if !readNotebookRequest(c, &request) {
		return
	}
	if request.Slug == nil || request.StellarVersion == nil || request.StellarConfig == nil {
		notebookError(c, fmt.Errorf("%w: slug, stellar_version and stellar_config are required", store.ErrNoteInvalid))
		return
	}
	note := &store.Note{NotebookID: notebook.ID, Slug: *request.Slug, Status: store.StatusDraft, AuthorID: c.GetInt64("userID"), BlogExtensions: "{}"}
	if !applyNotebookNoteRequest(c, note, request) || !h.prepareNotebookNote(c, note) {
		return
	}
	if _, err := h.store.CreateNote(note); err != nil {
		notebookError(c, err)
		return
	}
	h.publishContentChanged()
	c.JSON(http.StatusCreated, notebookNoteResponse(note))
}

func (h *Handler) updateNotebookNote(c *gin.Context) {
	h.saveNotebookNote(c, "")
}

func (h *Handler) publishNotebookNote(c *gin.Context) {
	h.saveNotebookNote(c, store.StatusPublished)
}

func (h *Handler) draftNotebookNote(c *gin.Context) {
	h.saveNotebookNote(c, store.StatusDraft)
}

func (h *Handler) saveNotebookNote(c *gin.Context, status string) {
	_, note, ok := h.requireNotebookNote(c)
	if !ok {
		return
	}
	var request NotebookNoteRequest
	if !readNotebookRequest(c, &request) || !applyNotebookNoteRequest(c, note, request) {
		return
	}
	if status != "" {
		note.Status = status
	}
	if !h.prepareNotebookNote(c, note) {
		return
	}
	if err := h.store.UpdateNote(note); err != nil {
		notebookError(c, err)
		return
	}
	h.publishContentChanged()
	c.JSON(http.StatusOK, notebookNoteResponse(note))
}

func (h *Handler) deleteNotebookNote(c *gin.Context) {
	_, note, ok := h.requireNotebookNote(c)
	if !ok {
		return
	}
	if err := h.store.DeleteNote(note.ID); err != nil {
		notebookError(c, err)
		return
	}
	h.publishContentChanged()
	c.JSON(http.StatusOK, NotebookWriteResponse{OK: true})
}

func (h *Handler) requireNotebookNote(c *gin.Context) (*store.Notebook, *store.Note, bool) {
	notebook, ok := h.requireNotebook(c)
	if !ok {
		return nil, nil, false
	}
	id, ok := h.parseIDParam(c)
	if !ok {
		return nil, nil, false
	}
	note, err := h.store.GetNoteByID(id)
	if err != nil {
		notebookError(c, err)
		return nil, nil, false
	}
	if note == nil || note.NotebookID != notebook.ID {
		notebookError(c, store.ErrNoteNotFound)
		return nil, nil, false
	}
	if !notebookOwned(c, note.AuthorID) {
		AbortFail(c, http.StatusForbidden, "只能管理自己的笔记")
		return nil, nil, false
	}
	if note.StellarVersion != 2 {
		notebookError(c, fmt.Errorf("%w: notebooks/%s/%s", store.ErrStellarMigrationRequired, note.NotebookID, note.Slug))
		return nil, nil, false
	}
	if _, err := note.ValidateStellar(); err != nil {
		notebookError(c, err)
		return nil, nil, false
	}
	return notebook, note, true
}

func applyNotebookNoteRequest(c *gin.Context, note *store.Note, request NotebookNoteRequest) bool {
	if request.NotebookID != nil && *request.NotebookID != note.NotebookID {
		notebookError(c, fmt.Errorf("%w: note source notebook cannot change", store.ErrNoteInvalid))
		return false
	}
	if request.Slug != nil {
		note.Slug = *request.Slug
	}
	if request.ContentMD != nil {
		note.ContentMD = *request.ContentMD
	}
	if request.Status != nil {
		note.Status = *request.Status
	}
	if request.Tags != nil {
		note.Tags = append([]string(nil), (*request.Tags)...)
	}
	if request.StellarVersion != nil {
		note.StellarVersion = *request.StellarVersion
	}
	if request.StellarConfig != nil {
		note.StellarConfig = *request.StellarConfig
	}
	if request.BlogExtensions != nil {
		note.BlogExtensions = *request.BlogExtensions
	}
	return true
}

func (h *Handler) prepareNotebookNote(c *gin.Context, note *store.Note) bool {
	if note.StellarVersion != 2 {
		notebookError(c, fmt.Errorf("%w: notebooks/%s/%s", store.ErrStellarMigrationRequired, note.NotebookID, note.Slug))
		return false
	}
	if note.Status != store.StatusDraft && note.Status != store.StatusPublished {
		notebookError(c, store.ErrNoteInvalid)
		return false
	}
	parsed, err := note.ValidateStellar()
	if err != nil {
		notebookError(c, fmt.Errorf("%w: %w", store.ErrNoteInvalid, err))
		return false
	}
	note.Title = notebookText(parsed.Value, "title", note.Slug)
	// 完整渲染由固定 v2 renderer 接入，不能退回旧标签 parser。
	return h.renderNotebookNote(c, note, parsed.Value)
}
