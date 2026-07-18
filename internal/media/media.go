// Package media 后台附件/图床管理：本地磁盘上传、列表、删除。
package media

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
	tmpl "blog/internal/template"
)

// 允许的图片扩展名与对应 Content-Type（双校验）。
var allowedExtMIME = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
}

// Handler 附件管理请求处理器。
type Handler struct {
	store       *store.Store
	engine      *tmpl.Engine
	site        *tmpl.Site
	uploadDir   string // 上传根目录（绝对路径）
	maxBytes    int64  // 单文件上限字节数
	maxUploadMB int
}

// New 创建 media handler，确保 attachments 表与上传目录存在。
func New(s *store.Store, engine *tmpl.Engine, site *tmpl.Site, uploadDir string, maxUploadMB int) (*Handler, error) {
	if maxUploadMB <= 0 {
		maxUploadMB = 5
	}
	if err := s.EnsureAttachmentsTable(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir upload dir %s: %w", uploadDir, err)
	}
	return &Handler{
		store:       s,
		engine:      engine,
		site:        site,
		uploadDir:   uploadDir,
		maxBytes:    int64(maxUploadMB) << 20,
		maxUploadMB: maxUploadMB,
	}, nil
}

// Register 挂 /admin/media 路由组，authMW 保护全部接口。
func (h *Handler) Register(r *gin.Engine, authMW gin.HandlerFunc) {
	g := r.Group("/admin/media", authMW)
	g.GET("", h.list)
	g.GET("/", h.list)
	g.POST("/upload", h.upload)
	g.POST("/delete/:id", h.delete)
}

const pageSize = 24

// list GET /admin/media：分页网格列表页。
func (h *Handler) list(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	total, err := h.store.CountAttachments()
	if err != nil {
		c.String(http.StatusInternalServerError, "count attachments: %v", err)
		return
	}
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages > 0 && page > totalPages {
		page = totalPages
	}
	atts, err := h.store.ListAttachments(page, pageSize)
	if err != nil {
		c.String(http.StatusInternalServerError, "list attachments: %v", err)
		return
	}
	rows := make([]*tmpl.AdminMediaRow, 0, len(atts))
	for _, a := range atts {
		rows = append(rows, &tmpl.AdminMediaRow{
			ID:       a.ID,
			Filename: a.Filename,
			URL:      a.URL,
			Mime:     a.Mime,
			Size:     humanSize(a.Size),
			Date:     a.CreatedAt.Format("2006-01-02 15:04"),
		})
	}
	pages := make([]int, totalPages)
	for i := range pages {
		pages[i] = i + 1
	}
	data := &tmpl.AdminMediaData{
		Site:        h.site,
		Page:        &tmpl.Page{Title: "附件管理", MenuID: "admin"},
		Items:       rows,
		CurrentPage: page,
		Pages:       pages,
		UploadError: c.Query("error"),
		MaxUploadMB: h.maxUploadMB,
	}
	html, err := h.engine.RenderMedia(data)
	if err != nil {
		c.String(http.StatusInternalServerError, "render admin/media: %v", err)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// upload POST /admin/media/upload：multipart 存盘 + 入库。
// 成功返回 JSON {"ok":true,"url":"...","id":N}；失败 400/500 + 原因。
func (h *Handler) upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxBytes+512<<10) // 略留 multipart 开销
	file, hdr, err := c.Request.FormFile("file")
	if err != nil {
		if strings.Contains(err.Error(), "request body too large") || err.Error() == "http: request body too large" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": fmt.Sprintf("文件超过 %d MB 上限", h.maxUploadMB)})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "请选择要上传的文件"})
		return
	}
	defer file.Close()

	if hdr.Size > 0 && hdr.Size > h.maxBytes {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": fmt.Sprintf("文件超过 %d MB 上限", h.maxUploadMB)})
		return
	}

	origName := filepath.Base(hdr.Filename)
	ext := strings.ToLower(filepath.Ext(origName))
	expectMIME, ok := allowedExtMIME[ext]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "仅支持 jpg/jpeg/png/gif/webp/svg 图片"})
		return
	}

	// Content-Type 双校验（浏览器可能给 application/octet-stream，仍以扩展名白名单为主，拒绝明确错误类型）
	ct := hdr.Header.Get("Content-Type")
	if ct != "" && ct != "application/octet-stream" && !strings.HasPrefix(ct, expectMIME) && !mimeCompatible(ct, expectMIME) {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "文件 Content-Type 与扩展名不匹配"})
		return
	}

	now := time.Now()
	relDir := now.Format("2006/01")
	storedName := fmt.Sprintf("%d_%s%s", now.Unix(), randomShort(), ext)
	relPath := filepath.ToSlash(filepath.Join(relDir, storedName))
	absDir := filepath.Join(h.uploadDir, filepath.FromSlash(relDir))
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "创建目录失败"})
		return
	}
	absPath := filepath.Join(absDir, storedName)

	out, err := os.OpenFile(absPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "写入文件失败"})
		return
	}
	written, copyErr := io.Copy(out, io.LimitReader(file, h.maxBytes+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		os.Remove(absPath)
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "保存文件失败"})
		return
	}
	if written > h.maxBytes {
		os.Remove(absPath)
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": fmt.Sprintf("文件超过 %d MB 上限", h.maxUploadMB)})
		return
	}

	url := "/uploads/" + relPath
	att := &store.Attachment{
		Filename:  origName,
		Path:      relPath,
		URL:       url,
		Mime:      expectMIME,
		Size:      written,
		CreatedAt: now,
	}
	id, err := h.store.CreateAttachment(att)
	if err != nil {
		os.Remove(absPath)
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "入库失败"})
		return
	}

	// 浏览器表单（无 JSON Accept）回列表；curl / API 返回 URL。
	if wantsJSON(c) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "url": url, "id": id})
		return
	}
	c.Redirect(http.StatusSeeOther, "/admin/media")
}

// delete POST /admin/media/delete/:id：删 DB 记录 + 磁盘文件（文件缺失忽略）。
func (h *Handler) delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.String(http.StatusBadRequest, "invalid attachment id")
		return
	}
	att, err := h.store.GetAttachment(id)
	if err != nil {
		c.String(http.StatusInternalServerError, "get attachment: %v", err)
		return
	}
	if att == nil {
		c.String(http.StatusNotFound, "attachment not found")
		return
	}
	// 先删文件（缺失忽略），再删记录
	_ = os.Remove(filepath.Join(h.uploadDir, filepath.FromSlash(att.Path)))
	if err := h.store.DeleteAttachment(id); err != nil {
		c.String(http.StatusInternalServerError, "delete: %v", err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/admin/media")
}

func wantsJSON(c *gin.Context) bool {
	accept := c.GetHeader("Accept")
	return strings.Contains(accept, "application/json") || c.Query("json") == "1"
}

func mimeCompatible(got, expect string) bool {
	got = strings.ToLower(strings.TrimSpace(strings.Split(got, ";")[0]))
	expect = strings.ToLower(expect)
	if got == expect {
		return true
	}
	// image/jpg 偶发别名
	if expect == "image/jpeg" && (got == "image/jpg" || got == "image/pjpeg") {
		return true
	}
	return false
}

func randomShort() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano()%1e8)
	}
	return hex.EncodeToString(b)
}

func humanSize(n int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
	)
	switch {
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
