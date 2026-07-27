package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// dashboardStats GET /api/v1/dashboard/stats：仪表盘统计数据。
func (h *Handler) dashboardStats(c *gin.Context) {
	postTotal, err := h.store.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	published, err := h.store.CountPublished()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	commentTotal, err := h.store.CountComments("")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	commentPending, err := h.store.CountComments(store.CommentPending)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	viewTotal, err := h.store.SumViews()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	attachmentTotal, err := h.store.CountAttachments("", "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"post_total":       len(postTotal),
		"post_published":   published,
		"comment_total":    commentTotal,
		"comment_pending":  commentPending,
		"view_total":       viewTotal,
		"attachment_total": attachmentTotal,
	})
}

// dashboardRecentComments GET /api/v1/dashboard/recent-comments：最近评论。
func (h *Handler) dashboardRecentComments(c *gin.Context) {
	recent, err := h.store.ListRecentComments(5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	mirror := h.site.Comments.GravatarMirror
	items := make([]*commentJSON, 0, len(recent))
	for _, cm := range recent {
		items = append(items, toCommentJSON(cm, mirror))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// dashboardLayoutSection settings 表中仪表盘布局的 section 名。
// 不在 config.KnownSections() 内：不出现在站点设置、不触发站点热更新。
const dashboardLayoutSection = "dashboard_layout"

// dashboardLayoutItem 布局元素：仅记录部件类型，尺寸由前端注册表决定。
type dashboardLayoutItem struct {
	Type string `json:"type"`
}

// defaultDashboardLayout 默认布局（与部件化前的页面顺序一致）。
var defaultDashboardLayout = []dashboardLayoutItem{
	{Type: "stat-posts"},
	{Type: "stat-comments"},
	{Type: "stat-views"},
	{Type: "stat-attachments"},
	{Type: "quick-actions"},
	{Type: "recent-comments"},
	{Type: "trend-chart"},
	{Type: "hourly-chart"},
}

// getDashboardLayout GET /api/v1/dashboard/layout：读取仪表盘布局，未保存过时返回默认布局。
func (h *Handler) getDashboardLayout(c *gin.Context) {
	raw, err := h.store.GetSetting(dashboardLayoutSection)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if raw == "" {
		c.JSON(http.StatusOK, gin.H{"items": defaultDashboardLayout})
		return
	}
	var items []dashboardLayoutItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		// 存量数据损坏时兜底默认布局
		c.JSON(http.StatusOK, gin.H{"items": defaultDashboardLayout})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// putDashboardLayout PUT /api/v1/dashboard/layout：校验并保存仪表盘布局。
func (h *Handler) putDashboardLayout(c *gin.Context) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取请求体失败"})
		return
	}
	var body struct {
		Items []dashboardLayoutItem `json:"items"`
	}
	if err := json.Unmarshal(raw, &body); err != nil || body.Items == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "布局格式错误"})
		return
	}
	for _, it := range body.Items {
		if it.Type == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "部件 type 不能为空"})
			return
		}
	}
	val, err := json.Marshal(body.Items)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.store.PutSetting(dashboardLayoutSection, string(val)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
