package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/backup"
	"blog/internal/stellarmigration"
	"blog/internal/stellarv2"
	"blog/internal/store"
)

// maxImportSize Markdown 导入上限（zip 含多篇文章，比单附件上限放宽到 50MB）。
const maxImportSize = 50 << 20

// maxRestoreSize 全量备份包上传上限（含 uploads，放宽到 500MB）。
const maxRestoreSize = 500 << 20

type MarkdownImportMissingTopicsProblem struct {
	Type          string   `json:"type"`
	Title         string   `json:"title"`
	Status        int      `json:"status"`
	Detail        string   `json:"detail"`
	Instance      string   `json:"instance"`
	Error         string   `json:"error"`
	MissingTopics []string `json:"missing_topics"`
}

// MarkdownImportUnsupportedParameter 不返回参数值，尤其不返回密码或原 front matter。
type MarkdownImportUnsupportedParameter struct {
	Source        string `json:"source"`
	Path          string `json:"path"`
	Field         string `json:"field"`
	Code          string `json:"code"`
	Message       string `json:"message"`
	RequiresDraft bool   `json:"requires_draft"`
}

type MarkdownImportUnsupportedParametersProblem struct {
	Type                  string                               `json:"type"`
	Title                 string                               `json:"title"`
	Status                int                                  `json:"status"`
	Detail                string                               `json:"detail"`
	Instance              string                               `json:"instance"`
	Error                 string                               `json:"error"`
	UnsupportedParameters []MarkdownImportUnsupportedParameter `json:"unsupported_parameters"`
}

type PostsMarkdownImportResponse struct {
	Imported int      `json:"imported"`
	Slugs    []string `json:"slugs"`
	Skipped  []string `json:"skipped"`
}

