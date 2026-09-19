package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/backup"
	"blog/internal/maintenance"
)

const (
	maxRestoreIdempotencyKey = 128
	maxRestoreJSONBody       = 4 << 10
)

// RestoreTaskResponse is the public representation of a restore task.
type RestoreTaskResponse struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	HasPreimage bool   `json:"has_preimage"`
}

type RestoreTasksResponse struct {
	Tasks []RestoreTaskResponse `json:"tasks"`
}

type RestoreCompletedResponse struct {
	Restored bool `json:"restored"`
}

type restoreSnapshotRequest struct {
	Snapshot string `json:"snapshot"`
	Strict   bool   `json:"strict"`
}

func restoreTaskResponse(task backup.RestoreTask) RestoreTaskResponse {
	return RestoreTaskResponse{
		ID:          task.ID,
		Status:      task.Status,
		Error:       task.Error,
		CreatedAt:   task.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt:   task.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		HasPreimage: task.HasPreimage,
	}
}

func restoreTaskErrorStatus(err error) int {
	if err == nil {
		return http.StatusInternalServerError
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "already active") || strings.Contains(message, "idempotency key already used") || errors.Is(err, maintenance.ErrBusy) || errors.Is(err, backup.ErrRestoreConflict) {
		return http.StatusConflict
	}
	if errors.Is(err, backup.ErrRestoreNotFound) || errors.Is(err, os.ErrNotExist) || strings.Contains(message, "not found") {
		return http.StatusNotFound
	}
	return http.StatusBadRequest
}

func restoreTaskFailure(c *gin.Context, err error) {
	status := restoreTaskErrorStatus(err)
	message := "恢复请求失败，请检查备份格式和服务器日志"
	if status == http.StatusConflict {
		message = "已有恢复或更新任务，或幂等键对应不同内容"
	}
	if status == http.StatusNotFound {
		message = "恢复任务或备份不存在"
	}
	Fail(c, status, message)
}

// startRestore accepts either a multipart file or a JSON snapshot reference.
// The source forms are deliberately exclusive so a request cannot silently
// restore a different source than the caller confirmed.
func (h *Handler) startRestore(c *gin.Context) {
	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if idempotencyKey == "" || len(idempotencyKey) > maxRestoreIdempotencyKey {
		Fail(c, http.StatusBadRequest, "Idempotency-Key 必填且长度不能超过 128 个字符")
		return
	}

	input := backup.RestoreInput{IdempotencyKey: idempotencyKey}
	contentType := strings.ToLower(c.GetHeader("Content-Type"))
	switch {
	case strings.HasPrefix(contentType, "multipart/form-data"):
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRestoreSize+64<<10)
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				Fail(c, http.StatusRequestEntityTooLarge, "备份包超过 500MB 上限")
				return
			}
			Fail(c, http.StatusBadRequest, "缺少上传文件（字段 file）")
			return
		}
		defer file.Close()
		if header.Size > maxRestoreSize {
			Fail(c, http.StatusRequestEntityTooLarge, "备份包超过 500MB 上限")
			return
		}
		input.Data, err = io.ReadAll(io.LimitReader(file, maxRestoreSize+1))
		if err != nil {
			Fail(c, http.StatusInternalServerError, "读取上传文件失败")
			return
		}
		if len(input.Data) > maxRestoreSize {
			Fail(c, http.StatusRequestEntityTooLarge, "备份包超过 500MB 上限")
			return
		}
		if c.PostForm("snapshot") != "" || len(c.Request.MultipartForm.File["file"]) != 1 {
			Fail(c, http.StatusBadRequest, "只能选择一个恢复来源")
			return
		}
		defer c.Request.MultipartForm.RemoveAll()
		input.Strict = c.PostForm("strict") == "true"
	case strings.HasPrefix(contentType, "application/json"):
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRestoreJSONBody)
		var req restoreSnapshotRequest
		raw, readErr := io.ReadAll(c.Request.Body)
		if readErr != nil {
			var maxErr *http.MaxBytesError
			if errors.As(readErr, &maxErr) {
				Fail(c, http.StatusRequestEntityTooLarge, "请求体超过 4KB 上限")
			} else {
				Fail(c, http.StatusBadRequest, "读取请求体失败")
			}
			return
		}
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				Fail(c, http.StatusRequestEntityTooLarge, "请求体超过 4KB 上限")
				return
			}
			Fail(c, http.StatusBadRequest, "snapshot 必填且请求格式必须为 JSON")
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			Fail(c, http.StatusBadRequest, "只允许一个 JSON 对象")
			return
		}
		if strings.TrimSpace(req.Snapshot) == "" {
			Fail(c, http.StatusBadRequest, "snapshot 必填且请求格式必须为 JSON")
			return
		}
		input.Snapshot = strings.TrimSpace(req.Snapshot)
		input.Strict = req.Strict
	default:
		Fail(c, http.StatusBadRequest, "Content-Type 必须为 multipart/form-data 或 application/json")
		return
	}

	task, err := h.backups.StartRestore(c.Request.Context(), input)
	if err != nil {
		restoreTaskFailure(c, err)
		return
	}
	c.JSON(http.StatusAccepted, restoreTaskResponse(task))
}

func (h *Handler) listRestoreTasks(c *gin.Context) {
	tasks, err := h.backups.ListRestoreTasks()
	if err != nil {
		restoreTaskFailure(c, err)
		return
	}
	response := RestoreTasksResponse{Tasks: make([]RestoreTaskResponse, 0, len(tasks))}
	for _, task := range tasks {
		response.Tasks = append(response.Tasks, restoreTaskResponse(task))
	}
	c.JSON(http.StatusOK, response)
}

func (h *Handler) getRestoreTask(c *gin.Context) {
	task, err := h.backups.GetRestoreTask(c.Param("id"))
	if err != nil {
		restoreTaskFailure(c, err)
		return
	}
	c.JSON(http.StatusOK, restoreTaskResponse(task))
}

// waitLegacyRestore keeps the old synchronous endpoint's response contract
// while sharing the asynchronous engine. The request lease must be released
// before waiting because the restore itself starts a maintenance window.
func (h *Handler) waitLegacyRestore(c *gin.Context, input backup.RestoreInput) error {
	task, err := h.backups.StartRestore(c.Request.Context(), input)
	if err != nil {
		return err
	}
	maintenance.ReleaseRequest(c.Request.Context())
	return h.backups.WaitRestore(context.Background(), task.ID)
}
