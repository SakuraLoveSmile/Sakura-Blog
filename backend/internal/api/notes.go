package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// getNotesContent GET /api/v1/notes-content：读 content/notes.md 原文（文件不存在返回空串）。
func (h *Handler) getNotesContent(c *gin.Context) {
	content := ""
	if raw, err := os.ReadFile(h.notesPath); err == nil {
		content = string(raw)
	}
	c.JSON(http.StatusOK, gin.H{"content": content})
}

// putNotesContent PUT /api/v1/notes-content：写回 notes.md。前台 /notes 每次请求实时读文件，保存即生效。
func (h *Handler) putNotesContent(c *gin.Context) {
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if err := os.MkdirAll(filepath.Dir(h.notesPath), 0o755); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.WriteFile(h.notesPath, []byte(req.Content), 0o644); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
