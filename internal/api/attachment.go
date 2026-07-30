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
	ThumbURL  string `json:"thumb_url"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	GroupID   int64  `json:"group_id"`
}

func toAttachmentJSON(a *store.Attachment) *attachmentJSON {
	return &attachmentJSON{
		ID:        a.ID,
		Filename:  a.Filename,
		URL:       a.URL,
		Mime:      a.Mime,
		Size:      a.Size,
		CreatedAt: a.CreatedAt.Format(time.RFC3339),
		ThumbURL:  a.ThumbURL,
		Width:     a.Width,
		Height:    a.Height,
		GroupID:   a.GroupID,
	}
}

// listAttachments GET /api/v1/attachments：附件列表（分页 + 搜索筛选）。
func (h *Handler) listAttachments(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "24"))
	keyword := strings.TrimSpace(c.Query("keyword"))
	kind := c.Query("kind") // "" | "image" | "other"
	groupID := parseGroupParam(c.Query("group"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 24
	}

	total, err := h.store.CountAttachments(keyword, kind, groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	attachments, err := h.store.ListAttachments(page, size, keyword, kind, groupID)
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

// uploadAttachment POST /api/v1/attachments/upload：上传文件（图片自动优化 + 缩略图）。
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

	// 读取全部数据用于图像优化
	srcData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}

	// 图像优化管线
	now := time.Now()
	subDir := now.Format("2006/01")
	var mainData []byte
	var outMime string
	var outExt string
	var imgW, imgH int
	var thumbData []byte

	if strings.HasPrefix(mime, "image/") {
		res, optErr := optimizeImage(srcData, mime)
		if optErr == nil && res != nil {
			mainData = res.MainData
			outMime = res.OutMime
			outExt = res.OutExt
			imgW = res.Width
			imgH = res.Height
			thumbData = res.ThumbData
		} else {
			mainData = srcData
			outMime = mime
			outExt = ext
		}
	} else {
		mainData = srcData
		outMime = mime
		outExt = ext
	}

	// 生成随机文件名
	randName := randomHex(8) + outExt
	relPath := filepath.Join(subDir, randName)
	absPath := filepath.Join(h.uploadDir, relPath)

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建目录失败"})
		return
	}

	// 写入主图
	if err := os.WriteFile(absPath, mainData, 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入文件失败"})
		return
	}

	url := "/uploads/" + filepath.ToSlash(relPath)

	// 写入缩略图
	var thumbPath, thumbURL string
	if len(thumbData) > 0 {
		thumbName := "thumb_" + randName
		thumbRel := filepath.Join(subDir, thumbName)
		thumbAbs := filepath.Join(h.uploadDir, thumbRel)
		if err := os.WriteFile(thumbAbs, thumbData, 0o644); err == nil {
			thumbPath = filepath.ToSlash(thumbRel)
			thumbURL = "/uploads/" + thumbPath
		}
	}

	att := &store.Attachment{
		Filename:  header.Filename,
		Path:      filepath.ToSlash(relPath),
		URL:       url,
		Mime:      outMime,
		Size:      int64(len(mainData)),
		ThumbPath: thumbPath,
		ThumbURL:  thumbURL,
		Width:     imgW,
		Height:    imgH,
		GroupID:   uploadGroupID(c, h.store),
	}
	if _, err := h.store.CreateAttachment(att); err != nil {
		os.Remove(absPath)
		if thumbPath != "" {
			os.Remove(filepath.Join(h.uploadDir, filepath.FromSlash(thumbPath)))
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存记录失败"})
		return
	}

	c.JSON(http.StatusCreated, toAttachmentJSON(att))
}

// deleteAttachment DELETE /api/v1/attachments/:id：删除附件（连带缩略图）。
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
	// 连带删除缩略图
	if att.ThumbPath != "" {
		os.Remove(filepath.Join(h.uploadDir, filepath.FromSlash(att.ThumbPath)))
	}

	if err := h.store.DeleteAttachment(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// attachmentRef 引用来源信息。
type attachmentRef struct {
	Type  string `json:"type"`  // "post" | "page" | "moment"
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

// attachmentRefs GET /api/v1/attachments/:id/refs：查询附件被哪些文章/页面引用。
func (h *Handler) attachmentRefs(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的附件 ID"})
		return
	}
	att, err := h.store.GetAttachment(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if att == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "附件不存在"})
		return
	}

	refs := h.scanAttachmentRefs(att.URL)
	c.JSON(http.StatusOK, gin.H{"refs": refs, "count": len(refs)})
}

// scanAttachmentRefs 在 posts + pages 的 content 中扫描 URL 引用。
func (h *Handler) scanAttachmentRefs(url string) []attachmentRef {
	var refs []attachmentRef
	pattern := "%" + url + "%"

	// 扫描文章
	rows, err := h.store.DB().Query(`SELECT slug, title FROM posts WHERE content LIKE ? AND deleted_at = ''`, pattern)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var slug, title string
			if rows.Scan(&slug, &title) == nil {
				refs = append(refs, attachmentRef{Type: "post", Slug: slug, Title: title})
			}
		}
	}

	// 扫描页面
	rows2, err2 := h.store.DB().Query(`SELECT slug, title FROM pages WHERE content LIKE ? AND deleted_at = ''`, pattern)
	if err2 == nil {
		defer rows2.Close()
		for rows2.Next() {
			var slug, title string
			if rows2.Scan(&slug, &title) == nil {
				refs = append(refs, attachmentRef{Type: "page", Slug: slug, Title: title})
			}
		}
	}

	// 扫描瞬间（Spec 43：images 列存 uploads 相对路径 JSON 数组，直接 LIKE 匹配 URL）
	rows3, err3 := h.store.DB().Query(`SELECT id, content_md FROM moments WHERE images LIKE ?`, pattern)
	if err3 == nil {
		defer rows3.Close()
		for rows3.Next() {
			var id int64
			var contentMD string
			if rows3.Scan(&id, &contentMD) == nil {
				refs = append(refs, attachmentRef{Type: "moment", Slug: strconv.FormatInt(id, 10), Title: momentRefTitle(id, contentMD)})
			}
		}
	}

	if refs == nil {
		refs = []attachmentRef{}
	}
	return refs
}

// momentRefTitle 为瞬间引用生成可读标题：取正文首行缩略，空则回落 “瞬间 #id”。
func momentRefTitle(id int64, contentMD string) string {
	text := strings.TrimSpace(contentMD)
	if i := strings.IndexAny(text, "\r\n"); i >= 0 {
		text = strings.TrimSpace(text[:i])
	}
	if text == "" {
		return "瞬间 #" + strconv.FormatInt(id, 10)
	}
	r := []rune(text)
	if len(r) > 20 {
		text = string(r[:20]) + "…"
	}
	return "瞬间：" + text
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// parseGroupParam 解析列表 group 查询参数：空字符串=不按组筛选（返回 nil）；
// "0"=仅未分组；其余数字=指定组。非法值当作不筛选。
func parseGroupParam(raw string) *int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v < 0 {
		return nil
	}
	return &v
}

// uploadGroupID 从上传表单取目标分组；缺省/非法/不存在的组均归为未分组。
func uploadGroupID(c *gin.Context, st *store.Store) int64 {
	raw := strings.TrimSpace(c.PostForm("group_id"))
	if raw == "" {
		return store.UngroupedGroupID
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		return store.UngroupedGroupID
	}
	if ok, _ := st.AttachmentGroupExists(v); !ok {
		return store.UngroupedGroupID
	}
	return v
}

// --- 分组端点（Spec 44） ---

// attachmentGroupJSON 分组 API 响应模型。
type attachmentGroupJSON struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Order int    `json:"order"`
	Count int    `json:"count"`
}

// listAttachmentGroups GET /api/v1/attachment-groups：列出全部分组（含组内附件数）。
func (h *Handler) listAttachmentGroups(c *gin.Context) {
	groups, err := h.store.ListAttachmentGroups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]*attachmentGroupJSON, 0, len(groups))
	for _, g := range groups {
		items = append(items, &attachmentGroupJSON{ID: g.ID, Name: g.Name, Order: g.Order, Count: g.Count})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// createAttachmentGroup POST /api/v1/attachment-groups：新建分组。
func (h *Handler) createAttachmentGroup(c *gin.Context) {
	var body struct {
		Name  string `json:"name"`
		Order int    `json:"order"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效请求体"})
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分组名称不能为空"})
		return
	}
	g := &store.AttachmentGroup{Name: name, Order: body.Order}
	if _, err := h.store.CreateAttachmentGroup(g); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, &attachmentGroupJSON{ID: g.ID, Name: g.Name, Order: g.Order})
}

