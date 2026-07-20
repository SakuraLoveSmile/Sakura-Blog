package api

import (
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// maxImportSize Markdown 导入上限（zip 含多篇文章，比单附件上限放宽到 50MB）。
const maxImportSize = 50 << 20

// exportBackup GET /api/v1/backup/export：全量备份 zip 流式下载（DB 快照 + uploads + site.yml）。
func (h *Handler) exportBackup(c *gin.Context) {
	name := "blog-backup-" + time.Now().Format("20060102-150405") + ".zip"
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	if err := h.backups.WriteZip(c.Writer); err != nil {
		// 头已发出，无法回写 JSON 错误，记日志并断流
		c.Error(err) //nolint:errcheck
		c.Abort()
	}
}

// exportPostsMarkdown GET /api/v1/backup/export/markdown：全部文章导出为 Markdown zip。
func (h *Handler) exportPostsMarkdown(c *gin.Context) {
	name := "posts-export-" + time.Now().Format("20060102-150405") + ".zip"
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	if err := h.backups.WritePostsZip(c.Writer); err != nil {
		c.Error(err) //nolint:errcheck
		c.Abort()
	}
}

// importPostsMarkdown POST /api/v1/backup/import/markdown：上传 .md / .zip，按 slug UPSERT 导入文章。
func (h *Handler) importPostsMarkdown(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少上传文件（字段 file）"})
		return
	}
	defer file.Close()
	if header.Size > maxImportSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件超过 50MB 上限"})
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxImportSize+1))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取上传文件失败"})
		return
	}
	if len(data) > maxImportSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件超过 50MB 上限"})
		return
	}
	res, err := h.backups.Import(header.Filename, data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"imported": len(res.Imported),
		"slugs":    res.Imported,
		"skipped":  res.Skipped,
	})
}

// backupStatus GET /api/v1/backup/status：自动备份配置 + 服务器留存备份列表。
func (h *Handler) backupStatus(c *gin.Context) {
	list, err := h.backups.ListBackups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取备份列表失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"auto_backup": h.backups.AutoEnabled(),
		"backup_keep": h.backups.Keep(),
		"backups":     list,
	})
}
