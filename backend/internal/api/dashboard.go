package api

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// quickActionsSection settings 表中存储快捷操作的段名。
const quickActionsSection = "dashboard_quick_actions"

// defaultQuickActions 未配置时展示的默认快捷操作 key 列表。
var defaultQuickActions = []string{"write_post", "review_comments", "upload_attachment", "settings"}

// maxQuickActions 用户最多可选的快捷操作数量。
const maxQuickActions = 8

// dashboardStats GET /api/v1/dashboard/stats：仪表盘统计数据。
func (h *Handler) dashboardStats(c *gin.Context) {
	postTotal, err := h.store.List()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	published, err := h.store.CountPublished()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	commentTotal, err := h.store.CountComments("")
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	commentPending, err := h.store.CountComments(store.CommentPending)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	viewTotal, err := h.store.SumViews()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	attachmentTotal, err := h.store.CountAttachments("", "", nil)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	userTotal, err := h.store.CountUsersByRole("")
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	// 计算周环比（与 7 天前对比）：趋势数据用于 sparkline 和增长百分比
	trend, err := h.store.TrendPVUV(14)
	if err != nil {
		trend = nil
	}

	// 组装 sparkline 数据（最近 7 天 PV）
	sparkline := make([]int, 0, 7)
	if len(trend) >= 7 {
		for _, d := range trend[len(trend)-7:] {
			sparkline = append(sparkline, d.PV)
		}
	} else if len(trend) > 0 {
		for _, d := range trend {
			sparkline = append(sparkline, d.PV)
		}
	}

	// 计算周增长百分比
	var postChange float64
	var viewChange float64
	var commentChange float64
	if len(trend) >= 14 {
		// 简单计算：最近 7 天 vs 前 7 天的 PV 增长
		recentPV := 0
		prevPV := 0
		for i := len(trend) - 7; i < len(trend); i++ {
			recentPV += trend[i].PV
		}
		for i := len(trend) - 14; i < len(trend)-7; i++ {
			prevPV += trend[i].PV
		}
		if prevPV > 0 {
			viewChange = float64(recentPV-prevPV) / float64(prevPV) * 100
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"post_total":       len(postTotal),
		"post_published":   published,
		"comment_total":    commentTotal,
		"comment_pending":  commentPending,
		"view_total":       viewTotal,
		"attachment_total": attachmentTotal,
		"user_total":       userTotal,
		"post_change":      postChange,
		"view_change":      viewChange,
		"comment_change":   commentChange,
		"sparkline":        sparkline,
	})
}

// dashboardRecentComments GET /api/v1/dashboard/recent-comments：最近评论。
func (h *Handler) dashboardRecentComments(c *gin.Context) {
	recent, err := h.store.ListRecentComments(5)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	mirror := h.site.Comments.GravatarMirror
	items := make([]*commentJSON, 0, len(recent))
	for _, cm := range recent {
		items = append(items, toCommentJSON(cm, mirror))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// dashboardRecentPosts GET /api/v1/dashboard/recent-posts：最近文章。
func (h *Handler) dashboardRecentPosts(c *gin.Context) {
	recent, err := h.store.ListRecentPosts(5)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]gin.H, 0, len(recent))
	for _, p := range recent {
		items = append(items, gin.H{
			"slug":      p.Slug,
			"title":     p.Title,
			"category":  p.Category,
			"status":    p.Status,
			"updated_at": p.UpdatedAt.Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// dashboardCategories GET /api/v1/dashboard/categories：分类分布统计。
func (h *Handler) dashboardCategories(c *gin.Context) {
	counts, err := h.store.CountByCategory()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	type catItem struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	total := 0
	items := make([]catItem, 0, len(counts))
	for name, cnt := range counts {
		if name == "" {
			continue
		}
		items = append(items, catItem{Name: name, Count: cnt})
		total += cnt
	}
	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"items": items,
	})
}

// quickActions GET /api/v1/dashboard/quick-actions：读取用户自定义的快捷操作 key 列表。
// 未配置时返回默认列表。前端根据 key 映射到完整的操作信息（label/icon/route）。
func (h *Handler) quickActions(c *gin.Context) {
	raw, err := h.store.GetSetting(quickActionsSection)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if raw == "" {
		c.JSON(http.StatusOK, gin.H{"items": defaultQuickActions})
		return
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		c.JSON(http.StatusOK, gin.H{"items": defaultQuickActions})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// saveQuickActions PUT /api/v1/dashboard/quick-actions：保存用户选择的快捷操作 key 列表。
// 限制：最多 maxQuickActions 项，自动去重去空。
func (h *Handler) saveQuickActions(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		Fail(c, http.StatusBadRequest, "读取请求体失败")
		return
	}
	var req struct {
		Items []string `json:"items"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		Fail(c, http.StatusBadRequest, "无效的 JSON")
		return
	}
	// 去空 + 去重，限制最大数量
	seen := make(map[string]bool)
	items := make([]string, 0, len(req.Items))
	for _, k := range req.Items {
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		items = append(items, k)
		if len(items) >= maxQuickActions {
			break
		}
	}
	data, err := json.Marshal(items)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.store.PutSetting(quickActionsSection, string(data)); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "items": items})
}