// updateAttachmentGroup PUT /api/v1/attachment-groups/:id：重命名/调排序。
func (h *Handler) updateAttachmentGroup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的分组 ID"})
		return
	}
	var body struct {
		Name  string `json:"name"`
		Order int    `json:"order"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效请求体"})
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分组名称不能为空"})
		return
	}
	if err := h.store.UpdateAttachmentGroup(id, name, body.Order); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分组不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// deleteAttachmentGroup DELETE /api/v1/attachment-groups/:id：删组（组内附件归未分组）。
func (h *Handler) deleteAttachmentGroup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的分组 ID"})
		return
	}
	if err := h.store.DeleteAttachmentGroup(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分组不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// moveAttachments POST /api/v1/attachments/move：批量移动附件到目标分组（group_id=0 为未分组）。
func (h *Handler) moveAttachments(c *gin.Context) {
	var body struct {
		IDs     []int64 `json:"ids"`
		GroupID int64   `json:"group_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效请求体"})
		return
	}
	if len(body.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未选择附件"})
		return
	}
	if len(body.IDs) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "单次最多移动 200 项"})
		return
	}
	if body.GroupID < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的目标分组"})
		return
	}
	ok, err := h.store.AttachmentGroupExists(body.GroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "目标分组不存在"})
		return
	}
	moved, err := h.store.MoveAttachments(body.IDs, body.GroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"moved": moved})
}