// restoreBackup POST /api/v1/backup/restore：上传全量备份 zip 一键还原（Spec 41）。
// 表单字段 file=zip，strict=true 时先清空 uploads 再解包。
// 安全审计 2026-08-16：FormFile 前先套 http.MaxBytesReader——此前 ParseMultipartForm
// 会把超限请求体整体解析（超出内存阈值部分落盘临时文件）后 header.Size 检查才生效，
// 认证用户可持续 stream 超大 multipart 体耗尽临时目录磁盘。
func (h *Handler) restoreBackup(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRestoreSize+64<<10)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		Fail(c, http.StatusBadRequest, "缺少上传文件（字段 file）")
		return
	}
	defer file.Close()
	if header.Size > maxRestoreSize {
		Fail(c, http.StatusRequestEntityTooLarge, "备份包超过 500MB 上限")
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxRestoreSize+1))
	if err != nil {
		Fail(c, http.StatusInternalServerError, "读取上传文件失败")
		return
	}
	if len(data) > maxRestoreSize {
		Fail(c, http.StatusRequestEntityTooLarge, "备份包超过 500MB 上限")
		return
	}
	strict := c.PostForm("strict") == "true"
	if err := h.waitLegacyRestore(c, backup.RestoreInput{Data: data, Strict: strict}); err != nil {
		Fail(c, http.StatusBadRequest, "恢复失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, RestoreCompletedResponse{Restored: true})
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
	if err := h.waitLegacyRestore(c, backup.RestoreInput{Snapshot: req.Name}); err != nil {
		Fail(c, http.StatusBadRequest, "恢复失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, RestoreCompletedResponse{Restored: true})
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
// 安全审计 2026-08-16：同 restoreBackup，FormFile 前套 MaxBytesReader（Spec 61 阶段 3
// 在附件/评论上传补齐了该防护，本端点当时遗漏）。
func (h *Handler) importPostsMarkdown(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImportSize+64<<10)
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
	var res *backup.ImportResult
	legacy := c.PostForm("legacy") == "true"
	var createTopics []string
	if value := c.PostForm("create_topics"); value != "" {
		if !legacy || json.Unmarshal([]byte(value), &createTopics) != nil {
			Fail(c, http.StatusBadRequest, "create_topics 必须是 1.x 导入中明确确认的专栏名称数组")
			return
		}
	}
	var ignored []backup.IgnoredMarkdownParameter
	if values, present := c.Request.MultipartForm.Value["ignored_parameters"]; present {
		valid := legacy && len(values) == 1
		if valid {
			decoder := json.NewDecoder(bytes.NewBufferString(values[0]))
			decoder.DisallowUnknownFields()
			valid = decoder.Decode(&ignored) == nil && ignored != nil && decoder.Decode(new(any)) == io.EOF
			for _, parameter := range ignored {
				valid = valid && parameter.Source != "" && parameter.Path != ""
			}
		}
		if !valid {
			Fail(c, http.StatusBadRequest, "ignored_parameters 必须是 1.x 导入中明确确认的 source、path 参数数组")
			return
		}
	}
	if legacy {
		res, err = h.backups.ImportLegacyMarkdownWithOptionsContext(c.Request.Context(), header.Filename, data, createTopics, ignored)
	} else {
		res, err = h.backups.ImportContext(c.Request.Context(), header.Filename, data)
	}
	if err != nil {
		var unsupported *stellarmigration.UnsupportedMarkdownParametersError
		if errors.As(err, &unsupported) {
			parameters := make([]MarkdownImportUnsupportedParameter, 0, len(unsupported.Parameters))
			for _, parameter := range unsupported.Parameters {
				parameters = append(parameters, MarkdownImportUnsupportedParameter{Source: parameter.Source, Path: parameter.Path, Field: parameter.Field, Code: parameter.Code, Message: parameter.Message, RequiresDraft: parameter.RequiresDraft})
			}
			message := unsupported.Error()
			c.JSON(http.StatusBadRequest, MarkdownImportUnsupportedParametersProblem{Type: "legacy_markdown_unsupported_parameters", Title: "需要确认不支持的参数", Status: http.StatusBadRequest, Detail: message, Error: message, Instance: c.Request.URL.Path, UnsupportedParameters: parameters})
			return
		}
		var missing *backup.MissingLegacyTopicsError
		if errors.As(err, &missing) {
			message := missing.Error()
			c.JSON(http.StatusBadRequest, MarkdownImportMissingTopicsProblem{
				Type: "legacy_markdown_topics_required", Title: "需要确认创建专栏", Status: http.StatusBadRequest,
				Detail: message, Error: message, Instance: c.Request.URL.Path,
				MissingTopics: missing.Topics,
			})
			return
		}
		var schema *stellarv2.SchemaError
		if !legacy && !errors.Is(err, store.ErrStellarMigrationRequired) && errors.As(err, &schema) {
			for _, issue := range schema.Issues {
				if strings.HasPrefix(issue.Source, "posts/") && (issue.Code == "removed_field" || issue.Path == "banner" && issue.ActualType == "string") {
					message := "检测到 1.x 文章字段，请确认使用“从 1.x 格式转换”后导入。"
					c.JSON(http.StatusBadRequest, Problem{
						Type: "legacy_markdown_requires_conversion", Title: "需要转换旧版文章", Status: http.StatusBadRequest,
						Detail: message, Error: message, Instance: c.Request.URL.Path,
					})
					return
				}
			}
		}
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, PostsMarkdownImportResponse{Imported: len(res.Imported), Slugs: res.Imported, Skipped: res.Skipped})
}

// backupStatus GET /api/v1/backup/status：自动备份配置 + 服务器留存备份列表。
func (h *Handler) backupStatus(c *gin.Context) {
	list, err := h.backups.ListBackups()
	if err != nil {
		Fail(c, http.StatusInternalServerError, "读取备份列表失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"auto_backup": h.backups.AutoEnabled(),
		"backup_keep": h.backups.Keep(),
		"backups":     list,
	})
}
