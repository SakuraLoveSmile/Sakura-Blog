package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"blog/internal/plugin"
)

type ScriptPluginListResponse struct {
	Items []plugin.Manifest `json:"items"`
}

type ScriptPluginHookRequest struct {
	Hook  plugin.Hook     `json:"hook"`
	Value json.RawMessage `json:"value"`
}

type ScriptPluginHookResponse struct {
	Value any `json:"value"`
}

func (h *Handler) listScriptPlugins(c *gin.Context) {
	if h.plugins == nil {
		c.JSON(http.StatusOK, ScriptPluginListResponse{Items: []plugin.Manifest{}})
		return
	}
	c.JSON(http.StatusOK, ScriptPluginListResponse{Items: h.plugins.Manifests()})
}

// runScriptPluginHook is an admin-only diagnostic/extension bridge. The
// runtime still enforces the manifest hook and permission declarations before
// any script executes.
func (h *Handler) runScriptPluginHook(c *gin.Context) {
	if h.plugins == nil {
		Fail(c, http.StatusServiceUnavailable, "脚本插件运行时未初始化")
		return
	}
	var request ScriptPluginHookRequest
	if err := c.ShouldBindJSON(&request); err != nil || len(request.Value) == 0 {
		Fail(c, http.StatusBadRequest, "插件 hook 请求格式错误")
		return
	}
	var value any
	if err := json.Unmarshal(request.Value, &value); err != nil {
		Fail(c, http.StatusBadRequest, "插件 hook value 必须是 JSON")
		return
	}
	result, err := h.plugins.Apply(c.Request.Context(), request.Hook, value)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, ScriptPluginHookResponse{Value: result})
}
