package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"blog/internal/content"
)

// 发布前预览：把编辑器当前表单快照存入内存 TTL map，返回随机 token；
// 前台 /preview/draft/:token 取快照渲染前台文章页。不落库、不污染分享列表；
// 服务重启后旧预览失效可接受。支持未保存的新文章。

// PostPreview is retained as an API compatibility alias during the module move.
type PostPreview = content.PostPreview

// createPostPreview POST /api/v1/preview/post：存表单快照，返回 token。
func (h *Handler) createPostPreview(c *gin.Context) {
	var p content.PostPreview
	if err := c.ShouldBindJSON(&p); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	if _, err := h.renderEditorStellar(p.ContentMD, StellarContentMetadata{p.StellarVersion, p.StellarConfig, p.BlogExtensions}, "post"); err != nil {
		failEditorStellar(c, err)
		return
	}
	token, err := content.CreatePostPreview(p)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "生成预览失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

// GetPostPreview 供前台 /preview/draft/:token 读取快照；过期或不存在返回 false。
func GetPostPreview(token string) (PostPreview, bool) {
	return content.GetPostPreview(token)
}
