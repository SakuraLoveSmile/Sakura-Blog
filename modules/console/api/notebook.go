package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"blog/internal/stellarv2"
	"blog/internal/store"

	"github.com/gin-gonic/gin"
)

type NotebookRequest struct {
	ID             *string `json:"id,omitempty"`
	StellarVersion *int    `json:"stellar_version,omitempty"`
	StellarConfig  *string `json:"stellar_config,omitempty"`
	BlogExtensions *string `json:"blog_extensions,omitempty"`
}

type NotebookResponse struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Icon           string    `json:"icon"`
	Cover          string    `json:"cover"`
	Sort           int       `json:"sort"`
	AuthorID       int64     `json:"author_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	StellarVersion int       `json:"stellar_version"`
	StellarConfig  string    `json:"stellar_config"`
	BlogExtensions string    `json:"blog_extensions"`
}

type NotebookListResponse struct {
	Items []NotebookResponse `json:"items"`
	Total int                `json:"total"`
	Page  int                `json:"page"`
	Size  int                `json:"size"`
}

type NotebookWriteResponse struct {
	OK bool `json:"ok"`
}

func notebookResponse(p *store.Notebook) NotebookResponse {
	return NotebookResponse{ID: p.ID, Name: p.Name, Description: p.Description, Icon: p.Icon, Cover: p.Cover, Sort: p.Sort, AuthorID: p.AuthorID, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt, StellarVersion: p.StellarVersion, StellarConfig: p.StellarConfig, BlogExtensions: p.BlogExtensions}
}

func (h *Handler) listNotebooks(c *gin.Context) {
	page, size, ok := notebookPagination(c)
	if !ok {
		return
	}
	all, err := h.store.ListNotebooks()
	if err != nil {
		notebookError(c, err)
		return
	}
	items := []NotebookResponse{}
	query := strings.ToLower(c.Query("q"))
	for _, notebook := range all {
		if !notebookOwned(c, notebook.AuthorID) {
			continue
		}
		if _, err := notebook.ValidateStellar(); err != nil {
			notebookError(c, err)
			return
		}
		if query != "" && !strings.Contains(strings.ToLower(notebook.ID+" "+notebook.Name+" "+notebook.Description), query) {
			continue
		}
		items = append(items, notebookResponse(notebook))
	}
	total := len(items)
	start, end := notebookPageBounds(total, page, size)
	c.JSON(http.StatusOK, NotebookListResponse{Items: items[start:end], Total: total, Page: page, Size: size})
}

func (h *Handler) getNotebook(c *gin.Context) {
	notebook, ok := h.requireNotebook(c)
	if ok {
		c.JSON(http.StatusOK, notebookResponse(notebook))
	}
}

func (h *Handler) createNotebook(c *gin.Context) {
	var request NotebookRequest
	if !readNotebookRequest(c, &request) {
		return
	}
	if request.ID == nil || request.StellarVersion == nil || request.StellarConfig == nil {
		notebookError(c, fmt.Errorf("%w: id, stellar_version and stellar_config are required", store.ErrNotebookInvalid))
		return
	}
	notebook := &store.Notebook{ID: strings.TrimSpace(*request.ID), AuthorID: c.GetInt64("userID"), BlogExtensions: "{}"}
	applyNotebookRequest(notebook, request)
	if !prepareNotebook(c, notebook) {
		return
	}
	if err := h.store.CreateNotebook(notebook); err != nil {
		notebookError(c, err)
		return
	}
	h.publishContentChanged()
	c.JSON(http.StatusCreated, notebookResponse(notebook))
}

func (h *Handler) updateNotebook(c *gin.Context) {
	notebook, ok := h.requireNotebook(c)
	if !ok {
		return
	}
	var request NotebookRequest
	if !readNotebookRequest(c, &request) {
		return
	}
	if request.ID != nil && *request.ID != notebook.ID {
		notebookError(c, fmt.Errorf("%w: notebook source identity cannot change", store.ErrNotebookInvalid))
		return
	}
	applyNotebookRequest(notebook, request)
	if !prepareNotebook(c, notebook) {
		return
	}
	if err := h.store.UpdateNotebook(notebook.ID, notebook); err != nil {
		notebookError(c, err)
		return
	}
	h.publishContentChanged()
	c.JSON(http.StatusOK, notebookResponse(notebook))
}

func (h *Handler) deleteNotebook(c *gin.Context) {
	notebook, ok := h.requireNotebook(c)
	if !ok {
		return
	}
	if err := h.store.DeleteNotebook(notebook.ID); err != nil {
		notebookError(c, err)
		return
	}
	h.publishContentChanged()
	c.JSON(http.StatusOK, NotebookWriteResponse{OK: true})
}

func applyNotebookRequest(p *store.Notebook, request NotebookRequest) {
	if request.StellarVersion != nil {
		p.StellarVersion = *request.StellarVersion
	}
	if request.StellarConfig != nil {
		p.StellarConfig = *request.StellarConfig
	}
	if request.BlogExtensions != nil {
		p.BlogExtensions = *request.BlogExtensions
	}
}

func prepareNotebook(c *gin.Context, p *store.Notebook) bool {
	if p.StellarVersion != 2 {
		notebookError(c, fmt.Errorf("%w: notebooks/%s", store.ErrStellarMigrationRequired, p.ID))
		return false
	}
	parsed, err := p.ValidateStellar()
	if err != nil {
		notebookError(c, fmt.Errorf("%w: %w", store.ErrNotebookInvalid, err))
		return false
	}
	p.Name = notebookText(parsed.Value, "name", p.ID)
	p.Description = notebookText(parsed.Value, "description", "")
	p.Icon = notebookText(parsed.Value, "icon", "")
	p.Cover = notebookText(parsed.Value, "cover", "")
	p.Sort = notebookInteger(parsed.Value, "listing.order")
	return true
}

func notebookOwned(c *gin.Context, authorID int64) bool {
	return contextHasPermission(c, store.PermissionContentManageAny) || (authorID > 0 && authorID == c.GetInt64("userID"))
}

func (h *Handler) requireNotebook(c *gin.Context) (*store.Notebook, bool) {
	notebook, err := h.store.GetNotebook(c.Param("notebook"))
	if err != nil {
		notebookError(c, err)
		return nil, false
	}
	if notebook == nil {
		notebookError(c, store.ErrNotebookNotFound)
		return nil, false
	}
	if !notebookOwned(c, notebook.AuthorID) {
		AbortFail(c, http.StatusForbidden, "只能管理自己的笔记本")
		return nil, false
	}
	if notebook.StellarVersion != 2 {
		notebookError(c, fmt.Errorf("%w: notebooks/%s", store.ErrStellarMigrationRequired, notebook.ID))
		return nil, false
	}
	if _, err := notebook.ValidateStellar(); err != nil {
		notebookError(c, err)
		return nil, false
	}
	return notebook, true
}

func readNotebookRequest(c *gin.Context, target any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSettingsBodyBytes)
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var limit *http.MaxBytesError
		status := http.StatusBadRequest
		if errors.As(err, &limit) {
			status = http.StatusRequestEntityTooLarge
		}
		Fail(c, status, "笔记请求读取失败")
		return false
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		Fail(c, http.StatusBadRequest, "笔记请求必须为 JSON 对象")
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		Fail(c, http.StatusBadRequest, "笔记请求 JSON 无效")
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		Fail(c, http.StatusBadRequest, "笔记请求必须为一个 JSON 对象")
		return false
	}
	return true
}

func notebookError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	var schema *stellarv2.SchemaError
	switch {
	case errors.Is(err, store.ErrStellarMigrationRequired):
		Fail(c, http.StatusConflict, err.Error())
		return
	case errors.As(err, &schema):
		stellarSettingsError(c, err, http.StatusBadRequest)
		return
	case errors.Is(err, store.ErrNotebookNotFound), errors.Is(err, store.ErrNoteNotFound):
		status = http.StatusNotFound
	case errors.Is(err, store.ErrNotebookConflict), errors.Is(err, store.ErrNoteConflict), errors.Is(err, store.ErrNotebookInUse):
		status = http.StatusConflict
	case errors.Is(err, store.ErrNotebookInvalid), errors.Is(err, store.ErrNoteInvalid):
		status = http.StatusBadRequest
	}
	Fail(c, status, err.Error())
}

func notebookPagination(c *gin.Context) (int, int, bool) {
	page, size := 1, 20
	for key, target := range map[string]*int{"page": &page, "size": &size} {
		if raw := c.Query(key); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || (key == "size" && value > 100) {
				Fail(c, http.StatusBadRequest, "分页参数无效")
				return 0, 0, false
			}
			*target = value
		}
	}
	return page, size, true
}

func notebookPageBounds(total, page, size int) (int, int) {
	if page-1 > total/size {
		return total, total
	}
	start := (page - 1) * size
	end := start + size
	if end > total {
		end = total
	}
	return start, end
}

func notebookText(object map[string]any, path, fallback string) string {
	value, _ := stellarv2.Get(object, path)
	text, ok := value.(string)
	if !ok || text == "" {
		return fallback
	}
	return text
}

func notebookInteger(object map[string]any, path string) int {
	value, _ := stellarv2.Get(object, path)
	switch n := value.(type) {
	case json.Number:
		parsed, _ := n.Int64()
		return int(parsed)
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}
