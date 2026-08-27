package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// parseIDParam 解析路径中的 :id 参数，非法时写 400 并返回 false。
func (h *Handler) parseIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		Fail(c, http.StatusBadRequest, "无效的 ID")
		return 0, false
	}
	return id, true
}
