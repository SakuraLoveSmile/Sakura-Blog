package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// 发布前预览：把编辑器当前表单快照存入内存 TTL map，返回随机 token；
// 前台 /preview/draft/:token 取快照渲染前台文章页。不落库、不污染分享列表；
// 服务重启后旧预览失效可接受。支持未保存的新文章。

// PostPreview 预览表单快照（字段对齐 PostEditor 表单，仅取渲染所需）。
type PostPreview struct {
	Title     string `json:"title"`
	ContentMD string `json:"content_md"`
	Excerpt   string `json:"excerpt"`
	Cover     string `json:"cover"`
	Topic     string `json:"topic"`
	Slug      string `json:"slug"`
	CreatedAt time.Time
}

type previewEntry struct {
	data    PostPreview
	expires time.Time
}

const previewTTL = 10 * time.Minute

var (
	previewMu    sync.Mutex
	previewStore = map[string]*previewEntry{}
)

// createPostPreview POST /api/v1/preview/post：存表单快照，返回 token。
func (h *Handler) createPostPreview(c *gin.Context) {
	var p PostPreview
	if err := c.ShouldBindJSON(&p); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	p.CreatedAt = time.Now()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		Fail(c, http.StatusInternalServerError, "生成预览失败")
		return
	}
	token := hex.EncodeToString(b)
	previewMu.Lock()
	pruneExpiredLocked()
	previewStore[token] = &previewEntry{data: p, expires: time.Now().Add(previewTTL)}
	previewMu.Unlock()
	c.JSON(http.StatusOK, gin.H{"token": token})
}

// GetPostPreview 供前台 /preview/draft/:token 读取快照；过期或不存在返回 false。
func GetPostPreview(token string) (PostPreview, bool) {
	previewMu.Lock()
	defer previewMu.Unlock()
	pruneExpiredLocked()
	e, ok := previewStore[token]
	if !ok {
		return PostPreview{}, false
	}
	return e.data, true
}

// pruneExpiredLocked 清理过期快照（调用方须持锁）。
func pruneExpiredLocked() {
	now := time.Now()
	for k, e := range previewStore {
		if now.After(e.expires) {
			delete(previewStore, k)
		}
	}
}
