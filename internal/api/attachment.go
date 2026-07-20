package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// 允许的图片扩展名与对应 Content-Type。
var allowedExtMIME = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
}

// attachmentJSON 附件 API 响应模型。
type attachmentJSON struct {
	ID        int64  `json:"id"`
	Filename  string `json:"filename"`
	URL       string `json:"url"`
	Mime      string `json:"mime"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"created_at"`
}

func toAttachmentJSON(a *store.Attachment) *attachmentJSON {
	return &attachmentJSON{
		ID:        a.ID,
		Filename:  a.Filename,
		URL:       a.URL,
		Mime:      a.Mime,
		Size:      a.Size,
		CreatedAt: a.CreatedAt.Format(time.RFC3339),
	}
}

// listAttachments GET /api/v1/attachments：附件列表（分页）。
func (h *Handler) listAttachments(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "24"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 24
	}

	total, err := h.store.CountAttachments()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	attachments, err := h.store.ListAttachments(page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	items := make([]*attachmentJSON, 0, len(attachments))
	for _, a := range attachments {
		items = append(items, toAttachmentJSON(a))
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// uploadAttachment POST /api/v1/attachments/upload：上传文件。
func (h *Handler) uploadAttachment(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未选择文件"})
		return
	}
	defer file.Close()

	if header.Size > h.maxUpload {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("文件超过 %d MB 限制", h.maxUpload>>20)})
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	mime, ok := allowedExtMIME[ext]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的文件类型: " + ext})
		return
	}

	// 生成随机文件名，按年月分目录
	now := time.Now()
	subDir := now.Format("2006/01")
	randName := randomHex(8) + ext
	relPath := filepath.Join(subDir, randName)
	absPath := filepath.Join(h.uploadDir, relPath)

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建目录失败"})
		return
	}

	dst, err := os.Create(absPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建文件失败"})
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		os.Remove(absPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入文件失败"})
		return
	}

	url := "/uploads/" + filepath.ToSlash(relPath)
	att := &store.Attachment{
		Filename: header.Filename,
		Path:     filepath.ToSlash(relPath),
		URL:      url,
		Mime:     mime,
		Size:     written,
	}
	if _, err := h.store.CreateAttachment(att); err != nil {
		os.Remove(absPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存记录失败"})
		return
	}

	c.JSON(http.StatusCreated, toAttachmentJSON(att))
}

// deleteAttachment DELETE /api/v1/attachments/:id：删除附件。
func (h *Handler) deleteAttachment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的附件 ID"})
		return
	}

	// 先查记录获取文件路径
	att, err := h.store.GetAttachment(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if att == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "附件不存在"})
		return
	}

	// 删除磁盘文件（失败不阻塞 DB 删除）
	if att.Path != "" {
		os.Remove(filepath.Join(h.uploadDir, filepath.FromSlash(att.Path)))
	}

	if err := h.store.DeleteAttachment(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
