package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"blog/internal/config"
	"blog/internal/linkwatch"
	"blog/internal/store"
)

// linkLabelJSON 友链角标（文案 + 6 位 hex 颜色，不带 #）。
type linkLabelJSON struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// linkJSON 友链 API 响应/请求模型。
type linkJSON struct {
	ID            int64  `json:"id"`
	GroupTitle    string `json:"group_title"`
	Name          string `json:"name"`
	URL           string `json:"url"`
	Avatar        string `json:"avatar"`
	Description   string `json:"description"`
	RSS           string `json:"rss"`
	Label         string `json:"label"`
	Color         string `json:"color"`
	Sort          int    `json:"sort"`
	Snapshot      string `json:"snapshot"`
	LastStatus    int    `json:"last_status"`
	LastCheckedAt int64  `json:"last_checked_at"`
	// Labels 完整有序角标列表（首枚即 label/color）。响应总是给出；请求可省略。
	Labels []linkLabelJSON `json:"labels"`
}

func toLinkJSON(l *store.Link) linkJSON {
	return linkJSON{
		ID: l.ID, GroupTitle: l.GroupTitle, Name: l.Name, URL: l.URL,
		Avatar: l.Avatar, Description: l.Description, RSS: l.RSS,
		Label: l.Label, Color: l.Color, Sort: l.Sort, Snapshot: l.Snapshot,
		LastStatus: l.LastStatus, LastCheckedAt: l.LastCheckedAt,
		Labels: linkLabelsJSON(l),
	}
}

// linkLabelsJSON 拼出完整有序角标列表：主角标在首位，附加角标紧随其后。
func linkLabelsJSON(l *store.Link) []linkLabelJSON {
	labels := make([]linkLabelJSON, 0, store.MaxLinkLabels)
	if name := strings.TrimSpace(l.Label); name != "" {
		labels = append(labels, linkLabelJSON{Name: name, Color: l.Color})
	}
	for _, extra := range l.ExtraLabels() {
		labels = append(labels, linkLabelJSON{Name: extra.Name, Color: extra.Color})
	}
	return labels
}

// requestedLabels 归一化请求里的角标：labels 提供时以它为准；省略时用 label/color
// 作首枚，并保留 existingExtras（兼容不识别多角标的旧客户端，避免编辑时清空附加角标）。
func (r *linkJSON) requestedLabels(existingExtras []store.LinkLabel) ([]store.LinkLabel, error) {
	var list []store.LinkLabel
	if r.Labels != nil {
		for _, l := range r.Labels {
			list = append(list, store.LinkLabel{Name: l.Name, Color: l.Color})
		}
	} else {
		list = append(list, store.LinkLabel{Name: r.Label, Color: r.Color})
		list = append(list, existingExtras...)
	}
	list = store.NormalizeLinkLabels(list)
	if len(list) > store.MaxLinkLabels {
		return nil, fmt.Errorf("角标最多 %d 枚", store.MaxLinkLabels)
	}
	for _, l := range list {
		if utf8.RuneCountInString(l.Name) > store.MaxLinkLabelNameRunes {
			return nil, fmt.Errorf("单个角标文案最多 %d 字", store.MaxLinkLabelNameRunes)
		}
	}
	return list, nil
}

// applyLabels 把归一化后的角标写回存储字段：首枚进 label/color，其余进 labels JSON。
func applyLabels(l *store.Link, labels []store.LinkLabel) {
	l.Label, l.Color, l.LabelsJSON = "", "", ""
	if len(labels) > 0 {
		l.Label, l.Color = labels[0].Name, labels[0].Color
	}
	rest := []store.LinkLabel(nil)
	if len(labels) > 1 {
		rest = labels[1:]
	}
	l.LabelsJSON = store.EncodeLinkLabels(rest)
}

func (r *linkJSON) toStore() *store.Link {
	return &store.Link{
		GroupTitle:    strings.TrimSpace(r.GroupTitle),
		Name:          strings.TrimSpace(r.Name),
		URL:           strings.TrimSpace(r.URL),
		Avatar:        strings.TrimSpace(r.Avatar),
		Description:   strings.TrimSpace(r.Description),
		RSS:           strings.TrimSpace(r.RSS),
		Label:         strings.TrimSpace(r.Label),
		Color:         strings.TrimSpace(r.Color),
		Sort:          r.Sort,
		Snapshot:      strings.TrimSpace(r.Snapshot),
		LastStatus:    r.LastStatus,
		LastCheckedAt: r.LastCheckedAt,
	}
}

