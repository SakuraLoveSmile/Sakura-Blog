package api

import (
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// maxImportSize Markdown 导入上限（zip 含多篇文章，比单附件上限放宽到 50MB）。
const maxImportSize = 50 << 20

// maxRestoreSize 全量备份包上传上限（含 uploads，放宽到 500MB）。
const maxRestoreSize = 500 << 20

// restoreBackup POST /api/v1/backup/restore：上传全量备份 zip 一键还原（Spec 41）。
// 表单字段 file=zip，strict=true 时先清空 uploads 再解包。
func (h *Handler) restoreBackup(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		Fail(c, http.StatusBadRequest, "缺少上传文件（字段 file）")
		return
	}
	defer file.Close()
	if header.Size > maxRestoreSize {
		Fail(c, http.StatusBadRequest, "备份包超过 500MB 上限")
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxRestoreSize+1))
	if err != nil {
		Fail(c, http.StatusInternalServerError, "读取上传文件失败")
		return
	}
	if len(data) > maxRestoreSize {
		Fail(c, http.StatusBadRequest, "备份包超过 500MB 上限")
		return
	}
	strict := c.PostForm("strict") == "true"
	if err := h.backups.Restore(data, strict); err != nil {
		Fail(c, http.StatusBadRequest, "恢复失败: " + err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"restored": true})
}

// restoreSnapshot POST /api/v1/backup/restore-snapshot：从服务器 backups 目录内的备份恢复。
// name 白名单校验在 backup.Service 内（禁止路径穿越，仅目录内 .zip/.db）。
func (h *Handler) restoreSnapshot(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		Fail(c, http.StatusBadRequest, "缺少备份文件名（name）")
		return
	}
	if err := h.backups.RestoreSnapshot(req.Name); err != nil {
		Fail(c, http.StatusBadRequest, "恢复失败: " + err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"restored": true})
}

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
		Fail(c, http.StatusBadRequest, "缺少上传文件（字段 file）")
		return
	}
	defer file.Close()
	if header.Size > maxImportSize {
		Fail(c, http.StatusBadRequest, "文件超过 50MB 上限")
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxImportSize+1))
	if err != nil {
		Fail(c, http.StatusInternalServerError, "读取上传文件失败")
		return
	}
	if len(data) > maxImportSize {
		Fail(c, http.StatusBadRequest, "文件超过 50MB 上限")
		return
	}
	res, err := h.backups.Import(header.Filename, data)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
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
		Fail(c, http.StatusInternalServerError, "读取备份列表失败: " + err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"auto_backup": h.backups.AutoEnabled(),
		"backup_keep": h.backups.Keep(),
		"backups":     list,
	})
}
