package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"

	"blog/internal/media"
	"blog/internal/safefetch"
	"blog/internal/store"
)

type attachmentURLImportRequest struct {
	URL     string `json:"url"`
	GroupID int64  `json:"group_id"`
}

// importAttachmentURL POST /api/v1/attachments/import-url：安全抓取图片 URL 并入库。
func (h *Handler) importAttachmentURL(c *gin.Context) {
	var req attachmentURLImportRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.URL) == "" {
		Fail(c, http.StatusBadRequest, "URL 不能为空")
		return
	}
	target, err := safefetch.ValidateURL(req.URL)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, safefetch.ErrForbiddenHost) {
			status = http.StatusForbidden
		}
		Fail(c, status, "URL 不可用")
		return
	}

	client := safefetch.New(safefetch.Options{Timeout: 30 * time.Second, MaxRedirects: 5})
	request, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, target.String(), nil)
	if err != nil {
		Fail(c, http.StatusBadRequest, "URL 不可用")
		return
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 (compatible; SakuraBlog/1.0)")
	response, err := client.Do(request)
	if err != nil {
		Fail(c, http.StatusBadGateway, "远程文件抓取失败")
		return
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		Fail(c, http.StatusBadGateway, fmt.Sprintf("远程服务器返回 HTTP %d", response.StatusCode))
		return
	}

	data, err := io.ReadAll(io.LimitReader(response.Body, h.maxUpload+1))
	if err != nil {
		Fail(c, http.StatusBadGateway, "读取远程文件失败")
		return
	}
	if int64(len(data)) > h.maxUpload {
		Fail(c, http.StatusBadRequest, fmt.Sprintf("远程文件超过 %d MB 限制", h.maxUpload>>20))
		return
	}
	filename, mimeType, ok := attachmentURLFileInfo(target, response.Header.Get("Content-Type"))
	if !ok {
		Fail(c, http.StatusBadRequest, "远程文件不是支持的图片类型")
		return
	}

	groupID := store.UngroupedGroupID
	if req.GroupID > 0 {
		if exists, _ := h.store.AttachmentGroupExists(req.GroupID); exists {
			groupID = req.GroupID
		}
	}
	attachment, err := h.persistImportedAttachment(filename, mimeType, data, groupID, c.GetInt64("userID"))
	if err != nil {
		if errors.Is(err, errImportedImage) {
			Fail(c, http.StatusBadRequest, err.Error())
		} else {
			Fail(c, http.StatusInternalServerError, "保存附件失败")
		}
		return
	}
	c.JSON(http.StatusCreated, toAttachmentJSON(attachment))
}

var errImportedImage = errors.New("远程图片无法解析")

func attachmentURLFileInfo(target *url.URL, contentType string) (filename, mimeType string, ok bool) {
	mimeType = strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	ext := strings.ToLower(filepath.Ext(path.Base(target.Path)))
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = allowedExtMIME[ext]
	}
	if _, ok := allowedExtMIME[ext]; !ok || !strings.HasPrefix(mimeType, "image/") {
		for candidateExt, candidateMIME := range allowedExtMIME {
			if candidateMIME == mimeType {
				ext = candidateExt
				break
			}
		}
	}
	if _, ok := allowedExtMIME[ext]; !ok || mimeType != allowedExtMIME[ext] {
		return "", "", false
	}

	base := path.Base(target.Path)
	if decoded, err := url.PathUnescape(base); err == nil {
		base = decoded
	}
	base = strings.TrimSpace(base)
	base = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '/' || r == '\\' {
			return -1
		}
		return r
	}, base)
	if base == "" || base == "." || base == ".." {
		base = "remote" + ext
	}
	if filepath.Ext(base) == "" {
		base += ext
	}
	if len([]rune(base)) > 180 {
		base = string([]rune(base)[:180]) + ext
	}
	return base, mimeType, true
}

func (h *Handler) persistImportedAttachment(filename, mimeType string, srcData []byte, groupID, ownerID int64) (*store.Attachment, error) {
	res, err := media.OptimizeImage(srcData, mimeType)
	if err != nil {
		return nil, errImportedImage
	}
	mainData, outMime, outExt := srcData, mimeType, strings.ToLower(filepath.Ext(filename))
	var width, height int
	var thumbData []byte
	if res != nil {
		mainData, outMime, outExt = res.MainData, res.OutMime, res.OutExt
		width, height, thumbData = res.Width, res.Height, res.ThumbData
	}

	now := time.Now()
	subDir := now.Format("2006/01")
	randName := randomHex(8) + outExt
	relPath := filepath.Join(subDir, randName)
	if err := h.storage.Save(filepath.ToSlash(relPath), mainData); err != nil {
		return nil, err
	}
	thumbPath, thumbURL := "", ""
	if len(thumbData) > 0 {
		thumbRel := filepath.Join(subDir, "thumb_"+randName)
		if err := h.storage.Thumb(filepath.ToSlash(thumbRel), thumbData); err == nil {
			thumbPath = filepath.ToSlash(thumbRel)
			thumbURL = "/uploads/" + thumbPath
		}
	}
	attachment := &store.Attachment{
		Filename: filename, Path: filepath.ToSlash(relPath), URL: "/uploads/" + filepath.ToSlash(relPath),
		Mime: outMime, Size: int64(len(mainData)), ThumbPath: thumbPath, ThumbURL: thumbURL,
		Width: width, Height: height, GroupID: groupID,
		OwnerID: ownerID,
	}
	if _, err := h.store.CreateAttachment(attachment); err != nil {
		_ = h.storage.Remove(attachment.Path)
		if thumbPath != "" {
			_ = h.storage.Remove(thumbPath)
		}
		return nil, err
	}
	return attachment, nil
}