// listLinks GET /api/v1/links：友链扁平列表（含 group_title，控制台按组渲染）。
func (h *Handler) listLinks(c *gin.Context) {
	links, err := h.store.ListLinks()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]linkJSON, 0, len(links))
	for _, l := range links {
		items = append(items, toLinkJSON(l))
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// createLink POST /api/v1/links：新增友链，成功后热更新站点。
func (h *Handler) createLink(c *gin.Context) {
	var req linkJSON
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.Name == "" || req.URL == "" {
		Fail(c, http.StatusBadRequest, "名称与链接不能为空")
		return
	}
	labels, err := req.requestedLabels(nil)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	l := req.toStore()
	applyLabels(l, labels)
	if _, err := h.store.CreateLink(l); err != nil {
		Fail(c, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}
	h.afterLinksChanged()
	c.JSON(http.StatusCreated, toLinkJSON(l))
}

// updateLink PUT /api/v1/links/:id：更新友链，成功后热更新站点。
func (h *Handler) updateLink(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "非法 id")
		return
	}
	existing, err := h.store.GetLink(id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		Fail(c, http.StatusNotFound, "友链不存在")
		return
	}
	var req linkJSON
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.Name == "" || req.URL == "" {
		Fail(c, http.StatusBadRequest, "名称与链接不能为空")
		return
	}
	labels, err := req.requestedLabels(existing.ExtraLabels())
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	l := req.toStore()
	applyLabels(l, labels)
	// 巡检状态由探测器维护，不接受友链编辑请求覆盖；兼容旧版客户端省略这些字段。
	l.LastStatus = existing.LastStatus
	l.LastCheckedAt = existing.LastCheckedAt
	if err := h.store.UpdateLink(id, l); err != nil {
		Fail(c, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}
	l.ID = id
	h.afterLinksChanged()
	c.JSON(http.StatusOK, toLinkJSON(l))
}

// deleteLink DELETE /api/v1/links/:id：删除友链，成功后热更新站点。
func (h *Handler) deleteLink(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "非法 id")
		return
	}
	if err := h.store.DeleteLink(id); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.afterLinksChanged()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// afterLinksChanged 友链增删改后：以 DB 为准重建站点，并触发聚合器即时刷新（若已注册）。
func (h *Handler) afterLinksChanged() {
	_ = config.RebuildSite(h.store, h.site)
	if h.onLinksChanged != nil {
		h.onLinksChanged()
	}
}

// 检测后自动打/摘的失效标签（Spec 56）：无标签的不可达友链自动标记，恢复正常后自动清除。
const (
	expiredLabel = linkwatch.ExpiredLabel
	expiredColor = linkwatch.ExpiredColor
)

// linkCheckResult 单条友链检测结果。
type linkCheckResult = linkwatch.Result

// checkLinks POST /api/v1/links/check：并发检测各友链 URL 可达性，返回每条 {id, ok, status}。
// status：HTTP 状态码；网络错误/超时为 0。
// Spec 56：不可达且无标签的友链自动打「失效」标签；恢复正常且标签为「失效」时自动摘除。
func (h *Handler) checkLinks(c *gin.Context) {
	links, err := h.store.ListLinks()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	results := make([]linkCheckResult, len(links))
	var wg sync.WaitGroup
	for i, l := range links {
		if l.URL == "" {
			results[i] = linkCheckResult{ID: l.ID, OK: false}
			continue
		}
		wg.Add(1)
		go func(idx int, id int64, url string) {
			defer wg.Done()
			ok, status := probeURL(url)
			results[idx] = linkCheckResult{ID: id, OK: ok, Status: status}
		}(i, l.ID, l.URL)
	}
	wg.Wait()
	checkedAt := time.Now().Unix()
	for _, result := range results {
		if err := h.store.UpdateLinkCheck(result.ID, result.Status, checkedAt); err != nil {
			Fail(c, http.StatusInternalServerError, "保存检测结果失败: "+err.Error())
			return
		}
	}
	for i, result := range results {
		links[i].LastStatus = result.Status
		links[i].LastCheckedAt = checkedAt
	}

	tagged, restored := applyExpiredLabels(h.store, links, results)
	// Health timestamps are part of the public link classification even when
	// no automatic label was added or removed, so always rebuild and refresh
	// the friends cache after persisting a check cycle.
	h.afterLinksChanged()
	c.JSON(http.StatusOK, gin.H{"items": results, "tagged": tagged, "restored": restored})
}

// applyExpiredLabels 依据检测结果自动维护「失效」标签：
// 不可达且无标签 → 打标；可达且标签恰为「失效」 → 摘除；已有其他标签不动。
// results 与 links 下标一一对应，返回打标/摘除条数。
func applyExpiredLabels(st *store.Store, links []*store.Link, results []linkCheckResult) (int, int) {
	return linkwatch.ApplyExpiredLabels(st, links, results)
}

// probeURL HEAD 探测（失败回退 GET），返回 (可达, HTTP 状态码)；2xx/3xx 视为可达，网络错误状态码为 0。
// var 形式便于测试注入：生产实现走 linkwatch.ProbeURL（safefetch 加固，拒绝环回/私网），
// 单测对 127.0.0.1 httptest 服务验证判定逻辑时替换为普通 client 版本。
var probeURL = linkwatch.ProbeURL

// snapshotLink POST /api/v1/links/:id/snapshot：立即抓取单条友链截图。
func (h *Handler) snapshotLink(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "非法 id")
		return
	}
	if h.linkWatch == nil {
		Fail(c, http.StatusServiceUnavailable, "截图巡检器未启用")
		return
	}
	link, err := h.linkWatch.Snapshot(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			Fail(c, http.StatusNotFound, "友链不存在")
			return
		}
		Fail(c, http.StatusBadGateway, "截图失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, toLinkJSON(link))
}
